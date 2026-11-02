// Package main is the entry point for wits: the command line, and on its own
// the terminal interface.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/TheDonDope/wits/cmd/wits/commands"
)

func main() {
	// A library's stray log line must never draw over the interface.
	log.SetOutput(io.Discard)
	if err := commands.Execute(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "🚨 %v\n", err)
		os.Exit(1)
	}
}
