// Package main is the entry point for the Wits TUI application.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/TheDonDope/wits/cmd/wits/commands"
	"github.com/TheDonDope/wits/pkg/repo"
	"github.com/TheDonDope/wits/pkg/version"
)

var rootCmd = &cobra.Command{
	Use:          "wits",
	Short:        "A tui for cannabis patients and users",
	Long:         "Wits is the weed information tracking system, aimed to help cannabis patients and users.",
	Version:      version.String(),
	SilenceUsage: true,
	// main reports the error itself, so cobra should not print it too.
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return commands.Home.RunE(cmd, args)
	},
}

func init() {
	// The completion command stays visible: tab completion offers the product
	// slugs, which is most of what makes short slugs worth having.
	rootCmd.AddCommand(
		commands.Init,
		commands.Bundle,
		commands.Buy,
		commands.Grind,
		commands.Import,
		commands.Sesh,
		commands.Device,
		commands.Temps,
		commands.Status,
		commands.Log,
		commands.Reconcile,
		commands.Restore,
		commands.Revert,
		commands.Export,
	)
}

func main() {
	ctx := context.Background()
	closeLog := openLog()
	defer closeLog()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "🚨 %v\n", err)
		os.Exit(1)
	}
}

// openLog sends log output to a file inside the repository when there is one,
// so that log lines never scribble over the TUI. Without a repository, logging
// is discarded rather than dumped on the terminal.
func openLog() func() {
	wd, err := os.Getwd()
	if err != nil {
		log.SetOutput(io.Discard)
		return func() {}
	}
	r, err := repo.Discover(wd)
	if err != nil {
		log.SetOutput(io.Discard)
		return func() {}
	}
	f, err := tea.LogToFile(filepath.Join(r.Root(), r.Config.LogFile), "debug")
	if err != nil {
		log.SetOutput(io.Discard)
		return func() {}
	}
	return func() { f.Close() }
}
