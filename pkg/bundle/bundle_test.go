package bundle

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/prices"
	"github.com/TheDonDope/wits/pkg/seal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain handles global test setup
func TestMain(m *testing.M) {
	// Disable log output during tests
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// berlin is a zone with a non-zero offset, so that the tests exercise the case
// the format has to carry explicitly.
var berlin = time.FixedZone("", 2*3600)

// fill writes the events into a fresh journal and returns it with the stored
// events, which carry the sequence numbers and hashes the journal assigned.
func fill(t *testing.T, events []journal.Event) (*journal.Journal, []journal.Event) {
	t.Helper()
	j := journal.Open(filepath.Join(t.TempDir(), "journal.ndjson"))
	stored := make([]journal.Event, 0, len(events))
	for _, e := range events {
		s, err := j.Append(e)
		require.NoError(t, err)
		stored = append(stored, s)
	}
	return j, stored
}

// sample is a small history touching every field the format carries.
func sample() []journal.Event {
	at := time.Date(2026, time.July, 9, 10, 30, 0, 0, berlin)
	return []journal.Event{
		{Type: journal.Purchase, Product: "enua-wedding-cake-221", Grams: 20, OccurredAt: at},
		{Type: journal.Purchase, Product: "cannamedical-lemon-cookie-281", Grams: 10, OccurredAt: at},
		{Type: journal.Grind, Product: "enua-wedding-cake-221", Grams: 0.75, OccurredAt: at.AddDate(0, 0, 1)},
		{Type: journal.Sesh, Product: "enua-wedding-cake-221", Grams: 0.3, OccurredAt: at.AddDate(0, 0, 1),
			Device: "volcano-hybrid", Temperature: 185, Note: "evening, with a space"},
		{Type: journal.Grind, Product: "cannamedical-lemon-cookie-281", Grams: 1.25,
			OccurredAt: at.AddDate(0, 0, 2), RecordedAt: at.AddDate(0, 0, 3)},
	}
}

// catalogs returns the catalogs matching sample().
func catalogs(t *testing.T) (*catalog.Catalog, *catalog.Devices) {
	t.Helper()
	c := &catalog.Catalog{}
	require.NoError(t, c.Add(catalog.Parse("Enua 22/1 Wedding Cake")))
	require.NoError(t, c.Add(catalog.Parse("Cannamedical 28/1 Lemon Cookie")))
	d := &catalog.Devices{}
	require.NoError(t, d.Add(&catalog.Device{Name: "Volcano Hybrid", Kind: "desktop", MinTemp: 40, MaxTemp: 230, DefaultTemp: 185}))
	return c, d
}

func TestRoundTrip(t *testing.T) {
	t.Run("RestoringReproducesTheJournalExactly", func(t *testing.T) {
		products, devices := catalogs(t)
		source, stored := fill(t, sample())

		var buf bytes.Buffer
		require.NoError(t, Write(&buf, Contents{Products: products, Devices: devices, Events: stored}))

		got, err := Read(&buf)
		require.NoError(t, err)

		_, restored := fill(t, got.Events)

		require.Len(t, restored, len(stored))
		for i := range stored {
			assert.Equal(t, stored[i].Hash, restored[i].Hash,
				"event %d should hash identically, so a restored journal verifies against the one it came from", i+1)
			want, err := journal.Marshal(stored[i])
			require.NoError(t, err)
			got, err := journal.Marshal(restored[i])
			require.NoError(t, err)
			assert.Equal(t, string(want), string(got), "event %d should be identical as stored", i+1)
		}

		// The strongest statement of the same thing: the files match byte for byte.
		original, err := os.ReadFile(source.Path())
		require.NoError(t, err)
		var rebuilt bytes.Buffer
		for _, e := range restored {
			line, err := journal.Marshal(e)
			require.NoError(t, err)
			rebuilt.Write(append(line, '\n'))
		}
		assert.Equal(t, string(original), rebuilt.String(), "the restored journal should be byte-identical")
	})

	t.Run("CarriesTheCatalogs", func(t *testing.T) {
		products, devices := catalogs(t)
		_, stored := fill(t, sample())

		var buf bytes.Buffer
		require.NoError(t, Write(&buf, Contents{Products: products, Devices: devices, Events: stored}))
		got, err := Read(&buf)
		require.NoError(t, err)

		p, err := got.Products.Find("enua-wedding-cake-221")
		require.NoError(t, err)
		assert.Equal(t, "Enua 22/1 Wedding Cake", p.Name, "Should keep the display name")
		assert.Equal(t, "Enua", p.Manufacturer, "Should keep the manufacturer")
		assert.Equal(t, 22.0, p.THC, "Should keep the THC percentage")

		d, err := got.Devices.Find("volcano-hybrid")
		require.NoError(t, err)
		assert.Equal(t, 230, d.MaxTemp, "Should keep the temperature range")
		assert.Equal(t, "desktop", d.Kind, "Should keep the kind")
	})

	t.Run("KeepsZoneOffsets", func(t *testing.T) {
		_, stored := fill(t, sample())

		var buf bytes.Buffer
		require.NoError(t, Write(&buf, Contents{Events: stored}))
		got, err := Read(&buf)
		require.NoError(t, err)

		for i, e := range got.Events {
			assert.Equal(t, offsetOf(stored[i].OccurredAt), offsetOf(e.OccurredAt),
				"event %d should keep its offset, or a late-evening entry would land on the wrong day", i+1)
		}
	})

	t.Run("KeepsUTCAsUTC", func(t *testing.T) {
		// A zero offset must come back as UTC rather than a fixed zone: they
		// render as "Z" and "+00:00", and the hash is taken over the rendering.
		_, stored := fill(t, []journal.Event{
			{Type: journal.Grind, Product: "wedding-cake", Grams: 1,
				OccurredAt: time.Date(2026, time.July, 9, 10, 0, 0, 0, time.UTC)},
		})

		var buf bytes.Buffer
		require.NoError(t, Write(&buf, Contents{Events: stored}))
		got, err := Read(&buf)
		require.NoError(t, err)

		_, restored := fill(t, got.Events)
		assert.Equal(t, stored[0].Hash, restored[0].Hash, "Should hash identically")
	})

	t.Run("SurvivesAwkwardText", func(t *testing.T) {
		_, stored := fill(t, []journal.Event{
			{Type: journal.Grind, Product: "wedding-cake", Grams: 1,
				OccurredAt: time.Date(2026, time.July, 9, 10, 0, 0, 0, berlin),
				Note:       `spaces and \ backslashes and = equals`},
		})

		var buf bytes.Buffer
		require.NoError(t, Write(&buf, Contents{Events: stored}))
		got, err := Read(&buf)
		require.NoError(t, err)

		_, restored := fill(t, got.Events)
		assert.Equal(t, stored[0].Note, restored[0].Note, "Should carry the note through unchanged")
		assert.Equal(t, stored[0].Hash, restored[0].Hash, "Should hash identically")
	})

	t.Run("EmptyRepository", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, Write(&buf, Contents{Products: &catalog.Catalog{}, Devices: &catalog.Devices{}}))

		got, err := Read(&buf)
		require.NoError(t, err)
		assert.Empty(t, got.Events, "Should read back as empty")
	})
}

