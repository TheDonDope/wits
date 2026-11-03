// Package commands holds the shape of what a script reads here: every list
// and every thing described, printed as JSON under a version and a kind, the
// way kubectl prints a resource, and a list printed again each time the
// repository changes.
package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"

	"github.com/TheDonDope/wits/pkg/repo"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// APIVersion names the shapes -o json prints. Within a version a field is
// only ever added; one renamed, retyped or taken away is the next version,
// so a script that checks it knows what it reads. TestShapes pins every
// field of every kind.
const APIVersion = "wits/v1"

// listOf is a list as -o json prints it: the version, the kind of list, and
// its items.
type listOf[T any] struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Items      []T    `json:"items"`
}

// writeJSON prints v indented, or on one line when it is one of a stream,
// as a list watched is, so a script can read it a line at a time.
func writeJSON(out io.Writer, v any, oneLine bool) error {
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	if !oneLine {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(v)
}

// writeList prints items as the kind of list they are.
func writeList[T any](out io.Writer, kind string, items []T, oneLine bool) error {
	if items == nil {
		items = []T{}
	}
	return writeJSON(out, listOf[T]{APIVersion: APIVersion, Kind: kind + "List", Items: items}, oneLine)
}

// writeObject prints one thing, its version and kind first and then its own
// fields, as kubectl prints one resource.
func writeObject(out io.Writer, kind string, v any) error {
	own, err := compact(v)
	if err != nil {
		return err
	}
	if len(own) < 2 || own[0] != '{' {
		return fmt.Errorf("%s is not an object", kind)
	}
	head, err := compact(struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
	}{APIVersion, kind})
	if err != nil {
		return err
	}
	joined := head[:len(head)-1]
	if len(own) > 2 {
		joined = append(append(joined, ','), own[1:]...)
	} else {
		joined = append(joined, '}')
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, joined, "", "  "); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, indented.String())
	return err
}

// compact is v as JSON on one line, an & left an &.
func compact(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(b.Bytes()), nil
}

// describeOutput is how describe prints: as text, or as JSON for a script.
var describeOutput string

// asJSON reports whether describe was asked for JSON, refusing anything
// but text or json.
func asJSON() (bool, error) {
	switch describeOutput {
	case "json":
		return true, nil
	case "text", "":
		return false, nil
	}
	return false, fmt.Errorf("-o %s: text or json", describeOutput)
}

// How often a watched list looks at the repository's files: as often as the
// server's change stream does.
var watchEvery = time.Second

// watched is a get's RunE that, with --watch, runs again each time the
// repository changes, until it is interrupted.
func watched(run func(*cobra.Command, []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if !getWatch {
			return run(cmd, args)
		}
		r, err := repo.Discover(".")
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
		defer stop()
		return watch(ctx, func() string { return workspace.Stamp(r) }, func(again bool) error {
			if again && getOutput != "json" {
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return run(cmd, args)
		})
	}
}

// watch prints once, then again each time stamp says something changed,
// until ctx is done.
func watch(ctx context.Context, stamp func() string, show func(again bool) error) error {
	look := time.NewTicker(watchEvery)
	defer look.Stop()
	last := stamp()
	if err := show(false); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-look.C:
		}
		if now := stamp(); now != last {
			last = now
			if err := show(true); err != nil {
				return err
			}
		}
	}
}
