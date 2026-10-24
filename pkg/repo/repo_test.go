package repo

import (
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/catalog"
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
		assert.Equal(t, "wits.log", r.Config.LogFile, "Should read the log file")
	})

	t.Run("IgnoresSettingsItNoLongerKnows", func(t *testing.T) {
		// Repositories initialised by older builds carry log_level, which
		// nothing ever read. Opening one must not fail over it.
		dir := t.TempDir()
		_, err := Init(dir)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, Dir, configFile),
			[]byte("version: 1\nlog_level: INFO\nlog_file: wits.log\n"), filePerm))

		r, err := Discover(dir)
		require.NoError(t, err)
		assert.Equal(t, "wits.log", r.Config.LogFile, "Should still read the settings it knows")
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

func TestUpdateProducts(t *testing.T) {
	r, err := Init(t.TempDir())
	require.NoError(t, err)
	// A second value over the same repository: another process.
	other, err := Discover(r.WorkTree())
	require.NoError(t, err)

	_, err = other.UpdateProducts(func(c *catalog.Catalog) error {
		return c.Add(&catalog.Product{Slug: "lemon", Name: "Lemon Cookie"})
	})
	require.NoError(t, err)

	got, err := r.UpdateProducts(func(c *catalog.Catalog) error {
		return c.Add(&catalog.Product{Slug: "wcake", Name: "Wedding Cake"})
	})
	require.NoError(t, err)

	assert.Len(t, got.Products, 2, "Should add onto the catalog as it stands, not as this value last read it")
	onDisk, err := catalog.Load(r.ProductsPath())
	require.NoError(t, err)
	assert.Len(t, onDisk.Products, 2, "and write both back")

	_, err = r.UpdateProducts(func(c *catalog.Catalog) error {
		c.Products = nil
		return errors.New("changed my mind")
	})
	assert.EqualError(t, err, "changed my mind")
	onDisk, err = catalog.Load(r.ProductsPath())
	require.NoError(t, err)
	assert.Len(t, onDisk.Products, 2, "A change that fails should write nothing")
}

func TestUpdateDevices(t *testing.T) {
	r, err := Init(t.TempDir())
	require.NoError(t, err)
	other, err := Discover(r.WorkTree())
	require.NoError(t, err)

	_, err = other.UpdateDevices(func(d *catalog.Devices) error { return d.Add(&catalog.Device{Name: "Volcano"}) })
	require.NoError(t, err)
	got, err := r.UpdateDevices(func(d *catalog.Devices) error { return d.Add(&catalog.Device{Name: "Mighty"}) })
	require.NoError(t, err)

	assert.Len(t, got.Devices, 2)
}

func TestSetThemeKeepsTheConfigAsItStands(t *testing.T) {
	r, err := Init(t.TempDir())
	require.NoError(t, err)
	other, err := Discover(r.WorkTree())
	require.NoError(t, err)
	other.Config.LogFile = "elsewhere.log"
	require.NoError(t, other.writeConfig())

	require.NoError(t, r.SetTheme("nord"))

	reopened, err := Discover(r.WorkTree())
	require.NoError(t, err)
	assert.Equal(t, "nord", reopened.Config.Theme)
	assert.Equal(t, "elsewhere.log", reopened.Config.LogFile, "Should not write back the log file this value read at open")
}