func TestWriteIsCompact(t *testing.T) {
	_, stored := fill(t, sample())

	var bundled bytes.Buffer
	require.NoError(t, Write(&bundled, Contents{Events: stored}))

	var raw int
	for _, e := range stored {
		line, err := journal.Marshal(e)
		require.NoError(t, err)
		raw += len(line) + 1
	}
	assert.Less(t, bundled.Len(), raw/3, "Should be a good deal smaller than the journal it came from")
}

func TestRead(t *testing.T) {
	for name, text := range map[string]string{
		"NotABundle":       "hello\n",
		"NoSeparator":      "wits-bundle 1\nP0 wedding-cake\n",
		"FutureVersion":    "wits-bundle 99\n--\n",
		"UnknownType":      "wits-bundle 1\nP0 wedding-cake\n--\nx0 0 100\n",
		"UnknownProduct":   "wits-bundle 1\nP0 wedding-cake\n--\ng9 0 100\n",
		"ShortEvent":       "wits-bundle 1\nP0 wedding-cake\n--\ng0 0\n",
		"UnknownAttribute": "wits-bundle 1\nP0 wedding-cake\n--\ng0 0 100 q=1\n",
		"Empty":            "",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Read(bytes.NewBufferString(text))
			assert.Error(t, err, "Should reject it rather than guess")
		})
	}

	t.Run("IgnoresCommentsAndBlankLines", func(t *testing.T) {
		got, err := Read(bytes.NewBufferString("wits-bundle 1\n# a note\n\nP0 wedding-cake\n--\n# another\n\ng0 0 100\n"))
		require.NoError(t, err)

		require.Len(t, got.Events, 1)
		assert.Equal(t, 1.0, got.Events[0].Grams, "Should read the event")
	})
}

