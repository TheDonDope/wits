package bundle

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	cannabis "github.com/TheDonDope/wits/pkg/cannabis"
	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/prices"
	"github.com/TheDonDope/wits/pkg/seal"
)

// can converts a stored integer back to a genetic type.
func can(g int) cannabis.GeneticType { return cannabis.GeneticType(g) }

// Read decodes a bundle.
//
// The events come back without their sequence numbers or hashes: those belong
// to a journal, and are assigned when the events are appended to one. Restoring
// into an empty journal in this order reproduces the chain exactly.
func Read(r io.Reader) (*Contents, error) {
	rd := &reader{sc: bufio.NewScanner(r)}
	rd.sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	head, ok := rd.next()
	if !ok {
		return nil, fmt.Errorf("bundle is empty")
	}
	version, err := checkHeader(head, rd.line)
	if err != nil {
		return nil, err
	}
	c, refs, err := rd.catalogs()
	if err != nil {
		return nil, err
	}
	if c.Events, err = rd.events(refs); err != nil {
		return nil, err
	}
	if c.Ratings, c.Prices, c.Seals, err = rd.logs(); err != nil {
		return nil, err
	}
	if version < 2 {
		if err := recoverFlows(c.Events); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// reader walks a bundle line by line, counting lines for its errors.
type reader struct {
	sc   *bufio.Scanner
	line int
}

// next returns the next line, or false at the end.
func (rd *reader) next() (string, bool) {
	if !rd.sc.Scan() {
		return "", false
	}
	rd.line++
	return rd.sc.Text(), true
}

// refs are the header's tables, which event lines refer to by position.
type refs struct {
	products, devices, notes []string
}

// catalogs reads the header up to the separator: the products, the devices
// and the notes.
func (rd *reader) catalogs() (*Contents, refs, error) {
	c := &Contents{Products: &catalog.Catalog{}, Devices: &catalog.Devices{}}
	var t refs
	for {
		text, ok := rd.next()
		if !ok {
			return nil, t, fmt.Errorf("bundle ended before its events began")
		}
		if text == separator {
			return c, t, nil
		}
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		if err := t.entry(c, text, rd.line); err != nil {
			return nil, t, err
		}
	}
}

// entry reads one header line into the contents and the tables.
func (t *refs) entry(c *Contents, text string, line int) error {
	switch text[0] {
	case 'P':
		slug, product, err := readProduct(text, line)
		if err != nil {
			return err
		}
		t.products = append(t.products, slug)
		c.Products.Products = append(c.Products.Products, product)
	case 'D':
		slug, device, err := readDevice(text, line)
		if err != nil {
			return err
		}
		t.devices = append(t.devices, slug)
		c.Devices.Devices = append(c.Devices.Devices, device)
	case 'N':
		parts := strings.SplitN(text, " ", 2)
		if len(parts) != 2 {
			return errorf(line, "note entry needs a value")
		}
		t.notes = append(t.notes, unescape(parts[1]))
	default:
		return errorf(line, "unknown header entry %q", text)
	}
	return nil
}

// events reads the event lines after the separator.
func (rd *reader) events(t refs) ([]journal.Event, error) {
	var events []journal.Event
	var st state
	for {
		text, ok := rd.next()
		if !ok {
			break
		}
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		if text == ratingsSeparator {
			break
		}
		e, next, err := readEvent(text, rd.line, t, st)
		if err != nil {
			return nil, err
		}
		st = next
		events = append(events, e)
	}
	return events, rd.sc.Err()
}

// logs reads what follows the events, if the bundle carries anything there:
// the effect log's ratings, and the price log's prices.
func (rd *reader) logs() ([]effects.Rating, []prices.Price, []seal.File, error) {
	var ratings []effects.Rating
	var paid []prices.Price
	var seals []seal.File
	for {
		text, ok := rd.next()
		if !ok {
			return ratings, paid, seals, rd.sc.Err()
		}
		var err error
		switch {
		case text == "" || strings.HasPrefix(text, "#"):
		case strings.HasPrefix(text, "C "):
			var p prices.Price
			p, err = readPrice(text, rd.line)
			paid = append(paid, p)
		case strings.HasPrefix(text, "K "):
			var f seal.File
			f, err = readSeal(text, rd.line)
			seals = append(seals, f)
		default:
			var r effects.Rating
			r, err = readRating(text, rd.line)
			ratings = append(ratings, r)
		}
		if err != nil {
			return nil, nil, nil, err
		}
	}
}

// readSeal decodes one seal line: K <name> <base64 seal> [<base64 proof>].
func readSeal(text string, line int) (seal.File, error) {
	parts := strings.Fields(text)
	if len(parts) < 3 || len(parts) > 4 {
		return seal.File{}, errorf(line, "a seal needs a name and its text")
	}
	body, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return seal.File{}, errorf(line, "unreadable seal %q", parts[1])
	}
	f := seal.File{Name: parts[1], Text: string(body)}
	if len(parts) == 4 {
		if f.OTS, err = base64.StdEncoding.DecodeString(parts[3]); err != nil {
			return seal.File{}, errorf(line, "unreadable timestamp of seal %q", parts[1])
		}
	}
	return f, nil
}

// readPrice decodes one price line: C <purchase> <at> <cents> [u=1] [s=<seller>] [n=<note>].
func readPrice(text string, line int) (prices.Price, error) {
	parts := strings.Fields(text)
	if len(parts) < 4 {
		return prices.Price{}, errorf(line, "a price needs a purchase, a time and its cents")
	}
	at, err := time.Parse(time.RFC3339, parts[2])
	if err != nil {
		return prices.Price{}, errorf(line, "unreadable price time %q", parts[2])
	}
	cents, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return prices.Price{}, errorf(line, "unreadable cents %q", parts[3])
	}
	p := prices.Price{Purchase: parts[1], At: at, Cents: cents}
	for _, attr := range parts[4:] {
		switch key, value := field(attr); key {
		case "n":
			p.Note = unescape(value)
		case "s":
			p.Seller = unescape(value)
		case "u":
			p.Unknown = value == "1"
		default:
			return prices.Price{}, errorf(line, "unknown price attribute %q", key)
		}
	}
	return p, nil
}

