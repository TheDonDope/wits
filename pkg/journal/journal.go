package journal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

// ErrBrokenChain is returned when a journal's hash chain does not verify, which
// means the file was edited or truncated outside of Wits.
var ErrBrokenChain = errors.New("journal hash chain is broken")

// Journal is an append-only log of events stored as newline-delimited JSON.
//
// The file is only ever opened for appending. Wits never rewrites it, so a
// failed or partial write can add a bad line but can never destroy an existing
// one. The single exception is that bad line itself: a write cut off before
// its newline — a crash, a full disk — is ignored on reading, and cut away
// before the next append (see prime).
//
// Appending is guarded twice over. The mutex serialises writers inside this
// process; an advisory lock on a file beside the journal serialises them across
// processes. Both are needed, because appending is a read-then-write: the tip is
// read to chain onto, and two writers that read the same tip would append two
// entries claiming the same predecessor and fork the chain. That matters as soon
// as anything other than a single command talks to a repository.
type Journal struct {
	mu   sync.Mutex
	path string

	// The tip of the chain, cached so that a run of appends — a restore is a
	// thousand of them — does not re-read the whole file for every line. The
	// file size pins the cache: another process may append between two of
	// ours, and a size that no longer matches means the tip must be re-read.
	seq    int
	tip    string
	size   int64
	primed bool
}

// lockPath is the file the cross-process lock is taken on. It is separate from
// the journal so that taking it never opens the journal for writing.
func (j *Journal) lockPath() string { return j.path + ".lock" }

// lock takes the cross-process append lock, returning the release function.
func (j *Journal) lock() (func(), error) {
	f, err := os.OpenFile(j.lockPath(), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := lockFile(f); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		unlockFile(f)
		f.Close()
	}, nil
}

// Open returns the journal stored at path. The file is created on the first
// append, so opening a journal that does not exist yet is not an error.
func Open(path string) *Journal {
	return &Journal{path: path}
}

// Path returns the file the journal is stored in.
func (j *Journal) Path() string { return j.path }

// Append validates the event, chains it onto the current tip and writes it as a
// single line. The stored event is returned with its Seq, Prev and Hash filled
// in.
func (j *Journal) Append(e Event) (Event, error) {
	stored, err := j.append(false, func([]Event) ([]Event, error) { return []Event{e}, nil })
	if err != nil {
		return Event{}, err
	}
	return stored[0], nil
}

// AppendWith builds entries from the journal as it stands and appends them,
// all under the locks, so nothing can be appended in between. build gets every
// entry, read fresh, and returns the ones to append, or an error to append
// nothing; returning none is not an error.
//
// It is how a check against the history holds. Checked anywhere else, it is a
// check of a copy: a command in another terminal can grind a jar to nothing
// between a check that it holds enough and the grind that relies on it, and
// two such grinds once left storage at -17.10 g.
//
// The entries are written in a single write, chained one onto the next.
func (j *Journal) AppendWith(build func(events []Event) ([]Event, error)) ([]Event, error) {
	return j.append(true, build)
}

// append takes both locks, brings the tip up to date — reading every entry for
// build when fresh is set — and writes what build returns.
func (j *Journal) append(fresh bool, build func([]Event) ([]Event, error)) ([]Event, error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	release, err := j.lock()
	if err != nil {
		return nil, err
	}
	defer release()

	events, err := j.prime(fresh)
	if err != nil {
		return nil, err
	}
	batch, err := build(events)
	if err != nil || len(batch) == 0 {
		return nil, err
	}

	var buf bytes.Buffer
	seq, tip := j.seq, j.tip
	stored := make([]Event, 0, len(batch))
	for _, e := range batch {
		e = withDefaults(e)
		if err := e.Validate(); err != nil {
			return nil, err
		}
		e.Seq, e.Prev = seq+1, tip
		if e.Hash, err = e.sum(tip); err != nil {
			return nil, err
		}
		line, err := json.Marshal(e)
		if err != nil {
			return nil, err
		}
		buf.Write(line)
		buf.WriteByte('\n')
		seq, tip = e.Seq, e.Hash
		stored = append(stored, e)
	}
	if err := j.write(buf.Bytes()); err != nil {
		// The cache no longer describes the file for certain; the next append
		// re-reads rather than chaining onto a tip that may not exist.
		j.primed = false
		return nil, err
	}
	j.seq, j.tip = seq, tip
	j.size += int64(buf.Len())
	return stored, nil
}

