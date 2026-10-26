package record

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/journal"
)

// habitual stocks a jar ten days ago with a habit: four sessions, three of
// them 0.2 g, the last on the morning of nine days ago.
func habitual(t *testing.T) (*Recorder, time.Time) {
	t.Helper()
	rec := recorder(t)
	ten := time.Now().Add(-10 * 24 * time.Hour).Truncate(time.Minute)
	_, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", "", 10, ten)
	require.NoError(t, err)
	_, err = rec.Grind("wcake-221", 2, ten.Add(time.Hour))
	require.NoError(t, err)
	for i, g := range []float64{0.2, 0.3, 0.2, 0.2} {
		_, err = rec.SessionWith(Sesh{Product: "wcake-221", Grams: g, At: ten.Add(time.Duration(2+i) * time.Hour), Temp: 185})
		require.NoError(t, err)
	}
	return rec, ten.Add(5 * time.Hour)
}

func TestAForgottenGrind(t *testing.T) {
	rec, last := habitual(t)

	planned, err := rec.Forgotten("wcake-221", journal.Storage, 6.8)
	require.NoError(t, err)
	require.Len(t, planned, 1, "Should put a light jar down to one grind")
	assert.Equal(t, journal.Grind, planned[0].Type)
	assert.Equal(t, 1.2, planned[0].Grams, "Should grind what the jar is short")
	assert.Equal(t, last.Add(time.Minute), planned[0].OccurredAt, "Should date it just after the jar's last entry")
	assert.Equal(t, 8.0, rec.Available("wcake-221", journal.Storage), "Should record nothing in a preview")

	written, err := rec.RecordForgotten("wcake-221", journal.Storage, 6.8)
	require.NoError(t, err)
	require.Len(t, written, 1)
	assert.Equal(t, ForgottenNote, written[0].Note, "Should say it was worked out at the scale")
	assert.Equal(t, 6.8, rec.Available("wcake-221", journal.Storage), "Should leave storage as weighed")
	assert.Equal(t, 2.3, rec.Available("wcake-221", journal.Stash), "Should put the grams in the stash, not out of the system")
}

func TestForgottenSessions(t *testing.T) {
	rec, last := habitual(t)

	written, err := rec.RecordForgotten("wcake-221", journal.Stash, 0.45)
	require.NoError(t, err)
	require.Len(t, written, 3, "Should split 0.65 g into sessions of the usual 0.2 g")
	var total journal.Centigrams
	for i, e := range written {
		assert.Equal(t, journal.Sesh, e.Type)
		assert.Equal(t, 185, e.Temperature, "Should have them as the last session was had")
		assert.True(t, e.OccurredAt.After(last), "Should date each after the jar's last entry")
		assert.False(t, e.OccurredAt.After(time.Now()), "Should date none ahead")
		if i > 0 {
			assert.True(t, e.OccurredAt.After(written[i-1].OccurredAt), "Should spread them out")
		}
		total += journal.CG(e.Grams)
	}
	assert.Equal(t, []float64{0.22, 0.22, 0.21}, []float64{written[0].Grams, written[1].Grams, written[2].Grams},
		"Should share the odd centigram to the first")
	assert.Equal(t, journal.CG(0.65), total, "Should add up to the shortfall exactly")
	assert.Equal(t, 0.45, rec.Available("wcake-221", journal.Stash), "Should leave the stash as weighed")
}

func TestOnlyAShortfallIsForgotten(t *testing.T) {
	rec, _ := habitual(t)

	_, err := rec.Forgotten("wcake-221", journal.Storage, 8)
	require.ErrorIs(t, err, ErrNotShort, "Should find nothing forgotten when the scale agrees")
	_, err = rec.Forgotten("wcake-221", journal.Storage, 9)
	require.ErrorIs(t, err, ErrNotShort, "Should not put grams found down to a forgotten entry")
	_, err = rec.RecordForgotten("wcake-221", journal.AVB, 0)
	require.ErrorIs(t, err, ErrNotShort, "Should leave the AVB jar to a plain adjustment")
	assert.Equal(t, 8.0, rec.Available("wcake-221", journal.Storage))
}

func TestAForgottenSessionWithoutAHabit(t *testing.T) {
	rec := recorder(t)
	at := time.Now().Add(-2 * time.Hour)
	_, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", "", 10, at)
	require.NoError(t, err)
	_, err = rec.Grind("wcake-221", 1, at)
	require.NoError(t, err)

	written, err := rec.RecordForgotten("wcake-221", journal.Stash, 0.5)
	require.NoError(t, err)
	require.Len(t, written, 1, "Should put it down to a session a day when none says how big one is")
	assert.Equal(t, 0.5, written[0].Grams)

	rec = recorder(t)
	week := time.Now().Add(-7 * 24 * time.Hour)
	_, _, _, err = rec.Buy("Enua 22/1 Wedding Cake", "", 10, week)
	require.NoError(t, err)
	_, err = rec.Grind("wcake-221", 4, week)
	require.NoError(t, err)
	written, err = rec.RecordForgotten("wcake-221", journal.Stash, 0.5)
	require.NoError(t, err)
	assert.Len(t, written, 7, "Should share a week's shortfall over its seven days")

	rec = recorder(t)
	_, _, _, err = rec.Buy("Enua 22/1 Wedding Cake", "", 10, week)
	require.NoError(t, err)
	_, err = rec.Grind("wcake-221", 5.5, week)
	require.NoError(t, err)
	rec.devices, err = rec.repo.UpdateDevices(func(d *catalog.Devices) error {
		return d.Add(&catalog.Device{Name: "Storz & Bickel Mighty+", Category: catalog.Vaporizer, Load: 0.15})
	})
	require.NoError(t, err)
	planned, err := rec.Forgotten("wcake-221", journal.Stash, 4.9)
	require.NoError(t, err)
	assert.Len(t, planned, 4, "Should size them by the gear's load, 0.6 g in fills of 0.15 g")

	_, err = rec.Forgotten("wcake-221", journal.Stash, 0)
	require.ErrorIs(t, err, ErrTooMany, "Should not invent thirty-seven sessions out of a leftover")
}
