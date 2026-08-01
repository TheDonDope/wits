package home

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/TheDonDope/wits-tui/pkg/repo"
	"github.com/TheDonDope/wits-tui/pkg/tui"
)

// Command launches the terminal interface.
var Command = &cobra.Command{
	Use:   "home",
	Short: "Launch the interface",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		r, err := repo.Discover(wd)
		if err != nil {
			return err
		}
		data, err := tui.Load(r)
		if err != nil {
			return err
		}
		if _, err := tea.NewProgram(tui.New(data)).Run(); err != nil {
			return fmt.Errorf("running the interface: %w", err)
		}
		return nil
	},
}
