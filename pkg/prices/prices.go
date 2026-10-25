// Package prices is what each fill cost: the price paid for a purchase,
// recorded beside the journal rather than in it.
//
// A purchase is never edited in place, and a price is not a transfer of grams,
// so it is not a journal entry either. It keeps the journal's disciplines the
// way the effect log does: append-only, written under a lock, hash-chained,
// each hash checked against the line as stored. A price names its purchase by
// the purchase's hash, and pricing a purchase again supersedes the earlier
// price, which stays in the log. Amounts are euro cents, the currency every
// prescription here has been paid in.
package prices

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
)

// ErrBrokenChain is returned when the log's hash chain does not verify.
var ErrBrokenChain = errors.New("price log hash chain is broken")

// Price is what was paid for one purchase. Field order is significant: the
// hash is taken over the JSON encoding.
type Price struct {
	Seq      int       `json:"seq"`
	Purchase string    `json:"purchase"`
	At       time.Time `json:"at"`
	Cents    int64     `json:"cents"`
	Note     string    `json:"note,omitempty"`
	Prev     string    `json:"prev"`
	Hash     string    `json:"hash"`
}

// Euros is the price in euros.
func (p Price) Euros() float64 { return float64(p.Cents) / 100 }

// Validate reports whether a price is well formed: a purchase named, and an
// amount paid.
func (p Price) Validate() error {
	if p.Purchase == "" {
		return errors.New("a price names the purchase it was paid for")
	}
	if p.Cents <= 0 {
		return fmt.Errorf("a price is more than nothing, got %s", Format(p.Cents))
	}
	return nil
}

// sum is the price's hash chained onto prev, taken with the hash blank.
func (p Price) sum(prev string) (string, error) {
	p.Prev, p.Hash = prev, ""
	payload, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(append([]byte(prev), payload...))
	return hex.EncodeToString(h[:]), nil
}

// seal makes a price what the log stores after the ones before it: dated,
// checked, numbered and chained.
func seal(p Price, before []Price) (Price, error) {
	if p.At.IsZero() {
		p.At = time.Now()
	}
	p.At = p.At.Truncate(time.Second)
	if err := p.Validate(); err != nil {
		return p, err
	}
	p.Seq, p.Prev = len(before)+1, ""
	if n := len(before); n > 0 {
		p.Prev = before[n-1].Hash
	}
	var err error
	p.Hash, err = p.sum(p.Prev)
	return p, err
}

// Chain seals prices one onto the next from the start of a log, as appending
// them to an empty one would: what a bundle's prices hash to once restored.
func Chain(prices []Price) ([]Price, error) {
	out := make([]Price, 0, len(prices))
	for _, p := range prices {
		sealed, err := seal(p, out)
		if err != nil {
			return nil, err
		}
		out = append(out, sealed)
	}
	return out, nil
}

// ParseEuros reads an amount of euros as typed — "120.50", "120,50 €",
// "€120" — into cents.
func ParseEuros(s string) (int64, error) {
	t := strings.TrimSpace(strings.NewReplacer("€", "", "EUR", "", "eur", "").Replace(s))
	t = strings.ReplaceAll(strings.TrimSpace(t), ",", ".")
	f, err := strconv.ParseFloat(t, 64)
	if err != nil || f <= 0 || math.IsInf(f, 0) {
		return 0, fmt.Errorf("%q is not an amount of euros", s)
	}
	cents := math.Round(f * 100)
	if math.Abs(f*100-cents) > 1e-6 {
		return 0, fmt.Errorf("%q is finer than a cent", s)
	}
	return int64(cents), nil
}

// Format writes cents as euros.
func Format(cents int64) string {
	return fmt.Sprintf("%.2f €", float64(cents)/100)
}

// Log is the price log stored at a path.
type Log struct {
	mu   sync.Mutex
	path string
}

// Open returns the log at path; it is created on the first append.
func Open(path string) *Log { return &Log{path: path} }

// Path returns the file the log is stored in.
func (l *Log) Path() string { return l.path }

