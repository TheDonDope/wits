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
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/chain"
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

// Summary writes scores in the order the dimensions are asked: "overall 4,
// sleep 5".
func Summary(scores map[Dimension]int) string {
	var out []string
	for _, d := range Dimensions {
		if v, ok := scores[d]; ok {
			out = append(out, fmt.Sprintf("%s %d", d, v))
		}
	}
	return strings.Join(out, ", ")
}

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

// Log is the effect log: a chain of ratings (pkg/chain).
type Log = chain.Log[Rating, *Rating]

// Open returns the effect log at path; it is created on the first append.
func Open(path string) *Log {
	return chain.Open[Rating](path, "effect log", "rating", ErrBrokenChain)
}

// Chain seals ratings one onto the next from the start of a log, as appending
// them to an empty one would.
func Chain(ratings []Rating) ([]Rating, error) { return chain.Chain[Rating](ratings) }

// Link is the rating's place in the log.
func (r *Rating) Link() chain.Link {
	return chain.Link{Seq: r.Seq, At: r.At, Prev: r.Prev, Hash: r.Hash}
}

// SetLink places the rating in the log.
func (r *Rating) SetLink(l chain.Link) { r.Seq, r.At, r.Prev, r.Hash = l.Seq, l.At, l.Prev, l.Hash }

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
