package cardmarket

import (
	"net/url"
	"strings"
	"testing"
)

func TestSearchURL(t *testing.T) {
	raw := SearchURL("Ariel - Singing Mermaid", GameLorcana, "mtgban")
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
	raw = SearchURL("Black Lotus", GameMagic, "")
	if strings.Contains(raw, "utm_") {
		t.Errorf("unaffiliated url carries tracking: %s", raw)
	}
	if !strings.Contains(raw, "/en/Magic/Products/Search") {
		t.Errorf("magic search url = %s", raw)
	}

	// Same contract as BuildURL for a number that names no game.
	if got := SearchURL("Chaos", 0, "mtgban"); got != "" {
		t.Errorf("unknown game = %q, want empty", got)
	}
}

// BuildURL keeps its behavior now that it shares the game and affiliate
// helpers with SearchURL.
func TestBuildURL(t *testing.T) {
	raw := BuildURL(12345, GameMagic, "mtgban", Finish{Foil: true})
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
	if got := BuildURL(1, 4, "", Finish{}); got != "" {
		t.Errorf("unknown game = %q, want empty", got)
	}

	// Every game the marketplace carries builds one now, including the
	// ones no caller here prices.
	for game := range gameNames {
		if got := BuildURL(1, game, "", Finish{}); got == "" {
			t.Errorf("game %d (%s) built no link", game, GameName(game))
		}
	}
	// No flags set omits all three rather than sending a falsy value.
	if raw := BuildURL(1, GameMagic, "", Finish{}); strings.Contains(raw, "isFoil") ||
		strings.Contains(raw, "isFirstEd") || strings.Contains(raw, "isReverseHolo") {
		t.Errorf("no-finish url should carry none of the three flags: %s", raw)
	}
	// Pokemon's two axes are independent and can both be set at once - a
	// 1st Edition Reverse Holo listing, confirmed to exist for the sets
	// where the two mechanics overlapped historically.
	q = mustQuery(t, BuildURL(1, GamePokemon, "", Finish{FirstEd: true, ReverseHolo: true}))
	if q.Get("isFirstEd") != "Y" || q.Get("isReverseHolo") != "Y" || q.Get("isFoil") != "" {
		t.Errorf("pokemon 1st-edition-reverse-holo query = %v", q)
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
