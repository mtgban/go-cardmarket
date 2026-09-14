package cardmarket

import (
	"net/url"
	"slices"
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
