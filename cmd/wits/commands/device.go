// Package commands holds the devices here: listed, described with what
// their usual setting releases, created, edited and deleted — and what a
// temperature releases, described on its own.
package commands

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/views"
)

var (
	deviceKind    string
	deviceMaker   string
	deviceMinTemp int
	deviceMaxTemp int
	deviceDefault int
	deviceLoad    float64
	deviceName    string
)

var getDevices = &cobra.Command{
	Use:     "devices",
	Aliases: []string{"device", "dev"},
	Short:   "The devices: what each is, how it heats, and how much it was used",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		use := views.Devices(s.State.Standing)
		var rows []deviceRow
		for _, d := range s.Devices.Devices {
			if getAll || !d.Retired {
				rows = append(rows, deviceRowOf(d, use[d.Slug]))
			}
		}
		return list(cmd.OutOrStdout(), rows, listing[deviceRow]{
			head: []string{"SLUG", "NAME", "KIND", "HEAT", "SESSIONS"},
			wide: []string{"MAKER", "LOAD", "GRAMS", "LAST USED"},
			cells: func(r deviceRow) ([]string, []string) {
				last := "-"
				if r.LastUsed != nil {
					last = r.LastUsed.Format(time.DateOnly)
				}
				return []string{r.Slug, r.Name, r.Kind, orDash(r.Heat), fmt.Sprint(r.Sessions)},
					[]string{orDash(r.Maker), gramsCell(r.Load), gramsCell(r.Grams), last}
			},
		})
	},
}

// deviceRow is one device as get lists it, and -o json prints it.
type deviceRow struct {
	Slug     string     `json:"slug"`
	Name     string     `json:"name"`
	Kind     string     `json:"kind"`
	Maker    string     `json:"maker,omitempty"`
	Heat     string     `json:"heat,omitempty"`
	MinTemp  int        `json:"min_temp,omitempty"`
	MaxTemp  int        `json:"max_temp,omitempty"`
	Usual    int        `json:"usual_temp,omitempty"`
	Load     float64    `json:"load"`
	Retired  bool       `json:"retired,omitempty"`
	Sessions int        `json:"sessions"`
	Heated   int        `json:"heated,omitempty"`
	Grams    float64    `json:"grams"`
	LastUsed *time.Time `json:"last_used,omitempty"`
}

func deviceRowOf(d *catalog.Device, u *views.Use) deviceRow {
	r := deviceRow{Slug: d.Slug, Name: d.Name, Kind: d.Is(), Maker: d.Maker, Heat: heatOf(d),
		MinTemp: d.MinTemp, MaxTemp: d.MaxTemp, Usual: d.DefaultTemp, Load: d.Load, Retired: d.Retired}
	if u != nil {
		r.Sessions, r.Heated, r.Grams = u.Sessions, u.Heated, u.Grams.Grams()
		if !u.LastUsed.IsZero() {
			r.LastUsed = &u.LastUsed
		}
	}
	return r
}

// heatOf says how a device heats: its range, its usual setting and its
// modes, those it has.
func heatOf(d *catalog.Device) string {
	var parts []string
	if d.MaxTemp > 0 {
		parts = append(parts, fmt.Sprintf("%d-%d°C", d.MinTemp, d.MaxTemp))
	}
	if d.DefaultTemp > 0 {
		parts = append(parts, fmt.Sprintf("usually %d°C", d.DefaultTemp))
	}
	if len(d.Modes) > 0 {
		parts = append(parts, strings.Join(d.ModeNames(), ", "))
	}
	return strings.Join(parts, " · ")
}