// readRating decodes one rating line: R <session> <at> <dim:score,…> [n=<note>].
func readRating(text string, line int) (effects.Rating, error) {
	parts := strings.Fields(text)
	if len(parts) < 4 || parts[0] != "R" {
		return effects.Rating{}, errorf(line, "a rating needs a session, a time and its scores")
	}
	at, err := time.Parse(time.RFC3339, parts[2])
	if err != nil {
		return effects.Rating{}, errorf(line, "unreadable rating time %q", parts[2])
	}
	r := effects.Rating{Session: parts[1], At: at, Scores: map[effects.Dimension]int{}}
	for _, s := range strings.Split(parts[3], ",") {
		d, v, ok := strings.Cut(s, ":")
		n, err := strconv.Atoi(v)
		if !ok || err != nil {
			return effects.Rating{}, errorf(line, "unreadable score %q", s)
		}
		r.Scores[effects.Dimension(d)] = n
	}
	for _, attr := range parts[4:] {
		if key, value := field(attr); key == "n" {
			r.Note = unescape(value)
		} else {
			return effects.Rating{}, errorf(line, "unknown rating attribute %q", key)
		}
	}
	return r, nil
}

// checkHeader validates the magic line and returns the format version.
func checkHeader(text string, line int) (int, error) {
	parts := strings.Fields(text)
	if len(parts) != 2 || parts[0] != Magic {
		return 0, errorf(line, "not a wits bundle")
	}
	version, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, errorf(line, "unreadable format version %q", parts[1])
	}
	if version > Version {
		return 0, errorf(line, "bundle is format version %d, this build understands %d", version, Version)
	}
	return version, nil
}

// state is what one event line resolves its deltas against, and what the next
// line will resolve against in turn.
type state struct {
	occurred       int64
	recorded       int64
	offset         int
	recordedOffset int
}

// readEvent decodes one event line against the running decode state.
func readEvent(text string, line int, t refs, in state) (journal.Event, state, error) {
	var e journal.Event
	out := in

	parts := strings.Fields(text)
	if len(parts) < 3 {
		return e, out, errorf(line, "event needs a type, a time and an amount")
	}
	typ, ok := typeOf[parts[0][0]]
	if !ok {
		return e, out, errorf(line, "unknown event type %q", string(parts[0][0]))
	}
	e.Type = typ
	pi, err := parseNum(parts[0][1:])
	if err != nil || pi < 0 || int(pi) >= len(t.products) {
		return e, out, errorf(line, "event refers to product %q, which the header does not define", parts[0][1:])
	}
	e.Product = t.products[pi]
	delta, err := parseNum(parts[1])
	if err != nil {
		return e, out, errorf(line, "unreadable time delta %q", parts[1])
	}
	out.occurred = in.occurred + delta
	cg, err := parseNum(parts[2])
	if err != nil {
		return e, out, errorf(line, "unreadable amount %q", parts[2])
	}
	e.Grams = grams(cg)

	// Without an r attribute the entry was recorded at the same instant as the
	// one before it, which is what a run of backfilled events looks like.
	out.recorded = in.recorded
	for _, attr := range parts[3:] {
		key, value := field(attr)
		decode, ok := eventAttrs[key]
		if !ok {
			return e, out, errorf(line, "unknown attribute %q", key)
		}
		if err := decode(&e, &out, in, value, t); err != nil {
			return e, out, errorf(line, "%v", err)
		}
	}

	e.OccurredAt = time.Unix(out.occurred, 0).In(zone(out.offset))
	e.RecordedAt = time.Unix(out.recorded, 0).In(zone(out.recordedOffset))
	if e.From == "" && e.To == "" {
		e.From, e.To, _ = journal.Flow(e.Type)
	}
	return e, out, nil
}

