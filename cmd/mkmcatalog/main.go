// Command mkmcatalog walks one game's Cardmarket catalog - every expansion
// and the products it shelves - and writes it in the shape a price run reads,
// so the twice-daily runs stop crawling the API. MTGJSON publishes the Magic
// file; this tool builds every other game's.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/mtgban/simplecloud"

	"github.com/mtgban/go-cardmarket"
)

// walkTimeout bounds the whole crawl. The client is patient with the API's
// rate limiter by design, so without a deadline here a game whose expansions
// all rate-limit at once would run until the scheduler killed it.
const walkTimeout = 3 * time.Hour

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	gameName := flag.String("game", "", "game to walk (lorcana, riftbound, onepiece, pokemon, yugioh, fleshandblood)")
	output := flag.String("output", "", "file or b2:// object to write; an .xz suffix compresses it")
	flag.Parse()

	gameID := cardmarket.GameFromName(*gameName)
	if *gameName == "" || gameID == 0 {
		return fmt.Errorf("unknown game %q", *gameName)
	}
	if *output == "" {
		return errors.New("missing -output")
	}

	appToken := os.Getenv("MKM_APP_TOKEN")
	appSecret := os.Getenv("MKM_APP_SECRET")
	if appToken == "" || appSecret == "" {
		return errors.New("missing MKM_APP_TOKEN or MKM_APP_SECRET")
	}

	ctx, cancel := context.WithTimeout(context.Background(), walkTimeout)
	defer cancel()
	client := cardmarket.NewClient(appToken, appSecret)

	expansions, err := client.Expansions(ctx, gameID)
	if err != nil {
		return err
	}

	catalog := cardmarket.Catalog{}
	catalog.Meta.Date = time.Now().Format("2006-01-02")
	catalog.Data.Expansions = make(map[int]cardmarket.CatalogExpansion, len(expansions))
	catalog.Data.Products = map[int]cardmarket.CatalogProduct{}

	// Any expansion failing fails the run: the client already retries the
	// transient errors, and a partial catalog uploaded on schedule would
	// silently unprice whatever it dropped.
	for i, expansion := range expansions {
		products, err := client.ExpansionSingles(ctx, expansion.IDExpansion)
		if err != nil {
			return fmt.Errorf("expansion %d %q: %w", expansion.IDExpansion, expansion.Name, err)
		}
		catalog.Data.Expansions[expansion.IDExpansion] = cardmarket.CatalogExpansion{
			Name: expansion.Name,
			Code: expansion.SetCode,
		}
		for _, product := range products {
			catalog.Data.Products[product.IDProduct] = cardmarket.CatalogProduct{
				ExpansionID: expansion.IDExpansion,
				Name:        product.Name,
				Number:      product.Number,
				Rarity:      product.Rarity,
				Version:     cardmarket.ProductVersion(&product),
			}
		}
		log.Printf("[%d/%d] %s: %d products", i+1, len(expansions), expansion.Name, len(products))
	}
	log.Printf("Collected %d products in %d expansions with %d requests",
		len(catalog.Data.Products), len(catalog.Data.Expansions), client.RequestNo())

	writer, err := openWriter(ctx, *output)
	if err != nil {
		return err
	}
	err = json.NewEncoder(writer).Encode(&catalog)
	if err != nil {
		writer.Close()
		return err
	}
	// Close is where a buffered cloud writer commits the upload, so it
	// reports whether anything was durably written at all
	return writer.Close()
}

// openWriter writes the object at path the way the other dumpers' buckets do:
// a bare path lands on disk, a b2:// one in the bucket, and simplecloud
// compresses either by its suffix. The credentials are named as the b2
// command line names them, so the same pair works for the CLI and for this.
func openWriter(ctx context.Context, path string) (io.WriteCloser, error) {
	u, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	var bucket simplecloud.Writer
	switch u.Scheme {
	case "":
		bucket = &simplecloud.FileBucket{}
	case "b2":
		accessKey := os.Getenv("B2_APPLICATION_KEY_ID")
		secretKey := os.Getenv("B2_APPLICATION_KEY")
		if accessKey == "" || secretKey == "" {
			return nil, errors.New("missing B2_APPLICATION_KEY_ID or B2_APPLICATION_KEY env vars")
		}
		bucket, err = simplecloud.NewB2Client(ctx, accessKey, secretKey, u.Host)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported path scheme %s", u.Scheme)
	}

	return simplecloud.InitWriter(ctx, bucket, path)
}
