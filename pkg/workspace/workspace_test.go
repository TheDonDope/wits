package workspace

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/repo"
)

// TestMain handles global test setup
func TestMain(m *testing.M) {
	// Disable log output during tests
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// filled returns a repository holding one fill and one grind.
func filled(t *testing.T) *repo.Repo {
	t.Helper()
	r, err := repo.Init(t.TempDir())
	require.NoError(t, err)

	ws, err := Read(r)
	require.NoError(t, err)
	_, _, _, err = ws.Recorder.Buy("Enua 22/1 Wedding Cake", "", 20, time.Now())
	require.NoError(t, err)
	_, err = ws.Recorder.Grind("wedding", 0.75, time.Now())
	require.NoError(t, err)
	return r
}

func TestRead(t *testing.T) {
	ws, err := Read(filled(t))
	require.NoError(t, err)

	assert.Len(t, ws.Events(), 2, "Should have read the journal")
	assert.Len(t, ws.Products.Products, 1, "Should have read the catalog")
	assert.NotNil(t, ws.Devices, "Should have read the devices, even when there are none")
	assert.Equal(t, 19.25, ws.State.Balances["wcake"].Storage, "Should have folded the journal")
	assert.NotNil(t, ws.Recorder, "Should be ready to record")
	assert.False(t, ws.OpenedAt.IsZero(), "Should stamp when the snapshot was taken")
}

func TestOpenFindsTheRepositoryFromBelow(t *testing.T) {
	r := filled(t)
	nested := filepath.Join(r.WorkTree(), "a", "b")
	require.NoError(t, os.MkdirAll(nested, 0700))

	ws, err := Open(nested)
	require.NoError(t, err)

	assert.Equal(t, r.Root(), ws.Repo.Root(), "Should walk up to the repository")
}

func TestOpenWithoutARepository(t *testing.T) {
	_, err := Open(t.TempDir())

	assert.ErrorIs(t, err, repo.ErrNotARepo, "Should report a missing repository rather than inventing one")
}

func TestSnapshotDoesNotMoveUnderneath(t *testing.T) {
	r := filled(t)
	ws, err := Read(r)
	require.NoError(t, err)
	before := len(ws.Events())

	// Something else writes to the same repository.
	_, err = r.Journal().Append(journal.Event{
		Type: journal.Grind, Product: "wcake", Grams: 0.5,
		From: journal.Storage, To: journal.Stash, OccurredAt: time.Now(),
	})
	require.NoError(t, err)

	assert.Len(t, ws.Events(), before,
		"A snapshot should not change under a screen that is drawing from it")

	fresh, err := ws.Reload()
	require.NoError(t, err)
	assert.Len(t, fresh.Events(), before+1, "and Reload should pick the new entry up")
}

func TestProductName(t *testing.T) {
	ws, err := Read(filled(t))
	require.NoError(t, err)

	assert.Equal(t, "Enua 22/1 Wedding Cake", ws.ProductName("wcake"),
		"Should resolve a slug to its display name")
	assert.Equal(t, "not-in-the-catalog", ws.ProductName("not-in-the-catalog"),
		"and should fall back to the slug, so an orphaned entry still reads sensibly")
}

func TestCycle(t *testing.T) {
	ws, err := Read(filled(t))
	require.NoError(t, err)

	cycle := ws.Cycle()
	require.NotNil(t, cycle, "Should report the cycle in progress")
	assert.Equal(t, 20.0, cycle.Purchased, "Should be the fill that opened it")
}

func TestReadEmptyRepository(t *testing.T) {
	r, err := repo.Init(t.TempDir())
	require.NoError(t, err)

	ws, err := Read(r)
	require.NoError(t, err)

	assert.Empty(t, ws.Events(), "Should read as empty rather than failing")
	assert.Nil(t, ws.Cycle(), "Should report no cycle")
	assert.NotNil(t, ws.Recorder, "and should still be ready to record the first entry")
}

func TestHere(t *testing.T) {
	t.Run("OpensTheRepositoryAroundTheWorkingDirectory", func(t *testing.T) {
		r := filled(t)
		nested := filepath.Join(r.WorkTree(), "notes")
		require.NoError(t, os.MkdirAll(nested, 0700))
		t.Chdir(nested)

		ws, err := Here()
		require.NoError(t, err)

		assert.Equal(t, r.Root(), ws.Repo.Root(), "Should find the repository from where it is run")
		assert.Len(t, ws.Events(), 2, "and should have read it")
	})

	t.Run("WithoutARepository", func(t *testing.T) {
		t.Chdir(t.TempDir())

		_, err := Here()

		assert.ErrorIs(t, err, repo.ErrNotARepo, "Should say there is no repository here")
	})
}

func TestJournalIsTheRepositoryJournal(t *testing.T) {
	r := filled(t)
	ws, err := Read(r)
	require.NoError(t, err)

	assert.Equal(t, r.Journal().Path(), ws.Journal().Path(), "Should hand out the repository's own journal")
	assert.NoError(t, ws.Journal().Verify(), "which should verify")
}

func TestReadRefusesABrokenCatalog(t *testing.T) {
	r, err := repo.Init(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(r.ProductsPath(), []byte("products: [oh dear\n"), 0600))

	_, err = Read(r)

	assert.Error(t, err, "Should refuse to open rather than quietly treat a broken catalog as empty")
}

func TestReadRefusesABrokenJournal(t *testing.T) {
	r, err := repo.Init(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(r.Journal().Path(), []byte("not json\n"), 0600))

	_, err = Read(r)

	assert.Error(t, err, "Should refuse to open rather than fold a journal it could not read")
}
