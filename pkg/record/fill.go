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

// Fill records a fill of one product or several, picked up at the same time:
// a purchase for each, written as one batch — all of them or none — and then
// each one's price, where it has one, to the price log, with its line's note
// or else the fill's.
//
// The lines are checked before the catalog is touched, so a fill refused for
// a line's grams leaves no new product behind it.
func (r *Recorder) Fill(lines []FillLine, at time.Time, note string) ([]Filled, error) {
	if len(lines) == 0 {
		return nil, ErrEmptyFill
	}
	if err := notAhead(journal.Event{Type: journal.Purchase, OccurredAt: at}, time.Now()); err != nil {
		return nil, err
	}
	for i, l := range lines {
		if strings.TrimSpace(l.Name) == "" {
			return nil, fmt.Errorf("line %d names no product", i+1)
		}
		if l.Grams <= 0 {
			return nil, fmt.Errorf("line %d, %s: the grams dispensed must be more than none", i+1, l.Name)
		}
		if l.Cents < 0 {
			return nil, fmt.Errorf("line %d, %s: a price is not less than nothing", i+1, l.Name)
		}
	}
	filled, err := r.fillProducts(lines)
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
	return filled, r.fillPrices(filled, stored, note)
}

// fillProducts finds each line's product, adding those not in the catalog.
func (r *Recorder) fillProducts(lines []FillLine) ([]Filled, error) {
	filled := make([]Filled, len(lines))
	for i, l := range lines {
		name := strings.TrimSpace(l.Name)
		product, added := r.products.Exact(name), false
		if product == nil {
			var err error
			if product, added, err = r.addProduct(name, strings.TrimSpace(l.Slug)); err != nil {
				return nil, fmt.Errorf("line %d: %w", i+1, err)
			}
		}
		filled[i] = Filled{Product: product, Added: added, Cents: l.Cents, Note: strings.TrimSpace(l.Note)}
	}
	return filled, nil
}

// fillPrices writes down the price of each purchase that has one. The
// purchases stand whatever happens here: a price missing is one wits
// reconcile asks for later, not a fill undone.
func (r *Recorder) fillPrices(filled []Filled, stored []journal.Event, note string) error {
	for i := range filled {
		filled[i].Event = stored[i]
	}
	for i := range filled {
		if filled[i].Cents == 0 {
			continue
		}
		n := filled[i].Note
		if n == "" {
			n = note
		}
		if _, err := r.Price(stored[i].Hash, filled[i].Cents, n, time.Now()); err != nil {
			return fmt.Errorf("the fill is recorded, but the price of %s is not: %w", filled[i].Product.Name, err)
		}
	}
	return nil
}
