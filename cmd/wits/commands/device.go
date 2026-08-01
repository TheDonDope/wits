package commands

import (
	"fmt"
	"text/tabwriter"

	"github.com/TheDonDope/wits-tui/pkg/catalog"
	"github.com/spf13/cobra"
)

var (
	deviceKind    string
	deviceMinTemp int
	deviceMaxTemp int
	deviceDefault int
)

// Device is the `wits device` command.
var Device = &cobra.Command{
	Use:   "device",
	Short: "Manage your vaporizers",
	Args:  cobra.NoArgs,
	RunE:  func(cmd *cobra.Command, args []string) error { return deviceList.RunE(cmd, args) },
}

var deviceAdd = &cobra.Command{
	Use:     "add <name>",
	Short:   "Add a device",
	Example: "  wits device add \"Volcano Hybrid\" --min 40 --max 230 --default 185",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		devices := s.Devices
		device := &catalog.Device{
			Name:        args[0],
			Kind:        deviceKind,
			MinTemp:     deviceMinTemp,
			MaxTemp:     deviceMaxTemp,
			DefaultTemp: deviceDefault,
		}
		if err := devices.Add(device); err != nil {
			return err
		}
		if err := devices.Save(s.Repo.DevicesPath()); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Added device %s (%s)\n", device.Slug, device.Name)
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
	Device.AddCommand(deviceAdd, deviceList)
}
