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

// Language is a language the marketplace trades in, as idLanguage numbers
// them.
type Language int

// The languages, in the order the documentation lists them.
const (
	LanguageEnglish Language = iota + 1
	LanguageFrench
	LanguageGerman
	LanguageSpanish
	LanguageItalian
	LanguageSimplifiedChinese
	LanguageJapanese
	LanguagePortuguese
	LanguageRussian
	LanguageKorean
	LanguageTraditionalChinese
)

var languageNames = map[Language]string{
	LanguageEnglish:            "English",
	LanguageFrench:             "French",
	LanguageGerman:             "German",
	LanguageSpanish:            "Spanish",
	LanguageItalian:            "Italian",
	LanguageSimplifiedChinese:  "Simplified Chinese",
	LanguageJapanese:           "Japanese",
	LanguagePortuguese:         "Portuguese",
	LanguageRussian:            "Russian",
	LanguageKorean:             "Korean",
	LanguageTraditionalChinese: "Traditional Chinese",
}

// LanguageName returns the language's name in English, as the API's own
// languageName field spells it, or "" for a number it carries no language
// for.
func LanguageName(language Language) string {
	return languageNames[language]
}

// LanguageFromName is the inverse, matching case-insensitively.
func LanguageFromName(name string) Language {
	for language, languageName := range languageNames {
		if strings.EqualFold(languageName, name) {
			return language
		}
	}
	return 0
}

// Country is a seller's country of origin, as sellerCountry numbers them.
type Country int

// The countries, in the order the marketplace numbers them. The blanks are
// numbers it answers for no country - the run is otherwise unbroken, and
// counting it keeps a country from being given a neighbour's number by a
// typo no reviewer would catch.
const (
	CountryAustria Country = iota + 1
	CountryBelgium
	CountryBulgaria
	CountrySwitzerland
	CountryCyprus
	CountryCzechRepublic
	CountryGermany
	CountryDenmark
	CountryEstonia
	CountrySpain
	CountryFinland
	CountryFrance
	CountryUnitedKingdom
	CountryGreece
	CountryHungary
	CountryIreland
	CountryItaly
	CountryLiechtenstein
	CountryLithuania
	CountryLuxembourg
	CountryLatvia
	CountryMalta
	CountryNetherlands
	CountryNorway
	CountryPoland
	CountryPortugal
	CountryRomania
	CountrySweden
	CountrySingapore
	CountrySlovenia
	CountrySlovakia
	_
	CountryCanada
	_
	CountryCroatia
	CountryJapan
	CountryIceland
)

var countryNames = map[Country]string{
	CountryAustria:       "Austria",
	CountryBelgium:       "Belgium",
	CountryBulgaria:      "Bulgaria",
	CountrySwitzerland:   "Switzerland",
	CountryCyprus:        "Cyprus",
	CountryCzechRepublic: "Czech Republic",
	CountryGermany:       "Germany",
	CountryDenmark:       "Denmark",
	CountryEstonia:       "Estonia",
	CountrySpain:         "Spain",
	CountryFinland:       "Finland",
	CountryFrance:        "France",
	CountryUnitedKingdom: "United Kingdom",
	CountryGreece:        "Greece",
	CountryHungary:       "Hungary",
	CountryIreland:       "Ireland",
	CountryItaly:         "Italy",
	CountryLiechtenstein: "Liechtenstein",
	CountryLithuania:     "Lithuania",
	CountryLuxembourg:    "Luxembourg",
	CountryLatvia:        "Latvia",
	CountryMalta:         "Malta",
	CountryNetherlands:   "Netherlands",
	CountryNorway:        "Norway",
	CountryPoland:        "Poland",
	CountryPortugal:      "Portugal",
	CountryRomania:       "Romania",
	CountrySweden:        "Sweden",
	CountrySingapore:     "Singapore",
	CountrySlovenia:      "Slovenia",
	CountrySlovakia:      "Slovakia",
	CountryCanada:        "Canada",
	CountryCroatia:       "Croatia",
	CountryJapan:         "Japan",
	CountryIceland:       "Iceland",
}

// CountryName returns the country's name in English, or "" for a number the
// marketplace carries no country for.
func CountryName(country Country) string {
	return countryNames[country]
}

// CountryFromName is the inverse, matching case-insensitively.
func CountryFromName(name string) Country {
	for country, countryName := range countryNames {
		if strings.EqualFold(countryName, name) {
			return country
		}
	}
	return 0
}

// Condition is a card's condition, which the API spells as a code rather
// than a number. The constants are declared best to worst, which is the
// order minCondition means by "or better".
type Condition string

// The conditions, best to worst.
const (
	ConditionMint        Condition = "MT"
	ConditionNearMint    Condition = "NM"
	ConditionExcellent   Condition = "EX"
	ConditionGood        Condition = "GD"
	ConditionLightPlayed Condition = "LP"
	ConditionPlayed      Condition = "PL"
	ConditionPoor        Condition = "PO"
)

// conditionNames spells each code out. The documentation writes Excellent
// "Exellent"; the name here is the word, not the typo.
var conditionNames = map[Condition]string{
	ConditionMint:        "Mint",
	ConditionNearMint:    "Near Mint",
	ConditionExcellent:   "Excellent",
	ConditionGood:        "Good",
	ConditionLightPlayed: "Light Played",
	ConditionPlayed:      "Played",
	ConditionPoor:        "Poor",
}

// ConditionName returns the condition spelled out, or "" for a code the
// marketplace does not use.
func ConditionName(condition Condition) string {
	return conditionNames[condition]
}

// ConditionFromName is the inverse, matching case-insensitively, and also
// accepting the code itself so a caller holding an Article's own condition
// string does not have to know which of the two it has.
func ConditionFromName(name string) Condition {
	for condition, conditionName := range conditionNames {
		if strings.EqualFold(conditionName, name) || strings.EqualFold(string(condition), name) {
			return condition
		}
	}
	return ""
}

// UserType is the kind of seller an article may be filtered to.
// Commercial includes powersellers; Powerseller is only those.
//
// Alone among these, it carries no Name and no FromName: its values are
// already the words the marketplace uses, so a table would map each one to
// itself. Note that an Article decodes the same idea as a number instead -
// ArticleSeller.IsCommercial, 0, 1 and 2 - which is the marketplace's
// inconsistency, not one worth hiding behind a shared type.
type UserType string

// The seller types an article may be filtered to.
const (
	UserTypePrivate     UserType = "private"
	UserTypeCommercial  UserType = "commercial"
	UserTypePowerseller UserType = "powerseller"
)

// UserScore is a seller's rating, as minUserScore numbers them. Like
// Condition, the constants run best to worst, which is the order
// minUserScore means by "or better".
type UserScore int

// The seller scores, best to worst.
const (
	UserScoreOutstanding UserScore = iota + 1
	UserScoreVeryGood
	UserScoreGood
	UserScoreAverage
	UserScoreBad
)

var userScoreNames = map[UserScore]string{
	UserScoreOutstanding: "Outstanding",
	UserScoreVeryGood:    "Very Good",
	UserScoreGood:        "Good",
	UserScoreAverage:     "Average",
	UserScoreBad:         "Bad",
}

// UserScoreName returns the score spelled out, or "" for a number the
// marketplace does not use.
func UserScoreName(score UserScore) string {
	return userScoreNames[score]
}

// UserScoreFromName is the inverse, matching case-insensitively.
func UserScoreFromName(name string) UserScore {
	for score, scoreName := range userScoreNames {
		if strings.EqualFold(scoreName, name) {
			return score
		}
	}
	return 0
}
