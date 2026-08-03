package ledger

import (
	"math"
	"sort"
	"time"

	"github.com/TheDonDope/wits/pkg/journal"
)

// Balance is how many grams of one product sit in each account.
type Balance struct {
	Product  string
	Storage  float64
	Stash    float64
	Consumed float64
	AVB      float64
}

// Total returns the grams of this product still held, that is everything that
// has not yet gone through a device.
func (b Balance) Total() float64 { return Round(b.Storage + b.Stash) }

// State is the result of replaying a journal.
type State struct {
	Balances map[string]*Balance
	Events   []journal.Event
	Cycles   []Cycle
}

// CycleGap is how long after a cycle opened a further purchase still counts as
// part of the same fill.
//
// A cycle runs from one fill to the next. Ending it when storage reaches zero
// instead sounds tidier, and usually amounts to the same thing, but it does not
// survive real data: 13 of the 47 spreadsheet cycles ended with a remainder
// between 0.01 g and 4.84 g rather than at zero, and consecutive cycles
// routinely overlap because a new sheet is started before the old one is
// finished — 2025-10 has entries as late as 26 November, while 2025-11 begins
// on the 1st. A single running total cannot describe two live cycles, so the
// fill is the only boundary that holds.
//
// Products for one prescription are collected together, so purchases within a
// few days of the cycle opening belong to it. The tightest real boundary in
// four years was eight days, between 2024-12-1 and 2024-12-2.
const CycleGap = 3 * 24 * time.Hour

// Cycle is one prescription fill, running until the next fill arrives. Cycles
// are derived, never stored.
type Cycle struct {
	Start     time.Time
	End       time.Time // zero while the cycle is still open
	Purchased float64
	Carried   float64 // grams already in storage when the fill arrived
	Ground    float64
	Leftover  float64 // grams still in storage when the next fill arrived
	Products  []string
	Events    []journal.Event

	// Opening is the storage balance each product carried into the cycle.
	// Without it, grinding down last month's remainder reads as overspending
	// this month's fill, and a product's "left" would run past 100%.
	Opening map[string]float64
}

// Open reports whether the cycle still has product in storage.
func (c Cycle) Open() bool { return c.End.IsZero() }

// Held returns the grams the cycle started with: the fill itself plus
// whatever the previous cycle left in storage.
func (c Cycle) Held() float64 { return Round(c.Purchased + c.Carried) }

// Remaining returns the grams of the cycle still in storage.
func (c Cycle) Remaining() float64 { return Round(c.Held() - c.Ground) }

// RemainingPct returns how much of the cycle is left, from 0 to 1.
func (c Cycle) RemainingPct() float64 {
	if c.Held() == 0 {
		return 0
	}
	return c.Remaining() / c.Held()
}

// PurchasedOf returns how many grams of one product the cycle's fill brought.
func (c Cycle) PurchasedOf(slug string) float64 {
	var grams float64
	for _, e := range c.Events {
		if e.Product == slug && e.Type == journal.Purchase {
			grams += e.Grams
		}
	}
	return Round(grams)
}

// HeldOf returns the grams of one product the cycle started with, carry-over
// included, which is what a per-product percentage has to be a percentage of.
func (c Cycle) HeldOf(slug string) float64 {
	return Round(c.PurchasedOf(slug) + c.Opening[slug])
}

// GroundOf returns the grams of one product ground during the cycle.
func (c Cycle) GroundOf(slug string) float64 {
	var grams float64
	for _, e := range c.Events {
		if e.Product == slug && e.Type == journal.Grind {
			grams += e.Grams
		}
	}
	return Round(grams)
}

// Fill returns the grams the cycle's own jars started with: what the fill
// dispensed, plus what those same jars still carried from an earlier fill of
// the same product — grinding draws on the jar, not on one vintage in it.
// The remainder sitting in other jars is the previous cycles' business and
// is deliberately not counted; Held is the whole shelf, Fill is this fill.
func (c Cycle) Fill() float64 {
	var grams float64
	for _, slug := range c.Products {
		grams += c.HeldOf(slug)
	}
	return Round(grams)
}

