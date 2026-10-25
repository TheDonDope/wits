// Package commands holds `wits verify` here: a seal or a carry statement checked.
package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/TheDonDope/wits/pkg/seal"
	"github.com/spf13/cobra"
)

// Verify is the `wits verify` command.
var Verify = &cobra.Command{
	Use:   "verify <file.seal | statement | address>",
	Short: "Check a seal or a statement of what is carried, against the journal",
	Long: "Check that a seal's signature is over its lines, made with the key it\n" +
		"names; that the journal here holds the head it seals, at the same place;\n" +
		"and, when its .ots proof is beside it, what that proof attests. Run outside\n" +
		"a repository, only the seal itself and its proof are checked. A statement\n" +
		"from `wits carry` — its .txt, or the address its QR code opens — is checked\n" +
		"the same way.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		text, err := os.ReadFile(args[0])
		if os.IsNotExist(err) && strings.Contains(args[0], "#") {
			text, err = []byte(args[0]), nil // the verify page's address, as the QR code carries it
		}
		if err != nil {
			return err
		}
		if strings.Contains(string(text), "wits-carry") || strings.Contains(string(text), "#") {
			return verifyCarry(cmd.OutOrStdout(), string(text))
		}
		f, err := seal.Parse(string(text))
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		if err := f.Verify(); err != nil {
			return err
		}
		fmt.Fprintf(out, "✓ signed by %s on %s\n", seal.Fingerprint(f.Key), f.At.Local().Format("2 January 2006, 15:04"))
		if err := verifyHead(out, f); err != nil {
			return err
		}
		return verifyProof(out, f, strings.TrimSuffix(args[0], ".seal")+".ots")
	},
}

// verifyCarry checks a statement of what is carried, and lists it.
func verifyCarry(out io.Writer, text string) error {
	c, err := seal.ParseCarry(strings.TrimSpace(text))
	if err != nil {
		return err
	}
	if err := c.Verify(); err != nil {
		return err
	}
	fmt.Fprintf(out, "✓ signed by %s\n", seal.Fingerprint(c.Key))
	writeCarry(out, c)
	return verifyHead(out, seal.Seal{Head: c.Head, Entries: c.Entries})
}

// verifyHead says whether the journal here holds the sealed head where the
// seal says it is.
func verifyHead(out io.Writer, f seal.Seal) error {
	s, err := open()
	if err != nil {
		fmt.Fprintf(out, "· not in a repository: the head %s, after %s, is not checked\n", shortHash(f.Head), entries(f.Entries))
		return nil
	}
	events, err := s.Journal().Events()
	if err != nil {
		return err
	}
	if f.Entries < 1 || f.Entries > len(events) || events[f.Entries-1].Hash != f.Head {
		return fmt.Errorf("the journal here does not hold the sealed head %s at entry %d", shortHash(f.Head), f.Entries)
	}
	fmt.Fprintf(out, "✓ the journal here holds what was sealed: %s, up to %s\n", entries(f.Entries), shortHash(f.Head))
	return nil
}

// verifyProof says what the seal's timestamp attests, when it has one.
func verifyProof(out io.Writer, f seal.Seal, path string) error {
	ots, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		fmt.Fprintln(out, "· no timestamp beside it: `wits seal --timestamp` makes one")
		return nil
	}
	if err != nil {
		return err
	}
	p, err := seal.Describe(ots)
	if err != nil {
		return err
	}
	if d := f.Digest(); !bytes.Equal(p.Digest, d[:]) {
		return errors.New("the timestamp is of something else, not this seal")
	}
	if len(p.Blocks) > 0 {
		fmt.Fprintf(out, "✓ timestamped in Bitcoin block %d; `ots verify` or opentimestamps.org check it against the chain\n", p.Blocks[0])
		return nil
	}
	fmt.Fprintf(out, "· timestamp pending at %s; `wits seal --upgrade` asks for it again\n", plural(len(p.Pending), "calendar"))
	return nil
}
