package journal

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ErrBrokenChain is returned when a journal's hash chain does not verify, which
// means the file was edited or truncated outside of Wits.
var ErrBrokenChain = errors.New("journal hash chain is broken")

// Journal is an append-only log of events stored as newline-delimited JSON.
//
// The file is only ever opened for appending. Wits never rewrites it, so a
// failed or partial write can add a bad line but can never destroy an existing
// one.
type Journal struct {
	mu   sync.Mutex
	path string
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
	j.mu.Lock()
	defer j.mu.Unlock()

	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.RecordedAt.IsZero() {
		e.RecordedAt = time.Now()
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = e.RecordedAt
	}
	if from, to, ok := Flow(e.Type); ok && e.From == "" && e.To == "" {
		e.From, e.To = from, to
	}
	if err := e.Validate(); err != nil {
		return Event{}, err
	}

	events, err := j.read()
	if err != nil {
		return Event{}, err
	}
	var prev string
	if n := len(events); n > 0 {
		prev = events[n-1].Hash
	}
	e.Seq = len(events) + 1
	e.Prev = prev
	if e.Hash, err = e.sum(prev); err != nil {
		return Event{}, err
	}

	line, err := json.Marshal(e)
	if err != nil {
		return Event{}, err
	}

	f, err := os.OpenFile(j.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return Event{}, err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return Event{}, err
	}
	if err := f.Sync(); err != nil {
		return Event{}, err
	}
	log.Printf("✅ 📓  (pkg/journal/journal.go) Append() -> %v \n", e)
	return e, nil
}

// Events returns every event in the journal, oldest first.
func (j *Journal) Events() ([]Event, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.read()
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

// read loads and decodes the whole journal. Callers must hold the mutex.
//
// A missing file is an empty journal, but any other read error is returned:
// silently treating an unreadable journal as empty is how an append would go on
// to record a first event on top of years of history.
func (j *Journal) read() ([]Event, error) {
	f, err := os.Open(j.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var events []Event
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			return nil, fmt.Errorf("journal line %d is not valid JSON: %w", len(events)+1, err)
		}
		events = append(events, e)
	}
	if err := sc.Err(); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return events, nil
}
