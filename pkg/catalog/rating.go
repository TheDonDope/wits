// Package catalog holds a strain's own rating here: what a product is thought
// of as a whole — overall, its taste and its aroma, whether it would be had
// again, and a line about it — apart from what any one session did, which the
// effect log keeps.
package catalog

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Again says whether a strain would be had again: on the next prescription,
// or not. Empty is not said.
type Again string

// The answers to "again?".
const (
	AgainYes Again = "yes"
	AgainNo  Again = "no"
)

// StrainRating is what a strain is thought of as a whole. Each score is 1
// (not at all) to 5 (very), and 0 when it was not given: a dimension not
// rated is left out, not rated low.
type StrainRating struct {
	Overall int       `yaml:"overall,omitempty"`
	Taste   int       `yaml:"taste,omitempty"`
	Aroma   int       `yaml:"aroma,omitempty"`
	Again   Again     `yaml:"again,omitempty"`
	Note    string    `yaml:"note,omitempty"`
	At      time.Time `yaml:"at"`
}

// ErrNothingRated is a strain rating with no score, no "again" and no note.
var ErrNothingRated = errors.New("a strain's rating says something: a score, whether you would have it again, or a line about it")

// Validate reports whether a rating is well formed: each score 1 to 5 or
// not given, "again" yes, no or not given, and something said.
func (r StrainRating) Validate() error {
	for _, s := range []struct {
		name  string
		score int
	}{{"overall", r.Overall}, {"taste", r.Taste}, {"aroma", r.Aroma}} {
		if s.score < 0 || s.score > 5 {
			return fmt.Errorf("%s is rated %d; a score is 1 to 5", s.name, s.score)
		}
	}
	if r.Again != "" && r.Again != AgainYes && r.Again != AgainNo {
		return fmt.Errorf("again is %q; it is yes or no, or left out", r.Again)
	}
	if r.Overall == 0 && r.Taste == 0 && r.Aroma == 0 && r.Again == "" && strings.TrimSpace(r.Note) == "" {
		return ErrNothingRated
	}
	return nil
}

// Summary writes a rating as a line: "overall 4, taste 5 · again".
func (r StrainRating) Summary() string {
	var parts []string
	for _, s := range []struct {
		name  string
		score int
	}{{"overall", r.Overall}, {"taste", r.Taste}, {"aroma", r.Aroma}} {
		if s.score > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", s.name, s.score))
		}
	}
	out := strings.Join(parts, ", ")
	switch r.Again {
	case AgainYes:
		out += " · again"
	case AgainNo:
		out += " · not again"
	}
	return strings.TrimPrefix(out, " · ")
}

// ParseRating reads scores as `wits describe --rating` takes them:
// "overall=4 taste=5", by spaces or commas.
func ParseRating(s string) (StrainRating, error) {
	var r StrainRating
	for _, pair := range strings.FieldsFunc(s, func(c rune) bool { return c == ' ' || c == ',' }) {
		name, value, ok := strings.Cut(pair, "=")
		var score int
		if _, err := fmt.Sscan(value, &score); !ok || err != nil {
			return r, fmt.Errorf("%q is not a score: write overall=4, taste=5 or aroma=3", pair)
		}
		switch strings.ToLower(name) {
		case "overall":
			r.Overall = score
		case "taste":
			r.Taste = score
		case "aroma", "smell":
			r.Aroma = score
		default:
			return r, fmt.Errorf("a strain is rated on overall, taste and aroma, not %q", name)
		}
	}
	return r, nil
}
