// Package record applies entries to a repository.
//
// It exists so that the commands and the interface record an entry the same
// way. The checks here are the ones that keep the journal describing the stashes
// on the table — chiefly that nothing takes more from a jar than is in it — and they
// should not live in two places where one could drift from the other.
package record

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/repo"
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
//
// A slug may be given for a new product; left empty, a short one is made up
// that is not already taken. It is the name every later entry refers to, so it
// is settled once, when the product first appears, and never afterwards.
func (r *Recorder) Buy(name, slug string, grams float64, at time.Time) (journal.Event, *catalog.Product, bool, error) {
	// Before the catalog is touched: a refused fill must not leave a product
	// behind.
	if err := notAhead(journal.Event{Type: journal.Purchase, OccurredAt: at}, time.Now()); err != nil {
		return journal.Event{}, nil, false, err
	}
	product := r.products.Exact(name)
	added := false
	if product == nil {
		var err error
		if product, added, err = r.addProduct(name, slug); err != nil {
			return journal.Event{}, nil, false, err
		}
	}
	e, err := r.one(func() (journal.Event, error) {
		return journal.Event{
			Type:       journal.Purchase,
			Product:    product.Slug,
			Grams:      grams,
			OccurredAt: at,
		}, nil
	})
	return e, product, added, err
}

// ErrPartialName is returned when a purchase names only part of a product.
var ErrPartialName = errors.New("not a product's full name or slug")

// addProduct adds the product a purchase names, under the catalog's lock and
// against the catalog as it stands, which may have gained it since this
// recorder's copy was read — then it is used, not added twice.
func (r *Recorder) addProduct(name, slug string) (*catalog.Product, bool, error) {
	var product *catalog.Product
	added := false
	err := r.changeProducts(func(c *catalog.Catalog) error {
		if product = c.Exact(name); product != nil {
			return nil
		}
		if err := partial(c, name, slug); err != nil {
			return err
		}
		product = catalog.Parse(name)
		if slug != "" {
			if err := catalog.CheckSlug(slug); err != nil {
				return err
			}
			if c.Taken(slug) {
				return fmt.Errorf("the slug %q is already in use", slug)
			}
			product.Slug = slug
		} else {
			product.Slug = catalog.NewHandle(product, c.Handles())
		}
		added = true
		return c.Add(product)
	})
	return product, added, err
}

// changeProducts applies change to the product catalog as it stands on disk,
// under its lock, and adopts what was written, so everything sharing this
// recorder's catalog — the screens included — sees it.
func (r *Recorder) changeProducts(change func(*catalog.Catalog) error) error {
	fresh, err := r.repo.UpdateProducts(change)
	if err != nil {
		return err
	}
	*r.products = *fresh
	return nil
}

// changeProduct applies change to one product, found in the catalog as it
// stands on disk.
func (r *Recorder) changeProduct(ref string, change func(*catalog.Product)) (*catalog.Product, error) {
	var product *catalog.Product
	err := r.changeProducts(func(c *catalog.Catalog) error {
		p, err := c.Find(ref)
		if err != nil {
			return err
		}
		change(p)
		product = p
		return nil
	})
	return product, err
}

// partial refuses a purchase whose name is only part of a known product's.
// Every other entry may abbreviate, but a purchase that matches nothing in
// full adds a product, so a guess either way is wrong: "MAC1" used to land
// on the MAC1+ jar unannounced, and "Can", matching two, made a third. A
// slug given with the name says a new product is meant.
func partial(c *catalog.Catalog, name, slug string) error {
	matches := c.Partial(name)
	if slug != "" || len(matches) == 0 {
		return nil
	}
	if len(matches) == 1 {
		p := matches[0]
		return fmt.Errorf("%q is %w, but it is part of %s (%s): buy it as %s, or give a new product a name of its own",
			name, ErrPartialName, p.Slug, p.Name, p.Slug)
	}
	slugs := make([]string, 0, len(matches))
	for _, p := range matches {
		slugs = append(slugs, p.Slug)
	}
	return fmt.Errorf("%q is %w, but it is part of %s: buy one of them by its slug, or give a new product a name of its own",
		name, ErrPartialName, strings.Join(slugs, ", "))
}

