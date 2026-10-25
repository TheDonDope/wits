// Package remote is push and pull: one repository kept on more than one
// device through a folder they share — Syncthing, a USB stick, anything that
// carries files. The folder holds one file, the repository as a bundle,
// encrypted with age to a key that stays in each device's .wits and never in
// the folder.
//
// The journal is hash-chained, so a pull works the way a git rebase does: the
// entries both devices share are kept, the remote's are taken as they are, and
// this device's own are chained again on top of them, with new hashes. A push
// writes only a repository that already holds everything on the remote, so
// two devices cannot keep rewriting each other's entries.
package remote

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"filippo.io/age"
	"github.com/TheDonDope/wits/pkg/bundle"
	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/prices"
	"github.com/TheDonDope/wits/pkg/repo"
	"github.com/TheDonDope/wits/pkg/seal"
)

// File is the one file a remote folder holds.
const File = "wits.age"

var (
	// ErrNoRemote is why push and pull refuse before a remote is set.
	ErrNoRemote = errors.New("no remote is set: wits remote add <folder>")
	// ErrBehind is why a push is refused: the remote holds what this device
	// does not, and writing over it would lose it.
	ErrBehind = errors.New("the remote holds entries this device does not: wits pull first")
	// ErrNoKey is why a remote that already holds a repository is refused
	// without the key it was encrypted to.
	ErrNoKey = errors.New("the remote is encrypted to a key this device does not have")
)

// Key reads the repository's age identity.
func Key(r *repo.Repo) (*age.X25519Identity, error) {
	data, err := os.ReadFile(r.RemoteKeyPath())
	if os.IsNotExist(err) {
		return nil, ErrNoKey
	}
	if err != nil {
		return nil, err
	}
	return age.ParseX25519Identity(strings.TrimSpace(string(data)))
}

// Add sets dir as the repository's remote. A key file brought from another
// device is taken as the repository's key; without one, the key already in
// .wits is kept, or a new one made. A folder that already holds a repository
// must open with the key, or nothing is changed.
func Add(r *repo.Repo, dir, keyFile string) error {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("%s is not a folder", dir)
	}
	id, err := identity(r, keyFile)
	if err != nil {
		return err
	}
	if _, err := Fetch(dir, id); err != nil {
		return err
	}
	if err := journal.WriteFile(r.RemoteKeyPath(), []byte(id.String()+"\n")); err != nil {
		return err
	}
	return r.SetRemote(dir)
}

// identity is the key Add settles on: the one brought, the one kept, or a new
// one.
func identity(r *repo.Repo, keyFile string) (*age.X25519Identity, error) {
	if keyFile != "" {
		data, err := os.ReadFile(keyFile)
		if err != nil {
			return nil, err
		}
		return age.ParseX25519Identity(strings.TrimSpace(string(data)))
	}
	id, err := Key(r)
	if errors.Is(err, ErrNoKey) {
		return age.GenerateX25519Identity()
	}
	return id, err
}

// Fetch reads the repository a remote folder holds, or nil when nothing has
// been pushed to it yet. age authenticates what it decrypts, so a file that
// was changed or cut short fails here rather than restoring as something else.
func Fetch(dir string, id *age.X25519Identity) (*bundle.Contents, error) {
	f, err := os.Open(filepath.Join(dir, File))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	plain, err := age.Decrypt(f, id)
	var mismatch *age.NoIdentityMatchError
	if errors.As(err, &mismatch) {
		return nil, ErrNoKey
	}
	if err != nil {
		return nil, fmt.Errorf("%s cannot be read: %w", File, err)
	}
	z, err := gzip.NewReader(plain)
	if err != nil {
		return nil, err
	}
	defer z.Close()
	c, err := bundle.Read(z)
	if err != nil {
		return nil, err
	}
	// A bundle carries no hashes: they follow from the entries, as a restore
	// recomputes them.
	if c.Events, err = journal.Chain(c.Events); err != nil {
		return nil, err
	}
	if c.Ratings, err = effects.Chain(c.Ratings); err != nil {
		return nil, err
	}
	c.Prices, err = prices.Chain(c.Prices)
	return c, err
}

