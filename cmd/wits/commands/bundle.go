// Package commands holds the repository carried elsewhere here: a bundle
// written, a repository cloned from one or from a shared folder, the remote,
// and push and pull.
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
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/remote"
	"github.com/TheDonDope/wits/pkg/repo"
	"github.com/TheDonDope/wits/pkg/seal"
	"github.com/spf13/cobra"
)

var (
	remoteKey string
	cloneKey  string
)

// Bundle is the `wits bundle` command, which only groups: a bundle is
// created, and read back by wits clone.
var Bundle = &cobra.Command{
	Use:   "bundle",
	Short: "Pack the whole repository into one file",
	Args:  cobra.NoArgs,
}

var bundleCreate = &cobra.Command{
	Use:   "create <file>",
	Short: "Write the whole repository to a single compact file",
	Long: "Write the catalogs and every event to one file, the way `git bundle\n" +
		"create` packs a repository for carrying elsewhere. `wits clone` reads it\n" +
		"back into a new repository, reproducing the journal exactly, hashes and\n" +
		"all. A file ending in .gz is compressed; - writes to the standard output.\n\n" +
		"The format is plain text, so the record stays legible with nothing but a\n" +
		"text editor, and diffs cleanly in git. It is small anyway: what the\n" +
		"journal spends most of its bytes on — sequence numbers, account pairs\n" +
		"and the hash chain — is recomputed on clone rather than written down.",
	Example: "  wits bundle create history.wits\n" +
		"  wits bundle create history.wits.gz",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		name := args[0]
		var out io.Writer = cmd.OutOrStdout()
		if name != "-" {
			f, err := os.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
			if err != nil {
				return err
			}
			defer f.Close()
			out = f
		}
		if strings.HasSuffix(name, ".gz") {
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
		seals, err := seal.Store{Dir: s.Repo.SealsDir()}.All()
		if err != nil {
			return err
		}
		if err := bundle.Write(out, bundle.Contents{
			Products: s.Products,
			Devices:  s.Devices,
			Events:   events,
			Ratings:  s.Ratings,
			Prices:   s.Prices,
			Seals:    seals,
		}); err != nil {
			return err
		}
		if name != "-" {
			fmt.Fprintf(cmd.ErrOrStderr(), "Bundled %d events into %s\n", len(events), name)
		}
		return nil
	},
}

// Clone is the `wits clone` command.
var Clone = &cobra.Command{
	Use:   "clone <bundle | folder> [<directory>]",
	Short: "Make a new repository from a bundle, or from a shared folder",
	Long: "Make a new repository the way `git clone` does, from a bundle that\n" +
		"wits bundle create wrote or from a remote's folder another device pushes\n" +
		"to. A bundle is replayed in order, so the journal, its hash chain\n" +
		"included, comes out identical to the one it was written from. A folder is\n" +
		"read with the key the first device made: --key, brought by hand.\n\n" +
		"The directory is made, or must be empty; without one it is named after\n" +
		"the bundle or the folder.",
	Example: "  wits clone history.wits\n" +
		"  wits clone history.wits.gz restored\n" +
		"  wits clone ~/Sync/wits --key remote.key",
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		source := args[0]
		into := cloneTarget(source)
		if len(args) == 2 {
			into = args[1]
		}
		if err := emptyOrAbsent(into); err != nil {
			return err
		}
		if info, err := os.Stat(source); err == nil && info.IsDir() {
			return cloneFolder(cmd, source, into)
		}
		return cloneBundle(cmd, source, into)
	},
}

// cloneTarget names the directory a clone makes when none is given: the
// source's name without its extensions, as git names a clone.
func cloneTarget(source string) string {
	name := filepath.Base(filepath.Clean(source))
	for _, ext := range []string{".gz", ".wits"} {
		name = strings.TrimSuffix(name, ext)
	}
	return name
}

// emptyOrAbsent refuses a directory that already holds something: a clone
// never interleaves two histories.
func emptyOrAbsent(dir string) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("%s already exists and is not empty", dir)
	}
	return nil
}

// cloneBundle makes a repository from a bundle, replayed in order.
func cloneBundle(cmd *cobra.Command, file, into string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	var in io.Reader = f
	if strings.HasSuffix(file, ".gz") {
		z, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer z.Close()
		in = z
	}

	step := progress(cmd.ErrOrStderr())
	step("reading " + filepath.Base(file))
	contents, err := bundle.Read(in)
	if err != nil {
		return err
	}
	r, err := repo.Init(into)
	if err != nil {
		return err
	}
	if err := bundle.Restore(r, contents, step); err != nil {
		return err
	}
	step("done")
	// Cloned faithfully all the same: a backup gives back what it holds.
	if short := ledger.Shortfalls(contents.Events); len(short) > 0 {
		verb := "take"
		if len(short) == 1 {
			verb = "takes"
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "⚠ %s %s grams that were not in the jar at the time; wits fsck lists them\n",
			entries(len(short)), verb)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Cloned %s, %s and %s into %s.\n", plural(len(products(contents)), "product"),
		plural(len(devices(contents)), "device"), plural(len(contents.Events), "event"), filepath.Dir(r.Root()))
	return nil
}

// cloneFolder makes a repository from a remote's folder: one made new, the
// folder set as its remote with the key brought from the first device, and
// everything in it pulled.
func cloneFolder(cmd *cobra.Command, folder, into string) error {
	if cloneKey == "" {
		return fmt.Errorf("a shared folder is read with the first device's key: wits clone %s --key <file>", folder)
	}
	r, err := repo.Init(into)
	if err != nil {
		return err
	}
	if err := remote.Add(r, folder, cloneKey); err != nil {
		return err
	}
	done, err := remote.Pull(r)
	if err != nil {
		return err
	}
	reportPull(cmd.OutOrStdout(), done)
	fmt.Fprintf(cmd.OutOrStdout(), "Cloned into %s, its remote %s.\n", filepath.Dir(r.Root()), r.Config.Remote)
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
		"another device by hand, and give it to `wits clone --key` or `wits remote\n" +
		"add --key` there.",
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
		if done.Events == 0 && done.Ratings == 0 && done.Prices == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "The remote has every entry and rating already.")
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Pushed %s and %s%s.\n", entries(done.Events), plural(done.Ratings, "rating"),
			andPrices(done.Prices))
		return nil
	},
}

// Pull is the `wits pull` command.
var Pull = &cobra.Command{
	Use:   "pull",
	Short: "Bring in what other devices pushed",
	Long: "Bring in the remote's entries, ratings, prices, products and devices.\n" +
		"The remote's entries are appended as they are, the way git merges, and\n" +
		"when both devices recorded, a merge entry names both heads; no entry gets\n" +
		"a new hash. Where the remote holds a product or a device differently, the\n" +
		"remote's stands.\n\n" +
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
	if done.Fetched == 0 && done.Ratings == 0 && done.Prices == 0 && len(done.Products) == 0 && len(done.Devices) == 0 {
		fmt.Fprintln(out, "Already up to date.")
		return
	}
	fmt.Fprintf(out, "Pulled %s and %s%s.\n", entries(done.Fetched), plural(done.Ratings, "rating"), andPrices(done.Prices))
	if done.Joined {
		fmt.Fprintln(out, "Merged with this device's own, which keep their hashes.")
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
	Clone.Flags().StringVar(&cloneKey, "key", "", "cloning a folder: the key file brought from the first device")
	Bundle.AddCommand(bundleCreate)
}

// andPrices names the prices a push or a pull carried, when it carried any.
func andPrices(n int) string {
	if n == 0 {
		return ""
	}
	return ", with " + plural(n, "price")
}
