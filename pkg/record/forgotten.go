// Package record applies entries to a repository with the guard rails; this
// file puts a scale reading short of the ledger down to something forgotten
// rather than lost: a jar lighter than it should be was ground and the grind
// never logged, ground grams that are gone were vaped in sessions never
// logged. Grams do not vanish into thin air; a forgotten entry is the likelier
// story, and the one that keeps the habits on screen true.
package record

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
)

// ForgottenNote is what an entry worked out at the scale carries, so it can
// be told from one recorded at the time — and is not asked to be rated.
const ForgottenNote = "forgotten, worked out at the scale"

// ErrNotShort is why nothing is put down to a forgotten entry: the scale
// reads no less than the ledger, or the account is not one a grind or a
// session draws on.
var ErrNotShort = errors.New("the scale reads no less than the ledger")

// ErrTooMany is why a shortfall is not put down to forgotten sessions: it
// would take more of them than anybody forgets — a leftover, or grams lost.
var ErrTooMany = errors.New("more sessions than anybody forgets")

// usualSessions is how many of the latest sessions say what a session usually
// is: enough for a habit, few enough to follow one that changed.
const usualSessions = 30

// mostForgotten caps how many sessions a shortfall is split into: past it a
// prediction is no longer one anybody could check against their memory.
const mostForgotten = 30

// Forgotten is what a scale reading short of the ledger would be put down to,
// without recording anything: a grind for storage, sessions for the stash.
// A screen shows it before it is committed to.
func (r *Recorder) Forgotten(ref string, account journal.Account, weighed float64) ([]journal.Event, error) {
	product, err := r.products.Find(ref)
	if err != nil {
		return nil, err
	}
	return r.forgotten(product.Slug, account, weighed, time.Now())
}

// RecordForgotten records what a scale reading short of the ledger is put
// down to, in one batch: worked out again under the lock, against the
// journal as it stands, so a session recorded elsewhere meanwhile is not
// predicted a second time.
func (r *Recorder) RecordForgotten(ref string, account journal.Account, weighed float64) ([]journal.Event, error) {
	if weighed < 0 {
		return nil, fmt.Errorf("a weight cannot be negative, got %.2f", weighed)
	}
	product, err := r.products.Find(ref)
	if err != nil {
		return nil, err
	}
	return r.write(func() ([]journal.Event, error) {
		return r.forgotten(product.Slug, account, weighed, time.Now())
	})
}

// forgotten builds the entries a shortfall is put down to.
//
// They are dated after everything recorded of the jar, so no entry already
// in the journal loses a gram to them: a grind put earlier could take grams a
// recorded one took, and sessions put earlier could take grams not ground
// yet. A forgotten grind goes just after the jar's last entry, forgotten
// sessions are spread over the time from then until now.
func (r *Recorder) forgotten(slug string, account journal.Account, weighed float64, now time.Time) ([]journal.Event, error) {
	expected := r.Available(slug, account)
	short := journal.CG(expected) - journal.CG(weighed)
	if short <= 0 {
		return nil, fmt.Errorf("%w: %.2f g weighed, %.2f g expected", ErrNotShort, weighed, expected)
	}
	start := r.lastOf(slug).Add(time.Minute).Truncate(time.Minute)
	if start.After(now) {
		start = now
	}
	switch account {
	case journal.Storage:
		return []journal.Event{{
			Type: journal.Grind, Product: slug, Grams: short.Grams(), OccurredAt: start, Note: ForgottenNote,
		}}, nil
	case journal.Stash:
		return r.forgottenSessions(slug, short, start, now)
	default:
		return nil, fmt.Errorf("%w: only a jar or what is ground of it is drawn on by an entry, not %s", ErrNotShort, account)
	}
}

// lastOf is when the latest entry of a product happened, or the zero time
// for one with none.
func (r *Recorder) lastOf(slug string) time.Time {
	var last time.Time
	for _, e := range r.state.Events {
		if e.Product == slug && e.OccurredAt.After(last) {
			last = e.OccurredAt
		}
	}
	return last
}

