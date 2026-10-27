package prices

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
)

func testLog(t *testing.T) *Log {
	t.Helper()
	return Open(filepath.Join(t.TempDir(), "prices.ndjson"))
}

var at = time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)

func TestParseEuros(t *testing.T) {
	for in, want := range map[string]int64{"120.50": 12050, "120,50 €": 12050, "€120": 12000, "0.01": 1, " 85 EUR ": 8500} {
		got, err := ParseEuros(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	for _, in := range []string{"", "abc", "-5", "0", "1.005"} {
		_, err := ParseEuros(in)
		assert.Error(t, err, "Should refuse %q", in)
	}
}

func TestAppendVerifyAndLatest(t *testing.T) {
	l := testLog(t)
	first, err := l.Append(Price{Purchase: "fill", At: at, Cents: 17000})
	require.NoError(t, err)
	_, err = l.Append(Price{Purchase: "fill", At: at, Cents: 18000, Note: "the label said 180"})
	require.NoError(t, err)
	_, err = l.Append(Price{Purchase: "", Cents: 100})
	assert.Error(t, err, "Should refuse a price for nothing")

	require.NoError(t, l.Verify())
	all, err := l.All()
	require.NoError(t, err)
	assert.Len(t, all, 2, "Should keep the price it superseded")
	assert.Equal(t, int64(18000), Latest(all)["fill"].Cents, "Should cost what it was priced at last")

	chained, err := Chain([]Price{{Purchase: "fill", At: at, Cents: 17000}})
	require.NoError(t, err)
	assert.Equal(t, first.Hash, chained[0].Hash, "Should hash as appending did")

	data, err := os.ReadFile(l.Path())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(l.Path(), []byte(string(data[:len(data)-30])+"0000000000000000000000000000\"}\n"), 0600))
	assert.ErrorIs(t, l.Verify(), ErrBrokenChain, "Should notice a hash that is not its own")
}

func TestRebase(t *testing.T) {
	phone, deck := testLog(t), testLog(t)
	_, err := phone.Append(Price{Purchase: "theirs", At: at, Cents: 100})
	require.NoError(t, err)
	_, err = deck.Append(Price{Purchase: "ours", At: at, Cents: 200})
	require.NoError(t, err)
	theirs, err := phone.All()
	require.NoError(t, err)

	fetched, rechained, err := deck.Rebase(theirs)

	require.NoError(t, err)
	assert.Equal(t, 1, fetched)
	assert.Equal(t, 1, rechained, "Should chain its own again")
	require.NoError(t, deck.Verify())
	ours, err := deck.All()
	require.NoError(t, err)
	require.Len(t, ours, 2)
	assert.Equal(t, theirs[0], ours[0], "Should keep the other copy's price as it is")
	assert.Equal(t, "ours", ours[1].Purchase, "Should still price the purchase it priced")
}

func TestAnalyse(t *testing.T) {
	buy := func(hash, product string, g float64, day int) journal.Event {
		return journal.Event{Type: journal.Purchase, Product: product, Grams: g, From: journal.External,
			To: journal.Storage, OccurredAt: at.AddDate(0, 0, day), Hash: hash}
	}
	grind := journal.Event{Type: journal.Grind, Product: "wcake", Grams: 2, From: journal.Storage, To: journal.Stash,
		OccurredAt: at.AddDate(0, 0, 1), Hash: "g"}
	sesh := journal.Event{Type: journal.Sesh, Product: "wcake", Grams: 0.5, From: journal.Stash, To: journal.Consumed,
		OccurredAt: at.AddDate(0, 0, 2), Hash: "s"}
	undone := buy("u", "lemon", 10, 0)
	undo := journal.Event{Type: journal.Adjust, Product: "lemon", Grams: 10, From: journal.Storage, To: journal.External,
		OccurredAt: at.AddDate(0, 0, 1), Reverts: "u", Hash: "x"}
	state := ledger.Fold([]journal.Event{buy("a", "wcake", 20, 0), buy("b", "papaya", 20, 0), undone, undo, grind, sesh})
	latest := map[string]Price{"a": {Purchase: "a", Cents: 17000}}
	ratings := []effects.Rating{{Session: "s", Scores: map[effects.Dimension]int{effects.Overall: 4}}}

	c := Analyse(state, latest, ratings)

	assert.Equal(t, int64(17000), c.Cents)
	assert.Equal(t, 20.0, c.Priced)
	assert.Equal(t, 1, c.Unpriced, "Should count the purchase without a price, and not the undone one")
	assert.InDelta(t, 850, c.PerGram(), 1e-9, "8.50 € a gram")
	require.Len(t, c.Fills, 1)
	assert.Equal(t, 40.0, c.Fills[0].Dispensed, "Should leave the undone purchase out")
	assert.Equal(t, []Month{{Month: "2026-08", Cents: 17000, Grams: 20}}, c.Months)
	assert.Equal(t, 1, c.Helped)
	assert.InDelta(t, 425, c.PerHelped(), 1e-9, "0.5 g at 8.50 € a gram")
}

func TestAPriceNotToBeHad(t *testing.T) {
	assert.NoError(t, Price{Purchase: "abc", Unknown: true}.Validate())
	assert.Error(t, Price{Purchase: "abc", Unknown: true, Cents: 100}.Validate(), "None to be had, so no amount")
	assert.Error(t, Price{Purchase: "abc"}.Validate(), "A price is still more than nothing")

	b, err := json.Marshal(Price{Purchase: "abc", Cents: 100})
	require.NoError(t, err)
	assert.NotContains(t, string(b), "unknown", "A price written before it encodes, and so hashes, as it did")
}

func TestTheSameStrainKeysAlikeUnderItsNames(t *testing.T) {
	producers := map[string]bool{"all": true, "nations": true}
	key := func(maker, cultivar string) string {
		k, _ := StrainOf(Product{Maker: maker, Cultivar: cultivar}, producers)
		return k
	}
	assert.Equal(t, "mac1", key("420 Evolution", "CA MAC: MAC1"))
	assert.Equal(t, "mac1", key("420 Evolution", "MAC 1"), "A number after a short name is part of it")
	assert.Equal(t, "mac1", key("Cantourage", "MAC 1+"))
	assert.Equal(t, "mac3", key("Barongo", "MAC3"), "MAC 3 is another cut")
	assert.Equal(t, "hindu kush", key("IMC", "T01 Hindu Kush"), "A pharmacy's code is no part of the strain")
	assert.Equal(t, "hindu kush", key("Quantity IMC", "Hindu Kush"))
	assert.Equal(t, key("Cannamedical", "Lemon Cookie"), key("Cannamedical", "Lemon Cookies"))
	assert.Equal(t, "ca gg", key("Quantity 420 Natural", "CA GG"), "A name of codes alone keeps them")

	k, name := StrainOf(Product{Name: "All Nations Mac Doughnut GM 25/1", Maker: "All Nations Mac Doughnut GM"}, producers)
	assert.Equal(t, "doughnut mac", k, "No cultivar: the name, less the producers known and the strength")
	assert.Equal(t, "Mac Doughnut", name)
}