// store writes contents to a remote folder, encrypted and compressed, in
// place of what it held: written beside it and renamed over it, so a folder
// being synced never carries half a file.
func store(dir string, id *age.X25519Identity, c bundle.Contents) error {
	var buf bytes.Buffer
	enc, err := age.Encrypt(&buf, id.Recipient())
	if err != nil {
		return err
	}
	z := gzip.NewWriter(enc)
	if err := bundle.Write(z, c); err != nil {
		return err
	}
	if err := z.Close(); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return journal.WriteFile(filepath.Join(dir, File), buf.Bytes())
}

// local is the repository as a bundle carries it.
func local(r *repo.Repo) (bundle.Contents, error) {
	var c bundle.Contents
	var err error
	if c.Events, err = r.Journal().Events(); err != nil {
		return c, err
	}
	if c.Ratings, err = r.Effects().All(); err != nil {
		return c, err
	}
	if c.Prices, err = r.Prices().All(); err != nil {
		return c, err
	}
	if c.Seals, err = (seal.Store{Dir: r.SealsDir()}).All(); err != nil {
		return c, err
	}
	if c.Products, err = catalog.Load(r.ProductsPath()); err != nil {
		return c, err
	}
	c.Devices, err = catalog.LoadDevices(r.DevicesPath())
	return c, err
}

// open is the remote folder and its key, or why there is none.
func open(r *repo.Repo) (string, *age.X25519Identity, error) {
	if r.Config.Remote == "" {
		return "", nil, ErrNoRemote
	}
	id, err := Key(r)
	return r.Config.Remote, id, err
}

// Pushed is what a push wrote: the entries, ratings, prices and seals the
// remote lacked.
type Pushed struct{ Events, Ratings, Prices, Seals int }

// Push writes the repository to its remote. It is refused when the remote
// holds an entry, a rating or a product this device does not: those would be
// lost, and a pull brings them in first. A product both hold is written as
// this device has it.
func Push(r *repo.Repo) (Pushed, error) {
	dir, id, err := open(r)
	if err != nil {
		return Pushed{}, err
	}
	ours, err := local(r)
	if err != nil {
		return Pushed{}, err
	}
	theirs, err := Fetch(dir, id)
	if err != nil {
		return Pushed{}, err
	}
	if theirs == nil {
		theirs = &bundle.Contents{}
	}
	if !holds(ours, *theirs) {
		return Pushed{}, ErrBehind
	}
	done := Pushed{Events: len(ours.Events) - len(theirs.Events), Ratings: len(ours.Ratings) - len(theirs.Ratings),
		Prices: len(ours.Prices) - len(theirs.Prices), Seals: len(ours.Seals) - len(theirs.Seals)}
	if done.Events == 0 && done.Ratings == 0 && done.Prices == 0 && done.Seals == 0 &&
		sameCatalogs(ours, *theirs) && sameProofs(ours, *theirs) {
		return done, nil
	}
	return done, store(dir, id, ours)
}

// holds reports whether ours already holds everything in theirs: their
// journal and their effect log as the start of ours, and every product and
// device they name.
func holds(ours, theirs bundle.Contents) bool {
	if len(theirs.Events) > len(ours.Events) || len(theirs.Ratings) > len(ours.Ratings) {
		return false
	}
	if n := len(theirs.Events); n > 0 && ours.Events[n-1].Hash != theirs.Events[n-1].Hash {
		return false
	}
	if n := len(theirs.Ratings); n > 0 && ours.Ratings[n-1].Hash != theirs.Ratings[n-1].Hash {
		return false
	}
	if n := len(theirs.Prices); n > len(ours.Prices) || (n > 0 && ours.Prices[n-1].Hash != theirs.Prices[n-1].Hash) {
		return false
	}
	for _, p := range products(theirs) {
		if findProduct(ours.Products, p.Slug) == nil {
			return false
		}
	}
	if !holdsSeals(ours.Seals, theirs.Seals) {
		return false
	}
	for _, d := range devices(theirs) {
		if findDevice(ours.Devices, d.Slug) == nil {
			return false
		}
	}
	return true
}

