// Command mkmpriceguide downloads Cardmarket's published catalog files for a
// game: the price guide, or the product list for singles or sealed product.
// None of them is signed, so it needs no credentials.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/mtgban/go-cardmarket"
)

const downloadTimeout = 10 * time.Minute

func run() int {
	mode := flag.String("mode", "prices", "Which file to download [prices]/singles/sealed")
	game := flag.Int("game", 1, "Select which game (default=magic)")

	flag.Parse()

	var output any
	var err error

	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()
	switch *mode {
	default:
		output, err = cardmarket.DownloadPriceGuide(ctx, *game)
	case "singles":
		output, err = cardmarket.DownloadProductListSingles(ctx, *game)
	case "sealed":
		output, err = cardmarket.DownloadProductListSealed(ctx, *game)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	err = enc.Encode(output)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	return 0
}

func main() {
	os.Exit(run())
}