// prime brings the cached tip in line with the file, and returns every entry
// when fresh is set or the file had to be read anyway. Callers must hold both
// locks. The size check is what keeps the cache honest across processes: our
// own appends grow the file by exactly what was written, so a size that does
// not match means somebody else appended and the tip has moved.
//
// An unfinished last line is truncated here, under the lock, so the next
// line starts on a line of its own rather than gluing itself onto the
// fragment. Those bytes were never an event: every complete one ends in a
// newline, and the append that wrote them reported its failure.
func (j *Journal) prime(fresh bool) ([]Event, error) {
	var size int64
	if st, err := os.Stat(j.path); err == nil {
		size = st.Size()
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if j.primed && size == j.size && !fresh {
		return nil, nil
	}
	events, valid, err := j.read()
	if err != nil {
		return nil, err
	}
	if size > valid {
		if err := os.Truncate(j.path, valid); err != nil {
			return nil, err
		}
		size = valid
	}
	j.seq = len(events)
	j.tip = ""
	if n := len(events); n > 0 {
		j.tip = events[n-1].Hash
	}
	j.size = size
	j.primed = true
	return events, nil
}

// withDefaults fills in the fields a caller may leave to the journal: the
// timestamps and the accounts implied by the event type.
//
// Timestamps are truncated to the second. Sub-second precision says nothing
// useful about when something was ground, and dropping it here means an event
// survives a round trip through a bundle unchanged, hash included.
func withDefaults(e Event) Event {
	if e.RecordedAt.IsZero() {
		e.RecordedAt = time.Now()
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = e.RecordedAt
	}
	e.RecordedAt = e.RecordedAt.Truncate(time.Second)
	e.OccurredAt = e.OccurredAt.Truncate(time.Second)
	if from, to, ok := Flow(e.Type); ok && e.From == "" && e.To == "" {
		e.From, e.To = from, to
	}
	return e
}

// write appends whole lines to the journal and flushes them to disk. The file
// is only ever opened for appending, so a failed write can add a bad line but
// can never destroy an existing one.
func (j *Journal) write(lines []byte) error {
	f, err := os.OpenFile(j.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(lines); err != nil {
		return err
	}
	return f.Sync()
}

// Events returns every event in the journal, oldest first.
func (j *Journal) Events() ([]Event, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	events, _, err := j.read()
	return events, err
}

// Verify walks the hash chain and reports the first entry that does not match.
// An empty or missing journal verifies successfully.
func (j *Journal) Verify() error {
	events, err := j.Events()
	if err != nil {
		return err
	}
	var prev string
	for _, e := range events {
		want, err := e.sum(prev)
		if err != nil {
			return err
		}
		if e.Prev != prev {
			return fmt.Errorf("%w: event %d expects prev %q but the chain is at %q", ErrBrokenChain, e.Seq, e.Prev, prev)
		}
		if e.Hash != want {
			return fmt.Errorf("%w: event %d has hash %q but hashes to %q", ErrBrokenChain, e.Seq, e.Hash, want)
		}
		prev = e.Hash
	}
	return nil
}

// read loads and decodes the whole journal, returning the events and how
// many bytes of the file they fill. Callers must hold the mutex.
//
// A missing file is an empty journal, but any other read error is returned:
// silently treating an unreadable journal as empty is how an append would go on
// to record a first event on top of years of history.
//
// Every stored line ends in a newline, so bytes after the last one are a write
// that never finished. They are left out rather than refused: one cut-off line
// used to make the whole repository unreadable, every command included. A bad
// line that did end in a newline is still an error — that is damage, not an
// interrupted write.
func (j *Journal) read() ([]Event, int64, error) {
	data, err := os.ReadFile(j.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, 0, err
	}
	data = data[:bytes.LastIndexByte(data, '\n')+1]

	var events []Event
	for n, line := range bytes.Split(data, []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			return nil, 0, fmt.Errorf("journal line %d is not valid JSON: %w", n+1, err)
		}
		events = append(events, e)
	}
	return events, int64(len(data)), nil
}
