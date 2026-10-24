package ledger

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
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
func (b Balance) Total() float64 { return plus(b.Storage, b.Stash) }

// State is the result of replaying a journal.
type State struct {
	Balances map[string]*Balance
	// Events are the journal's entries in the order they happened, which is
	// the order they are folded in. Entries at the same moment keep the order
	// they were recorded in. The journal itself stays in recording order —
	// its hash chain is that order — so anything that needs the chain reads
	// the journal, not this.
	Events []journal.Event
	Cycles []Cycle

	// lots is each product's jar split into the fills that stocked it,
	// oldest first. It is how a gram in storage stays on the account of the
	// cycle that dispensed it, and how a cycle knows when it is empty.
	lots map[string][]lot
}

// Fill is grams in the fill's scope: one cycle's own grams — what it
// dispensed, and what of that is left.
//
// It is one of the three scopes every figure on screen speaks, with Shelf and
// Jar (ROADMAP.md has the table). Mixing scopes in one figure — a fill's
// remaining over the shelf's held, say — has been the most recurring bug in
// this codebase. As distinct types they cannot be added, compared or divided
// without a conversion, so a mix does not compile, and every place one scope
// is deliberately turned into another is a conversion that can be read and
// searched for.
type Fill float64

// Shelf is grams in the shelf's scope: every jar standing at a moment,
// whichever fill dispensed it. See Fill.
type Shelf float64

// Jar is grams in the jar's scope: one product across its lifetime, every
// fill of it. See Fill.
type Jar float64

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
	Purchased Fill
	Carried   Shelf // grams already in storage when the fill arrived
	// Ground is every grind during the fill's tenure, from whichever jar —
	// grams drain oldest first, so it is the shelf's grinding over the
	// period, not how much of this fill has been ground.
	Ground   Shelf
	Products []string
	Events   []journal.Event // everything that happened during the fill's tenure

	// Opening is the storage balance each product carried into the cycle,
	// noted for the record: those grams stay on their own cycles' accounts.
	Opening map[string]Shelf
}

// Open reports whether any of the cycle's own grams are still in storage.
func (c Cycle) Open() bool { return c.End.IsZero() }

// Held returns the grams on the whole shelf when the cycle opened: the fill
// itself plus everything earlier cycles still had standing. The supply
// projection starts here; the cycle's own arithmetic does not.
func (c Cycle) Held() Shelf { return Shelf(plus(float64(c.Purchased), float64(c.Carried))) }

// PurchasedOf returns how many grams of one product the cycle's fill brought.
func (c Cycle) PurchasedOf(slug string) Fill {
	var cg journal.Centigrams
	for _, e := range c.Events {
		if e.Product == slug && e.Type == journal.Purchase {
			cg += journal.CG(e.Grams)
		}
	}
	return Fill(cg.Grams())
}

// lot is one fill's share of a product's jar. A jar refilled before it was
// empty holds grams of two cycles, and no scale can say whose leave first —
// so the ledger says the oldest do: grinds consume lots first-in-first-out,
// and each cycle's claim shrinks in the order it was dispensed.
type lot struct {
	cycle int
	cg    journal.Centigrams
}

// folder carries a fold's running accounts: what each cycle still holds in
// storage across all its jars, and which cycle a product was last bought in.
// cur indexes into s.Cycles rather than pointing at an element: appending to
// the slice can move the backing array, which would strand a pointer.
type folder struct {
	s     *State
	share []journal.Centigrams
	last  map[string]int
	cur   int
	// drew is what each entry that drew on storage took from its lots, by
	// hash. An entry can take less than its grams — two devices ground the
	// same last grams — and undoing it must put back only what it took, or
	// the fill reads grams its jar does not hold.
	drew map[string]journal.Centigrams
}

// settle closes a cycle the moment its share is ground away, and reopens it
// if a reconciliation finds grams again.
func (f *folder) settle(cycle int, at time.Time) {
	if f.share[cycle] <= 0 {
		f.share[cycle] = 0
		f.s.Cycles[cycle].End = at
	} else if !f.s.Cycles[cycle].End.IsZero() {
		f.s.Cycles[cycle].End = time.Time{}
	}
}

