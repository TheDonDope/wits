// Package review is the year on one card: the grams, the strains, the longest
// break, the temperature rated best, the dearest gram and the cheapest. Every
// figure is the year's own — the days from New Year to New Year's Eve, or to
// today in the year still running — and is one the report, the effect log or
// the price log already computes, gathered here rather than worked out anew.
package review

import (
	"cmp"
	"slices"
	"time"

	"github.com/TheDonDope/wits/pkg/cannabis"
	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/prices"
	"github.com/TheDonDope/wits/pkg/report"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// BreakDays is how long a break is at the least: a week with nothing ground
// and no session, the tolerance curve's rule, so the two name the same breaks.
const BreakDays = 7

// minRated is how many ratings a temperature band needs before it is named
// the best: a band rated once is a rating, not a finding.
const minRated = 5

// Review is one year in wits.
type Review struct {
	Year     int
	From, To time.Time
	Days     int
	// Running is whether the year is still going: To is then today.
	Running bool

	Ground      float64
	GrindDays   int
	PerGrindDay float64
	Sessions    int
	Seshed      float64
	Dispensed   float64
	Pickups     int
	Products    int

	// Months is the grams ground in each of the year's months, January first.
	Months [12]float64
	// Days by day: what was ground and seshed on each day with either.
	Daily []Day
	// Strains is the year's strains, the most ground first.
	Strains []Strain
	// Busiest is the day the most was ground on.
	Busiest Day

	Break   *Break
	Best    *Band
	Liked   *Liked
	Device  *Device
	Terpene *Terpene

	Paid     int64
	Priced   float64
	Dearest  *Bought
	Cheapest *Bought
	Bought   []Bought
}

// Day is what one day saw.
type Day struct {
	Day    time.Time
	Ground float64
	Seshed float64
}

// Strain is one product's year.
type Strain struct {
	Name   string
	Ground float64
	// Share is its part of the year's grams ground, 0 to 1.
	Share float64
}

// Break is the longest stretch of the year with nothing ground and no
// session: From and To are its first and last quiet days.
type Break struct {
	From, To time.Time
	Days     int
	// Breaks is how many breaks the year held, this one among them.
	Breaks int
}

// Band is the temperature band rated best overall.
type Band struct {
	Label string
	Mean  float64
	Count int
}

// Liked is the strain rated best overall over its sessions of the year.
type Liked struct {
	Name  string
	Mean  float64
	Count int
}

// Device is the device most sessions of the year were had with.
type Device struct {
	Name     string
	Sessions int
	Seshed   float64
}

// Terpene is the terpene most of the year's grams led with.
type Terpene struct {
	Name    string
	Flavors []string
	// Share is the part of the grams ground whose product leads with it.
	Share float64
}

// Bought is a purchase of the year with its price.
type Bought struct {
	At      time.Time
	Name    string
	Seller  string
	Grams   float64
	Cents   int64
	PerGram float64 // cents
}

// Years is every year the journal has an entry in, oldest first.
func Years(ws *workspace.Workspace) []int {
	var out []int
	for _, e := range ws.State.Standing {
		if y := e.OccurredAt.Year(); !slices.Contains(out, y) {
			out = append(out, y)
		}
	}
	slices.Sort(out)
	return out
}

// Year gathers the year, measured against the workspace's moment.
func Year(ws *workspace.Workspace, year int) Review {
	loc := ws.OpenedAt.Location()
	from := time.Date(year, time.January, 1, 0, 0, 0, 0, loc)
	to := time.Date(year, time.December, 31, 0, 0, 0, 0, loc)
	r := Review{Year: year, From: from, To: to}
	if today := midnight(ws.OpenedAt); today.Before(to) {
		r.To, r.Running = today, true
	}
	r.Days = int(r.To.Sub(from).Hours()/24) + 1
	rep := report.Build(ws, r.From, r.To, "")
	r.Ground, r.GrindDays, r.PerGrindDay = rep.Ground, rep.GrindDays, rep.PerGrindDay
	r.Sessions, r.Seshed, r.Dispensed, r.Pickups = rep.Sessions, rep.Seshed, rep.Dispensed, pickups(ws, from, r.To)
	in := within(ws.State.Standing, from, r.To)
	r.Daily, r.Months, r.Busiest = days(in)
	r.Strains = strains(rep.Products, r.Ground)
	r.Products = len(r.Strains)
	r.Break = longest(r.Daily, from, r.To)
	insights := effects.Analyse(ws.Ratings, in, ws.ProductName)
	r.Best = bestBand(insights.ByTemp)
	r.Liked = liked(rep.Best)
	r.Device = device(ws, in)
	r.Terpene = terpene(ws, in, r.Ground)
	priced(ws, &r)
	return r
}

// within is the standing entries from the first day to the last, both in.
func within(standing []journal.Event, from, to time.Time) []journal.Event {
	var out []journal.Event
	end := to.AddDate(0, 0, 1)
	for _, e := range standing {
		if !e.OccurredAt.Before(from) && e.OccurredAt.Before(end) {
			out = append(out, e)
		}
	}
	return out
}

// pickups is how many fills opened in the days: prescriptions picked up.
func pickups(ws *workspace.Workspace, from, to time.Time) int {
	n := 0
	for _, c := range ws.State.Cycles {
		if !c.Start.Before(from) && c.Start.Before(to.AddDate(0, 0, 1)) {
			n++
		}
	}
	return n
}

// days is the year day by day, by month, and its busiest day.
func days(in []journal.Event) ([]Day, [12]float64, Day) {
	var months [12]journal.Centigrams
	by := map[time.Time]*[2]journal.Centigrams{}
	for _, e := range in {
		if e.Type != journal.Grind && e.Type != journal.Sesh {
			continue
		}
		d := midnight(e.OccurredAt)
		if by[d] == nil {
			by[d] = &[2]journal.Centigrams{}
		}
		if e.Type == journal.Grind {
			by[d][0] += journal.CG(e.Grams)
			months[e.OccurredAt.Month()-1] += journal.CG(e.Grams)
		} else {
			by[d][1] += journal.CG(e.Grams)
		}
	}
	out := make([]Day, 0, len(by))
	for d, g := range by {
		out = append(out, Day{Day: d, Ground: g[0].Grams(), Seshed: g[1].Grams()})
	}
	slices.SortFunc(out, func(a, b Day) int { return a.Day.Compare(b.Day) })
	var busiest Day
	for _, d := range out {
		if d.Ground > busiest.Ground {
			busiest = d
		}
	}
	var m [12]float64
	for i, g := range months {
		m[i] = g.Grams()
	}
	return out, m, busiest
}

// strains is the year's products that were ground, with their share of it.
func strains(uses []report.Use, ground float64) []Strain {
	var out []Strain
	for _, u := range uses {
		if u.Ground <= 0 {
			continue
		}
		s := Strain{Name: u.Name, Ground: u.Ground}
		if ground > 0 {
			s.Share = u.Ground / ground
		}
		out = append(out, s)
	}
	slices.SortFunc(out, func(a, b Strain) int { return cmp.Compare(b.Ground, a.Ground) })
	return out
}

// longest is the longest run of quiet days between from and to, and how many
// runs were a break; nil when the year held none.
func longest(daily []Day, from, to time.Time) *Break {
	// By the calendar date, not the instant: an entry keeps the zone it was
	// written in, and midnight there is not midnight here.
	busy := map[string]bool{}
	for _, d := range daily {
		busy[d.Day.Format(time.DateOnly)] = true
	}
	var best *Break
	n, run := 0, 0
	var start time.Time
	closeRun := func(end time.Time) {
		if run >= BreakDays {
			n++
			if best == nil || run > best.Days {
				best = &Break{From: start, To: end, Days: run}
			}
		}
		run = 0
	}
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		if busy[d.Format(time.DateOnly)] {
			closeRun(d.AddDate(0, 0, -1))
			continue
		}
		if run == 0 {
			start = d
		}
		run++
	}
	closeRun(to)
	if best != nil {
		best.Breaks = n
	}
	return best
}

