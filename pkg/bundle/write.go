package bundle

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/prices"
	"github.com/TheDonDope/wits/pkg/seal"
)

// Contents is everything a bundle carries.
type Contents struct {
	Products *catalog.Catalog
	Devices  *catalog.Devices
	Events   []journal.Event
	// Ratings is the effect log, superseded ratings included.
	Ratings []effects.Rating
	// Prices is the price log, superseded prices included.
	Prices []prices.Price
	// Seals are the seals of the journal's head, with their timestamps: a
	// timestamp cannot be made again later, so it travels with the record.
	Seals []seal.File
}

// Write encodes the contents as a bundle.
//
// Only what cannot be derived is written down. Sequence numbers follow from
// position, the account pair follows from the event type — except where it
// does not, an adjustment's, which is written — and the hash chain is
// recomputed on restore, so none of the rest appear in the file. Timestamps and
// amounts are stored as deltas against the previous event, which is what makes
// a run of daily entries cost a handful of bytes each.
func Write(w io.Writer, c Contents) error {
	out := bufio.NewWriter(w)

	fmt.Fprintf(out, "%s %d\n", Magic, Version)

	products := productIndex(c.Events, c.Products)
	for i, p := range products.slugs {
		fmt.Fprintf(out, "P%s %s", num(int64(i)), escape(p))
		if meta := products.byslug[p]; meta != nil {
			writeProduct(out, meta)
		}
		fmt.Fprint(out, "\n")
	}
	devices := deviceIndex(c.Events, c.Devices)
	for i, d := range devices.slugs {
		fmt.Fprintf(out, "D%s %s", num(int64(i)), escape(d))
		if meta := devices.byslug[d]; meta != nil {
			writeDevice(out, meta)
		}
		fmt.Fprint(out, "\n")
	}
	notes := noteIndex(c.Events)
	for i, n := range notes.notes {
		fmt.Fprintf(out, "N%s %s\n", num(int64(i)), escape(n))
	}
	fmt.Fprintln(out, separator)

	cur := cursor{offset: -1 << 30, recordedOffset: -1 << 30}
	for _, e := range c.Events {
		if err := cur.write(out, e, products.index, devices.index, notes.index); err != nil {
			return err
		}
	}
	if len(c.Ratings) > 0 || len(c.Prices) > 0 || len(c.Seals) > 0 {
		fmt.Fprintln(out, ratingsSeparator)
	}
	writeRatings(out, c.Ratings)
	writePrices(out, c.Prices)
	writeSeals(out, c.Seals)
	return out.Flush()
}

// cursor is what the previous event line left behind, which the next one is
// written relative to.
type cursor struct {
	occurred, recorded     int64
	offset, recordedOffset int
}

// write encodes one event as a line, relative to the cursor, and moves it on.
func (c *cursor) write(out io.Writer, e journal.Event, products, devices, notes map[string]int) error {
	code, ok := typeCodes[e.Type]
	if !ok {
		return fmt.Errorf("cannot bundle unknown event type %q", e.Type)
	}
	occurred := e.OccurredAt.Unix()
	recorded := e.RecordedAt.Unix()

	fmt.Fprintf(out, "%c%s %s %s", code, num(int64(products[e.Product])),
		num(occurred-c.occurred), num(centigrams(e.Grams)))
	// The recorded time is written relative to the previous event's recorded
	// time, not to this event's occurred time. Entries are typed in roughly
	// in order, so consecutive recorded times are close together even when a
	// whole history is backfilled years after the fact.
	if d := recorded - c.recorded; d != 0 || c.recorded == 0 {
		fmt.Fprintf(out, " r=%s", num(d))
	}
	if off := offsetOf(e.OccurredAt); off != c.offset {
		fmt.Fprintf(out, " z=%s", num(int64(off)))
		c.offset = off
	}
	// The recorded time is in whatever zone the entry was typed in, which is
	// not always the zone the event occurred in: a backdated entry keeps the
	// original day's offset while being recorded in today's.
	if off := offsetOf(e.RecordedAt); off != c.recordedOffset {
		fmt.Fprintf(out, " zr=%s", num(int64(off)))
		c.recordedOffset = off
	}
	writeRefs(out, e, devices, notes)
	fmt.Fprint(out, "\n")
	c.occurred, c.recorded = occurred, recorded
	return nil
}

