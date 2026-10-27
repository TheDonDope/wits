// Package chain is a log kept beside the journal: append-only, written under
// a lock, each record hash-chained onto the one before it and checked against
// the line as stored. The effect log and the price log are two of them.
//
// They were two copies of the same code, a struct apart, and a fix to one was
// a fix the other still needed: restoring either appended a record at a time,
// reading the whole log and flushing it to disk for each. Here the disciplines
// are kept once, and a record type says only how to reach its chain fields.
//
// A record keeps its own JSON field order, since its hash is taken over its
// encoding: the logs already written must verify as they are.
package chain

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/TheDonDope/wits/pkg/journal"
)

// Link is a record's place in its chain.
type Link struct {
	Seq        int
	At         time.Time
	Prev, Hash string
}

// Record is what a log holds, through a pointer to it: a record that can say
// whether it is well formed and where it stands in the chain. The journal
// entry it is about is named by a hash that never changes, so the log has no
// need to know which field holds it.
type Record[T any] interface {
	*T
	Validate() error
	Link() Link
	SetLink(Link)
}

// Log is a chained log stored at a path.
type Log[T any, P Record[T]] struct {
	mu   sync.Mutex
	path string
	// name and noun say what the log is in an error: "effect log", "rating".
	name, noun string
	broken     error
}

// Open returns the log at path, created on its first append. Errors call it
// name and its records noun, and a chain that does not verify wraps broken.
func Open[T any, P Record[T]](path, name, noun string, broken error) *Log[T, P] {
	return &Log[T, P]{path: path, name: name, noun: noun, broken: broken}
}

// Path returns the file the log is stored in.
func (l *Log[T, P]) Path() string { return l.path }

// Append checks a record, chains it onto the log's tip and writes it, under
// the lock the journal takes. The stored record is returned.
func (l *Log[T, P]) Append(r T) (T, error) {
	out, err := l.AppendAll([]T{r})
	if err != nil {
		var zero T
		return zero, err
	}
	return out[0], nil
}

// AppendAll chains records onto the log's tip and writes them as one: one
// read, one write, one flush, and none of them if any is refused. It is how a
// restore puts a log back — a record at a time read the log and flushed it
// once each, 19 s for 1500 ratings on a real disk.
func (l *Log[T, P]) AppendAll(records []T) ([]T, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	release, err := journal.Lock(l.path)
	if err != nil {
		return nil, err
	}
	defer release()
	before, _, err := l.read()
	if err != nil {
		return nil, err
	}
	chained := slices.Clone(before)
	var buf bytes.Buffer
	for _, r := range records {
		if r, err = seal[T, P](r, chained); err != nil {
			return nil, err
		}
		line, err := json.Marshal(r)
		if err != nil {
			return nil, err
		}
		buf.Write(append(line, '\n'))
		chained = append(chained, r)
	}
	if buf.Len() == 0 {
		return nil, nil
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := f.Write(buf.Bytes()); err != nil {
		return nil, err
	}
	return chained[len(before):], f.Sync()
}

// All returns every record, oldest first, superseded ones included.
func (l *Log[T, P]) All() ([]T, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	records, _, err := l.read()
	return records, err
}

// Verify walks the chain over the lines as stored, the way the journal's
// Verify does: the hash is the last field, so blanking it gives back the bytes
// that were hashed.
func (l *Log[T, P]) Verify() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, lines, err := l.read()
	if err != nil {
		return err
	}
	prev := ""
	for n, line := range lines {
		var r T
		if err := json.Unmarshal(line, &r); err != nil {
			return fmt.Errorf("%s line %d is not valid JSON: %w", l.name, n+1, err)
		}
		link := P(&r).Link()
		stored := []byte(`,"hash":"` + link.Hash + `"}`)
		if link.Prev != prev || !bytes.HasSuffix(line, stored) {
			return fmt.Errorf("%w at %s %d", l.broken, l.noun, link.Seq)
		}
		payload := append(bytes.Clone(line[:len(line)-len(stored)]), `,"hash":""}`...)
		sum := sha256.Sum256(append([]byte(prev), payload...))
		if hex.EncodeToString(sum[:]) != link.Hash {
			return fmt.Errorf("%w at %s %d", l.broken, l.noun, link.Seq)
		}
		prev = link.Hash
	}
	return nil
}

