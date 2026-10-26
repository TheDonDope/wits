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

// Shape is where an entry stands among a list's when it does not simply
// follow the one before it: the position, counted from 1, of the entry its
// Prev names, of the one its Merge names, and its Seq. Zero is the plain
// case — the entry before, no merge, its own position. Only entries brought
// in from another device, and the merges that join them, have one.
type Shape struct {
	Prev, Merge, Seq int
}

// Chain seals entries one onto the next from the start of a journal, as
// appending them to an empty one would — or, for an entry with a shape, onto
// the entries its shape names: what a bundle's entries hash to once restored,
// read without restoring them. shapes is by position, and may be shorter.
func Chain(events []Event, shapes []Shape) ([]Event, error) {
	out := make([]Event, len(events))
	for i, e := range events {
		var sh Shape
		if i < len(shapes) {
			sh = shapes[i]
		}
		prev := ""
		switch {
		case sh.Prev > 0 && sh.Prev <= i:
			prev = out[sh.Prev-1].Hash
		case sh.Prev != 0:
			return nil, fmt.Errorf("%w: entry %d names an entry after it", ErrBrokenChain, i+1)
		case i > 0:
			prev = out[i-1].Hash
		}
		if sh.Merge > 0 {
			if sh.Merge > i {
				return nil, fmt.Errorf("%w: entry %d merges an entry after it", ErrBrokenChain, i+1)
			}
			e.Merge = out[sh.Merge-1].Hash
		}
		seq := i
		if sh.Seq > 0 {
			seq = sh.Seq - 1
		}
		var err error
		if out[i], err = seal(e, seq, prev); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Shapes is what Chain needs to rebuild events as they stand: the shape of
// every entry that does not simply follow the one before it, by position —
// nil for a journal no merge ever touched.
func Shapes(events []Event) []Shape {
	at := map[string]int{}
	var out []Shape
	for i, e := range events {
		var sh Shape
		if i > 0 && e.Prev != events[i-1].Hash {
			sh.Prev = at[e.Prev]
		}
		if e.Merge != "" {
			sh.Merge = at[e.Merge]
		}
		if e.Seq != i+1 {
			sh.Seq = e.Seq
		}
		if sh != (Shape{}) {
			for len(out) < i {
				out = append(out, Shape{})
			}
			out = append(out, sh)
		}
		at[e.Hash] = i + 1
	}
	return out
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
	links := newLinks()
	for n, line := range lines {
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			return fmt.Errorf("journal line %d is not valid JSON: %w", n+1, err)
		}
		if err := links.follows(e, n); err != nil {
			return err
		}
		stored := []byte(`,"hash":"` + e.Hash + `"}`)
		if !bytes.HasSuffix(line, stored) {
			return fmt.Errorf("%w: event %d does not end in its hash", ErrBrokenChain, e.Seq)
		}
		payload := append(bytes.Clone(line[:len(line)-len(stored)]), `,"hash":""}`...)
		sum := sha256.Sum256(append([]byte(e.Prev), payload...))
		if want := hex.EncodeToString(sum[:]); e.Hash != want {
			return fmt.Errorf("%w: event %d has hash %q but hashes to %q", ErrBrokenChain, e.Seq, e.Hash, want)
		}
		links.add(e)
	}
	return links.oneHead()
}

// links checks that entries are chained as a journal's must be: each naming
// as its Prev — and a merge as its Merge too — an entry that comes before
// it, the first alone naming none; and that every entry but the last is
// named by one after it, so the last one's hash covers them all.
type links struct {
	seen, named map[string]bool
	last        string
}

func newLinks() *links { return &links{seen: map[string]bool{}, named: map[string]bool{}} }

// follows reports whether the n-th entry, counted from 0, names only entries before it.
func (l *links) follows(e Event, n int) error {
	switch {
	case n == 0 && e.Prev != "":
		return fmt.Errorf("%w: the first entry names %q before it", ErrBrokenChain, e.Prev)
	case n > 0 && !l.seen[e.Prev]:
		return fmt.Errorf("%w: event %d expects prev %q, which comes nowhere before it", ErrBrokenChain, e.Seq, e.Prev)
	case e.Merge != "" && !l.seen[e.Merge]:
		return fmt.Errorf("%w: event %d merges %q, which comes nowhere before it", ErrBrokenChain, e.Seq, e.Merge)
	}
	return nil
}

func (l *links) add(e Event) {
	l.seen[e.Hash], l.named[e.Prev] = true, true
	if e.Merge != "" {
		l.named[e.Merge] = true
	}
	l.last = e.Hash
}

// oneHead reports whether every entry but the last is named by a later one.
func (l *links) oneHead() error {
	for h := range l.seen {
		if h != l.last && !l.named[h] {
			return fmt.Errorf("%w: %.7s is named by no entry after it, so no head covers it", ErrBrokenChain, h)
		}
	}
	return nil
}

// linked checks events read from elsewhere — another device's copy, a
// bundle — as Verify checks the file: each chained onto what it names, and
// hashing to its own hash.
func linked(events []Event) error {
	l := newLinks()
	for i, e := range events {
		if err := l.follows(e, i); err != nil {
			return err
		}
		sum, err := e.sum(e.Prev)
		if err != nil {
			return err
		}
		if e.Hash != sum {
			return fmt.Errorf("%w: entry %d of the other copy", ErrBrokenChain, i+1)
		}
		l.add(e)
	}
	return l.oneHead()
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

// Holds reports whether events hold a head sealed after the given number of
// entries: the head is among them, and it covers exactly that many — itself
// and every entry it names, and they theirs. A merged journal holds the same
// entries on every device, though not at the same lines, so a seal is
// checked by what its head covers, not by where it stands.
func Holds(events []Event, head string, entries int) bool {
	at := map[string]Event{}
	for _, e := range events {
		at[e.Hash] = e
	}
	if _, ok := at[head]; !ok {
		return false
	}
	covered := map[string]bool{}
	for todo := []string{head}; len(todo) > 0; {
		h := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		e, ok := at[h]
		if !ok || covered[h] {
			continue
		}
		covered[h] = true
		todo = append(todo, e.Prev)
		if e.Merge != "" {
			todo = append(todo, e.Merge)
		}
	}
	return len(covered) == entries
}

// Merged is what a Merge did: how many of the other copy's entries this
// journal lacked, and whether a merge entry joined the two heads.
type Merged struct {
	Fetched int
	Joined  bool
}

// Merge brings in another device's copy of the journal. Its entries this
// journal lacks are appended as they are, hashes and all; if this journal
// holds entries the other lacks too, a merge entry follows them, naming both
// heads, as git merges two branches rather than rebasing one. Nothing already
// here is rewritten: every entry keeps the hash it was given, so what rates,
// prices or seals it still names it, on every device. Appended only, like
// every other write.
//
// then runs under the journal's lock once the journal is written.
func (j *Journal) Merge(theirs []Event, then func(Merged) error) (Merged, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	release, err := j.lock()
	if err != nil {
		return Merged{}, err
	}
	defer release()
	if err := linked(theirs); err != nil {
		return Merged{}, err
	}
	// prime, not read: it cuts away a write that never finished.
	local, err := j.prime(true)
	if err != nil {
		return Merged{}, err
	}
	ours := map[string]bool{}
	for _, e := range local {
		ours[e.Hash] = true
	}
	held := map[string]bool{}
	var batch []Event
	for _, e := range theirs {
		held[e.Hash] = true
		if !ours[e.Hash] {
			batch = append(batch, e)
		}
	}
	done := Merged{Fetched: len(batch)}
	if len(batch) == 0 {
		return done, then(done)
	}
	if n := len(local); n > 0 && !held[local[n-1].Hash] {
		join, err := seal(Event{Type: Merge, From: External, To: External, Merge: local[n-1].Hash,
			OccurredAt: time.Now(), RecordedAt: time.Now()}, len(local)+len(batch), batch[len(batch)-1].Hash)
		if err != nil {
			return Merged{}, err
		}
		batch = append(batch, join)
		done.Joined = true
	}
	j.primed = false
	if err := j.write(encode(batch)); err != nil {
		return Merged{}, err
	}
	return done, then(done)
}

// Import writes a whole journal into an empty one as it stands — every entry
// with the hash it has, merges and all — after checking it chains: what a
// restore writes, so the journal it makes verifies against the one bundled.
func (j *Journal) Import(events []Event) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	release, err := j.lock()
	if err != nil {
		return err
	}
	defer release()
	local, err := j.prime(true)
	if err != nil {
		return err
	}
	if len(local) > 0 {
		return fmt.Errorf("journal already holds %d events; restore into an empty repository", len(local))
	}
	if err := linked(events); err != nil {
		return err
	}
	j.primed = false
	return j.write(encode(events))
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
