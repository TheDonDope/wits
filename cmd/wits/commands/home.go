package commands

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/TheDonDope/wits/pkg/tui"
)

// Home launches the terminal interface. It is what `wits` on its own runs.
var Home = &cobra.Command{
	Use:   "home",
	Short: "Launch the interface",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		// The same way every other command finds a repository, rather than a
		// second copy of the discovery this one used to carry.
		s, err := open()
		if err != nil {
			return err
		}
		if _, err := tea.NewProgram(tui.New(tui.From(s.Workspace))).Run(); err != nil {
			return fmt.Errorf("running the interface: %w", err)
		}
		return nil
	},
}