// Rebase brings in another device's copy of the log: shared records kept, the
// other's taken as they are, and this log's own chained again on top. The
// records keep naming the entries they named, since a pull moves no journal
// entry. It returns how many of the other's it lacked and how many of its own
// it chained again.
func (l *Log[T, P]) Rebase(onto []T) (fetched, rechained int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	release, err := journal.Lock(l.path)
	if err != nil {
		return 0, 0, err
	}
	defer release()
	if err := l.whole(onto); err != nil {
		return 0, 0, err
	}
	local, _, err := l.read()
	if err != nil {
		return 0, 0, err
	}
	k := 0
	for k < len(local) && k < len(onto) && P(&local[k]).Link().Hash == P(&onto[k]).Link().Hash {
		k++
	}
	tail := local[k:]
	if len(onto) == k {
		return 0, 0, nil // nothing new
	}
	merged := slices.Clone(onto)
	for _, r := range tail {
		if r, err = seal[T, P](r, merged); err != nil {
			return 0, 0, err
		}
		merged = append(merged, r)
	}
	var buf bytes.Buffer
	for _, r := range merged {
		line, err := json.Marshal(r)
		if err != nil {
			return 0, 0, err
		}
		buf.Write(append(line, '\n'))
	}
	return len(onto) - k, len(tail), journal.WriteFile(l.path, buf.Bytes())
}

// Chain seals records one onto the next from the start of a log, as appending
// them to an empty one would: what a bundle's records hash to once restored.
func Chain[T any, P Record[T]](records []T) ([]T, error) {
	out := make([]T, 0, len(records))
	for _, r := range records {
		sealed, err := seal[T, P](r, out)
		if err != nil {
			return nil, err
		}
		out = append(out, sealed)
	}
	return out, nil
}

// seal makes a record what the log stores after the records before it:
// dated, checked, numbered and chained.
func seal[T any, P Record[T]](r T, before []T) (T, error) {
	p := P(&r)
	link := p.Link()
	if link.At.IsZero() {
		link.At = time.Now()
	}
	link.At = link.At.Truncate(time.Second)
	p.SetLink(link)
	if err := p.Validate(); err != nil {
		return r, err
	}
	link.Seq, link.Prev = len(before)+1, ""
	if n := len(before); n > 0 {
		link.Prev = P(&before[n-1]).Link().Hash
	}
	p.SetLink(link)
	var err error
	link.Hash, err = sum[T, P](r, link.Prev)
	p.SetLink(link)
	return r, err
}

// sum is a record's hash chained onto prev, taken with the hash blank.
func sum[T any, P Record[T]](r T, prev string) (string, error) {
	link := P(&r).Link()
	link.Prev, link.Hash = prev, ""
	P(&r).SetLink(link)
	payload, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(append([]byte(prev), payload...))
	return hex.EncodeToString(h[:]), nil
}

// whole reports whether records form a whole chain from the start.
func (l *Log[T, P]) whole(records []T) error {
	prev := ""
	for i, r := range records {
		h, err := sum[T, P](r, prev)
		if err != nil {
			return err
		}
		if link := P(&r).Link(); link.Seq != i+1 || link.Prev != prev || link.Hash != h {
			return fmt.Errorf("%w at %s %d of the other copy", l.broken, l.noun, i+1)
		}
		prev = P(&r).Link().Hash
	}
	return nil
}

// read loads the complete lines and the records they hold; a line cut off
// before its newline is a write that never finished, and is left out.
// Callers hold the mutex.
func (l *Log[T, P]) read() ([]T, [][]byte, error) {
	data, err := os.ReadFile(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	data = data[:bytes.LastIndexByte(data, '\n')+1]
	var records []T
	var lines [][]byte
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := bytes.Clone(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var r T
		if err := json.Unmarshal(line, &r); err != nil {
			return nil, nil, fmt.Errorf("%s line %d is not valid JSON: %w", l.name, len(lines)+1, err)
		}
		records = append(records, r)
		lines = append(lines, line)
	}
	return records, lines, sc.Err()
}
