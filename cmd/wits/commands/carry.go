// Package commands holds `wits carry` here: a signed statement of what is held.
package commands

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/report"
	"github.com/TheDonDope/wits/pkg/seal"
	"github.com/spf13/cobra"
	"rsc.io/qr"
)

var (
	carryAll bool
	carryOut string
)

// Carry is the `wits carry` command.
var Carry = &cobra.Command{
	Use:   "carry",
	Short: "Sign what is held, as a QR code that checks it against what was dispensed",
	Long: "List what is held now — each product sealed, ground and already vaped,\n" +
		"and the fills that dispensed it — and sign it with the sealing key, as a\n" +
		"statement that names the journal's head. Its QR code opens a page that\n" +
		"checks the signature in the browser: the statement rides after the #, which\n" +
		"a browser never sends to the server it asks for the page.\n\n" +
		"Only the products on the shelf unless --all says every jar holding anything.\n" +
		"--out writes the QR code as .svg or .png, or the statement as .txt.",
	Example: "  wits carry --out carry.svg\n  wits verify carry.txt",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		c, err := carryStatement(cmd, s)
		if err != nil {
			return err
		}
		writeCarry(cmd.OutOrStdout(), c)
		if carryOut == "" {
			return nil
		}
		data, err := carryFile(c, carryOut)
		if err != nil {
			return err
		}
		if err := os.WriteFile(carryOut, data, 0600); err != nil {
			return err
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Wrote %s\n", carryOut)
		return nil
	},
}

// carryStatement signs what is held now.
func carryStatement(cmd *cobra.Command, s *session) (seal.Carry, error) {
	items := report.Holdings(s.Workspace, carryAll)
	if len(items) == 0 {
		return seal.Carry{}, errors.New("nothing is held: there is nothing to show")
	}
	events, err := s.Journal().Events()
	if err != nil {
		return seal.Carry{}, err
	}
	key, made, err := seal.Key(s.Repo.SealKeyPath())
	if err != nil {
		return seal.Carry{}, err
	}
	if made {
		fmt.Fprintf(cmd.ErrOrStderr(), "Made a sealing key in %s; its fingerprint is %s\n",
			s.Repo.SealKeyPath(), seal.Fingerprint(key.Public().(ed25519.PublicKey)))
	}
	return seal.MakeCarry(key, events[len(events)-1].Hash, len(events), s.OpenedAt, items), nil
}

// writeCarry lists a statement's holdings.
func writeCarry(out io.Writer, c seal.Carry) {
	fmt.Fprintf(out, "Held on %s, by the journal's %s up to %s:\n\n", c.At.Local().Format("2 January 2006, 15:04"),
		entries(c.Entries), shortHash(c.Head))
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PRODUCT\tSEALED\tGROUND\tAVB\tDISPENSED")
	var total journal.Centigrams
	for _, it := range c.Items {
		var fills []string
		for _, f := range it.Fills {
			fills = append(fills, fmt.Sprintf("%.2fg on %s", f.Grams, f.Day))
		}
		fmt.Fprintf(w, "%s\t%.2fg\t%.2fg\t%.2fg\t%s\n", it.Name, it.Sealed, it.Ground, it.AVB, strings.Join(fills, ", "))
		total += journal.CG(it.Sealed) + journal.CG(it.Ground) + journal.CG(it.AVB)
	}
	w.Flush()
	fmt.Fprintf(out, "\n%.2fg in all. Signed by %s; the QR code opens it at %s\n", total.Grams(),
		seal.Fingerprint(c.Key), strings.TrimSuffix(seal.VerifyPage, "#"))
}

// carryFile is the statement as the file asked for: its QR code, or its text.
func carryFile(c seal.Carry, path string) ([]byte, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".svg":
		svg, err := report.QR(c.URL())
		return []byte(svg), err
	case ".png":
		code, err := qr.Encode(c.URL(), qr.L)
		if err != nil {
			return nil, err
		}
		return code.PNG(), nil
	case ".txt":
		return []byte(c.String()), nil
	}
	return nil, fmt.Errorf("%s: write the QR code as .svg or .png, or the statement as .txt", path)
}

func init() {
	Carry.Flags().BoolVar(&carryAll, "all", false, "every jar holding anything, not only those on the shelf")
	Carry.Flags().StringVar(&carryOut, "out", "", "write the QR code (.svg, .png) or the statement (.txt)")
}
