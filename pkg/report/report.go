// Package report is the one page for the prescriber: the months asked for,
// grams a day, which strains, what the sessions did by the effect log, and
// what is left of the fill in progress — as a PDF (pdf.go).
//
// Beside it are the other views the commands and the server both draw from a
// workspace: what is held, for wits carry (carry.go), and the map of the
// strains (strains.go).
package report

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/seal"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// Report is what the page says, every figure for the days asked for unless it
// says otherwise: what is left now is the fill's and the shelf's, today.
type Report struct {
	From, To  time.Time
	Generated time.Time
	Version   string
	Days      int

	Ground       float64
	GrindDays    int
	PerGrindDay  float64
	MedianPerDay float64
	PerDay       float64
	Dispensed    float64
	Purchases    int
	Seshed       float64
	Sessions     int

	Products []Use
	Weeks    []Week
	Rated    int
	Effects  []Effect
	Best     []effects.Group
	Now      *Now

	Entries int
	Head    string
	// Seal is the seal of the head, when it has one: who vouches for the
	// journal this page was made from.
	Seal *seal.Seal
}

// Use is what one product went through in the days asked for.
type Use struct {
	Name               string
	THC, CBD           float64
	Dispensed, Ground  float64
	Seshed             float64
	SessionsWithRating int
}

// Week is what was ground in the seven days from Start.
type Week struct {
	Start  time.Time
	Ground float64
}

// Effect is one dimension's mean over the sessions rated on it.
type Effect struct {
	Dimension effects.Dimension
	Mean      float64
	Count     int
}

// Now is the fill in progress and the shelf, today.
type Now struct {
	Fill               int
	Since              time.Time
	Dispensed, Left    float64
	Shelf              float64
	EmptyOn, OrderBy   time.Time
	HasRefill          bool
	OlderJars, Carried float64
}

// Build gathers the report for the days from from to to, both included.
func Build(ws *workspace.Workspace, from, to time.Time, version string) Report {
	from, to = midnight(from), midnight(to)
	r := Report{From: from, To: to, Generated: ws.OpenedAt, Version: version,
		Days: int(to.Sub(from).Hours()/24) + 1, Entries: len(ws.State.Events)}
	if events, err := ws.Journal().Events(); err == nil && len(events) > 0 {
		r.Head = events[len(events)-1].Hash
		r.Seal = sealOf(ws, r.Head)
	}
	var in []journal.Event
	for _, e := range ws.State.Standing {
		if !e.OccurredAt.Before(from) && e.OccurredAt.Before(to.AddDate(0, 0, 1)) {
			in = append(in, e)
		}
	}
	stats := ledger.Summarise(in)
	r.Ground, r.GrindDays, r.PerGrindDay, r.MedianPerDay = stats.Ground, stats.ActiveDays, stats.PerActiveDay, stats.MedianPerDay
	r.PerDay = ledger.Round(stats.Ground / float64(r.Days))
	r.Products = uses(ws, in, &r)
	r.Weeks = weeks(in, from, to)
	r.Rated, r.Effects = rated(ws.Ratings, in)
	insights := effects.Analyse(ws.Ratings, in, ws.ProductName)
	r.Best = best(insights.ByProduct)
	r.Now = now(ws)
	return r
}

// uses totals the range by product, the most ground first, and the range's
// purchases and sessions with them.
func uses(ws *workspace.Workspace, in []journal.Event, r *Report) []Use {
	by := map[string]*Use{}
	var dispensed, seshed journal.Centigrams
	for _, e := range in {
		if e.Product == "" {
			continue
		}
		u := by[e.Product]
		if u == nil {
			u = &Use{Name: ws.ProductName(e.Product)}
			if p, err := ws.Products.Find(e.Product); err == nil {
				u.THC, u.CBD = p.THC, p.CBD
			}
			by[e.Product] = u
		}
		switch e.Type {
		case journal.Purchase:
			u.Dispensed = ledger.Round(u.Dispensed + e.Grams)
			dispensed += journal.CG(e.Grams)
			r.Purchases++
		case journal.Grind:
			u.Ground = ledger.Round(u.Ground + e.Grams)
		case journal.Sesh:
			u.Seshed = ledger.Round(u.Seshed + e.Grams)
			seshed += journal.CG(e.Grams)
			r.Sessions++
		}
	}
	r.Dispensed, r.Seshed = dispensed.Grams(), seshed.Grams()
	out := make([]Use, 0, len(by))
	for _, u := range by {
		out = append(out, *u)
	}
	slices.SortFunc(out, func(a, b Use) int {
		return cmp.Or(cmp.Compare(b.Ground+b.Dispensed, a.Ground+a.Dispensed), strings.Compare(a.Name, b.Name))
	})
	return out
}

