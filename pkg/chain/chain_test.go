package chain

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// note is the smallest record a log can hold: what it is about, a word, and
// its place in the chain, in the field order its hash is taken over.
type note struct {
	Seq   int       `json:"seq"`
	About string    `json:"about"`
	At    time.Time `json:"at"`
	Text  string    `json:"text"`
	Prev  string    `json:"prev"`
	Hash  string    `json:"hash"`
}

func (n *note) Validate() error {
	if n.Text == "" {
		return errors.New("a note says something")
	}
	return nil
}
func (n *note) Link() Link     { return Link{Seq: n.Seq, At: n.At, Prev: n.Prev, Hash: n.Hash} }
func (n *note) SetLink(l Link) { n.Seq, n.At, n.Prev, n.Hash = l.Seq, l.At, l.Prev, l.Hash }

var errBroken = errors.New("note log hash chain is broken")

var at = time.Date(2026, time.July, 1, 20, 0, 0, 0, time.UTC)

func testLog(t *testing.T) *Log[note, *note] {
	t.Helper()
	return Open[note](filepath.Join(t.TempDir(), "notes.ndjson"), "note log", "note", errBroken)
}

func lines(t *testing.T, l *Log[note, *note]) int {
	t.Helper()
	data, err := os.ReadFile(l.Path())
	require.NoError(t, err)
	return strings.Count(string(data), "\n")
}

func TestAppend(t *testing.T) {
	l := testLog(t)

	first, err := l.Append(note{About: "a", At: at.Add(time.Millisecond), Text: "one"})
	require.NoError(t, err)
	second, err := l.Append(note{About: "b", At: at, Text: "two"})
	require.NoError(t, err)

	assert.Equal(t, 1, first.Seq)
	assert.Equal(t, at, first.At, "Should keep the time to the second")
	assert.Equal(t, 2, second.Seq)
	assert.Equal(t, first.Hash, second.Prev, "Should chain each onto the one before")
	require.NoError(t, l.Verify())
	all, err := l.All()
	require.NoError(t, err)
	assert.Equal(t, []note{first, second}, all)

	before := time.Now().Truncate(time.Second)
	undated, err := l.Append(note{About: "b", Text: "three"})
	require.NoError(t, err)
	assert.False(t, undated.At.Before(before), "Should date an undated record now")

	_, err = l.Append(note{About: "c"})
	assert.Error(t, err, "Should refuse a record that is not well formed")
	assert.Equal(t, 3, lines(t, l), "and write nothing for it")
}

func TestAppendAll(t *testing.T) {
	l := testLog(t)
	_, err := l.Append(note{About: "a", At: at, Text: "one"})
	require.NoError(t, err)

	t.Run("OneWriteForAll", func(t *testing.T) {
		out, err := l.AppendAll([]note{{About: "b", At: at, Text: "two"}, {About: "c", At: at, Text: "three"}})

		require.NoError(t, err)
		require.Len(t, out, 2, "Should return what it stored")
		assert.Equal(t, 2, out[0].Seq)
		assert.Equal(t, out[0].Hash, out[1].Prev)
		assert.Equal(t, 3, lines(t, l))
		require.NoError(t, l.Verify())
	})

	t.Run("NoneIfAnyIsRefused", func(t *testing.T) {
		_, err := l.AppendAll([]note{{About: "d", At: at, Text: "four"}, {About: "e", At: at}})

		assert.Error(t, err)
		assert.Equal(t, 3, lines(t, l), "Should write none of the batch")
	})

	t.Run("Nothing", func(t *testing.T) {
		out, err := l.AppendAll(nil)

		require.NoError(t, err)
		assert.Empty(t, out)
	})
}

func TestChainIsWhatAppendingWrites(t *testing.T) {
	l := testLog(t)
	in := []note{{About: "a", At: at, Text: "one"}, {About: "b", At: at, Text: "two"}}
	stored, err := l.AppendAll(in)
	require.NoError(t, err)

	chained, err := Chain[note](in)

	require.NoError(t, err)
	assert.Equal(t, stored, chained)
	_, err = Chain[note]([]note{{About: "a", At: at}})
	assert.Error(t, err)
}

func TestVerify(t *testing.T) {
	l := testLog(t)
	_, err := l.AppendAll([]note{{About: "a", At: at, Text: "one"}, {About: "b", At: at, Text: "two"}})
	require.NoError(t, err)
	data, err := os.ReadFile(l.Path())
	require.NoError(t, err)

	t.Run("AChangedRecord", func(t *testing.T) {
		require.NoError(t, os.WriteFile(l.Path(), []byte(strings.Replace(string(data), "two", "TWO", 1)), 0600))

		assert.ErrorIs(t, l.Verify(), errBroken)
		assert.ErrorContains(t, l.Verify(), "at note 2")
	})

	t.Run("ALineCutOff", func(t *testing.T) {
		require.NoError(t, os.WriteFile(l.Path(), append(data, `{"seq":3,"about"`...), 0600))

		require.NoError(t, l.Verify(), "Should leave out a write that never finished")
		all, err := l.All()
		require.NoError(t, err)
		assert.Len(t, all, 2)
	})

	t.Run("NotJSON", func(t *testing.T) {
		require.NoError(t, os.WriteFile(l.Path(), []byte("not json\n"), 0600))

		assert.ErrorContains(t, l.Verify(), "note log line 1 is not valid JSON")
	})
}

func TestRebase(t *testing.T) {
	t.Run("TakesTheirsAndChainsOursOnTop", func(t *testing.T) {
		phone, deck := testLog(t), testLog(t)
		_, err := phone.Append(note{About: "theirs", At: at, Text: "one"})
		require.NoError(t, err)
		_, err = deck.Append(note{About: "ours", At: at, Text: "two"})
		require.NoError(t, err)
		theirs, err := phone.All()
		require.NoError(t, err)

		fetched, rechained, err := deck.Rebase(theirs)

		require.NoError(t, err)
		assert.Equal(t, 1, fetched)
		assert.Equal(t, 1, rechained)
		require.NoError(t, deck.Verify())
		ours, err := deck.All()
		require.NoError(t, err)
		require.Len(t, ours, 2)
		assert.Equal(t, theirs[0], ours[0], "Should keep the other copy's record as it is")
		assert.Equal(t, "ours", ours[1].About, "Should keep naming the entry it named")
		assert.Equal(t, ours[0].Hash, ours[1].Prev, "Should chain its own onto theirs")
	})

	t.Run("NothingNew", func(t *testing.T) {
		deck := testLog(t)
		_, err := deck.Append(note{About: "a", At: at, Text: "one"})
		require.NoError(t, err)
		same, err := deck.All()
		require.NoError(t, err)

		fetched, rechained, err := deck.Rebase(same)

		require.NoError(t, err)
		assert.Zero(t, fetched)
		assert.Zero(t, rechained)
	})

	t.Run("RefusesABrokenChain", func(t *testing.T) {
		phone := testLog(t)
		_, err := phone.Append(note{About: "a", At: at, Text: "one"})
		require.NoError(t, err)
		theirs, err := phone.All()
		require.NoError(t, err)
		theirs[0].Text = "changed"

		_, _, err = testLog(t).Rebase(theirs)

		assert.ErrorIs(t, err, errBroken)
		assert.ErrorContains(t, err, "at note 1 of the other copy")
	})
}