// corrected is a history with both kinds of adjustment: an undo, and a
// weighing with the note wits writes for one.
func corrected(t *testing.T) (*journal.Journal, []journal.Event) {
	t.Helper()
	at := time.Date(2026, time.July, 9, 10, 30, 0, 0, berlin)
	j, stored := fill(t, []journal.Event{
		{Type: journal.Purchase, Product: "enua-wedding-cake-221", Grams: 20, OccurredAt: at},
		{Type: journal.Grind, Product: "enua-wedding-cake-221", Grams: 2, OccurredAt: at.AddDate(0, 0, 1)},
	})
	more := []journal.Event{
		{Type: journal.Adjust, Product: "enua-wedding-cake-221", Grams: 2, From: journal.Stash, To: journal.Storage,
			OccurredAt: at.AddDate(0, 0, 2), Reverts: stored[1].Hash, Note: "reverts " + stored[1].Hash[:7]},
		{Type: journal.Adjust, Product: "enua-wedding-cake-221", Grams: 1.5, From: journal.Storage, To: journal.External,
			OccurredAt: at.AddDate(0, 0, 3), Note: "reconciled storage: 18.50 g weighed, 20.00 g expected"},
		{Type: journal.Adjust, Product: "enua-wedding-cake-221", Grams: 0.5, From: journal.External, To: journal.AVB,
			OccurredAt: at.AddDate(0, 0, 4), Note: "reconciled the AVB jar: 0.50 g weighed, 0.00 g expected"},
	}
	for _, e := range more {
		s, err := j.Append(e)
		require.NoError(t, err)
		stored = append(stored, s)
	}
	return j, stored
}

// restored appends a bundle's events to a fresh journal and returns its bytes.
func restored(t *testing.T, events []journal.Event) []byte {
	t.Helper()
	j, _ := fill(t, events)
	data, err := os.ReadFile(j.Path())
	require.NoError(t, err)
	return data
}

func TestAdjustmentsKeepTheirAccounts(t *testing.T) {
	j, stored := corrected(t)
	var buf bytes.Buffer
	require.NoError(t, Write(&buf, Contents{Events: stored}))
	assert.Contains(t, buf.String(), " f=stash>storage", "An undo's accounts should be written")
	assert.Contains(t, buf.String(), " f=storage>external", "and a weighing's")

	got, err := Read(&buf)
	require.NoError(t, err)

	original, err := os.ReadFile(j.Path())
	require.NoError(t, err)
	assert.Equal(t, string(original), string(restored(t, got.Events)),
		"Restoring should reproduce the journal exactly; the accounts once came back external to external")
}

