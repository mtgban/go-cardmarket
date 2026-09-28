package cardmarket

import (
	"net/url"
	"strings"
	"testing"
)

func TestSearchURL(t *testing.T) {
	raw := SearchURL(GameLorcana, "Ariel - Singing Mermaid", URLOption{Affiliate: "mtgban"})
	u, err := parseChecked(t, raw)
	if err != nil {
		t.Fatal(err)
	}
	if u.Path != "/en/Lorcana/Products/Search" {
		t.Errorf("path = %q, want /en/Lorcana/Products/Search", u.Path)
	}
	if got := u.Query().Get("searchString"); got != "Ariel - Singing Mermaid" {
		t.Errorf("searchString = %q", got)
	}
	if got := u.Query().Get("utm_source"); got != "mtgban" {
		t.Errorf("utm_source = %q, want mtgban", got)
	}

	// No affiliate configured leaves the tracking parameters off entirely.
	raw = SearchURL(GameMagic, "Black Lotus", URLOption{})
	if strings.Contains(raw, "utm_") {
		t.Errorf("unaffiliated url carries tracking: %s", raw)
	}
	if !strings.Contains(raw, "/en/Magic/Products/Search") {
		t.Errorf("magic search url = %s", raw)
	}

	// Same contract as BuildURL for a number that names no game.
	if got := SearchURL(0, "Chaos", URLOption{Affiliate: "mtgban"}); got != "" {
		t.Errorf("unknown game = %q, want empty", got)
	}
}

func TestBuildURL(t *testing.T) {
	raw := BuildURL(GameMagic, 12345, URLOption{Foil: Only, Language: LanguageEnglish, Affiliate: "mtgban"})
	u, err := parseChecked(t, raw)
	if err != nil {
		t.Fatal(err)
	}
	if u.Path != "/en/Magic/Products" {
		t.Errorf("path = %q, want /en/Magic/Products", u.Path)
	}
	q := u.Query()
	if q.Get("idProduct") != "12345" || q.Get("language") != "1" || q.Get("isFoil") != "Y" {
		t.Errorf("query = %v", q)
	}
	if q.Get("utm_source") != "mtgban" || q.Get("utm_medium") != "text" || q.Get("utm_campaign") != "card_prices" {
		t.Errorf("affiliate params = %v", q)
	}

	// A number the name table carries no entry for builds no link.
	if got := BuildURL(4, 1, URLOption{}); got != "" {
		t.Errorf("unknown game = %q, want empty", got)
	}

	// Every game the marketplace carries builds one, including the ones
	// no caller here prices - which games exist is not this package's
	// opinion to hold.
	for game := range gameNames {
		if got := BuildURL(game, 1, URLOption{}); got == "" {
			t.Errorf("game %d (%s) built no link", game, GameName(game))
		}
	}
}

// The zero option asks for no narrowing at all. It used to send language=1
// whether or not anyone wanted it; now a caller that wants English says so.
func TestBuildURLZeroOption(t *testing.T) {
	raw := BuildURL(GameMagic, 1, URLOption{})
	q := mustQuery(t, raw)
	for _, param := range []string{
		"isFoil", "isFirstEd", "isReverseHolo", "isSigned", "isAltered",
		"sellerType", "sellerCountry", "language",
	} {
		if got := q.Get(param); got != "" {
			t.Errorf("zero option sent %s=%q", param, got)
		}
	}
	if q.Get("idProduct") != "1" {
		t.Errorf("idProduct = %q, want 1", q.Get("idProduct"))
	}
	if strings.Contains(raw, "utm_") {
		t.Errorf("zero option carries tracking: %s", raw)
	}
}

// Pokemon's two axes are independent and can both be set at once - a 1st
// Edition Reverse Holo listing, confirmed to exist for the sets where the
// two mechanics overlapped historically.
func TestBuildURLFinishFlags(t *testing.T) {
	q := mustQuery(t, BuildURL(GamePokemon, 1, URLOption{FirstEd: Only, ReverseHolo: Only}))
	if q.Get("isFirstEd") != "Y" || q.Get("isReverseHolo") != "Y" || q.Get("isFoil") != "" {
		t.Errorf("pokemon 1st-edition-reverse-holo query = %v", q)
	}
}

