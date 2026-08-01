package bundle

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/journal"
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
		{Type: journal.Purchase, Product: "enua-wedding-cake", Grams: 20, OccurredAt: at},
		{Type: journal.Purchase, Product: "cannamedical-lemon-cookie", Grams: 10, OccurredAt: at},
		{Type: journal.Grind, Product: "enua-wedding-cake", Grams: 0.75, OccurredAt: at.AddDate(0, 0, 1)},
		{Type: journal.Sesh, Product: "enua-wedding-cake", Grams: 0.3, OccurredAt: at.AddDate(0, 0, 1),
			Device: "volcano-hybrid", Temperature: 185, Note: "evening, with a space"},
		{Type: journal.Grind, Product: "cannamedical-lemon-cookie", Grams: 1.25,
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

		p, err := got.Products.Find("enua-wedding-cake")
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
