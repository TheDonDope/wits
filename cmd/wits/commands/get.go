// Package commands holds the kubectl half of the command line here: get,
// describe, edit, create and delete, each taking the kind of thing first,
// and the listing every get prints, as a table, a wider one, or JSON.
package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	getOutput   string
	getSelector string
	getAll      bool
	getWatch    bool
)

// Get is the `wits get` command: a list of one kind of thing.
var Get = &cobra.Command{
	Use:   "get <kind>",
	Short: "List one kind of thing: products, devices, fills, sessions, entries",
	Long: "List one kind of thing, the way kubectl get lists a kind of resource:\n" +
		"products (the jars), devices, fills, sessions, entries or plugins. -o\n" +
		"wide adds columns, -o json prints every field for a script, as\n" +
		"{\"apiVersion\": \"wits/v1\", \"kind\": \"ProductList\", \"items\": [...]}. -l\n" +
		"keeps the rows whose fields match, by the names -o json prints: -l\n" +
		"maker=enua, or several, by commas. --watch prints the list again each\n" +
		"time the repository changes; with -o json, a list a line.",
	Example: "  wits get products\n" +
		"  wits get products --all -o wide\n" +
		"  wits get devices -l type=vaporizer\n" +
		"  wits get sessions -l device=mighty -o json\n" +
		"  wits get fills --watch",
	Args: cobra.NoArgs,
}

// Describe is the `wits describe` command: one thing, in full.
var Describe = &cobra.Command{
	Use:   "describe <kind> <name>",
	Short: "Tell one thing in full: a product, a device, a fill, a temperature",
	Long: "Tell one thing in full, the way kubectl describe does. -o json prints\n" +
		"it for a script, its version and kind first: {\"apiVersion\":\n" +
		"\"wits/v1\", \"kind\": \"Product\", ...}.",
	Example: "  wits describe product wedding\n" +
		"  wits describe fill -o json",
	Args: cobra.NoArgs,
}

// Edit is the `wits edit` command: a catalog entry changed in place, its
// slug kept, since every entry names it by that.
var Edit = &cobra.Command{
	Use:   "edit <kind> <name>",
	Short: "Change a product or a device, as far as the flags say",
	Args:  cobra.NoArgs,
}

// Create is the `wits create` command. A product comes into the catalog
// with its first purchase (wits buy), so only a device is created.
var Create = &cobra.Command{
	Use:   "create <kind> <name>",
	Short: "Add a device to the catalog",
	Args:  cobra.NoArgs,
}

// Delete is the `wits delete` command. Only a device leaves the catalog:
// a product's entries are its history, and the journal keeps all of it.
var Delete = &cobra.Command{
	Use:   "delete <kind> <name>",
	Short: "Take a device out of the catalog",
	Args:  cobra.NoArgs,
}

func init() {
	Get.PersistentFlags().StringVarP(&getOutput, "output", "o", "table", "table, wide or json")
	Get.PersistentFlags().StringVarP(&getSelector, "selector", "l", "", "keep the rows whose fields match, as key=value,key=value")
	Get.PersistentFlags().BoolVarP(&getAll, "all", "A", false, "everything, not only what is held or open")
	Get.PersistentFlags().BoolVarP(&getWatch, "watch", "w", false, "print it again each time the repository changes")
	Describe.PersistentFlags().StringVarP(&describeOutput, "output", "o", "text", "text or json")
}

// listing is how one kind of thing is printed: its kind, as -o json names
// it, its columns, those -o wide adds, and the cells of a row for each.
type listing[T any] struct {
	kind       string
	head, wide []string
	cells      func(T) (narrow, wide []string)
}

// list prints rows as the flags ask, after the selector has kept those it
// matches.
func list[T any](out io.Writer, rows []T, l listing[T]) error {
	kept, err := selected(rows)
	if err != nil {
		return err
	}
	switch getOutput {
	case "json":
		return writeList(out, l.kind, kept, getWatch)
	case "table", "wide":
	default:
		return fmt.Errorf("-o %s: table, wide or json", getOutput)
	}
	if len(kept) == 0 {
		fmt.Fprintln(out, "Nothing to list.")
		return nil
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	head := l.head
	if getOutput == "wide" {
		head = append(head[:len(head):len(head)], l.wide...)
	}
	fmt.Fprintln(w, strings.Join(head, "\t"))
	for _, r := range kept {
		cells, more := l.cells(r)
		if getOutput == "wide" {
			cells = append(cells, more...)
		}
		fmt.Fprintln(w, strings.Join(cells, "\t"))
	}
	return w.Flush()
}

// selected keeps the rows whose fields, by their JSON names, match every
// key=value of the selector, without regard to case. A list matches when
// it holds the value.
func selected[T any](rows []T) ([]T, error) {
	if getSelector == "" {
		return rows, nil
	}
	want := map[string]string{}
	for _, pair := range strings.Split(getSelector, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(pair), "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("-l %q: write it as key=value", pair)
		}
		want[k] = v
	}
	var kept []T
	for _, r := range rows {
		ok, err := matches(r, want)
		if err != nil {
			return nil, err
		}
		if ok {
			kept = append(kept, r)
		}
	}
	return kept, nil
}

// matches reports whether one row's fields hold every value wanted.
func matches(row any, want map[string]string) (bool, error) {
	raw, err := json.Marshal(row)
	if err != nil {
		return false, err
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		return false, err
	}
	for k, v := range want {
		if !holds(fields[k], v) {
			return false, nil
		}
	}
	return true, nil
}

// holds compares one field with the value asked for.
func holds(field any, v string) bool {
	if many, ok := field.([]any); ok {
		for _, one := range many {
			if holds(one, v) {
				return true
			}
		}
		return false
	}
	return field != nil && strings.EqualFold(fmt.Sprint(field), v)
}

// orDash is a cell for a value that may be missing.
func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

// gramsCell is a cell of grams.
func gramsCell(g float64) string { return fmt.Sprintf("%.2f g", g) }
