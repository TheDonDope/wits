// Package main is the entry point for the Wits TUI application.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/TheDonDope/wits-tui/cmd/wits/commands"
	"github.com/TheDonDope/wits-tui/cmd/wits/home"
	"github.com/TheDonDope/wits-tui/pkg/repo"
	"github.com/TheDonDope/wits-tui/pkg/version"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	// Version contains the application version number. It's set via ldflags
	// when building.
	Version = ""

	// CommitSHA contains the SHA of the commit that this application was built
	// against. It's set via ldflags when building.
	CommitSHA = ""

	// CommitDate contains the date of the commit that this application was
	// built against. It's set via ldflags when building.
	CommitDate = ""

	rootCmd = &cobra.Command{
		Use:          "wits",
		Short:        "A tui for cannabis patients and users",
		Long:         "Wits is the weed information tracking system, aimed to help cannabis patients and users.",
		SilenceUsage: true,
		// main reports the error itself, so cobra should not print it too.
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return home.Command.RunE(cmd, args)
		},
	}
)

func init() {
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
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
		commands.Restore,
		commands.Revert,
		commands.Export,
	)

	if len(CommitSHA) >= 7 {
		vt := rootCmd.VersionTemplate()
		rootCmd.SetVersionTemplate(vt[:len(vt)-1] + " (" + CommitSHA[0:7] + ")\n")
	}
	if Version == "" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Sum != "" {
			Version = info.Main.Version
		} else {
			Version = "unknown (built from source)"
		}
	}
	rootCmd.Version = Version

	version.Version = Version
	version.CommitSHA = CommitSHA
	version.CommitDate = CommitDate
}

func main() {
	ctx := context.Background()
	closeLog := openLog()
	defer closeLog()
	log.Println("🚀 🖥️  (cmd/wits/main.go) main()")
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
