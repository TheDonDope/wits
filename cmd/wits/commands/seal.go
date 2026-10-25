// Package commands holds `wits seal` here: the journal's head signed, and timestamped on request.
package commands

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/TheDonDope/wits/pkg/seal"
	"github.com/spf13/cobra"
)

var (
	sealTimestamp bool
	sealUpgrade   bool
	sealKey       bool
)

// Seal is the `wits seal` command.
var Seal = &cobra.Command{
	Use:   "seal",
	Short: "Sign the journal's head, so others can check what it held",
	Long: "Sign the hash of the journal's last entry with a key of your own. The hash\n" +
		"covers every entry before it, so a seal fixes the whole journal up to that\n" +
		"point. The key is made the first time, in .wits/seal.key; hand out its\n" +
		"fingerprint (--key), never the file.\n\n" +
		"--timestamp also sends the seal's SHA-256 — that hash and nothing else —\n" +
		"to the public OpenTimestamps calendars, which commit it into Bitcoin within\n" +
		"a few hours. --upgrade asks them for the finished proof. `ots verify` and\n" +
		"opentimestamps.org check the .ots file without wits.",
	Example: "  wits seal --timestamp\n  wits seal --upgrade\n  wits verify .wits/seals/2026-10-24-f81bbd2cc893.seal",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		switch {
		case sealKey:
			return showSealKey(cmd.OutOrStdout(), s)
		case sealUpgrade:
			return upgradeSeals(cmd, s)
		}
		f, made, err := sealHead(cmd, s)
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		if made {
			fmt.Fprintf(out, "Sealed the journal's %s, head %s, as %s.seal\n", entries(f.Entries), shortHash(f.Head), f.Name())
		} else {
			fmt.Fprintf(out, "The head %s is sealed already, as %s.seal\n", shortHash(f.Head), f.Name())
		}
		if sealTimestamp {
			return timestampSeal(cmd, s, f)
		}
		return nil
	},
}

// sealHead seals the journal's head, or finds the seal it already has.
func sealHead(cmd *cobra.Command, s *session) (seal.Seal, bool, error) {
	events, err := s.Journal().Events()
	if err != nil {
		return seal.Seal{}, false, err
	}
	if len(events) == 0 {
		return seal.Seal{}, false, errors.New("the journal is empty: there is nothing to seal")
	}
	head := events[len(events)-1].Hash
	st := seal.Store{Dir: s.Repo.SealsDir()}
	kept, err := st.All()
	if err != nil {
		return seal.Seal{}, false, err
	}
	for _, f := range kept {
		if old, err := seal.Parse(f.Text); err == nil && old.Head == head {
			return old, false, nil
		}
	}
	key, made, err := seal.Key(s.Repo.SealKeyPath())
	if err != nil {
		return seal.Seal{}, false, err
	}
	if made {
		fmt.Fprintf(cmd.ErrOrStderr(), "Made a sealing key in %s; its fingerprint is %s\n",
			s.Repo.SealKeyPath(), seal.Fingerprint(key.Public().(ed25519.PublicKey)))
	}
	f := seal.Make(key, head, len(events), time.Now())
	return f, true, st.Save(seal.File{Name: f.Name(), Text: f.String()})
}

// timestampSeal sends the seal's hash to the calendars and keeps the proof.
func timestampSeal(cmd *cobra.Command, s *session, f seal.Seal) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 45*time.Second)
	defer cancel()
	ots, n, err := seal.Stamp(ctx, &http.Client{Timeout: 20 * time.Second}, f.Digest(), seal.Calendars)
	if err != nil {
		return err
	}
	if err := (seal.Store{Dir: s.Repo.SealsDir()}).Save(seal.File{Name: f.Name(), Text: f.String(), OTS: ots}); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Sent the seal's SHA-256, and nothing else, to %s; the proof is %s.ots, pending until it is in a block.\n",
		plural(n, "calendar"), f.Name())
	return nil
}

// upgradeSeals asks the calendars for the finished proof of every pending seal.
func upgradeSeals(cmd *cobra.Command, s *session) error {
	st := seal.Store{Dir: s.Repo.SealsDir()}
	kept, err := st.All()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 20 * time.Second}
	out := cmd.OutOrStdout()
	asked := 0
	for _, f := range kept {
		if p, err := seal.Describe(f.OTS); err != nil || len(p.Pending) == 0 {
			continue
		}
		asked++
		upgraded, changed, err := seal.Upgrade(ctx, client, f.OTS)
		if err != nil {
			return err
		}
		if !changed {
			fmt.Fprintf(out, "%s: still pending\n", f.Name)
			continue
		}
		f.OTS = upgraded
		if err := st.Save(f); err != nil {
			return err
		}
		p, _ := seal.Describe(upgraded)
		fmt.Fprintf(out, "%s: in Bitcoin block %d\n", f.Name, p.Blocks[0])
	}
	if asked == 0 {
		fmt.Fprintln(out, "No seal is waiting for its proof.")
	}
	return nil
}

// showSealKey prints the public half of the sealing key.
func showSealKey(out io.Writer, s *session) error {
	key, err := seal.LoadKey(s.Repo.SealKeyPath())
	if err != nil {
		return err
	}
	pub := key.Public().(ed25519.PublicKey)
	fmt.Fprintf(out, "%s\ned25519:%s\n", seal.Fingerprint(pub), base64.StdEncoding.EncodeToString(pub))
	return nil
}

func init() {
	Seal.Flags().BoolVar(&sealTimestamp, "timestamp", false, "also timestamp it with OpenTimestamps (sends its SHA-256 to the calendars)")
	Seal.Flags().BoolVar(&sealUpgrade, "upgrade", false, "ask the calendars for the finished proofs")
	Seal.Flags().BoolVar(&sealKey, "key", false, "show the sealing key's public half and fingerprint")
}
