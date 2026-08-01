package record

import (
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits-tui/pkg/catalog"
	"github.com/TheDonDope/wits-tui/pkg/journal"
	"github.com/TheDonDope/wits-tui/pkg/ledger"
	"github.com/TheDonDope/wits-tui/pkg/repo"
)

// TestMain handles global test setup
func TestMain(m *testing.M) {
	// Disable log output during tests
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// recorder returns a recorder over a fresh repository.
func recorder(t *testing.T) *Recorder {
	t.Helper()
	r, err := repo.Init(t.TempDir())
	require.NoError(t, err)
	return New(r, &catalog.Catalog{}, &catalog.Devices{}, ledger.Fold(nil))
}

func TestBuy(t *testing.T) {
	rec := recorder(t)

	e, p, added, err := rec.Buy("Enua 22/1 Wedding Cake", 20, time.Now())
	require.NoError(t, err)

	assert.True(t, added, "Should register a product it has not seen")
	assert.Equal(t, "enua-wedding-cake", p.Slug, "Should slug the name")
	assert.Equal(t, journal.Purchase, e.Type, "Should record a purchase")
	assert.Equal(t, 20.0, rec.Available("enua-wedding-cake", journal.Storage), "Should land in storage")

	_, _, added, err = rec.Buy("Enua 22/1 Wedding Cake", 10, time.Now())
	require.NoError(t, err)
	assert.False(t, added, "Should reuse the product the second time")
	assert.Equal(t, 30.0, rec.Available("enua-wedding-cake", journal.Storage), "Should top up storage")
}

func TestGrind(t *testing.T) {
	rec := recorder(t)
	_, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", 20, time.Now())
	require.NoError(t, err)

	t.Run("MovesStorageIntoTheTin", func(t *testing.T) {
		_, err := rec.Grind("wedding", 0.75, time.Now())
		require.NoError(t, err)

		assert.Equal(t, 19.25, rec.Available("enua-wedding-cake", journal.Storage), "Should leave storage short")
		assert.Equal(t, 0.75, rec.Available("enua-wedding-cake", journal.Stash), "Should fill the tin")
	})

	t.Run("RefusesToOverdraw", func(t *testing.T) {
		_, err := rec.Grind("wedding", 500, time.Now())

		assert.ErrorContains(t, err, "cannot take", "Should refuse rather than go negative")
		assert.Equal(t, 19.25, rec.Available("enua-wedding-cake", journal.Storage), "Should not have recorded anything")
	})

	t.Run("UnknownProduct", func(t *testing.T) {
		_, err := rec.Grind("blueberry", 1, time.Now())
		assert.Error(t, err, "Should not grind a product it has never seen")
	})
}

func TestSession(t *testing.T) {
	rec := recorder(t)
	_, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", 20, time.Now())
	require.NoError(t, err)
	_, err = rec.Grind("wedding", 1.0, time.Now())
	require.NoError(t, err)

	t.Run("DrawsOnTheTin", func(t *testing.T) {
		e, err := rec.Session("wedding", 0.4, time.Now(), "", 0, "evening")
		require.NoError(t, err)

		assert.Equal(t, journal.Sesh, e.Type, "Should record a session")
		assert.Equal(t, "evening", e.Note, "Should keep the note")
		assert.Equal(t, 0.6, rec.Available("enua-wedding-cake", journal.Stash), "Should empty the tin by that much")
	})

	t.Run("RefusesMoreThanTheTinHolds", func(t *testing.T) {
		_, err := rec.Session("wedding", 99, time.Now(), "", 0, "")
		assert.ErrorContains(t, err, "in the tin", "Should say which account is short")
	})

	t.Run("RefusesATemperatureTheDeviceCannotReach", func(t *testing.T) {
		devices := &catalog.Devices{}
		require.NoError(t, devices.Add(&catalog.Device{Name: "Volcano", MaxTemp: 230, DefaultTemp: 185}))
		rec.devices = devices

		_, err := rec.Session("wedding", 0.1, time.Now(), "volcano", 250, "")
		assert.ErrorContains(t, err, "only goes up to", "Should refuse an impossible setting")
	})

	t.Run("UsesTheDeviceDefaultTemperature", func(t *testing.T) {
		devices := &catalog.Devices{}
		require.NoError(t, devices.Add(&catalog.Device{Name: "Volcano", MaxTemp: 230, DefaultTemp: 185}))
		rec.devices = devices

		e, err := rec.Session("wedding", 0.1, time.Now(), "volcano", 0, "")
		require.NoError(t, err)
		assert.Equal(t, 185, e.Temperature, "Should fall back to the device default")
	})
}

func TestStateFollowsAlong(t *testing.T) {
	rec := recorder(t)
	_, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", 20, time.Now())
	require.NoError(t, err)

	assert.Len(t, rec.State().Events, 1, "Should fold each entry as it is recorded")
	assert.NotNil(t, rec.State().CurrentCycle(), "Should open a cycle on the first fill")
}
