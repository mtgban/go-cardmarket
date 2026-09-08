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
)

// Client reads Cardmarket's API, which signs every request with an app token
// and secret.
type Client struct {
	client *http.Client
	auth   *authTransport
}

// NewClient returns a client signing with the given app credentials.
//
// The API is very sensitive to concurrent requests and answers a burst with
// 429s, so the retry is deliberately patient: up to 20 attempts, each waiting
// a jittered 2 to 10 seconds multiplied by the attempt number. That is up to
// half an hour of backoff for one unlucky request, which is the intended
// behaviour against a rate limiter but is not a bound anyone should rely on -
// give long walks a context deadline.
func NewClient(appToken, appSecret string) *Client {
	mkm := Client{}
	client := retryablehttp.NewClient()
	client.Logger = nil
	client.Backoff = retryablehttp.LinearJitterBackoff
	client.RetryWaitMin = 2 * time.Second
	client.RetryWaitMax = 10 * time.Second
	client.RetryMax = 20
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
// one, and verbatim where it did not.
func (mkm *Client) get(ctx context.Context, link string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, http.NoBody)
	if err != nil {
		return err
	}

	resp, err := mkm.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}

	// The error body decodes as an object like any other, so it is read
	// first: unmarshalling it into the expected shape would succeed and
	// hand back an empty result.
	var apiErr APIError
	if json.Unmarshal(data, &apiErr) == nil && apiErr.Description != "" {
		return &apiErr
	}

	err = json.Unmarshal(data, out)
	if err != nil {
		return errors.New(string(data))
	}
	return nil
}

// Expansions returns every expansion of one game.
func (mkm *Client) Expansions(ctx context.Context, gameID int) ([]Expansion, error) {
	var response struct {
		Expansions []Expansion `json:"expansion"`
	}
	err := mkm.get(ctx, fmt.Sprintf(gameExpansionsURL, gameID), &response)
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
	err := mkm.get(ctx, productsBaseURL+fmt.Sprint(id), &response)
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
	err := mkm.get(ctx, expansionsBaseURL+fmt.Sprint(id)+"/singles", &response)
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

// Articles returns the listings on a product, one page at a time. Pages
// start at zero, and the API requires both bounds: asking for a page size
// without a start is refused.
func (mkm *Client) Articles(ctx context.Context, id int, options map[string]string, page, maxResults int) ([]Article, error) {
	u, err := url.Parse(articlesBaseURL + fmt.Sprint(id))
	if err != nil {
		return nil, err
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
	err = mkm.get(ctx, u.String(), &response)
	if err != nil {
		return nil, err
	}
	return response.Articles, nil
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

	for key, value := range req.URL.Query() {
		q.Set(key, value[0])
	}
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
