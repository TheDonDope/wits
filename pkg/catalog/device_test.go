package catalog

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDevices(t *testing.T) {
	t.Run("AddAndFind", func(t *testing.T) {
		d := &Devices{}
		require.NoError(t, d.Add(&Device{Name: "Volcano Hybrid", MaxTemp: 230, DefaultTemp: 185}))

		device, err := d.Find("volcano")
		require.NoError(t, err)
		assert.Equal(t, "volcano-hybrid", device.Slug, "Should resolve a substring")
		assert.Equal(t, 185, device.DefaultTemp, "Should keep the default temperature")
	})

	t.Run("RefusesADuplicate", func(t *testing.T) {
		d := &Devices{}
		require.NoError(t, d.Add(&Device{Name: "Volcano Hybrid"}))

		assert.ErrorIs(t, d.Add(&Device{Name: "Volcano Hybrid"}), ErrDuplicate, "Should not shadow an existing device")
	})

	t.Run("RoundTrip", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "devices.yml")
		d := &Devices{}
		require.NoError(t, d.Add(&Device{Name: "Mighty+", Kind: "portable", MinTemp: 40, MaxTemp: 210}))
		require.NoError(t, d.Save(path))

		loaded, err := LoadDevices(path)
		require.NoError(t, err)

		require.Len(t, loaded.Devices, 1)
		assert.Equal(t, 210, loaded.Devices[0].MaxTemp, "Should survive the round trip")
	})

	t.Run("MissingFileIsAnEmptyCatalog", func(t *testing.T) {
		d, err := LoadDevices(filepath.Join(t.TempDir(), "devices.yml"))

		assert.NoError(t, err, "Should not fail before the first device is added")
		assert.Empty(t, d.Devices, "Should be empty")
	})
}

func TestReleasedAt(t *testing.T) {
	t.Run("BelowEverything", func(t *testing.T) {
		assert.Empty(t, ReleasedAt(100), "Should release nothing below the lowest boiling point")
	})

	t.Run("ReleasesWhatItReaches", func(t *testing.T) {
		released := ReleasedAt(160)

		names := map[string]bool{}
		for _, r := range released {
			names[r.Name] = true
			assert.LessOrEqual(t, r.BoilingPoint, 160, "Should not include a compound it cannot reach")
		}
		assert.True(t, names["Δ-9-THC"], "Should reach THC at 157°C")
		assert.False(t, names["CBD"], "Should not reach CBD at 165°C")
	})

	t.Run("HottestLast", func(t *testing.T) {
		released := ReleasedAt(220)

		for i := 1; i < len(released); i++ {
			assert.LessOrEqual(t, released[i-1].BoilingPoint, released[i].BoilingPoint,
				"Should be ordered by boiling point")
		}
	})
}

func TestHazards(t *testing.T) {
	t.Run("Safe", func(t *testing.T) {
		assert.Empty(t, Hazards(200), "Should find nothing harmful below 205°C")
	})

	t.Run("Benzene", func(t *testing.T) {
		hazards := Hazards(205)

		require.Len(t, hazards, 1, "Should flag benzene at its boiling point")
		assert.Equal(t, "Benzene", hazards[0].Name, "Should name the hazard")
		assert.Contains(t, hazards[0].Effects, "carcinogenic", "Should say why it matters")
	})
}
