// Package main is the entry point for wits: the command line, and on its own
// the terminal interface.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/TheDonDope/wits/cmd/wits/commands"
)

func main() {
	// A library's stray log line must never draw over the interface.
	log.SetOutput(io.Discard)
	err := commands.Execute(context.Background(), os.Args[1:])
	// A plugin has said what it had to; its status is wits's own.
	var exit commands.Exit
	if errors.As(err, &exit) {
		os.Exit(exit.Code)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "🚨 %v\n", err)
		os.Exit(1)
	}
}
