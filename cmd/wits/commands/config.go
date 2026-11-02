// Package commands holds `wits config` here: the repository's settings, read
// and set the way git config reads and sets its own.
package commands

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/TheDonDope/wits/pkg/repo"
	"github.com/TheDonDope/wits/pkg/tui"
)

var configUnset bool

// Config is the `wits config` command.
var Config = &cobra.Command{
	Use:   "config [<key> [<value>]]",
	Short: "Read or set the repository's settings, and its aliases",
	Long: "Read or set a setting of the repository, kept in .wits/config:\n\n" +
		"  theme        the colours of the interface, one of wits config --help's list\n" +
		"  lead         the days the pharmacy takes, which wits report refill counts\n" +
		"  remote       the folder push and pull share (set by wits remote add)\n" +
		"  alias.<name> a shorthand: `wits <name>` runs what it stands for\n\n" +
		"With no key, every setting. With a key, its value. With a value, it is set;\n" +
		"--unset takes an alias away. A command of wits's own always wins over an\n" +
		"alias of the same name.\n\n" +
		"Themes: " + strings.Join(tui.ThemeIDs(), ", ") + ".",
	Example: "  wits config\n" +
		"  wits config theme nord\n" +
		"  wits config alias.g grind\n" +
		"  wits config alias.evening \"sesh --device mighty --temp 185\"\n" +
		"  wits config --unset alias.g",
	Args: cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		switch {
		case configUnset:
			if len(args) != 1 || !strings.HasPrefix(args[0], "alias.") {
				return fmt.Errorf("--unset takes one alias away: wits config --unset alias.<name>")
			}
			return s.Repo.SetAlias(strings.TrimPrefix(args[0], "alias."), "")
		case len(args) == 0:
			listConfig(out, s.Repo.Config)
			return nil
		case len(args) == 1:
			v, ok := configValue(s.Repo.Config, args[0])
			if !ok {
				return fmt.Errorf("%s is not set", args[0])
			}
			fmt.Fprintln(out, v)
			return nil
		default:
			return setConfig(s.Repo, args[0], args[1])
		}
	},
}

func init() {
	Config.Flags().BoolVar(&configUnset, "unset", false, "take an alias away")
}

// configValue reads one key, and whether it is set.
func configValue(c repo.Config, key string) (string, bool) {
	switch key {
	case "theme":
		return c.Theme, c.Theme != ""
	case "lead":
		return strconv.Itoa(c.Lead()), true
	case "remote":
		return c.Remote, c.Remote != ""
	}
	if name, ok := strings.CutPrefix(key, "alias."); ok {
		v, set := c.Aliases[name]
		return v, set
	}
	return "", false
}

// setConfig sets one key, checking its value as the interface would.
func setConfig(r *repo.Repo, key, value string) error {
	switch key {
	case "theme":
		if !slices.Contains(tui.ThemeIDs(), value) {
			return fmt.Errorf("%q is not a theme; one of %s", value, strings.Join(tui.ThemeIDs(), ", "))
		}
		return r.SetTheme(value)
	case "lead":
		days, err := strconv.Atoi(value)
		if err != nil || days < 0 {
			return fmt.Errorf("a lead time is a number of days, not %q", value)
		}
		return r.SetLead(days)
	case "remote":
		return fmt.Errorf("a remote comes with its key: wits remote add <folder>")
	}
	name, ok := strings.CutPrefix(key, "alias.")
	if !ok || name == "" || strings.ContainsAny(name, " \t") {
		return fmt.Errorf("%q is not a setting; theme, lead, remote or alias.<name>", key)
	}
	if c, _, err := Root.Find([]string{name}); err == nil && c != Root {
		return fmt.Errorf("%s is a command of wits's own, which an alias cannot hide", name)
	}
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("an alias stands for a command; --unset takes one away")
	}
	return r.SetAlias(name, value)
}

// listConfig writes every setting that is set, as key=value lines.
func listConfig(out io.Writer, c repo.Config) {
	for _, key := range []string{"theme", "lead", "remote"} {
		if v, ok := configValue(c, key); ok {
			fmt.Fprintf(out, "%s=%s\n", key, v)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(c.Aliases)) {
		fmt.Fprintf(out, "alias.%s=%s\n", name, c.Aliases[name])
	}
}