// FillRemaining returns the grams of the fill's own jars not yet ground, by
// the cycle's events. A live shelf can disagree by whatever a reconciliation
// adjusted; a screen holding the balances should prefer State.FillOnShelf.
func (c Cycle) FillRemaining() float64 {
	remaining := c.Fill()
	for _, slug := range c.Products {
		remaining -= c.GroundOf(slug)
	}
	return Round(remaining)
}

// FillRemainingPct returns how much of the fill is left, from 0 to 1.
func (c Cycle) FillRemainingPct() float64 {
	if c.Fill() == 0 {
		return 0
	}
	return c.FillRemaining() / c.Fill()
}

// Fold replays the events and returns the state they describe. Events are
// folded in journal order, which is the order they were recorded in.
func Fold(events []journal.Event) *State {
	s := &State{Balances: map[string]*Balance{}, Events: events}

	// cur indexes into s.Cycles rather than pointing at an element: appending to
	// the slice can move the backing array, which would strand a pointer.
	cur := -1
	for _, e := range events {
		b := s.balance(e.Product)
		apply(b, e.From, -e.Grams)
		apply(b, e.To, e.Grams)

		switch e.Type {
		case journal.Purchase:
			if cur == -1 || e.OccurredAt.Sub(s.Cycles[cur].Start) > CycleGap {
				if cur != -1 {
					s.Cycles[cur].End = e.OccurredAt
					s.Cycles[cur].Leftover = s.Cycles[cur].Remaining()
				}
				// The purchase itself has already been applied above, so the
				// snapshot backs it out again: what was carried in is what sat
				// in storage before this fill arrived.
				opening := map[string]float64{}
				carried := 0.0
				for slug, bal := range s.Balances {
					held := bal.Storage
					if slug == e.Product {
						held = Round(held - e.Grams)
					}
					if held > 0 {
						opening[slug] = held
						carried += held
					}
				}
				s.Cycles = append(s.Cycles, Cycle{
					Start: e.OccurredAt, Opening: opening, Carried: Round(carried),
				})
				cur = len(s.Cycles) - 1
			}
			s.Cycles[cur].Purchased = Round(s.Cycles[cur].Purchased + e.Grams)
			if !contains(s.Cycles[cur].Products, e.Product) {
				s.Cycles[cur].Products = append(s.Cycles[cur].Products, e.Product)
			}
		case journal.Grind:
			if cur != -1 {
				s.Cycles[cur].Ground = Round(s.Cycles[cur].Ground + e.Grams)
			}
		}
		if cur != -1 {
			s.Cycles[cur].Events = append(s.Cycles[cur].Events, e)
		}
	}
	return s
}

// balance returns the balance record for a product, creating it on first use.
func (s *State) balance(product string) *Balance {
	b, ok := s.Balances[product]
	if !ok {
		b = &Balance{Product: product}
		s.Balances[product] = b
	}
	return b
}

// Products returns every product the journal mentions, in alphabetical order,
// so that output is stable rather than following Go's map iteration.
func (s *State) Products() []string {
	products := make([]string, 0, len(s.Balances))
	for p := range s.Balances {
		products = append(products, p)
	}
	sort.Strings(products)
	return products
}

// Held returns the grams of a product that have not yet gone through a device.
func (s *State) Held(product string) float64 {
	if b, ok := s.Balances[product]; ok {
		return b.Total()
	}
	return 0
}

// CurrentCycle returns the cycle in progress, or nil when storage is empty.
func (s *State) CurrentCycle() *Cycle {
	if n := len(s.Cycles); n > 0 && s.Cycles[n-1].Open() {
		return &s.Cycles[n-1]
	}
	return nil
}

