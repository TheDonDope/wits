package bundle

import (
	"bufio"
	"fmt"
	"io"
	"sort"

	"github.com/TheDonDope/wits-tui/pkg/catalog"
	"github.com/TheDonDope/wits-tui/pkg/journal"
)

// Contents is everything a bundle carries.
type Contents struct {
	Products *catalog.Catalog
	Devices  *catalog.Devices
	Events   []journal.Event
}

// Write encodes the contents as a bundle.
//
// Only what cannot be derived is written down. Sequence numbers follow from
// position, the account pair follows from the event type, and the hash chain is
// recomputed on restore, so none of them appear in the file. Timestamps and
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

	var prevOccurred, prevRecorded int64
	prevOffset := -1 << 30
	prevRecordedOffset := -1 << 30
	for _, e := range c.Events {
		code, ok := typeCodes[e.Type]
		if !ok {
			return fmt.Errorf("cannot bundle unknown event type %q", e.Type)
		}
		occurred := e.OccurredAt.Unix()
		recorded := e.RecordedAt.Unix()

		fmt.Fprintf(out, "%c%s %s %s",
			code,
			num(int64(products.index[e.Product])),
			num(occurred-prevOccurred),
			num(centigrams(e.Grams)),
		)
		// The recorded time is written relative to the previous event's recorded
		// time, not to this event's occurred time. Entries are typed in roughly
		// in order, so consecutive recorded times are close together even when a
		// whole history is backfilled years after the fact.
		if d := recorded - prevRecorded; d != 0 || prevRecorded == 0 {
			fmt.Fprintf(out, " r=%s", num(d))
		}
		if off := offsetOf(e.OccurredAt); off != prevOffset {
			fmt.Fprintf(out, " z=%s", num(int64(off)))
			prevOffset = off
		}
		// The recorded time is in whatever zone the entry was typed in, which is
		// not always the zone the event occurred in: a backdated entry keeps the
		// original day's offset while being recorded in today's.
		if off := offsetOf(e.RecordedAt); off != prevRecordedOffset {
			fmt.Fprintf(out, " zr=%s", num(int64(off)))
			prevRecordedOffset = off
		}
		if e.Device != "" {
			fmt.Fprintf(out, " d=%s", num(int64(devices.index[e.Device])))
		}
		if e.Temperature != 0 {
			fmt.Fprintf(out, " t=%s", num(int64(e.Temperature)))
		}
		if e.Note != "" {
			fmt.Fprintf(out, " n=%s", num(int64(notes.index[e.Note])))
		}
		if e.Reverts != "" {
			fmt.Fprintf(out, " v=%s", e.Reverts)
		}
		fmt.Fprint(out, "\n")

		prevOccurred, prevRecorded = occurred, recorded
	}
	return out.Flush()
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
	if p.Radiated {
		fmt.Fprint(out, " r=1")
	}
	if !p.AddedAt.IsZero() {
		fmt.Fprintf(out, " a=%s", num(p.AddedAt.Unix()))
	}
}

// writeDevice appends a device's details.
func writeDevice(out io.Writer, d *catalog.Device) {
	if d.Name != "" {
		fmt.Fprintf(out, " n=%s", escape(d.Name))
	}
	if d.Kind != "" {
		fmt.Fprintf(out, " k=%s", escape(d.Kind))
	}
	if d.MinTemp != 0 {
		fmt.Fprintf(out, " lo=%d", d.MinTemp)
	}
	if d.MaxTemp != 0 {
		fmt.Fprintf(out, " hi=%d", d.MaxTemp)
	}
	if d.DefaultTemp != 0 {
		fmt.Fprintf(out, " df=%d", d.DefaultTemp)
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