// bestBand is the temperature band with the best overall mean, among those
// rated often enough to say so.
func bestBand(groups []effects.Group) *Band {
	var best *Band
	for _, g := range groups {
		n := g.Counts[effects.Overall]
		if n < minRated {
			continue
		}
		if m := g.Means[effects.Overall]; best == nil || m > best.Mean || (m == best.Mean && n > best.Count) {
			best = &Band{Label: g.Label, Mean: m, Count: n}
		}
	}
	return best
}

// liked is the report's best-rated product of the days, if any was rated.
func liked(best []effects.Group) *Liked {
	if len(best) == 0 {
		return nil
	}
	g := best[0]
	return &Liked{Name: g.Label, Mean: g.Means[effects.Overall], Count: g.Counts[effects.Overall]}
}

// device is the device with the most sessions in the days.
func device(ws *workspace.Workspace, in []journal.Event) *Device {
	by := map[string]*Device{}
	for _, e := range in {
		if e.Type != journal.Sesh || e.Device == "" {
			continue
		}
		d := by[e.Device]
		if d == nil {
			d = &Device{Name: e.Device}
			if dev, err := ws.Devices.Find(e.Device); err == nil && dev != nil {
				d.Name = dev.Name
			}
			by[e.Device] = d
		}
		d.Sessions++
		d.Seshed = ledger.Round(d.Seshed + e.Grams)
	}
	var best *Device
	for _, d := range by {
		if best == nil || d.Sessions > best.Sessions || (d.Sessions == best.Sessions && d.Name < best.Name) {
			best = d
		}
	}
	return best
}

