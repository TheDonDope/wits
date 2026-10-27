// Package views works out the figures the terminal interface and the browser
// both show, once, from the events: what was ground and vaped day by day, and
// what each device was used for. Each used to keep its own loop for them, the
// terminal adding grams as floats and the server in centigrams, and a figure
// fixed in one stayed wrong in the other. Callers pass the events that stand,
// in the order they happened (ledger.State.Standing).
package views

import (
	"slices"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
)

// Day is one calendar day's grams ground and vaped, in the zone each entry
// happened in.
type Day struct {
	Date           string // YYYY-MM-DD
	Ground, Seshed journal.Centigrams
}

// Days is every day an entry happened on, oldest first, with what was ground
// and vaped on it; a day with only other entries has none of either.
func Days(events []journal.Event) []Day {
	at := map[string]int{}
	var out []Day
	for _, e := range events {
		d := e.OccurredAt.Format(time.DateOnly)
		i, ok := at[d]
		if !ok {
			i = len(out)
			at[d] = i
			out = append(out, Day{Date: d})
		}
		switch e.Type {
		case journal.Grind:
			out[i].Ground += journal.CG(e.Grams)
		case journal.Sesh:
			out[i].Seshed += journal.CG(e.Grams)
		}
	}
	slices.SortFunc(out, func(a, b Day) int { return strings.Compare(a.Date, b.Date) })
	return out
}

// Daily is the grams of one kind of entry by calendar day, for a calendar
// that colours the days: only days that had some.
func Daily(events []journal.Event, kind journal.Type) map[string]journal.Centigrams {
	out := map[string]journal.Centigrams{}
	for _, e := range events {
		if e.Type == kind {
			out[e.OccurredAt.Format(time.DateOnly)] += journal.CG(e.Grams)
		}
	}
	return out
}

// Use is what a device was used for: the sessions had with it and their
// grams and temperatures, the sessions it heated, and when it was last used
// for either.
type Use struct {
	Sessions int
	Grams    journal.Centigrams
	// Temps is how many of its sessions said a temperature, and TempSum
	// what they said together.
	Temps, TempSum int
	Heated         int
	LastUsed       time.Time
}

// MeanTemp is the mean temperature its sessions said, or 0.
func (u Use) MeanTemp() int {
	if u.Temps == 0 {
		return 0
	}
	return u.TempSum / u.Temps
}

// Devices is each device's use, by its slug; sessions that named none are
// under "", owned rather than dropped: they happened.
func Devices(events []journal.Event) map[string]*Use {
	out := map[string]*Use{}
	get := func(slug string) *Use {
		u := out[slug]
		if u == nil {
			u = &Use{}
			out[slug] = u
		}
		return u
	}
	for _, e := range events {
		if e.Type != journal.Sesh {
			continue
		}
		u := get(e.Device)
		u.Sessions++
		u.Grams += journal.CG(e.Grams)
		if e.Temperature > 0 {
			u.Temps++
			u.TempSum += e.Temperature
		}
		if e.OccurredAt.After(u.LastUsed) {
			u.LastUsed = e.OccurredAt
		}
		if e.Heater != "" {
			h := get(e.Heater)
			h.Heated++
			if e.OccurredAt.After(h.LastUsed) {
				h.LastUsed = e.OccurredAt
			}
		}
	}
	return out
}

// AVBHeld is what the AVB jar holds: every product's AVB, which is one jar
// on the shelf whatever products it came from.
func AVBHeld(balances map[string]*ledger.Balance) journal.Centigrams {
	var held journal.Centigrams
	for _, b := range balances {
		held += journal.CG(b.AVB)
	}
	return held
}
