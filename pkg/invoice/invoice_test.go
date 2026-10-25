package invoice

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sample is the shape of a German pharmacy's order confirmation as pdf.js
// gives its text, a row a line — the rows that matter, and the kind of rows
// around them that must not be mistaken for them. Names and addresses are
// made up; the real invoice holds a patient's, and stays out of the repository.
const sample = `BESTELLDATUM
28.08.2026
Apotheke am Beispielplatz
Musterstraße 1
12345 Musterstadt
RECHNUNGSADRESSE	LIEFERADRESSE	KUNDENINFORMATIONEN
Erika Mustermann	Selbstabholung	Geb.-Datum: 01.01.1990
BESTELL-NR.
2608-ABC-B000001
Name	Menge	Preis	Gesamtpreis
All Nations GM 29/1 Mac Doughnut	20 g	5,84 €	116,80 €
NICE 32/1 MOS Morning Sun	20 g	5,80 €	116,00 €
Sumo PAB 25/1 Papaya Bomb	20 g	5,95 €	119,00 €
Versandkosten	0,00 €
Gesamtbetrag exkl. USt	295,63 €
USt 19 %	56,17 €
Gesamtbetrag inkl. USt	351,80 €
Montag bis Freitag 8.00 - 19.00
Steuernummer: 240/204/00000`

func TestRead(t *testing.T) {
	inv := Read(sample)

	assert.Equal(t, time.Date(2026, 8, 28, 0, 0, 0, 0, time.Local), inv.Date, "The order date, not the birth date")
	assert.Equal(t, "2608-ABC-B000001", inv.Number)
	require.Len(t, inv.Lines, 3)
	assert.Equal(t, Line{Name: "All Nations GM 29/1 Mac Doughnut", Grams: 20, PerGram: 584, Total: 11680}, inv.Lines[0])
	assert.Equal(t, Line{Name: "Sumo PAB 25/1 Papaya Bomb", Grams: 20, PerGram: 595, Total: 11900}, inv.Lines[2])
	assert.Equal(t, int64(29563), inv.Net)
	assert.Equal(t, int64(5617), inv.VAT)
	assert.Equal(t, 19, inv.Rate)
	assert.Equal(t, int64(35180), inv.Gross)
	assert.Equal(t, inv.Gross, inv.Sum(), "The lines are priced with VAT: they add up to the gross total")
}

func TestReadsWhatAnotherLayoutPrints(t *testing.T) {
	inv := Read("Rechnungsdatum\n01.02.2026\nX 22/1 Y   12,5 g   10,00   125,00 €\nGesamtbetrag inkl. MwSt  1.250,00 €")

	assert.Equal(t, time.Date(2026, 2, 1, 0, 0, 0, 0, time.Local), inv.Date)
	require.Len(t, inv.Lines, 1)
	assert.Equal(t, Line{Name: "X 22/1 Y", Grams: 12.5, PerGram: 1000, Total: 12500}, inv.Lines[0],
		"A decimal comma in the grams, no euro sign on the price")
	assert.Equal(t, int64(125000), inv.Gross, "Thousands are dotted")
}

func TestNothingToRead(t *testing.T) {
	inv := Read("rezeptkopie.png\n\nVersandkosten 0,00 €")

	assert.Empty(t, inv.Lines)
	assert.True(t, inv.Date.IsZero())
}
