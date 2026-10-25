// Package record applies entries to a repository; this file records a whole
// fill at once — every product a pickup brought, and what each cost.
package record

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/journal"
)

// FillLine is one product of a fill: its name as the label or the invoice
// prints it (or its slug), a slug for it should it be new, the grams
// dispensed, what they cost in cents — zero when the price is not known — and
// a note for the price, the invoice it is from, when it is not the fill's.
type FillLine struct {
	Name  string
	Slug  string
	Grams float64
	Cents int64
	Note  string
}

// Filled is one purchase a fill recorded: the entry, its product, whether the
// product was new, and the price written down for it, with its note.
type Filled struct {
	Event   journal.Event
	Product *catalog.Product
	Added   bool
	Cents   int64
	Note    string
}

// ErrEmptyFill is why a fill that names no product is refused.
var ErrEmptyFill = errors.New("a fill names at least one product")

// ErrUnpriced is why a fill that was recorded says one of its prices was not:
// the purchases stand, and wits reconcile asks for the price later.
var ErrUnpriced = errors.New("the fill is recorded, but a price is not")

// Fill records a fill of one product or several, picked up at the same time:
// a purchase for each, written as one batch — all of them or none — and then
// each one's price, where it has one, to the price log, with its line's note
// or else the fill's.
//
// Every line is checked, and every new product added, before the journal is
// touched, the new products in one write of the catalog: a fill refused for
// any line leaves no other line's new product behind.
func (r *Recorder) Fill(lines []FillLine, at time.Time, note string) ([]Filled, error) {
	if len(lines) == 0 {
		return nil, ErrEmptyFill
	}
	if err := notAhead(journal.Event{Type: journal.Purchase, OccurredAt: at}, time.Now()); err != nil {
		return nil, err
	}
	for i, l := range lines {
		if err := checkLine(l); err != nil {
			return nil, lineError(i, len(lines), err)
		}
	}
	filled, err := r.resolve(lines)
	if err != nil {
		return nil, err
	}
	stored, err := r.write(func() ([]journal.Event, error) {
		batch := make([]journal.Event, len(filled))
		for i, f := range filled {
			batch[i] = journal.Event{Type: journal.Purchase, Product: f.Product.Slug, Grams: lines[i].Grams, OccurredAt: at}
		}
		return batch, nil
	})
	if err != nil {
		return nil, err
	}
	for i := range filled {
		filled[i].Event = stored[i]
	}
	return filled, r.fillPrices(filled, note)
}

// checkLine is what a line must be before anything is written for it.
func checkLine(l FillLine) error {
	switch {
	case strings.TrimSpace(l.Name) == "":
		return errors.New("no product named")
	case l.Grams <= 0:
		return errors.New("the grams dispensed must be more than none")
	case l.Cents < 0:
		return errors.New("a price is not less than nothing")
	}
	return nil
}

// lineError says which line a refusal is about, when there is more than one.
func lineError(i, n int, err error) error {
	if n == 1 {
		return err
	}
	return fmt.Errorf("line %d: %w", i+1, err)
}

// resolve finds each line's product. Those the catalog lacks are added, all of
// them in one change of the catalog as it stands on disk: one line refused,
// none is added.
func (r *Recorder) resolve(lines []FillLine) ([]Filled, error) {
	filled := make([]Filled, len(lines))
	missing := false
	for i, l := range lines {
		filled[i] = Filled{Product: r.products.Exact(strings.TrimSpace(l.Name)), Cents: l.Cents, Note: strings.TrimSpace(l.Note)}
		missing = missing || filled[i].Product == nil
	}
	if !missing {
		return filled, nil
	}
	err := r.changeProducts(func(c *catalog.Catalog) error {
		for i, l := range lines {
			name := strings.TrimSpace(l.Name)
			// Found again in the catalog being changed: it may have gained the
			// product since this recorder's copy was read, or from a line above.
			if p := c.Exact(name); p != nil {
				filled[i].Product, filled[i].Added = p, false
				continue
			}
			p, err := newProduct(c, name, strings.TrimSpace(l.Slug))
			if err != nil {
				return lineError(i, len(lines), err)
			}
			filled[i].Product, filled[i].Added = p, true
		}
		return nil
	})
	return filled, err
}

// fillPrices writes down the price of each purchase that has one. The
// purchases stand whatever happens here: a price missing is one wits
// reconcile asks for later, not a fill undone.
func (r *Recorder) fillPrices(filled []Filled, note string) error {
	for _, f := range filled {
		if f.Cents == 0 {
			continue
		}
		n := f.Note
		if n == "" {
			n = note
		}
		if _, err := r.Price(f.Event.Hash, f.Cents, n, time.Now()); err != nil {
			return fmt.Errorf("%w: %s, %w", ErrUnpriced, f.Product.Name, err)
		}
	}
	return nil
}
