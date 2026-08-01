// Package workspace opens a repository and holds its current state.
//
// Opening a repository is the same six steps every time: find it, read the
// product and device catalogs, read the journal, and fold it. Those steps were
// written out separately in the commands and in the interface, in different
// orders, and anything else that wanted to read a repository — a server, say —
// would have written them a third time. They live here instead.
//
// A workspace is a snapshot. It reads the journal once and folds it once, so
// every screen and every handler working from one sees the same figures.
// Reload takes a fresh one after something has been written.
package workspace

import (
	"os"
	"time"

	"github.com/TheDonDope/wits-tui/pkg/catalog"
	"github.com/TheDonDope/wits-tui/pkg/journal"
	"github.com/TheDonDope/wits-tui/pkg/ledger"
	"github.com/TheDonDope/wits-tui/pkg/record"
	"github.com/TheDonDope/wits-tui/pkg/repo"
)

// Workspace is a repository and everything derived from it.
type Workspace struct {
	Repo     *repo.Repo
	Products *catalog.Catalog
	Devices  *catalog.Devices
	State    *ledger.State
	Recorder *record.Recorder

	// OpenedAt is when the snapshot was taken. Anything reporting "how long has
	// this cycle been running" should measure against it rather than call
	// time.Now itself, so a single view cannot disagree with itself.
	OpenedAt time.Time
}

// Open finds the repository containing dir and reads it.
func Open(dir string) (*Workspace, error) {
	r, err := repo.Discover(dir)
	if err != nil {
		return nil, err
	}
	return Read(r)
}

// Here opens the repository containing the working directory.
func Here() (*Workspace, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return Open(wd)
}

// Read loads an already-discovered repository.
func Read(r *repo.Repo) (*Workspace, error) {
	products, err := catalog.Load(r.ProductsPath())
	if err != nil {
		return nil, err
	}
	devices, err := catalog.LoadDevices(r.DevicesPath())
	if err != nil {
		return nil, err
	}
	events, err := r.Journal().Events()
	if err != nil {
		return nil, err
	}
	state := ledger.Fold(events)
	return &Workspace{
		Repo:     r,
		Products: products,
		Devices:  devices,
		State:    state,
		Recorder: record.New(r, products, devices, state),
		OpenedAt: time.Now(),
	}, nil
}

// Reload returns a fresh snapshot of the same repository.
func (w *Workspace) Reload() (*Workspace, error) { return Read(w.Repo) }

// Journal returns the repository's journal.
func (w *Workspace) Journal() *journal.Journal { return w.Repo.Journal() }

// Events returns the entries the snapshot was folded from.
func (w *Workspace) Events() []journal.Event { return w.State.Events }

// Cycle returns the prescription cycle in progress, or nil.
func (w *Workspace) Cycle() *ledger.Cycle { return w.State.CurrentCycle() }

// ProductName resolves a slug to its display name, falling back to the slug so
// that an entry for a product missing from the catalog still reads sensibly.
func (w *Workspace) ProductName(slug string) string {
	if w.Products != nil {
		if p, err := w.Products.Find(slug); err == nil {
			return p.Name
		}
	}
	return slug
}
