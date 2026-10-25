package api

import (
	"encoding/json"
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

// matched reads an invoice's text and names, for each of its lines, the
// product of the purchase it matched in the workbook, or "".
func matched(t *testing.T, s *Server, text string) (*invoiceMatch, []string) {
	t.Helper()
	body, err := json.Marshal(map[string]string{"text": text})
	require.NoError(t, err)
	inv := decodeInto[invoiceReading](t, call(t, s, "POST", "/api/invoice", string(body)))
	if inv.Match == nil {
		return nil, nil
	}
	ws := must(t, s.snapshot)
	names := make([]string, len(inv.Match.Lines))
	for i, h := range inv.Match.Lines {
		if h != "" {
			e, err := ws.Recorder.Find(h)
			require.NoError(t, err)
			names[i] = e.Product
		}
	}
	return inv.Match, names
}

// The invoices below are as the two pharmacies print them, the products and
// dates as on the real ones, the person left out.
func TestAnInvoiceMatchesItsFillByWhatItShares(t *testing.T) {
	s, _ := workbook(t)

	m, got := matched(t, s, "Reservierungs ID:\t2506-ABC-0679\tReservierungsdatum:\t26.06.2025\n"+
		"Produkt\tMenge\tPreis\tGesamtpreis\nCantourage MAC 1+\t15g\t10,69 €\t160,35 €\n"+
		"Cookies 22/1 Hawaiian Rain\t10g\t7,48 €\t74,80 €\nCannamedical CM 28/1 DM Diamond Mints\t10g\t8,89 €\t88,90 €")
	require.NotNil(t, m)
	assert.Equal(t, 19, m.Fill)
	assert.Equal(t, []string{"cantourage-mac-1-251", "cansativa-hawaiian-rain-221", "cannamedical-diamond-mints-281"}, got,
		"The strain the invoice names under its breeder is the jar named for its distributor")

	m, got = matched(t, s, "Reservierungs ID:\t2505-ABC-0322\tReservierungsdatum:\t13.05.2025\n"+
		"Produkt\tMenge\tPreis\tGesamtpreis\nCannamedical Hybrid Ultra DK 28/1 Ghost\t10g\t8,89 €\t88,90 €\n"+
		"420 Evolution 22/1 CA MAC\t10g\t8,95 €\t89,50 €")
	require.NotNil(t, m)
	assert.Equal(t, 17, m.Fill)
	assert.Equal(t, 0, m.Days)
	assert.Equal(t, []string{"cannamedical-hybrid-ultra-dk-ghost-train-haze-281", "420-evolution-mac-1-221"}, got,
		"A name the invoice cuts short is still its jar")

	m, got = matched(t, s, "Rechnung Nr.\tMYCA-65055\nRechnungsdatum:\t04/12/2024\nProdukt\tAnzahl (g)\tPreis\n"+
		"Remexian 14/1 White Widow - PZN: 08000604\nRemexian Pharma - THC: 14%, CBD: < 1% - Blüte - Sativa\t10g\t3.48€\t34.80€\n- White Widow\n"+
		"Tilray THC18 Spotlight Porto Blueberry - PZN: 16767525\n5g\t5.99€\t29.95€\nTilray - THC: 18%, CBD: < 1% - Blüte - Indica - Blueberry")
	require.NotNil(t, m)
	assert.Equal(t, 13, m.Fill)
	assert.Equal(t, 7, m.Days, "Shipped: a week from the order to the jar")
	assert.Equal(t, []string{"", "tilray-spotlight-porto-blueberry-181"}, got,
		"A White Widow at 14/1 is not the fill's 16/1")

	m, _ = matched(t, s, "BESTELLDATUM\n29.09.2026\nName\tMenge\tPreis\tGesamtpreis\n"+
		"Pedanios 25/1 FLH-CA Frozen Lemon Haze\t20 g\t5,80 €\t116,00 €")
	assert.Nil(t, m, "A pickup the workbook does not hold is a new fill")
}
