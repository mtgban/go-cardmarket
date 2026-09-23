package cardmarket

import (
	"fmt"
	"net/url"
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

	// Language asks the page to prefer one language, which it honours
	// where the card has a printing in it and quietly ignores where it
	// does not. Zero asks for nothing; LanguageEnglish is the usual
	// choice and was what this package used to send unconditionally.
	Language Language

	// Affiliate is the tag the link is attributed to, empty for none.
	//
	// It sits here rather than beside the name it used to follow because
	// two adjacent strings are two strings in the wrong order sooner or
	// later, and SearchURL's pair compiled perfectly that way: a search
	// of the catalog for the affiliate, tagged with the card's name.
	// Named at the call site, that cannot happen - at the cost of being
	// quietly omittable, which loses a tag rather than building a wrong
	// link.
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

// values renders the option as the parameters the storefront reads.
//
// The finish flags and the listing flags are spelled differently by the
// marketplace, and neither spelling is this package's invention: isFoil is a
// bare parameter, while a signed or altered filter is a nested extra[...].
// Both take Y and N, both were read off the site's own filter form, and both
// were confirmed to partition a real product's listings.
func (opt URLOption) values() url.Values {
	v := url.Values{}

	opt.Foil.set(v, "isFoil")
	opt.FirstEd.set(v, "isFirstEd")
	opt.ReverseHolo.set(v, "isReverseHolo")
	opt.Signed.set(v, "extra[isSigned]")
	opt.Altered.set(v, "extra[isAltered]")

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