func TestReadsFormat1Bundles(t *testing.T) {
	format1 := func(t *testing.T, stored []journal.Event) string {
		var buf bytes.Buffer
		require.NoError(t, Write(&buf, Contents{Events: stored}))
		text := strings.Replace(buf.String(), fmt.Sprintf("wits-bundle %d", Version), "wits-bundle 1", 1)
		return regexp.MustCompile(` f=[a-z]+>[a-z]+`).ReplaceAllString(text, "")
	}

	t.Run("RecoversTheAccounts", func(t *testing.T) {
		j, stored := corrected(t)

		got, err := Read(bytes.NewBufferString(format1(t, stored)))

		require.NoError(t, err)
		assert.Equal(t, journal.Stash, got.Events[2].From, "An undo reverses the entry it names")
		assert.Equal(t, journal.External, got.Events[3].To, "A weighing that found less sends grams out")
		assert.Equal(t, journal.AVB, got.Events[4].To, "and one that found more brings them in")
		original, err := os.ReadFile(j.Path())
		require.NoError(t, err)
		assert.Equal(t, string(original), string(restored(t, got.Events)), "and the journal comes back exactly")
	})

	t.Run("RefusesWhatItCannotRecover", func(t *testing.T) {
		_, stored := fill(t, []journal.Event{
			{Type: journal.Purchase, Product: "wcake", Grams: 20, OccurredAt: time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)},
			{Type: journal.Adjust, Product: "wcake", Grams: 1, From: journal.Storage, To: journal.External,
				OccurredAt: time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC), Note: "spilled it"},
		})

		_, err := Read(bytes.NewBufferString(format1(t, stored)))

		assert.ErrorContains(t, err, "its note does not say them", "Should refuse rather than restore it moving nothing")
		assert.ErrorContains(t, err, "event 2")
	})
}

func TestReadRejectsUnknownAccounts(t *testing.T) {
	_, err := Read(bytes.NewBufferString("wits-bundle 2\nP0 wcake\n--\na0 0 100 f=storage>nowhere\n"))
	assert.ErrorContains(t, err, "unreadable accounts")
}

func TestRatingsTravelInTheBundle(t *testing.T) {
	ratings := []effects.Rating{
		{Session: strings.Repeat("a", 64), At: time.Date(2026, 7, 9, 21, 0, 0, 0, berlin),
			Scores: map[effects.Dimension]int{effects.Overall: 4, effects.Sleep: 5}, Note: "slept, with a space"},
		{Session: strings.Repeat("a", 64), At: time.Date(2026, 7, 10, 8, 0, 0, 0, berlin),
			Scores: map[effects.Dimension]int{effects.Overall: 5}},
	}
	var buf bytes.Buffer
	require.NoError(t, Write(&buf, Contents{Ratings: ratings}))
	assert.Contains(t, buf.String(), "\n==\n", "behind their own separator")

	got, err := Read(&buf)
	require.NoError(t, err)
	require.Len(t, got.Ratings, 2)
	assert.Equal(t, ratings[0].Scores, got.Ratings[0].Scores)
	assert.Equal(t, "slept, with a space", got.Ratings[0].Note)
	assert.True(t, ratings[1].At.Equal(got.Ratings[1].At))

	_, err = Read(bytes.NewBufferString("wits-bundle 3\nP0 w\n--\n==\nR abc soon overall:4\n"))
	assert.ErrorContains(t, err, "unreadable rating time")
}

func TestPricesTravelInTheBundle(t *testing.T) {
	paid := []prices.Price{
		{Purchase: strings.Repeat("b", 64), At: time.Date(2026, 8, 29, 12, 0, 0, 0, berlin), Cents: 17000},
		{Purchase: strings.Repeat("b", 64), At: time.Date(2026, 8, 30, 9, 0, 0, 0, berlin), Cents: 18000, Note: "the label said 180"},
	}
	var buf bytes.Buffer
	require.NoError(t, Write(&buf, Contents{Prices: paid}))
	assert.Contains(t, buf.String(), "\n==\nC ", "Should open the section for prices alone, with no ratings")
	assert.True(t, strings.HasPrefix(buf.String(), fmt.Sprintf("wits-bundle %d\n", Version)), "Should be the current format")

	got, err := Read(&buf)
	require.NoError(t, err)
	require.Len(t, got.Prices, 2)
	assert.Equal(t, int64(18000), got.Prices[1].Cents)
	assert.Equal(t, "the label said 180", got.Prices[1].Note)
	assert.True(t, paid[0].At.Equal(got.Prices[0].At))
	assert.Empty(t, got.Ratings)

	_, err = Read(bytes.NewBufferString("wits-bundle 4\nP0 w\n--\n==\nC abc 2026-08-29T12:00:00Z lots\n"))
	assert.ErrorContains(t, err, "unreadable cents")
}

