package tui

import (
	"testing"
	"time"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/stretchr/testify/assert"
)

func TestGrindTotals(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 10, d, 20, 0, 0, 0, time.UTC) }
	grind := func(product string, grams float64, d int) journal.Event {
		return journal.Event{Type: journal.Grind, Product: product, Grams: grams, OccurredAt: day(d)}
	}
	events := []journal.Event{
		grind("kush", 1.63, 1), grind("kush", 1.18, 2),
		grind("cake", 2.81, 1),
		grind("haze", 2.81, 2),
		{Type: journal.Sesh, Product: "kush", Grams: 0.3, OccurredAt: day(2)},
	}

	for range 20 { // the slugs come out of a map: an unbroken tie shows within a few tries
		totals, days, ground, slugs := grindTotals(events)

		assert.Equal(t, []string{"cake", "haze", "kush"}, slugs, "Should rank a tie by slug, the same every time")
		assert.Equal(t, 2.81, totals["kush"], "Should add in centigrams, not drift as floats")
		assert.Equal(t, 8.43, ground)
		assert.Len(t, days["kush"], 2)
	}
}

func TestDailyGrams(t *testing.T) {
	at := func(d, h int) time.Time { return time.Date(2026, 10, d, h, 0, 0, 0, time.UTC) }
	events := []journal.Event{
		{Type: journal.Sesh, Grams: 0.1, OccurredAt: at(3, 9)},
		{Type: journal.Grind, Grams: 5, OccurredAt: at(1, 9)},
		{Type: journal.Sesh, Grams: 0.2, OccurredAt: at(3, 21)},
		{Type: journal.Sesh, Grams: 0.25, OccurredAt: at(5, 12)},
	}

	perDay, first, last := dailyGrams(events, journal.Sesh)

	assert.Equal(t, map[string]float64{"2026-10-03": 0.3, "2026-10-05": 0.25}, perDay, "Should add each day's sessions, and nothing else")
	assert.Equal(t, at(3, 9), first, "Should start at the first session, not the grind before it")
	assert.Equal(t, at(5, 12), last)
	assert.Equal(t, []float64{0.3, 0, 0.25}, daySeries(perDay, first, last), "Should give a day with nothing a zero")

	_, first, last = dailyGrams(events, journal.Purchase)
	assert.True(t, first.IsZero() && last.IsZero(), "Should say when nothing of the kind happened")
}

func TestUsageByDevice(t *testing.T) {
	at := time.Date(2026, 10, 1, 20, 0, 0, 0, time.UTC)
	sesh := func(device string, grams float64, temp int) journal.Event {
		return journal.Event{Type: journal.Sesh, Device: device, Grams: grams, Temperature: temp, OccurredAt: at}
	}
	events := []journal.Event{
		sesh("volcano", 0.2, 180), sesh("volcano", 0.2, 190),
		sesh("mighty", 0.4, 0),
		sesh("", 0.1, 0),
	}

	for range 20 {
		names, byDev := usageByDevice(events)

		assert.Equal(t, []string{"mighty", "volcano", "no device"}, names, "Should rank a tie by name, the same every time")
		assert.Equal(t, "0.40 g · 2 sessions · avg 185°C", useNote(byDev["volcano"]))
		assert.Equal(t, "0.40 g · 1 session", useNote(byDev["mighty"]), "Should say no temperature when none was set")
	}
}