// eventAttrs decode an event line's optional key=value attributes, each into
// the event or the running state. A table rather than a switch: each one is
// a line to read, not a branch to follow.
var eventAttrs = map[string]func(e *journal.Event, out *state, in state, value string, t refs) error{
	"r": func(_ *journal.Event, out *state, in state, v string, _ refs) error {
		d, err := parseNum(v)
		if err != nil {
			return fmt.Errorf("unreadable recorded delta %q", v)
		}
		out.recorded = in.recorded + d
		return nil
	},
	"z": func(_ *journal.Event, out *state, _ state, v string, _ refs) error {
		off, err := parseNum(v)
		if err != nil {
			return fmt.Errorf("unreadable zone offset %q", v)
		}
		out.offset = int(off)
		return nil
	},
	"zr": func(_ *journal.Event, out *state, _ state, v string, _ refs) error {
		off, err := parseNum(v)
		if err != nil {
			return fmt.Errorf("unreadable recorded zone offset %q", v)
		}
		out.recordedOffset = int(off)
		return nil
	},
	"d": func(e *journal.Event, _ *state, _ state, v string, t refs) error {
		i, err := parseNum(v)
		if err != nil || i < 0 || int(i) >= len(t.devices) {
			return fmt.Errorf("event refers to device %q, which the header does not define", v)
		}
		e.Device = t.devices[i]
		return nil
	},
	"m": func(e *journal.Event, _ *state, _ state, v string, _ refs) error {
		e.Mode = unescape(v)
		return nil
	},
	"h": func(e *journal.Event, _ *state, _ state, v string, t refs) error {
		i, err := parseNum(v)
		if err != nil || i < 0 || int(i) >= len(t.devices) {
			return fmt.Errorf("event refers to heater %q, which the header does not define", v)
		}
		e.Heater = t.devices[i]
		return nil
	},
	"s": func(e *journal.Event, _ *state, _ state, v string, _ refs) error {
		e.Setting = unescape(v)
		return nil
	},
	"tm": func(e *journal.Event, _ *state, _ state, v string, _ refs) error {
		secs, err := parseNum(v)
		if err != nil {
			return fmt.Errorf("unreadable timer %q", v)
		}
		e.Timer = int(secs)
		return nil
	},
	"t": func(e *journal.Event, _ *state, _ state, v string, _ refs) error {
		temp, err := parseNum(v)
		if err != nil {
			return fmt.Errorf("unreadable temperature %q", v)
		}
		e.Temperature = int(temp)
		return nil
	},
	"n": func(e *journal.Event, _ *state, _ state, v string, t refs) error {
		i, err := parseNum(v)
		if err != nil || i < 0 || int(i) >= len(t.notes) {
			return fmt.Errorf("event refers to note %q, which the header does not define", v)
		}
		e.Note = t.notes[i]
		return nil
	},
	"v": func(e *journal.Event, _ *state, _ state, v string, _ refs) error {
		e.Reverts = v
		return nil
	},
	"f": func(e *journal.Event, _ *state, _ state, v string, _ refs) error {
		from, to, ok := strings.Cut(v, ">")
		if !ok || !accounts[journal.Account(from)] || !accounts[journal.Account(to)] {
			return fmt.Errorf("unreadable accounts %q", v)
		}
		e.From, e.To = journal.Account(from), journal.Account(to)
		return nil
	},
}

// accounts are the account names an f attribute may use.
var accounts = map[journal.Account]bool{
	journal.External: true, journal.Storage: true, journal.Stash: true,
	journal.Consumed: true, journal.AVB: true,
}

