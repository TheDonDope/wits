package bundle

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	cannabis "github.com/TheDonDope/wits-tui/pkg/cannabis"
	"github.com/TheDonDope/wits-tui/pkg/catalog"
	"github.com/TheDonDope/wits-tui/pkg/journal"
)

// can converts a stored integer back to a genetic type.
func can(g int) cannabis.GeneticType { return cannabis.GeneticType(g) }

// Read decodes a bundle.
//
// The events come back without their sequence numbers or hashes: those belong
// to a journal, and are assigned when the events are appended to one. Restoring
// into an empty journal in this order reproduces the chain exactly.
func Read(r io.Reader) (*Contents, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	line := 0
	next := func() (string, bool) {
		if !sc.Scan() {
			return "", false
		}
		line++
		return sc.Text(), true
	}

	head, ok := next()
	if !ok {
		return nil, fmt.Errorf("bundle is empty")
	}
	if err := checkHeader(head, line); err != nil {
		return nil, err
	}

	c := &Contents{Products: &catalog.Catalog{}, Devices: &catalog.Devices{}}
	var products, devices, notes []string

	for {
		text, ok := next()
		if !ok {
			return nil, fmt.Errorf("bundle ended before its events began")
		}
		if text == separator {
			break
		}
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		switch text[0] {
		case 'P':
			slug, product, err := readProduct(text, line)
			if err != nil {
				return nil, err
			}
			products = append(products, slug)
			c.Products.Products = append(c.Products.Products, product)
		case 'D':
			slug, device, err := readDevice(text, line)
			if err != nil {
				return nil, err
			}
			devices = append(devices, slug)
			c.Devices.Devices = append(c.Devices.Devices, device)
		case 'N':
			parts := strings.SplitN(text, " ", 2)
			if len(parts) != 2 {
				return nil, errorf(line, "note entry needs a value")
			}
			notes = append(notes, unescape(parts[1]))
		default:
			return nil, errorf(line, "unknown header entry %q", text)
		}
	}

	var occurred, recorded int64
	offset, recordedOffset := 0, 0
	for {
		text, ok := next()
		if !ok {
			break
		}
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		e, st, err := readEvent(text, line, products, devices, notes, state{occurred, recorded, offset, recordedOffset})
		if err != nil {
			return nil, err
		}
		occurred, recorded, offset, recordedOffset = st.occurred, st.recorded, st.offset, st.recordedOffset
		c.Events = append(c.Events, e)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return c, nil
}

// checkHeader validates the magic line.
func checkHeader(text string, line int) error {
	parts := strings.Fields(text)
	if len(parts) != 2 || parts[0] != Magic {
		return errorf(line, "not a wits bundle")
	}
	version, err := strconv.Atoi(parts[1])
	if err != nil {
		return errorf(line, "unreadable format version %q", parts[1])
	}
	if version > Version {
		return errorf(line, "bundle is format version %d, this build understands %d", version, Version)
	}
	return nil
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
func readEvent(text string, line int, products, devices, notes []string, in state) (journal.Event, state, error) {
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
	if err != nil || pi < 0 || int(pi) >= len(products) {
		return e, out, errorf(line, "event refers to product %q, which the header does not define", parts[0][1:])
	}
	e.Product = products[pi]

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
		switch key {
		case "r":
			d, err := parseNum(value)
			if err != nil {
				return e, out, errorf(line, "unreadable recorded delta %q", value)
			}
			out.recorded = in.recorded + d
		case "z":
			off, err := parseNum(value)
			if err != nil {
				return e, out, errorf(line, "unreadable zone offset %q", value)
			}
			out.offset = int(off)
		case "zr":
			off, err := parseNum(value)
			if err != nil {
				return e, out, errorf(line, "unreadable recorded zone offset %q", value)
			}
			out.recordedOffset = int(off)
		case "d":
			di, err := parseNum(value)
			if err != nil || di < 0 || int(di) >= len(devices) {
				return e, out, errorf(line, "event refers to device %q, which the header does not define", value)
			}
			e.Device = devices[di]
		case "t":
			temp, err := parseNum(value)
			if err != nil {
				return e, out, errorf(line, "unreadable temperature %q", value)
			}
			e.Temperature = int(temp)
		case "n":
			ni, err := parseNum(value)
			if err != nil || ni < 0 || int(ni) >= len(notes) {
				return e, out, errorf(line, "event refers to note %q, which the header does not define", value)
			}
			e.Note = notes[ni]
		case "v":
			e.Reverts = value
		default:
			return e, out, errorf(line, "unknown attribute %q", key)
		}
	}

	e.OccurredAt = time.Unix(out.occurred, 0).In(zone(out.offset))
	e.RecordedAt = time.Unix(out.recorded, 0).In(zone(out.recordedOffset))
	e.From, e.To, _ = journal.Flow(e.Type)
	return e, out, nil
}

// readProduct decodes a product header entry.
func readProduct(text string, line int) (string, *catalog.Product, error) {
	parts := strings.Fields(text)
	if len(parts) < 2 {
		return "", nil, errorf(line, "product entry needs a slug")
	}
	slug := unescape(parts[1])
	p := &catalog.Product{Slug: slug, Name: slug}
	for _, attr := range parts[2:] {
		key, value := field(attr)
		var err error
		switch key {
		case "n":
			p.Name = unescape(value)
		case "m":
			p.Manufacturer = unescape(value)
		case "c":
			p.Cultivar = unescape(value)
		case "o":
			p.Country = unescape(value)
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
		case "a":
			var at int64
			if at, err = parseNum(value); err == nil {
				p.AddedAt = time.Unix(at, 0)
			}
		default:
			return "", nil, errorf(line, "unknown product attribute %q", key)
		}
		if err != nil {
			return "", nil, errorf(line, "unreadable product attribute %q: %v", key, err)
		}
	}
	return slug, p, nil
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
		var err error
		switch key {
		case "n":
			d.Name = unescape(value)
		case "k":
			d.Kind = unescape(value)
		case "lo":
			d.MinTemp, err = strconv.Atoi(value)
		case "hi":
			d.MaxTemp, err = strconv.Atoi(value)
		case "df":
			d.DefaultTemp, err = strconv.Atoi(value)
		default:
			return "", nil, errorf(line, "unknown device attribute %q", key)
		}
		if err != nil {
			return "", nil, errorf(line, "unreadable device attribute %q: %v", key, err)
		}
	}
	return slug, d, nil
}