// Append checks a price, chains it onto the log's tip and writes it, under the
// lock the journal takes. The stored price is returned.
func (l *Log) Append(p Price) (Price, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	release, err := journal.Lock(l.path)
	if err != nil {
		return Price{}, err
	}
	defer release()
	prices, _, err := l.read()
	if err != nil {
		return Price{}, err
	}
	if p, err = seal(p, prices); err != nil {
		return Price{}, err
	}
	line, err := json.Marshal(p)
	if err != nil {
		return Price{}, err
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return Price{}, err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return Price{}, err
	}
	return p, f.Sync()
}

// All returns every price, oldest first, superseded ones included.
func (l *Log) All() ([]Price, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	prices, _, err := l.read()
	return prices, err
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
		var p Price
		if err := json.Unmarshal(line, &p); err != nil {
			return fmt.Errorf("price log line %d is not valid JSON: %w", n+1, err)
		}
		stored := []byte(`,"hash":"` + p.Hash + `"}`)
		if p.Prev != prev || !bytes.HasSuffix(line, stored) {
			return fmt.Errorf("%w at price %d", ErrBrokenChain, p.Seq)
		}
		payload := append(bytes.Clone(line[:len(line)-len(stored)]), `,"hash":""}`...)
		sum := sha256.Sum256(append([]byte(prev), payload...))
		if hex.EncodeToString(sum[:]) != p.Hash {
			return fmt.Errorf("%w at price %d", ErrBrokenChain, p.Seq)
		}
		prev = p.Hash
	}
	return nil
}

// Rebase brings in another device's copy of the log the way the journal's
// Rebase does: shared prices kept, the other's taken as they are, and this
// log's own chained again on top, each pointed at the new hash of a purchase
// that moved. It returns how many of the other's it lacked.
func (l *Log) Rebase(onto []Price, moved map[string]string) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	release, err := journal.Lock(l.path)
	if err != nil {
		return 0, err
	}
	defer release()
	if err := whole(onto); err != nil {
		return 0, err
	}
	local, _, err := l.read()
	if err != nil {
		return 0, err
	}
	k := 0
	for k < len(local) && k < len(onto) && local[k].Hash == onto[k].Hash {
		k++
	}
	tail := local[k:]
	if len(onto) == k && !slices.ContainsFunc(tail, func(p Price) bool { _, ok := moved[p.Purchase]; return ok }) {
		return 0, nil
	}
	merged := slices.Clone(onto)
	for _, p := range tail {
		if to, ok := moved[p.Purchase]; ok {
			p.Purchase = to
		}
		if p, err = seal(p, merged); err != nil {
			return 0, err
		}
		merged = append(merged, p)
	}
	var buf bytes.Buffer
	for _, p := range merged {
		line, _ := json.Marshal(p) // a Price always encodes
		buf.Write(append(line, '\n'))
	}
	return len(onto) - k, journal.WriteFile(l.path, buf.Bytes())
}

// whole reports whether prices form a whole chain from the start.
func whole(prices []Price) error {
	prev := ""
	for i, p := range prices {
		sum, err := p.sum(prev)
		if err != nil {
			return err
		}
		if p.Seq != i+1 || p.Prev != prev || p.Hash != sum {
			return fmt.Errorf("%w at price %d of the other copy", ErrBrokenChain, i+1)
		}
		prev = p.Hash
	}
	return nil
}