// weeks is the grinding in each seven days of the range, the last week cut
// short where the range ends.
func weeks(in []journal.Event, from, to time.Time) []Week {
	var out []Week
	for d := from; !d.After(to); d = d.AddDate(0, 0, 7) {
		out = append(out, Week{Start: d})
	}
	for _, e := range in {
		if e.Type != journal.Grind {
			continue
		}
		i := int(midnight(e.OccurredAt).Sub(from).Hours() / 24 / 7)
		if i >= 0 && i < len(out) {
			out[i].Ground = ledger.Round(out[i].Ground + e.Grams)
		}
	}
	return out
}

// rated is how many of the range's sessions were rated, and each dimension's
// mean over the ones rated on it.
func rated(ratings []effects.Rating, in []journal.Event) (int, []Effect) {
	latest := effects.Latest(ratings)
	sums := map[effects.Dimension][2]int{}
	n := 0
	for _, e := range in {
		r, ok := latest[e.Hash]
		if e.Type != journal.Sesh || !ok {
			continue
		}
		n++
		for d, v := range r.Scores {
			s := sums[d]
			sums[d] = [2]int{s[0] + v, s[1] + 1}
		}
	}
	var out []Effect
	for _, d := range effects.Dimensions {
		if s := sums[d]; s[1] > 0 {
			out = append(out, Effect{Dimension: d, Mean: float64(s[0]) / float64(s[1]), Count: s[1]})
		}
	}
	return n, out
}

// best is up to three products by their overall mean, the most rated first
// among equals.
func best(groups []effects.Group) []effects.Group {
	var out []effects.Group
	for _, g := range groups {
		if g.Counts[effects.Overall] > 0 {
			out = append(out, g)
		}
	}
	slices.SortFunc(out, func(a, b effects.Group) int {
		return cmp.Or(cmp.Compare(b.Means[effects.Overall], a.Means[effects.Overall]),
			cmp.Compare(b.Counts[effects.Overall], a.Counts[effects.Overall]))
	})
	return out[:min(3, len(out))]
}

// now is the fill in progress and the shelf, today, with when to order.
func now(ws *workspace.Workspace) *Now {
	c := ws.Cycle()
	if c == nil {
		return nil
	}
	n := &Now{Fill: c.Seq + 1, Since: c.Start, Dispensed: float64(c.Purchased), Left: float64(ws.State.FillOnShelf(c))}
	carried, _, _ := ws.State.CarriedOnShelf(c)
	n.Carried = float64(carried)
	for _, b := range ws.State.Balances {
		n.Shelf = ledger.Round(n.Shelf + b.Storage)
	}
	if plan, ok := ledger.PlanRefill(c, ws.OpenedAt, ws.Repo.Config.Lead()); ok {
		n.EmptyOn, n.OrderBy, n.HasRefill = plan.EmptyOn, plan.OrderBy, true
	}
	return n
}

// sealOf is the kept seal of the head, if there is one that verifies.
func sealOf(ws *workspace.Workspace, head string) *seal.Seal {
	kept, err := seal.Store{Dir: ws.Repo.SealsDir()}.All()
	if err != nil {
		return nil
	}
	for _, f := range kept {
		if s, err := seal.Parse(f.Text); err == nil && s.Head == head && s.Verify() == nil {
			return &s
		}
	}
	return nil
}

func midnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
