package journal

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

// lock takes the cross-process append lock, returning the release function.
func (j *Journal) lock() (func(), error) { return Lock(j.path) }

// Lock takes an exclusive advisory lock for the file at path, waiting for any
// other holder, and returns the function that releases it. The lock is taken
// on a file beside it, path + ".lock", so that taking it never opens the file
// itself for writing.
//
// It is the journal's, exported because the repository's other files need the
// same discipline: a catalog is read, changed and written back, and two
// processes doing that at once lose one of the changes.
func Lock(path string) (func(), error) {
	f, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
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

// WriteFile replaces the file at path with data, so that a crash leaves either
// the old file or the new one and never a part of either. The data goes to a
// temporary file beside it, which is flushed and then renamed over the old
// one; a rename within a directory is atomic.
//
// The journal is only ever appended to; this is for the files that are
// rewritten whole — the catalogs and the config — which os.WriteFile truncates
// first, so a crash between the truncate and the write left them empty.
func WriteFile(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // a no-op once renamed
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
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
		if e, err = seal(e, seq, tip); err != nil {
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

// seal makes an entry what the journal stores after seq entries ending in
// tip: its defaults filled in, checked, numbered and chained.
func seal(e Event, seq int, tip string) (Event, error) {
	e = withDefaults(e)
	if err := e.Validate(); err != nil {
		return e, err
	}
	e.Seq, e.Prev = seq+1, tip
	var err error
	e.Hash, err = e.sum(tip)
	return e, err
}

// Chain seals entries one onto the next from the start of a journal, as
// appending them to an empty one would: what a bundle's entries hash to once
// restored, read without restoring them.
func Chain(events []Event) ([]Event, error) {
	out := make([]Event, len(events))
	tip := ""
	for i, e := range events {
		var err error
		if out[i], err = seal(e, i, tip); err != nil {
			return nil, err
		}
		tip = out[i].Hash
	}
	return out, nil
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
//
// Each hash is checked against the line as stored, not against the event
// decoded and encoded again. Re-encoding ties every old hash to how this
// build's Event encodes: a field added without omitempty, or a float written
// differently, would have broken every entry ever made, though not a byte of
// the file had changed. The stored line differs from what was hashed only in
// the hash itself, which is the last field, so blanking it gives back exactly
// the bytes that were hashed.
func (j *Journal) Verify() error {
	j.mu.Lock()
	lines, _, err := j.lines()
	j.mu.Unlock()
	if err != nil {
		return err
	}
	var prev string
	for n, line := range lines {
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			return fmt.Errorf("journal line %d is not valid JSON: %w", n+1, err)
		}
		if e.Prev != prev {
			return fmt.Errorf("%w: event %d expects prev %q but the chain is at %q", ErrBrokenChain, e.Seq, e.Prev, prev)
		}
		stored := []byte(`,"hash":"` + e.Hash + `"}`)
		if !bytes.HasSuffix(line, stored) {
			return fmt.Errorf("%w: event %d does not end in its hash", ErrBrokenChain, e.Seq)
		}
		payload := append(bytes.Clone(line[:len(line)-len(stored)]), `,"hash":""}`...)
		sum := sha256.Sum256(append([]byte(prev), payload...))
		if want := hex.EncodeToString(sum[:]); e.Hash != want {
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
	lines, valid, err := j.lines()
	if err != nil {
		return nil, 0, err
	}
	events := make([]Event, 0, len(lines))
	for n, line := range lines {
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			return nil, 0, fmt.Errorf("journal line %d is not valid JSON: %w", n+1, err)
		}
		events = append(events, e)
	}
	return events, valid, nil
}

// lines returns the journal's complete lines as stored, and how many bytes of
// the file they fill; see read. Callers must hold the mutex.
func (j *Journal) lines() ([][]byte, int64, error) {
	data, err := os.ReadFile(j.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, 0, err
	}
	data = data[:bytes.LastIndexByte(data, '\n')+1]
	var lines [][]byte
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		if len(line) > 0 {
			lines = append(lines, line)
		}
	}
	return lines, int64(len(data)), nil
}

// Rebased is what a Rebase did: how many of the other chain's entries this
// journal lacked, and which of its own it chained again on top of them, by
// the hash each had and the hash it has now.
type Rebased struct {
	Fetched int
	Moved   map[string]string
}

// Rebase brings in another device's copy of the journal. The entries the two
// share are kept; the ones only the other holds are taken as they are, hashes
// and all; and the ones only this journal holds are chained again on top of
// them, the way git rebases a branch that was never pushed. Those get new
// hashes — a hash names a place in the chain — and an undo among them is
// pointed at the new hash of what it undoes. The journal is rewritten whole,
// atomically, and the old one kept beside it as journal.ndjson.orig.
//
// then runs under the journal's lock once the journal is written, so the
// effect log can follow the sessions that moved before anything else can
// record against their old hashes.
func (j *Journal) Rebase(onto []Event, then func(Rebased) error) (Rebased, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	release, err := j.lock()
	if err != nil {
		return Rebased{}, err
	}
	defer release()
	if err := chained(onto); err != nil {
		return Rebased{}, err
	}
	// prime, not read: it cuts away a write that never finished, which a
	// fast-forward would otherwise append onto.
	local, err := j.prime(true)
	if err != nil {
		return Rebased{}, err
	}
	k := shared(local, onto)
	done := Rebased{Fetched: len(onto) - k, Moved: map[string]string{}}
	if done.Fetched > 0 {
		if err := j.rewrite(local, onto, k, done.Moved); err != nil {
			return Rebased{}, err
		}
	}
	return done, then(done)
}

// rewrite puts onto in place of the journal's first k entries, the ones
// after them chained on top, recording in moved each hash that changed.
func (j *Journal) rewrite(local, onto []Event, k int, moved map[string]string) error {
	j.primed = false
	if k == len(local) {
		return j.write(encode(onto[k:]))
	}
	merged := slices.Clone(onto)
	for _, e := range local[k:] {
		if to, ok := moved[e.Reverts]; ok {
			e.Reverts = to
		}
		old := e.Hash
		e.Seq, e.Prev = len(merged)+1, merged[len(merged)-1].Hash
		var err error
		if e.Hash, err = e.sum(e.Prev); err != nil {
			return err
		}
		moved[old] = e.Hash
		merged = append(merged, e)
	}
	if err := copyFile(j.path, j.path+".orig"); err != nil {
		return err
	}
	return WriteFile(j.path, encode(merged))
}

// chained reports whether events form a whole chain from the start: each
// numbered in turn, naming the one before, and hashing to its own hash.
func chained(events []Event) error {
	prev := ""
	for i, e := range events {
		sum, err := e.sum(prev)
		if err != nil {
			return err
		}
		if e.Seq != i+1 || e.Prev != prev || e.Hash != sum {
			return fmt.Errorf("%w: entry %d of the other copy", ErrBrokenChain, i+1)
		}
		prev = e.Hash
	}
	return nil
}

// shared is how many entries two chains begin with in common. Each hash
// covers everything before it, so the first that differs ends what is shared.
func shared(a, b []Event) int {
	k := 0
	for k < len(a) && k < len(b) && a[k].Hash == b[k].Hash {
		k++
	}
	return k
}

// encode writes events as they are stored, a line each.
func encode(events []Event) []byte {
	var buf bytes.Buffer
	for _, e := range events {
		line, _ := json.Marshal(e) // an Event always encodes
		buf.Write(line)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// copyFile copies a file whole, atomically; a missing one copies nothing.
func copyFile(from, to string) error {
	data, err := os.ReadFile(from)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return WriteFile(to, data)
}