// Grind moves grams from a product's storage into its stash.
func (r *Recorder) Grind(ref string, grams float64, at time.Time) (journal.Event, error) {
	if err := notAhead(journal.Event{Type: journal.Grind, OccurredAt: at}, time.Now()); err != nil {
		return journal.Event{}, err
	}
	product, err := r.products.Find(ref)
	if err != nil {
		return journal.Event{}, err
	}
	return r.one(func() (journal.Event, error) {
		if err := r.check(product.Slug, grams, journal.Storage); err != nil {
			return journal.Event{}, err
		}
		return journal.Event{
			Type:       journal.Grind,
			Product:    product.Slug,
			Grams:      grams,
			OccurredAt: at,
		}, nil
	})
}

// Session records a session drawing on a product's stash.
func (r *Recorder) Session(ref string, grams float64, at time.Time, device string, temp int, note string) (journal.Event, error) {
	if err := notAhead(journal.Event{Type: journal.Sesh, OccurredAt: at}, time.Now()); err != nil {
		return journal.Event{}, err
	}
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
	return r.one(func() (journal.Event, error) {
		if err := r.check(product.Slug, grams, journal.Stash); err != nil {
			return journal.Event{}, err
		}
		return journal.Event{
			Type:        journal.Sesh,
			Product:     product.Slug,
			Grams:       grams,
			OccurredAt:  at,
			Device:      slug,
			Temperature: temp,
			Note:        note,
		}, nil
	})
}

// Available returns how many grams of a product sit in an account.
func (r *Recorder) Available(slug string, account journal.Account) float64 {
	if b := r.state.Balances[slug]; b != nil {
		return b.In(account)
	}
	return 0
}

// check refuses to take more from an account than it holds. The journal
// would record it happily, but grams that are not in the jar cannot come out
// of it, and a log that says they did has stopped describing the jar.
//
// External is outside the system, and is not a jar: undoing a use of AVB, or
// amending a purchase upward, draws on it freely.
func (r *Recorder) check(slug string, grams float64, account journal.Account) error {
	if account == journal.External {
		return nil
	}
	have := r.Available(slug, account)
	if have >= grams {
		return nil
	}
	// The same names the reconcile forms use, so an account is called one
	// thing everywhere. Undoing an avb-collect used to complain about
	// "storage", which is not where the grams were.
	where := "in " + string(account)
	if name, ok := reconcilable[account]; ok {
		where = "in " + name
	}
	return fmt.Errorf("only %.2fg of %s %s, cannot take %.2fg", have, slug, where, grams)
}

// write appends the entries build makes, and folds them into the running
// state, so a recorder used twice in a row sees its own first entry.
//
// build runs under the journal's locks, against a state folded from the
// journal as it stands at that moment — not the snapshot the recorder was
// made from, which a command in another terminal may have overtaken. Every
// check a builder makes is therefore a check of the real history; made
// against the snapshot, two grinds of the same jar both passed and left
// storage at -17.10 g.
//
// The ledger folds entries by date, so a backdated one can take grams that
// were not there yet while today's balance looks fine — a grind dated before
// the fill it draws on. check only sees today; impossible sees every day.
func (r *Recorder) write(build func() ([]journal.Event, error)) ([]journal.Event, error) {
	stored, err := r.repo.Journal().AppendWith(func(events []journal.Event) ([]journal.Event, error) {
		r.state = ledger.Fold(events)
		batch, err := build()
		if err != nil {
			return nil, err
		}
		// Taken after the builder, which dates corrections with its own now.
		now := time.Now()
		for i := range batch {
			if batch[i].OccurredAt.IsZero() {
				batch[i].OccurredAt = now
			}
			if err := notAhead(batch[i], now); err != nil {
				return nil, err
			}
			// The journal fills the accounts in on append; the replay needs
			// them now, or the entry moves nothing and takes nothing.
			if batch[i].From == "" && batch[i].To == "" {
				batch[i].From, batch[i].To, _ = journal.Flow(batch[i].Type)
			}
		}
		return batch, r.impossible(batch)
	})
	if err != nil {
		return nil, err
	}
	r.state = ledger.Fold(append(slices.Clip(r.state.Events), stored...))
	return stored, nil
}

// ErrAhead is why an entry dated after the moment it is written is refused:
// the journal records what happened, and a fill or a grind that has not
// happened yet is a plan, not a record.
var ErrAhead = errors.New("an entry is recorded once it has happened")

// ErrClash is why a jar holding an impossible entry takes nothing more but an
// undo.
var ErrClash = errors.New("this jar's record does not add up")

// notAhead refuses an entry dated after now.
func notAhead(e journal.Event, now time.Time) error {
	if e.OccurredAt.After(now) {
		return fmt.Errorf("%s is dated %s, after now: %w", e.Type, e.OccurredAt.Format("2006-01-02 15:04"), ErrAhead)
	}
	return nil
}