// Pulled is what a pull brought in.
type Pulled struct {
	// Fetched is the remote's entries this device lacked; Rechained, this
	// device's own, chained again on top of them with new hashes.
	Fetched, Rechained int
	// Ratings, Prices and Seals are the remote's ratings, prices and seals
	// this device lacked, a seal counted too when its proof had got further.
	Ratings, Prices, Seals int
	// Products and Devices are the slugs taken from the remote: new here, or
	// held here differently, in which case the remote's stands.
	Products, Devices []string
	// Clashes are the entries that took grams that were not there once the
	// two journals were one: the same last grams ground on two devices.
	Clashes []ledger.Shortfall
}

// Pull brings the remote's repository into this one: its catalogs first, so
// every entry names a product this device knows, then the journal and the
// effect log, rebased. Clashes are kept, both entries standing as written,
// and returned; the recorder refuses anything more on a clashed jar until
// one of them is undone.
func Pull(r *repo.Repo) (Pulled, error) {
	dir, id, err := open(r)
	if err != nil {
		return Pulled{}, err
	}
	theirs, err := Fetch(dir, id)
	if err != nil || theirs == nil {
		return Pulled{}, err
	}
	before, err := r.Journal().Events()
	if err != nil {
		return Pulled{}, err
	}
	var done Pulled
	if done.Products, done.Devices, err = mergeCatalogs(r, *theirs); err != nil {
		return done, err
	}
	rebased, err := r.Journal().Rebase(theirs.Events, func(rb journal.Rebased) error {
		var rerr error
		if done.Ratings, _, rerr = r.Effects().Rebase(theirs.Ratings, rb.Moved); rerr != nil {
			return rerr
		}
		done.Prices, _, rerr = r.Prices().Rebase(theirs.Prices, rb.Moved)
		return rerr
	})
	if err != nil {
		return done, err
	}
	done.Fetched, done.Rechained = rebased.Fetched, len(rebased.Moved)
	if done.Seals, err = mergeSeals(r, theirs.Seals); err != nil {
		return done, err
	}
	after, err := r.Journal().Events()
	if err != nil {
		return done, err
	}
	done.Clashes = fresh(ledger.Shortfalls(before), ledger.Shortfalls(after), rebased.Moved)
	return done, nil
}

// fresh is the shortfalls in after that were not in before, an entry that was
// chained again known by its new hash.
func fresh(before, after []ledger.Shortfall, moved map[string]string) []ledger.Shortfall {
	had := map[string]bool{}
	for _, s := range before {
		hash := s.Event.Hash
		if to, ok := moved[hash]; ok {
			hash = to
		}
		had[hash] = true
	}
	var out []ledger.Shortfall
	for _, s := range after {
		if !had[s.Event.Hash] {
			out = append(out, s)
		}
	}
	return out
}

