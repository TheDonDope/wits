package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/importer"
	"github.com/TheDonDope/wits/pkg/prices"
	"github.com/TheDonDope/wits/pkg/repo"
)

// workbook is a server over the committed workbook, imported.
func workbook(t *testing.T) (*Server, *repo.Repo) {
	t.Helper()
	r, err := repo.Init(t.TempDir())
	require.NoError(t, err)
	result, err := importer.Read("../../assets/Tracking.2022.cleaned.xlsx")
	require.NoError(t, err)
	require.NoError(t, importer.Commit(r, result, nil))
	return New(r, ""), r
}

// invoiceText is a pharmacy's order confirmation as pdf.js gives its text:
// the product lines and totals as the real one prints them, everything that
// would name the patient left out.
const invoiceText = `BESTELLDATUM\n28.08.2026\nGeb.-Datum: 01.01.1990\nBESTELL-NR.\n2608-ABC-B000001\n` +
	`Name\tMenge\tPreis\tGesamtpreis\n` +
	`All Nations GM 29/1 Mac Doughnut\t20 g\t5,84 €\t116,80 €\n` +
	`NICE 32/1 MOS Morning Sun\t20 g\t5,80 €\t116,00 €\n` +
	`Sumo PAB 25/1 Papaya Bomb\t20 g\t5,95 €\t119,00 €\n` +
	`Gesamtbetrag exkl. USt\t295,63 €\nUSt 19 %\t56,17 €\nGesamtbetrag inkl. USt\t351,80 €`

func TestReadsAnInvoiceAgainstTheJars(t *testing.T) {
	s, _ := workbook(t)

	w := call(t, s, "POST", "/api/invoice", `{"text": "`+invoiceText+`"}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	inv := decodeInto[invoiceReading](t, w)
	assert.Equal(t, "2026-08-28", inv.Date)
	assert.Equal(t, "2608-ABC-B000001", inv.Number)
	require.Len(t, inv.Lines, 3)
	assert.Equal(t, invoiceLine{Name: "All Nations 29/1 Mac Doughnut", Slug: "all-nations-mac-doughnut-291",
		Printed: "All Nations GM 29/1 Mac Doughnut", Grams: 20, PerGram: 584, Total: 11680}, inv.Lines[0],
		"The jar at the invoice's strength, not the GM 25/1 beside it")
	assert.Equal(t, "nice-morning-sun-321", inv.Lines[1].Slug)
	assert.Equal(t, "sumo-papaya-bomb-251", inv.Lines[2].Slug)
	assert.Equal(t, int64(35180), inv.Gross)
	assert.Equal(t, inv.Gross, inv.Sum)

	assert.Equal(t, http.StatusUnprocessableEntity, call(t, s, "POST", "/api/invoice", `{"text": "rezeptkopie.png"}`).Code)
}

func TestRecordsAFillOfSeveralProducts(t *testing.T) {
	s, r := workbook(t)

	w := call(t, s, "POST", "/api/fill", `{"date": "2026-08-28", "note": "invoice 2608", "lines": [
		{"name": "All Nations 29/1 Mac Doughnut", "grams": 20, "total": "116,80"},
		{"name": "NICE 32/1 Morning Sun", "grams": 20, "total": "116,00"},
		{"name": "Sumo 25/1 Papaya Bomb", "grams": 20, "total": "119,00"}]}`)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	out := decodeInto[flushed](t, w)
	require.Len(t, out.Entries, 3)
	assert.Equal(t, "picked up 60.00 g of 3 products, for 351.80 €", out.Message,
		"An invoice's totals are taken as printed, VAT and all")
	paid, err := r.Prices().All()
	require.NoError(t, err)
	assert.Len(t, prices.Latest(paid), 3)
}

func TestAFillTypedInAddsVAT(t *testing.T) {
	s, _ := server(t, "")

	w := call(t, s, "POST", "/api/fill", `{"vat": true, "lines": [
		{"name": "Enua 22/1 Wedding Cake", "grams": 10, "per_gram": "5,00"},
		{"name": "Demecan 20/1 Typ 1", "grams": 5, "per_gram": "8,40"},
		{"name": "Aurora 18/1 Pink Kush", "grams": 5}]}`)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, "picked up 20.00 g of 3 products, 2 of them new, 109.48 € for the 2 with a price",
		decodeInto[flushed](t, w).Message, "10 g at 5 € and 5 g at 8.40 €, 92 € before VAT, 109.48 € with it")

	w = call(t, s, "POST", "/api/fill", `{"lines": [{"name": "Enua 22/1 Wedding Cake", "grams": 10, "per_gram": "5,00"}]}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Contains(t, decodeInto[flushed](t, w).Message, "for 50.00 €", "and none without it")

	w = call(t, s, "POST", "/api/fill", `{"lines": [{"name": "Enua 22/1 Wedding Cake", "grams": 10}, {"name": "", "grams": 1}]}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "line 2")
}

func TestLineCentsMatchesTheForm(t *testing.T) {
	cents, err := lineCents(fillLine{Grams: 30, PerGram: "2,05"}, true)
	require.NoError(t, err)
	assert.Equal(t, int64(7319), cents, "The figure fill-form.spec.ts pins the form to")
}