// recoverFlows gives back the accounts of the adjustments in a format 1
// bundle, which did not write them: it took every entry's accounts to follow
// from its type, and an adjustment's do not — an undo moves grams back the
// way they came, a weighing in or out of the account weighed. Restored as
// external to external, they moved nothing, and the repository came back with
// the wrong balances and a journal that was not the original.
//
// An undo names the entry it undoes by hash, so the chain is rebuilt as the
// events are read to find it, and its accounts are that entry's, reversed. A
// weighing or a clean-up with the note wits wrote for it says its account and
// direction. Any other adjustment cannot be restored faithfully, and the
// restore is refused rather than made wrong.
func recoverFlows(events []journal.Event) error {
	byHash := map[string]journal.Event{}
	prev := ""
	for i := range events {
		e := &events[i]
		if e.Type == journal.Adjust {
			from, to, err := recoveredFlow(*e, byHash)
			if err != nil {
				return fmt.Errorf("event %d, an adjustment of %.2f g of %s on %s: %w",
					i+1, e.Grams, e.Product, e.OccurredAt.Format(time.DateOnly), err)
			}
			e.From, e.To = from, to
		}
		sealed := *e
		sealed.Seq, sealed.Prev = i+1, prev
		hash, err := journal.Hash(sealed, prev)
		if err != nil {
			return err
		}
		byHash[hash] = sealed
		prev = hash
	}
	return nil
}

// reconciledNote is the note a weighing writes when none is given.
var reconciledNote = regexp.MustCompile(`^reconciled (storage|the stash|the AVB jar): ([0-9.]+) g weighed, ([0-9.]+) g expected$`)

// cleanedNote is the note the storage screen's history clean-up writes; it
// always empties a stash.
const cleanedNote = "clean history: consumed at some point"

// recoveredFlow works out one format 1 adjustment's accounts; see recoverFlows.
func recoveredFlow(e journal.Event, byHash map[string]journal.Event) (journal.Account, journal.Account, error) {
	if e.Reverts != "" {
		original, ok := byHash[e.Reverts]
		if !ok {
			return "", "", fmt.Errorf("it undoes %.7s, which comes nowhere before it", e.Reverts)
		}
		return original.To, original.From, nil
	}
	if e.Note == cleanedNote {
		return journal.Stash, journal.External, nil
	}
	m := reconciledNote.FindStringSubmatch(e.Note)
	if m == nil {
		return "", "", fmt.Errorf("format 1 bundles did not write an adjustment's accounts, and its note does not say them")
	}
	account := map[string]journal.Account{"storage": journal.Storage, "the stash": journal.Stash, "the AVB jar": journal.AVB}[m[1]]
	weighed, _ := strconv.ParseFloat(m[2], 64)
	expected, _ := strconv.ParseFloat(m[3], 64)
	if weighed < expected {
		return account, journal.External, nil
	}
	return journal.External, account, nil
}

// readProduct decodes a product header entry.
func readProduct(text string, line int) (string, *catalog.Product, error) {
	parts := strings.Fields(text)
	if len(parts) < 2 {
		return "", nil, errorf(line, "product entry needs a slug")
	}
	slug := unescape(parts[1])
	p := &catalog.Product{Slug: slug, Name: slug}
	words := map[string]*string{"n": &p.Name, "m": &p.Manufacturer, "c": &p.Cultivar, "o": &p.Country}
	for _, attr := range parts[2:] {
		key, value := field(attr)
		if dst, ok := words[key]; ok {
			*dst = unescape(value)
			continue
		}
		var err error
		switch key {
		case "thc":
			p.THC, err = strconv.ParseFloat(value, 64)
		case "cbd":
			p.CBD, err = strconv.ParseFloat(value, 64)
		case "g":
			var g int
			g, err = strconv.Atoi(value)
			p.Genetic = can(g)
		case "r":
			p.Radiated = value == "1"
		case "tp":
			p.Terpenes = catalog.ParseTerpenes(unescape(value))
		case "sr", "sn":
			err = readStrainRating(p, key, value)
		case "a":
			var at int64
			if at, err = parseNum(value); err == nil {
				p.AddedAt = time.Unix(at, 0)
			}
		case "ch":
			p.Changed, err = readChanged(unescape(value))
		default:
			return "", nil, errorf(line, "unknown product attribute %q", key)
		}
		if err != nil {
			return "", nil, errorf(line, "unreadable product attribute %q: %v", key, err)
		}
	}
	return slug, p, nil
}

