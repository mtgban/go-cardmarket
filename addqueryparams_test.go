package cardmarket

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
)

// A repeated query parameter must sign with every value it carries, not
// just the first - the request sent on the wire keeps all of them
// regardless, so a signature computed over a truncated set would mismatch
// what Cardmarket's server actually received and fail with an opaque 401.
func TestAddQueryParamsKeepsEveryValueOfARepeatedKey(t *testing.T) {
	q := url.Values{"oauth_consumer_key": {"token"}}
	addQueryParams(q, url.Values{
		"sellerCountry": {"13", "4"},
		"start":         {"0"},
	})

	if got := q["sellerCountry"]; len(got) != 2 {
		t.Errorf("sellerCountry = %v, want both values kept", got)
	}
	if got := q.Get("start"); got != "0" {
		t.Errorf("start = %q, want \"0\"", got)
	}
	if got := q.Get("oauth_consumer_key"); got != "token" {
		t.Errorf("an existing param was clobbered: oauth_consumer_key = %q, want \"token\"", got)
	}
}

// A repeated key's values must be signed in numeric order, not the order
// they arrived in and not RFC 5849 §3.4.1.3.2's byte-value ordering.
// Confirmed live against the real API (varying only the wire order of a
// repeated sellerCountry value): signing "4" before "13" authenticated
// regardless of which order the wire carried them in, and signing "13"
// before "4" - byte-value order, since '1' < '4' - got a 401 the moment the
// wire order didn't already happen to agree with it.
func TestAddQueryParamsSortsRepeatedValuesNumerically(t *testing.T) {
	wireOrders := [][]string{
		{"13", "4"}, // descending numerically, ascending by byte value
		{"4", "13"}, // ascending both ways - would pass even by accident
	}
	for _, wire := range wireOrders {
		q := url.Values{}
		addQueryParams(q, url.Values{"sellerCountry": wire})
		if got := q["sellerCountry"]; !slices.Equal(got, []string{"4", "13"}) {
			t.Errorf("wire order %v: sellerCountry signed as %v, want numeric order [4 13] regardless of wire order", wire, got)
		}
	}
}

// A value that isn't a plain integer can't be numerically compared, so it
// falls back to byte-value sorting - the one thing every value sorts by
// unambiguously, and the same rule RFC 5849 itself specifies.
func TestAddQueryParamsFallsBackToByteValueForNonNumeric(t *testing.T) {
	q := url.Values{}
	addQueryParams(q, url.Values{"lang": {"fr", "de", "en"}})
	if got := q["lang"]; !slices.Equal(got, []string{"de", "en", "fr"}) {
		t.Errorf("lang = %v, want byte-value order [de en fr]", got)
	}
}

// A single-valued parameter behaves exactly as before - this pins that the
// fix for repeated keys didn't change the ordinary case.
func TestAddQueryParamsSingleValue(t *testing.T) {
	q := url.Values{}
	addQueryParams(q, url.Values{"idProduct": {"1234"}})

	if got := q.Get("idProduct"); got != "1234" {
		t.Errorf("idProduct = %q, want \"1234\"", got)
	}
}

// The signed order of a key's values has to come from the values alone: the
// server normalizes them before checking, so an order that follows the wire
// signs the same request two ways and one of them is a 401. Integers sort
// numerically, ahead of anything that is not one, and everything else - a
// tie like "01" and "1" included - by its encoded bytes.
func TestAddQueryParamsOrderIgnoresTheWire(t *testing.T) {
	tests := [][]string{
		{"9", "10", "1a"},
		{"01", "1"},
		{"-1", "01", "1", "a", "b"},
		// Encoded, "é" is "%C3%A9", which sorts before "z".
		{"é", "z"},
	}
	for _, want := range tests {
		for _, wire := range permutations(want) {
			q := url.Values{}
			addQueryParams(q, url.Values{"k": wire})
			if got := q["k"]; !slices.Equal(got, want) {
				t.Errorf("wire order %q: k signed as %q, want %q", wire, got, want)
			}
		}
	}
}

func permutations(values []string) [][]string {
	if len(values) <= 1 {
		return [][]string{slices.Clone(values)}
	}
	var out [][]string
	for i := range values {
		rest := slices.Concat(values[:i], values[i+1:])
		for _, tail := range permutations(rest) {
			out = append(out, append([]string{values[i]}, tail...))
		}
	}
	return out
}

// The signature itself, recomputed by hand over the one order Cardmarket was
// seen to accept for a repeated key, "4" before "13", for either wire order.
func TestSignatureOfARepeatedKey(t *testing.T) {
	for _, wire := range []string{"sellerCountry=13&sellerCountry=4", "sellerCountry=4&sellerCountry=13"} {
		parent := &headerTransport{}
		auth := &authTransport{
			Parent:            parent,
			AppToken:          "app token",
			AppSecret:         "app&secret",
			AccessToken:       "access",
			AccessTokenSecret: "access-secret",
		}
		const link = "https://apiv2.cardmarket.com/ws/v2.0/output.json/articles/1"
		req, err := http.NewRequest(http.MethodGet, link+"?"+wire, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := auth.RoundTrip(req); err != nil {
			t.Fatal(err)
		}

		header := parent.auth
		nonce := headerParam(t, header, "oauth_nonce")
		timestamp := headerParam(t, header, "oauth_timestamp")
		params := "oauth_consumer_key=app%20token" +
			"&oauth_nonce=" + url.QueryEscape(nonce) +
			"&oauth_signature_method=HMAC-SHA1" +
			"&oauth_timestamp=" + timestamp +
			"&oauth_token=access" +
			"&oauth_version=1.0" +
			"&sellerCountry=4&sellerCountry=13"
		base := "GET&" + url.QueryEscape(link) + "&" + url.QueryEscape(params)
		mac := hmac.New(sha1.New, []byte("app%26secret&access-secret"))
		mac.Write([]byte(base))
		want := base64.StdEncoding.EncodeToString(mac.Sum(nil))

		if got := headerParam(t, header, "oauth_signature"); got != want {
			t.Errorf("wire %q: oauth_signature = %q, want %q", wire, got, want)
		}
	}
}

// headerTransport keeps the Authorization header of the request it answers.
type headerTransport struct{ auth string }

func (h *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	h.auth = req.Header.Get("Authorization")
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Request: req}, nil
}

func headerParam(t *testing.T, header, name string) string {
	t.Helper()
	_, rest, found := strings.Cut(header, name+"=\"")
	if !found {
		t.Fatalf("no %s in %q", name, header)
	}
	value, _, _ := strings.Cut(rest, "\"")
	return value
}
