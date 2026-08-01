package commands

import (
	"fmt"

	"github.com/TheDonDope/wits/pkg/repo"
	"github.com/spf13/cobra"
)

// Init is the `wits init` command.
var Init = &cobra.Command{
	Use:   "init [directory]",
	Short: "Create an empty Wits repository",
	Long: "Create an empty Wits repository, the way `git init` creates one.\n\n" +
		"This makes a .wits directory holding your configuration, your product\n" +
		"and device catalogs, and the append-only journal every command writes to.",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) == 1 {
			dir = args[0]
		}
		r, err := repo.Init(dir)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Initialised empty Wits repository in %s\n", r.Root())
		return nil
	},
}
