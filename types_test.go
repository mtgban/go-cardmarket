package cardmarket

import (
	"strings"
	"testing"
)

// The game ids are a bare iota run, so a game inserted in the wrong place
// renumbers every game after it - and nothing would say so, because the
// numbers only ever leave this package inside a download path. Pinning them
// turns that into a failing test rather than a game quietly priced from
// another game's catalog file. Each number here was read off the published
// catalog at downloads.s3.cardmarket.com: the file for 24 shelves "Gundam
// Single", 23's shelves "Cyberpunk Single".
//
// A table of pairs rather than a map: two constants that collide are the
// failure being guarded against, and a map literal would not compile.
func TestGameIDs(t *testing.T) {
	ids := []struct {
		name string
		got  Game
		want Game
	}{
		{"Magic", GameMagic, 1},
		{"WorldOfWarcraft", GameWorldOfWarcraft, 2},
		{"YuGiOh", GameYuGiOh, 3},
		{"TheSpoils", GameTheSpoils, 5},
		{"Pokemon", GamePokemon, 6},
		{"ForceOfWill", GameForceOfWill, 7},
		{"CardfightVanguard", GameCardfightVanguard, 8},
		{"FinalFantasy", GameFinalFantasy, 9},
		{"WeissSchwarz", GameWeissSchwarz, 10},
		{"Dragoborne", GameDragoborne, 11},
		{"MyLittlePony", GameMyLittlePony, 12},
		{"DragonBallSuper", GameDragonBallSuper, 13},
		{"StarWarsDestiny", GameStarWarsDestiny, 15},
		{"FleshAndBlood", GameFleshAndBlood, 16},
		{"Digimon", GameDigimon, 17},
		{"OnePiece", GameOnePiece, 18},
		{"Lorcana", GameLorcana, 19},
		{"BattleSpiritsSaga", GameBattleSpiritsSaga, 20},
		{"StarWarsUnlimited", GameStarWarsUnlimited, 21},
		{"Riftbound", GameRiftbound, 22},
		{"Cyberpunk", GameCyberpunk, 23},
		{"Gundam", GameGundam, 24},
	}
	for _, id := range ids {
		if id.got != id.want {
			t.Errorf("%s = %d, want %d", id.name, id.got, id.want)
		}
	}
	if len(gameNames) != len(ids) {
		t.Errorf("%d games named, %d pinned - a new game needs both", len(gameNames), len(ids))
	}
}

// The language and country numbers are upstream's own, published on the
// Articles resource. They are pinned for the same reason the games are: a
// wrong one filters to the wrong thing and answers a plausible page.
//
// Pairs rather than a map, like TestGameIDs and for the same reason. These
// runs have blanks in them, and a blank miscounted gives one entry its
// neighbour's number - which as a map key is "duplicate key 33 in map
// literal", naming neither of the two countries involved.
func TestDocumentedIDs(t *testing.T) {
	languages := []struct {
		name string
		got  Language
		want int
	}{
		{"English", LanguageEnglish, 1},
		{"French", LanguageFrench, 2},
		{"German", LanguageGerman, 3},
		{"Spanish", LanguageSpanish, 4},
		{"Italian", LanguageItalian, 5},
		{"Simplified Chinese", LanguageSimplifiedChinese, 6},
		{"Japanese", LanguageJapanese, 7},
		{"Portuguese", LanguagePortuguese, 8},
		{"Russian", LanguageRussian, 9},
		{"Korean", LanguageKorean, 10},
		{"Traditional Chinese", LanguageTraditionalChinese, 11},
	}
	for _, id := range languages {
		if int(id.got) != id.want {
			t.Errorf("%s = %d, want %d", id.name, id.got, id.want)
		}
	}
	if len(languageNames) != len(languages) {
		t.Errorf("%d languages named, %d pinned", len(languageNames), len(languages))
	}

	scores := []struct {
		name string
		got  UserScore
		want int
	}{
		{"Outstanding", UserScoreOutstanding, 1},
		{"Very Good", UserScoreVeryGood, 2},
		{"Good", UserScoreGood, 3},
		{"Average", UserScoreAverage, 4},
		{"Bad", UserScoreBad, 5},
	}
	for _, id := range scores {
		if int(id.got) != id.want {
			t.Errorf("%s = %d, want %d", id.name, id.got, id.want)
		}
	}
	if len(userScoreNames) != len(scores) {
		t.Errorf("%d scores named, %d pinned", len(userScoreNames), len(scores))
	}

	countries := []struct {
		name string
		got  Country
		want int
	}{
		{"Austria", CountryAustria, 1},
		{"Belgium", CountryBelgium, 2},
		{"Bulgaria", CountryBulgaria, 3},
		{"Switzerland", CountrySwitzerland, 4},
		{"Cyprus", CountryCyprus, 5},
		{"Czech Republic", CountryCzechRepublic, 6},
		{"Germany", CountryGermany, 7},
		{"Denmark", CountryDenmark, 8},
		{"Estonia", CountryEstonia, 9},
		{"Spain", CountrySpain, 10},
		{"Finland", CountryFinland, 11},
		{"France", CountryFrance, 12},
		{"United Kingdom", CountryUnitedKingdom, 13},
		{"Greece", CountryGreece, 14},
		{"Hungary", CountryHungary, 15},
		{"Ireland", CountryIreland, 16},
		{"Italy", CountryItaly, 17},
		{"Liechtenstein", CountryLiechtenstein, 18},
		{"Lithuania", CountryLithuania, 19},
		{"Luxembourg", CountryLuxembourg, 20},
		{"Latvia", CountryLatvia, 21},
		{"Malta", CountryMalta, 22},
		{"Netherlands", CountryNetherlands, 23},
		{"Norway", CountryNorway, 24},
		{"Poland", CountryPoland, 25},
		{"Portugal", CountryPortugal, 26},
		{"Romania", CountryRomania, 27},
		{"Sweden", CountrySweden, 28},
		// Singapore, Canada and Japan are on the marketplace's own
		// seller-country filter and absent from its documentation,
		// which is where the rest of these came from. The blanks at 32
		// and 34 are numbers it answers for no country.
		{"Singapore", CountrySingapore, 29},
		{"Slovenia", CountrySlovenia, 30},
		{"Slovakia", CountrySlovakia, 31},
		{"Canada", CountryCanada, 33},
		{"Croatia", CountryCroatia, 35},
		{"Japan", CountryJapan, 36},
		{"Iceland", CountryIceland, 37},
	}
	for _, id := range countries {
		if int(id.got) != id.want {
			t.Errorf("%s = %d, want %d", id.name, id.got, id.want)
		}
	}
	if len(countryNames) != len(countries) {
		t.Errorf("%d countries named, %d pinned", len(countryNames), len(countries))
	}
}

