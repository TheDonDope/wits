package catalog

import (
	"os"
	"path/filepath"
	"strings"
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

func TestNewHandle(t *testing.T) {
	var free []string

	t.Run("ReadsLikeSomeoneAbbreviatingIt", func(t *testing.T) {
		for name, want := range map[string]string{
			"Enua 22/1 Wedding Cake":             "wcake",
			"Pedanios 22/1 DNK Ghost Train Haze": "dgth",
			"Cantourage 25/1 MAC1+":              "mac1",
			"Khiron 20/1 Munson":                 "muns",
		} {
			t.Run(name, func(t *testing.T) {
				got := NewHandle(Parse(name), free)
				assert.Equal(t, want, got, "Should abbreviate the cultivar, not the manufacturer")
			})
		}
	})

	t.Run("AlwaysThreeToFiveCharacters", func(t *testing.T) {
		for _, name := range []string{
			"Enua 22/1 Wedding Cake", "X 1/1 Ab", "Maker 20/1 A", "Nothing At All",
			"Cansativa Vanilla Cake 26/1", "CNBS FK 30/1: Florida Kush.",
		} {
			h := NewHandle(Parse(name), free)
			assert.GreaterOrEqual(t, len(h), minHandle, "%q gave %q", name, h)
			assert.LessOrEqual(t, len(h), maxHandle, "%q gave %q", name, h)
			assert.NoError(t, CheckSlug(h), "%q gave %q, which is not a usable slug", name, h)
		}
	})

	t.Run("AvoidsWhatIsAlreadyTaken", func(t *testing.T) {
		c := &Catalog{}
		var handles []string
		// The same product bought five times over, which is what a recurring
		// prescription looks like.
		for i := 0; i < 5; i++ {
			p := Parse("Enua 22/1 Wedding Cake")
			p.Slug = NewHandle(p, c.Handles())
			require.NoError(t, c.Add(p))
			handles = append(handles, p.Slug)
		}

		seen := map[string]bool{}
		for _, h := range handles {
			assert.False(t, seen[h], "%q was handed out twice", h)
			assert.LessOrEqual(t, len(h), maxHandle, "%q outgrew the limit", h)
			seen[h] = true
		}
		// And none of them is a keystroke from another, since references are
		// resolved by prefix.
		for i, a := range handles {
			for j, b := range handles {
				if i != j {
					assert.False(t, strings.HasPrefix(a, b),
						"%q has %q as a prefix; one typo lands on the wrong jar", a, b)
				}
			}
		}
	})

	t.Run("SurvivesAWholeCatalogOfCollisions", func(t *testing.T) {
		c := &Catalog{}
		for i := 0; i < 60; i++ {
			p := Parse("Maker 20/1 Same Name")
			p.Slug = NewHandle(p, c.Handles())
			require.NoError(t, c.Add(p), "collision at %d", i)
		}
		assert.Len(t, c.Products, 60, "every one should have got its own slug")
	})

	t.Run("FallsBackToTheNameWithoutACultivar", func(t *testing.T) {
		h := NewHandle(&Product{Name: "Liefermenge Indica"}, free)

		assert.NotEmpty(t, h, "Should still produce something")
		assert.GreaterOrEqual(t, len(h), minHandle)
	})
}

func TestCheckSlug(t *testing.T) {
	for _, ok := range []string{"wc", "wcake", "mac1", "ghost-train", "a1"} {
		assert.NoError(t, CheckSlug(ok), "%q should be usable", ok)
	}
	for _, bad := range []string{"", "w", "Wcake", "wc ake", "wc/ake", "-wc", strings.Repeat("a", 25)} {
		assert.ErrorIs(t, CheckSlug(bad), ErrBadSlug, "%q should be refused", bad)
	}
}

func TestTaken(t *testing.T) {
	c := &Catalog{}
	require.NoError(t, c.Add(&Product{Slug: "wcake", Name: "Wedding Cake"}))

	assert.True(t, c.Taken("wcake"), "Should know what it holds")
	assert.False(t, c.Taken("lcook"), "and what it does not")
}
