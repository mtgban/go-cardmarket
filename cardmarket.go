// Package cardmarket reads Cardmarket's marketplace: the signed API that
// answers for products, expansions and listings, and the catalog files the
// site publishes for anyone to download. The entities are named as
// https://apiv2.cardmarket.com/ws/documentation names them, so a field can be
// looked up in one place rather than translated first.
package cardmarket

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

const (
	productsBaseURL   = "https://apiv2.cardmarket.com/ws/v2.0/output.json/products/"
	articlesBaseURL   = "https://apiv2.cardmarket.com/ws/v2.0/output.json/articles/"
	expansionsBaseURL = "https://apiv2.cardmarket.com/ws/v2.0/output.json/expansions/"

	gameExpansionsURL = "https://apiv2.cardmarket.com/ws/v2.0/output.json/games/%d/expansions"

	// MaxEntities is how many results one request may ask for
	MaxEntities = 100

	// requestTimeout bounds a single attempt. The retry policy already
	// bounds how many attempts there are; without this a connection that
	// accepts and then stalls holds the walk open with nothing to time it
	// out but the caller's context.
	requestTimeout = 60 * time.Second

	// rateLimitRetries and serverErrorRetries are two budgets because the
	// two failures they answer are not the same failure. A 429 is the
	// concurrency limiter saying "not yet", carries a Retry-After of one
	// second, and clears - waiting it out twenty times costs twenty seconds
	// and works. A 5xx is the API unwell on one particular request, and no
	// number of attempts has ever talked one round: expansion 1645
	// ("Pokemon Products") and 5303 ("Unnumbered Promos") each answered 21
	// straight 5xx over six minutes, on more than one day, and failed the
	// whole dump both times. Spending a fifth of the budget there fails in
	// well under a minute and leaves the patience where it is earned.
	rateLimitRetries   = 20
	serverErrorRetries = 4
)

// attemptsKey carries the per-request tally checkRetry counts a 5xx with.
// retryablehttp hands CheckRetry the request's context and nothing else - no
// attempt number - so a budget narrower than RetryMax has to travel with the
// request itself.
type attemptsKey struct{}

// attempts is that tally. One request is in flight at a time by design (see
// NewClient), and each gets its own, so it needs no locking.
type attempts struct{ serverErrors int }

// withAttempts gives a request its own tally. A request made without one is
// not refused - it falls back to the single RetryMax budget, which is the old
// behaviour rather than a new failure.
func withAttempts(ctx context.Context) context.Context {
	return context.WithValue(ctx, attemptsKey{}, &attempts{})
}

// checkRetry is retryablehttp's own policy with the 5xx budget applied on
// top: what gets retried is unchanged, how long it keeps being retried is not.
func checkRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	retry, policyErr := retryablehttp.DefaultRetryPolicy(ctx, resp, err)
	if policyErr != nil || !retry {
		return retry, policyErr
	}

	// Transport errors are counted with the server's: a connection that
	// refuses or stalls repeatedly is the same kind of unwell, and at a
	// 60-second timeout apiece it is the more expensive kind to sit through.
	if resp != nil && resp.StatusCode < 500 {
		return true, nil
	}

	tally, ok := ctx.Value(attemptsKey{}).(*attempts)
	if !ok {
		return true, nil
	}
	tally.serverErrors++
	if tally.serverErrors < serverErrorRetries {
		return true, nil
	}

	// Stopping with a nil error would be read as "this response is the
	// final, acceptable one": retryablehttp's success test is doErr,
	// checkErr and shouldRetry all clear, so it would return the 503 as a
	// response and never reach ErrorHandler. A 503 whose body happens to
	// fit the shape being decoded - {"single":[]} - would then read as a
	// shelf with no cards and unprice every card on it, silently, which is
	// the failure this budget exists to make loud. Stopping with an error
	// takes the failure path instead.
	if resp != nil {
		return false, fmt.Errorf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	return false, err
}

