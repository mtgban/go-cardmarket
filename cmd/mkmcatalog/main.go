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
	"slices"
	"strings"
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
	gameName := flag.String("game", "", "game to walk (lorcana, riftbound, gundam, onepiece, pokemon, yugioh, fleshandblood)")
	output := flag.String("output", "", "file or b2:// object to write; an .xz suffix compresses it")
	previous := flag.String("previous", "", "catalog to carry an unanswerable expansion's products over from; a file or b2:// object, read only if the walk leaves one")
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

	failed := walk(ctx, client, expansions, &catalog)

	// One more pass before giving up on them. The 5xx that fails an
	// expansion is the API unwell on that one request rather than a
	// property of the shelf - the same expansion walks fine most days -
	// and by the end of a long walk the moment has usually passed.
	if len(failed) > 0 {
		log.Printf("Retrying %d expansion(s) the API would not answer for", len(failed))
		again := make([]cardmarket.Expansion, len(failed))
		for i, one := range failed {
			again[i] = one.expansion
		}
		failed = walk(ctx, client, again, &catalog)
	}

	if len(failed) > 0 {
		// Carrying over is for a shelf the API would not answer for. A
		// walk cut short by its own deadline or a cancelled context has
		// not been refused anything - the expansions it never reached
		// would carry over on a technicality, and a late deadline could
		// republish up to a tenth of the game from yesterday without
		// ever saying the walk did not finish.
		if ctx.Err() != nil {
			return fmt.Errorf("walk did not finish (%d of %d expansions read): %w",
				len(catalog.Data.Expansions), len(expansions), ctx.Err())
		}
		if limit := carryLimit(len(expansions)); len(failed) > limit {
			return fmt.Errorf("%d of %d expansions went unanswered, more than the %d a carry-over covers",
				len(failed), len(expansions), limit)
		}
		err = carryOver(ctx, *previous, failed, &catalog)
		if err != nil {
			return err
		}
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

// unanswered is one expansion the walk could not read, and why. The reason
// travels with it because it is the only thing that says whether the shelf
// is worth carrying over or the walk itself is in trouble, and it is gone by
// the time anyone reads the catalog.
type unanswered struct {
	expansion cardmarket.Expansion
	err       error
}

// describe names unanswered expansions for an error a person has to act on,
// which means saying what went wrong with each and not only how many did.
func describe(failed []unanswered) string {
	lines := make([]string, 0, len(failed))
	for _, one := range failed {
		lines = append(lines, fmt.Sprintf("%d %q (%v)",
			one.expansion.IDExpansion, one.expansion.Name, one.err))
	}
	return strings.Join(lines, "; ")
}

// walk reads each expansion's singles into the catalog and returns the ones
// the API would not answer for.
//
// A failure no longer ends the walk. It used to, on the reasoning that a
// partial catalog uploaded on schedule would silently unprice whatever it
// dropped - which is right, and is why carryOver exists rather than why the
// other 781 expansions should be thrown away with the one. Two shelves
// failing this way (1645 "Pokemon Products", 5303 "Unnumbered Promos") cost
// two whole days of two games' catalogs.
func walk(ctx context.Context, client *cardmarket.Client,
	expansions []cardmarket.Expansion, catalog *cardmarket.Catalog) []unanswered {
	var failed []unanswered

	for i, expansion := range expansions {
		// A cancelled context fails every remaining expansion instantly,
		// which would otherwise walk the rest of the game just to mark
		// it all unanswered. run refuses to carry any of it over.
		if ctx.Err() != nil {
			log.Printf("Stopping the walk at %d/%d: %v", i+1, len(expansions), ctx.Err())
			break
		}

		products, err := client.ExpansionSingles(ctx, expansion.IDExpansion)
		if err != nil {
			log.Printf("[%d/%d] %s: %v", i+1, len(expansions), expansion.Name, err)
			failed = append(failed, unanswered{expansion: expansion, err: err})
			continue
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

	return failed
}

// carryLimit is how many unanswered expansions a walk may carry over before
// the walk itself is the thing that is broken. A shelf or two the API will
// not answer for is exactly what carrying over is for; most of a game going
// unanswered is credentials, a deadline or an outage, and carrying all of it
// over would republish yesterday's catalog as today's and call it a success
// - the one failure mode worse than the crash this replaced, because nothing
// would ever say so. The floor of one keeps a game with a handful of
// expansions from being held to a stricter rule than Pokemon's 782.
func carryLimit(expansions int) int {
	return max(1, expansions/10)
}

// carryOver fills the expansions the walk could not read from the previous
// catalog, so a shelf the API would not answer for keeps yesterday's
// products instead of vanishing from the file.
//
// This is safe here in a way it would not be for a price file: the catalog
// carries names, numbers, rarities and versions and no prices at all, so
// yesterday's entry for a card is worth the same as today's. What ages is
// only the shelf's membership - a product added since the last good walk is
// missing until one succeeds - which is a far smaller wrong than unpricing
// every card on the shelf.
//
// An expansion the previous catalog has nothing for does not fail the run.
// That case is almost exactly one thing: a shelf added since the last good
// walk, because any older one is in the previous catalog by definition. And
// there, failing makes things strictly worse - it leaves yesterday's whole
// file in place, so the new shelf is missing anyway and every other shelf is
// a day stale on top of it. The expansion is recorded with no products, said
// plainly in the log, and named in the meta.
//
// Being unable to read the previous catalog at all does fail the run. A
// missing object and a mistyped bucket or a revoked key are not worth trying
// to tell apart from the error: publishing holes every day because -previous
// quietly pointed nowhere is the failure nobody would notice, and a brand
// new game hits this only if its very first walk also loses a shelf.
func carryOver(ctx context.Context, path string, failed []unanswered,
	catalog *cardmarket.Catalog) error {
	if path == "" {
		return fmt.Errorf("no -previous to carry %d unanswered expansion(s) over from: %s",
			len(failed), describe(failed))
	}

	reader, err := openReader(ctx, path)
	if err != nil {
		return fmt.Errorf("reading %s to carry over %s: %w", path, describe(failed), err)
	}
	defer reader.Close()

	old, err := cardmarket.LoadCatalog(reader)
	if err != nil {
		return fmt.Errorf("reading %s to carry over %s: %w", path, describe(failed), err)
	}

	// One pass over the previous file rather than one per failed
	// expansion: the big catalogs run to tens of thousands of products.
	wanted := make(map[int]bool, len(failed))
	for _, one := range failed {
		wanted[one.expansion.IDExpansion] = true
	}
	carried := make(map[int]int, len(failed))
	for id, product := range old.Data.Products {
		if !wanted[product.ExpansionID] {
			continue
		}
		catalog.Data.Products[id] = product
		carried[product.ExpansionID]++
	}

	for _, one := range failed {
		id := one.expansion.IDExpansion
		// A shelf the API refuses once is a bad morning; one it has
		// refused every run since is a shelf nobody is reading, and the
		// catalog would go on quietly carrying or emptying it with
		// nothing to say how long that had been true. The previous
		// file's own meta answers it for nothing.
		stuck := ""
		if slices.Contains(old.Meta.Unwalked, id) {
			stuck = fmt.Sprintf(" - and %s did not walk it either, so it has gone unread since at least %s",
				path, old.Meta.Date)
		}
		// The expansion's own name and code come from this run's
		// Expansions call, which succeeded - only its products are old.
		catalog.Data.Expansions[id] = cardmarket.CatalogExpansion{
			Name: one.expansion.Name,
			Code: one.expansion.SetCode,
		}
		catalog.Meta.Unwalked = append(catalog.Meta.Unwalked, id)

		if carried[id] == 0 {
			log.Printf("Expansion %d %q went unanswered (%v) and %s has nothing for it, "+
				"so it is in this catalog with no products - a shelf added since the last good walk%s",
				id, one.expansion.Name, one.err, path, stuck)
			continue
		}
		log.Printf("Carried %d products over for expansion %d %q, unanswered (%v), from %s%s",
			carried[id], id, one.expansion.Name, one.err, path, stuck)
	}
	slices.Sort(catalog.Meta.Unwalked)

	return nil
}

// openReader opens the previous catalog the way openWriter opens the new
// one, and decompresses it by the same suffix.
func openReader(ctx context.Context, path string) (io.ReadCloser, error) {
	u, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	if u.Scheme == "b2" {
		accessKey := os.Getenv("B2_APPLICATION_KEY_ID")
		secretKey := os.Getenv("B2_APPLICATION_KEY")
		if accessKey == "" || secretKey == "" {
			return nil, errors.New("missing B2_APPLICATION_KEY_ID or B2_APPLICATION_KEY env vars")
		}
		return simplecloud.Open(ctx, path, simplecloud.WithB2Credentials(accessKey, secretKey))
	}

	return simplecloud.Open(ctx, path)
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
