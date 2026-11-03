package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/plugins/internal/wits"
)

// TestRender: a column a week from a Monday, a row a weekday, today the
// last cell, the days shaded by their quartile, and the sums under it.
func TestRender(t *testing.T) {
	now := time.Date(2026, 11, 4, 20, 0, 0, 0, time.Local) // a Wednesday
	at := func(d int) time.Time { return now.AddDate(0, 0, -d) }
	sessions := []wits.Session{
		{At: at(0), Grams: 0.1}, {At: at(1), Grams: 0.2}, {At: at(2), Grams: 0.3},
		{At: at(7), Grams: 0.4}, {At: at(7), Grams: 0.4}, {At: at(30), Grams: 9},
	}
	var out bytes.Buffer
	render(&out, sessions, now, 2, false)
	lines := strings.Split(out.String(), "\n")
	require.GreaterOrEqual(t, len(lines), 8)
	assert.Equal(t, "Nov", strings.TrimSpace(lines[0]), "November begins in the second week")
	// The days used: 0.1, 0.2, 0.3 and 0.8 g, so each falls in its own shade.
	assert.Equal(t, "Mon ·▓", lines[1], "Monday's 0.3 g; the Monday before, none")
	assert.Equal(t, "    ·▒", lines[2], "Tuesday's 0.2 g")
	assert.Equal(t, "Wed █░", lines[3], "a week ago two sessions, 0.8 g; today, the last cell, 0.1 g")
	assert.Equal(t, "    ·", lines[4], "no Thursday yet this week")
	// The session 30 days back is outside the two weeks.
	assert.Contains(t, out.String(), "5 sessions, 1.40 g in 2 weeks: a session on 4 of 10 days; the longest break, 4 days.")
}
