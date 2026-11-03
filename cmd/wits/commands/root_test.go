package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stockedRepo is a repository with a fill, a grind and a session on a
// device: something of every kind get lists.
func stockedRepo(t *testing.T) string {
	t.Helper()
	dir := repository(t)
	for _, args := range [][]string{
		{"create", "device", "Mighty+", "--min", "40", "--max", "210", "--default", "180"},
		{"buy", "Enua 22/1 Wedding Cake", "20g", "--date", "2026-07-01"},
		{"buy", "Sumo 25/1 Papaya Bomb", "10g", "--date", "2026-07-01"},
		{"grind", "wedding", "2", "--date", "2026-07-02"},
		{"sesh", "wedding", "0.3", "--device", "mighty", "--date", "2026-07-02"},
	} {
		_, err := run(t, dir, args...)
		require.NoError(t, err, args)
	}
	return dir
}

// TestRevisions: an entry is named by its hash, or counted back from the
// one written last, as git counts commits.
func TestRevisions(t *testing.T) {
	dir := stockedRepo(t)

	out, err := run(t, dir, "show", "HEAD")
	require.NoError(t, err)
	assert.Contains(t, out, "sesh ", "HEAD is the entry written last")

	for _, rev := range []string{"HEAD~1", "HEAD^", "@~1"} {
		out, err = run(t, dir, "show", rev)
		require.NoError(t, err, rev)
		assert.Contains(t, out, "grind ", "%s is the one before it", rev)
	}

	_, err = run(t, dir, "show", "HEAD~9")
	assert.ErrorContains(t, err, "before the first entry")
	_, err = run(t, dir, "show", "HEAD~x")
	assert.ErrorContains(t, err, "not a revision")

	out, err = run(t, dir, "revert", "HEAD")
	require.NoError(t, err)
	assert.Contains(t, out, "undid sesh 0.30g wcake-221")
	out, err = run(t, dir, "amend", "HEAD~2", "1.5")
	require.NoError(t, err)
	assert.Contains(t, out, "1.50g, not 2.00g", "HEAD~2, counted past the undo, is the grind")
}

// TestGet: every kind lists, wider with -o wide, whole with -o json, and
// only the rows a selector matches.
func TestGet(t *testing.T) {
	dir := stockedRepo(t)

	out, err := run(t, dir, "get", "products")
	require.NoError(t, err)
	assert.Regexp(t, `wcake-221\s+Enua 22/1 Wedding Cake\s+18.00 g\s+1.70 g`, out)
	assert.NotContains(t, out, "THC/CBD", "the narrow table leaves the label out")

	out, err = run(t, dir, "get", "jars", "-o", "wide", "-l", "slug=wcake-221")
	require.NoError(t, err)
	assert.Contains(t, out, "THC/CBD", "-o wide adds the label")
	assert.NotContains(t, out, "Papaya", "-l keeps only the rows it matches")

	out, err = run(t, dir, "get", "sessions", "-o", "json")
	require.NoError(t, err)
	var sessions listOf[sessionRow]
	require.NoError(t, json.Unmarshal([]byte(out), &sessions))
	assert.Equal(t, "wits/v1", sessions.APIVersion)
	assert.Equal(t, "SessionList", sessions.Kind)
	require.Len(t, sessions.Items, 1)
	assert.Equal(t, "mighty", sessions.Items[0].Device)

	out, err = run(t, dir, "get", "devices", "-l", "type=torch", "-o", "json")
	require.NoError(t, err)
	assert.JSONEq(t, `{"apiVersion": "wits/v1", "kind": "DeviceList", "items": []}`, out,
		"no row is an empty list, not null, for a script")

	out, err = run(t, dir, "get", "fills")
	require.NoError(t, err)
	assert.Regexp(t, `1\s+2026-07-01\s+30.00 g\s+28.00 g\s+open`, out)

	out, err = run(t, dir, "get", "entries", "-l", "type=purchase")
	require.NoError(t, err)
	assert.Contains(t, out, "pbomb-251")
	assert.NotContains(t, out, "grind")

	_, err = run(t, dir, "get", "devices", "-o", "yaml")
	assert.ErrorContains(t, err, "table, wide or json")
	_, err = run(t, dir, "get", "devices", "-l", "kind")
	assert.ErrorContains(t, err, "key=value")
}

// TestDescribe: a fill, a device and a temperature told in full.
func TestDescribe(t *testing.T) {
	dir := stockedRepo(t)

	out, err := run(t, dir, "describe", "fill")
	require.NoError(t, err)
	assert.Contains(t, out, "Fill 1, opened 1 Jul 2026, open")
	assert.Contains(t, out, "Dispensed 30.00 g, 28.00 g of it left in storage")

	_, err = run(t, dir, "describe", "fill", "2")
	assert.ErrorContains(t, err, "numbered 1 to 1")

	out, err = run(t, dir, "describe", "device", "mighty")
	require.NoError(t, err)
	assert.Contains(t, out, "Used      1 session, 0.30 g, last on 2 Jul 2026")
	assert.Contains(t, out, "At 180°C, as it is usually set:")

	out, err = run(t, dir, "describe", "temp", "230")
	require.NoError(t, err)
	assert.Contains(t, out, "boiling point of Benzene")
}

