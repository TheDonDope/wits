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

	// lots is each product's jar split into the fills that stocked it,
	// oldest first. It is how a gram in storage stays on the account of the
	// cycle that dispensed it, and how a cycle knows when it is empty.
	lots map[string][]lot
}

// CycleGap is how long after a cycle opened a further purchase still counts as
// part of the same fill.
//
// One prescription is sometimes picked up across a couple of days, so
// purchases within this window of the cycle opening belong to the same fill.
// Anything later is the next prescription and opens the next cycle — the
// tightest real boundary in four years of records was eight days, between
// 2024-12-1 and 2024-12-2.
//
// Only the opening is time-based. The close is not: a cycle ends when its own
// jars are empty, however long that takes — 13 of the 47 spreadsheet cycles
// ended with a remainder rather than at zero, and a cycle with grams standing
// simply stays open beside its successors.
const CycleGap = 3 * 24 * time.Hour

// Cycle is one prescription fill. It opens with a purchase and closes when
// its own jars are empty — not when the next fill arrives, because a fill
// outliving its month is normal and a remainder belongs to the cycle that
// dispensed it. Cycles overlap: several can stand open at once, each holding
// what is left of its own fill. Cycles are derived, never stored.
type Cycle struct {
	Seq       int // position in State.Cycles
	Start     time.Time
	End       time.Time // zero while any of the cycle's own grams remain
	Purchased float64
	Carried   float64 // grams already in storage when the fill arrived
	Ground    float64
	Products  []string
	Events    []journal.Event // everything recorded during the fill's tenure

	// Opening is the storage balance each product carried into the cycle,
	// noted for the record: those grams stay on their own cycles' accounts.
	Opening map[string]float64
}

// Open reports whether any of the cycle's own grams are still in storage.
func (c Cycle) Open() bool { return c.End.IsZero() }

// Held returns the grams on the whole shelf when the cycle opened: the fill
// itself plus everything earlier cycles still had standing. The supply
// projection starts here; the cycle's own arithmetic does not.
func (c Cycle) Held() float64 { return Round(c.Purchased + c.Carried) }

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

// lot is one fill's share of a product's jar. A jar refilled before it was
// empty holds grams of two cycles, and no scale can say whose leave first —
// so the ledger says the oldest do: grinds consume lots first-in-first-out,
// and each cycle's claim shrinks in the order it was dispensed.
type lot struct {
	cycle int
	grams float64
}

