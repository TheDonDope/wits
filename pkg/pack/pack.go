// Package pack is what to take on a trip: the grams for the days away at the
// rate the refill planner runs on, with a margin, drawn from the jars on the
// shelf — evenly, so every jar goes down by about the same grams and the trip
// has a mix of them, or oldest fill first — each jar's ground grams before its
// sealed ones. A plan names its jars as a statement of what is carried does,
// so exactly the grams packed can be signed (wits pack, the Trip page).
package pack

import (
	"cmp"
	"errors"
	"math"
	"slices"
	"strings"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/report"
	"github.com/TheDonDope/wits/pkg/seal"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// Mode is how a plan draws on the jars.
type Mode string

const (
	// Even takes about the same grams from every jar on the shelf.
	Even Mode = "even"
	// Oldest empties the jars of the oldest fill first.
	Oldest Mode = "oldest"
)

// DefaultMargin is the share taken on top, in per cent: a day's delay, a
// heavier evening.
const DefaultMargin = 20

// ErrNoRate is why there is nothing to plan by: no fill, or none ground yet.
var ErrNoRate = errors.New("there is no rate to pack by: grind from this fill first")

// step is what a share is rounded up to: a tenth of a gram, which a pocket
// scale shows and a hand can portion.
const step = 10 // centigrams

// Plan is what to take.
type Plan struct {
	Days   int
	Margin int
	Mode   Mode
	// Rate is the fill's grams a day with grinding, the refill planner's.
	Rate float64
	// Need is the days at the rate with the margin, rounded up to a tenth.
	Need float64
	// Packed is what the lines add up to: Need, or all there is when Short.
	Packed float64
	// Shelf is every gram the jars on the shelf hold, ground or sealed.
	Shelf float64
	// Short is whether the shelf holds less than the trip needs.
	Short bool
	Lines []Line
}

// Line is what one jar gives the trip.
type Line struct {
	Name string
	// Stash and Sealed are what is taken of each; HeldStash and HeldSealed,
	// what the jar holds of each before.
	Stash, Sealed         float64
	HeldStash, HeldSealed float64
	Fills                 []seal.Dispensed
	// Days is what is taken, in days at the rate.
	Days float64
}

// Taken is what the line takes in all.
func (l Line) Taken() float64 { return (journal.CG(l.Stash) + journal.CG(l.Sealed)).Grams() }

// Make plans days away with margin per cent on top.
func Make(ws *workspace.Workspace, days, margin int, mode Mode) (Plan, error) {
	if days < 1 {
		return Plan{}, errors.New("a trip is a day or more")
	}
	if margin < 0 {
		return Plan{}, errors.New("a margin is nothing or more")
	}
	refill, ok := ledger.PlanRefill(ws.Cycle(), ws.OpenedAt, 0)
	if !ok {
		return Plan{}, ErrNoRate
	}
	if mode != Oldest {
		mode = Even
	}
	p := Plan{Days: days, Margin: margin, Mode: mode, Rate: refill.Rate}
	need := journal.Centigrams(math.Ceil(refill.Rate*float64(days)*(1+float64(margin)/100)*100/step) * step)
	p.Need = need.Grams()
	jars := shelf(ws)
	var all journal.Centigrams
	for _, j := range jars {
		all += held(j)
	}
	p.Shelf = all.Grams()
	if all < need {
		p.Short, need = true, all
	}
	var takes []journal.Centigrams
	if mode == Oldest {
		takes = oldest(jars, need)
	} else {
		takes = even(jars, need)
	}
	var packed journal.Centigrams
	for i, j := range jars {
		if takes[i] <= 0 {
			continue
		}
		stash := min(takes[i], journal.CG(j.Ground))
		l := Line{Name: j.Name, Stash: stash.Grams(), Sealed: (takes[i] - stash).Grams(),
			HeldStash: j.Ground, HeldSealed: j.Sealed, Fills: j.Fills, Days: ledger.Round(takes[i].Grams() / refill.Rate)}
		p.Lines = append(p.Lines, l)
		packed += takes[i]
	}
	p.Packed = packed.Grams()
	return p, nil
}

// shelf is the jars on the shelf, oldest fill first: what a statement of
// what is carried lists, without the stashes left from years ago.
func shelf(ws *workspace.Workspace) []seal.Item {
	jars := report.Holdings(ws, false)
	slices.SortStableFunc(jars, func(a, b seal.Item) int {
		return cmp.Or(strings.Compare(firstDay(a), firstDay(b)), strings.Compare(a.Name, b.Name))
	})
	return jars
}

func firstDay(it seal.Item) string {
	if len(it.Fills) == 0 {
		return ""
	}
	return it.Fills[0].Day
}

func held(it seal.Item) journal.Centigrams { return journal.CG(it.Ground) + journal.CG(it.Sealed) }

// oldest takes each jar whole, oldest first, until the need is met.
func oldest(jars []seal.Item, need journal.Centigrams) []journal.Centigrams {
	out := make([]journal.Centigrams, len(jars))
	for i, j := range jars {
		if need <= 0 {
			break
		}
		out[i] = min(held(j), need)
		need -= out[i]
	}
	return out
}

// even shares the need out equally over the jars, a jar that holds less than
// its share giving all it has and the rest shared again over the others; each
// share is rounded up to a tenth of a gram, never past what its jar holds.
func even(jars []seal.Item, need journal.Centigrams) []journal.Centigrams {
	out := make([]journal.Centigrams, len(jars))
	open := make([]int, 0, len(jars))
	for i, j := range jars {
		if held(j) > 0 {
			open = append(open, i)
		}
	}
	for len(open) > 0 && need > 0 {
		fair := need / journal.Centigrams(len(open))
		var still []int
		for _, i := range open {
			if held(jars[i]) <= fair {
				out[i] = held(jars[i])
				need -= out[i]
			} else {
				still = append(still, i)
			}
		}
		if len(still) == len(open) {
			share(jars, out, open, need)
			break
		}
		open = still
	}
	return out
}

// share gives each open jar the need shared out in tenths of a gram: the
// same to each, rounded down, and a tenth more to each of the jars that keep
// the most until the shares add up to the need. Rounded down, what is left
// over is fewer tenths than there are jars, so no jar gets two.
func share(jars []seal.Item, out []journal.Centigrams, open []int, need journal.Centigrams) {
	each := need / journal.Centigrams(len(open)) / step * step
	for _, i := range open {
		out[i] = each
	}
	byLeft := slices.Clone(open)
	slices.SortStableFunc(byLeft, func(a, b int) int { return cmp.Compare(held(jars[b]), held(jars[a])) })
	rest := need - each*journal.Centigrams(len(open))
	for _, i := range byLeft {
		if rest <= 0 {
			return
		}
		give := min(step, rest, held(jars[i])-out[i])
		out[i] += give
		rest -= give
	}
}

// Statement is the plan as a statement of what is carried: exactly the grams
// packed, each jar with the fills that dispensed it.
func (p Plan) Statement() []seal.Item {
	out := make([]seal.Item, 0, len(p.Lines))
	for _, l := range p.Lines {
		out = append(out, seal.Item{Name: l.Name, Sealed: l.Sealed, Ground: l.Stash, Fills: l.Fills})
	}
	return out
}
