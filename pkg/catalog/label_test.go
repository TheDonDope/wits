package catalog

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/TheDonDope/wits/pkg/journal"
)

func TestReadLabel(t *testing.T) {
	t.Run("APharmacysOwnLabel", func(t *testing.T) {
		l := ReadLabel(`Apotheke am Markt
Hauptstr. 12/1, 10115 Berlin
Tel. 030 123456
Patient: Max Mustermann
Cannabisblüten, unzerkleinert
Enua 22/1 Wedding Cake
THC 22 % · CBD < 1 %
Inhalt: 10 g
Ch.-B.: 2310/22
Verwendbar bis: 03/2027
Hergestellt am 12.10.2026`)

		assert.Equal(t, "Enua 22/1 Wedding Cake", l.Name, "Should read the product line, not the address's house number")
		assert.Equal(t, 22.0, l.THC)
		assert.Equal(t, 1.0, l.CBD)
		assert.Equal(t, journal.CG(10), l.Grams, "Should read the amount off the line naming the contents")
	})

	t.Run("APharmacySystemsArticleInCapitals", func(t *testing.T) {
		l := ReadLabel("PZN 18234567\nPEDANIOS 22/1 DNK GHOST TRAIN HAZE CANNABISBLÜTEN 10 G\n1 St.")

		assert.Equal(t, "Pedanios 22/1 DNK Ghost Train Haze", l.Name,
			"Should give words the case a name is kept in, leave codes in capitals, and drop what kind of thing it is")
		assert.Equal(t, journal.CG(10), l.Grams)
	})

	t.Run("TheDeliveryColumnTheWorkbookWasCopiedFrom", func(t *testing.T) {
		l := ReadLabel("Liefermenge 420 Evolution 27/1 Ice Cream Cake (g) 15")

		assert.Equal(t, "420 Evolution 27/1 Ice Cream Cake", l.Name, "Should drop the heading and the unit")
		assert.Equal(t, 27.0, l.THC)
		assert.Equal(t, journal.CG(15), l.Grams, "Should read the number the heading's unit is for")
	})

	t.Run("ARatioOnALineOfItsOwn", func(t *testing.T) {
		l := ReadLabel("Enua Wedding Cake\n22/1\n15 g")

		assert.Equal(t, "Enua Wedding Cake 22/1", l.Name, "Should join the ratio to the name above it")
		assert.Equal(t, journal.CG(15), l.Grams)
	})

	t.Run("OnlyPercentages", func(t *testing.T) {
		l := ReadLabel("Cannabisblüten Bedrocan\nTHC: 22,0 %  CBD < 1,0 %\nMenge 5")

		assert.Empty(t, l.Name, "Should not make up a product line")
		assert.Equal(t, 22.0, l.THC, "Should still read the ratio off the percentages")
		assert.Equal(t, 1.0, l.CBD, "Less than 1 % is the 1 in 22/1")
		assert.Equal(t, journal.CG(5), l.Grams, "Should read a bare number on the line naming the quantity")
	})

	t.Run("MilligramsAreNotGrams", func(t *testing.T) {
		l := ReadLabel("Tilray 18/1 Spotlight Porto Blueberry\nTHC 180 mg/g\nEinzeldosis 50 mg\nInhalt 7,5 g")

		assert.Equal(t, journal.CG(7.5), l.Grams, "Should read past the milligrams, with a decimal comma")
		assert.Equal(t, "Tilray 18/1 Spotlight Porto Blueberry", l.Name)
	})

	t.Run("APercentageOnTheProductLine", func(t *testing.T) {
		l := ReadLabel("Canopy 21/1 Jack Haze THC 21 % CBD <1 % 10g")

		assert.Equal(t, "Canopy 21/1 Jack Haze", l.Name, "Should leave the contents off the name")
		assert.Equal(t, journal.CG(10), l.Grams)
	})

	t.Run("DatesAreNotRatios", func(t *testing.T) {
		l := ReadLabel("Abgabe 12/10/2026\nVerwendbar bis 03/2027")

		assert.Empty(t, l.Name)
		assert.Zero(t, l.THC)
		assert.Zero(t, l.Grams)
	})

	t.Run("AnAmountThatCannotBeOnePrescription", func(t *testing.T) {
		l := ReadLabel("Enua 22/1 Wedding Cake\nLagerbestand 500 g")

		assert.Zero(t, l.Grams, "Should not take 500 g for what was dispensed")
	})

	t.Run("NothingToRead", func(t *testing.T) {
		assert.Equal(t, Label{}, ReadLabel(""))
	})
}

func TestMatching(t *testing.T) {
	c := &Catalog{}
	for _, name := range []string{
		"Enua 22/1 Wedding Cake", "Khiron 20/1 Wedding Cake", "Quantity IMC 20/1 Hindu Kush",
		"Pedanios 22/1 DNK Ghost Train Haze", "enua 22/1 Black Cherry Punch",
	} {
		p := Parse(name)
		p.Slug = NewHandle(p, c.Handles())
		assert.NoError(t, c.Add(p))
	}
	name := func(p *Product) string {
		if p == nil {
			return ""
		}
		return p.Name
	}

	for label, want := range map[string]string{
		"ENUA 22/1 WEDDING CAKE":         "Enua 22/1 Wedding Cake",
		"Enua Wedding Cake 22/1":         "Enua 22/1 Wedding Cake",
		"IMC 20/1 Hindu Kush":            "Quantity IMC 20/1 Hindu Kush",
		"Pedanios 22/1 Ghost Train Haze": "Pedanios 22/1 DNK Ghost Train Haze",
		"Enua 22/1 Wedding Cake CA":      "Enua 22/1 Wedding Cake",
		"Khiron 22/1 Wedding Cake":       "",
		"Enua 25/1 Wedding Cake":         "",
		"Enua 22/1 Lemonade Haze":        "",
		"":                               "",
	} {
		t.Run(label, func(t *testing.T) {
			assert.Equal(t, want, name(c.Matching(label)))
		})
	}
}