// Signed and altered are bare parameters, like isFoil.
func TestBuildURLExclusions(t *testing.T) {
	raw := BuildURL(GameMagic, 1, URLOption{Signed: None, Altered: None})
	q := mustQuery(t, raw)
	if q.Get("isSigned") != "N" || q.Get("isAltered") != "N" {
		t.Errorf("exclusions = %v, want isSigned=N and isAltered=N", q)
	}
	if strings.Contains(raw, "extra") {
		t.Errorf("exclusions sent nested: %s", raw)
	}

	// Asked for separately, only the one asked for is sent.
	q = mustQuery(t, BuildURL(GameMagic, 1, URLOption{Signed: None}))
	if q.Get("isSigned") != "N" || q.Get("isAltered") != "" {
		t.Errorf("signed-only exclusion sent %v", q)
	}
}

// Village Bell-Ringer's foil in English, neither signed nor altered, then
// the same narrowed to German and Dutch powersellers. Compared as whole
// strings: a parsed query reads back a spelling the storefront ignores as
// happily as one it reads.
func TestBuildURLVillageBellRinger(t *testing.T) {
	const tag = "&utm_campaign=card_prices&utm_medium=text&utm_source=mtgban"
	opt := URLOption{
		Foil:      Only,
		Signed:    None,
		Altered:   None,
		Language:  LanguageEnglish,
		Affiliate: "mtgban",
	}
	want := "https://www.cardmarket.com/en/Magic/Products?idProduct=250689" +
		"&isAltered=N&isFoil=Y&isSigned=N&language=1" + tag
	if got := BuildURL(GameMagic, 250689, opt); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}

	opt.SellerTypes = []UserType{UserTypePowerseller}
	opt.SellerCountries = []Country{CountryNetherlands, CountryGermany}
	want = "https://www.cardmarket.com/en/Magic/Products?idProduct=250689" +
		"&isAltered=N&isFoil=Y&isSigned=N&language=1" +
		"&sellerCountry=7%2C23&sellerType=2" + tag
	if got := BuildURL(GameMagic, 250689, opt); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// A list is one parameter, its values joined by commas in ascending order
// whatever order they were named in.
func TestBuildURLSellers(t *testing.T) {
	tests := []struct {
		name  string
		opt   URLOption
		param string
		want  string
	}{
		{"powersellers", URLOption{SellerTypes: []UserType{UserTypePowerseller}},
			"sellerType", "2"},
		{"every type", URLOption{SellerTypes: []UserType{UserTypePowerseller, UserTypePrivate, UserTypeCommercial}},
			"sellerType", "0,1,2"},
		{"private alone is not no filter", URLOption{SellerTypes: []UserType{UserTypePrivate}},
			"sellerType", "0"},
		{"a type with no number is left out", URLOption{SellerTypes: []UserType{"wholesale"}},
			"sellerType", ""},
		{"one country", URLOption{SellerCountries: []Country{CountryGermany}},
			"sellerCountry", "7"},
		{"two countries, named backwards", URLOption{SellerCountries: []Country{CountryNetherlands, CountryGermany}},
			"sellerCountry", "7,23"},
		{"a country named twice", URLOption{SellerCountries: []Country{CountryJapan, CountryGermany, CountryJapan}},
			"sellerCountry", "7,36"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := mustQuery(t, BuildURL(GameMagic, 1, tt.opt))
			if got := q.Get(tt.param); got != tt.want {
				t.Errorf("%s = %q, want %q", tt.param, got, tt.want)
			}
		})
	}

	// Sorting the list is the builder's business, not the caller's slice.
	countries := []Country{CountryNetherlands, CountryGermany}
	BuildURL(GameMagic, 1, URLOption{SellerCountries: countries})
	if countries[0] != CountryNetherlands {
		t.Errorf("caller's countries reordered to %v", countries)
	}
}