// writeRefs appends the attributes an event has only sometimes: its device,
// temperature, note, the entry it undoes, and its accounts when they are not
// the ones its type implies — an adjustment's never are. Format 1 left them
// out, taking every event's accounts to follow from its type, and every undo
// and weighing came back from a restore moving nothing.
func writeRefs(out io.Writer, e journal.Event, devices, notes map[string]int) {
	if e.Device != "" {
		fmt.Fprintf(out, " d=%s", num(int64(devices[e.Device])))
	}
	if e.Temperature != 0 {
		fmt.Fprintf(out, " t=%s", num(int64(e.Temperature)))
	}
	// Format 8: a session's mode, the device that heated it, and its setting.
	if e.Mode != "" {
		fmt.Fprintf(out, " m=%s", escape(e.Mode))
	}
	if e.Heater != "" {
		fmt.Fprintf(out, " h=%s", num(int64(devices[e.Heater])))
	}
	if e.Setting != "" {
		fmt.Fprintf(out, " s=%s", escape(e.Setting))
	}
	if e.Timer != 0 {
		fmt.Fprintf(out, " tm=%s", num(int64(e.Timer)))
	}
	if e.Note != "" {
		fmt.Fprintf(out, " n=%s", num(int64(notes[e.Note])))
	}
	if e.Reverts != "" {
		fmt.Fprintf(out, " v=%s", e.Reverts)
	}
	if from, to, _ := journal.Flow(e.Type); e.From != from || e.To != to {
		fmt.Fprintf(out, " f=%s>%s", e.From, e.To)
	}
}

// writeProduct appends a product's details as key=value attributes. Empty
// fields are left out entirely.
func writeProduct(out io.Writer, p *catalog.Product) {
	for _, f := range []struct {
		key   string
		value string
	}{
		{"n", p.Name},
		{"m", p.Manufacturer},
		{"c", p.Cultivar},
		{"o", p.Country},
	} {
		if f.value != "" {
			fmt.Fprintf(out, " %s=%s", f.key, escape(f.value))
		}
	}
	if p.THC != 0 {
		fmt.Fprintf(out, " thc=%s", trimFloat(p.THC))
	}
	if p.CBD != 0 {
		fmt.Fprintf(out, " cbd=%s", trimFloat(p.CBD))
	}
	if p.Genetic != 0 {
		fmt.Fprintf(out, " g=%d", int(p.Genetic))
	}
	if len(p.Terpenes) > 0 {
		fmt.Fprintf(out, " tp=%s", escape(strings.Join(p.Terpenes, ",")))
	}
	if p.Radiated {
		fmt.Fprint(out, " r=1")
	}
	if !p.AddedAt.IsZero() {
		fmt.Fprintf(out, " a=%s", num(p.AddedAt.Unix()))
	}
}

// modes writes a device's modes, each its name, way of heating, temperature
// and timer apart by @, the empty ones at the end left off:
// "On demand@convection@165|Session@hybrid@185@420".
func modes(ms []catalog.Mode) string {
	parts := make([]string, len(ms))
	for i, m := range ms {
		fields := []string{m.Name, m.Signature, "", ""}
		if m.Temp != 0 {
			fields[2] = strconv.Itoa(m.Temp)
		}
		if m.Timer != 0 {
			fields[3] = strconv.Itoa(m.Timer)
		}
		for len(fields) > 1 && fields[len(fields)-1] == "" {
			fields = fields[:len(fields)-1]
		}
		parts[i] = strings.Join(fields, "@")
	}
	return strings.Join(parts, "|")
}

// writeDevice appends a device's details. Format 8 added everything after
// the temperatures; modes are joined by |, a mode's fields by @.
func writeDevice(out io.Writer, d *catalog.Device) {
	text := []struct{ key, value string }{
		{"n", d.Name}, {"k", d.Kind}, {"c", d.Category}, {"mk", d.Maker}, {"md", d.Model},
		{"co", d.Country}, {"pw", d.Power}, {"sg", d.Signature}, {"us", d.Use},
		{"mo", modes(d.Modes)}, {"sh", d.Shop}, {"bo", d.BoughtOn}, {"ln", d.Link}, {"nt", d.Notes},
	}
	for _, t := range text {
		if t.value != "" {
			fmt.Fprintf(out, " %s=%s", t.key, escape(t.value))
		}
	}
	numbers := []struct {
		key   string
		value int64
	}{{"lo", int64(d.MinTemp)}, {"hi", int64(d.MaxTemp)}, {"df", int64(d.DefaultTemp)}, {"pr", d.Cents}, {"ao", int64(d.AutoOff)}}
	for _, n := range numbers {
		if n.value != 0 {
			fmt.Fprintf(out, " %s=%d", n.key, n.value)
		}
	}
	if d.Water {
		fmt.Fprint(out, " w=1")
	}
	if d.Retired {
		fmt.Fprint(out, " rt=1")
	}
}

// trimFloat renders a percentage without trailing zeroes.
func trimFloat(f float64) string {
	return fmt.Sprintf("%g", f)
}

// index maps slugs to the small integers the event lines refer to them by.
type index struct {
	slugs  []string
	index  map[string]int
	byslug map[string]*catalog.Product
}

// deviceIdx is the device equivalent of index.
type deviceIdx struct {
	slugs  []string
	index  map[string]int
	byslug map[string]*catalog.Device
}