// mergeCatalogs takes the remote's products and devices into this
// repository's catalogs, under their locks, and returns the slugs it took.
func mergeCatalogs(r *repo.Repo, theirs bundle.Contents) (products, devices []string, err error) {
	_, err = r.UpdateProducts(func(c *catalog.Catalog) error {
		products = takeProducts(c, theirs)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	_, err = r.UpdateDevices(func(c *catalog.Devices) error {
		devices = takeDevices(c, theirs)
		return nil
	})
	return products, devices, err
}

// takeProducts adds or replaces each of the remote's products that this
// catalog lacks or holds differently.
func takeProducts(c *catalog.Catalog, theirs bundle.Contents) []string {
	var took []string
	for _, p := range products(theirs) {
		i := slices.IndexFunc(c.Products, func(q *catalog.Product) bool { return q.Slug == p.Slug })
		switch {
		case i < 0:
			c.Products = append(c.Products, p)
		case !sameProduct(c.Products[i], p):
			c.Products[i] = p
		default:
			continue
		}
		took = append(took, p.Slug)
	}
	return took
}

// takeDevices does for devices what takeProducts does for products.
func takeDevices(c *catalog.Devices, theirs bundle.Contents) []string {
	var took []string
	for _, d := range devices(theirs) {
		i := slices.IndexFunc(c.Devices, func(e *catalog.Device) bool { return e.Slug == d.Slug })
		switch {
		case i < 0:
			c.Devices = append(c.Devices, d)
		case *c.Devices[i] != *d:
			c.Devices[i] = d
		default:
			continue
		}
		took = append(took, d.Slug)
	}
	return took
}

// sameProduct compares two products field by field; a time is the same time
// whatever zone it was read back in.
func sameProduct(a, b *catalog.Product) bool {
	x, y := *a, *b
	if !x.AddedAt.Equal(y.AddedAt) || !slices.Equal(x.Terpenes, y.Terpenes) {
		return false
	}
	x.AddedAt, x.Terpenes, y.Terpenes = y.AddedAt, nil, nil
	return fmt.Sprint(x) == fmt.Sprint(y)
}

// sameCatalogs reports whether two copies name the same products and devices
// alike.
func sameCatalogs(a, b bundle.Contents) bool {
	pa, pb, da, db := products(a), products(b), devices(a), devices(b)
	if len(pa) != len(pb) || len(da) != len(db) {
		return false
	}
	for _, p := range pa {
		if q := findProduct(b.Products, p.Slug); q == nil || !sameProduct(p, q) {
			return false
		}
	}
	for _, d := range da {
		if e := findDevice(b.Devices, d.Slug); e == nil || *e != *d {
			return false
		}
	}
	return true
}

// findProduct is the product with exactly this slug; Exact would take a name.
func findProduct(c *catalog.Catalog, slug string) *catalog.Product {
	if c == nil {
		return nil
	}
	for _, p := range c.Products {
		if p.Slug == slug {
			return p
		}
	}
	return nil
}

// findDevice is the device with exactly this slug.
func findDevice(d *catalog.Devices, slug string) *catalog.Device {
	if d == nil {
		return nil
	}
	for _, e := range d.Devices {
		if e.Slug == slug {
			return e
		}
	}
	return nil
}

func products(c bundle.Contents) []*catalog.Product {
	if c.Products == nil {
		return nil
	}
	return c.Products.Products
}

func devices(c bundle.Contents) []*catalog.Device {
	if c.Devices == nil {
		return nil
	}
	return c.Devices.Devices
}

// mergeSeals keeps the remote's seals this device lacks, and the remote's
// proof of a seal both hold when it has got further: a proof is only ever
// completed, never undone.
func mergeSeals(r *repo.Repo, theirs []seal.File) (int, error) {
	st := seal.Store{Dir: r.SealsDir()}
	ours, err := st.All()
	if err != nil {
		return 0, err
	}
	took := 0
	for _, f := range theirs {
		i := slices.IndexFunc(ours, func(o seal.File) bool { return o.Name == f.Name })
		if i >= 0 && further(ours[i].OTS) >= further(f.OTS) {
			continue
		}
		if err := st.Save(f); err != nil {
			return took, err
		}
		took++
	}
	return took, nil
}

// holdsSeals reports whether ours holds every seal of theirs, each proof at
// least as far along: a push must not take back a proof the remote finished.
func holdsSeals(ours, theirs []seal.File) bool {
	for _, f := range theirs {
		i := slices.IndexFunc(ours, func(o seal.File) bool { return o.Name == f.Name })
		if i < 0 || further(ours[i].OTS) < further(f.OTS) {
			return false
		}
	}
	return true
}

// further is how far a proof has got: none, pending, or in blocks.
func further(ots []byte) int {
	p, err := seal.Describe(ots)
	switch {
	case err != nil:
		return 0
	case len(p.Blocks) > 0:
		return 2
	}
	return 1
}

// sameProofs reports whether every seal both hold has got as far on both.
func sameProofs(a, b bundle.Contents) bool {
	for _, f := range a.Seals {
		i := slices.IndexFunc(b.Seals, func(o seal.File) bool { return o.Name == f.Name })
		if i >= 0 && further(f.OTS) != further(b.Seals[i].OTS) {
			return false
		}
	}
	return true
}