// one is write for a single entry.
func (r *Recorder) one(build func() (journal.Event, error)) (journal.Event, error) {
	stored, err := r.write(func() ([]journal.Event, error) {
		e, err := build()
		return []journal.Event{e}, err
	})
	if err != nil {
		return journal.Event{}, err
	}
	return stored[0], nil
}

// impossible refuses entries that take grams that were not there to take at
// their moment, or that leave a later entry without the grams it took.
//
// A jar that already holds an impossible entry — two devices' journals merged
// into one, each having ground its last grams — takes nothing more but an
// undo until it is resolved: anything recorded on top of it would be built on
// grams that may not be there.
func (r *Recorder) impossible(batch []journal.Event) error {
	before := map[string]bool{}
	clashed := map[string]string{}
	for _, short := range ledger.Shortfalls(r.state.Events) {
		before[short.Event.Hash] = true
		clashed[short.Event.Product] = short.Event.Hash
	}
	for _, e := range batch {
		if hash, ok := clashed[e.Product]; ok && e.Reverts == "" {
			return fmt.Errorf("%w: %s took grams that were not in %s; wits revert it, or the entry it clashes with, first",
				ErrClash, short(hash), e.Product)
		}
	}
	for _, short := range ledger.Shortfalls(append(slices.Clip(r.state.Events), batch...)) {
		e := short.Event
		if e.Hash != "" && before[e.Hash] {
			continue
		}
		where := string(short.Account)
		if name, ok := reconcilable[short.Account]; ok {
			where = name
		}
		if e.Hash == "" {
			return fmt.Errorf("%s %.2fg of %s on %s: there were only %.2fg in %s then",
				e.Type, e.Grams, e.Product, e.OccurredAt.Format(time.DateOnly), short.Held, where)
		}
		// An entry already recorded loses the grams it took to the new one.
		added := batch[len(batch)-1]
		return fmt.Errorf("%s %.2fg of %s on %s would leave only %.2fg in %s for the %s of %.2fg on %s",
			added.Type, added.Grams, added.Product, added.OccurredAt.Format(time.DateOnly),
			short.Held, where, e.Type, e.Grams, e.OccurredAt.Format(time.DateOnly))
	}
	return nil
}

// State returns the folded state, including anything this recorder appended.
func (r *Recorder) State() *ledger.State { return r.state }

// Revert records the correction of an earlier entry.
//
// Nothing is removed. The journal is append-only and hash chained, which is what
// lets a bundle be verified against the repository it came from, so an entry is
// undone by moving the same grams back the way they came. The correction names
// the entry it reverses, so the pair can be recognised and hidden from a view
// that wants to show only what currently stands.
func (r *Recorder) Revert(hash string, reason string) (journal.Event, error) {
	return r.one(func() (journal.Event, error) { return r.undo(hash, reason) })
}

// undo builds the correction of an entry without recording it.
func (r *Recorder) undo(hash string, reason string) (journal.Event, error) {
	original, err := r.Find(hash)
	if err != nil {
		return journal.Event{}, err
	}
	if original.Type == journal.Adjust {
		return journal.Event{}, fmt.Errorf("%s is already a correction", short(hash))
	}
	if r.RevertOf(original.Hash) != nil {
		return journal.Event{}, fmt.Errorf("%s has already been corrected", short(hash))
	}
	if reason == "" {
		reason = "reverts " + short(hash)
	}
	at := time.Now()
	// Putting the grams back must not take more than the account they went
	// into holds: if they have since been ground on or used, the later entries
	// have to go first.
	if err := r.check(original.Product, original.Grams, original.To); err != nil {
		return journal.Event{}, fmt.Errorf("cannot undo %s: %w", short(hash), err)
	}
	return journal.Event{
		Type:       journal.Adjust,
		Product:    original.Product,
		Grams:      original.Grams,
		From:       original.To,
		To:         original.From,
		OccurredAt: at,
		Reverts:    original.Hash,
		Note:       reason,
	}, nil
}