func TestAPriceKeepsItsPharmacyAndItsNoneToBeHad(t *testing.T) {
	paid := []prices.Price{
		{Purchase: strings.Repeat("b", 64), At: time.Date(2026, 8, 29, 12, 0, 0, 0, berlin), Cents: 11680,
			Seller: "Cannabis Apotheke Nordostbahnhof", Note: "invoice 2608"},
		{Purchase: strings.Repeat("c", 64), At: time.Date(2026, 8, 29, 12, 0, 0, 0, berlin), Unknown: true},
	}
	var buf bytes.Buffer
	require.NoError(t, Write(&buf, Contents{Prices: paid}))

	got, err := Read(&buf)

	require.NoError(t, err)
	require.Len(t, got.Prices, 2)
	assert.Equal(t, "Cannabis Apotheke Nordostbahnhof", got.Prices[0].Seller, "The pharmacy travels")
	assert.True(t, got.Prices[1].Unknown, "and so does a price not to be had")
	assert.Zero(t, got.Prices[1].Cents)
	want, err := prices.Chain(paid)
	require.NoError(t, err)
	restored, err := prices.Chain(got.Prices)
	require.NoError(t, err, "A price not to be had restores: it is no price of nothing")
	assert.Equal(t, want[1].Hash, restored[1].Hash, "Restored, each hashes as it did")
}

func TestADeviceAndASessionsModeTravel(t *testing.T) {
	tornado := &catalog.Device{Slug: "tornado", Name: "Vestratto Tornado", Category: catalog.Vaporizer, Maker: "Vestratto",
		Model: "Tornado", Power: catalog.Manual, Signature: catalog.Convection, Use: catalog.SessionUse, Cents: 20800,
		Shop: "vestratto.com", BoughtOn: "2026-03-01", Link: "https://example.org/tornado", Notes: "full soak, 3250 J"}
	yll := &catalog.Device{Slug: "yll-3", Name: "YLL 3.0", Category: catalog.Induction, Power: catalog.Electric,
		Modes: []catalog.Mode{{Name: "30 W"}, {Name: "45 W, gentle"}, {Name: "60 W"}}, Retired: true}
	frolic := &catalog.Device{Slug: "frolic", Name: "Limelight Herb Frolic", Power: catalog.Electric, Use: catalog.BothUses,
		AutoOff: 300, Load: 0.15, Changed: catalog.Changed{"load": time.Unix(1792973444, 0), "name": time.Unix(1792970000, 0)}, Modes: []catalog.Mode{{Name: "On demand", Signature: catalog.Convection, Temp: 165},
			{Name: "Session", Signature: catalog.Hybrid, Temp: 185, Timer: 420}}}
	pipe := &catalog.Device{Slug: "bubbler", Name: "Bubbler", Category: catalog.Glass, Water: true}
	devices := &catalog.Devices{Devices: []*catalog.Device{tornado, yll, pipe, frolic}}
	at := time.Date(2026, 10, 1, 20, 0, 0, 0, berlin)
	events := []journal.Event{
		{Type: journal.Purchase, OccurredAt: at, RecordedAt: at, Product: "w", Grams: 10, From: journal.External, To: journal.Storage},
		{Type: journal.Grind, OccurredAt: at, RecordedAt: at, Product: "w", Grams: 1, From: journal.Storage, To: journal.Stash},
		{Type: journal.Sesh, OccurredAt: at, RecordedAt: at, Product: "w", Grams: 0.1, From: journal.Stash, To: journal.Consumed,
			Device: "tornado", Mode: "full soak", Heater: "yll-3", Setting: "45 W, gentle"},
		{Type: journal.Sesh, OccurredAt: at, RecordedAt: at, Product: "w", Grams: 0.2, From: journal.Stash, To: journal.Consumed,
			Device: "frolic", Mode: "Session", Temperature: 185, Timer: 420},
	}
	var buf bytes.Buffer
	require.NoError(t, Write(&buf, Contents{Devices: devices, Events: events}))
	assert.True(t, strings.HasPrefix(buf.String(), fmt.Sprintf("wits-bundle %d\n", Version)))

	got, err := Read(&buf)

	require.NoError(t, err)
	assert.Equal(t, tornado, findDev(got.Devices, "tornado"), "Every field a device has travels")
	assert.Equal(t, yll, findDev(got.Devices, "yll-3"), "a list of modes with a comma in one, and a device retired")
	assert.True(t, findDev(got.Devices, "bubbler").Water)
	assert.Equal(t, frolic, findDev(got.Devices, "frolic"), "A mode keeps the way it heats")
	s := got.Events[2]
	assert.Equal(t, "full soak", s.Mode)
	assert.Equal(t, "yll-3", s.Heater, "The device that heated it, by its place in the header")
	assert.Equal(t, "45 W, gentle", s.Setting)
	assert.Equal(t, 420, got.Events[3].Timer, "A session's timer travels")
}

