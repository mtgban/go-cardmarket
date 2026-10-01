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

// createdAtLayout is how the published files date themselves -
// "2026-09-30T09:55:34+0200" - whose offset has no colon, so it is not
// RFC 3339 and time.Time will not decode it on its own.
const createdAtLayout = "2006-01-02T15:04:05-0700"

// publishedFile is the envelope every published file comes in. A price guide
// fills PriceGuides and a product list Products.
type publishedFile struct {
	Version     int           `json:"version"`
	CreatedAt   string        `json:"createdAt"`
	PriceGuides []PriceGuide  `json:"priceGuides"`
	Products    []ProductList `json:"products"`
}

func downloadFile(ctx context.Context, link string) (*publishedFile, error) {
	resp, err := getDownload(ctx, link)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var file publishedFile
	err = json.NewDecoder(resp.Body).Decode(&file)
	if err != nil {
		return nil, err
	}
	return &file, nil
}

// createdAt reads the time the file was built. An unreadable one is an
// error, not a zero time, since a caller asking for it means to judge it.
func (f *publishedFile) createdAt() (time.Time, error) {
	createdAt, err := time.Parse(createdAtLayout, f.CreatedAt)
	if err != nil {
		return time.Time{}, fmt.Errorf("cardmarket: published file createdAt: %w", err)
	}
	return createdAt, nil
}

// PriceGuideFile is a published price guide: when Cardmarket built it, and
// its rows.
type PriceGuideFile struct {
	Version     int
	CreatedAt   time.Time
	PriceGuides []PriceGuide
}

// DownloadPriceGuide downloads the published price guide for one game.
func DownloadPriceGuide(ctx context.Context, game Game) ([]PriceGuide, error) {
	file, err := downloadFile(ctx, fmt.Sprintf(priceGuideURL, game))
	if err != nil {
		return nil, err
	}
	return file.PriceGuides, nil
}

// DownloadPriceGuideFile downloads one game's price guide with the time
// Cardmarket built it, so a caller can refuse a file that has stopped
// updating. How old is too old is the caller's to say: each file is rebuilt
// on its own schedule.
func DownloadPriceGuideFile(ctx context.Context, game Game) (*PriceGuideFile, error) {
	return priceGuideFile(ctx, fmt.Sprintf(priceGuideURL, game))
}

func priceGuideFile(ctx context.Context, link string) (*PriceGuideFile, error) {
	file, err := downloadFile(ctx, link)
	if err != nil {
		return nil, err
	}
	createdAt, err := file.createdAt()
	if err != nil {
		return nil, err
	}
	return &PriceGuideFile{
		Version:     file.Version,
		CreatedAt:   createdAt,
		PriceGuides: file.PriceGuides,
	}, nil
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

// ProductListFile is a published product list: when Cardmarket built it,
// and its rows.
type ProductListFile struct {
	Version   int
	CreatedAt time.Time
	Products  []ProductList
}

// DownloadProductListSingles downloads the catalog of one game's singles.
func DownloadProductListSingles(ctx context.Context, game Game) ([]ProductList, error) {
	file, err := downloadFile(ctx, fmt.Sprintf(productListSinglesURL, game))
	if err != nil {
		return nil, err
	}
	return file.Products, nil
}

// DownloadProductListSealed downloads the catalog of one game's sealed
// product.
func DownloadProductListSealed(ctx context.Context, game Game) ([]ProductList, error) {
	file, err := downloadFile(ctx, fmt.Sprintf(productListSealedURL, game))
	if err != nil {
		return nil, err
	}
	return file.Products, nil
}

// DownloadProductListSinglesFile is DownloadProductListSingles with the time
// Cardmarket built the file; see DownloadPriceGuideFile.
func DownloadProductListSinglesFile(ctx context.Context, game Game) (*ProductListFile, error) {
	return productListFile(ctx, fmt.Sprintf(productListSinglesURL, game))
}

// DownloadProductListSealedFile is DownloadProductListSealed with the time
// Cardmarket built the file; see DownloadPriceGuideFile.
func DownloadProductListSealedFile(ctx context.Context, game Game) (*ProductListFile, error) {
	return productListFile(ctx, fmt.Sprintf(productListSealedURL, game))
}

func productListFile(ctx context.Context, link string) (*ProductListFile, error) {
	file, err := downloadFile(ctx, link)
	if err != nil {
		return nil, err
	}
	createdAt, err := file.createdAt()
	if err != nil {
		return nil, err
	}
	return &ProductListFile{
		Version:   file.Version,
		CreatedAt: createdAt,
		Products:  file.Products,
	}, nil
}
