package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// plugin writes a program named wits-<name> into dir.
func writePlugin(t *testing.T, dir, name, script string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, pluginPrefix+name), []byte("#!/bin/sh\n"+script+"\n"), 0o755))
}

// TestPlugins: a program wits-<name> on the PATH runs as `wits <name>`, in
// the directory -C names, told where wits is, and its status is wits's own.
// A command of wits's own always wins over one.
func TestPlugins(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the plugins here are shell scripts")
	}
	bin, repo := t.TempDir(), stockedRepo(t)
	writePlugin(t, bin, "hello", `echo "$@ in $WITS_DIR from $(pwd), wits at $WITS"; exit 3`)
	writePlugin(t, bin, "get", `echo should never run`)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	var out bytes.Buffer
	Root.SetOut(&out)
	Root.SetErr(&out)
	err := Execute(context.Background(), []string{"-C", repo, "hello", "a", "--b"})
	assert.Equal(t, Exit{Code: 3}, err, "the plugin's status is wits's own")
	self, _ := os.Executable()
	assert.Equal(t, "a --b in "+repo+" from "+repo+", wits at "+self+"\n", out.String())

	err = Execute(context.Background(), []string{"--dry-run", "hello"})
	assert.ErrorContains(t, err, "--dry-run is for wits's own commands")

	got, err := run(t, repo, "get", "plugins", "-o", "wide")
	require.NoError(t, err)
	assert.Contains(t, got, "hello")
	assert.Regexp(t, `get\s+\S+wits-get\s+wits get is wits's own`, got, "a plugin named like a command never runs")
	assert.NotContains(t, got, "should never run")

	_, err = run(t, repo, "rate", "session", "overall=4", "sleep=5")
	require.NoError(t, err)
	got, err = run(t, repo, "get", "sessions", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, got, `"scores": {`, "a plugin reads the rating as numbers")
	assert.Contains(t, got, `"sleep": 5`)
}
