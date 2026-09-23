package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mtgban/go-cardmarket"
)

// writePrevious writes a catalog to a temp file and returns its path, the
// shape carryOver reads a previous run's output back in.
func writePrevious(t *testing.T, catalog cardmarket.Catalog) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cardmarket_catalog.json")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := json.NewEncoder(file).Encode(&catalog); err != nil {
		t.Fatal(err)
	}
	return path
}

// freshCatalog is the walk's own output: one expansion read successfully,
// which must survive whatever carrying over does.
func freshCatalog() cardmarket.Catalog {
	catalog := cardmarket.Catalog{}
	catalog.Data.Expansions = map[int]cardmarket.CatalogExpansion{
		1: {Name: "Walked Fine", Code: "WF"},
	}
	catalog.Data.Products = map[int]cardmarket.CatalogProduct{
		99: {ExpansionID: 1, Name: "Freshly Walked"},
	}
	return catalog
}

// An expansion the API would not answer for keeps yesterday's products, and
// says so in the meta - the point being that the rest of the walk survives
// it, which it did not before.
func TestCarryOver(t *testing.T) {
	previous := cardmarket.Catalog{}
	previous.Data.Expansions = map[int]cardmarket.CatalogExpansion{
		1645: {Name: "Pokémon Products", Code: "PP"},
		1:    {Name: "Walked Fine", Code: "WF"},
	}
	previous.Data.Products = map[int]cardmarket.CatalogProduct{
		10: {ExpansionID: 1645, Name: "Pikachu", Number: "1", Rarity: "Promo", Version: 2},
		11: {ExpansionID: 1645, Name: "Eevee", Number: "2"},
		12: {ExpansionID: 1, Name: "Somewhere Else", Number: "3"},
	}
	path := writePrevious(t, previous)

	catalog := freshCatalog()
	failed := []unanswered{{
		expansion: cardmarket.Expansion{IDExpansion: 1645, Name: "Pokémon Products", SetCode: "PP"},
		err:       errors.New("cardmarket: 503 Service Unavailable after 4 attempt(s)"),
	}}

	if err := carryOver(context.Background(), path, failed, &catalog); err != nil {
		t.Fatalf("carryOver() = %v", err)
	}

	// Only the failed expansion's products come over: a product of an
	// expansion this walk read is this walk's business, not the old file's.
	if _, ok := catalog.Data.Products[12]; ok {
		t.Error("carried a product of an expansion that walked fine")
	}
	if _, ok := catalog.Data.Products[99]; !ok {
		t.Error("dropped a product this walk read")
	}
	for _, id := range []int{10, 11} {
		if _, ok := catalog.Data.Products[id]; !ok {
			t.Errorf("product %d was not carried over", id)
		}
	}
	if got := catalog.Data.Products[10]; got.Version != 2 || got.Rarity != "Promo" {
		t.Errorf("product 10 lost fields in the carry: %+v", got)
	}

	// The expansion's own name and code come from this run, which read
	// them successfully - only its products are old.
	if got := catalog.Data.Expansions[1645]; got.Name != "Pokémon Products" || got.Code != "PP" {
		t.Errorf("expansion 1645 = %+v, want the walk's own name and code", got)
	}

	// A reader cannot otherwise tell a carried shelf from a walked one.
	if len(catalog.Meta.Unwalked) != 1 || catalog.Meta.Unwalked[0] != 1645 {
		t.Errorf("meta.Unwalked = %v, want [1645]", catalog.Meta.Unwalked)
	}
}