// Amend corrects the amount of an earlier entry, by reverting it and recording
// it again as it should have been.
//
// The correction and the re-recorded entry are written together, under one
// lock and in one write, so another writer cannot slip between them and leave
// the entry undone but not re-recorded.
func (r *Recorder) Amend(hash string, grams float64, note string) (journal.Event, error) {
	if grams <= 0 {
		return journal.Event{}, fmt.Errorf("grams must be positive, got %v", grams)
	}
	stored, err := r.write(func() ([]journal.Event, error) {
		original, err := r.Find(hash)
		if err != nil {
			return nil, err
		}
		// The revert frees the original amount back into the source account,
		// so only the difference beyond it has to be there already.
		if extra := round(grams - original.Grams); extra > 0 {
			if err := r.check(original.Product, extra, original.From); err != nil {
				return nil, fmt.Errorf("cannot amend %s to %.2fg: %w", short(hash), grams, err)
			}
		}
		undo, err := r.undo(hash, "amended")
		if err != nil {
			return nil, err
		}
		corrected := original
		corrected.Grams = grams
		corrected.Hash, corrected.Prev, corrected.Seq = "", "", 0
		corrected.RecordedAt = time.Time{}
		if note != "" {
			corrected.Note = note
		}
		return []journal.Event{undo, corrected}, nil
	})
	if err != nil {
		return journal.Event{}, err
	}
	return stored[1], nil
}

// Find returns the event with the given hash, which may be abbreviated.
func (r *Recorder) Find(hash string) (journal.Event, error) {
	if hash == "" {
		return journal.Event{}, fmt.Errorf("no entry given")
	}
	var found *journal.Event
	for i := range r.state.Events {
		if strings.HasPrefix(r.state.Events[i].Hash, hash) {
			if found != nil {
				return journal.Event{}, fmt.Errorf("%s matches more than one entry", hash)
			}
			found = &r.state.Events[i]
		}
	}
	if found == nil {
		return journal.Event{}, fmt.Errorf("no entry matches %s", hash)
	}
	return *found, nil
}

// RevertOf returns the correction of an entry, if it has been corrected.
func (r *Recorder) RevertOf(hash string) *journal.Event {
	for i := range r.state.Events {
		if r.state.Events[i].Reverts == hash {
			return &r.state.Events[i]
		}
	}
	return nil
}

// Reverted returns the hashes of every entry that has been corrected, along with
// the corrections themselves, so a view can leave the pair out.
func Reverted(events []journal.Event) map[string]bool {
	out := map[string]bool{}
	for _, e := range events {
		if e.Reverts != "" {
			out[e.Reverts] = true
			out[e.Hash] = true
		}
	}
	return out
}

// short abbreviates a hash the way a commit is abbreviated.
func short(h string) string {
	if len(h) > 7 {
		return h[:7]
	}
	return h
}

// ErrNothingToReconcile is returned when the scale agrees with the ledger.
var ErrNothingToReconcile = errors.New("the ledger already matches the scale")

// reconcilable are the accounts that hold something you can put on a scale.
// Consumed is not one of them: it is what has gone through a device and is on
// its way to becoming AVB, and there is no jar of it to weigh.
var reconcilable = map[journal.Account]string{
	journal.Storage: "storage",
	journal.Stash:   "the stash",
	journal.AVB:     "the AVB jar",
}

// Reconcile records the difference between what the ledger believes an account
// holds and what the scale actually reads.
//
// Ledgers drift: a little is spilled, a session goes unlogged, a scale is read
// wrong. The honest repair is not to edit the past — nobody knows which entry
// was wrong — but to record that the account is now known to hold a different
// amount, and by how much. The difference becomes an adjustment, which the fold
// applies like any other transfer.
func (r *Recorder) Reconcile(ref string, account journal.Account, weighed float64, note string) (journal.Event, error) {
	_, ok := reconcilable[account]
	if !ok {
		return journal.Event{}, fmt.Errorf("%s cannot be weighed", account)
	}
	product, err := r.products.Find(ref)
	if err != nil {
		return journal.Event{}, err
	}
	if weighed < 0 {
		return journal.Event{}, fmt.Errorf("a weight cannot be negative, got %.2f", weighed)
	}

	return r.one(func() (journal.Event, error) {
		return r.adjustment(product.Slug, account, weighed, note)
	})
}

