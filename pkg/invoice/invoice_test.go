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

// shipped is the other pharmacy's invoice as pdf.js gives its text: a product
// over three rows under its PZN, prices with a decimal point, the date with
// slashes, and shipping charged. The person is made up; the products and
// prices are as printed.
const shipped = `Versandapotheke - Beispielweg 1 - 12345 Musterstadt
Erika Mustermann	Rechnung Nr.	MYCA-64272
Musterstraße 1
Bestellnummer:	MYCA-64272
12345 Musterstadt	Rechnungsdatum:	30/12/2024
Gesamt Brutto:	171.54€
Produkt	Anzahl (g)	Preis
Steuern
Cannabis 1A 16/1 White Widow - PZN: 18323407
Weeco Pharma GmbH - THC: 16%, CBD: < 1% - Blüte -	20g	3.76€	75.20€
Hybrid - White Widow
MAC 1+ - PZN: 18046066
Cantourage - THC: 25%, CBD: < 1% - Blüte - Hybrid - Alien	5g	10.88€	54.40€
Cookies x Columbian Starfighter
Tilray THC18 Spotlight Porto Blueberry - PZN: 16767525
5g	5.99€	29.95€
Tilray - THC: 18%, CBD: < 1% - Blüte - Indica - Blueberry
GO! Express und Logistics	10.07€	11.99€
Zwischensumme	144.15€
MwSt. 19%	27.39€
Gesamt Brutto	171.54€
Bereits bezahlt	0.00€
Offener Betrag	171.54€`

func TestReadsAProductOverSeveralRows(t *testing.T) {
	inv := Read(shipped)

	assert.Equal(t, time.Date(2024, 12, 30, 0, 0, 0, 0, time.Local), inv.Date, "Day first, with slashes")
	assert.Equal(t, "MYCA-64272", inv.Number)
	require.Len(t, inv.Lines, 3)
	assert.Equal(t, Line{Name: "Cannabis 1A 16/1 White Widow", Grams: 20, PerGram: 376, Total: 7520}, inv.Lines[0],
		"The name by its PZN; its strain is already in it")
	assert.Equal(t, Line{Name: "MAC 1+ 25/1 Alien Cookies x Columbian Starfighter", Grams: 5, PerGram: 1088, Total: 5440}, inv.Lines[1],
		"The strength and the strain from the description, across its rows")
	assert.Equal(t, "Tilray THC18 Spotlight Porto Blueberry 18/1", inv.Lines[2].Name, "Grams on a row of their own")
	assert.Equal(t, []Other{{Name: "GO! Express und Logistics", Total: 1199}}, inv.Other, "Shipping is no strain")
	assert.Equal(t, int64(17154), inv.Gross)
	assert.Equal(t, int64(14415), inv.Net)
	assert.Equal(t, inv.Gross, inv.Sum()+inv.LeftOut(), "The strains and the shipping are the invoice's total")
}

func TestReadsTheDateAndNumberBesideTheirLabels(t *testing.T) {
	reserved := Read("Geb.: 01.01.1990\nReservierungs ID:\t2506-ABC-0181\tReservierungsdatum:\t09.06.2025\n" +
		"Produkt\tMenge\tPreis\tGesamtpreis\nEnua 22/1 SAM CA Super Alien Mints\t10g\t5,95 €\t59,50 €\nVersandkosten\t1\t0,00 €\t0,00 €")
	billed := Read("Geb.: 01.01.1990 \x00 Zahlungen vor Ort\nRechnung Nr.\t2606-ABC-1637\tRechnungsdatum\t15.06.2026\n" +
		"Produkt\tMenge\tPreis\tGesamtpreis\nenua 22/1 WDC Wedding Cake\t5 g\t4,95 €\t24,75 €")

	assert.Equal(t, time.Date(2025, 6, 9, 0, 0, 0, 0, time.Local), reserved.Date, "Not the birth date")
	assert.Equal(t, "2506-ABC-0181", reserved.Number)
	assert.Len(t, reserved.Lines, 1)
	assert.Empty(t, reserved.Other, "Free shipping is nothing left out")
	assert.Equal(t, time.Date(2026, 6, 15, 0, 0, 0, 0, time.Local), billed.Date, "Past a NUL in the text")
	assert.Equal(t, "2606-ABC-1637", billed.Number)
	assert.Len(t, billed.Lines, 1)
}

func TestLeavesOutWhatIsNotFlowerByTheGram(t *testing.T) {
	inv := Read("Name\tMenge\tPreis\tGesamtpreis\n" +
		"Curaleaf WPT Lösung in Curaleaf Vape Kartusche (1,0 ml)\t4 St.\t49,20 €\t196,80 €\n" +
		"Pedanios 25/1 FLH-CA Frozen Lemon Haze\t20 g\t5,80 €\t116,00 €\nVersandkosten\t0,00 €\n" +
		"Gesamtbetrag exkl. USt\t262,86 €\nUSt 19 %\t49,94 €\nGesamtbetrag inkl. USt\t312,80 €")

	require.Len(t, inv.Lines, 1, "A vape by the piece is not a jar")
	assert.Equal(t, "Pedanios 25/1 FLH-CA Frozen Lemon Haze", inv.Lines[0].Name)
	assert.Equal(t, []Other{{Name: "Curaleaf WPT Lösung in Curaleaf Vape Kartusche (1,0 ml)", Total: 19680}}, inv.Other)
	assert.Equal(t, inv.Gross, inv.Sum()+inv.LeftOut())
}
