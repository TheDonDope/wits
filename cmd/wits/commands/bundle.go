package commands

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/TheDonDope/wits/pkg/bundle"
	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/spf13/cobra"
)

var (
	bundleOut  string
	bundleGzip bool
)

// Bundle is the `wits bundle` command.
var Bundle = &cobra.Command{
	Use:   "bundle",
	Short: "Write the whole repository to a single compact file",
	Long: "Write the catalogs and every event to one file, the way `git bundle`\n" +
		"packs a repository for carrying elsewhere. `wits restore` reads it back\n" +
		"into an empty repository, reproducing the journal exactly, hashes and\n" +
		"all.\n\n" +
		"The format is plain text, so the record stays legible with nothing but a\n" +
		"text editor, and diffs cleanly in git. It is small anyway: what the\n" +
		"journal spends most of its bytes on — sequence numbers, account pairs\n" +
		"and the hash chain — is recomputed on restore rather than written down.",
	Example: "  wits bundle --out history.wits\n" +
		"  wits bundle --gzip --out history.wits.gz",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		var out io.Writer = cmd.OutOrStdout()
		if bundleOut != "" {
			f, err := os.OpenFile(bundleOut, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
			if err != nil {
				return err
			}
			defer f.Close()
			out = f
		}
		if bundleGzip {
			z := gzip.NewWriter(out)
			defer z.Close()
			out = z
		}

		// The journal, not the fold: the fold orders entries by date, and a
		// bundle has to keep the recorded order its hash chain is made of.
		events, err := s.Journal().Events()
		if err != nil {
			return err
		}
		if err := bundle.Write(out, bundle.Contents{
			Products: s.Products,
			Devices:  s.Devices,
			Events:   events,
		}); err != nil {
			return err
		}
		if bundleOut != "" {
			fmt.Fprintf(cmd.ErrOrStderr(), "Bundled %d events into %s\n", len(events), bundleOut)
		}
		return nil
	},
}

// Restore is the `wits restore` command.
var Restore = &cobra.Command{
	Use:   "restore <file>",
	Short: "Rebuild a repository from a bundle",
	Long: "Read a bundle back into the repository, replaying its events in order\n" +
		"so that the journal, its hash chain included, comes out identical to the\n" +
		"one the bundle was written from.\n\n" +
		"The repository must be empty. Restoring into a journal that already holds\n" +
		"events would interleave two histories, and there is no way to do that\n" +
		"without deciding which one is right.",
	Example: "  wits init . && wits restore history.wits",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		if n := len(s.State.Events); n > 0 {
			return fmt.Errorf("journal already holds %d events; restore into an empty repository", n)
		}

		f, err := os.Open(args[0])
		if err != nil {
			return err
		}
		defer f.Close()

		var in io.Reader = f
		if strings.HasSuffix(args[0], ".gz") {
			z, err := gzip.NewReader(f)
			if err != nil {
				return err
			}
			defer z.Close()
			in = z
		}

		contents, err := bundle.Read(in)
		if err != nil {
			return err
		}

		if contents.Products != nil && len(contents.Products.Products) > 0 {
			if _, err := s.Repo.UpdateProducts(func(c *catalog.Catalog) error {
				c.Products = contents.Products.Products
				return nil
			}); err != nil {
				return err
			}
		}
		if contents.Devices != nil && len(contents.Devices.Devices) > 0 {
			if _, err := s.Repo.UpdateDevices(func(d *catalog.Devices) error {
				d.Devices = contents.Devices.Devices
				return nil
			}); err != nil {
				return err
			}
		}
		for i, e := range contents.Events {
			if _, err := s.Journal().Append(e); err != nil {
				return fmt.Errorf("restoring event %d: %w", i+1, err)
			}
		}
		if err := s.Journal().Verify(); err != nil {
			return fmt.Errorf("the restored journal does not verify: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Restored %d products, %d devices and %d events.\n",
			len(contents.Products.Products), len(contents.Devices.Devices), len(contents.Events))
		return nil
	},
}

func init() {
	Bundle.Flags().StringVar(&bundleOut, "out", "", "write to this file instead of stdout")
	Bundle.Flags().BoolVar(&bundleGzip, "gzip", false, "compress the bundle")
}
