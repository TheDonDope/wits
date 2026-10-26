// Package bundle reads a bundle back into a repository here: the catalogs,
// the journal as one batch, then the effect and price logs and the seals.
package bundle

import (
	"fmt"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/prices"
	"github.com/TheDonDope/wits/pkg/repo"
	"github.com/TheDonDope/wits/pkg/seal"
)

// Restore writes a bundle's contents into an empty repository: the catalogs,
// then every event as one batch — one write, one flush, refused whole if any
// event does not validate — and checks the chain it made; then the effect
// log, the price log and the seals the same way. An entry at a time flushed
// once per entry, which on a real disk is seconds of silence — 19 s for 1500
// ratings — and a failure halfway left a journal too full to restore into
// again, so the ratings and prices are checked before anything is written.
func Restore(r *repo.Repo, contents *Contents, step func(string)) error {
	if _, err := effects.Chain(contents.Ratings); err != nil {
		return fmt.Errorf("the bundle's effect log: %w", err)
	}
	if _, err := prices.Chain(contents.Prices); err != nil {
		return fmt.Errorf("the bundle's price log: %w", err)
	}
	if err := restoreCatalogs(r, contents, step); err != nil {
		return err
	}
	step("writing " + count(len(contents.Events), "event") + " to the journal")
	// Chained as they stood, merges and all, and written as they are, so the
	// journal hashes as the one bundled did.
	chained, err := journal.Chain(contents.Events, contents.Shapes)
	if err != nil {
		return err
	}
	if err := r.Journal().Import(chained); err != nil {
		return err
	}
	step("verifying the chain")
	if err := r.Journal().Verify(); err != nil {
		return fmt.Errorf("the restored journal does not verify: %w", err)
	}
	if err := restoreLogs(r, contents, step); err != nil {
		return err
	}
	return restoreSeals(r, contents, step)
}

// restoreCatalogs writes a bundle's products and devices back.
func restoreCatalogs(r *repo.Repo, contents *Contents, step func(string)) error {
	if p := contents.products(); len(p) > 0 {
		step("restoring " + count(len(p), "product"))
		if _, err := r.UpdateProducts(func(c *catalog.Catalog) error { c.Products = p; return nil }); err != nil {
			return err
		}
	}
	if d := contents.devices(); len(d) > 0 {
		step("restoring " + count(len(d), "device"))
		if _, err := r.UpdateDevices(func(c *catalog.Devices) error { c.Devices = d; return nil }); err != nil {
			return err
		}
	}
	return nil
}

// restoreLogs writes a bundle's effect log and price log back, each as one
// batch, and checks the chains they made.
func restoreLogs(r *repo.Repo, contents *Contents, step func(string)) error {
	if len(contents.Ratings) > 0 {
		step("restoring " + count(len(contents.Ratings), "rating"))
		if _, err := r.Effects().AppendAll(contents.Ratings); err != nil {
			return fmt.Errorf("restoring the effect log: %w", err)
		}
		if err := r.Effects().Verify(); err != nil {
			return fmt.Errorf("the restored effect log does not verify: %w", err)
		}
	}
	if len(contents.Prices) > 0 {
		step("restoring " + count(len(contents.Prices), "price"))
		if _, err := r.Prices().AppendAll(contents.Prices); err != nil {
			return fmt.Errorf("restoring the price log: %w", err)
		}
		if err := r.Prices().Verify(); err != nil {
			return fmt.Errorf("the restored price log does not verify: %w", err)
		}
	}
	return nil
}

// restoreSeals writes a bundle's seals and their proofs back.
func restoreSeals(r *repo.Repo, contents *Contents, step func(string)) error {
	if len(contents.Seals) == 0 {
		return nil
	}
	step("restoring " + count(len(contents.Seals), "seal"))
	for _, f := range contents.Seals {
		if err := (seal.Store{Dir: r.SealsDir()}).Save(f); err != nil {
			return err
		}
	}
	return nil
}

// products is the bundle's products, or none when it carried no catalog.
func (c *Contents) products() []*catalog.Product {
	if c.Products == nil {
		return nil
	}
	return c.Products.Products
}

// devices is the bundle's devices, or none when it carried no catalog.
func (c *Contents) devices() []*catalog.Device {
	if c.Devices == nil {
		return nil
	}
	return c.Devices.Devices
}

// count is a number of things, the noun plural unless it is one.
func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