// productIndex orders products by how often they appear, so the ones referred
// to most get the shortest identifiers.
func productIndex(events []journal.Event, c *catalog.Catalog) index {
	counts := map[string]int{}
	for _, e := range events {
		if e.Product != "" {
			counts[e.Product]++
		}
	}
	if c != nil {
		for _, p := range c.Products {
			if _, ok := counts[p.Slug]; !ok {
				counts[p.Slug] = 0
			}
		}
	}
	idx := index{index: map[string]int{}, byslug: map[string]*catalog.Product{}}
	for slug := range counts {
		idx.slugs = append(idx.slugs, slug)
	}
	sort.Slice(idx.slugs, func(i, j int) bool {
		if counts[idx.slugs[i]] != counts[idx.slugs[j]] {
			return counts[idx.slugs[i]] > counts[idx.slugs[j]]
		}
		return idx.slugs[i] < idx.slugs[j]
	})
	for i, slug := range idx.slugs {
		idx.index[slug] = i
	}
	if c != nil {
		for _, p := range c.Products {
			idx.byslug[p.Slug] = p
		}
	}
	return idx
}

// deviceIndex does for devices what productIndex does for products.
func deviceIndex(events []journal.Event, d *catalog.Devices) deviceIdx {
	counts := map[string]int{}
	for _, e := range events {
		if e.Device != "" {
			counts[e.Device]++
		}
		if e.Heater != "" {
			counts[e.Heater]++
		}
	}
	if d != nil {
		for _, dev := range d.Devices {
			if _, ok := counts[dev.Slug]; !ok {
				counts[dev.Slug] = 0
			}
		}
	}
	idx := deviceIdx{index: map[string]int{}, byslug: map[string]*catalog.Device{}}
	for slug := range counts {
		idx.slugs = append(idx.slugs, slug)
	}
	sort.Slice(idx.slugs, func(i, j int) bool {
		if counts[idx.slugs[i]] != counts[idx.slugs[j]] {
			return counts[idx.slugs[i]] > counts[idx.slugs[j]]
		}
		return idx.slugs[i] < idx.slugs[j]
	})
	for i, slug := range idx.slugs {
		idx.index[slug] = i
	}
	if d != nil {
		for _, dev := range d.Devices {
			idx.byslug[dev.Slug] = dev
		}
	}
	return idx
}

// noteIndex collects the distinct notes, most frequent first. Notes repeat
// heavily — a backfilled history tags every event with where it came from — so
// naming each one once and referring to it by number is worth the header line.
type notesIdx struct {
	notes []string
	index map[string]int
}

func noteIndex(events []journal.Event) notesIdx {
	counts := map[string]int{}
	for _, e := range events {
		if e.Note != "" {
			counts[e.Note]++
		}
	}
	idx := notesIdx{index: map[string]int{}}
	for n := range counts {
		idx.notes = append(idx.notes, n)
	}
	sort.Slice(idx.notes, func(i, j int) bool {
		if counts[idx.notes[i]] != counts[idx.notes[j]] {
			return counts[idx.notes[i]] > counts[idx.notes[j]]
		}
		return idx.notes[i] < idx.notes[j]
	})
	for i, n := range idx.notes {
		idx.index[n] = i
	}
	return idx
}

// writeRatings appends the effect log after the events, behind its own
// separator: a line a rating, the session it rates by hash, the moment it was
// rated, its scores, and its note. Format 3 added the section; a bundle without
// ratings writes none, and reads in any build that knows the format.
func writeRatings(out io.Writer, ratings []effects.Rating) {
	for _, r := range ratings {
		scores := make([]string, 0, len(r.Scores))
		for _, d := range effects.Dimensions {
			if v, ok := r.Scores[d]; ok {
				scores = append(scores, fmt.Sprintf("%s:%d", d, v))
			}
		}
		fmt.Fprintf(out, "R %s %s %s", r.Session, r.At.Format(time.RFC3339), strings.Join(scores, ","))
		if r.Note != "" {
			fmt.Fprintf(out, " n=%s", escape(r.Note))
		}
		fmt.Fprint(out, "\n")
	}
}

// writePrices appends the price log after the ratings, in the same section: a
// line a price, the purchase by hash, the moment it was given, its cents, and
// its note. Format 4 added them; format 7 the pharmacy (s=) and a price not to
// be had (u=1), both written only when there.
func writePrices(out io.Writer, paid []prices.Price) {
	for _, p := range paid {
		fmt.Fprintf(out, "C %s %s %d", p.Purchase, p.At.Format(time.RFC3339), p.Cents)
		if p.Unknown {
			fmt.Fprint(out, " u=1")
		}
		if p.Seller != "" {
			fmt.Fprintf(out, " s=%s", escape(p.Seller))
		}
		if p.Note != "" {
			fmt.Fprintf(out, " n=%s", escape(p.Note))
		}
		fmt.Fprint(out, "\n")
	}
}

// writeSeals appends the seals, a line each: the name, the seal's text and,
// when it has one, its timestamp proof, both in base64 — a seal is lines of
// its own, and a proof is bytes. Format 5 added them.
func writeSeals(out io.Writer, seals []seal.File) {
	for _, f := range seals {
		fmt.Fprintf(out, "K %s %s", f.Name, base64.StdEncoding.EncodeToString([]byte(f.Text)))
		if len(f.OTS) > 0 {
			fmt.Fprintf(out, " %s", base64.StdEncoding.EncodeToString(f.OTS))
		}
		fmt.Fprint(out, "\n")
	}
}
