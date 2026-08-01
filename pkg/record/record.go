// Package record applies entries to a repository.
//
// It exists so that the commands and the interface record an entry the same
// way. The checks here are the ones that keep the journal describing the tins
// on the table — chiefly that an account cannot be drawn below zero — and they
// should not live in two places where one could drift from the other.
package record

import (
	"fmt"
	"time"

	"github.com/TheDonDope/wits-tui/pkg/catalog"
	"github.com/TheDonDope/wits-tui/pkg/journal"
	"github.com/TheDonDope/wits-tui/pkg/ledger"
	"github.com/TheDonDope/wits-tui/pkg/repo"
)

// Recorder appends entries to a repository's journal.
type Recorder struct {
	repo     *repo.Repo
	products *catalog.Catalog
	devices  *catalog.Devices
	state    *ledger.State
}

// New returns a recorder over the given repository state.
func New(r *repo.Repo, products *catalog.Catalog, devices *catalog.Devices, state *ledger.State) *Recorder {
	return &Recorder{repo: r, products: products, devices: devices, state: state}
}

// Buy records a prescription fill, adding the product to the catalog if it is
// not known yet. The product is returned along with whether it was new.
func (r *Recorder) Buy(name string, grams float64, at time.Time) (journal.Event, *catalog.Product, bool, error) {
	product, err := r.products.Find(name)
	added := false
	if err != nil {
		product = catalog.Parse(name)
		if err := r.products.Add(product); err != nil {
			return journal.Event{}, nil, false, err
		}
		if err := r.products.Save(r.repo.ProductsPath()); err != nil {
			return journal.Event{}, nil, false, err
		}
		added = true
	}
	e, err := r.append(journal.Event{
		Type:       journal.Purchase,
		Product:    product.Slug,
		Grams:      grams,
		OccurredAt: at,
	})
	return e, product, added, err
}

// Grind moves grams from a product's storage into its tin.
func (r *Recorder) Grind(ref string, grams float64, at time.Time) (journal.Event, error) {
	product, err := r.products.Find(ref)
	if err != nil {
		return journal.Event{}, err
	}
	if err := r.check(product.Slug, grams, journal.Storage); err != nil {
		return journal.Event{}, err
	}
	return r.append(journal.Event{
		Type:       journal.Grind,
		Product:    product.Slug,
		Grams:      grams,
		OccurredAt: at,
	})
}

// Session records a session drawing on a product's tin.
func (r *Recorder) Session(ref string, grams float64, at time.Time, device string, temp int, note string) (journal.Event, error) {
	product, err := r.products.Find(ref)
	if err != nil {
		return journal.Event{}, err
	}

	var slug string
	if device != "" {
		d, err := r.devices.Find(device)
		if err != nil {
			return journal.Event{}, err
		}
		slug = d.Slug
		if temp == 0 {
			temp = d.DefaultTemp
		}
		if d.MaxTemp > 0 && temp > d.MaxTemp {
			return journal.Event{}, fmt.Errorf("%s only goes up to %d°C", d.Name, d.MaxTemp)
		}
	}
	if err := r.check(product.Slug, grams, journal.Stash); err != nil {
		return journal.Event{}, err
	}
	return r.append(journal.Event{
		Type:        journal.Sesh,
		Product:     product.Slug,
		Grams:       grams,
		OccurredAt:  at,
		Device:      slug,
		Temperature: temp,
		Note:        note,
	})
}

// Available returns how many grams of a product sit in an account.
func (r *Recorder) Available(slug string, account journal.Account) float64 {
	b := r.state.Balances[slug]
	if b == nil {
		return 0
	}
	switch account {
	case journal.Storage:
		return b.Storage
	case journal.Stash:
		return b.Stash
	case journal.AVB:
		return b.AVB
	default:
		return 0
	}
}

// check refuses to draw an account below zero. The journal would record it
// happily, but a negative balance means the log has stopped describing what is
// actually there.
func (r *Recorder) check(slug string, grams float64, account journal.Account) error {
	have := r.Available(slug, account)
	if have >= grams {
		return nil
	}
	where := "in storage"
	if account == journal.Stash {
		where = "in the tin"
	}
	return fmt.Errorf("only %.2fg of %s %s, cannot take %.2fg", have, slug, where, grams)
}

// append writes the event and folds it into the running state, so a recorder
// used twice in a row sees its own first entry.
func (r *Recorder) append(e journal.Event) (journal.Event, error) {
	stored, err := r.repo.Journal().Append(e)
	if err != nil {
		return journal.Event{}, err
	}
	r.state = ledger.Fold(append(r.state.Events, stored))
	return stored, nil
}

// State returns the folded state, including anything this recorder appended.
func (r *Recorder) State() *ledger.State { return r.state }
