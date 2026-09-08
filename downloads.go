package cardmarket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/go-cleanhttp"
)

// The games Cardmarket carries, as their API numbers them.
const (
	GameMagic = iota + 1
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
)

// The published catalog files, which the site serves to anyone: no app
// credentials, no request allowance, and a whole game per request.
const (
	priceGuideURL         = "https://downloads.s3.cardmarket.com/productCatalog/priceGuide/price_guide_%d.json"
	productListSinglesURL = "https://downloads.s3.cardmarket.com/productCatalog/productList/products_singles_%d.json"
	productListSealedURL  = "https://downloads.s3.cardmarket.com/productCatalog/productList/products_nonsingles_%d.json"

	// downloadTimeout bounds a whole file. These run to tens of megabytes
	// for the larger games, so it is generous; it is here to stop a stalled
	// connection holding a scheduled job open, not to pace a slow one.
	downloadTimeout = 5 * time.Minute
)

// downloadClient is the plain client the published files are read with. They
// are unsigned static objects, so they need neither the API's credentials nor
// its patience with rate limits.
func downloadClient() *http.Client {
	client := cleanhttp.DefaultClient()
	client.Timeout = downloadTimeout
	return client
}

// PriceGuide is one product's published prices: the low, the trend, and the
// averages Cardmarket derives rather than any single listing.
type PriceGuide struct {
	IDProduct        int     `json:"idProduct"`
	AvgSellPrice     float64 `json:"avg"`
	LowPrice         float64 `json:"low"`
	TrendPrice       float64 `json:"trend"`
	FoilAvgSellPrice float64 `json:"avg-foil"`
	FoilLowPrice     float64 `json:"low-foil"`
	FoilTrendPrice   float64 `json:"trend-foil"`
	// Pokemon's guide names the second printing's prices "holo" rather
	// than "foil", and publishes them for the cards sold in a reverse
	// holo and for no others; see SecondPrinting.
	HoloAvgSellPrice float64 `json:"avg-holo"`
	HoloLowPrice     float64 `json:"low-holo"`
	HoloTrendPrice   float64 `json:"trend-holo"`
	AvgDay1          float64 `json:"avg1"`
	AvgDay7          float64 `json:"avg7"`
	AvgDay30         float64 `json:"avg30"`
	FoilAvgDay1      float64 `json:"avg1-foil"`
	FoilAvgDay7      float64 `json:"avg7-foil"`
	FoilAvgDay30     float64 `json:"avg30-foil"`
}

// SecondPrinting names the prices of the printing sold beside the product's
// default one, under whichever heading the game's guide publishes them. Most
// games sell a foil beside a plain card; Pokemon sells a reverse holo, and
// its guide says so.
func (pg PriceGuide) SecondPrinting(gameID int) (low, trend float64) {
	if gameID == GamePokemon {
		return pg.HoloLowPrice, pg.HoloTrendPrice
	}
	return pg.FoilLowPrice, pg.FoilTrendPrice
}

// DownloadPriceGuide downloads the published price guide for one game.
func DownloadPriceGuide(ctx context.Context, gameID int) ([]PriceGuide, error) {
	link := fmt.Sprintf(priceGuideURL, gameID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := downloadClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response struct {
		Version     int          `json:"version"`
		CreatedAt   string       `json:"createdAt"`
		PriceGuides []PriceGuide `json:"priceGuides"`
	}
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	return response.PriceGuides, nil
}

// ProductList is one entry of the catalog dump, which names products without
// pricing them.
type ProductList struct {
	IDProduct    int    `json:"idProduct"`
	Name         string `json:"name"`
	CategoryID   int    `json:"idCategory"`
	CategoryName string `json:"categoryName"`
	ExpansionID  int    `json:"idExpansion"`
	MetacardID   int    `json:"idMetacard"`
	DateAdded    string `json:"dateAdded"`
}

// DownloadProductListSingles downloads the catalog of one game's singles.
func DownloadProductListSingles(ctx context.Context, gameID int) ([]ProductList, error) {
	return downloadProductList(ctx, fmt.Sprintf(productListSinglesURL, gameID))
}

// DownloadProductListSealed downloads the catalog of one game's sealed
// product.
func DownloadProductListSealed(ctx context.Context, gameID int) ([]ProductList, error) {
	return downloadProductList(ctx, fmt.Sprintf(productListSealedURL, gameID))
}

func downloadProductList(ctx context.Context, link string) ([]ProductList, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := downloadClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response struct {
		Version   int           `json:"version"`
		CreatedAt string        `json:"createdAt"`
		Products  []ProductList `json:"products"`
	}
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	return response.Products, nil
}

var gameNames = map[int]string{
	GameMagic:         "Magic",
	GameLorcana:       "Lorcana",
	GameRiftbound:     "Riftbound",
	GameOnePiece:      "OnePiece",
	GameYuGiOh:        "YuGiOh",
	GameFleshAndBlood: "FleshAndBlood",
	GamePokemon:       "Pokemon",
}

// GameName returns the game as Cardmarket spells it, or "" for a game whose
// catalog is not covered.
func GameName(idGame int) string {
	return gameNames[idGame]
}

// GameFromName is the inverse, matching case-insensitively so a caller can
// hand over the name it already knows a game by ("lorcana") instead of
// translating to an id first; an unnamed game is Magic. Unknown games answer
// 0, which the URL builders reject: a game Cardmarket does not carry yields no
// link at all rather than one pointing at a path it does not serve.
func GameFromName(name string) int {
	if name == "" {
		return GameMagic
	}
	for idGame, gameName := range gameNames {
		if strings.EqualFold(gameName, name) {
			return idGame
		}
	}
	return 0
}

// SearchURL returns the catalog search for a product name, the fallback for a
// card whose Cardmarket product id is not known. Empty for an uncovered game,
// like BuildURL.
func SearchURL(name string, idGame int, affiliate string) string {
	game := GameName(idGame)
	if game == "" {
		return ""
	}

	u, err := url.Parse(fmt.Sprintf("https://www.cardmarket.com/en/%s/Products/Search", game))
	if err != nil {
		return ""
	}

	v := url.Values{}
	v.Set("searchString", name)
	setAffiliate(v, affiliate)

	u.RawQuery = v.Encode()
	return u.String()
}

func setAffiliate(v url.Values, affiliate string) {
	if affiliate == "" {
		return
	}
	v.Set("utm_source", affiliate)
	v.Set("utm_medium", "text")
	v.Set("utm_campaign", "card_prices")
}

// BuildURL builds the storefront link for a product, carrying an affiliate tag
// when one is given.
func BuildURL(idProduct, idGame int, affiliate string, foil bool) string {
	game := GameName(idGame)
	if game == "" {
		return ""
	}

	u, err := url.Parse(fmt.Sprintf("https://www.cardmarket.com/en/%s/Products", game))
	if err != nil {
		return ""
	}

	v := url.Values{}

	v.Set("idProduct", fmt.Sprint(idProduct))

	// Set English as preferred language, it switches to the default one
	// automatically in case the card has is non-English only
	v.Set("language", "1")

	if foil {
		v.Set("isFoil", "Y")
	}

	setAffiliate(v, affiliate)

	u.RawQuery = v.Encode()
	return u.String()
}
