package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlugify(t *testing.T) {
	for name, want := range map[string]string{
		"Enua 22/1 Wedding Cake (g)":                         "enua-wedding-cake",
		"Cantourage 25/1 MAC1+ (g)":                          "cantourage-mac1",
		"420 Evolution 27/1 Ice Cream Cake":                  "420-evolution-ice-cream-cake",
		"CNBS FK 30/1: Florida Kush.":                        "cnbs-fk-florida-kush",
		"Cannamedical Hybrid Ultra DK 28/1 Ghost Train Haze": "cannamedical-hybrid-ultra-dk-ghost-train-haze",
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, want, Slugify(name), "Should be a stable, typeable identifier")
		})
	}
}

func TestParse(t *testing.T) {
	t.Run("TheUsualConvention", func(t *testing.T) {
		p := Parse("Enua 22/1 Wedding Cake (g)")

		assert.Equal(t, "Enua 22/1 Wedding Cake", p.Name, "Should keep the name as written, minus the unit")
		assert.Equal(t, "Enua", p.Manufacturer, "Should read the manufacturer")
		assert.Equal(t, "Wedding Cake", p.Cultivar, "Should read the cultivar")
		assert.Equal(t, 22.0, p.THC, "Should read the THC percentage")
		assert.Equal(t, 1.0, p.CBD, "Should read the CBD percentage")
	})

	t.Run("KeepsAnInlinedProductLineWithTheManufacturer", func(t *testing.T) {
		p := Parse("Cannamedical Hybrid Ultra DK 28/1 Ghost Train Haze (g)")

		assert.Equal(t, "Cannamedical Hybrid Ultra DK", p.Manufacturer, "Should not try to split the product line out")
		assert.Equal(t, "Ghost Train Haze", p.Cultivar, "Should read the cultivar")
	})

	t.Run("TrimsSeparators", func(t *testing.T) {
		p := Parse("CNBS FK 30/1: Florida Kush.")

		assert.Equal(t, "CNBS FK", p.Manufacturer, "Should trim the trailing colon")
		assert.Equal(t, "Florida Kush", p.Cultivar, "Should trim the trailing period")
	})

	t.Run("NameWithoutARatio", func(t *testing.T) {
		p := Parse("Liefermenge Indica")

		assert.Equal(t, "Liefermenge Indica", p.Name, "Should keep the name")
		assert.Empty(t, p.Manufacturer, "Should not invent a manufacturer")
		assert.Zero(t, p.THC, "Should not invent a THC value")
	})
}

func TestFind(t *testing.T) {
	c := &Catalog{}
	require.NoError(t, c.Add(&Product{Name: "Enua 22/1 Wedding Cake"}))
	require.NoError(t, c.Add(&Product{Name: "Cannamedical 28/1 Lemon Cookie"}))
	require.NoError(t, c.Add(&Product{Name: "Khiron 20/1 Wedding Cake"}))

	t.Run("BySlug", func(t *testing.T) {
		p, err := c.Find("enua-wedding-cake")
		require.NoError(t, err)
		assert.Equal(t, "Enua 22/1 Wedding Cake", p.Name, "Should resolve an exact slug")
	})

	t.Run("ByDisplayName", func(t *testing.T) {
		p, err := c.Find("Cannamedical 28/1 Lemon Cookie")
		require.NoError(t, err)
		assert.Equal(t, "cannamedical-lemon-cookie", p.Slug, "Should resolve an exact display name")
	})

	t.Run("ByUniqueSubstring", func(t *testing.T) {
		p, err := c.Find("lemon")
		require.NoError(t, err)
		assert.Equal(t, "cannamedical-lemon-cookie", p.Slug, "Should resolve a unique substring for fast entry")
	})

	t.Run("RefusesToGuessBetweenTwoMatches", func(t *testing.T) {
		_, err := c.Find("wedding")

		assert.ErrorIs(t, err, ErrAmbiguous, "Should not pick one of two products for you")
		assert.Contains(t, err.Error(), "enua-wedding-cake", "Should say which products matched")
		assert.Contains(t, err.Error(), "khiron-wedding-cake", "Should say which products matched")
	})

	t.Run("Unknown", func(t *testing.T) {
		_, err := c.Find("blueberry")
		assert.ErrorIs(t, err, ErrNotFound, "Should report an unknown reference")
	})

	t.Run("Empty", func(t *testing.T) {
		_, err := c.Find("")
		assert.ErrorIs(t, err, ErrNotFound, "Should not match everything on an empty reference")
	})
}

func TestAddedAtIsToTheSecond(t *testing.T) {
	c := &Catalog{}
	require.NoError(t, c.Add(&Product{Name: "Enua 22/1 Wedding Cake"}))

	assert.Zero(t, c.Products[0].AddedAt.Nanosecond(),
		"Should be to the second, so a bundle round trip leaves the catalog unchanged")
}

func TestAdd(t *testing.T) {
	c := &Catalog{}
	require.NoError(t, c.Add(&Product{Name: "Enua 22/1 Wedding Cake"}))

	err := c.Add(&Product{Name: "Enua 22/1 Wedding Cake"})

	assert.ErrorIs(t, err, ErrDuplicate, "Should refuse to shadow an existing product")
	assert.Len(t, c.Products, 1, "Should not have added it twice")
}

func TestLoadAndSave(t *testing.T) {
	t.Run("RoundTrip", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "products.yml")
		c := &Catalog{}
		require.NoError(t, c.Add(Parse("Enua 22/1 Wedding Cake (g)")))
		require.NoError(t, c.Save(path))

		loaded, err := Load(path)
		require.NoError(t, err)

		require.Len(t, loaded.Products, 1)
		assert.Equal(t, "Enua", loaded.Products[0].Manufacturer, "Should survive the round trip")
		assert.Equal(t, 22.0, loaded.Products[0].THC, "Should survive the round trip")
	})

	t.Run("MissingFileIsAnEmptyCatalog", func(t *testing.T) {
		c, err := Load(filepath.Join(t.TempDir(), "products.yml"))

		assert.NoError(t, err, "Should not fail before the first product is added")
		assert.Empty(t, c.Products, "Should be empty")
	})

	t.Run("MalformedFileIsAnError", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "products.yml")
		require.NoError(t, os.WriteFile(path, []byte("products: [oh dear\n"), 0600))

		_, err := Load(path)

		assert.Error(t, err, "Should not mistake a broken catalog for an empty one")
	})
}
