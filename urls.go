package cardmarket

import (
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// URLOption is everything that shapes a storefront link but the game and the
// thing being linked to: which listings the caller means, which language the
// page should prefer, and whose affiliate tag it carries.
//
// The zero value asks for nothing at all - no narrowing, no language, no
// tag - and each field set adds a parameter the marketplace understands.
// What is worth filtering out is the caller's judgement, so this package
// spells the filters and decides none of them.
//
// A warning, from the marketplace rather than from here: its filters fail
// open. A parameter that does not apply to a game, or a value it does not
// recognise, is ignored and the unfiltered listing is answered - not an
// error. A link is a suggestion to a browser, so that is survivable; a price
// read through a filter is not, and should be checked against each article's
// own flags rather than trusted to the request.
//
// The finish flags are the game's own and not one vocabulary: a Magic
// listing carries Foil and neither of the others, a Pokemon listing carries
// FirstEd and ReverseHolo and no Foil at all. They are independent per
// Article (a Pokemon listing can carry FirstEd and ReverseHolo together,
// confirmed live), so a caller holding a real Article can pass its flags
// straight through with no per-game mapping of its own.
type URLOption struct {
	// Foil, FirstEd and ReverseHolo narrow to a printing. None is worth
	// as much as Only here: a non-foil price that links to a page
	// showing foils is quoting from a shelf the reader cannot see.
	Foil        Filter
	FirstEd     Filter
	ReverseHolo Filter

	// Signed and Altered narrow by what a signature or an alteration has
	// done to a listing, which is to make it incomparable with the rest
	// of the shelf. They are named as the Article carries them.
	Signed  Filter
	Altered Filter

	// SellerTypes and SellerCountries narrow to the listings of the kinds
	// of seller named, based in the countries named: UserTypePowerseller
	// with CountryGermany and CountryNetherlands is the German and Dutch
	// powersellers alone. Each is a list because the storefront's own
	// controls are, and an empty one asks for every seller. The order
	// given does not matter; the link is the same either way. A seller
	// type the storefront has no number for is left out.
	SellerTypes     []UserType
	SellerCountries []Country

	// Language asks the page to prefer one language, which it honours
	// where the card has a printing in it and quietly ignores where it
	// does not. Zero asks for nothing; LanguageEnglish is the usual choice.
	Language Language

	// Affiliate is the tag the link is attributed to, empty for none. It
	// is a named field so it cannot be transposed with SearchURL's name;
	// the cost is that leaving it out loses the tag without a word.
	Affiliate string
}

// set writes the filter under one parameter name, or leaves it out where the
// caller does not care - which is the marketplace's own 0, spelled by saying
// nothing.
func (f Filter) set(v url.Values, name string) {
	switch f {
	case Only:
		v.Set(name, "Y")
	case None:
		v.Set(name, "N")
	}
}

// setBool writes the filter as the API spells a flag: true, false, or not at
// all.
func (f Filter) setBool(v url.Values, name string) {
	switch f {
	case Only:
		v.Set(name, "true")
	case None:
		v.Set(name, "false")
	}
}

// sellerTypes numbers the seller types as the storefront's sellerType does,
// which is how ArticleSeller.IsCommercial numbers them: private 0,
// professional 1, powerseller 2.
var sellerTypes = map[UserType]int{
	UserTypePrivate:     0,
	UserTypeCommercial:  1,
	UserTypePowerseller: 2,
}

// setList writes a multiple choice as the storefront's own links do: one
// parameter, its values joined by commas in ascending order. An empty list
// writes nothing, which is every value at once.
func setList(v url.Values, name string, ids []int) {
	if len(ids) == 0 {
		return
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.Itoa(id)
	}
	v.Set(name, strings.Join(parts, ","))
}

// values renders the option as the parameters the storefront reads. Every
// flag is a bare parameter taking Y or N.
func (opt URLOption) values() url.Values {
	v := url.Values{}

	opt.Foil.set(v, "isFoil")
	opt.FirstEd.set(v, "isFirstEd")
	opt.ReverseHolo.set(v, "isReverseHolo")
	opt.Signed.set(v, "isSigned")
	opt.Altered.set(v, "isAltered")

	var types []int
	for _, userType := range opt.SellerTypes {
		if n, ok := sellerTypes[userType]; ok {
			types = append(types, n)
		}
	}
	setList(v, "sellerType", types)

	countries := make([]int, len(opt.SellerCountries))
	for i, country := range opt.SellerCountries {
		countries[i] = int(country)
	}
	setList(v, "sellerCountry", countries)

	if opt.Language != 0 {
		v.Set("language", fmt.Sprint(int(opt.Language)))
	}
	if opt.Affiliate != "" {
		v.Set("utm_source", opt.Affiliate)
		v.Set("utm_medium", "text")
		v.Set("utm_campaign", "card_prices")
	}

	return v
}

// gameURL is the storefront root for one game, or nil for a game the
// marketplace does not carry - which is how both builders come to answer no
// link at all rather than one pointing at a path the site does not serve.
func gameURL(game Game, path string) *url.URL {
	name := GameName(game)
	if name == "" {
		return nil
	}
	u, err := url.Parse(fmt.Sprintf("https://www.cardmarket.com/en/%s/%s", name, path))
	if err != nil {
		return nil
	}
	return u
}

// BuildURL builds the storefront link for a product, narrowed and tagged by
// whatever the option carries.
func BuildURL(game Game, idProduct int, opt URLOption) string {
	u := gameURL(game, "Products")
	if u == nil {
		return ""
	}

	v := opt.values()
	v.Set("idProduct", fmt.Sprint(idProduct))

	u.RawQuery = v.Encode()
	return u.String()
}

// SearchURL returns the catalog search for a product name, the fallback for
// a card whose Cardmarket product id is not known. It takes the same option
// as BuildURL and is empty for the same games, so adding a filter to one
// cannot quietly leave the other behind.
func SearchURL(game Game, name string, opt URLOption) string {
	u := gameURL(game, "Products/Search")
	if u == nil {
		return ""
	}

	v := opt.values()
	v.Set("searchString", name)

	u.RawQuery = v.Encode()
	return u.String()
}