// forgottenSessions splits the grams into sessions of the usual size; has
// them as the latest session was had — its device, mode, heater and
// temperature; and spreads them evenly from start to now. With no session
// on record, the size is a load of the gear in use, and failing that one
// session a day. The grams are shared in centigrams, the
// odd ones to the first sessions, so they add up to the shortfall exactly.
func (r *Recorder) forgottenSessions(slug string, short journal.Centigrams, start, now time.Time) ([]journal.Event, error) {
	usual, last := r.usualSession()
	if usual == 0 {
		usual = r.usualLoad()
	}
	span := now.Sub(start)
	n := int64(math.Ceil(span.Hours() / 24))
	if usual > 0 {
		n = int64(math.Round(float64(short) / float64(usual)))
	}
	n = min(max(n, 1), int64(short))
	if n > mostForgotten {
		return nil, fmt.Errorf("%w: %.2f g would be %d sessions — recorded as lost instead, or cleared as a leftover", ErrTooMany, short.Grams(), n)
	}

	out := make([]journal.Event, 0, n)
	for i := range n {
		grams := short / journal.Centigrams(n)
		if journal.Centigrams(i) < short%journal.Centigrams(n) {
			grams++
		}
		// To the minute, but never before start: the jar's last entry may be
		// in this very minute.
		at := start.Add(span * time.Duration(i+1) / time.Duration(n+1)).Truncate(time.Minute)
		if at.Before(start) {
			at = start
		}
		out = append(out, journal.Event{
			Type: journal.Sesh, Product: slug, Grams: grams.Grams(), OccurredAt: at,
			Device: last.Device, Temperature: last.Temperature, Mode: last.Mode,
			Heater: last.Heater, Setting: last.Setting, Timer: last.Timer, Note: ForgottenNote,
		})
	}
	return out, nil
}

// What forgotten sessions are sized by, as SizedBy says.
const (
	BySessions = "sessions" // the sessions on record
	ByLoad     = "load"     // the load of the gear in use
	ByDay      = "day"      // one a day, with neither
)

// SizedBy says what forgotten sessions would be sized by: the sessions on
// record, the gear's load without them, or a session a day without either.
func (r *Recorder) SizedBy() string {
	if usual, _ := r.usualSession(); usual > 0 {
		return BySessions
	}
	if r.usualLoad() > 0 {
		return ByLoad
	}
	return ByDay
}

// usualLoad is what a session on the gear in use takes: the mean load of
// the devices a session is had with, not retired, that say how much they
// hold — nothing when none does.
func (r *Recorder) usualLoad() journal.Centigrams {
	var sum journal.Centigrams
	n := 0
	for _, d := range r.devices.Devices {
		if d.Load > 0 && !d.Retired && d.SessionWith() {
			sum += journal.CG(d.Load)
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return journal.Centigrams(math.Round(float64(sum) / float64(n)))
}

// usualSession is the amount a session most often takes among the latest
// that stand — the smaller one on a tie — and the latest session itself.
// Neither is anything without a session on record.
func (r *Recorder) usualSession() (journal.Centigrams, journal.Event) {
	standing := ledger.Standing(r.state.Events)
	count := map[journal.Centigrams]int{}
	var last journal.Event
	seen := 0
	for i := len(standing) - 1; i >= 0 && seen < usualSessions; i-- {
		e := standing[i]
		if e.Type != journal.Sesh {
			continue
		}
		if seen == 0 {
			last = e
		}
		count[journal.CG(e.Grams)]++
		seen++
	}
	var usual journal.Centigrams
	for g, c := range count {
		if c > count[usual] || (c == count[usual] && g < usual) {
			usual = g
		}
	}
	return usual, last
}

// ForgottenWords says in a few words what a shortfall would be put down to:
// the grind and when, or how many sessions of about what size, over which
// days, with what — the same words on the command line and in the terminal
// interface.
func ForgottenWords(planned []journal.Event, devices *catalog.Devices) string {
	if len(planned) == 0 {
		return "nothing"
	}
	first := planned[0]
	if first.Type == journal.Grind {
		return fmt.Sprintf("a grind of %.2f g on %s, just after the jar's last entry", first.Grams, first.OccurredAt.Format("Mon 02 Jan"))
	}
	var total journal.Centigrams
	for _, e := range planned {
		total += journal.CG(e.Grams)
	}
	what := fmt.Sprintf("%d sessions of about %.2f g", len(planned), (total / journal.Centigrams(len(planned))).Grams())
	if len(planned) == 1 {
		what = fmt.Sprintf("a session of %.2f g", first.Grams)
	}
	when := first.OccurredAt.Format("02 Jan")
	if last := planned[len(planned)-1].OccurredAt.Format("02 Jan"); last != when {
		when += " – " + last
	}
	if d, err := devices.Find(first.Device); first.Device != "" && err == nil {
		return fmt.Sprintf("%s, %s, with the %s", what, when, d.Name)
	}
	return what + ", " + when
}