func findDev(d *catalog.Devices, slug string) *catalog.Device {
	for _, x := range d.Devices {
		if x.Slug == slug {
			return x
		}
	}
	return nil
}

func TestSealsTravelInTheBundle(t *testing.T) {
	seals := []seal.File{
		{Name: "2026-10-24-f81bbd2cc893", Text: "wits-seal 1\nhead f81b\n", OTS: []byte{0x00, 'O', 0xff, 0x01}},
		{Name: "2026-10-25-aaaaaaaaaaaa", Text: "wits-seal 1\nhead aaaa\n"},
	}
	var buf bytes.Buffer
	require.NoError(t, Write(&buf, Contents{Seals: seals}))

	got, err := Read(&buf)

	require.NoError(t, err)
	assert.Equal(t, seals, got.Seals, "Should bring back each seal and its proof, byte for byte")
	_, err = Read(bytes.NewBufferString("wits-bundle 5\nP0 w\n--\n==\nK name !!!\n"))
	assert.ErrorContains(t, err, "unreadable seal")
}

func TestTerpenesTravelInTheBundle(t *testing.T) {
	products := &catalog.Catalog{Products: []*catalog.Product{
		{Slug: "wcake", Name: "Enua 22/1 Wedding Cake", Terpenes: []string{"β-Caryophyllen", "Limonen", "Myrcen"}},
	}}
	var buf bytes.Buffer
	require.NoError(t, Write(&buf, Contents{Products: products}))

	got, err := Read(&buf)

	require.NoError(t, err)
	require.Len(t, got.Products.Products, 1)
	assert.Equal(t, []string{"β-Caryophyllen", "Limonen", "Myrcen"}, got.Products.Products[0].Terpenes,
		"Should bring the terpenes back, in their order: every format before 6 dropped them")
}

// TestStrainRatingTravelsInTheBundle: a strain's own rating comes back as it
// went, its line with spaces in it and its moment to the second; a strain
// with only a line, or never rated, too.
func TestStrainRatingTravelsInTheBundle(t *testing.T) {
	at := time.Date(2026, 10, 26, 9, 30, 0, 0, time.UTC)
	products := &catalog.Catalog{Products: []*catalog.Product{
		{Slug: "a", Name: "A", Rating: &catalog.StrainRating{Overall: 4, Taste: 5, Again: catalog.AgainYes, Note: "lemon, then pepper", At: at}},
		{Slug: "b", Name: "B", Rating: &catalog.StrainRating{Aroma: 2, Again: catalog.AgainNo, At: at}},
		{Slug: "c", Name: "C"},
	}}
	var buf bytes.Buffer
	require.NoError(t, Write(&buf, Contents{Products: products}))
	assert.Contains(t, buf.String(), " sr=4/5/0/y/")

	got, err := Read(&buf)
	require.NoError(t, err)
	byslug := map[string]*catalog.Product{}
	for _, p := range got.Products.Products {
		byslug[p.Slug] = p
	}
	require.NotNil(t, byslug["a"].Rating)
	assert.Equal(t, catalog.StrainRating{Overall: 4, Taste: 5, Again: catalog.AgainYes, Note: "lemon, then pepper", At: at},
		*withUTC(byslug["a"].Rating))
	assert.Equal(t, catalog.AgainNo, byslug["b"].Rating.Again)
	assert.Equal(t, 2, byslug["b"].Rating.Aroma)
	assert.Nil(t, byslug["c"].Rating)
}

// withUTC is a rating with its moment in UTC, to compare.
func withUTC(r *catalog.StrainRating) *catalog.StrainRating {
	c := *r
	c.At = c.At.UTC()
	return &c
}
