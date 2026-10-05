package cardmarket

import (
	"strings"
	"testing"
)

// catalogFixture is the shape mkmcatalog writes: string keys on both
// tables, an expansion code, and a rarity and version where the product
// carries them.
const catalogFixture = `{
	"meta": {"date": "2026-09-08"},
	"data": {
		"expansions": {
			"1": {"name": "Alpha", "code": "LEA"},
			"4170": {"name": "Unnumbered Promos", "code": "UNP"}
		},
		"products": {
			"14866": {
				"expansionId": 1,
				"name": "Laughing Hyena",
				"number": "103",
				"rarity": "Common"
			},
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
	if len(catalog.Data.Products) != 2 || len(catalog.Data.Expansions) != 2 {
		t.Fatalf("got %d products and %d expansions, want 2 and 2",
			len(catalog.Data.Products), len(catalog.Data.Expansions))
	}
	product, found := catalog.Data.Products[14866]
	if !found {
		t.Fatal("the string key 14866 did not become the number")
	}
	if product.Number != "103" || product.Rarity != "Common" || product.Version != 0 {
		t.Errorf("product 14866 = %+v", product)
	}
	if product := catalog.Data.Products[571798]; product.Rarity != "Promo" || product.Version != 2 {
		t.Errorf("product 571798 = %+v", product)
	}
	if expansion := catalog.Data.Expansions[4170]; expansion.Name != "Unnumbered Promos" || expansion.Code != "UNP" {
		t.Errorf("expansion 4170 = %+v", expansion)
	}
	if catalog.Meta.Date != "2026-09-08" {
		t.Errorf("meta = %+v", catalog.Meta)
	}
}

// TestLoadCatalogUnknownKeys reads a file carrying keys the struct does not
// name, as MTGJSON's CardmarketIdentifiers does: they are ignored, not
// refused.
func TestLoadCatalogUnknownKeys(t *testing.T) {
	const fixture = `{
		"meta": {"date": "2026-09-03", "version": "5.3.0"},
		"data": {
			"expansions": {"1": {"name": "Alpha", "setCodes": ["LEA"]}},
			"products": {"14866": {"expansionId": 1, "name": "Laughing Hyena", "uuids": ["aaa"]}}
		}
	}`
	catalog, err := LoadCatalog(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("an unknown key should be ignored, not refused: %v", err)
	}
	if catalog.Data.Products[14866].Name != "Laughing Hyena" {
		t.Errorf("product 14866 = %+v", catalog.Data.Products[14866])
	}
}

func TestLoadCatalogEmpty(t *testing.T) {
	if _, err := LoadCatalog(strings.NewReader(`{"data":{}}`)); err == nil {
		t.Error("a catalog naming no products should refuse to load")
	}
}
