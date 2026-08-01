package bundle

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/journal"
)

// Magic identifies a bundle, and Version is the format it is written in.
const (
	Magic   = "wits-bundle"
	Version = 1
)

// separator ends the header and begins the events.
const separator = "--"

// typeCodes abbreviate the event types to a single character. The codes are
// part of the format, so they may be added to but never reused for something
// else.
var typeCodes = map[journal.Type]byte{
	journal.Purchase:   'b',
	journal.Grind:      'g',
	journal.Sesh:       's',
	journal.AVBCollect: 'c',
	journal.AVBUse:     'u',
	journal.Adjust:     'a',
}

// typeOf reverses typeCodes.
var typeOf = func() map[byte]journal.Type {
	m := make(map[byte]journal.Type, len(typeCodes))
	for t, c := range typeCodes {
		m[c] = t
	}
	return m
}()

// num encodes an integer.
//
// Base 36 would shave a character off most of these, but a bundle is meant to
// be readable with nothing but a text editor, and "86400" says a day where
// "1lvo" says nothing at all. The saving is a few percent; compression takes it
// back if it matters.
func num(n int64) string { return strconv.FormatInt(n, 10) }

// parseNum decodes an integer.
func parseNum(s string) (int64, error) { return strconv.ParseInt(s, 10, 64) }

// centigrams converts grams to the hundredths the scale actually reads, so that
// amounts are integers in the file and no floating point error can creep in
// across a round trip.
func centigrams(grams float64) int64 { return int64(grams*100 + 0.5) }

// grams converts centigrams back.
func grams(cg int64) float64 { return float64(cg) / 100 }

// zone returns the location for an offset in minutes east of UTC. Zero is
// returned as time.UTC rather than a fixed zone, because the two format
// differently: UTC renders as "Z" and a fixed zone as "+00:00", and the hash is
// taken over that rendering.
func zone(offsetMinutes int) *time.Location {
	if offsetMinutes == 0 {
		return time.UTC
	}
	return time.FixedZone("", offsetMinutes*60)
}

// offsetOf returns a time's offset from UTC in minutes.
func offsetOf(t time.Time) int {
	_, seconds := t.Zone()
	return seconds / 60
}

// escape makes a value safe to put in a space-separated field.
func escape(s string) string {
	r := strings.NewReplacer("\\", `\\`, " ", `\s`, "\n", `\n`)
	return r.Replace(s)
}

// unescape reverses escape.
func unescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 's':
			b.WriteByte(' ')
		case 'n':
			b.WriteByte('\n')
		case '\\':
			b.WriteByte('\\')
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// field splits a "key=value" attribute.
func field(s string) (key, value string) {
	if i := strings.IndexByte(s, '='); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

// errorf builds a parse error naming the line it came from.
func errorf(line int, format string, args ...any) error {
	return fmt.Errorf("bundle line %d: %s", line, fmt.Sprintf(format, args...))
}