// consume draws grams out of a product's jar, oldest lot first, and notes
// what the entry took.
func (f *folder) consume(e journal.Event, cg journal.Centigrams) {
	product, at, want := e.Product, e.OccurredAt, cg
	defer func() { f.drew[e.Hash] = want - cg }()
	q := f.s.lots[product]
	for cg > 0 && len(q) > 0 {
		take := min(cg, q[0].cg)
		q[0].cg -= take
		cg -= take
		f.share[q[0].cycle] -= take
		f.settle(q[0].cycle, at)
		if q[0].cg <= 0 {
			q = q[1:]
		}
	}
	f.s.lots[product] = q
}

// credit puts found grams back into the jar's newest lot: an upward
// reconciliation corrects the present jar, not a bygone fill.
func (f *folder) credit(product string, cg journal.Centigrams, at time.Time) {
	if len(f.s.Cycles) == 0 {
		return
	}
	cycle := f.last[product]
	q := f.s.lots[product]
	if n := len(q); n > 0 {
		cycle = q[n-1].cycle
		q[n-1].cg += cg
	} else {
		q = append(q, lot{cycle: cycle, cg: cg})
	}
	f.s.lots[product] = q
	f.share[cycle] += cg
	f.settle(cycle, at)
}

// purchase books a fill into its cycle, opening a new one when the purchase
// falls outside the running cycle's pickup window. The one before stays open
// for as long as its own jars hold something: a fill outliving its month is
// normal, and its remainder is its own, not the newcomer's.
func (f *folder) purchase(e journal.Event) {
	s := f.s
	if f.cur == -1 || e.OccurredAt.Sub(s.Cycles[f.cur].Start) > CycleGap {
		// The purchase itself has already been applied above, so the
		// snapshot backs it out again: what was carried in is what sat in
		// storage before this fill arrived.
		opening := map[string]Shelf{}
		var carried journal.Centigrams
		for slug, bal := range s.Balances {
			held := journal.CG(bal.Storage)
			if slug == e.Product {
				held -= journal.CG(e.Grams)
			}
			if held > 0 {
				opening[slug] = Shelf(held.Grams())
				carried += held
			}
		}
		s.Cycles = append(s.Cycles, Cycle{
			Seq: len(s.Cycles), Start: e.OccurredAt,
			Opening: opening, Carried: Shelf(carried.Grams()),
		})
		f.share = append(f.share, 0)
		f.cur = len(s.Cycles) - 1
	}
	s.Cycles[f.cur].Purchased = Fill(plus(float64(s.Cycles[f.cur].Purchased), e.Grams))
	if !contains(s.Cycles[f.cur].Products, e.Product) {
		s.Cycles[f.cur].Products = append(s.Cycles[f.cur].Products, e.Product)
	}
	s.lots[e.Product] = append(s.lots[e.Product], lot{cycle: f.cur, cg: journal.CG(e.Grams)})
	f.share[f.cur] += journal.CG(e.Grams)
	f.last[e.Product] = f.cur
}

// Shortfall is an entry that takes more from an account than the account
// held at its moment: grams that were not there to take.
type Shortfall struct {
	Event   journal.Event
	Account journal.Account
	Held    float64 // what the account held just before; never less than nothing
}

// String says what happened, in the jar's terms: on 2026-07-02, grind 5.00 g
// of wcake-221 took more than storage held — 1.00 g.
func (s Shortfall) String() string {
	return fmt.Sprintf("on %s, %s %.2f g of %s took more than %s held — %.2f g",
		s.Event.OccurredAt.Format(time.DateOnly), s.Event.Type, s.Event.Grams, s.Event.Product,
		jarName[s.Account], s.Held)
}

// jarName is what each account is called in a sentence.
var jarName = map[journal.Account]string{
	journal.Storage: "storage", journal.Stash: "the stash",
	journal.Consumed: "the vaporizer", journal.AVB: "the AVB jar",
}