// giveUp names what the client gave up on. retryablehttp's own message is
// "GET <url> giving up after N attempt(s)", which says how hard it tried and
// not one word about why - and the difference between a rate limit to wait
// out and a 500 to route around is exactly what a caller needs to know.
func giveUp(resp *http.Response, err error, tries int) (*http.Response, error) {
	if resp != nil {
		resp.Body.Close()
		return nil, fmt.Errorf("cardmarket: %d %s after %d attempt(s)",
			resp.StatusCode, http.StatusText(resp.StatusCode), tries)
	}
	return nil, fmt.Errorf("cardmarket: after %d attempt(s): %w", tries, err)
}

// Client reads Cardmarket's API, which signs every request with an app token
// and secret.
type Client struct {
	client *http.Client
	auth   *authTransport
}

// NewClient returns a client signing with the given app credentials.
//
// The API is very sensitive to concurrent requests and answers a burst with
// 429s - measured as a concurrency limiter rather than a rate limiter: one
// request in flight at a time clears with no 429s at all, at roughly the
// same throughput a burst of many concurrent requests achieves after most of
// them are rejected and retried. Every 429 observed carries a `Retry-After`
// of one second, so the backoff honours it (falling back to jittered
// exponential backoff, up to 2-10 seconds times the attempt number, for a
// response with none) rather than waiting a fixed multi-second interval
// regardless of what the server actually asked for. A 429 is retried up to
// rateLimitRetries times and a 5xx only serverErrorRetries, for the reasons
// given there; give long walks a context deadline, since neither is a bound
// anyone should rely on.
func NewClient(appToken, appSecret string) *Client {
	mkm := Client{}
	client := retryablehttp.NewClient()
	client.Logger = nil
	client.Backoff = retryablehttp.DefaultBackoff
	client.RetryWaitMin = 2 * time.Second
	client.RetryWaitMax = 10 * time.Second
	client.RetryMax = rateLimitRetries
	client.CheckRetry = checkRetry
	client.ErrorHandler = giveUp
	client.HTTPClient.Timeout = requestTimeout

	auth := &authTransport{
		Parent:    client.HTTPClient.Transport,
		AppToken:  appToken,
		AppSecret: appSecret,
	}

	client.HTTPClient.Transport = auth
	mkm.auth = auth
	mkm.client = client.StandardClient()
	return &mkm
}

// RequestNo returns how many requests the client has made, which matters
// against Cardmarket's daily allowance.
func (mkm *Client) RequestNo() int {
	return int(mkm.auth.RequestNo.Load())
}

