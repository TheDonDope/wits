package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/TheDonDope/wits/plugins/internal/wits"
)

// TestTolerance: months tallied oldest first, the last three pooled
// against the three before them.
func TestTolerance(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.Local)
	var sessions []wits.Session
	for m := range 6 {
		g := 0.2
		if m >= 3 {
			g = 0.25
		}
		for range 10 {
			sessions = append(sessions, wits.Session{At: time.Date(2026, time.Month(6+m), 10, 20, 0, 0, 0, time.Local), Grams: g})
		}
	}
	ms := tally(sessions, now, 6)
	assert.Equal(t, "2026-06", ms[0].start.Format("2006-01"))
	assert.Equal(t, 10, ms[5].sessions)
	assert.InDelta(t, 2.5, ms[5].grams, 1e-9)
	assert.Contains(t, verdict(ms, 15), "25% more than three months before (0.250 g, was 0.200 g)")
	assert.Contains(t, verdict(ms, 30), "Steady: a session takes 0.250 g, +25%")
	assert.Contains(t, verdict(ms[:5], 15), "Six months or more")

	var out bytes.Buffer
	render(&out, ms, 15)
	assert.Contains(t, out.String(), "2026-09  10        2.50 g  0.250 g    +25%  ████████████████████")
}
