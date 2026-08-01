package journal

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain handles global test setup
func TestMain(m *testing.M) {
	// Disable log output during tests
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// testJournal returns a journal backed by a file in a fresh temp directory.
func testJournal(t *testing.T) *Journal {
	t.Helper()
	return Open(filepath.Join(t.TempDir(), "journal.ndjson"))
}

// write replaces the journal file with the given events, bypassing Append so
// that tests can produce files Wits itself would never write.
func write(t *testing.T, path string, events []Event) error {
	t.Helper()
	var buf []byte
	for _, e := range events {
		line, err := json.Marshal(e)
		if err != nil {
			return err
		}
		buf = append(buf, append(line, '\n')...)
	}
	return os.WriteFile(path, buf, 0600)
}

// rewrite replaces a single line of the journal file with the given event.
func rewrite(t *testing.T, path string, idx int, e Event) error {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := bytes.Split(bytes.TrimRight(raw, "\n"), []byte("\n"))
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	lines[idx] = line
	return os.WriteFile(path, append(bytes.Join(lines, []byte("\n")), '\n'), 0600)
}

func TestAppend(t *testing.T) {
	t.Run("FillsInTheDerivedFields", func(t *testing.T) {
		j := testJournal(t)

		e, err := j.Append(Event{Type: Purchase, Product: "wedding-cake", Grams: 20})
		require.NoError(t, err)

		assert.Equal(t, 1, e.Seq, "Should be the first event")
		assert.NotEmpty(t, e.Hash, "Should be hashed")
		assert.Empty(t, e.Prev, "Should have no predecessor")
		assert.Equal(t, External, e.From, "Should derive the source account")
		assert.Equal(t, Storage, e.To, "Should derive the target account")
		assert.False(t, e.OccurredAt.IsZero(), "Should default occurred_at")
		assert.False(t, e.RecordedAt.IsZero(), "Should stamp recorded_at")
	})

	t.Run("ChainsOntoTheTip", func(t *testing.T) {
		j := testJournal(t)

		first, err := j.Append(Event{Type: Purchase, Product: "wedding-cake", Grams: 20})
		require.NoError(t, err)
		second, err := j.Append(Event{Type: Grind, Product: "wedding-cake", Grams: 0.75})
		require.NoError(t, err)

		assert.Equal(t, 2, second.Seq, "Should increment the sequence")
		assert.Equal(t, first.Hash, second.Prev, "Should chain onto the previous hash")
		assert.NoError(t, j.Verify(), "Should verify")
	})

	t.Run("KeepsABackdatedEntryHonest", func(t *testing.T) {
		j := testJournal(t)
		yesterday := time.Now().AddDate(0, 0, -1)

		e, err := j.Append(Event{Type: Grind, Product: "wedding-cake", Grams: 0.75, OccurredAt: yesterday})
		require.NoError(t, err)

		assert.True(t, e.OccurredAt.Before(e.RecordedAt), "Should record that the entry was late")
	})

	t.Run("RejectsInvalidEvents", func(t *testing.T) {
		for name, e := range map[string]Event{
			"UnknownType":    {Type: "smoke", Product: "wedding-cake", Grams: 1},
			"ZeroGrams":      {Type: Grind, Product: "wedding-cake", Grams: 0},
			"NegativeGrams":  {Type: Grind, Product: "wedding-cake", Grams: -1},
			"MissingProduct": {Type: Grind, Grams: 1},
		} {
			t.Run(name, func(t *testing.T) {
				j := testJournal(t)
				_, err := j.Append(e)
				assert.Error(t, err, "Should reject the event")

				events, err := j.Events()
				require.NoError(t, err)
				assert.Empty(t, events, "Should not have written anything")
			})
		}
	})
}

func TestAppendIsAppendOnly(t *testing.T) {
	j := testJournal(t)

	_, err := j.Append(Event{Type: Purchase, Product: "wedding-cake", Grams: 20})
	require.NoError(t, err)
	before, err := os.ReadFile(j.Path())
	require.NoError(t, err)

	_, err = j.Append(Event{Type: Grind, Product: "wedding-cake", Grams: 0.75})
	require.NoError(t, err)
	after, err := os.ReadFile(j.Path())
	require.NoError(t, err)

	assert.Equal(t, before, after[:len(before)], "Should leave the existing bytes untouched")
}

func TestEvents(t *testing.T) {
	t.Run("MissingFileIsAnEmptyJournal", func(t *testing.T) {
		events, err := testJournal(t).Events()

		assert.NoError(t, err, "Should not fail on a journal that does not exist yet")
		assert.Empty(t, events, "Should be empty")
	})

	t.Run("UnreadableFileIsAnError", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("running as root, permissions are not enforced")
		}
		path := filepath.Join(t.TempDir(), "journal.ndjson")
		require.NoError(t, os.WriteFile(path, []byte("{}\n"), 0000))

		_, err := Open(path).Events()

		assert.Error(t, err, "Should not mistake an unreadable journal for an empty one")
	})

	t.Run("CorruptLineIsAnError", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "journal.ndjson")
		require.NoError(t, os.WriteFile(path, []byte("not json\n"), 0600))

		_, err := Open(path).Events()

		assert.Error(t, err, "Should not silently skip a line it cannot parse")
	})

	t.Run("PreservesOrder", func(t *testing.T) {
		j := testJournal(t)
		for i := 0; i < 5; i++ {
			_, err := j.Append(Event{Type: Grind, Product: "wedding-cake", Grams: 0.5})
			require.NoError(t, err)
		}

		events, err := j.Events()
		require.NoError(t, err)

		require.Len(t, events, 5)
		for i, e := range events {
			assert.Equal(t, i+1, e.Seq, "Should be in journal order")
		}
	})
}