// FillOnShelf returns the storage the cycle's own jars still hold: the live
// counterpart of Cycle.FillRemaining, which also feels what reconciliation
// adjusted after the fact.
func (s *State) FillOnShelf(c *Cycle) float64 {
	var grams float64
	for _, slug := range c.Products {
		if b, ok := s.Balances[slug]; ok {
			grams += b.Storage
		}
	}
	return Round(grams)
}

// CarriedOnShelf returns the storage still sitting outside the cycle's own
// jars, and how many jars hold it — the previous cycles' remainder, which the
// fill's arithmetic deliberately leaves out.
func (s *State) CarriedOnShelf(c *Cycle) (float64, int) {
	own := map[string]bool{}
	for _, slug := range c.Products {
		own[slug] = true
	}
	var grams float64
	var jars int
	for slug, b := range s.Balances {
		if !own[slug] && b.Storage > 0 {
			grams += b.Storage
			jars++
		}
	}
	return Round(grams), jars
}

// Stats summarises a run of events over time.
//
// The spreadsheet this replaces counted "therapy days" as the number of dated
// rows, which included days pre-filled with a zero amount. Both readings are
// reported here, and named, so it is clear which one a number came from.
type Stats struct {
	Ground        float64
	ActiveDays    int // days with an amount actually ground
	ElapsedDays   int // calendar days from the first to the last event
	First, Last   time.Time
	PerActiveDay  float64
	PerElapsedDay float64
	MedianPerDay  float64
}

// DaysLeft estimates how much longer the given number of grams will last at the
// observed rate. It returns 0 when there is no rate to extrapolate from.
func (st Stats) DaysLeft(grams float64) float64 {
	if st.PerActiveDay <= 0 {
		return 0
	}
	return Round(grams / st.PerActiveDay)
}

// Summarise returns the statistics for the grind events among the given events.
// Grinding is what the spreadsheet recorded and what there is history for;
// consumption out of the stash is tracked separately.
func Summarise(events []journal.Event) Stats {
	perDay := map[string]float64{}
	var st Stats
	for _, e := range events {
		if e.Type != journal.Grind {
			continue
		}
		day := e.OccurredAt.Format(time.DateOnly)
		perDay[day] = Round(perDay[day] + e.Grams)
		st.Ground = Round(st.Ground + e.Grams)
		if st.First.IsZero() || e.OccurredAt.Before(st.First) {
			st.First = e.OccurredAt
		}
		if e.OccurredAt.After(st.Last) {
			st.Last = e.OccurredAt
		}
	}

	amounts := make([]float64, 0, len(perDay))
	for _, g := range perDay {
		if g > 0 {
			amounts = append(amounts, g)
		}
	}
	sort.Float64s(amounts)

	st.ActiveDays = len(amounts)
	if !st.First.IsZero() {
		st.ElapsedDays = int(st.Last.Sub(st.First).Hours()/24) + 1
	}
	if st.ActiveDays > 0 {
		st.PerActiveDay = Round(st.Ground / float64(st.ActiveDays))
		st.MedianPerDay = median(amounts)
	}
	if st.ElapsedDays > 0 {
		st.PerElapsedDay = Round(st.Ground / float64(st.ElapsedDays))
	}
	return st
}

// apply moves grams into an account on a balance. Movements to and from
// External fall outside the tracked accounts and are ignored.
func apply(b *Balance, account journal.Account, grams float64) {
	switch account {
	case journal.Storage:
		b.Storage = Round(b.Storage + grams)
	case journal.Stash:
		b.Stash = Round(b.Stash + grams)
	case journal.Consumed:
		b.Consumed = Round(b.Consumed + grams)
	case journal.AVB:
		b.AVB = Round(b.AVB + grams)
	}
}

// median returns the median of a sorted slice.
func median(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return sorted[n/2]
	}
	return Round((sorted[n/2-1] + sorted[n/2]) / 2)
}

// Round trims a value to centigrams, which is the precision a jeweller's scale
// gives and the precision the source spreadsheet recorded. It is exported so
// that everything handling grams rounds them the same one way.
func Round(g float64) float64 { return math.Round(g*100) / 100 }

// contains reports whether s holds v.
func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
