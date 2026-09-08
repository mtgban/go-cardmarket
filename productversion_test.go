package cardmarket

import "testing"

// TestProductVersion pins where each game's version index is read from. The
// marketplace does not put it in one place: Magic and Yu-Gi-Oh write it into
// the product's name, Pokemon writes it only into the product's address, and
// a catalog that read one field would carry the index for two games and lose
// it for the third.
func TestProductVersion(t *testing.T) {
	for _, tt := range []struct {
		desc    string
		name    string
		website string
		want    int
	}{
		{
			// The Magic shape, as MTGJSON publishes it.
			desc: "Magic names it",
			name: "Feral Shadow (V.1)",
			want: 1,
		},
		{
			// Yu-Gi-Oh's carries the rarity behind the index.
			desc: "Yu-Gi-Oh names it with the rarity behind",
			name: "7 Colored Fish (V.2 - Common)",
			want: 2,
		},
		{
			// Pokemon names none of its 72,752 products this way; the
			// index is in the address alone.
			desc:    "Pokemon addresses it",
			name:    "Budew ",
			website: "/en/Pokemon/Products/Singles/Southeast-Asia-Promos/Budew-V2-SEAPRE-004",
			want:    2,
		},
		{
			// The card's own name ends in V, which carries no digits and
			// so cannot be read as the index. The last match is the one.
			desc:    "a card named V is not its own index",
			name:    "Serperior V ",
			website: "/en/Pokemon/Products/Singles/Silver-Tempest/Serperior-V-V3-SITTG13",
			want:    3,
		},
		{
			desc:    "nor is VMAX",
			name:    "Duraludon VMAX ",
			website: "/en/Pokemon/Products/Singles/Silver-Tempest/Duraludon-VMAX-V1-SITTG21",
			want:    1,
		},
		{
			// A V card sold as one product carries no index at all.
			desc:    "an unindexed V card answers none",
			name:    "Blaziken V ",
			website: "/en/Pokemon/Products/Singles/Silver-Tempest/Blaziken-V-SITTG14",
			want:    0,
		},
		{
			desc:    "nor does an ordinary product",
			name:    "Miraidon ex ",
			website: "/en/Pokemon/Products/Singles/Southeast-Asia-Promos/Miraidon-ex-SEASVI-081",
			want:    0,
		},
		{
			// The name wins where both carry one, the name being the
			// marketplace's own wording rather than a slug of it.
			desc:    "the name wins over the address",
			name:    "Hand of Death (V.2)",
			website: "/en/Magic/Products/Singles/Fourth-Edition/Hand-of-Death-V9-4ED",
			want:    2,
		},
		{
			desc: "and a product carrying neither answers none",
			name: "Altar's Light",
			want: 0,
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			product := Product{Name: tt.name, Website: tt.website}
			if got := ProductVersion(&product); got != tt.want {
				t.Errorf("ProductVersion(%q, %q) = %d, want %d", tt.name, tt.website, got, tt.want)
			}
		})
	}
}
