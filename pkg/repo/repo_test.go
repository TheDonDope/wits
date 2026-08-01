package repo

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain handles global test setup
func TestMain(m *testing.M) {
	// Disable log output during tests
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

func TestInit(t *testing.T) {
	t.Run("CreatesTheRepository", func(t *testing.T) {
		dir := t.TempDir()

		r, err := Init(dir)
		require.NoError(t, err)

		assert.Equal(t, filepath.Join(dir, Dir), r.Root(), "Should create .wits in the given directory")
		assert.Equal(t, dir, r.WorkTree(), "Should report the work tree")
		for _, path := range []string{r.ProductsPath(), r.DevicesPath(), filepath.Join(r.Root(), configFile)} {
			assert.FileExists(t, path, "Should seed %s", filepath.Base(path))
		}
	})

	t.Run("KeepsHealthDataPrivate", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("running as root, permissions are not enforced")
		}
		r, err := Init(t.TempDir())
		require.NoError(t, err)

		info, err := os.Stat(r.Root())
		require.NoError(t, err)
		assert.Equal(t, dirPerm, info.Mode().Perm(), "Should not let anyone else into the repository")

		info, err = os.Stat(r.ProductsPath())
		require.NoError(t, err)
		assert.Equal(t, filePerm, info.Mode().Perm(), "Should not let anyone else read the catalog")
	})

	t.Run("RefusesToClobberAnExistingRepository", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Init(dir)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, Dir, journalFile), []byte("precious\n"), filePerm))

		_, err = Init(dir)

		assert.ErrorIs(t, err, ErrAlreadyARepo, "Should refuse to initialise twice")
		data, err := os.ReadFile(filepath.Join(dir, Dir, journalFile))
		require.NoError(t, err)
		assert.Equal(t, "precious\n", string(data), "Should leave the existing journal alone")
	})
}

func TestDiscover(t *testing.T) {
	t.Run("FindsTheRepositoryFromASubdirectory", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Init(dir)
		require.NoError(t, err)
		nested := filepath.Join(dir, "a", "b", "c")
		require.NoError(t, os.MkdirAll(nested, 0700))

		r, err := Discover(nested)
		require.NoError(t, err)

		assert.Equal(t, filepath.Join(dir, Dir), r.Root(), "Should walk up to the repository")
	})

	t.Run("ReportsWhenThereIsNoRepository", func(t *testing.T) {
		_, err := Discover(t.TempDir())

		assert.ErrorIs(t, err, ErrNotARepo, "Should report a missing repository rather than inventing one")
	})

	t.Run("ReadsTheConfiguration", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Init(dir)
		require.NoError(t, err)

		r, err := Discover(dir)
		require.NoError(t, err)

		assert.Equal(t, Version, r.Config.Version, "Should read the format version")
		assert.Equal(t, "INFO", r.Config.LogLevel, "Should read the log level")
	})

	t.Run("RefusesAFutureFormatVersion", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Init(dir)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, Dir, configFile), []byte("version: 99\n"), filePerm))

		_, err = Discover(dir)

		assert.Error(t, err, "Should not guess at a format it does not understand")
	})
}

func TestJournal(t *testing.T) {
	dir := t.TempDir()
	r, err := Init(dir)
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(r.Root(), journalFile), r.Journal().Path(), "Should hand out the repository journal")
}
