package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/plugins/internal/wits"
)

// TestRank: a jar is lifted by its own sessions above the mean, and by the
// sessions of every strain sharing its terpenes; an empty jar is no pick.
func TestRank(t *testing.T) {
	products := []wits.Product{
		{Slug: "good", Name: "Good", Stash: 1, Terpenes: []string{"myrcene"}},
		{Slug: "kin", Name: "Kin", Storage: 5, Terpenes: []string{"myrcene"}},
		{Slug: "meh", Name: "Meh", Stash: 1, Terpenes: []string{"pinene"}},
		{Slug: "gone", Name: "Gone", Terpenes: []string{"myrcene"}},
	}
	var sessions []wits.Session
	rate := func(slug string, v, n int) {
		for range n {
			sessions = append(sessions, wits.Session{Product: slug, Scores: map[string]int{"sleep": v}})
		}
	}
	rate("good", 5, 3)
	rate("meh", 2, 3)
	rate("gone", 5, 1)

	r := rank(sessions, products, "sleep", 3)
	require.Len(t, r.picks, 3, "the empty jar is no pick")
	assert.Equal(t, 7, r.rated)
	assert.Equal(t, []string{"good", "kin", "meh"}, []string{r.picks[0].product.Slug, r.picks[1].product.Slug, r.picks[2].product.Slug})
	assert.Equal(t, 0, r.picks[1].own.n, "kin is picked on its terpenes alone")

	var out bytes.Buffer
	render(&out, r, "sleep", 1)
	assert.Contains(t, out.String(), "Tonight, for sleep (7 rated sessions")
	assert.Contains(t, out.String(), "1. Good (good)")
	assert.NotContains(t, out.String(), "2. Kin", "--top 1 names one")

	out.Reset()
	render(&out, rank(sessions, products, "focus", 3), "focus", 3)
	assert.Contains(t, out.String(), "No session is rated for focus yet")
}