// Shortfalls replays the events in the order they happened and returns each
// entry that takes more than its account held at that moment — a grind dated
// before the fill it draws on, say.
//
// Grams are flower in a jar, not money in an account: a jar cannot hold less
// than nothing, so each account is followed down to empty and no further. An
// entry that takes more than there is takes what there is, and is listed.
//
// An undone entry and the correction that undid it are left out: together
// they moved nothing. Two devices that each ground a jar's last 2 g merge
// into an entry that took grams that were not there, and undoing one of them
// is how that is resolved — dated now, the undo cannot reach back to the
// moment of the grind, so the pair has to be passed over to clear it.
func Shortfalls(events []journal.Event) []Shortfall {
	held := map[string]map[journal.Account]journal.Centigrams{}
	var out []Shortfall
	for _, e := range Standing(events) {
		if e.Product == "" {
			continue
		}
		jar := held[e.Product]
		if jar == nil {
			jar = map[journal.Account]journal.Centigrams{}
			held[e.Product] = jar
		}
		cg := journal.CG(e.Grams)
		if e.From != journal.External {
			if have := jar[e.From]; cg > have {
				out = append(out, Shortfall{Event: e, Account: e.From, Held: have.Grams()})
				cg = have
			}
			jar[e.From] -= cg
		}
		if e.To != journal.External {
			jar[e.To] += journal.CG(e.Grams)
		}
	}
	return out
}

// Chronological returns a copy of the events in the order they happened,
// keeping the recorded order among entries at the same moment.
//
// Entries can be backdated, so recording order is not the order of events:
// a fill entered a month late used to fold into whichever cycle was open
// when it was typed in, however far back it was dated.
func Chronological(events []journal.Event) []journal.Event {
	out := slices.Clone(events)
	slices.SortStableFunc(out, func(a, b journal.Event) int {
		return a.OccurredAt.Compare(b.OccurredAt)
	})
	return out
}

// Fold replays every event and returns the state they describe. Events are
// folded in the order they happened (see Chronological), so a backdated entry
// lands where its date puts it rather than where it was typed in. Nothing is
// dated ahead: the recorder refuses an entry that has not happened yet.
func Fold(events []journal.Event) *State {
	s := &State{Balances: map[string]*Balance{}, Events: Chronological(events), lots: map[string][]lot{}}
	f := &folder{s: s, last: map[string]int{}, cur: -1, drew: map[string]journal.Centigrams{}}
	for _, e := range s.Events {
		f.apply(e)
	}
	return s
}

