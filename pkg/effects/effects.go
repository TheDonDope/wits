// Package effects is the effect log: what a session did, rated afterwards.
//
// The journal says where the grams went; this says what they did. It is kept
// beside the journal rather than in it, because the journal's doctrine is that
// every entry is a transfer and mass is conserved end to end, and a rating
// moves no grams. It keeps the journal's other disciplines: it is append-only,
// written under a lock, and hash-chained, each hash checked against the line
// as it was stored. A rating names the session it rates by the session's hash;
// rating a session again supersedes the earlier rating, which stays in the log.
package effects

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/TheDonDope/wits/pkg/journal"
)

// Dimension is one thing a session can be rated on.
type Dimension string

// The dimensions a session is rated on, each from 1 (not at all) to 5 (very
// well). Overall is how it went; the rest are what it was for.
const (
	Overall  Dimension = "overall"
	Pain     Dimension = "pain"
	Sleep    Dimension = "sleep"
	Calm     Dimension = "calm"
	Mood     Dimension = "mood"
	Focus    Dimension = "focus"
	Appetite Dimension = "appetite"
)

// Dimensions are every dimension, in the order they are asked.
var Dimensions = []Dimension{Overall, Pain, Sleep, Calm, Mood, Focus, Appetite}

// Lowest and Highest bound a score.
const (
	Lowest  = 1
	Highest = 5
)

// ErrBrokenChain is returned when the log's hash chain does not verify.
var ErrBrokenChain = errors.New("effect log hash chain is broken")

// Rating is what one session did, as rated afterwards. Field order is
// significant: the hash is taken over the JSON encoding, which follows it, and
// the scores encode in sorted key order, so the encoding is fixed.
type Rating struct {
	Seq     int               `json:"seq"`
	Session string            `json:"session"`
	At      time.Time         `json:"at"`
	Scores  map[Dimension]int `json:"scores"`
	Note    string            `json:"note,omitempty"`
	Prev    string            `json:"prev"`
	Hash    string            `json:"hash"`
}

// Validate reports whether a rating is well formed: a session named, and at
// least one known dimension scored within bounds.
func (r Rating) Validate() error {
	if r.Session == "" {
		return errors.New("a rating names the session it rates")
	}
	if len(r.Scores) == 0 {
		return errors.New("a rating scores at least one thing")
	}
	for d, v := range r.Scores {
		if !known(d) {
			return fmt.Errorf("%q is not something a session is rated on; the dimensions are %s", d, names())
		}
		if v < Lowest || v > Highest {
			return fmt.Errorf("%s is scored %d to %d, not %d", d, Lowest, Highest, v)
		}
	}
	return nil
}

func known(d Dimension) bool {
	for _, k := range Dimensions {
		if k == d {
			return true
		}
	}
	return false
}

func names() string {
	out := make([]string, len(Dimensions))
	for i, d := range Dimensions {
		out[i] = string(d)
	}
	return strings.Join(out, ", ")
}

// sum is the rating's hash chained onto prev, taken with the hash blank.
func (r Rating) sum(prev string) (string, error) {
	r.Prev, r.Hash = prev, ""
	payload, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(append([]byte(prev), payload...))
	return hex.EncodeToString(h[:]), nil
}

// Log is the effect log stored at a path.
type Log struct {
	mu   sync.Mutex
	path string
}

// Open returns the log at path; it is created on the first append.
func Open(path string) *Log { return &Log{path: path} }

// Path returns the file the log is stored in.
func (l *Log) Path() string { return l.path }

// Append validates a rating, chains it onto the log's tip and writes it, under
// the same kind of lock the journal takes. The stored rating is returned.
func (l *Log) Append(r Rating) (Rating, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	release, err := journal.Lock(l.path)
	if err != nil {
		return Rating{}, err
	}
	defer release()

	if r.At.IsZero() {
		r.At = time.Now()
	}
	r.At = r.At.Truncate(time.Second)
	if err := r.Validate(); err != nil {
		return Rating{}, err
	}
	ratings, _, err := l.read()
	if err != nil {
		return Rating{}, err
	}
	r.Seq, r.Prev = len(ratings)+1, ""
	if n := len(ratings); n > 0 {
		r.Prev = ratings[n-1].Hash
	}
	if r.Hash, err = r.sum(r.Prev); err != nil {
		return Rating{}, err
	}
	line, err := json.Marshal(r)
	if err != nil {
		return Rating{}, err
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return Rating{}, err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return Rating{}, err
	}
	return r, f.Sync()
}

// All returns every rating, oldest first, superseded ones included.
func (l *Log) All() ([]Rating, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	ratings, _, err := l.read()
	return ratings, err
}