// APIError is the body Cardmarket answers a rejected request with. The
// description is the only part meant for a person; the rest says which of
// the request it objected to.
type APIError struct {
	Description       string `json:"mkm_error_description"`
	InternalCode      any    `json:"internal_error_code"`
	PostDataField     any    `json:"post_data_field"`
	StatusCode        int    `json:"http_status_code"`
	StatusDescription string `json:"http_status_code_description"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("cardmarket: %d %s: %s", e.StatusCode, e.StatusDescription, e.Description)
	}
	return "cardmarket: " + e.Description
}

// Link is one HATEOAS entry, which every entity carries: what the link is
// for, where it points, and the method that follows it.
type Link struct {
	Rel    string `json:"rel"`
	Href   string `json:"href"`
	Method string `json:"method"`
}

// Localization is a name as one language spells it. Every named entity
// carries the five languages the marketplace trades in.
type Localization struct {
	Name         string `json:"name"`
	IDLanguage   int    `json:"idLanguage"`
	LanguageName string `json:"languageName"`
}

// Expansion is a set as Cardmarket files it.
type Expansion struct {
	IDExpansion  int            `json:"idExpansion"`
	Name         string         `json:"enName"`
	Localization []Localization `json:"localization,omitempty"`
	// SetCode is the marketplace's own abbreviation, which is the only
	// field telling a foreign catalog from the English one it shadows.
	SetCode     string `json:"abbreviation"`
	Icon        int    `json:"icon"`
	ReleaseDate string `json:"releaseDate"`
	IsReleased  bool   `json:"isReleased"`
	IDGame      int    `json:"idGame"`
	Links       []Link `json:"links,omitempty"`
}

// get performs a signed request and decodes the body into out. A body that
// is not the expected shape is returned as the API's own error where it sent
// one, and verbatim where it did not. The response's Content-Range header is
// returned alongside, verbatim and unparsed (see ParseContentRange) - a
// listing endpoint is the only caller that has any use for it, but reading
// it here means no caller has to reach past the decoded body for it.
func (mkm *Client) get(ctx context.Context, link string, out any) (string, error) {
	req, err := http.NewRequestWithContext(withAttempts(ctx), http.MethodGet, link, http.NoBody)
	if err != nil {
		return "", err
	}

	resp, err := mkm.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	contentRange := resp.Header.Get("Content-Range")

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return contentRange, err
	}

	// A non-2xx status with an empty body is a real error, not a clean
	// zero-result answer - retryablehttp's own policy already exhausted its
	// retries on anything retryable (429, 5xx) before this ever returns, so
	// what's left is either a genuine rejection (401, 403, ...) or one of
	// the API's own documented empty successes (204, on a query matching
	// nothing). Trusting an empty body regardless of status once let an
	// edge-level block (403, empty body, no APIError JSON to catch it) read
	// as "no error, nothing found" - silently wrong, not merely incomplete.
	if (resp.StatusCode < 200 || resp.StatusCode >= 300) && len(data) == 0 {
		return contentRange, fmt.Errorf("cardmarket: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	if len(data) == 0 {
		return contentRange, nil
	}

	// The error body decodes as an object like any other, so it is read
	// first: unmarshalling it into the expected shape would succeed and
	// hand back an empty result.
	var apiErr APIError
	if json.Unmarshal(data, &apiErr) == nil && apiErr.Description != "" {
		return contentRange, &apiErr
	}

	err = json.Unmarshal(data, out)
	if err != nil {
		return contentRange, errors.New(string(data))
	}
	return contentRange, nil
}

// ParseContentRange reads the total listing count out of a Content-Range
// header ("0-29/33", or "0-99/1000+" once Cardmarket stops counting past its
// own cap). Capped reports the latter case: total is 1000, the highest
// figure the API ever reports, not the real count, which is unknowable past
// it. An empty or unrecognized header reports ok=false rather than a
// misleading zero.
func ParseContentRange(header string) (total int, capped bool, ok bool) {
	_, totalPart, found := strings.Cut(header, "/")
	if !found || totalPart == "" {
		return 0, false, false
	}
	if strings.HasSuffix(totalPart, "+") {
		capped = true
		totalPart = strings.TrimSuffix(totalPart, "+")
	}
	total, err := strconv.Atoi(totalPart)
	if err != nil {
		return 0, false, false
	}
	return total, capped, true
}

// Expansions returns every expansion of one game.
func (mkm *Client) Expansions(ctx context.Context, gameID int) ([]Expansion, error) {
	var response struct {
		Expansions []Expansion `json:"expansion"`
	}
	_, err := mkm.get(ctx, fmt.Sprintf(gameExpansionsURL, gameID), &response)
	if err != nil {
		return nil, err
	}
	return response.Expansions, nil
}

// Product is one catalog entry, a card or a sealed item.
//
// Two of its fields depend on which request built it, the marketplace
// answering the same entity two ways: ExpansionName and ExpansionIcon are
// filled by ExpansionSingles, and Expansion and PriceGuide by Product. A
// product read one way carries nothing of the other's.
type Product struct {
	IDProduct     int `json:"idProduct"`
	IDMetaproduct int `json:"idMetaproduct"`
	// CountReprints is how many products the metaproduct bundles, which is
	// how many times the card has been printed across every expansion.
	CountReprints int            `json:"countReprints"`
	Name          string         `json:"enName"`
	LocName       string         `json:"locName"`
	Localization  []Localization `json:"localization,omitempty"`
	Website       string         `json:"website"`
	// Image is a pre-signed address that expires an hour after the request
	// that returned it. It is worth reading and never worth storing.
	Image        string `json:"image"`
	GameName     string `json:"gameName"`
	CategoryName string `json:"categoryName"`
	IDGame       int    `json:"idGame"`
	Number       string `json:"number"`
	// Rarity is what the marketplace calls the printing's rarity. It is the
	// game's own vocabulary where the game has one - Common through Secret
	// Rare for Pokemon - and "Promo" for a whole promotional shelf.
	Rarity string `json:"rarity"`
	// ExpansionName and ExpansionIcon are the ExpansionSingles form.
	ExpansionName string `json:"expansionName"`
	ExpansionIcon int    `json:"expansionIcon"`
	// ExpansionCode is the marketplace's own abbreviation of the
	// expansion, read off the catalog rather than the product
	ExpansionCode string `json:"-"`
	// Expansion and PriceGuide are the single-product form.
	Expansion struct {
		IDExpansion   int    `json:"idExpansion"`
		Name          string `json:"enName"`
		ExpansionIcon int    `json:"expansionIcon"`
	} `json:"expansion"`
	// PriceGuide is keyed by the marketplace's own headings - SELL, LOW,
	// LOWEX, LOWFOIL, AVG, TREND, TRENDFOIL. The documentation spells the
	// third LOWEX+; the API sends LOWEX.
	PriceGuide    map[string]float64 `json:"priceGuide"`
	CountArticles int                `json:"countArticles"`
	CountFoils    int                `json:"countFoils"`
	Links         []Link             `json:"links,omitempty"`
}

// Product returns one catalog entry by id.
func (mkm *Client) Product(ctx context.Context, id int) (*Product, error) {
	var response struct {
		Product Product `json:"product"`
	}
	_, err := mkm.get(ctx, productsBaseURL+fmt.Sprint(id), &response)
	if err != nil {
		return nil, err
	}
	return &response.Product, nil
}

// ExpansionSingles returns every single card in one expansion.
func (mkm *Client) ExpansionSingles(ctx context.Context, id int) ([]Product, error) {
	var response struct {
		Expansion Expansion `json:"expansion"`
		Single    []Product `json:"single"`
	}
	_, err := mkm.get(ctx, expansionsBaseURL+fmt.Sprint(id)+"/singles", &response)
	if err != nil {
		return nil, err
	}
	return response.Single, nil
}

// ArticlePrice is one listing's price in one currency. A listing is quoted
// in the seller's currency and in every currency the marketplace converts
// it to.
type ArticlePrice struct {
	Price        float64 `json:"price"`
	IDCurrency   int     `json:"idCurrency"`
	CurrencyCode string  `json:"currencyCode"`
}

// ArticleSeller is who is selling, minus the fields naming the person. The
// marketplace answers a seller's first name, email, phone, VAT number and
// legal information beside their trading record; none of that is needed to
// judge a price, so none of it is decoded and none of it can leak into a
// log of an article.
type ArticleSeller struct {
	IDUser       int    `json:"idUser"`
	Username     string `json:"username"`
	IsCommercial int    `json:"isCommercial"`
	IsSeller     bool   `json:"isSeller"`
	Reputation   int    `json:"reputation"`
	SellCount    int    `json:"sellCount"`
	SoldItems    int    `json:"soldItems"`
	OnVacation   bool   `json:"onVacation"`
	// AvgShippingTime is in days, and RiskGroup and LossPercentage are the
	// marketplace's own read on whether an order will arrive.
	AvgShippingTime int    `json:"avgShippingTime"`
	RiskGroup       int    `json:"riskGroup"`
	LossPercentage  string `json:"lossPercentage"`
	Address         struct {
		Country string `json:"country"`
	} `json:"address"`
}

// Article is one seller's listing on a product.
//
// The finish flags are the game's own and not one vocabulary: a Magic
// listing carries IsFoil and neither of the others, a Pokemon listing
// carries IsFirstEd and IsReverseHolo and no IsFoil at all. A flag absent
// from a game's listings decodes false, so read the one its game uses.
type Article struct {
	IDArticle int `json:"idArticle"`
	IDProduct int `json:"idProduct"`
	Language  struct {
		IDLanguage   int    `json:"idLanguage"`
		LanguageName string `json:"languageName"`
	} `json:"language"`
	Comments       string         `json:"comments"`
	Price          float64        `json:"price"`
	IDCurrency     int            `json:"idCurrency"`
	CurrencyCode   string         `json:"currencyCode"`
	Prices         []ArticlePrice `json:"prices,omitempty"`
	Count          int            `json:"count"`
	InShoppingCart bool           `json:"inShoppingCart"`
	Condition      string         `json:"condition"`
	Product        struct {
		Name        string `json:"enName"`
		LocName     string `json:"locName"`
		Expansion   string `json:"expansion"`
		IDExpansion int    `json:"idExpansion"`
		SetCode     string `json:"abbreviation"`
		Number      string `json:"nr"`
		Rarity      string `json:"rarity"`
		IDGame      int    `json:"idGame"`
		ExpIcon     int    `json:"expIcon"`
	} `json:"product"`
	Seller ArticleSeller `json:"seller"`

	IsFoil        bool `json:"isFoil"`
	IsFirstEd     bool `json:"isFirstEd"`
	IsReverseHolo bool `json:"isReverseHolo"`
	IsSigned      bool `json:"isSigned"`
	IsAltered     bool `json:"isAltered"`

	Links []Link `json:"links,omitempty"`
}

// DefaultArticleFilter is the filter a price should be read through: played
// or better, from a seller with a record, and neither signed nor altered.
// Anything looser prices a card off a listing nobody would buy.
func DefaultArticleFilter(onlyEnglish bool) map[string]string {
	options := map[string]string{
		"minCondition": "GD",
		"minUserScore": "3",
		"isSigned":     "false",
		"isAltered":    "false",
	}
	if onlyEnglish {
		options["idLanguage"] = "1"
	}
	return options
}

// Articles returns the listings on a product, one page at a time, together
// with the listing's true total count (see ParseContentRange) so a caller
// can stop paginating once it's covered the total, or confirm a filter
// (isFoil, idLanguage, ...) actually narrowed the result rather than being
// silently ignored - Cardmarket's filters fail open on an unrecognized name
// or value, answering the unfiltered list rather than an error. total is 0
// when the header is missing or unparseable; capped reports Cardmarket's own
// 1000-result ceiling, past which the real count is unknowable. Pages start
// at zero, and the API requires both bounds: asking for a page size without
// a start is refused.
func (mkm *Client) Articles(ctx context.Context, id int, options map[string]string, page, maxResults int) (articles []Article, total int, capped bool, err error) {
	u, err := url.Parse(articlesBaseURL + fmt.Sprint(id))
	if err != nil {
		return nil, 0, false, err
	}
	params := url.Values{}
	for key, value := range options {
		params.Set(key, value)
	}
	params.Set("start", fmt.Sprint(page*maxResults))
	params.Set("maxResults", fmt.Sprint(maxResults))
	u.RawQuery = params.Encode()

	var response struct {
		Articles []Article `json:"article"`
	}
	contentRange, err := mkm.get(ctx, u.String(), &response)
	if err != nil {
		return nil, 0, false, err
	}
	total, capped, _ = ParseContentRange(contentRange)
	return response.Articles, total, capped, nil
}

type authTransport struct {
	Parent    http.RoundTripper
	AppToken  string
	AppSecret string

	// May be empty
	AccessToken       string
	AccessTokenSecret string

	RequestNo atomic.Int64
}

// addQueryParams folds a request's query parameters into the set a request
// is signed with, every value of a repeated key included and sorted - the
// request itself still carries all of them on the wire regardless of what
// order they're signed in, so a signature computed over only the first, or
// over the rest in the wrong order, mismatches what the server actually
// checks and fails with an opaque 401.
//
// The sort is numeric, not RFC 5849 §3.4.1.3.2's byte-value ordering:
// confirmed live (varying only the wire order of a repeated sellerCountry
// value against the real API) that Cardmarket's server expects a repeated
// key's values normalized by numeric value - "4" before "13" - where
// byte-value order would put "13" first. Sending them in the wire's own
// order authenticated only when that happened to already be ascending; the
// same values reversed on the wire, signed in that same (now descending)
// order, came back 401. A value that doesn't parse as a plain integer falls
// back to byte-value sorting, since nothing suggests this normalization
// extends past the one parameter (a numeric id) it's ever been observed to
// accept repeated at all.
//
// Currently unreachable through this package's own exported calls
// (Client.Articles builds its query from a plain map[string]string, which
// cannot hold a repeated key), but a latent trap for any future caller who
// builds a request with one directly.
func addQueryParams(q url.Values, query url.Values) {
	for key, values := range query {
		sorted := slices.Clone(values)
		sort.Slice(sorted, func(i, j int) bool {
			a, aErr := strconv.Atoi(sorted[i])
			b, bErr := strconv.Atoi(sorted[j])
			if aErr == nil && bErr == nil {
				return a < b
			}
			return sorted[i] < sorted[j]
		})
		for _, value := range sorted {
			q.Add(key, value)
		}
	}
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Generate nonce
	rawID := make([]byte, 16)
	_, err := rand.Read(rawID)
	if err != nil {
		return nil, fmt.Errorf("unable to generate nonce: %w", err)
	}

	nonce := base64.RawStdEncoding.EncodeToString(rawID)

	// Items we need
	q := url.Values{}
	q.Set("oauth_consumer_key", t.AppToken)
	q.Set("oauth_nonce", nonce)
	q.Set("oauth_signature_method", "HMAC-SHA1")
	q.Set("oauth_timestamp", fmt.Sprintf("%d", time.Now().Unix()))
	q.Set("oauth_token", t.AccessToken)
	q.Set("oauth_version", "1.0")

	addQueryParams(q, req.URL.Query())
	// MKM expects path-encoded queries because javascript, but q.Encode() uses
	// the query-encoding, so perform the only replacement that matters
	queries := strings.Replace(q.Encode(), "+", "%20", -1)

	// Duplicate request url and drop query parameters
	authURL := &url.URL{}
	*authURL = *req.URL
	authURL.RawQuery = ""

	// Message and key
	msg := fmt.Sprintf("%s&%s&%s", req.Method, url.QueryEscape(authURL.String()), url.QueryEscape(queries))

	signkey := fmt.Sprintf("%s&%s", url.QueryEscape(t.AppSecret), url.QueryEscape(t.AccessTokenSecret))

	mac := hmac.New(sha1.New, []byte(signkey))
	mac.Write([]byte(msg))
	msgHash := mac.Sum(nil)
	signature := base64.StdEncoding.EncodeToString(msgHash)

	// Build the header
	auth := "OAuth realm=\"" + authURL.String() + "\", "
	for key, val := range q {
		// Only keep oauth parameters here
		if !strings.HasPrefix(key, "oauth") {
			continue
		}
		auth += key + "=\"" + val[0] + "\", "
	}
	auth += "oauth_signature=\"" + signature + "\""

	req.Header.Set("Authorization", auth)

	t.RequestNo.Add(1)
	return t.Parent.RoundTrip(req)
}