// apply folds one event into the running state.
func (f *folder) apply(e journal.Event) {
	s := f.s
	cg := journal.CG(e.Grams)
	b := s.balance(e.Product)
	b.apply(e.From, -cg)
	b.apply(e.To, cg)

	switch e.Type {
	case journal.Purchase:
		f.purchase(e)
	case journal.Grind:
		if f.cur != -1 {
			s.Cycles[f.cur].Ground = Shelf(plus(float64(s.Cycles[f.cur].Ground), e.Grams))
		}
		f.consume(e, cg)
	default:
		// Anything else that moves grams through storage — adjustments
		// down and up, corrections either way — settles the lots too, so
		// a jar reconciled to zero closes its cycles' claims.
		if e.From == journal.Storage {
			f.consume(e, cg)
		}
		if e.To == journal.Storage && e.Product != "" {
			if took, ok := f.drew[e.Reverts]; ok {
				cg = min(cg, took)
			}
			f.credit(e.Product, cg, e.OccurredAt)
		}
	}
	if f.cur != -1 {
		s.Cycles[f.cur].Events = append(s.Cycles[f.cur].Events, e)
	}
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

// CycleAt returns the fill in progress at t: the latest one dispensed by
// then, while anything on the shelf keeps a cycle open, or nil once every
// cycle has ground down to nothing. The latest fill is the prescription in
// progress even when an older cycle's jar outlives it.
//
// A replay asks at a moment in the past, when later fills had not been
// dispensed yet, so a fill that starts after t is not counted.
func (s *State) CycleAt(t time.Time) *Cycle {
	latest := -1
	open := false
	for i := range s.Cycles {
		if s.Cycles[i].Start.After(t) {
			break
		}
		latest = i
		open = open || s.Cycles[i].Open()
	}
	if latest < 0 || !open {
		return nil
	}
	return &s.Cycles[latest]
}

// ShareOf returns the grams of one product still standing on the cycle's own
// account — the jar's balance minus whatever older or newer fills hold in it.
func (s *State) ShareOf(c *Cycle, slug string) Fill { return Fill(s.shareOf(c, slug).Grams()) }

// shareOf is ShareOf in centigrams.
func (s *State) shareOf(c *Cycle, slug string) journal.Centigrams {
	var cg journal.Centigrams
	for _, l := range s.lots[slug] {
		if l.cycle == c.Seq {
			cg += l.cg
		}
	}
	return cg
}

// FillGround splits what the cycle's fill of a product has had ground — what
// it dispensed less what of it is still sealed — into stash and AVB, each
// capped to what is left of it.
//
// The stash and the AVB jar hold grams of every fill of a product, and they
// cannot be split by fill: only storage is kept in lots. Shown beside a fill's
// own storage they put an older fill's grams on this one's account, and a
// row could add up past what the fill dispensed. Capped to the fill's ground
// grams they stay within it; on a shelf where each product came from one
// fill they are the jars' figures exactly.
func (s *State) FillGround(c *Cycle, slug string) (stash, avb Fill) {
	ground := c.PurchasedOf(slug) - s.ShareOf(c, slug)
	b := s.Balances[slug]
	if b == nil || ground <= 0 {
		return 0, 0
	}
	stash = min(ground, Fill(b.Stash))
	avb = min(ground-stash, Fill(b.AVB))
	return stash, avb
}

// FillOnShelf returns the grams of the cycle's fill still in storage: its
// own lots, summed over its products.
func (s *State) FillOnShelf(c *Cycle) Fill {
	var cg journal.Centigrams
	for _, slug := range c.Products {
		cg += s.shareOf(c, slug)
	}
	return Fill(cg.Grams())
}

// CarriedOnShelf returns the storage still standing on other cycles'
// accounts — the older fills' remainders — with the jars holding it and the
// cycles still open for it.
func (s *State) CarriedOnShelf(c *Cycle) (grams Shelf, jars, cycles int) {
	open := map[int]bool{}
	var total journal.Centigrams
	for _, q := range s.lots {
		var other journal.Centigrams
		for _, l := range q {
			if l.cycle != c.Seq && l.cg > 0 {
				other += l.cg
				open[l.cycle] = true
			}
		}
		if other > 0 {
			jars++
			total += other
		}
	}
	return Shelf(total.Grams()), jars, len(open)
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

// DaysWorth is how many days of grinding the grams amount to at the observed
// rate, or 0 when there is no rate to go by.
//
// It is not when they run out. Grams drain oldest first, so a fill's own are
// not touched until the older jars standing beside it are empty: "lasts ~19
// days" on the fill's card read as a date, and disagreed with the shelf's
// projection under it, which counts the older jars too.
func (st Stats) DaysWorth(grams float64) float64 {
	if st.PerActiveDay <= 0 {
		return 0
	}
	return Round(grams / st.PerActiveDay)
}

// Summarise returns the statistics for the grind events among the given events.
// Grinding is what the spreadsheet recorded and what there is history for;
// consumption out of the stash is tracked separately.
func Summarise(events []journal.Event) Stats {
	perDay := map[string]journal.Centigrams{}
	var ground journal.Centigrams
	var st Stats
	for _, e := range events {
		if e.Type != journal.Grind {
			continue
		}
		perDay[e.OccurredAt.Format(time.DateOnly)] += journal.CG(e.Grams)
		ground += journal.CG(e.Grams)
		if st.First.IsZero() || e.OccurredAt.Before(st.First) {
			st.First = e.OccurredAt
		}
		if e.OccurredAt.After(st.Last) {
			st.Last = e.OccurredAt
		}
	}

	st.Ground = ground.Grams()
	amounts := make([]float64, 0, len(perDay))
	for _, cg := range perDay {
		if cg > 0 {
			amounts = append(amounts, cg.Grams())
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

// In returns the grams the balance holds in an account; External holds none.
func (b Balance) In(account journal.Account) float64 {
	if p := b.slot(account); p != nil {
		return *p
	}
	return 0
}

// apply moves centigrams into an account on a balance, adding in centigrams
// so the balance is exact however many entries it is the sum of. Movements to
// and from External fall outside the tracked accounts and are ignored.
func (b *Balance) apply(account journal.Account, cg journal.Centigrams) {
	if p := b.slot(account); p != nil {
		*p = (journal.CG(*p) + cg).Grams()
	}
}

// slot is the field holding an account's grams, or nil for External.
func (b *Balance) slot(account journal.Account) *float64 {
	switch account {
	case journal.Storage:
		return &b.Storage
	case journal.Stash:
		return &b.Stash
	case journal.Consumed:
		return &b.Consumed
	case journal.AVB:
		return &b.AVB
	}
	return nil
}

// plus adds two amounts in centigrams.
func plus(a, b float64) float64 { return (journal.CG(a) + journal.CG(b)).Grams() }

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

// ParseGrams reads an amount as typed — "0.75", "0.75g", "0,75 g" — and
// refuses one finer than a centigram, which nothing after the journal keeps:
// the screens show two places and a bundle stores centigrams, so 0.333 g came
// back from a restore as 0.33 g. Every place an amount is typed in, the
// commands and every form, reads it here; it was seven copies, each a little
// different. Whether zero or less is allowed is the caller's to say.
func ParseGrams(s string) (float64, error) {
	t := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.ToLower(s)), "g"))
	grams, err := strconv.ParseFloat(strings.Replace(t, ",", ".", 1), 64)
	if err != nil || math.IsNaN(grams) || math.IsInf(grams, 0) {
		return 0, fmt.Errorf("%q is not an amount in grams", strings.TrimSpace(s))
	}
	if !journal.Whole(grams) {
		return 0, fmt.Errorf("%s g is finer than a scale reads; give it to two decimal places", t)
	}
	return grams, nil
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

// Refill is when to order the next prescription: the day the shelf runs out
// at the fill's rate, less the pharmacy's lead time.
type Refill struct {
	// Shelf is what stands in storage today, every jar — the projection's
	// level, not the fill's own grams, since jars drain oldest first.
	Shelf Shelf
	// Rate is the fill's grams per day with grinding, the rate every
	// projection runs at.
	Rate float64
	// EmptyOn is the day the shelf runs out; OrderBy, Lead days before it.
	EmptyOn, OrderBy time.Time
	Lead             int
}

// Overdue reports whether the day to order by has passed.
func (r Refill) Overdue(now time.Time) bool { return midnight(r.OrderBy).Before(midnight(now)) }

// PlanRefill is when to order, as the supply projection sees it: the shelf
// held when the fill arrived, less every grind since, running down from now
// at the fill's rate. It is false without a fill or a rate to go by.
func PlanRefill(c *Cycle, now time.Time, lead int) (Refill, bool) {
	if c == nil {
		return Refill{}, false
	}
	shelf := journal.CG(float64(c.Held()))
	for _, e := range c.Events {
		if e.Type == journal.Grind && !e.OccurredAt.After(now) {
			shelf -= journal.CG(e.Grams)
		}
	}
	rate := Summarise(c.Events).PerActiveDay
	if rate <= 0 {
		return Refill{}, false
	}
	shelf = max(shelf, 0)
	empty := midnight(now).AddDate(0, 0, int(math.Ceil(shelf.Grams()/rate)))
	return Refill{Shelf: Shelf(shelf.Grams()), Rate: rate, EmptyOn: empty,
		OrderBy: empty.AddDate(0, 0, -lead), Lead: lead}, true
}

// midnight is the start of the calendar day a moment falls on, in its zone.
func midnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// Standing is the entries that stand, in the order they happened: an undone
// entry and the undo that undid it moved nothing between them, and neither
// counts.
func Standing(events []journal.Event) []journal.Event {
	undone := map[string]bool{}
	for _, e := range events {
		if e.Reverts != "" {
			undone[e.Reverts] = true
		}
	}
	var out []journal.Event
	for _, e := range Chronological(events) {
		if e.Reverts == "" && !undone[e.Hash] {
			out = append(out, e)
		}
	}
	return out
}

// Uncollected is, by device, the sessions put through it since its last
// collection, up to at: what the AVB weighed out of it now came from. A
// session named no device is under "".
func Uncollected(events []journal.Event, at time.Time) map[string][]journal.Event {
	out := map[string][]journal.Event{}
	for _, e := range Standing(events) {
		if e.OccurredAt.After(at) {
			break
		}
		switch e.Type {
		case journal.Sesh:
			out[e.Device] = append(out[e.Device], e)
		case journal.AVBCollect:
			delete(out, e.Device)
		}
	}
	return out
}

// Yield is one emptying of a vaporizer: the AVB weighed out of it, against
// the grams its sessions put in since the last.
type Yield struct {
	At        time.Time
	Device    string
	Collected float64
	From      float64
	Sessions  int
}

// Ratio is the share of what went in that came back out as AVB.
func (y Yield) Ratio() float64 {
	if y.From <= 0 {
		return 0
	}
	return y.Collected / y.From
}

// Yields is every collection, oldest first. A collection is written as an
// entry a product, all at one moment from one device, so the entries are put
// back together by moment and device.
func Yields(events []journal.Event) []Yield {
	type pending struct {
		grams    journal.Centigrams
		sessions int
	}
	put := map[string]*pending{}
	var out []Yield
	for _, e := range Standing(events) {
		switch e.Type {
		case journal.Sesh:
			p := put[e.Device]
			if p == nil {
				p = &pending{}
				put[e.Device] = p
			}
			p.grams += journal.CG(e.Grams)
			p.sessions++
		case journal.AVBCollect:
			if n := len(out); n > 0 && out[n-1].At.Equal(e.OccurredAt) && out[n-1].Device == e.Device {
				out[n-1].Collected = plus(out[n-1].Collected, e.Grams)
				continue
			}
			y := Yield{At: e.OccurredAt, Device: e.Device, Collected: e.Grams}
			if p := put[e.Device]; p != nil {
				y.From, y.Sessions = p.grams.Grams(), p.sessions
			}
			delete(put, e.Device)
			out = append(out, y)
		}
	}
	return out
}

// Split shares total out between the keys in proportion to their weights,
// in whole centigrams that add up to it exactly: each its share rounded down,
// and the centigrams left over to the largest remainders, ties by key. A key
// whose share is nothing is left out.
func Split(total journal.Centigrams, weights map[string]journal.Centigrams) map[string]journal.Centigrams {
	var sum journal.Centigrams
	keys := make([]string, 0, len(weights))
	for k, w := range weights {
		if w > 0 {
			sum += w
			keys = append(keys, k)
		}
	}
	out := map[string]journal.Centigrams{}
	if sum == 0 || total <= 0 {
		return out
	}
	sort.Strings(keys)
	rest := map[string]journal.Centigrams{}
	left := total
	for _, k := range keys {
		out[k] = total * weights[k] / sum
		rest[k] = total * weights[k] % sum
		left -= out[k]
	}
	sort.SliceStable(keys, func(i, j int) bool { return rest[keys[i]] > rest[keys[j]] })
	for i := 0; left > 0; i++ {
		out[keys[i%len(keys)]]++
		left--
	}
	for k, v := range out {
		if v == 0 {
			delete(out, k)
		}
	}
	return out
}