// Verify walks the chain over the lines as stored, the way the journal's
// Verify does: the hash is the last field, so blanking it gives back the bytes
// that were hashed.
func (l *Log) Verify() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, lines, err := l.read()
	if err != nil {
		return err
	}
	prev := ""
	for n, line := range lines {
		var r Rating
		if err := json.Unmarshal(line, &r); err != nil {
			return fmt.Errorf("effect log line %d is not valid JSON: %w", n+1, err)
		}
		stored := []byte(`,"hash":"` + r.Hash + `"}`)
		if r.Prev != prev || !bytes.HasSuffix(line, stored) {
			return fmt.Errorf("%w at rating %d", ErrBrokenChain, r.Seq)
		}
		payload := append(bytes.Clone(line[:len(line)-len(stored)]), `,"hash":""}`...)
		sum := sha256.Sum256(append([]byte(prev), payload...))
		if hex.EncodeToString(sum[:]) != r.Hash {
			return fmt.Errorf("%w at rating %d", ErrBrokenChain, r.Seq)
		}
		prev = r.Hash
	}
	return nil
}

// read loads the complete lines and the ratings they hold; a line cut off
// before its newline is a write that never finished, and is left out. Callers
// hold the mutex.
func (l *Log) read() ([]Rating, [][]byte, error) {
	data, err := os.ReadFile(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	data = data[:bytes.LastIndexByte(data, '\n')+1]
	var ratings []Rating
	var lines [][]byte
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := bytes.Clone(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var r Rating
		if err := json.Unmarshal(line, &r); err != nil {
			return nil, nil, fmt.Errorf("effect log line %d is not valid JSON: %w", len(lines)+1, err)
		}
		ratings = append(ratings, r)
		lines = append(lines, line)
	}
	return ratings, lines, sc.Err()
}

// Latest returns each session's newest rating: a session rated again is rated
// as it was rated last.
func Latest(ratings []Rating) map[string]Rating {
	out := make(map[string]Rating, len(ratings))
	for _, r := range ratings {
		out[r.Session] = r
	}
	return out
}

// Group is what one kind of session did on average: how many were rated, and
// the mean score on each dimension that any of them scored.
type Group struct {
	Key   string                `json:"key"`
	Label string                `json:"label"`
	Count int                   `json:"count"`
	Means map[Dimension]float64 `json:"means"`
	// Counts is how many ratings each mean is taken over: a mean of one is a
	// rating, not a finding, and a client should say so.
	Counts map[Dimension]int `json:"counts"`
}

// Insights is what the log says works, grouped three ways.
type Insights struct {
	Rated       int     `json:"rated"`
	ByProduct   []Group `json:"by_product"`
	ByTemp      []Group `json:"by_temperature"`
	ByTimeOfDay []Group `json:"by_time_of_day"`
}

// Analyse groups the sessions' latest ratings by product, by temperature band
// and by the time of day they happened. Sessions that were undone are left
// out with their ratings; so are ratings of sessions the journal lacks.
func Analyse(ratings []Rating, sessions []journal.Event, name func(string) string) Insights {
	latest := Latest(ratings)
	undone := map[string]bool{}
	for _, e := range sessions {
		if e.Reverts != "" {
			undone[e.Reverts] = true
		}
	}
	acc := map[string]map[string]*sums{}
	add := func(by, key string, r Rating) {
		if acc[by] == nil {
			acc[by] = map[string]*sums{}
		}
		s := acc[by][key]
		if s == nil {
			s = &sums{total: map[Dimension]int{}, n: map[Dimension]int{}}
			acc[by][key] = s
		}
		s.count++
		for d, v := range r.Scores {
			s.total[d] += v
			s.n[d]++
		}
	}
	out := Insights{}
	for _, e := range sessions {
		r, ok := latest[e.Hash]
		if !ok || e.Type != journal.Sesh || undone[e.Hash] {
			continue
		}
		out.Rated++
		add("product", e.Product, r)
		if e.Temperature > 0 {
			add("temp", tempBand(e.Temperature), r)
		}
		add("time", timeOfDay(e.OccurredAt), r)
	}
	out.ByProduct = groups(acc["product"], name)
	out.ByTemp = groups(acc["temp"], func(k string) string { return k + " °C" })
	out.ByTimeOfDay = groups(acc["time"], func(k string) string { return k[2:] })
	return out
}

type sums struct {
	count int
	total map[Dimension]int
	n     map[Dimension]int
}

func groups(by map[string]*sums, label func(string) string) []Group {
	out := make([]Group, 0, len(by))
	for key, s := range by {
		g := Group{Key: key, Label: label(key), Count: s.count, Means: map[Dimension]float64{}, Counts: s.n}
		for d, t := range s.total {
			g.Means[d] = float64(int(float64(t)/float64(s.n[d])*100+0.5)) / 100
		}
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// tempBand is a temperature's ten-degree band: 185 is "180–189".
func tempBand(c int) string {
	lo := c / 10 * 10
	return fmt.Sprintf("%d–%d", lo, lo+9)
}

// timeOfDay is the part of the day a session happened in, prefixed so the
// parts sort in the order of the day.
func timeOfDay(t time.Time) string {
	switch h := t.Hour(); {
	case h < 6:
		return "1 night"
	case h < 12:
		return "2 morning"
	case h < 18:
		return "3 afternoon"
	default:
		return "4 evening"
	}
}
