package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/prices"
)

func TestUnpricedComeByTheirFill(t *testing.T) {
	s, _ := workbook(t)

	all := decodeInto[[]unpricedEntry](t, call(t, s, "GET", "/api/unpriced?all=1", ""))

	fills := map[int]int{}
	for _, e := range all {
		fills[e.Fill]++
	}
	assert.Len(t, all, 88)
	assert.Len(t, fills, 30, "Each pickup is one fill")
	assert.Equal(t, 3, fills[30], "Three strains on the last prescription")
	assert.Equal(t, 2, fills[12])
}

func TestPricesAFillAtAPriceAGram(t *testing.T) {
	s, r := workbook(t)
	var last []unpricedEntry
	for _, e := range decodeInto[[]unpricedEntry](t, call(t, s, "GET", "/api/unpriced", "")) {
		if e.Fill == 30 {
			last = append(last, e)
		}
	}
	require.Len(t, last, 3)

	// A bad price refuses the whole fill: nothing is written.
	w := call(t, s, "POST", "/api/price/fill", `{"vat": true, "lines": [
		{"purchase": "`+last[0].Hash+`", "per_gram": "5,80"},
		{"purchase": "`+last[1].Hash+`", "per_gram": "cheap"}]}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	paid, err := r.Prices().All()
	require.NoError(t, err)
	assert.Empty(t, paid)

	// 20 g at 5,80 € with 19 % VAT is 138,04 €; at 5,84 €, 138,992 € rounds to
	// 138,99 €. The blank line keeps no price.
	w = call(t, s, "POST", "/api/price/fill", `{"vat": true, "lines": [
		{"purchase": "`+last[0].Hash+`", "per_gram": "5,80"},
		{"purchase": "`+last[1].Hash+`", "per_gram": "5.84"},
		{"purchase": "`+last[2].Hash+`", "per_gram": ""}]}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "277.03 € for 40.00 g, 2 purchases")
	latest := prices.Latest(must(t, r.Prices().All))
	assert.Equal(t, int64(13804), latest[last[0].Hash].Cents)
	assert.Equal(t, int64(13899), latest[last[1].Hash].Cents)
	assert.NotContains(t, latest, last[2].Hash)

	// Without VAT, the price a gram is the price.
	w = call(t, s, "POST", "/api/price/fill", `{"lines": [{"purchase": "`+last[2].Hash+`", "per_gram": "5,95"}]}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, int64(11900), prices.Latest(must(t, r.Prices().All))[last[2].Hash].Cents)

	assert.Equal(t, http.StatusBadRequest, call(t, s, "POST", "/api/price/fill",
		`{"lines": [{"purchase": "`+last[2].Hash+`", "per_gram": " "}]}`).Code, "Nothing to price")
}

func must[T any](t *testing.T, f func() (T, error)) T {
	t.Helper()
	v, err := f()
	require.NoError(t, err)
	return v
}

func TestAnInvoiceFindsTheFillItIsFor(t *testing.T) {
	s, _ := workbook(t)
	var last []unpricedEntry
	for _, e := range decodeInto[[]unpricedEntry](t, call(t, s, "GET", "/api/unpriced", "")) {
		if e.Fill == 30 {
			last = append(last, e)
		}
	}
	require.Len(t, last, 3)

	inv := decodeInto[invoiceReading](t, call(t, s, "POST", "/api/invoice", `{"text": "`+invoiceText+`"}`))

	require.NotNil(t, inv.Match, "Ordered on the 28th, picked up on the 29th: one fill")
	assert.Equal(t, 30, inv.Match.Fill)
	assert.Equal(t, 1, inv.Match.Days)
	assert.Equal(t, 0, inv.Match.Priced)
	byProduct := map[string]string{}
	for _, e := range last {
		byProduct[e.Product] = e.Hash
	}
	for i, l := range inv.Lines {
		assert.Equal(t, byProduct[l.Slug], inv.Match.Lines[i], l.Name)
	}

	// Priced from it, each line at the invoice's own total.
	w := call(t, s, "POST", "/api/price/fill", `{"lines": [
		{"purchase": "`+inv.Match.Lines[0]+`", "total": "116,80", "note": "invoice 2608"},
		{"purchase": "`+inv.Match.Lines[1]+`", "total": "116,00", "note": "invoice 2608"},
		{"purchase": "`+inv.Match.Lines[2]+`", "total": "119,00", "note": "invoice 2608"}]}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "351.80 € for 60.00 g, 3 purchases", "The invoice's total, to the cent")
	again := decodeInto[invoiceReading](t, call(t, s, "POST", "/api/invoice", `{"text": "`+invoiceText+`"}`))
	assert.Equal(t, 3, again.Match.Priced)

	far := strings.Replace(invoiceText, "28.08.2026", "28.08.2019", 1)
	assert.Nil(t, decodeInto[invoiceReading](t, call(t, s, "POST", "/api/invoice", `{"text": "`+far+`"}`)).Match,
		"No fill within a week of it: a new one")
}
