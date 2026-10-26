// Command wits-types writes the shapes the API answers in as TypeScript, for
// the browser's models to be checked against: `make types`.
package main

import (
	"fmt"
	"os"

	"github.com/TheDonDope/wits/pkg/api"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: wits-types <file.ts>")
		os.Exit(2)
	}
	if err := os.WriteFile(os.Args[1], []byte(api.TypeScript()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
