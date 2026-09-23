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

// The table is read in both directions by one map, so a game cannot be added
// to a name without reaching its lookup.
func TestGameNameRoundTrip(t *testing.T) {
	for game, name := range gameNames {
		for _, spelling := range []string{name, strings.ToLower(name), strings.ToUpper(name)} {
			if got := GameFromName(spelling); got != game {
				t.Errorf("GameFromName(%q) = %d, want %d", spelling, got, game)
			}
		}
	}
}

// An unknown name is not a game, and a zero is not a name. "" used to answer
// Magic, which made a missing game indistinguishable from the commonest one.
func TestUnknownGameNames(t *testing.T) {
	for _, name := range []string{"", "Magic: The Gathering", "Klingon"} {
		if got := GameFromName(name); got != 0 {
			t.Errorf("GameFromName(%q) = %d, want 0", name, got)
		}
	}
	if got := GameName(0); got != "" {
		t.Errorf("GameName(0) = %q, want empty", got)
	}
	if got := GameName(4); got != "" {
		t.Errorf("GameName(4) = %q, want empty - 4 is a gap in the run", got)
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