// adjustment builds the entry that brings an account to what was weighed.
// It runs under the journal's lock, so the difference is taken from the
// balance as it stands — a grind recorded elsewhere since the snapshot would
// otherwise be booked again as a loss.
func (r *Recorder) adjustment(slug string, account journal.Account, weighed float64, note string) (journal.Event, error) {
	where := reconcilable[account]
	expected := r.Available(slug, account)
	difference := round(weighed - expected)
	if difference == 0 {
		return journal.Event{}, fmt.Errorf("%w: %.2f g in %s", ErrNothingToReconcile, expected, where)
	}

	// Grams that are there but unaccounted for come in from outside the system;
	// grams the ledger has but the jar does not have gone out of it. Either way
	// the entry is a transfer, so the accounts still balance afterwards.
	from, to := journal.External, account
	if difference < 0 {
		from, to = account, journal.External
	}
	if note == "" {
		note = fmt.Sprintf("reconciled %s: %.2f g weighed, %.2f g expected", where, weighed, expected)
	}
	return journal.Event{
		Type:       journal.Adjust,
		Product:    slug,
		Grams:      math.Abs(difference),
		From:       from,
		To:         to,
		OccurredAt: time.Now(),
		Note:       note,
	}, nil
}

// FlushNote is the note a flushed stash carries — the same words the
// terminal's history clean-up writes, so a bundle made by an older wits reads
// either back to the same accounts.
const FlushNote = "clean history: consumed at some point"

// Flush records stashes as empty: their grams went through a device at some
// point, the sessions never logged. Every stash holding something is flushed
// when no product is named. The adjustments are one batch under one lock —
// all of them or none — and each stash is read as it stands then, so a grind
// recorded elsewhere meanwhile is flushed with the rest rather than left over.
func (r *Recorder) Flush(refs []string) ([]journal.Event, error) {
	slugs := make([]string, 0, len(refs))
	for _, ref := range refs {
		p, err := r.products.Find(ref)
		if err != nil {
			return nil, err
		}
		slugs = append(slugs, p.Slug)
	}
	return r.write(func() ([]journal.Event, error) {
		names := slugs
		if len(names) == 0 {
			names = r.state.Products()
		}
		var batch []journal.Event
		for _, slug := range names {
			if slug == "" || r.Available(slug, journal.Stash) <= 0 {
				continue
			}
			e, err := r.adjustment(slug, journal.Stash, 0, FlushNote)
			if err != nil {
				return nil, err
			}
			batch = append(batch, e)
		}
		if len(batch) == 0 {
			return nil, ErrNothingToReconcile
		}
		return batch, nil
	})
}

// Rate records what a session did: scores from 1 to 5 on any of the
// dimensions, and a note. The session is named by its hash, abbreviated or
// not; it must be a session, and one that stands — rating an undone session
// would rate grams that never went through a device. Rating a session again
// supersedes the earlier rating; the log keeps both.
func (r *Recorder) Rate(session string, scores map[effects.Dimension]int, note string, at time.Time) (effects.Rating, error) {
	e, err := r.Find(session)
	if err != nil {
		return effects.Rating{}, err
	}
	if e.Type != journal.Sesh {
		return effects.Rating{}, fmt.Errorf("%s is a %s, not a session; only a session is rated", short(e.Hash), e.Type)
	}
	if r.RevertOf(e.Hash) != nil {
		return effects.Rating{}, fmt.Errorf("the session %s was undone", short(e.Hash))
	}
	return r.repo.Effects().Append(effects.Rating{
		Session: e.Hash,
		At:      at,
		Scores:  scores,
		Note:    strings.TrimSpace(note),
	})
}

// Difference reports what reconciling an account to a weighed amount would
// change, without recording anything. A screen can show it before it is
// committed to.
func (r *Recorder) Difference(ref string, account journal.Account, weighed float64) (expected, difference float64, err error) {
	product, err := r.products.Find(ref)
	if err != nil {
		return 0, 0, err
	}
	expected = r.Available(product.Slug, account)
	return expected, round(weighed - expected), nil
}

// round trims to centigrams, the way the whole ledger does.
func round(g float64) float64 { return ledger.Round(g) }

// Rename changes what a product is called, and nothing else.
//
// The slug is left alone deliberately: it is the name every entry in the
// journal refers to, and changing it would leave those entries pointing at a
// product that no longer exists. What a jar is called can be corrected; what it
// is remains what it was.
func (r *Recorder) Rename(ref, name string) (*catalog.Product, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("a product needs a name")
	}
	return r.changeProduct(ref, func(p *catalog.Product) { p.Name = name })
}

// Describe replaces a product's parsed details, leaving its slug and its
// entries alone.
func (r *Recorder) Describe(ref string, p catalog.Product) (*catalog.Product, error) {
	if strings.TrimSpace(p.Name) == "" {
		return nil, fmt.Errorf("a product needs a name")
	}
	return r.changeProduct(ref, func(product *catalog.Product) {
		product.Name = strings.TrimSpace(p.Name)
		product.Manufacturer = strings.TrimSpace(p.Manufacturer)
		product.Cultivar = strings.TrimSpace(p.Cultivar)
		product.THC, product.CBD = p.THC, p.CBD
	})
}

