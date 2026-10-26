package review

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/repo"
	"github.com/TheDonDope/wits/pkg/workspace"
)

func day(m time.Month, d, h int) time.Time { return time.Date(2024, m, d, h, 0, 0, 0, time.Local) }

// fixture is 2024: two fills, grinding in January and in March with a
// break between, six sessions at 185 °C rated well and one at 205 rated
// badly, two prices, and the lemon cultivar's terpenes.
func fixture(t *testing.T) *workspace.Workspace {
	t.Helper()
	r, err := repo.Init(t.TempDir())
	require.NoError(t, err)
	ws, err := workspace.Read(r)
	require.NoError(t, err)
	rec := ws.Recorder
	cake, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", "", 20, day(time.January, 2, 10))
	require.NoError(t, err)
	lemon, _, _, err := rec.Buy("Cantourage 25/1 Lemon Haze", "", 10, day(time.January, 2, 10))
	require.NoError(t, err)
	_, err = rec.Price(cake.Hash, 20000, "", time.Now())
	require.NoError(t, err)
	_, err = rec.Price(lemon.Hash, 6000, "", time.Now())
	require.NoError(t, err)
	_, err = rec.Terpenes("lhaze", []string{"Limonene", "Myrcene"})
	require.NoError(t, err)
	for _, g := range []struct {
		ref   string
		grams float64
		at    time.Time
	}{{"wcake", 2, day(time.January, 3, 20)}, {"lhaze", 4, day(time.January, 4, 20)}, {"wcake", 1, day(time.March, 1, 20)}} {
		_, err = rec.Grind(g.ref, g.grams, g.at)
		require.NoError(t, err)
	}
	for i := range 6 {
		s, err := rec.Session("lhaze", 0.2, day(time.January, 5+i, 21), "", 185, "")
		require.NoError(t, err)
		_, err = rec.Rate(s.Hash, map[effects.Dimension]int{effects.Overall: 4 + i%2}, "", time.Now())
		require.NoError(t, err)
	}
	s, err := rec.Session("wcake", 0.2, day(time.March, 2, 21), "", 205, "")
	require.NoError(t, err)
	_, err = rec.Rate(s.Hash, map[effects.Dimension]int{effects.Overall: 2}, "", time.Now())
	require.NoError(t, err)
	ws, err = workspace.Read(r)
	require.NoError(t, err)
	return ws
}

func TestYear(t *testing.T) {
	ws := fixture(t)

	r := Year(ws, 2024)

	assert.False(t, r.Running, "2024 is over")
	assert.Equal(t, 366, r.Days, "A leap year, both ends counted")
	assert.Equal(t, 7.0, r.Ground)
	assert.Equal(t, 3, r.GrindDays)
	assert.Equal(t, 1, r.Pickups, "Two products picked up together are one prescription")
	assert.Equal(t, 7, r.Sessions)
	assert.Equal(t, 6.0, r.Months[0])
	assert.Equal(t, 1.0, r.Months[2])
	require.Len(t, r.Strains, 2)
	assert.Equal(t, "Cantourage 25/1 Lemon Haze", r.Strains[0].Name, "The most ground first")
	assert.InDelta(t, 4.0/7, r.Strains[0].Share, 1e-9)
	assert.Equal(t, day(time.January, 4, 0), r.Busiest.Day)
}

func TestYearBreak(t *testing.T) {
	r := Year(fixture(t), 2024)

	require.NotNil(t, r.Break)
	assert.Equal(t, 304, r.Break.Days, "From the day after the last session, 3 March, to New Year's Eve")
	assert.Equal(t, day(time.March, 3, 0), r.Break.From)
	assert.Equal(t, 2, r.Break.Breaks, "January's last session to March, and March to the year's end")
}

func TestYearRatings(t *testing.T) {
	r := Year(fixture(t), 2024)

	require.NotNil(t, r.Best, "Six ratings at 185 °C are enough")
	assert.Equal(t, "180–189 °C", r.Best.Label)
	assert.Equal(t, 6, r.Best.Count)
	assert.InDelta(t, 4.5, r.Best.Mean, 1e-9)
	require.NotNil(t, r.Liked)
	assert.Equal(t, "Cantourage 25/1 Lemon Haze", r.Liked.Name)
	require.NotNil(t, r.Terpene)
	assert.Equal(t, "Limonene", r.Terpene.Name)
	assert.InDelta(t, 4.0/7, r.Terpene.Share, 1e-9, "Only the lemon's grams lead with a terpene written down")
}

func TestYearPrices(t *testing.T) {
	r := Year(fixture(t), 2024)

	assert.Equal(t, int64(26000), r.Paid)
	assert.Equal(t, 30.0, r.Priced)
	require.NotNil(t, r.Dearest)
	require.NotNil(t, r.Cheapest)
	assert.Equal(t, 1000.0, r.Dearest.PerGram, "200 € for 20 g")
	assert.Equal(t, 600.0, r.Cheapest.PerGram, "60 € for 10 g")
	assert.Equal(t, []int{2024}, Years(fixture(t)))
}

func TestYearEmpty(t *testing.T) {
	r := Year(fixture(t), 2019)

	assert.Zero(t, r.Ground)
	assert.Nil(t, r.Dearest)
	assert.Nil(t, r.Liked)
}
