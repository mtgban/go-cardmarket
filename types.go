package cardmarket

import "strings"

// The types Cardmarket enumerates. Each identifier is its own type rather
// than an int, because the API spends small integers freely - a game, a language, a
// country and a seller score are all "3" to a compiler that is told nothing
// else, and every one of them appears as a query parameter on the same
// request. Naming them costs nothing and makes the wrong one a compile
// error.
//
// The values are upstream's, from
// https://apiv2.cardmarket.com/ws/documentation, except where noted.

// Game is a game Cardmarket carries, as its API numbers them.
//
// The documentation does not publish this table - /games answers it at
// runtime - so these are read off the published catalog files, whose paths
// are keyed by exactly this number: products_singles_24.json shelves "Gundam
// Single". TestGameIDs pins them, since a game inserted in the wrong place
// would renumber every game after it and price one game from another's file.
type Game int

// The games, in the order the API numbers them. The blanks are numbers it
// no longer answers for - a game delisted since leaves its id spent.
const (
	GameMagic Game = iota + 1
	GameWorldOfWarcraft
	GameYuGiOh
	_
	GameTheSpoils
	GamePokemon
	GameForceOfWill
	GameCardfightVanguard
	GameFinalFantasy
	GameWeissSchwarz
	GameDragoborne
	GameMyLittlePony
	GameDragonBallSuper
	_
	GameStarWarsDestiny
	GameFleshAndBlood
	GameDigimon
	GameOnePiece
	GameLorcana
	GameBattleSpiritsSaga
	GameStarWarsUnlimited
	GameRiftbound
	GameCyberpunk
	GameGundam
)

// gameNames is the spelling each game takes in a storefront address, read
// off the site's own navigation rather than guessed: the abbreviated ones
// ("FoW", "WoW", "Vanguard", "Spoils") are not shortenings this package
// invented, and the full names are not interchangeable with them.
var gameNames = map[Game]string{
	GameMagic:             "Magic",
	GameWorldOfWarcraft:   "WoW",
	GameYuGiOh:            "YuGiOh",
	GameTheSpoils:         "Spoils",
	GamePokemon:           "Pokemon",
	GameForceOfWill:       "FoW",
	GameCardfightVanguard: "Vanguard",
	GameFinalFantasy:      "FinalFantasy",
	GameWeissSchwarz:      "WeissSchwarz",
	GameDragoborne:        "Dragoborne",
	GameMyLittlePony:      "MyLittlePony",
	GameDragonBallSuper:   "DragonBallSuper",
	GameStarWarsDestiny:   "StarWarsDestiny",
	GameFleshAndBlood:     "FleshAndBlood",
	GameDigimon:           "Digimon",
	GameOnePiece:          "OnePiece",
	GameLorcana:           "Lorcana",
	GameBattleSpiritsSaga: "BattleSpiritsSaga",
	GameStarWarsUnlimited: "StarWarsUnlimited",
	GameRiftbound:         "Riftbound",
	GameCyberpunk:         "Cyberpunk",
	GameGundam:            "Gundam",
}

// GameName returns the game as Cardmarket spells it in an address, or "" for
// a number it carries no game for.
//
// Every game the marketplace sells is named here, not only the ones a
// particular caller prices. Which games are worth reading is the caller's
// business; which games exist is Cardmarket's.
func GameName(game Game) string {
	return gameNames[game]
}

// GameFromName is the inverse, matching case-insensitively so a caller can
// hand over the name it already knows a game by ("lorcana") instead of
// translating to an id first. An unknown name answers 0, which the URL
// builders reject: a game Cardmarket does not carry yields no link at all
// rather than one pointing at a path it does not serve.
func GameFromName(name string) Game {
	for game, gameName := range gameNames {
		if strings.EqualFold(gameName, name) {
			return game
		}
	}
	return 0
}
