// Package record applies entries to a repository.
//
// It exists so that the commands and the interface record an entry the same
// way. The checks here are the ones that keep the journal describing the stashes
// on the table — chiefly that an account cannot be drawn below zero — and they
// should not live in two places where one could drift from the other.
package record

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/catalog"
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
	product := r.products.Exact(name)
	added := false
	if product == nil {
		if err := r.partial(name, slug); err != nil {
			return journal.Event{}, nil, false, err
		}
		product = catalog.Parse(name)
		if slug != "" {
			if err := catalog.CheckSlug(slug); err != nil {
				return journal.Event{}, nil, false, err
			}
			if r.products.Taken(slug) {
				return journal.Event{}, nil, false, fmt.Errorf("the slug %q is already in use", slug)
			}
			product.Slug = slug
		} else {
			product.Slug = catalog.NewHandle(product, r.products.Handles())
		}
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

// ErrPartialName is returned when a purchase names only part of a product.
var ErrPartialName = errors.New("not a product's full name or slug")

// partial refuses a purchase whose name is only part of a known product's.
// Every other entry may abbreviate, but a purchase that matches nothing in
// full adds a product, so a guess either way is wrong: "MAC1" used to land
// on the MAC1+ jar unannounced, and "Can", matching two, made a third. A
// slug given with the name says a new product is meant.
func (r *Recorder) partial(name, slug string) error {
	matches := r.products.Partial(name)
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

// Session records a session drawing on a product's stash.
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
	if b := r.state.Balances[slug]; b != nil {
		return b.In(account)
	}
	return 0
}

// check refuses to draw an account below zero. The journal would record it
// happily, but a negative balance means the log has stopped describing what is
// actually there.
//
// External is outside the system and cannot be overdrawn: undoing a use of
// AVB, or amending a purchase upward, draws on it freely.
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

// append writes the event and folds it into the running state, so a recorder
// used twice in a row sees its own first entry.
//
// The ledger folds entries by date, so a backdated one can overdraw an account
// in the past while today's balance looks fine — a grind dated before the fill
// it draws on. check only sees today; this sees every day.
func (r *Recorder) append(e journal.Event) (journal.Event, error) {
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now()
	}
	// The journal fills the accounts in on append; the trial fold needs them
	// now, or the entry moves nothing and cannot overdraw anything.
	if e.From == "" && e.To == "" {
		e.From, e.To, _ = journal.Flow(e.Type)
	}
	if err := r.overdraws(e); err != nil {
		return journal.Event{}, err
	}
	stored, err := r.repo.Journal().Append(e)
	if err != nil {
		return journal.Event{}, err
	}
	r.state = ledger.Fold(append(slices.Clip(r.state.Events), stored))
	return stored, nil
}

// overdraws refuses an entry that would leave an account below zero at any
// point in the replay, where none was before.
func (r *Recorder) overdraws(e journal.Event) error {
	before := map[string]bool{}
	for _, o := range r.state.Overdrawn {
		before[o.Event.Hash] = true
	}
	next := ledger.Fold(append(slices.Clip(r.state.Events), e))
	for _, o := range next.Overdrawn {
		if o.Event.Hash != "" && before[o.Event.Hash] {
			continue
		}
		where := string(o.Account)
		if name, ok := reconcilable[o.Account]; ok {
			where = name
		}
		return fmt.Errorf("%s %.2fg of %s on %s would leave %.2fg in %s on %s: the grams were not there yet",
			e.Type, e.Grams, e.Product, e.OccurredAt.Format(time.DateOnly),
			o.Grams, where, o.Event.OccurredAt.Format(time.DateOnly))
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
	// Putting the grams back must not overdraw the account they went into: if
	// they have since been ground on or used, the later entries have to go first.
	if err := r.check(original.Product, original.Grams, original.To); err != nil {
		return journal.Event{}, fmt.Errorf("cannot undo %s: %w", short(hash), err)
	}
	if reason == "" {
		reason = "reverts " + short(hash)
	}
	// A correction cannot come before what it corrects: undoing an entry
	// dated ahead of today is dated with it, or the fold would take the grams
	// back before they arrived.
	at := time.Now()
	if original.OccurredAt.After(at) {
		at = original.OccurredAt
	}
	return r.append(journal.Event{
		Type:       journal.Adjust,
		Product:    original.Product,
		Grams:      original.Grams,
		From:       original.To,
		To:         original.From,
		OccurredAt: at,
		Reverts:    original.Hash,
		Note:       reason,
	})
}

// Amend corrects the amount of an earlier entry, by reverting it and recording
// it again as it should have been.
//
// The two appends are not a transaction, so everything that could refuse the
// second one is checked before the first is written. Reverting and then
// failing to re-record would leave the entry silently undone, which is worse
// than either refusing outright or recording the correction.
func (r *Recorder) Amend(hash string, grams float64, note string) (journal.Event, error) {
	original, err := r.Find(hash)
	if err != nil {
		return journal.Event{}, err
	}
	if grams <= 0 {
		return journal.Event{}, fmt.Errorf("grams must be positive, got %v", grams)
	}
	// The revert frees the original amount back into the source account, so
	// only the difference beyond it has to be there already.
	if extra := round(grams - original.Grams); extra > 0 {
		if err := r.check(original.Product, extra, original.From); err != nil {
			return journal.Event{}, fmt.Errorf("cannot amend %s to %.2fg: %w", short(hash), grams, err)
		}
	}
	if _, err := r.Revert(hash, "amended"); err != nil {
		return journal.Event{}, err
	}
	corrected := original
	corrected.Grams = grams
	corrected.Hash, corrected.Prev, corrected.Seq = "", "", 0
	corrected.RecordedAt = time.Time{}
	if note != "" {
		corrected.Note = note
	}
	return r.append(corrected)
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
	where, ok := reconcilable[account]
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

	expected := r.Available(product.Slug, account)
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
	return r.append(journal.Event{
		Type:       journal.Adjust,
		Product:    product.Slug,
		Grams:      math.Abs(difference),
		From:       from,
		To:         to,
		OccurredAt: time.Now(),
		Note:       note,
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
	product, err := r.products.Find(ref)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("a product needs a name")
	}
	product.Name = strings.TrimSpace(name)
	if err := r.products.Save(r.repo.ProductsPath()); err != nil {
		return nil, err
	}
	return product, nil
}

// Describe replaces a product's parsed details, leaving its slug and its
// entries alone.
func (r *Recorder) Describe(ref string, p catalog.Product) (*catalog.Product, error) {
	product, err := r.products.Find(ref)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(p.Name) == "" {
		return nil, fmt.Errorf("a product needs a name")
	}
	product.Name = strings.TrimSpace(p.Name)
	product.Manufacturer = strings.TrimSpace(p.Manufacturer)
	product.Cultivar = strings.TrimSpace(p.Cultivar)
	product.THC, product.CBD = p.THC, p.CBD
	if err := r.products.Save(r.repo.ProductsPath()); err != nil {
		return nil, err
	}
	return product, nil
}
