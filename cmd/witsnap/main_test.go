package main

import (
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/repo"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// seeded builds a real repository through the same recorder the commands use:
// one fill, two grinds on the same day, and two sessions without a device —
// one of them at a temperature.
func seeded(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	r, err := repo.Init(dir)
	require.NoError(t, err)
	ws, err := workspace.Read(r)
	require.NoError(t, err)

	day := func(n int) time.Time { return time.Date(2026, time.July, 9+n, 0, 0, 0, 0, time.UTC) }
	_, _, _, err = ws.Recorder.Buy("Enua 22/1 Wedding Cake", "", 20, day(0))
	require.NoError(t, err)
	_, err = ws.Recorder.Grind("wcake", 1.5, day(1))
	require.NoError(t, err)
	_, err = ws.Recorder.Grind("wcake", 0.5, day(1))
	require.NoError(t, err)
	_, err = ws.Recorder.Session("wcake", 0.3, day(2), "", 0, "")
	require.NoError(t, err)
	_, err = ws.Recorder.Session("wcake", 0.4, day(3), "", 190, "")
	require.NoError(t, err)
	return dir
}

// stdout runs fn and returns what it printed.
func stdout(t *testing.T, fn func() error) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	saved := os.Stdout
	os.Stdout = w
	runErr := fn()
	os.Stdout = saved
	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	require.NoError(t, runErr)
	return string(out)
}

func TestJSONIsTheFold(t *testing.T) {
	dir := seeded(t)
	var got state
	require.NoError(t, json.Unmarshal([]byte(stdout(t, func() error {
		return dump([]string{"--repo", dir})
	})), &got))

	assert.Equal(t, 5, got.Events, "every entry is counted")
	require.Contains(t, got.Balances, "wcake-221")
	assert.InDelta(t, 18.0, got.Balances["wcake-221"].Storage, 0.001, "20 g bought, 2 g ground")

	require.Len(t, got.Cycles, 1)
	assert.Nil(t, got.Cycles[0].Events, "the cycle summary leaves its events to the journal")

	require.NotNil(t, got.Current, "a cycle is in progress")
	assert.InDelta(t, 20.0, got.Current.Held, 0.001, "held is the fill's own grams")
	assert.InDelta(t, 18.0, got.Current.Remaining, 0.001)
	assert.InDelta(t, 0.9, got.Current.RemainingPct, 0.001)

	assert.InDelta(t, 2.0, got.PerDay["2026-07-10"].Ground, 0.001, "two grinds on one day add up")
	assert.InDelta(t, 0.3, got.PerDay["2026-07-11"].Seshed, 0.001)
	assert.NotContains(t, got.PerDay, "2026-07-09", "a purchase is neither ground nor seshed")

	require.Len(t, got.Devices, 1, "sessions without a device are owned, not dropped")
	assert.Equal(t, deviceUse{Device: "no device", Sessions: 2, Grams: 0.7, AvgTemp: 190}, got.Devices[0],
		"the average temperature counts only the sessions that recorded one")
}

func TestJSONWithoutACycle(t *testing.T) {
	dir := t.TempDir()
	_, err := repo.Init(dir)
	require.NoError(t, err)
	out := stdout(t, func() error { return dump([]string{"--repo", dir}) })
	assert.NotContains(t, out, "current_cycle", "an empty repository has no cycle in progress to report")
	assert.Contains(t, out, `"events": 0`)
}

func TestScreens(t *testing.T) {
	dir := seeded(t)

	all := stdout(t, func() error { return screens([]string{"--repo", dir}) })
	for _, name := range []string{"dashboard", "journal", "analysis", "storage", "stash", "sessions", "devices"} {
		assert.Contains(t, all, "==== "+name+" ====", "every tab is photographed, headed by its name")
	}

	one := stdout(t, func() error {
		return screens([]string{"--repo", dir, "--screen", "storage", "--width", "120", "--height", "30"})
	})
	assert.NotContains(t, one, "====", "a single screen needs no heading")
	assert.Contains(t, one, "Enua 22/1 Wedding Cake")

	err := screens([]string{"--repo", dir, "--screen", "seance"})
	assert.ErrorContains(t, err, `no screen called "seance"`, "the séance is gone, and witsnap says so")
}

func TestOpenNeedsARepository(t *testing.T) {
	_, err := open(t.TempDir())
	assert.Error(t, err, "a directory without .wits is not a repository")
}
