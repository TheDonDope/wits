package api

import (
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestADryRunWritesNothingAndSaysWhatItWouldDo(t *testing.T) {
	s, r := server(t, "")
	journal, err := os.ReadFile(r.JournalPath())
	require.NoError(t, err)

	w := call(t, s, "POST", "/api/grind?dry=1", `{"product":"wcake","grams":0.5}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	p := decodeInto[preview](t, w)
	assert.Equal(t, "ground 0.50 g of Enua 22/1 Wedding Cake", p.Message, "Should say what it would record")
	require.Len(t, p.Jars, 1)
	assert.Equal(t, accounts{Storage: 18, Stash: 1.5, Consumed: 0.5}, p.Jars[0].Before)
	assert.Equal(t, accounts{Storage: 17.5, Stash: 2, Consumed: 0.5}, p.Jars[0].After)
	assert.Empty(t, p.Entries[0].Hash, "Should not chain what it did not write")

	after, err := os.ReadFile(r.JournalPath())
	require.NoError(t, err)
	assert.Equal(t, journal, after, "Should write nothing")

	w = call(t, s, "POST", "/api/grind?dry=1", `{"product":"wcake","grams":500}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "cannot take", "Should be refused in the recorder's words")
}

func TestADryFillPricesItsLinesAndAddsNoProduct(t *testing.T) {
	s, r := server(t, "")
	products, err := os.ReadFile(r.ProductsPath())
	require.NoError(t, err)

	w := call(t, s, "POST", "/api/fill?dry=1", `{"vat":true,"lines":[
		{"name":"Enua 22/1 Wedding Cake","grams":20,"total":"116.00"},
		{"name":"Aurora 26/1 Pink Kush","grams":15,"per_gram":"8.50"}]}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	p := decodeInto[preview](t, w)
	require.Len(t, p.Lines, 2)
	assert.Equal(t, previewLine{Product: "wcake-221", Cents: 11600}, p.Lines[0], "Should take an invoice's total as it is")
	assert.Equal(t, int64(15173), p.Lines[1].Cents, "Should price 15 g at 8.50 € with 19% VAT, rounded once")
	assert.Equal(t, int64(2423), p.Lines[1].VAT, "and say the VAT in it")
	assert.True(t, p.Lines[1].Added, "Should say the strain is new")
	assert.Contains(t, p.Message, "267.73", "Should say the fill's price")

	after, err := os.ReadFile(r.ProductsPath())
	require.NoError(t, err)
	assert.Equal(t, products, after, "Should add no product")
	_, err = os.Stat(r.PricesPath())
	assert.True(t, os.IsNotExist(err), "Should write no price")
}
