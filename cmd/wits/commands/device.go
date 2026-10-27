package commands

import (
	"fmt"
	"text/tabwriter"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/views"
	"github.com/spf13/cobra"
)

var (
	deviceKind    string
	deviceMinTemp int
	deviceMaxTemp int
	deviceDefault int
	deviceLoad    float64
	deviceName    string
)

// Device is the `wits device` command.
var Device = &cobra.Command{
	Use:   "device",
	Short: "Manage your vaporizers",
	Args:  cobra.NoArgs,
	RunE:  func(cmd *cobra.Command, args []string) error { return deviceList.RunE(cmd, args) },
}

var deviceAdd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a device",
	Example: "  wits device add \"Volcano Hybrid\" --min 40 --max 230 --default 185\n" +
		"  wits device add \"Storz & Bickel Mighty+\" --load 0.15",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		device := &catalog.Device{
			Name:        args[0],
			Kind:        deviceKind,
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
		fmt.Fprintf(cmd.OutOrStdout(), "Added device %s (%s)\n", device.Slug, device.Name)
		return nil
	},
}

var deviceEdit = &cobra.Command{
	Use:   "edit <device>",
	Short: "Change what a device is; its sessions keep naming it",
	Long: "Change a device's name, kind, temperatures or load, as far as the flags\n" +
		"say. Its slug stays, since every session it was used for names it by that.",
	Example: "  wits device edit mighty --load 0.15\n" +
		"  wits device edit volcano --name \"Volcano Hybrid\" --default 190",
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
		return fmt.Errorf("nothing to change: say what, with --name, --kind, --min, --max, --default or --load")
	}
	if d.Load < 0 {
		return fmt.Errorf("a load holds some grams or none, not %.2f", d.Load)
	}
	return nil
}

var deviceRemove = &cobra.Command{
	Use:   "remove <device>",
	Short: "Take a device out of the catalog; its sessions keep its name",
	Long: "Take a device out of the catalog. The sessions it was used for keep\n" +
		"naming it, so what is lost is its description, not its history. A device\n" +
		"put away for good can be retired instead, in the browser, and kept.",
	Example: "  wits device remove volcano",
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
		fmt.Fprintf(cmd.OutOrStdout(), "Removed device %s (%s); %d sessions keep its name\n", gone.Slug, gone.Name, used)
		return nil
	},
}

var deviceList = &cobra.Command{
	Use:   "list",
	Short: "List your devices",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		devices := s.Devices
		out := cmd.OutOrStdout()
		if len(devices.Devices) == 0 {
			fmt.Fprintln(out, "No devices yet. Add one with `wits device add`.")
			return nil
		}
		w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SLUG\tNAME\tKIND\tRANGE\tDEFAULT")
		for _, d := range devices.Devices {
			temps := "-"
			if d.MaxTemp > 0 {
				temps = fmt.Sprintf("%d-%d°C", d.MinTemp, d.MaxTemp)
			}
			def := "-"
			if d.DefaultTemp > 0 {
				def = fmt.Sprintf("%d°C", d.DefaultTemp)
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", d.Slug, d.Name, d.Kind, temps, def)
		}
		return w.Flush()
	},
}

// Temps is the `wits temps` command.
var Temps = &cobra.Command{
	Use:   "temps <celsius>",
	Short: "Show what a temperature releases",
	Long: "Show which cannabinoids and terpenes a given temperature is hot enough\n" +
		"to volatilise, and warn when it is hot enough to produce benzene.",
	Example: "  wits temps 185",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var celsius int
		if _, err := fmt.Sscanf(args[0], "%d", &celsius); err != nil {
			return fmt.Errorf("%q is not a temperature in degrees Celsius", args[0])
		}
		out := cmd.OutOrStdout()
		released := catalog.ReleasedAt(celsius)
		if len(released) == 0 {
			fmt.Fprintf(out, "At %d°C nothing has reached its boiling point yet.\n", celsius)
			return nil
		}
		w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "COMPOUND\tBOILS AT\tEFFECTS")
		for _, r := range released {
			mark := ""
			if r.Harmful {
				mark = " ⚠️"
			}
			fmt.Fprintf(w, "%s%s\t%d°C\t%v\n", r.Name, mark, r.BoilingPoint, join(r.Effects))
		}
		w.Flush()
		for _, h := range catalog.Hazards(celsius) {
			fmt.Fprintf(out, "\n⚠️  %d°C is at or above the %d°C boiling point of %s.\n", celsius, h.BoilingPoint, h.Name)
		}
		return nil
	},
}

// join renders a list of effects for a table cell.
func join(effects []string) string {
	if len(effects) == 0 {
		return "-"
	}
	s := effects[0]
	for _, e := range effects[1:] {
		s += ", " + e
	}
	return s
}

func init() {
	deviceAdd.Flags().StringVar(&deviceKind, "kind", "", "the kind of device, for example desktop or portable")
	deviceAdd.Flags().IntVar(&deviceMinTemp, "min", 0, "the lowest temperature it can be set to")
	deviceAdd.Flags().IntVar(&deviceMaxTemp, "max", 0, "the highest temperature it can be set to")
	deviceAdd.Flags().IntVar(&deviceDefault, "default", 0, "the temperature to assume when none is given")
	deviceAdd.Flags().Float64Var(&deviceLoad, "load", 0, "the grams one fill holds, as usually loaded — sizes a forgotten session")
	deviceEdit.Flags().StringVar(&deviceName, "name", "", "what it is called")
	deviceEdit.Flags().StringVar(&deviceKind, "kind", "", "the kind of device, for example desktop or portable")
	deviceEdit.Flags().IntVar(&deviceMinTemp, "min", 0, "the lowest temperature it can be set to")
	deviceEdit.Flags().IntVar(&deviceMaxTemp, "max", 0, "the highest temperature it can be set to")
	deviceEdit.Flags().IntVar(&deviceDefault, "default", 0, "the temperature to assume when none is given")
	deviceEdit.Flags().Float64Var(&deviceLoad, "load", 0, "the grams one fill holds, as usually loaded")
	Device.AddCommand(deviceAdd, deviceEdit, deviceRemove, deviceList)
}
