package cardmarket

import (
	"context"
	"testing"
)

// The zero query asks for every listing, so it sends no filter at all.
func TestArticleQueryZero(t *testing.T) {
	v, err := ArticleQuery{}.values()
	if err != nil {
		t.Fatalf("values() = %v", err)
	}
	if len(v) != 0 {
		t.Errorf("zero query sent %s", v.Encode())
	}
}

// Every field lands under the name the documentation gives it, spelled the
// way the API reads it.
func TestArticleQueryValues(t *testing.T) {
	v, err := ArticleQuery{
		UserType:      UserTypeCommercial,
		MinUserScore:  UserScoreGood,
		MinCondition:  ConditionNearMint,
		Language:      LanguageJapanese,
		SellerCountry: CountryGermany,
		Foil:          Only,
		FirstEd:       Only,
		ReverseHolo:   None,
		Signed:        None,
		Altered:       None,
		MinAvailable:  4,
	}.values()
	if err != nil {
		t.Fatalf("values() = %v", err)
	}
	want := "idLanguage=7&isAltered=false&isFirstEd=true&isFoil=true&isReverseHolo=false" +
		"&isSigned=false&minAvailable=4&minCondition=NM&minUserScore=3&sellerCountry=7&userType=commercial"
	if got := v.Encode(); got != want {
		t.Errorf("values() =\n %s\nwant\n %s", got, want)
	}
}

// A value outside the package's constants would be ignored by the API and
// answered unfiltered, so it is refused before anything is sent.
func TestArticleQueryRefusesUnknownValues(t *testing.T) {
	tests := map[string]ArticleQuery{
		"user type":     {UserType: "reseller"},
		"user score":    {MinUserScore: 6},
		"condition":     {MinCondition: "Excellent"},
		"language":      {Language: 99},
		"country":       {SellerCountry: 99},
		"filter":        {Foil: 3},
		"negative min":  {MinAvailable: -1},
		"filter below":  {Signed: -1},
		"blank country": {SellerCountry: 32},
	}
	for name, query := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := query.values(); err == nil {
				t.Errorf("values() accepted %+v", query)
			}
		})
	}
}

// Articles refuses such a query without spending a request on it.
func TestArticlesRefusesBeforeRequesting(t *testing.T) {
	mkm := NewClient("test-token", "test-secret")
	_, _, _, err := mkm.Articles(context.Background(), 1, ArticleQuery{Language: 99}, 0, 10)
	if err == nil {
		t.Fatal("Articles() accepted an unknown language")
	}
	if n := mkm.RequestNo(); n != 0 {
		t.Errorf("made %d requests, want none", n)
	}
}