var describeDevice = &cobra.Command{
	Use:     "device <device>",
	Aliases: []string{"devices", "dev"},
	Short:   "A device in full: what it is, how it was used, and what its usual setting releases",
	Example: "  wits describe device mighty",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		d, err := s.Devices.Find(args[0])
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		r := deviceRowOf(d, views.Devices(s.State.Standing)[d.Slug])
		fmt.Fprintf(out, "%s (%s)\n", d.Name, d.Slug)
		line := func(label, value string) {
			if value != "" {
				fmt.Fprintf(out, "%-9s %s\n", label, value)
			}
		}
		line("Kind", r.Kind)
		line("Maker", r.Maker)
		line("Heat", r.Heat)
		if d.Load > 0 {
			line("Load", gramsCell(d.Load))
		}
		used := plural(r.Sessions, "session") + ", " + gramsCell(r.Grams)
		if d.Heats() {
			used = fmt.Sprintf("heated %s", plural(r.Heated, "session"))
		}
		if r.LastUsed != nil {
			used += ", last on " + r.LastUsed.Format("2 Jan 2006")
		}
		line("Used", used)
		if d.Retired {
			line("Retired", "yes")
		}
		if d.DefaultTemp > 0 {
			fmt.Fprintf(out, "\nAt %d°C, as it is usually set:\n", d.DefaultTemp)
			writeReleasedTable(out, d.DefaultTemp)
		}
		return nil
	},
}

var createDevice = &cobra.Command{
	Use:     "device <name>",
	Aliases: []string{"dev"},
	Short:   "Add a device",
	Example: "  wits create device \"Volcano Hybrid\" --min 40 --max 230 --default 185\n" +
		"  wits create device \"Storz & Bickel Mighty+\" --load 0.15",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		device := &catalog.Device{
			Name:        args[0],
			Kind:        deviceKind,
			Maker:       deviceMaker,
			MinTemp:     deviceMinTemp,
			MaxTemp:     deviceMaxTemp,
			DefaultTemp: deviceDefault,
			Load:        journal.CG(deviceLoad).Grams(),
		}
		if deviceLoad < 0 {
			return fmt.Errorf("a load holds some grams or none, not %.2f", deviceLoad)
		}
		if _, err := s.Repo.UpdateDevices(func(d *catalog.Devices) error {
			return d.Add(device)
		}); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Created device %s (%s)\n", device.Slug, device.Name)
		return nil
	},
}

var editDevice = &cobra.Command{
	Use:     "device <device>",
	Aliases: []string{"dev"},
	Short:   "Change what a device is; its sessions keep naming it",
	Long: "Change a device's name, kind, maker, temperatures or load, as far as the\n" +
		"flags say. Its slug stays, since every session it was used for names it by that.",
	Example: "  wits edit device mighty --load 0.15\n" +
		"  wits edit device volcano --name \"Volcano Hybrid\" --default 190",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		var edited catalog.Device
		if _, err := s.Repo.UpdateDevices(func(c *catalog.Devices) error {
			d, err := c.Find(args[0])
			if err != nil {
				return err
			}
			if err := editFromFlags(cmd, d); err != nil {
				return err
			}
			edited = *d
			return d.Check()
		}); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Changed device %s (%s)\n", edited.Slug, edited.Name)
		return nil
	},
}

// editFromFlags changes a device as far as the flags say, and no further.
func editFromFlags(cmd *cobra.Command, d *catalog.Device) error {
	changed := false
	for _, f := range []struct {
		name string
		set  func()
	}{
		{"name", func() { d.Name = deviceName }},
		{"kind", func() { d.Kind = deviceKind }},
		{"maker", func() { d.Maker = deviceMaker }},
		{"min", func() { d.MinTemp = deviceMinTemp }},
		{"max", func() { d.MaxTemp = deviceMaxTemp }},
		{"default", func() { d.DefaultTemp = deviceDefault }},
		{"load", func() { d.Load = journal.CG(deviceLoad).Grams() }},
	} {
		if cmd.Flags().Changed(f.name) {
			f.set()
			changed = true
		}
	}
	if !changed {
		return fmt.Errorf("nothing to change: say what, with --name, --kind, --maker, --min, --max, --default or --load")
	}
	if d.Load < 0 {
		return fmt.Errorf("a load holds some grams or none, not %.2f", d.Load)
	}
	return nil
}