// read loads the complete lines and the prices they hold; a line cut off
// before its newline is a write that never finished, and is left out.
func (l *Log) read() ([]Price, [][]byte, error) {
	data, err := os.ReadFile(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	data = data[:bytes.LastIndexByte(data, '\n')+1]
	var prices []Price
	var lines [][]byte
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := bytes.Clone(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var p Price
		if err := json.Unmarshal(line, &p); err != nil {
			return nil, nil, fmt.Errorf("price log line %d is not valid JSON: %w", len(lines)+1, err)
		}
		prices = append(prices, p)
		lines = append(lines, line)
	}
	return prices, lines, sc.Err()
}

// Latest returns each purchase's newest price: a purchase priced again costs
// what it was priced at last.
func Latest(prices []Price) map[string]Price {
	out := make(map[string]Price, len(prices))
	for _, p := range prices {
		out[p.Purchase] = p
	}
	return out
}

// Fill is what one prescription fill cost: its purchases, the grams of them
// that have a price, and what those cost.
type Fill struct {
	Seq       int
	Start     time.Time
	Dispensed float64
	Priced    float64
	Cents     int64
	Unpriced  int
}

// PerGram is the fill's cost in cents per gram, over the grams with a price.
func (f Fill) PerGram() float64 { return perGram(f.Cents, f.Priced) }

// Month is what was paid in one calendar month, by when it was dispensed.
type Month struct {
	Month string
	Cents int64
	Grams float64
}

// Costs is what the prescriptions cost: by fill, by month, and per session
// that helped — rated 4 or more overall, at its product's price per gram.
type Costs struct {
	Fills        []Fill
	Months       []Month
	Cents        int64
	Priced       float64
	Unpriced     int
	Helped       int
	HelpedCents  int64
	HelpedNoCost int
}

// PerGram is everything paid over every gram with a price.
func (c Costs) PerGram() float64 { return perGram(c.Cents, c.Priced) }

// PerHelped is what a session that helped cost, on average.
func (c Costs) PerHelped() float64 {
	if c.Helped == 0 {
		return 0
	}
	return float64(c.HelpedCents) / float64(c.Helped)
}

func perGram(cents int64, grams float64) float64 {
	if grams <= 0 {
		return 0
	}
	return float64(cents) / grams
}

// helps is the overall rating from which a session counts as one that helped.
const helps = 4

// Analyse is what the purchases cost, by the price each was last given. An
// undone purchase cost nothing; a purchase without a price is counted, not
// guessed at.
func Analyse(state *ledger.State, latest map[string]Price, ratings []effects.Rating) Costs {
	var out Costs
	byMonth := map[string]*Month{}
	byProduct := map[string][2]float64{} // cents, priced grams
	standing := map[string]bool{}
	for _, e := range state.Standing {
		standing[e.Hash] = true
	}
	for _, c := range state.Cycles {
		f := Fill{Seq: c.Seq, Start: c.Start}
		for _, e := range c.Events {
			if e.Type != journal.Purchase || !standing[e.Hash] {
				continue
			}
			f.Dispensed = ledger.Round(f.Dispensed + e.Grams)
			p, ok := latest[e.Hash]
			if !ok {
				f.Unpriced++
				continue
			}
			f.Priced = ledger.Round(f.Priced + e.Grams)
			f.Cents += p.Cents
			month := e.OccurredAt.Format("2006-01")
			m := byMonth[month]
			if m == nil {
				m = &Month{Month: month}
				byMonth[month] = m
			}
			m.Cents += p.Cents
			m.Grams = ledger.Round(m.Grams + e.Grams)
			pg := byProduct[e.Product]
			byProduct[e.Product] = [2]float64{pg[0] + float64(p.Cents), pg[1] + e.Grams}
		}
		out.Fills = append(out.Fills, f)
		out.Cents += f.Cents
		out.Priced = ledger.Round(out.Priced + f.Priced)
		out.Unpriced += f.Unpriced
	}
	for _, m := range byMonth {
		out.Months = append(out.Months, *m)
	}
	slices.SortFunc(out.Months, func(a, b Month) int { return strings.Compare(a.Month, b.Month) })
	helped(&out, state, byProduct, ratings)
	return out
}

// helped counts the sessions rated as having helped, and what their grams cost
// at their product's price per gram.
func helped(out *Costs, state *ledger.State, byProduct map[string][2]float64, ratings []effects.Rating) {
	rated := effects.Latest(ratings)
	var cents float64
	for _, e := range state.Standing {
		r, ok := rated[e.Hash]
		if e.Type != journal.Sesh || !ok || r.Scores[effects.Overall] < helps {
			continue
		}
		pg := byProduct[e.Product]
		if pg[1] <= 0 {
			out.HelpedNoCost++
			continue
		}
		out.Helped++
		cents += e.Grams * pg[0] / pg[1]
	}
	out.HelpedCents = int64(math.Round(cents))
}