// Whatever the option narrows, it narrows both builders. They read the same
// values precisely so a filter added to one cannot quietly skip the other.
func TestSearchURLTakesTheSameOption(t *testing.T) {
	opt := URLOption{
		Foil:            Only,
		Signed:          None,
		Altered:         None,
		SellerTypes:     []UserType{UserTypeCommercial, UserTypePowerseller},
		SellerCountries: []Country{CountryJapan},
		Language:        LanguageJapanese,
	}
	search := mustQuery(t, SearchURL(GamePokemon, "Pikachu", opt))
	build := mustQuery(t, BuildURL(GamePokemon, 1, opt))

	for _, param := range []string{
		"isFoil", "isSigned", "isAltered", "sellerType", "sellerCountry", "language",
	} {
		if build.Get(param) == "" {
			t.Errorf("%s: not sent", param)
		}
		if search.Get(param) != build.Get(param) {
			t.Errorf("%s: search %q, build %q", param, search.Get(param), build.Get(param))
		}
	}
	if build.Get("language") != "7" {
		t.Errorf("language = %q, want 7 (Japanese)", build.Get("language"))
	}
}

func mustQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	u, err := parseChecked(t, raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query()
}

func parseChecked(t *testing.T, raw string) (*url.URL, error) {
	t.Helper()
	if raw == "" {
		t.Fatal("empty url")
	}
	return url.Parse(raw)
}

// Every flag is three-valued, because the marketplace's are: Any says
// nothing, Only asks for the listings that carry it, None for the ones that
// do not. None matters as much as Only on a finish - a non-foil price
// linking to a page full of foils is quoting from a shelf the reader cannot
// see. isFoil=Y and isFoil=N were confirmed to partition a real product's
// listings, 50 rows each with nothing in common.
func TestBuildURLFilterStates(t *testing.T) {
	tests := []struct {
		name string
		opt  URLOption
		want map[string]string
	}{
		{"any sends nothing", URLOption{}, map[string]string{"isFoil": "", "isSigned": ""}},
		{"only foil", URLOption{Foil: Only}, map[string]string{"isFoil": "Y"}},
		{"non-foil only", URLOption{Foil: None}, map[string]string{"isFoil": "N"}},
		{"only signed", URLOption{Signed: Only}, map[string]string{"isSigned": "Y"}},
		{"no signed", URLOption{Signed: None}, map[string]string{"isSigned": "N"}},
		{"no altered, any foil", URLOption{Altered: None},
			map[string]string{"isAltered": "N", "isFoil": ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := mustQuery(t, BuildURL(GameMagic, 1, tt.opt))
			for param, want := range tt.want {
				if got := q.Get(param); got != want {
					t.Errorf("%s = %q, want %q", param, got, want)
				}
			}
		})
	}
}

// Any is the zero value, so a caller that says nothing narrows nothing.
func TestFilterZeroValueIsAny(t *testing.T) {
	var f Filter
	if f != Any {
		t.Errorf("zero Filter = %d, want Any (%d)", f, Any)
	}
}

// The affiliate tag moved into the option because SearchURL took it beside
// the card's name, and two adjacent strings are two strings in the wrong
// order sooner or later - a search for the affiliate, tagged with the card.
// Named at the call site it cannot be transposed, and it reaches both
// builders the same way.
func TestAffiliateOnBothBuilders(t *testing.T) {
	opt := URLOption{Affiliate: "mtgban"}
	for name, raw := range map[string]string{
		"build":  BuildURL(GameMagic, 1, opt),
		"search": SearchURL(GameMagic, "Black Lotus", opt),
	} {
		q := mustQuery(t, raw)
		if q.Get("utm_source") != "mtgban" || q.Get("utm_medium") != "text" ||
			q.Get("utm_campaign") != "card_prices" {
			t.Errorf("%s: affiliate params = %v", name, q)
		}
		// The tag must not be mistaken for what is being looked up.
		if q.Get("searchString") == "mtgban" || q.Get("idProduct") == "mtgban" {
			t.Errorf("%s: affiliate leaked into the subject: %v", name, q)
		}
	}
}
