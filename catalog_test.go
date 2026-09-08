package cardmarket

import (
	"strings"
	"testing"
)

// catalogFixture is the shape MTGJSON publishes Magic's file in, which is
// the shape mkmcatalog writes the other games' in: string keys on both
// tables, and the fields one producer fills that the other does not.
const catalogFixture = `{
	"meta": {"date": "2026-09-03", "version": "5.3.0"},
	"data": {
		"expansions": {
			"1": {"name": "Alpha", "setCodes": ["LEA"]}
		},
		"products": {
			"14866": {
				"expansionId": 1,
				"name": "Laughing Hyena",
				"number": "103",
				"uuids": ["aaa", "bbb"]
			},
			"999999": {
				"expansionId": 1,
				"name": "No Printing Of Ours"
			}
		}
	}
}`

// mintedFixture is what mkmcatalog writes: an expansion code where MTGJSON
// writes set codes, and a rarity and version where it writes uuids.
const mintedFixture = `{
	"meta": {"date": "2026-09-08", "version": ""},
	"data": {
		"expansions": {
			"4170": {"name": "Unnumbered Promos", "code": "UNP"}
		},
		"products": {
			"571798": {
				"expansionId": 4170,
				"name": "Touch Change!",
				"rarity": "Promo",
				"version": 2
			}
		}
	}
}`

func TestLoadCatalog(t *testing.T) {
	catalog, err := LoadCatalog(strings.NewReader(catalogFixture))
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	if len(catalog.Data.Products) != 2 || len(catalog.Data.Expansions) != 1 {
		t.Fatalf("got %d products and %d expansions, want 2 and 1",
			len(catalog.Data.Products), len(catalog.Data.Expansions))
	}
	product, found := catalog.Data.Products[14866]
	if !found {
		t.Fatal("the string key 14866 did not become the number")
	}
	if product.Number != "103" || len(product.UUIDs) != 2 {
		t.Errorf("product 14866 = %+v", product)
	}
	if catalog.Data.Expansions[1].Name != "Alpha" {
		t.Errorf("expansion 1 = %+v", catalog.Data.Expansions[1])
	}
	// The meta is carried through and judged by nobody.
	if catalog.Meta.Date != "2026-09-03" || catalog.Meta.Version != "5.3.0" {
		t.Errorf("meta = %+v", catalog.Meta)
	}
	// MTGJSON's setCodes are no longer decoded, and a key the struct does
	// not name must not fail the load.
	if _, err := LoadCatalog(strings.NewReader(catalogFixture)); err != nil {
		t.Errorf("an unknown key should be ignored, not refused: %v", err)
	}
}

// TestLoadCatalogMinted reads the half mkmcatalog fills, which carries no
// uuids at all - the field MTGJSON's Magic file is mostly made of.
func TestLoadCatalogMinted(t *testing.T) {
	catalog, err := LoadCatalog(strings.NewReader(mintedFixture))
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	product := catalog.Data.Products[571798]
	if product.Rarity != "Promo" || product.Version != 2 || len(product.UUIDs) != 0 {
		t.Errorf("product 571798 = %+v", product)
	}
	if code := catalog.Data.Expansions[4170].Code; code != "UNP" {
		t.Errorf("expansion 4170 code = %q, want UNP", code)
	}
}

func TestLoadCatalogEmpty(t *testing.T) {
	if _, err := LoadCatalog(strings.NewReader(`{"data":{}}`)); err == nil {
		t.Error("a catalog naming no products should refuse to load")
	}
}
