// Package main is the entry point for the Wits TUI application.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/TheDonDope/wits/cmd/wits/commands"
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
		commands.Price,
		commands.Costs,
		commands.Grind,
		commands.Import,
		commands.Sesh,
		commands.AVB,
		commands.Rate,
		commands.Device,
		commands.Temps,
		commands.Status,
		commands.Refill,
		commands.Log,
		commands.Show,
		commands.Reconcile,
		commands.Restore,
		commands.Revert,
		commands.Export,
		commands.Report,
		commands.Fsck,
		commands.Remote,
		commands.Push,
		commands.Pull,
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

// openLog discards log output, so a library's stray log line can never draw
// over the interface. It used to send it to wits.log in the repository, which
// every launch opened and nothing ever wrote to.
func openLog() func() {
	log.SetOutput(io.Discard)
	return func() {}
}
