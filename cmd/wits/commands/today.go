// Package commands holds `wits today` here: the day so far, and the THC in it.
package commands

import (
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"

	"github.com/TheDonDope/wits/pkg/today"
	"github.com/spf13/cobra"
)

var todayDate string

// Today is the `wits today` command.
var Today = &cobra.Command{
	Use:   "today",
	Short: "The day so far: its sessions, and the THC and CBD in them",
	Long: "The day so far: every session with the THC and CBD in the grams put through\n" +
		"the device — grams × the strain's per cent × 10, in milligrams — and their\n" +
		"sum, against a usual day of the last 30 with a session. It is what was in\n" +
		"the material, not what reached the body: a vaporizer releases part of it.\n" +
		"A strain with no THC written down is counted, not guessed at. --date for\n" +
		"another day; the dashboard's Today draws the same.",
	Example: "  wits today\n  wits today --date 2026-10-24",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		at := s.OpenedAt
		if todayDate != "" {
			if at, err = parseDate(todayDate); err != nil {
				return err
			}
		}
		writeToday(cmd.OutOrStdout(), today.Of(s.Workspace, at))
		return nil
	},
}

// writeToday says the day, then its sessions one a line, then the rest of it.
func writeToday(out io.Writer, d today.Today) {
	fmt.Fprintf(out, "%s: %s\n", d.Day.Format("Monday 2 January"), todayLine(d))
	if d.Unknown > 0 {
		fmt.Fprintf(out, "%s of a strain with no THC written down; `wits describe` gives it one.\n", plural(d.Unknown, "session"))
	}
	if len(d.Sessions) > 0 {
		fmt.Fprintln(out)
		w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "TIME\tPRODUCT\tDEVICE\tGRAMS\tTHC\tCBD\tRATED")
		for _, s := range d.Sessions {
			fmt.Fprintf(w, "%s\t%s\t%s\t%.2fg\t%s\t%s\t%s\n", s.At.Format("15:04"), s.Name, deviceAt(s), s.Grams,
				milligrams(s.THC, s.Known), milligrams(s.CBD, s.Known), stars(s.Overall))
		}
		w.Flush()
	}
	fmt.Fprintln(out)
	if d.Ground > 0 {
		fmt.Fprintf(out, "Ground %.2f g. ", d.Ground)
	}
	if u := d.Usual; u.Days > 0 {
		fmt.Fprintf(out, "A usual day of the last 30 with a session: %.2f g, %.0f mg THC (%s).\n", u.Seshed, u.THC, plural(u.Days, "day"))
	} else {
		fmt.Fprintln(out, "No session in the 30 days before to compare with.")
	}
}

// todayLine is the day in one line, for wits status too.
func todayLine(d today.Today) string {
	if len(d.Sessions) == 0 {
		return "no session yet"
	}
	return fmt.Sprintf("%s, %.2f g through a device: %.0f mg THC and %.0f mg CBD in the material",
		plural(len(d.Sessions), "session"), d.Seshed, d.THC, d.CBD)
}

func deviceAt(s today.Session) string {
	switch {
	case s.Device == "":
		return "-"
	case s.Temp > 0:
		return s.Device + " at " + strconv.Itoa(s.Temp) + "°C"
	}
	return s.Device
}

func milligrams(v float64, known bool) string {
	if !known {
		return "?"
	}
	return fmt.Sprintf("%.1f mg", v)
}

func stars(n int) string {
	if n == 0 {
		return "-"
	}
	return fmt.Sprintf("%d of 5", n)
}

func init() {
	Today.Flags().StringVar(&todayDate, "date", "", "another day, as 2026-10-24")
}
