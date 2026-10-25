// Package report: what is held today, for a signed statement of it (wits carry), and its QR code.

package report

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/seal"
	"github.com/TheDonDope/wits/pkg/workspace"
	"rsc.io/qr"
)

// Holdings is what is held today, by product, for a statement of what is
// carried: each product's sealed, ground and already vaped grams, the jar's
// own balances, and the fills its sealed grams stand on — or, once they are
// all ground, the last fill of it. Without all, only the products on the
// shelf: a stash left from years ago is the ledger's, not what is carried.
func Holdings(ws *workspace.Workspace, all bool) []seal.Item {
	var out []seal.Item
	for slug, b := range ws.State.Balances {
		if b.Storage <= 0 && (!all || b.Stash+b.AVB <= 0) {
			continue
		}
		it := seal.Item{Name: ws.ProductName(slug), Sealed: b.Storage, Ground: b.Stash, AVB: b.AVB, Fills: fillsOf(ws, slug)}
		out = append(out, it)
	}
	slices.SortFunc(out, func(a, b seal.Item) int {
		return cmp.Or(cmp.Compare(b.Sealed, a.Sealed), strings.Compare(a.Name, b.Name))
	})
	return out
}

// fillsOf is the purchases a product's sealed grams stand on, oldest first,
// or its last purchase when none of its fills holds sealed grams any more.
func fillsOf(ws *workspace.Workspace, slug string) []seal.Dispensed {
	// Over the whole journal: an undo can land in a later fill than its entry.
	stands := map[string]bool{}
	for _, e := range ws.State.Standing {
		stands[e.Hash] = true
	}
	var standing, last []seal.Dispensed
	for i := range ws.State.Cycles {
		c := &ws.State.Cycles[i]
		holds := ws.State.ShareOf(c, slug) > 0
		for _, e := range c.Events {
			if e.Type != journal.Purchase || e.Product != slug || !stands[e.Hash] {
				continue
			}
			d := seal.Dispensed{Day: e.OccurredAt.Format(time.DateOnly), Grams: e.Grams}
			last = []seal.Dispensed{d}
			if holds {
				standing = append(standing, d)
			}
		}
	}
	if len(standing) > 0 {
		return standing
	}
	return last
}

// QR draws text as a QR code in SVG, a square a module, dark on light with
// the quiet zone the standard asks for, at the lowest error correction: a
// code read off a screen is not scratched, and the lowest makes it the
// smallest.
func QR(text string) (string, error) {
	code, err := qr.Encode(text, qr.L)
	if err != nil {
		return "", err
	}
	const quiet = 4
	n := code.Size + 2*quiet
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges">`, n, n)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#fff"/><path fill="#000" d="`, n, n)
	for y := range code.Size {
		for x := range code.Size {
			if code.Black(x, y) {
				fmt.Fprintf(&b, "M%d %dh1v1h-1z", x+quiet, y+quiet)
			}
		}
	}
	b.WriteString(`"/></svg>`)
	return b.String(), nil
}