// ErrNothingToCollect is why a collection is refused when no session has been
// through the device since it was last emptied.
var ErrNothingToCollect = errors.New("no session has been through it since it was last emptied")

// CollectAVB records the AVB weighed out of a device: grams, from the
// sessions put through it since its last collection. A device left unnamed is
// the one those sessions used, when there is only one. The grams are shared
// between the sessions' products in proportion to what each put in, an entry
// a product, all at one moment, written together; more cannot come out than
// went in.
func (r *Recorder) CollectAVB(device string, grams float64, at time.Time, note string) ([]journal.Event, error) {
	if err := notAhead(journal.Event{Type: journal.AVBCollect, OccurredAt: at}, time.Now()); err != nil {
		return nil, err
	}
	if device != "" {
		d, err := r.devices.Find(device)
		if err != nil {
			return nil, err
		}
		device = d.Slug
	}
	return r.write(func() ([]journal.Event, error) {
		slug, sessions, err := r.pending(device, at)
		if err != nil {
			return nil, err
		}
		put := map[string]journal.Centigrams{}
		var total journal.Centigrams
		for _, s := range sessions {
			put[s.Product] += journal.CG(s.Grams)
			total += journal.CG(s.Grams)
		}
		if journal.CG(grams) > total {
			return nil, fmt.Errorf("%.2fg of AVB cannot come out of the %.2fg its %s put in",
				grams, total.Grams(), plural(len(sessions), "session"))
		}
		if note == "" {
			note = fmt.Sprintf("from %s, %.2fg", plural(len(sessions), "session"), total.Grams())
		}
		return r.shared(journal.AVBCollect, ledger.Split(journal.CG(grams), put), at, slug, note, journal.Consumed)
	})
}

// pending is the device a collection is from, and its sessions since it was
// last emptied: the one named, or the only one with sessions waiting.
func (r *Recorder) pending(device string, at time.Time) (string, []journal.Event, error) {
	waiting := ledger.Uncollected(r.state.Events, at)
	if device != "" || len(waiting) == 0 {
		if len(waiting[device]) == 0 {
			return "", nil, ErrNothingToCollect
		}
		return device, waiting[device], nil
	}
	if len(waiting) > 1 {
		names := make([]string, 0, len(waiting))
		for d := range waiting {
			names = append(names, cmp.Or(d, "no device named"))
		}
		sort.Strings(names)
		return "", nil, fmt.Errorf("sessions are waiting on more than one device (%s): say which was emptied",
			strings.Join(names, ", "))
	}
	for d, sessions := range waiting {
		return d, sessions, nil
	}
	return "", nil, ErrNothingToCollect
}

// UseAVB records AVB used up — for butter, a tincture — drawn from every
// product's AVB in proportion to what each holds: the jar is one jar.
func (r *Recorder) UseAVB(grams float64, at time.Time, note string) ([]journal.Event, error) {
	if err := notAhead(journal.Event{Type: journal.AVBUse, OccurredAt: at}, time.Now()); err != nil {
		return nil, err
	}
	return r.write(func() ([]journal.Event, error) {
		held := map[string]journal.Centigrams{}
		var total journal.Centigrams
		for slug, b := range r.state.Balances {
			held[slug] = journal.CG(b.AVB)
			total += held[slug]
		}
		if journal.CG(grams) > total {
			return nil, fmt.Errorf("only %.2fg in the AVB jar, cannot take %.2fg", total.Grams(), grams)
		}
		return r.shared(journal.AVBUse, ledger.Split(journal.CG(grams), held), at, "", note, journal.AVB)
	})
}

// shared is an entry a product for grams shared out between them, each
// checked against the account it draws on.
func (r *Recorder) shared(typ journal.Type, shares map[string]journal.Centigrams, at time.Time, device, note string, from journal.Account) ([]journal.Event, error) {
	slugs := make([]string, 0, len(shares))
	for slug := range shares {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	var batch []journal.Event
	for _, slug := range slugs {
		g := shares[slug].Grams()
		if err := r.check(slug, g, from); err != nil {
			return nil, err
		}
		batch = append(batch, journal.Event{Type: typ, Product: slug, Grams: g, OccurredAt: at, Device: device, Note: note})
	}
	return batch, nil
}

// plural counts a noun, adding an s past one.
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