// readStrainRating decodes a strain's own rating: its scores, again and when
// (sr), or its line (sn), whichever comes first making the rating.
func readStrainRating(p *catalog.Product, key, value string) error {
	if p.Rating == nil {
		p.Rating = &catalog.StrainRating{}
	}
	if key == "sn" {
		p.Rating.Note = unescape(value)
		return nil
	}
	f := strings.Split(value, "/")
	if len(f) != 5 {
		return fmt.Errorf("a rating is five fields, not %d", len(f))
	}
	scores := []*int{&p.Rating.Overall, &p.Rating.Taste, &p.Rating.Aroma}
	for i, s := range scores {
		var err error
		if *s, err = strconv.Atoi(f[i]); err != nil {
			return err
		}
	}
	p.Rating.Again = map[string]catalog.Again{"y": catalog.AgainYes, "n": catalog.AgainNo}[f[3]]
	at, err := parseNum(f[4])
	p.Rating.At = time.Unix(at, 0)
	return err
}

// readChanged reads when each field was last changed: "name:unix,…".
func readChanged(text string) (catalog.Changed, error) {
	out := catalog.Changed{}
	for _, part := range strings.Split(text, ",") {
		name, at, ok := strings.Cut(part, ":")
		if !ok {
			return nil, fmt.Errorf("unreadable stamp %q", part)
		}
		n, err := parseNum(at)
		if err != nil {
			return nil, fmt.Errorf("unreadable stamp %q", part)
		}
		out[name] = time.Unix(n, 0)
	}
	return out, nil
}

// readDevice decodes a device header entry.
func readDevice(text string, line int) (string, *catalog.Device, error) {
	parts := strings.Fields(text)
	if len(parts) < 2 {
		return "", nil, errorf(line, "device entry needs a slug")
	}
	slug := unescape(parts[1])
	d := &catalog.Device{Slug: slug, Name: slug}
	for _, attr := range parts[2:] {
		key, value := field(attr)
		if err := deviceAttr(d, key, value); err != nil {
			return "", nil, errorf(line, "%v", err)
		}
	}
	return slug, d, nil
}

// deviceText is where each text attribute of a device line goes.
var deviceText = map[string]func(*catalog.Device) *string{
	"n": func(d *catalog.Device) *string { return &d.Name }, "k": func(d *catalog.Device) *string { return &d.Kind },
	"c": func(d *catalog.Device) *string { return &d.Category }, "mk": func(d *catalog.Device) *string { return &d.Maker },
	"md": func(d *catalog.Device) *string { return &d.Model }, "co": func(d *catalog.Device) *string { return &d.Country },
	"pw": func(d *catalog.Device) *string { return &d.Power }, "sg": func(d *catalog.Device) *string { return &d.Signature },
	"us": func(d *catalog.Device) *string { return &d.Use }, "sh": func(d *catalog.Device) *string { return &d.Shop },
	"bo": func(d *catalog.Device) *string { return &d.BoughtOn }, "ln": func(d *catalog.Device) *string { return &d.Link },
	"nt": func(d *catalog.Device) *string { return &d.Notes },
}

// readModes reads a device's modes: name@signature@temp@timer, the last ones optional.
func readModes(text string) ([]catalog.Mode, error) {
	var out []catalog.Mode
	for _, part := range strings.Split(text, "|") {
		fields := append(strings.Split(part, "@"), "", "", "")
		m := catalog.Mode{Name: fields[0], Signature: fields[1]}
		for _, n := range []struct {
			text string
			into *int
		}{{fields[2], &m.Temp}, {fields[3], &m.Timer}} {
			if n.text == "" {
				continue
			}
			v, err := strconv.Atoi(n.text)
			if err != nil {
				return nil, fmt.Errorf("unreadable mode %q", part)
			}
			*n.into = v
		}
		out = append(out, m)
	}
	return out, nil
}

// deviceAttr reads one attribute of a device line into the device.
func deviceAttr(d *catalog.Device, key, value string) error {
	if at, ok := deviceText[key]; ok {
		*at(d) = unescape(value)
		return nil
	}
	var err error
	switch key {
	case "lo":
		d.MinTemp, err = strconv.Atoi(value)
	case "hi":
		d.MaxTemp, err = strconv.Atoi(value)
	case "df":
		d.DefaultTemp, err = strconv.Atoi(value)
	case "pr":
		d.Cents, err = strconv.ParseInt(value, 10, 64)
	case "ao":
		d.AutoOff, err = strconv.Atoi(value)
	case "ld":
		var cg int64
		cg, err = strconv.ParseInt(value, 10, 64)
		d.Load = journal.Centigrams(cg).Grams()
	case "mo":
		d.Modes, err = readModes(unescape(value))
	case "w":
		d.Water = value == "1"
	case "rt":
		d.Retired = value == "1"
	case "ch":
		d.Changed, err = readChanged(unescape(value))
	default:
		return fmt.Errorf("unknown device attribute %q", key)
	}
	if err != nil {
		return fmt.Errorf("unreadable device attribute %q: %v", key, err)
	}
	return nil
}
