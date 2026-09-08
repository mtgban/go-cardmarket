package cardmarket

import (
	"net/url"
	"strings"
	"testing"
)

// Cardmarket has no game-agnostic product path, so a URL missing its game
// segment 404s rather than degrading to a general search.
func TestGameName(t *testing.T) {
	if got := GameName(GameMagic); got != "Magic" {
		t.Errorf("GameName(magic) = %q, want Magic", got)
	}
	if got := GameName(GameLorcana); got != "Lorcana" {
		t.Errorf("GameName(lorcana) = %q, want Lorcana", got)
	}
	if got := GameName(GamePokemon); got != "Pokemon" {
		t.Errorf("GameName(pokemon) = %q, want Pokemon", got)
	}
	// A game whose catalog is not covered has no spelling to use.
	if got := GameName(GameForceOfWill); got != "" {
		t.Errorf("GameName(forceofwill) = %q, want empty", got)
	}
	if got := GameName(0); got != "" {
		t.Errorf("GameName(0) = %q, want empty", got)
	}
}

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

	// Same contract as BuildURL for a game that is not covered.
	if got := SearchURL("Chaos", GameForceOfWill, "mtgban"); got != "" {
		t.Errorf("uncovered game = %q, want empty", got)
	}
}

// BuildURL keeps its behavior now that it shares the game and affiliate
// helpers with SearchURL.
func TestBuildURL(t *testing.T) {
	raw := BuildURL(12345, GameMagic, "mtgban", true)
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

	// A game the name table carries no entry for builds no link. Pokemon
	// stood here until it gained one.
	if got := BuildURL(1, GameForceOfWill, "", false); got != "" {
		t.Errorf("uncovered game = %q, want empty", got)
	}
	// Non-foil omits the flag rather than sending a falsy value.
	if strings.Contains(BuildURL(1, GameMagic, "", false), "isFoil") {
		t.Error("non-foil url should not carry isFoil")
	}
}

func parseChecked(t *testing.T, raw string) (*url.URL, error) {
	t.Helper()
	if raw == "" {
		t.Fatal("empty url")
	}
	return url.Parse(raw)
}

// GameFromName lets a caller pass the name it already knows a game by, so
// the two directions have to agree for every covered catalog - they read the
// same table precisely so adding a game cannot extend one and not the other.
func TestGameFromName(t *testing.T) {
	for idGame, name := range gameNames {
		if got := GameFromName(name); got != idGame {
			t.Errorf("GameFromName(%q) = %d, want %d", name, got, idGame)
		}
		// Callers spell games in their own case ("lorcana", "Magic").
		if got := GameFromName(strings.ToLower(name)); got != idGame {
			t.Errorf("GameFromName(%q) = %d, want %d", strings.ToLower(name), got, idGame)
		}
		if got := GameFromName(strings.ToUpper(name)); got != idGame {
			t.Errorf("GameFromName(%q) = %d, want %d", strings.ToUpper(name), got, idGame)
		}
	}

	// An unnamed game is the default one rather than an unknown one.
	if got := GameFromName(""); got != GameMagic {
		t.Errorf("GameFromName(\"\") = %d, want Magic (%d)", got, GameMagic)
	}

	// A game this package does not carry has no id, and the URL builders
	// turn that into no link rather than a wrong one. Pokemon used to stand
	// here and now carries an id of its own, so the example is a game the
	// table still names nothing for.
	for _, name := range []string{"digimon", "Magic: The Gathering"} {
		if got := GameFromName(name); got != 0 {
			t.Errorf("GameFromName(%q) = %d, want 0", name, got)
		}
	}
	if got := SearchURL("Agumon", GameFromName("digimon"), "mtgban"); got != "" {
		t.Errorf("uncovered game produced %q, want no link", got)
	}
	if got := BuildURL(1, GameFromName("digimon"), "mtgban", false); got != "" {
		t.Errorf("uncovered game produced %q, want no link", got)
	}
}
