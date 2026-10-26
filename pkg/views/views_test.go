package views

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/TheDonDope/wits/pkg/journal"
)

func at(day, hour int) time.Time { return time.Date(2026, 10, day, hour, 0, 0, 0, time.UTC) }

var events = []journal.Event{
	{Type: journal.Purchase, Product: "w", Grams: 20, OccurredAt: at(1, 9)},
	{Type: journal.Grind, Product: "w", Grams: 0.1, OccurredAt: at(1, 10)},
	{Type: journal.Grind, Product: "w", Grams: 0.2, OccurredAt: at(1, 11)},
	{Type: journal.Sesh, Product: "w", Grams: 0.15, OccurredAt: at(1, 12), Device: "mighty", Temperature: 180},
	{Type: journal.Sesh, Product: "w", Grams: 0.15, OccurredAt: at(3, 12), Device: "tornado", Heater: "yll"},
	{Type: journal.Sesh, Product: "w", Grams: 0.1, OccurredAt: at(3, 20), Device: "mighty", Temperature: 190},
	{Type: journal.Sesh, Product: "w", Grams: 0.05, OccurredAt: at(4, 8)},
}

func TestDays(t *testing.T) {
	days := Days(events)
	assert.Equal(t, []Day{
		{Date: "2026-10-01", Ground: 30, Seshed: 15},
		{Date: "2026-10-03", Seshed: 25},
		{Date: "2026-10-04", Seshed: 5},
	}, days, "0.1 and 0.2 g is 0.30 g in centigrams, not 0.30000000000000004")
	assert.Equal(t, map[string]journal.Centigrams{"2026-10-01": 30}, Daily(events, journal.Grind))
}

func TestDevices(t *testing.T) {
	use := Devices(events)
	assert.Equal(t, Use{Sessions: 2, Grams: 25, Temps: 2, TempSum: 370, LastUsed: at(3, 20)}, *use["mighty"])
	assert.Equal(t, 185, use["mighty"].MeanTemp())
	assert.Equal(t, Use{Heated: 1, LastUsed: at(3, 12)}, *use["yll"], "Should count what a heater heated")
	assert.Equal(t, 1, use[""].Sessions, "Should own a session that named no device")
}