var deleteDevice = &cobra.Command{
	Use:     "device <device>",
	Aliases: []string{"dev"},
	Short:   "Take a device out of the catalog; its sessions keep its name",
	Long: "Take a device out of the catalog. The sessions it was used for keep\n" +
		"naming it, so what is lost is its description, not its history. A device\n" +
		"put away for good can be retired instead, in the browser, and kept.",
	Example: "  wits delete device volcano",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		var gone *catalog.Device
		if _, err := s.Repo.UpdateDevices(func(c *catalog.Devices) error {
			d, err := c.Find(args[0])
			if err != nil {
				return err
			}
			gone, err = c.Remove(d.Slug)
			return err
		}); err != nil {
			return err
		}
		used := 0
		if u := views.Devices(s.State.Standing)[gone.Slug]; u != nil {
			used = u.Sessions
		}
		keep := "keep"
		if used == 1 {
			keep = "keeps"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Deleted device %s (%s); %s %s its name\n", gone.Slug, gone.Name, plural(used, "session"), keep)
		return nil
	},
}

var describeTemperature = &cobra.Command{
	Use:     "temperature <celsius>",
	Aliases: []string{"temp", "temps"},
	Short:   "What a temperature releases, and whether it is too hot",
	Long: "Show which cannabinoids and terpenes a given temperature is hot enough\n" +
		"to volatilise, and warn when it is hot enough to produce benzene.",
	Example: "  wits describe temperature 185",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var celsius int
		if _, err := fmt.Sscanf(strings.TrimSuffix(args[0], "°C"), "%d", &celsius); err != nil {
			return fmt.Errorf("%q is not a temperature in degrees Celsius", args[0])
		}
		writeReleasedTable(cmd.OutOrStdout(), celsius)
		return nil
	},
}

// writeReleasedTable lists what a temperature releases, and the hazards it
// reaches.
func writeReleasedTable(out io.Writer, celsius int) {
	released := catalog.ReleasedAt(celsius)
	if len(released) == 0 {
		fmt.Fprintf(out, "At %d°C nothing has reached its boiling point yet.\n", celsius)
		return
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "COMPOUND\tBOILS AT\tEFFECTS")
	for _, r := range released {
		mark := ""
		if r.Harmful {
			mark = " ⚠️"
		}
		fmt.Fprintf(w, "%s%s\t%d°C\t%s\n", r.Name, mark, r.BoilingPoint, orDash(strings.Join(r.Effects, ", ")))
	}
	w.Flush()
	for _, h := range catalog.Hazards(celsius) {
		fmt.Fprintf(out, "\n⚠️  %d°C is at or above the %d°C boiling point of %s.\n", celsius, h.BoilingPoint, h.Name)
	}
}

func init() {
	for _, c := range []*cobra.Command{createDevice, editDevice} {
		f := c.Flags()
		f.StringVar(&deviceKind, "kind", "", "the kind of device, for example desktop or portable")
		f.StringVar(&deviceMaker, "maker", "", "who makes it")
		f.IntVar(&deviceMinTemp, "min", 0, "the lowest temperature it can be set to")
		f.IntVar(&deviceMaxTemp, "max", 0, "the highest temperature it can be set to")
		f.IntVar(&deviceDefault, "default", 0, "the temperature it is usually set to, assumed when none is given")
		f.Float64Var(&deviceLoad, "load", 0, "the grams one fill holds, as usually loaded — sizes a forgotten session")
	}
	editDevice.Flags().StringVar(&deviceName, "name", "", "what it is called")
	Get.AddCommand(getDevices)
	Describe.AddCommand(describeDevice, describeTemperature)
	Create.AddCommand(createDevice)
	Edit.AddCommand(editDevice)
	Delete.AddCommand(deleteDevice)
}