// A shelf added since the last good walk is the one case where the previous
// catalog has nothing to give, and failing there would be strictly worse
// than carrying on: it would leave yesterday's whole file in place, so the
// new shelf would be missing anyway and every other shelf a day stale with
// it. The run continues, and the meta says the shelf is not this walk's.
func TestCarryOverKeepsANewExpansion(t *testing.T) {
	previous := cardmarket.Catalog{}
	previous.Data.Expansions = map[int]cardmarket.CatalogExpansion{1: {Name: "Walked Fine"}}
	previous.Data.Products = map[int]cardmarket.CatalogProduct{
		12: {ExpansionID: 1, Name: "Somewhere Else"},
	}
	path := writePrevious(t, previous)

	catalog := freshCatalog()
	failed := []unanswered{{
		expansion: cardmarket.Expansion{IDExpansion: 4242, Name: "Brand New Shelf", SetCode: "GD07"},
		err:       errors.New("cardmarket: 503 Service Unavailable after 4 attempt(s)"),
	}}

	if err := carryOver(context.Background(), path, failed, &catalog); err != nil {
		t.Fatalf("carryOver() = %v, want the walk to survive a brand new shelf", err)
	}

	if _, ok := catalog.Data.Products[99]; !ok {
		t.Error("dropped a product this walk read")
	}
	// The shelf is named, so a reader knows it exists and why it is empty.
	if got := catalog.Data.Expansions[4242]; got.Name != "Brand New Shelf" || got.Code != "GD07" {
		t.Errorf("expansion 4242 = %+v, want it recorded from the walk", got)
	}
	for id, product := range catalog.Data.Products {
		if product.ExpansionID == 4242 {
			t.Errorf("product %d appeared for an expansion nothing could fill", id)
		}
	}
	if len(catalog.Meta.Unwalked) != 1 || catalog.Meta.Unwalked[0] != 4242 {
		t.Errorf("meta.Unwalked = %v, want [4242]", catalog.Meta.Unwalked)
	}
}

// Without a -previous there is nothing to carry from, and the error has to
// say which expansions were lost and why, not only that some were.
func TestCarryOverWithoutPrevious(t *testing.T) {
	catalog := freshCatalog()
	failed := []unanswered{{
		expansion: cardmarket.Expansion{IDExpansion: 5303, Name: "Unnumbered Promos"},
		err:       errors.New("cardmarket: 503 Service Unavailable after 4 attempt(s)"),
	}}

	err := carryOver(context.Background(), "", failed, &catalog)
	if err == nil {
		t.Fatal("carryOver() succeeded with no -previous, want an error")
	}
	for _, want := range []string{"5303", "Unnumbered Promos", "503"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q: %v", want, err)
		}
	}
}

// A -previous that cannot be read is a mistyped bucket or a revoked key far
// more often than it is a game's first ever walk, and publishing holes every
// day because it quietly pointed nowhere is the failure nobody would notice.
func TestCarryOverUnreadablePrevious(t *testing.T) {
	catalog := freshCatalog()
	failed := []unanswered{{
		expansion: cardmarket.Expansion{IDExpansion: 5303, Name: "Unnumbered Promos"},
		err:       errors.New("cardmarket: 503 Service Unavailable after 4 attempt(s)"),
	}}
	missing := filepath.Join(t.TempDir(), "nothing-here.json")

	err := carryOver(context.Background(), missing, failed, &catalog)
	if err == nil {
		t.Fatal("carryOver() succeeded against an unreadable -previous, want an error")
	}
	if !strings.Contains(err.Error(), "5303") {
		t.Errorf("error does not name what was lost: %v", err)
	}
}

// Carrying over is for a shelf the API will not answer for, not for a walk
// that fell over: a run that carried most of a game across would republish
// yesterday's catalog as today's and report success.
func TestCarryLimit(t *testing.T) {
	tests := []struct {
		expansions int
		want       int
	}{
		{782, 78}, // Pokemon
		{149, 14}, // One Piece
		{29, 2},   // Gundam
		{2, 1},    // a floor, so a tiny game is not held to a stricter rule
		{0, 1},
	}
	for _, tt := range tests {
		if got := carryLimit(tt.expansions); got != tt.want {
			t.Errorf("carryLimit(%d) = %d, want %d", tt.expansions, got, tt.want)
		}
	}
}
