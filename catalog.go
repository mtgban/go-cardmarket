package cardmarket

import (
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strconv"
)

// Catalog is a published Cardmarket catalog: every singles product mapped to
// the printings it sells, which is what a price run reads instead of walking
// the API.
//
// Two programs write this file and they fill different halves of it. MTGJSON
// publishes Magic's, with the uuids of the printings each product stands for
// and no rarity, version or expansion code. cmd/mkmcatalog walks the API for
// every other game, which answers the rarity and the version and the code but
// knows nothing of any datastore's uuids. A field absent from one producer is
// absent from every product it writes, not missing from a particular row.
type Catalog struct {
	Meta CatalogMeta `json:"meta"`
	Data CatalogData `json:"data"`
}

// CatalogMeta says when the file was built and by which version of whatever
// built it. Both are carried through and neither is enforced: the two
// producers version themselves differently and a reader that refused an
// unfamiliar string would refuse a file it can read perfectly well.
type CatalogMeta struct {
	Date    string `json:"date"`
	Version string `json:"version"`
}

// CatalogData is the file's two tables. JSON has only string keys, so the
// ids are written as strings and read back as the numbers they are, which
// encoding/json does on its own for an integer-keyed map.
type CatalogData struct {
	Expansions map[int]CatalogExpansion `json:"expansions"`
	Products   map[int]CatalogProduct   `json:"products"`
}

// CatalogProduct is one product of the catalog: what the marketplace calls
// it, where it files it, and the printings it stands for.
type CatalogProduct struct {
	ExpansionID int    `json:"expansionId"`
	Name        string `json:"name"`
	Number      string `json:"number,omitempty"`
	Rarity      string `json:"rarity,omitempty"`
	// Version is the index the marketplace counts a card's printings with
	// where one shelf sells several - the second Budew of a stamp
	// programme, the master-ball pattern beside the poke-ball one. It is
	// the only thing telling two products of one name and number apart, and
	// zero where the product carries none. See ProductVersion for where it
	// is read from, which is not the same field in every game.
	Version int `json:"version,omitempty"`
	// UUIDs are the printings the product sells, as MTGJSON links them.
	// Only Magic's file carries any.
	UUIDs []string `json:"uuids,omitempty"`
}

// CatalogExpansion names one expansion of the catalog. Code is the
// marketplace's own abbreviation, which is where the foreign catalogs say
// what they are; MTGJSON's file does not carry it.
type CatalogExpansion struct {
	Name string `json:"name"`
	Code string `json:"code,omitempty"`
}

// LoadCatalog reads a published catalog. A file naming no products is
// refused: it is the shape a truncated download and an unrelated JSON file
// both take, and pricing from it would quietly unprice a whole game.
func LoadCatalog(reader io.Reader) (*Catalog, error) {
	var catalog Catalog
	err := json.NewDecoder(reader).Decode(&catalog)
	if err != nil {
		return nil, err
	}
	if len(catalog.Data.Products) == 0 {
		return nil, errors.New("empty catalog")
	}
	return &catalog, nil
}

// nameVersionRe matches the index Cardmarket writes into a product's name,
// which is where Magic and Yu-Gi-Oh carry it: "Feral Shadow (V.1)" and
// "7 Colored Fish (V.2 - Common)" both.
var nameVersionRe = regexp.MustCompile(`\(V\.(\d+)`)

// slugVersionRe matches the same index written into the product's own web
// address, which is where Pokemon carries it and nowhere else:
// "Budew-V2-SEAPRE-004". The last occurrence is the index, the segments
// before it being the card's name - "Serperior-V-V3-SITTG13" is Serperior V
// at version 3, and the V of its name carries no digits to be mistaken for
// one.
var slugVersionRe = regexp.MustCompile(`-V(\d+)-`)

// ProductVersion reads that index off a product, or zero where it carries
// none. The marketplace publishes it in the name for some games and only in
// the address for others - of the catalogs we walk, 31,523 of Yu-Gi-Oh's
// 86,628 products name it and not one of Pokemon's 72,752 does, though its
// shelves are full of it - so both are read and the name wins.
func ProductVersion(product *Product) int {
	if fields := nameVersionRe.FindStringSubmatch(product.Name); fields != nil {
		version, err := strconv.Atoi(fields[1])
		if err == nil {
			return version
		}
	}
	matches := slugVersionRe.FindAllStringSubmatch(product.Website, -1)
	if len(matches) > 0 {
		version, err := strconv.Atoi(matches[len(matches)-1][1])
		if err == nil {
			return version
		}
	}
	return 0
}
