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
  - `Articles(ctx, productID, options, page, maxResults) ([]Article, error)`

`options` is yours to choose. This package spells the filters and picks none
of them: what condition is worth pricing, and which sellers are worth reading,
are judgements about your own use, not facts about the marketplace.

Pages start at zero and the API requires both bounds — asking for a page size
without a start is refused. `MaxEntities` (**100**) is the largest page it
serves.

### Two shapes of one Product

The marketplace answers the same entity two ways, and which request built a
`Product` decides which half of it is filled:

| filled by | fields |
|---|---|
| `ExpansionSingles` | `ExpansionName`, `ExpansionIcon` |
| `Product` | `Expansion` (nested), `PriceGuide` |

A product read one way carries nothing of the other's.

### Games are typed, and all of them are named

`Game` is its own type rather than an `int`, so handing a language or a
country where a game belongs stops compiling instead of answering a plausible
page. `GameName` and `GameFromName` read one table, so a game cannot be added
to one direction and not the other:

```go
cardmarket.GameFromName("gundam")          // GameGundam
cardmarket.GameName(cardmarket.GameGundam) // "Gundam"
```

That table now covers **every** game the marketplace sells, not only the ones
any particular caller prices — including the abbreviations the storefront
actually uses (`FoW`, `WoW`, `Vanguard`, `Spoils`), which are not
interchangeable with the full names. An unknown name answers `0`, and so
builds no link at all; `""` answers `0` too, where it used to answer Magic.

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
  way, and failing would leave every other shelf a day stale too. What does
  still fail the run is a `-previous` that cannot be read at all, and more
  than a tenth of the expansions going unanswered.
- `-output` — a path, or a `b2://bucket/object` one; an `.xz` suffix compresses it

A `b2://` output reads `B2_APPLICATION_KEY_ID` and `B2_APPLICATION_KEY` — the
same names the `b2` command line uses, so one pair of credentials works for
both.

Any expansion failing fails the run. The client already retries the transient
errors, and a partial catalog uploaded on schedule would silently unprice
whatever it dropped.

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
