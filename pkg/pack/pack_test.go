package pack

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/record"
	"github.com/TheDonDope/wits/pkg/repo"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// fixture is a fill of three strains ten days ago — 10, 10 and 1 g — with
// 2 g of each of the first two ground on two days: 2 g a day, the rate.
func fixture(t *testing.T) *workspace.Workspace {
	t.Helper()
	r, err := repo.Init(t.TempDir())
	require.NoError(t, err)
	ws, err := workspace.Read(r)
	require.NoError(t, err)
	at := time.Now().AddDate(0, 0, -10)
	_, err = ws.Recorder.Fill([]record.FillLine{
		{Name: "Enua 22/1 Wedding Cake", Grams: 10}, {Name: "Cantourage 25/1 Lemon Haze", Grams: 10},
		{Name: "Aurora 20/1 Pink Kush", Grams: 1}}, at, "")
	require.NoError(t, err)
	_, err = ws.Recorder.Grind("wcake", 2, at.AddDate(0, 0, 1))
	require.NoError(t, err)
	_, err = ws.Recorder.Grind("lhaze", 2, at.AddDate(0, 0, 2))
	require.NoError(t, err)
	ws, err = workspace.Read(r)
	require.NoError(t, err)
	return ws
}

func taken(p Plan) map[string][2]float64 {
	out := map[string][2]float64{}
	for _, l := range p.Lines {
		out[l.Name] = [2]float64{l.Stash, l.Sealed}
	}
	return out
}

func TestEven(t *testing.T) {
	p, err := Make(fixture(t), 3, 0, Even)
	require.NoError(t, err)

	assert.Equal(t, 2.0, p.Rate)
	assert.Equal(t, 6.0, p.Need)
	assert.Equal(t, 6.0, p.Packed)
	assert.False(t, p.Short)
	assert.Equal(t, map[string][2]float64{
		"Aurora 20/1 Pink Kush":      {0, 1},   // all it has
		"Cantourage 25/1 Lemon Haze": {2, 0.5}, // the stash first
		"Enua 22/1 Wedding Cake":     {2, 0.5},
	}, taken(p), "The small jar gives all it holds, and the other two share the rest")
}

func TestEvenTenths(t *testing.T) {
	p, err := Make(fixture(t), 1, 20, Even)
	require.NoError(t, err)

	assert.Equal(t, 2.4, p.Need, "2 g with 20 % on top")
	assert.Equal(t, 2.4, p.Packed, "Shares in tenths that add up to the need")
	for _, l := range p.Lines {
		assert.InDelta(t, 0.8, l.Taken(), 0.1001, "%s: about level", l.Name)
	}
}

func TestOldest(t *testing.T) {
	p, err := Make(fixture(t), 3, 0, Oldest)
	require.NoError(t, err)

	require.Len(t, p.Lines, 2, "One fill: by name, until the need is met")
	assert.Equal(t, map[string][2]float64{
		"Aurora 20/1 Pink Kush":      {0, 1},
		"Cantourage 25/1 Lemon Haze": {2, 3},
	}, taken(p))
}

func TestShort(t *testing.T) {
	p, err := Make(fixture(t), 20, 0, Even)
	require.NoError(t, err)

	assert.True(t, p.Short)
	assert.Equal(t, 40.0, p.Need)
	assert.Equal(t, 21.0, p.Shelf, "Ground or sealed, every gram dispensed is still held")
	assert.Equal(t, 21.0, p.Packed, "Everything there is")
}

func TestStatement(t *testing.T) {
	p, err := Make(fixture(t), 3, 0, Even)
	require.NoError(t, err)

	items := p.Statement()
	var total float64
	for _, it := range items {
		total += it.Sealed + it.Ground + it.AVB
		assert.NotEmpty(t, it.Fills, "%s: the fill it came from", it.Name)
	}
	assert.InDelta(t, p.Packed, total, 1e-9, "Signed for exactly what is packed")
}

func TestNoRate(t *testing.T) {
	r, err := repo.Init(t.TempDir())
	require.NoError(t, err)
	ws, err := workspace.Read(r)
	require.NoError(t, err)

	_, err = Make(ws, 3, 0, Even)
	assert.ErrorIs(t, err, ErrNoRate)
	_, err = Make(fixture(t), 0, 0, Even)
	assert.Error(t, err, "A trip is a day or more")
}
