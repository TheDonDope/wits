package commands

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/TheDonDope/wits/pkg/bundle"
	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/remote"
	"github.com/TheDonDope/wits/pkg/repo"
	"github.com/spf13/cobra"
)

var (
	bundleOut  string
	bundleGzip bool
	remoteKey  string
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
			Ratings:  s.Ratings,
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

		step := progress(cmd.ErrOrStderr())
		step("reading " + filepath.Base(args[0]))
		contents, err := bundle.Read(in)
		if err != nil {
			return err
		}
		if err := restore(s.Repo, contents, step); err != nil {
			return err
		}
		step("done")
		// Restored faithfully all the same: a backup gives back what it holds.
		if short := ledger.Shortfalls(contents.Events); len(short) > 0 {
			verb := "take"
			if len(short) == 1 {
				verb = "takes"
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "⚠ %s %s grams that were not in the jar at the time; wits fsck lists them\n",
				entries(len(short)), verb)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Restored %s, %s and %s.\n", plural(len(products(contents)), "product"),
			plural(len(devices(contents)), "device"), plural(len(contents.Events), "event"))
		return nil
	},
}

// restore writes a bundle's contents into an empty repository: the catalogs,
// then every event as one batch — one write, one flush, refused whole if any
// event does not validate — and checks the chain it made. An event at a time
// flushed once per entry, which on a real disk is seconds of silence, and a
// failure halfway left a journal too full to restore into again.
func restore(r *repo.Repo, contents *bundle.Contents, step func(string)) error {
	if p := products(contents); len(p) > 0 {
		step("restoring " + plural(len(p), "product"))
		if _, err := r.UpdateProducts(func(c *catalog.Catalog) error { c.Products = p; return nil }); err != nil {
			return err
		}
	}
	if d := devices(contents); len(d) > 0 {
		step("restoring " + plural(len(d), "device"))
		if _, err := r.UpdateDevices(func(c *catalog.Devices) error { c.Devices = d; return nil }); err != nil {
			return err
		}
	}
	step("writing " + plural(len(contents.Events), "event") + " to the journal")
	if _, err := r.Journal().AppendWith(func(events []journal.Event) ([]journal.Event, error) {
		if len(events) > 0 {
			return nil, fmt.Errorf("journal already holds %d events; restore into an empty repository", len(events))
		}
		return contents.Events, nil
	}); err != nil {
		return err
	}
	step("verifying the chain")
	if err := r.Journal().Verify(); err != nil {
		return fmt.Errorf("the restored journal does not verify: %w", err)
	}
	if len(contents.Ratings) > 0 {
		step("restoring " + plural(len(contents.Ratings), "rating"))
		for _, rating := range contents.Ratings {
			if _, err := r.Effects().Append(rating); err != nil {
				return fmt.Errorf("restoring the effect log: %w", err)
			}
		}
		if err := r.Effects().Verify(); err != nil {
			return fmt.Errorf("the restored effect log does not verify: %w", err)
		}
	}
	return nil
}

// products returns a bundle's products, or none when it carried no catalog.
func products(c *bundle.Contents) []*catalog.Product {
	if c.Products == nil {
		return nil
	}
	return c.Products.Products
}

// devices returns a bundle's devices, or none when it carried no catalog.
func devices(c *bundle.Contents) []*catalog.Device {
	if c.Devices == nil {
		return nil
	}
	return c.Devices.Devices
}

// Remote is the `wits remote` command: where push and pull go.
var Remote = &cobra.Command{
	Use:   "remote",
	Short: "Show the folder push and pull share with other devices",
	Long: "A remote is a folder more than one device can reach — a Syncthing share,\n" +
		"a USB stick — holding the repository as one file, encrypted with age to a\n" +
		"key kept in .wits/remote.key and never in the folder. Bring that key to\n" +
		"another device by hand, and give it to `wits remote add --key` there.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		if s.Repo.Config.Remote == "" {
			return remote.ErrNoRemote
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s\nkey: %s\n", s.Repo.Config.Remote, s.Repo.RemoteKeyPath())
		return nil
	},
}