// terpene is the terpene the most grams ground led with: each grind counts
// for its product's dominant terpene, when the product has one written down.
func terpene(ws *workspace.Workspace, in []journal.Event, ground float64) *Terpene {
	by := map[string]float64{}
	for _, e := range in {
		if e.Type != journal.Grind {
			continue
		}
		p, err := ws.Products.Find(e.Product)
		if err != nil || p == nil || len(p.Terpenes) == 0 {
			continue
		}
		by[p.Terpenes[0]] += e.Grams
	}
	var name string
	for t, g := range by {
		if name == "" || g > by[name] || (g == by[name] && t < name) {
			name = t
		}
	}
	if name == "" || ground <= 0 {
		return nil
	}
	t := &Terpene{Name: name, Share: by[name] / ground}
	if known := cannabis.Lookup(name); known != nil {
		t.Name, t.Flavors = known.Name, known.Flavors
	}
	return t
}

// priced is what the year's purchases cost, and its dearest and cheapest gram.
func priced(ws *workspace.Workspace, r *Review) {
	product := func(slug string) prices.Product {
		if p, err := ws.Products.Find(slug); err == nil && p != nil {
			return prices.Product{Name: p.Name, Maker: p.Manufacturer, Cultivar: p.Cultivar, THC: p.THC, CBD: p.CBD}
		}
		return prices.Product{Name: slug}
	}
	end := r.To.AddDate(0, 0, 1)
	var grams journal.Centigrams
	for _, b := range prices.Insight(ws.State, prices.Latest(ws.Prices), product).Bought {
		if b.At.Before(r.From) || !b.At.Before(end) || b.Grams <= 0 {
			continue
		}
		it := Bought{At: b.At, Name: b.Name, Seller: b.Seller, Grams: b.Grams, Cents: b.Cents, PerGram: b.PerGram()}
		r.Bought = append(r.Bought, it)
		r.Paid += b.Cents
		grams += journal.CG(b.Grams)
	}
	r.Priced = grams.Grams()
	r.Dearest, r.Cheapest = extremes(r.Bought)
}

// extremes is the dearest and the cheapest purchase a gram, the first of equals.
func extremes(bought []Bought) (dear, cheap *Bought) {
	for i := range bought {
		if dear == nil || bought[i].PerGram > dear.PerGram {
			dear = &bought[i]
		}
		if cheap == nil || bought[i].PerGram < cheap.PerGram {
			cheap = &bought[i]
		}
	}
	return dear, cheap
}

// PerGram is what a gram of the year cost, in cents, over the grams with a price.
func (r Review) PerGram() float64 {
	if r.Priced <= 0 {
		return 0
	}
	return float64(r.Paid) / r.Priced
}

func midnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
