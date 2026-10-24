package effects

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/journal"
)

func testLog(t *testing.T) *Log {
	t.Helper()
	return Open(filepath.Join(t.TempDir(), "effects.ndjson"))
}

func TestAppendAndVerify(t *testing.T) {
	l := testLog(t)
	first, err := l.Append(Rating{Session: "abc", Scores: map[Dimension]int{Overall: 4, Sleep: 5}, Note: "slept"})
	require.NoError(t, err)
	second, err := l.Append(Rating{Session: "abc", Scores: map[Dimension]int{Overall: 5}})
	require.NoError(t, err)

	assert.Equal(t, 1, first.Seq)
	assert.Equal(t, first.Hash, second.Prev, "chained onto the one before")
	require.NoError(t, l.Verify())

	all, err := l.All()
	require.NoError(t, err)
	require.Len(t, all, 2, "a rating again supersedes, and the log keeps both")
	assert.Equal(t, 5, Latest(all)["abc"].Scores[Overall], "the newer stands")

	data, err := os.ReadFile(l.Path())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(l.Path(), bytes.Replace(data, []byte(`"sleep":5`), []byte(`"sleep":4`), 1), 0600))
	assert.ErrorIs(t, l.Verify(), ErrBrokenChain, "an edit to a stored line breaks the chain")
}

func TestAnUnfinishedLineIsLeftOut(t *testing.T) {
	l := testLog(t)
	_, err := l.Append(Rating{Session: "abc", Scores: map[Dimension]int{Overall: 3}})
	require.NoError(t, err)
	f, err := os.OpenFile(l.Path(), os.O_APPEND|os.O_WRONLY, 0600)
	require.NoError(t, err)
	_, err = f.WriteString(`{"seq":2,"sess`)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	all, err := l.All()
	require.NoError(t, err)
	assert.Len(t, all, 1)
}

func TestValidate(t *testing.T) {
	for name, r := range map[string]Rating{
		"NoSession":     {Scores: map[Dimension]int{Overall: 3}},
		"NoScores":      {Session: "abc"},
		"UnknownThing":  {Session: "abc", Scores: map[Dimension]int{"vibes": 3}},
		"TooHigh":       {Session: "abc", Scores: map[Dimension]int{Sleep: 6}},
		"BelowTheScale": {Session: "abc", Scores: map[Dimension]int{Sleep: 0}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := testLog(t).Append(r)
			assert.Error(t, err)
		})
	}
}

func TestAnalyse(t *testing.T) {
	at := func(day, hour int) time.Time { return time.Date(2026, 7, day, hour, 0, 0, 0, time.UTC) }
	sessions := []journal.Event{
		{Type: journal.Sesh, Hash: "s1", Product: "wcake", Temperature: 185, OccurredAt: at(1, 21)},
		{Type: journal.Sesh, Hash: "s2", Product: "wcake", Temperature: 182, OccurredAt: at(2, 22)},
		{Type: journal.Sesh, Hash: "s3", Product: "lemon", Temperature: 200, OccurredAt: at(3, 9)},
		{Type: journal.Sesh, Hash: "s4", Product: "lemon", OccurredAt: at(4, 10)},
		{Type: journal.Adjust, Hash: "u4", Reverts: "s4", OccurredAt: at(4, 11)},
	}
	ratings := []Rating{
		{Session: "s1", Scores: map[Dimension]int{Overall: 2, Sleep: 3}},
		{Session: "s1", Scores: map[Dimension]int{Overall: 4, Sleep: 5}}, // supersedes
		{Session: "s2", Scores: map[Dimension]int{Overall: 5, Sleep: 4}},
		{Session: "s3", Scores: map[Dimension]int{Overall: 3, Focus: 4}},
		{Session: "s4", Scores: map[Dimension]int{Overall: 1}}, // undone: left out
		{Session: "nowhere", Scores: map[Dimension]int{Overall: 1}},
	}

	in := Analyse(ratings, sessions, func(s string) string { return "Product " + s })

	assert.Equal(t, 3, in.Rated)
	require.Len(t, in.ByProduct, 2)
	assert.Equal(t, Group{Key: "lemon", Label: "Product lemon", Count: 1,
		Means:  map[Dimension]float64{Overall: 3, Focus: 4},
		Counts: map[Dimension]int{Overall: 1, Focus: 1}}, in.ByProduct[0])
	assert.Equal(t, 4.5, in.ByProduct[1].Means[Overall], "wcake: 4 and 5, the newer rating of s1")
	assert.Equal(t, 4.5, in.ByProduct[1].Means[Sleep])
	assert.Equal(t, []string{"180–189", "200–209"}, keys(in.ByTemp))
	assert.Equal(t, []string{"morning", "evening"}, labels(in.ByTimeOfDay))
}

func keys(gs []Group) []string {
	out := make([]string, len(gs))
	for i, g := range gs {
		out[i] = g.Key
	}
	return out
}

func labels(gs []Group) []string {
	out := make([]string, len(gs))
	for i, g := range gs {
		out[i] = g.Label
	}
	return out
}
