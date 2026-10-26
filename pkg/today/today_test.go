package today

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/record"
	"github.com/TheDonDope/wits/pkg/repo"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// fixture is a fill of a 20/1 strain and one with no strength written down,
// two sessions of the first today on the Volcano, one of the second, a grind
// today, and two days of sessions in the weeks before.
func fixture(t *testing.T) (*workspace.Workspace, time.Time) {
	t.Helper()
	r, err := repo.Init(t.TempDir())
	require.NoError(t, err)
	_, err = r.UpdateDevices(func(d *catalog.Devices) error { return d.Add(&catalog.Device{Name: "Volcano"}) })
	require.NoError(t, err)
	ws, err := workspace.Read(r)
	require.NoError(t, err)
	now := time.Now()
	morning := time.Date(now.Year(), now.Month(), now.Day(), 0, 30, 0, 0, time.Local)
	if now.Before(morning.Add(time.Hour)) {
		t.Skip("too early in the day for three sessions before now")
	}
	_, err = ws.Recorder.Fill([]record.FillLine{{Name: "Enua 20/1 Wedding Cake", Grams: 20}, {Name: "Mystery Haze", Grams: 5}},
		now.AddDate(0, 0, -20), "")
	require.NoError(t, err)
	_, err = ws.Recorder.Grind("wcake", 5, now.AddDate(0, 0, -19))
	require.NoError(t, err)
	_, err = ws.Recorder.Grind("mystery", 1, now.AddDate(0, 0, -19))
	require.NoError(t, err)
	for _, d := range []int{-10, -5} {
		_, err = ws.Recorder.Session("wcake", 0.5, now.AddDate(0, 0, d), "", 0, "")
		require.NoError(t, err)
	}
	s, err := ws.Recorder.Session("wcake", 0.15, morning, "volcano", 185, "")
	require.NoError(t, err)
	_, err = ws.Recorder.Rate(s.Hash, map[effects.Dimension]int{effects.Overall: 4}, "", now)
	require.NoError(t, err)
	_, err = ws.Recorder.Session("wcake", 0.25, morning.Add(10*time.Minute), "volcano", 190, "")
	require.NoError(t, err)
	_, err = ws.Recorder.Session("mystery", 0.1, morning.Add(20*time.Minute), "", 0, "")
	require.NoError(t, err)
	_, err = ws.Recorder.Grind("wcake", 1, morning.Add(30*time.Minute))
	require.NoError(t, err)
	ws, err = workspace.Read(r)
	require.NoError(t, err)
	return ws, now
}

func TestOf(t *testing.T) {
	ws, now := fixture(t)

	d := Of(ws, now)

	require.Len(t, d.Sessions, 3)
	first := d.Sessions[0]
	assert.Equal(t, 30.0, first.THC, "0.15 g at 20 % is 30 mg")
	assert.Equal(t, 1.5, first.CBD, "and at 1 %, 1.5 mg")
	assert.Equal(t, "Volcano", first.Device)
	assert.Equal(t, 4, first.Overall)
	assert.Equal(t, 80.0, d.THC, "30 mg and 50 mg; the strain with no strength adds nothing")
	assert.Equal(t, 1, d.Unknown)
	assert.Equal(t, 0.5, d.Seshed)
	assert.Equal(t, 1.0, d.Ground)
	require.Len(t, d.Strains, 2)
	assert.Equal(t, Share{Name: "Enua 20/1 Wedding Cake", Sessions: 2, Grams: 0.4, THC: 80}, d.Strains[0])
	require.Len(t, d.Devices, 2)
	assert.Equal(t, "Volcano", d.Devices[0].Name)
	assert.Equal(t, "no device named", d.Devices[1].Name)
}

func TestUsual(t *testing.T) {
	ws, now := fixture(t)

	u := Of(ws, now).Usual

	assert.Equal(t, 2, u.Days, "The two days before today with a session")
	assert.Equal(t, 0.5, u.Seshed)
	assert.Equal(t, 100.0, u.THC, "0.5 g at 20 % a day")
}

func TestAnotherDay(t *testing.T) {
	ws, now := fixture(t)

	d := Of(ws, now.AddDate(0, 0, -5))

	require.Len(t, d.Sessions, 1)
	assert.Equal(t, 100.0, d.THC)
	assert.Zero(t, d.Ground)
}