// Fold replays the events and returns the state they describe. Events are
// folded in journal order, which is the order they were recorded in.
func Fold(events []journal.Event) *State {
	s := &State{Balances: map[string]*Balance{}, Events: events, lots: map[string][]lot{}}

	// share is what each cycle still holds in storage across all its jars;
	// a cycle closes the moment its share is ground away, and reopens if a
	// reconciliation finds grams again.
	var share []float64
	settle := func(cycle int, at time.Time) {
		if Round(share[cycle]) <= 0 {
			share[cycle] = 0
			s.Cycles[cycle].End = at
		} else if !s.Cycles[cycle].End.IsZero() {
			s.Cycles[cycle].End = time.Time{}
		}
	}
	// consume draws grams out of a product's jar, oldest lot first.
	consume := func(product string, grams float64, at time.Time) {
		q := s.lots[product]
		for grams > 0 && len(q) > 0 {
			take := math.Min(grams, q[0].grams)
			q[0].grams = Round(q[0].grams - take)
			grams = Round(grams - take)
			share[q[0].cycle] = Round(share[q[0].cycle] - take)
			settle(q[0].cycle, at)
			if q[0].grams <= 0 {
				q = q[1:]
			}
		}
		s.lots[product] = q
	}
	// credit puts found grams back into the jar's newest lot: an upward
	// reconciliation corrects the present jar, not a bygone fill.
	credit := func(product string, grams float64, cycle int, at time.Time) {
		if len(s.Cycles) == 0 {
			return
		}
		q := s.lots[product]
		if n := len(q); n > 0 {
			cycle = q[n-1].cycle
			q[n-1].grams = Round(q[n-1].grams + grams)
		} else {
			q = append(q, lot{cycle: cycle, grams: grams})
		}
		s.lots[product] = q
		share[cycle] = Round(share[cycle] + grams)
		settle(cycle, at)
	}

	// cur indexes into s.Cycles rather than pointing at an element: appending to
	// the slice can move the backing array, which would strand a pointer.
	cur := -1
	lastCycleOf := map[string]int{}
	for _, e := range events {
		b := s.balance(e.Product)
		apply(b, e.From, -e.Grams)
		apply(b, e.To, e.Grams)

		switch e.Type {
		case journal.Purchase:
			if cur == -1 || e.OccurredAt.Sub(s.Cycles[cur].Start) > CycleGap {
				// A new fill opens a new cycle. The one before stays open
				// for as long as its own jars hold something: a fill
				// outliving its month is normal, and its remainder is its
				// own, not the newcomer's.
				//
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
					Seq: len(s.Cycles), Start: e.OccurredAt,
					Opening: opening, Carried: Round(carried),
				})
				share = append(share, 0)
				cur = len(s.Cycles) - 1
			}
			s.Cycles[cur].Purchased = Round(s.Cycles[cur].Purchased + e.Grams)
			if !contains(s.Cycles[cur].Products, e.Product) {
				s.Cycles[cur].Products = append(s.Cycles[cur].Products, e.Product)
			}
			s.lots[e.Product] = append(s.lots[e.Product], lot{cycle: cur, grams: e.Grams})
			share[cur] = Round(share[cur] + e.Grams)
			lastCycleOf[e.Product] = cur
		case journal.Grind:
			if cur != -1 {
				s.Cycles[cur].Ground = Round(s.Cycles[cur].Ground + e.Grams)
			}
			consume(e.Product, e.Grams, e.OccurredAt)
		default:
			// Anything else that moves grams through storage — adjustments
			// down and up, corrections either way — settles the lots too, so
			// a jar reconciled to zero closes its cycles' claims.
			if e.From == journal.Storage {
				consume(e.Product, e.Grams, e.OccurredAt)
			}
			if e.To == journal.Storage && e.Product != "" {
				credit(e.Product, e.Grams, lastCycleOf[e.Product], e.OccurredAt)
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

// CurrentCycle returns the latest fill while anything on the shelf keeps a
// cycle open, or nil once every cycle has ground down to nothing. The latest
// fill is the prescription in progress even when an older cycle's jar
// outlives it.
func (s *State) CurrentCycle() *Cycle {
	if len(s.Cycles) == 0 {
		return nil
	}
	for i := range s.Cycles {
		if s.Cycles[i].Open() {
			return &s.Cycles[len(s.Cycles)-1]
		}
	}
	return nil
}

// ShareOf returns the grams of one product still standing on the cycle's own
// account — the jar's balance minus whatever older or newer fills hold in it.
func (s *State) ShareOf(c *Cycle, slug string) float64 {
	var grams float64
	for _, l := range s.lots[slug] {
		if l.cycle == c.Seq {
			grams += l.grams
		}
	}
	return Round(grams)
}

// FillOnShelf returns the grams of the cycle's fill still in storage: its
// own lots, summed over its products.
func (s *State) FillOnShelf(c *Cycle) float64 {
	var grams float64
	for _, slug := range c.Products {
		grams += s.ShareOf(c, slug)
	}
	return Round(grams)
}

// CarriedOnShelf returns the storage still standing on other cycles'
// accounts — the older fills' remainders — with the jars holding it and the
// cycles still open for it.
func (s *State) CarriedOnShelf(c *Cycle) (grams float64, jars, cycles int) {
	open := map[int]bool{}
	for _, q := range s.lots {
		var other float64
		for _, l := range q {
			if l.cycle != c.Seq && l.grams > 0 {
				other += l.grams
				open[l.cycle] = true
			}
		}
		if other > 0 {
			jars++
			grams += other
		}
	}
	return Round(grams), jars, len(open)
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
