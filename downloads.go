package cardmarket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/go-cleanhttp"
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
func (pg PriceGuide) SecondPrinting(game Game) (low, trend float64) {
	if game == GamePokemon {
		return pg.HoloLowPrice, pg.HoloTrendPrice
	}
	return pg.FoilLowPrice, pg.FoilTrendPrice
}

// getDownload fetches one published file, refusing a response that is not
// one. A game whose catalog the site has not published yet is not a 404 with
// a JSON body but a 403 with an XML one, which a decoder reports as "invalid
// character '<' looking for beginning of value" - a parser bug by the look of
// it, rather than the absent file it actually is. Gundam's price guide
// answered exactly that for the weeks after the game appeared.
func getDownload(ctx context.Context, link string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := downloadClient().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("cardmarket: %d %s: %s",
			resp.StatusCode, http.StatusText(resp.StatusCode), link)
	}

	return resp, nil
}

// DownloadPriceGuide downloads the published price guide for one game.
func DownloadPriceGuide(ctx context.Context, game Game) ([]PriceGuide, error) {
	resp, err := getDownload(ctx, fmt.Sprintf(priceGuideURL, game))
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
func DownloadProductListSingles(ctx context.Context, game Game) ([]ProductList, error) {
	return downloadProductList(ctx, fmt.Sprintf(productListSinglesURL, game))
}

// DownloadProductListSealed downloads the catalog of one game's sealed
// product.
func DownloadProductListSealed(ctx context.Context, game Game) ([]ProductList, error) {
	return downloadProductList(ctx, fmt.Sprintf(productListSealedURL, game))
}

func downloadProductList(ctx context.Context, link string) ([]ProductList, error) {
	resp, err := getDownload(ctx, link)
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