// Every table is read in both directions, so adding an entry cannot extend
// one and not the other - the property TestGameFromName used to check for
// games alone, now that there are four of them.
func TestNameRoundTrips(t *testing.T) {
	t.Run("game", func(t *testing.T) {
		for game, name := range gameNames {
			roundTrip(t, name, func(s string) bool { return GameFromName(s) == game })
		}
	})
	t.Run("language", func(t *testing.T) {
		for language, name := range languageNames {
			roundTrip(t, name, func(s string) bool { return LanguageFromName(s) == language })
		}
	})
	t.Run("country", func(t *testing.T) {
		for country, name := range countryNames {
			roundTrip(t, name, func(s string) bool { return CountryFromName(s) == country })
		}
	})
	t.Run("userscore", func(t *testing.T) {
		for score, name := range userScoreNames {
			roundTrip(t, name, func(s string) bool { return UserScoreFromName(s) == score })
		}
	})
	t.Run("condition", func(t *testing.T) {
		for condition, name := range conditionNames {
			roundTrip(t, name, func(s string) bool { return ConditionFromName(s) == condition })
		}
	})
}

// roundTrip checks a name maps back to its own id however the caller
// happens to have capitalised it.
func roundTrip(t *testing.T, name string, matches func(string) bool) {
	t.Helper()
	for _, spelling := range []string{name, strings.ToLower(name), strings.ToUpper(name)} {
		if !matches(spelling) {
			t.Errorf("%q did not map back to the entry it names", spelling)
		}
	}
}

// An unknown name is not a game, and a zero is not a name. Both directions
// answer empty rather than something plausible - the URL builders turn that
// into no link at all, which is the point.
func TestUnknownNames(t *testing.T) {
	for _, name := range []string{"", "Magic: The Gathering", "Klingon"} {
		if got := GameFromName(name); got != 0 {
			t.Errorf("GameFromName(%q) = %d, want 0", name, got)
		}
	}
	if got := LanguageFromName("Elvish"); got != 0 {
		t.Errorf("LanguageFromName(elvish) = %d, want 0", got)
	}
	if got := CountryFromName("Atlantis"); got != 0 {
		t.Errorf("CountryFromName(atlantis) = %d, want 0", got)
	}
	if got := UserScoreFromName("Sublime"); got != 0 {
		t.Errorf("UserScoreFromName(sublime) = %d, want 0", got)
	}
	if got := GameName(0); got != "" {
		t.Errorf("GameName(0) = %q, want empty", got)
	}
	if got := GameName(4); got != "" {
		t.Errorf("GameName(4) = %q, want empty - 4 is a gap in the run", got)
	}
	if got := LanguageName(99); got != "" {
		t.Errorf("LanguageName(99) = %q, want empty", got)
	}
	if got := CountryName(32); got != "" {
		t.Errorf("CountryName(32) = %q, want empty - 32 is a gap", got)
	}
}

// The abbreviated spellings are the marketplace's, not shortenings invented
// here, and a link built from the full name would 404.
func TestAbbreviatedGameNames(t *testing.T) {
	abbreviated := map[Game]string{
		GameForceOfWill:       "FoW",
		GameWorldOfWarcraft:   "WoW",
		GameCardfightVanguard: "Vanguard",
		GameTheSpoils:         "Spoils",
	}
	for game, want := range abbreviated {
		if got := GameName(game); got != want {
			t.Errorf("GameName(%d) = %q, want %q", game, got, want)
		}
	}
}

// A caller holding an Article has its condition as a code, not a name, and
// should not have to know which of the two it is looking at.
func TestConditionFromCode(t *testing.T) {
	for _, code := range []string{"NM", "nm", "Near Mint", "near mint"} {
		if got := ConditionFromName(code); got != ConditionNearMint {
			t.Errorf("ConditionFromName(%q) = %q, want NM", code, got)
		}
	}
	if got := ConditionFromName("Pristine"); got != "" {
		t.Errorf("ConditionFromName(pristine) = %q, want empty", got)
	}
	// The documentation spells this one "Exellent"; the name is the word.
	if got := ConditionName(ConditionExcellent); got != "Excellent" {
		t.Errorf("ConditionName(EX) = %q, want Excellent", got)
	}
}