var remoteAdd = &cobra.Command{
	Use:   "add <folder>",
	Short: "Share the repository through a folder",
	Long: "Set the folder push and pull go to. On the first device a key is made;\n" +
		"on every other, pass the first device's key with --key, or the folder's\n" +
		"contents cannot be read.",
	Example: "  wits remote add ~/Sync/wits\n" +
		"  wits remote add /run/media/stick/wits --key remote.key",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		if err := remote.Add(s.Repo, args[0], remoteKey); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Remote set to %s. Its key is %s: bring it to every other device, and nowhere else.\n",
			s.Repo.Config.Remote, s.Repo.RemoteKeyPath())
		return nil
	},
}

// Push is the `wits push` command.
var Push = &cobra.Command{
	Use:   "push",
	Short: "Write the repository to its remote",
	Long: "Write the repository to the remote folder, for the other devices to pull.\n" +
		"Refused when the remote holds anything this device does not — pull first,\n" +
		"so nothing another device wrote is lost.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		done, err := remote.Push(s.Repo)
		if err != nil {
			return err
		}
		if done.Events == 0 && done.Ratings == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "The remote has every entry and rating already.")
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Pushed %s and %s.\n", entries(done.Events), plural(done.Ratings, "rating"))
		return nil
	},
}

// Pull is the `wits pull` command.
var Pull = &cobra.Command{
	Use:   "pull",
	Short: "Bring in what other devices pushed",
	Long: "Bring in the remote's entries, ratings, products and devices. Entries\n" +
		"this device made since its last push are chained again on top of the\n" +
		"remote's, the way git rebases, and get new hashes; the old journal is kept\n" +
		"as .wits/journal.ndjson.orig. Where the remote holds a product or a device\n" +
		"differently, the remote's stands.\n\n" +
		"Two devices can each record the same last grams of a jar. Both entries are\n" +
		"kept, as written, and named; the jar takes nothing more until one of them\n" +
		"is undone with wits revert.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		done, err := remote.Pull(s.Repo)
		if err != nil {
			return err
		}
		reportPull(cmd.OutOrStdout(), done)
		return nil
	},
}

// reportPull says what a pull brought in, and every clash it made.
func reportPull(out io.Writer, done remote.Pulled) {
	if done.Fetched == 0 && done.Ratings == 0 && len(done.Products) == 0 && len(done.Devices) == 0 {
		fmt.Fprintln(out, "Already up to date.")
		return
	}
	fmt.Fprintf(out, "Pulled %s and %s.\n", entries(done.Fetched), plural(done.Ratings, "rating"))
	if done.Rechained > 0 {
		fmt.Fprintf(out, "Chained %s of this device's on top, with new hashes.\n", entries(done.Rechained))
	}
	if took := append(slices.Clone(done.Products), done.Devices...); len(took) > 0 && len(took) <= 5 {
		fmt.Fprintf(out, "Took the remote's %s.\n", strings.Join(took, ", "))
	} else if len(took) > 5 {
		fmt.Fprintf(out, "Took the remote's %s and %s.\n",
			plural(len(done.Products), "product"), plural(len(done.Devices), "device"))
	}
	for _, c := range done.Clashes {
		fmt.Fprintf(out, "🚨 %s %s\n", shortHash(c.Event.Hash), c)
	}
	if len(done.Clashes) > 0 {
		fmt.Fprintln(out, "   Two devices recorded the same grams: wits revert one of the entries. The jar takes nothing else until then.")
	}
}

func init() {
	Remote.AddCommand(remoteAdd)
	remoteAdd.Flags().StringVar(&remoteKey, "key", "", "the key file brought from the first device")
	Bundle.Flags().StringVar(&bundleOut, "out", "", "write to this file instead of stdout")
	Bundle.Flags().BoolVar(&bundleGzip, "gzip", false, "compress the bundle")
}
