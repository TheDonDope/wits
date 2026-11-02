// Package commands holds the root of the command line here: the tree, the
// flags every command takes, and the aliases a repository keeps.
package commands

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/TheDonDope/wits/pkg/repo"
	"github.com/TheDonDope/wits/pkg/version"
)

var (
	chdir  string
	dryRun bool
)

// rehearsable marks a command whose writes go through the recorder, which
// can build and check them without writing: the only kind --dry-run means
// anything for. A command without it refuses the flag rather than write.
func rehearsable() map[string]string {
	return map[string]string{"dry-run": "rehearsed by the recorder"}
}

// The groups the help lists the commands under: the journal is written the
// way git commits, and everything else is looked at the way kubectl does it.
const (
	groupJournal = "journal"
	groupLook    = "look"
	groupRepo    = "repo"
	groupTrust   = "trust"
)

// Root is `wits`: on its own the terminal interface, with a command the
// command line.
var Root = &cobra.Command{
	Use:   "wits",
	Short: "The weed information tracking system",
	Long: "Wits is the weed information tracking system, aimed to help cannabis patients and users.\n\n" +
		"The journal is written the way git commits: short verbs, run as it happens\n" +
		"(buy, grind, sesh, weigh). Everything else is read the way kubectl reads a\n" +
		"cluster: get lists, describe tells one in full, edit, create and delete\n" +
		"change the catalogs. On its own, wits opens the terminal interface.",
	Version: version.String(),
	Args:    cobra.NoArgs,
	// main reports the error itself, so cobra should not print it too, and
	// a refused entry is not a reason to print the usage.
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		if chdir != "" {
			if err := os.Chdir(chdir); err != nil {
				return fmt.Errorf("-C: %w", err)
			}
		}
		if dryRun && cmd.Annotations["dry-run"] == "" {
			return fmt.Errorf("%s cannot be rehearsed; --dry-run is for what the recorder writes", cmd.CommandPath())
		}
		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, _ []string) {
		if dryRun {
			fmt.Fprintln(cmd.ErrOrStderr(), "Dry run: nothing was written.")
		}
	},
	RunE: func(cmd *cobra.Command, args []string) error { return Home.RunE(cmd, args) },
}

func init() {
	Root.PersistentFlags().StringVarP(&chdir, "directory", "C", "", "run as if wits was started in this directory")
	// Long only: -n is log's count, as it is git's.
	Root.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "build and check the entry, and write nothing")
	// In the order a day goes, not the alphabet's.
	cobra.EnableCommandSorting = false
	Root.AddGroup(
		&cobra.Group{ID: groupJournal, Title: "The journal, written as it happens:"},
		&cobra.Group{ID: groupLook, Title: "Looking, and the catalogs:"},
		&cobra.Group{ID: groupRepo, Title: "The repository:"},
		&cobra.Group{ID: groupTrust, Title: "Proof of what is held:"},
	)
	add := func(group string, cmds ...*cobra.Command) {
		for _, c := range cmds {
			c.GroupID = group
			Root.AddCommand(c)
		}
	}
	add(groupJournal, Buy, Grind, Sesh, Weigh, AVB, Price, Status, Log, Show, Revert, Amend)
	add(groupLook, Get, Describe, Edit, Create, Delete, Rate, Report)
	add(groupRepo, Init, Clone, Remote, Push, Pull, Fsck, Bundle, Import, Export, Config)
	add(groupTrust, Seal, Verify, Carry, Pack)
	// The completion command stays: tab completion offers the product slugs,
	// which is most of what makes short slugs worth having.
	Root.SetCompletionCommandGroupID(groupRepo)
	Root.SetHelpCommandGroupID(groupRepo)
}

// Execute runs the command line on args, an alias of the repository's
// expanded first.
func Execute(ctx context.Context, args []string) error {
	Root.SetArgs(expand(args))
	return Root.ExecuteContext(ctx)
}

// expand replaces the command an alias names with what the alias stands
// for, the way git expands its own: `wits config alias.g "grind"` makes
// `wits g wedding 1` a grind. A command of wits's own always wins, so an
// alias can never hide one.
func expand(args []string) []string {
	at, dir := commandAt(args)
	if at < 0 {
		return args
	}
	if c, _, err := Root.Find(args[at : at+1]); err == nil && c != Root {
		return args
	}
	r, err := repo.Discover(dir)
	if err != nil {
		return args
	}
	stands, ok := r.Config.Aliases[args[at]]
	if !ok {
		return args
	}
	return slices.Concat(args[:at], strings.Fields(stands), args[at+1:])
}

// commandAt is where the command stands among the arguments, past the
// global flags, and the directory -C names, if it names one.
func commandAt(args []string) (int, string) {
	dir := "."
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case (a == "-C" || a == "--directory") && i+1 < len(args):
			dir = args[i+1]
			i++
		case strings.HasPrefix(a, "--directory="):
			dir = strings.TrimPrefix(a, "--directory=")
		case strings.HasPrefix(a, "-C") && len(a) > 2:
			dir = a[2:]
		case strings.HasPrefix(a, "-"):
		default:
			return i, dir
		}
	}
	return -1, dir
}
