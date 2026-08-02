package journal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// Account is one of the places grams can sit. Every event moves grams between
// two accounts, so mass is conserved from the pharmacy through to the AVB.
type Account string

const (
	// External is outside the system: the pharmacy on the way in, and whatever
	// the AVB gets used for on the way out.
	External Account = "external"
	// Storage holds sealed product as it was dispensed.
	Storage Account = "storage"
	// Stash holds ground product, one per product.
	Stash Account = "stash"
	// Consumed holds what a session has put through a device but that has not
	// been weighed as AVB yet.
	Consumed Account = "consumed"
	// AVB holds already vaped bud, weighed after collection.
	AVB Account = "avb"
)

// Type is the kind of an event. Each type implies a fixed pair of accounts,
// described by Flow.
type Type string

const (
	// Purchase brings a prescription fill into storage.
	Purchase Type = "purchase"
	// Grind moves product from storage into the stash.
	Grind Type = "grind"
	// Sesh is a session: it draws ground product from the stash and puts it
	// through a device. What is left of it becomes AVB once weighed.
	Sesh Type = "sesh"
	// AVBCollect records already vaped bud as weighed when emptying a device.
	AVBCollect Type = "avb-collect"
	// AVBUse draws already vaped bud down for edibles, tincture or similar.
	AVBUse Type = "avb-use"
	// Adjust corrects a balance for a spill or a scale correction.
	Adjust Type = "adjust"
)

// flows maps each event type to the accounts it moves grams between.
var flows = map[Type][2]Account{
	Purchase:   {External, Storage},
	Grind:      {Storage, Stash},
	Sesh:       {Stash, Consumed},
	AVBCollect: {Consumed, AVB},
	AVBUse:     {AVB, External},
	Adjust:     {External, External},
}

// Flow returns the accounts an event type moves grams from and to.
func Flow(t Type) (from, to Account, ok bool) {
	f, ok := flows[t]
	return f[0], f[1], ok
}

// Event is a single immutable entry in the journal.
//
// OccurredAt is when the grams actually moved, RecordedAt is when it was typed
// in. They differ whenever an entry is backdated, which keeps a late entry
// honest about being late. Both are kept to second precision: nanoseconds are
// noise in a log measured in grams per day, and the extra digits would have to
// be carried through every export to keep the hashes reproducible.
//
// An event has no identifier of its own. Its hash names it, the way a commit
// hash names a commit, which is what lets a bundle be restored into a journal
// that verifies against the one it came from.
//
// Field order is significant: the hash is taken over the JSON encoding, and
// encoding/json emits fields in declaration order.
type Event struct {
	Seq         int       `json:"seq"`
	Type        Type      `json:"type"`
	OccurredAt  time.Time `json:"occurred_at"`
	RecordedAt  time.Time `json:"recorded_at"`
	Product     string    `json:"product,omitempty"`
	Grams       float64   `json:"grams"`
	From        Account   `json:"from"`
	To          Account   `json:"to"`
	Device      string    `json:"device,omitempty"`
	Temperature int       `json:"temperature,omitempty"`
	Note        string    `json:"note,omitempty"`
	Reverts     string    `json:"reverts,omitempty"`
	Prev        string    `json:"prev"`
	Hash        string    `json:"hash"`
}

// Validate reports whether the event is well formed enough to be appended.
func (e Event) Validate() error {
	from, to, ok := Flow(e.Type)
	if !ok {
		return fmt.Errorf("unknown event type %q", e.Type)
	}
	if e.Type != Adjust && (e.From != from || e.To != to) {
		return fmt.Errorf("event type %q moves %s -> %s, not %s -> %s", e.Type, from, to, e.From, e.To)
	}
	if e.Grams <= 0 {
		return fmt.Errorf("grams must be positive, got %v", e.Grams)
	}
	if e.Type != AVBUse && e.Product == "" {
		return fmt.Errorf("event type %q requires a product", e.Type)
	}
	if e.OccurredAt.IsZero() {
		return fmt.Errorf("event has no occurred_at timestamp")
	}
	return nil
}

// sum returns the hash of the event chained onto prev. The event's own Hash is
// excluded from the calculation, so that hashing is reproducible from the
// stored line.
func (e Event) sum(prev string) (string, error) {
	e.Prev = prev
	e.Hash = ""
	payload, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(append([]byte(prev), payload...))
	return hex.EncodeToString(h[:]), nil
}

// String returns a one-line representation of the event, in the spirit of
// `git log --oneline`.
func (e Event) String() string {
	at := e.OccurredAt.Format("2006-01-02")
	short := e.Hash
	if len(short) > 7 {
		short = short[:7]
	}
	if e.Product == "" {
		return fmt.Sprintf("%s %s %-11s %.2fg", short, at, e.Type, e.Grams)
	}
	return fmt.Sprintf("%s %s %-11s %.2fg %s", short, at, e.Type, e.Grams, e.Product)
}

// Marshal encodes the event exactly as it is stored in the journal. It exists
// so that a restored journal can be compared byte for byte against the one it
// was bundled from.
func Marshal(e Event) ([]byte, error) { return json.Marshal(e) }
