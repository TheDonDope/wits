// Package commands holds the plugins here: a program named wits-<name> on
// the PATH runs as `wits <name>`, the way git and kubectl find theirs, and
// `wits get plugins` lists the ones it would find.
package commands

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

// pluginPrefix names a plugin: wits-heatmap is `wits heatmap`.
const pluginPrefix = "wits-"

// Exit is a plugin's exit status, handed on as wits's own: main exits with
// it and prints nothing, since the plugin has said what it had to say.
type Exit struct{ Code int }

func (e Exit) Error() string { return fmt.Sprintf("exit status %d", e.Code) }

// plugin finds the program a command line names, when it names no command
// of wits's own: where it stands among the arguments, the directory -C
// gives, and its path. ok is false when there is none to run.
func plugin(args []string) (at int, dir, path string, ok bool) {
	at, dir = commandAt(args)
	if at < 0 || builtin(args[at]) {
		return 0, "", "", false
	}
	path, err := exec.LookPath(pluginPrefix + args[at])
	if err != nil {
		return 0, "", "", false
	}
	return at, dir, path, true
}

// builtin reports whether a name is one of wits's own commands, or an
// alias of one: those always win over a plugin of the same name.
func builtin(name string) bool {
	c, _, err := Root.Find([]string{name})
	return err == nil && c != Root
}

// runPlugin runs a plugin in the directory -C named, with the arguments
// after its name. It is told where wits is (WITS) and the repository's
// directory (WITS_DIR), so the wits it calls back is this one.
func runPlugin(ctx context.Context, args []string, at int, dir, path string) error {
	if slices.ContainsFunc(args[:at], func(a string) bool { return strings.HasPrefix(a, "--dry-run") }) {
		return fmt.Errorf("--dry-run is for wits's own commands; %s%s has its own flags", pluginPrefix, args[at])
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	c := exec.CommandContext(ctx, path, args[at+1:]...)
	c.Dir = abs
	c.Env = append(os.Environ(), "WITS="+self, "WITS_DIR="+abs)
	c.Stdin, c.Stdout, c.Stderr = Root.InOrStdin(), Root.OutOrStdout(), Root.ErrOrStderr()
	var exit *exec.ExitError
	if err := c.Run(); errors.As(err, &exit) {
		return Exit{Code: exit.ExitCode()}
	} else if err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return nil
}

// pluginRow is one plugin as get lists it.
type pluginRow struct {
	Name string `json:"name"`
	Path string `json:"path"`
	// Hidden says why the plugin never runs: a command of wits's own has
	// its name, or a plugin earlier on the PATH does.
	Hidden string `json:"hidden,omitempty"`
}

var getPlugins = &cobra.Command{
	Use:     "plugins",
	Aliases: []string{"plugin"},
	Short:   "The programs named wits-<name> on your PATH, which run as wits <name>",
	Long: "Every program named wits-<name> on the PATH, in the order the PATH finds\n" +
		"them. Each runs as `wits <name>`, told where wits is ($WITS) and which\n" +
		"directory -C named ($WITS_DIR). One that a command of wits's own, or an\n" +
		"earlier plugin, has the name of never runs; -o wide says which.",
	Example: "  wits get plugins\n" +
		"  wits heatmap --weeks 26",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return list(cmd.OutOrStdout(), plugins(filepath.SplitList(os.Getenv("PATH"))), listing[pluginRow]{
			head: []string{"NAME", "PATH"},
			wide: []string{"HIDDEN"},
			cells: func(r pluginRow) ([]string, []string) {
				return []string{r.Name, r.Path}, []string{orDash(r.Hidden)}
			},
		})
	},
}

// plugins reads the directories of a PATH, in its order, for programs
// named as plugins.
func plugins(dirs []string) []pluginRow {
	var rows []pluginRow
	seen, read := map[string]bool{}, map[string]bool{}
	for _, d := range dirs {
		// A directory named twice on the PATH is read once.
		if read[filepath.Clean(d)] {
			continue
		}
		read[filepath.Clean(d)] = true
		entries, err := os.ReadDir(cmp.Or(d, "."))
		if err != nil {
			continue
		}
		for _, e := range entries {
			name, ok := pluginName(e)
			if !ok {
				continue
			}
			r := pluginRow{Name: name, Path: filepath.Join(d, e.Name())}
			switch {
			case builtin(name):
				r.Hidden = "wits " + name + " is wits's own"
			case seen[name]:
				r.Hidden = "an earlier one on the PATH"
			}
			seen[name] = true
			rows = append(rows, r)
		}
	}
	return rows
}

// pluginName is the command a directory entry would run as, if it is a
// program named as a plugin.
func pluginName(e os.DirEntry) (string, bool) {
	name, ok := strings.CutPrefix(e.Name(), pluginPrefix)
	if !ok || name == "" || e.IsDir() {
		return "", false
	}
	if runtime.GOOS == "windows" {
		return strings.TrimSuffix(name, filepath.Ext(name)), strings.EqualFold(filepath.Ext(name), ".exe")
	}
	info, err := e.Info()
	return name, err == nil && info.Mode()&0o111 != 0
}

func init() {
	Get.AddCommand(getPlugins)
}