// TestDryRun: a write the recorder makes is rehearsed and written nowhere;
// anything else refuses the flag rather than write.
func TestDryRun(t *testing.T) {
	dir := stockedRepo(t)

	out, err := run(t, dir, "grind", "wedding", "1", "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, out, "[rehearsed] grind 1.00g wcake-221, 17.00g left in storage", "it says what the grind would leave")
	assert.Contains(t, out, "Dry run: nothing was written.")

	out, err = run(t, dir, "rate", "session", "overall=4", "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, out, "rated session")
	_, err = run(t, dir, "rate", "session", "overall=9", "--dry-run")
	assert.ErrorContains(t, err, "1 to 5", "a rehearsed rating is checked as a written one is")

	out, err = run(t, dir, "get", "products")
	require.NoError(t, err)
	assert.Regexp(t, `wcake-221\s+Enua 22/1 Wedding Cake\s+18.00 g`, out, "Storage is as it was")
	out, err = run(t, dir, "get", "sessions", "-o", "wide")
	require.NoError(t, err)
	assert.NotContains(t, out, "overall", "and no rating was written")

	_, err = run(t, dir, "status", "--dry-run")
	assert.ErrorContains(t, err, "cannot be rehearsed")
}

// TestDirectory: -C runs a command as if started in another directory.
func TestDirectory(t *testing.T) {
	dir := stockedRepo(t)

	out, err := run(t, t.TempDir(), "-C", dir, "get", "products")

	require.NoError(t, err)
	assert.Contains(t, out, "wcake-221")
}

// TestConfigAndAliases: settings read and set, an alias expanded before the
// command line is parsed, and a command of wits's own never hidden.
func TestConfigAndAliases(t *testing.T) {
	dir := stockedRepo(t)

	_, err := run(t, dir, "config", "alias.g", "grind --dry-run")
	require.NoError(t, err)
	_, err = run(t, dir, "config", "theme", "nord")
	require.NoError(t, err)
	out, err := run(t, dir, "config")
	require.NoError(t, err)
	assert.Equal(t, "theme=nord\nlead=3\nalias.g=grind --dry-run\n", out)

	assert.Equal(t, []string{"grind", "--dry-run", "wedding", "1"}, expand([]string{"g", "wedding", "1"}))
	assert.Equal(t, []string{"-C", dir, "grind", "--dry-run", "wedding"}, expand([]string{"-C", dir, "g", "wedding"}),
		"past -C, in the repository it names")
	assert.Equal(t, []string{"log"}, expand([]string{"log"}))

	_, err = run(t, dir, "config", "alias.log", "status")
	assert.ErrorContains(t, err, "cannot hide")
	_, err = run(t, dir, "config", "theme", "plaid")
	assert.ErrorContains(t, err, "not a theme")
	_, err = run(t, dir, "config", "colour", "red")
	assert.ErrorContains(t, err, "not a setting")

	_, err = run(t, dir, "config", "--unset", "alias.g")
	require.NoError(t, err)
	assert.Equal(t, []string{"g"}, expand([]string{"g"}))
}

// TestClone: a repository made from a bundle, and from a shared folder with
// the first device's key.
func TestClone(t *testing.T) {
	dir := stockedRepo(t)
	file := filepath.Join(t.TempDir(), "history.wits.gz")
	_, err := run(t, dir, "bundle", "create", file)
	require.NoError(t, err)

	elsewhere := t.TempDir()
	out, err := run(t, elsewhere, "clone", file)
	require.NoError(t, err)
	assert.Contains(t, out, "Cloned 2 products, 1 device and 4 events into "+filepath.Join(elsewhere, "history")+".\n",
		"named after the bundle, its extensions dropped")
	_, err = run(t, elsewhere, "clone", file)
	assert.ErrorContains(t, err, "not empty", "A clone never lands on another history")

	share := t.TempDir()
	_, err = run(t, dir, "remote", "add", share)
	require.NoError(t, err)
	_, err = run(t, dir, "push")
	require.NoError(t, err)
	_, err = run(t, elsewhere, "clone", share, "phone")
	assert.ErrorContains(t, err, "--key", "A folder is read with the first device's key")
	require.NoError(t, os.RemoveAll(filepath.Join(elsewhere, "phone")))

	out, err = run(t, elsewhere, "clone", share, "phone", "--key", filepath.Join(dir, ".wits", "remote.key"))
	require.NoError(t, err)
	assert.Contains(t, out, "Pulled 4 entries")
	status, err := run(t, filepath.Join(elsewhere, "phone"), "get", "products")
	require.NoError(t, err)
	assert.Contains(t, status, "wcake-221")
}