func TestVerify(t *testing.T) {
	t.Run("EmptyJournal", func(t *testing.T) {
		assert.NoError(t, testJournal(t).Verify(), "Should verify an empty journal")
	})

	t.Run("DetectsATamperedAmount", func(t *testing.T) {
		j := testJournal(t)
		_, err := j.Append(Event{Type: Purchase, Product: "wedding-cake", Grams: 20})
		require.NoError(t, err)
		_, err = j.Append(Event{Type: Grind, Product: "wedding-cake", Grams: 0.75})
		require.NoError(t, err)

		// Rewrite the first line with a different amount, keeping its hash.
		events, err := j.Events()
		require.NoError(t, err)
		tampered := events[0]
		tampered.Grams = 200
		require.NoError(t, rewrite(t, j.Path(), 0, tampered))

		assert.ErrorIs(t, j.Verify(), ErrBrokenChain, "Should detect the edit")
	})

	t.Run("DetectsATruncatedJournal", func(t *testing.T) {
		j := testJournal(t)
		_, err := j.Append(Event{Type: Purchase, Product: "wedding-cake", Grams: 20})
		require.NoError(t, err)
		_, err = j.Append(Event{Type: Grind, Product: "wedding-cake", Grams: 0.75})
		require.NoError(t, err)

		// Drop the first line, leaving the second dangling.
		events, err := j.Events()
		require.NoError(t, err)
		require.NoError(t, write(t, j.Path(), events[1:]))

		assert.ErrorIs(t, j.Verify(), ErrBrokenChain, "Should detect the missing entry")
	})
}

func TestFlow(t *testing.T) {
	for _, tt := range []struct {
		typ      Type
		from, to Account
	}{
		{Purchase, External, Storage},
		{Grind, Storage, Stash},
		{Sesh, Stash, Consumed},
		{AVBCollect, Consumed, AVB},
		{AVBUse, AVB, External},
	} {
		t.Run(string(tt.typ), func(t *testing.T) {
			from, to, ok := Flow(tt.typ)

			assert.True(t, ok, "Should be a known type")
			assert.Equal(t, tt.from, from, "Should move grams out of the right account")
			assert.Equal(t, tt.to, to, "Should move grams into the right account")
		})
	}

	t.Run("UnknownType", func(t *testing.T) {
		_, _, ok := Flow("smoke")
		assert.False(t, ok, "Should not know the type")
	})
}

func TestAppendIsSafeAcrossProcesses(t *testing.T) {
	// Two Journal values over one file stand in for two processes: they share no
	// mutex, so only the file lock keeps them from reading the same tip and
	// appending two entries that claim the same predecessor.
	path := filepath.Join(t.TempDir(), "journal.ndjson")

	const writers = 8
	var wg sync.WaitGroup
	errs := make(chan error, writers)
	start := make(chan struct{})

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			j := Open(path) // its own Journal, its own mutex
			<-start
			_, err := j.Append(Event{Type: Grind, Product: "wedding-cake", Grams: 0.5})
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	j := Open(path)
	events, err := j.Events()
	require.NoError(t, err)
	assert.Len(t, events, writers, "Every append should have landed")
	assert.NoError(t, j.Verify(),
		"The chain must still verify: without the file lock, two writers read the same tip and fork it")

	seqs := map[int]bool{}
	for _, e := range events {
		assert.False(t, seqs[e.Seq], "Sequence %d was used twice", e.Seq)
		seqs[e.Seq] = true
	}
}

func TestAppendToAnUnwritableRepository(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, permissions are not enforced")
	}
	dir := t.TempDir()
	j := Open(filepath.Join(dir, "journal.ndjson"))
	require.NoError(t, os.Chmod(dir, 0500))
	t.Cleanup(func() { os.Chmod(dir, 0700) })

	_, err := j.Append(Event{Type: Grind, Product: "wedding-cake", Grams: 1})

	assert.Error(t, err, "Should fail plainly rather than appear to have recorded something")
}

func TestVerifyReportsAnUnreadableJournal(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, permissions are not enforced")
	}
	path := filepath.Join(t.TempDir(), "journal.ndjson")
	require.NoError(t, os.WriteFile(path, []byte("{}\n"), 0000))

	err := Open(path).Verify()

	assert.Error(t, err, "Should not report a journal it cannot read as verified")
}
