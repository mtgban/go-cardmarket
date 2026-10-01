# go-cardmarket

A small Go client for the **Cardmarket** API (catalog & pricing), plus the two
tools that read its published catalog files. It handles **OAuth 1.0a request
signing**, **rate-limit backoff**, and resilient HTTP via **retryablehttp**.

Entities are named as
[Cardmarket's own documentation](https://apiv2.cardmarket.com/ws/documentation)
names them — `Product`, `Expansion`, `Article` — so a field can be looked up in
one place rather than translated first.

---

## Install

```bash
go get github.com/mtgban/go-cardmarket
```

---

## Quick start

```go
package main

import (
    "context"
    "fmt"

    "github.com/mtgban/go-cardmarket"
)

func main() {
    c := cardmarket.NewClient("<APP_TOKEN>", "<APP_SECRET>")

    expansions, err := c.Expansions(context.TODO(), cardmarket.GamePokemon)
    if err != nil {
        // <handle err>
    }
    fmt.Println("expansions:", len(expansions))
}
```

---

## Two transports, two naming conventions

Cardmarket publishes its catalog twice over, and the method name says which
one you are reading:

- **Bare entity names** (`Product`, `Expansions`, `Articles`, …) are the signed
  API at `apiv2.cardmarket.com`. They need app credentials and count against a
  daily request allowance.
- **`Download*`** (`DownloadPriceGuide`, `DownloadProductListSingles`, …) are
  the static files at `downloads.s3.cardmarket.com`. No credentials, no
  allowance, a whole game per request.

Prefer the downloads wherever they answer the question. Walking a game through
the API is thousands of requests; the same data as a file is one.

---

## Authentication

`NewClient(appToken, appSecret)` wires an `http.RoundTripper` that signs every
request with OAuth 1.0a (HMAC-SHA1), as Cardmarket's
[auth documentation](https://apiv2.cardmarket.com/ws/documentation/API:Auth_Overview)
describes. Only the app token and secret are needed for the public catalog
requests; the access-token pair stays empty.

`RequestNo()` reports how many requests the client has made, which is what to
watch against the daily allowance.

---

## API helpers

- **Expansions**
  - `Expansions(ctx, game) ([]Expansion, error)` — every expansion of a game
  - `ExpansionSingles(ctx, expansionID) ([]Product, error)` — every single card in one
- **Products**
  - `Product(ctx, productID) (*Product, error)`
- **Articles** (listings)
  - `Articles(ctx, productID, options, page, maxResults) (articles []Article, total int, capped bool, err error)`

`options` is yours to choose. This package spells the filters and picks none
of them: what condition is worth pricing, and which sellers are worth reading,
are judgements about your own use, not facts about the marketplace.

> Cardmarket's filters **fail open**. A parameter that does not apply to a
> game, or a value it does not recognise, is ignored and the unfiltered
> listing is answered — not an error. Check each `Article`'s own flags rather
> than trusting the request.

Pages start at zero and the API requires both bounds — asking for a page size
without a start is refused. `MaxEntities` (**100**) is the largest page it
serves.

`total` is the listing's full count, read from the response's `Content-Range`,
so a caller can stop once it has paged through everything; it is `0` when the
header is missing. `capped` means Cardmarket stopped counting at 1000 and the
real count is unknown.

### Two shapes of one Product

The marketplace answers the same entity two ways, and which request built a
`Product` decides which half of it is filled:

| filled by | fields |
|---|---|
| `ExpansionSingles` | `ExpansionName`, `ExpansionIcon` |
| `Product` | `Expansion` (nested), `PriceGuide` |

A product read one way carries nothing of the other's.

### Identifiers are typed

`Game`, `Language`, `Country`, `Condition`, `UserType` and `UserScore` are
each their own type. The API spends small integers freely — a game, a
language, a country and a seller score are all `3` to a compiler told nothing
else, and they ride on the same request — so naming them makes the wrong one
a compile error instead of a plausible page.

Each has a `…Name` and a `…FromName`, read from one table so an entry cannot
be added to one direction and not the other — all but `UserType`, whose
constants are already the words the marketplace uses:

```go
cardmarket.GameFromName("gundam")                    // GameGundam
cardmarket.GameName(cardmarket.GameGundam)           // "Gundam"
cardmarket.LanguageName(cardmarket.LanguageJapanese) // "Japanese"
```

`GameName` covers **every** game the marketplace sells, not only the ones any
particular caller prices — including the abbreviations the storefront
actually uses (`FoW`, `WoW`, `Vanguard`, `Spoils`), which are not
interchangeable with the full names. An unknown name answers `0`, and so
builds no link at all.

The language and country numbers are upstream's. Three countries —
**Singapore (29), Canada (33), Japan (36)** — are on the marketplace's own
seller-country filter but missing from its documentation; they are included
here, because filtering by the documented list alone silently drops every
seller in them.

`Condition` and `UserScore` are declared best to worst, which is the order
`minCondition` and `minUserScore` mean by "or better". `ConditionFromName`
takes either the code or the word, so a caller holding an `Article`'s own
condition does not have to know which it has.

### Storefront links

`BuildURL(game, productID, opt)` and `SearchURL(game, name, opt)` take the
same `URLOption`, so a filter added to one cannot quietly skip the other:

```go
url := cardmarket.BuildURL(game, id, cardmarket.URLOption{
    Foil:      cardmarket.Only,
    Signed:    cardmarket.None,
    Altered:   cardmarket.None,
    Language:  cardmarket.LanguageEnglish,
    Affiliate: "mtgban",
})
```

Every flag is three-valued, because the marketplace's are: `Any` says
nothing, `Only` asks for the listings that carry it, `None` for the ones that
do not. `Any` is the zero value, so `URLOption{}` narrows nothing.

`None` earns its place on the finishes too — a non-foil price that links to a
page showing foils is quoting from a shelf the reader cannot see.

Every flag is a bare parameter (`isFoil=N`, `isSigned=N`, `isAltered=N`).

`SellerTypes` and `SellerCountries` narrow by who is selling. Each is a list,
as the storefront's own controls are, and is written as one parameter with its
values joined by commas in ascending order, whatever order they were named in:

```go
cardmarket.URLOption{
    SellerTypes:     []cardmarket.UserType{cardmarket.UserTypePowerseller},
    SellerCountries: []cardmarket.Country{cardmarket.CountryNetherlands, cardmarket.CountryGermany},
}
```

The storefront numbers the seller types where the API names them, so the
`UserType` words are translated on the way out: private is `0`, professional
`1` and powerseller `2`, as an `Article`'s `IsCommercial` numbers them.

`Language` asks the page to prefer one language, and sends nothing at zero.
`Affiliate` tags the link with `utm_source`, `utm_medium` and `utm_campaign`,
and sends nothing when empty.

Both builders return `""` for a number that names no game, rather than a link
to a path the site does not serve.

### Finish flags are per-game

A Magic `Article` carries `IsFoil` and neither of the others. A Pokémon
`Article` carries `IsFirstEd` and `IsReverseHolo` and no `IsFoil` at all. A
flag absent from a game's listings decodes false, so read the one its game
uses.

---

## Published catalog files

- `DownloadPriceGuide(ctx, game) ([]PriceGuide, error)`
- `DownloadProductListSingles(ctx, game) ([]ProductList, error)`
- `DownloadProductListSealed(ctx, game) ([]ProductList, error)`

Each has a `…File` form — `DownloadPriceGuideFile(ctx, game) (*PriceGuideFile, error)`,
and likewise for the two product lists — that also returns the file's `Version`
and the `CreatedAt` Cardmarket stamped it with, so a caller can refuse a file
that has stopped updating. How old is too old is yours to say: the price guide
and the product lists are rebuilt on different schedules. An unreadable
`CreatedAt` is an error in the `…File` forms; the plain ones never read it.

`PriceGuide.SecondPrinting(game)` reads the printing sold beside the default
one under whichever heading the game's guide publishes it — most games publish
a foil, Pokémon publishes a reverse holo.

---

## The catalog file

`Catalog` is the id map a price run reads instead of walking the API. Two
programs write it and they fill different halves:

| field | MTGJSON (Magic) | mkmcatalog (everything else) |
|---|---|---|
| `expansionId`, `name` | yes | yes |
| `number` | mostly | yes |
| `uuids` | yes | never |
| `rarity`, `version` | never | yes |
| expansion `code` | never | yes |

A field absent from one producer is absent from every product it writes, not
missing from a particular row.

```go
catalog, err := cardmarket.LoadCatalog(reader)
product := catalog.Data.Products[571798]
```

`meta.date` and `meta.version` are carried through and neither is enforced: the
two producers version themselves differently, and a reader refusing an
unfamiliar string would refuse a file it can read perfectly well. A file naming
no products *is* refused — that is the shape a truncated download takes.

---

## Commands

### mkmcatalog

Walks one game's catalog and writes the file above.

```bash
go build ./cmd/mkmcatalog

MKM_APP_TOKEN=... MKM_APP_SECRET=... \
  ./mkmcatalog -game pokemon -output cardmarket_catalog.json
```

Flags:
- `-game` — any game by name (`lorcana`, `riftbound`, `gundam`, `pokemon`, …);
  see `GameFromName`
- `-previous` — a catalog to carry an unanswerable expansion's products over
  from, read only when the walk leaves one. The API intermittently answers
  5xx for a particular expansion however many times it is asked; rather than
  throwing away the whole walk over one shelf, its products are taken from
  this file and the expansion is named in `meta.unwalked`. A shelf added
  since the last good walk has nothing to carry over and is recorded with no
  products rather than failing the run — it would have been missing either
  way, and failing would leave every other shelf a day stale too.
- `-output` — a path, or a `b2://bucket/object` one; an `.xz` suffix compresses it

A `b2://` output reads `B2_APPLICATION_KEY_ID` and `B2_APPLICATION_KEY` — the
same names the `b2` command line uses, so one pair of credentials works for
both.

An expansion the API will not answer for gets one more try at the end of the
walk before it is carried over. The run fails, rather than publish a catalog
that silently unprices whatever it dropped, when:

- the walk does not finish within its three-hour deadline
- more than a tenth of the expansions go unanswered
- an expansion needs carrying over and `-previous` is missing or unreadable

### mkmpriceguide

Downloads a published file to stdout. Needs no credentials.

```bash
go build ./cmd/mkmpriceguide
./mkmpriceguide -game pokemon -mode prices > pokemon-prices.json
```

Flags:
- `-game` — the game by name (see the `Game*` constants)
- `-mode` — `prices` (default), `singles`, `sealed`

---

## License

MIT
