package commands

import (
	"fmt"
	"io"
	"strings"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/spf13/cobra"
)

var (
	seshDate   string
	seshDevice string
	seshTemp   int
	seshNote   string
)

// Sesh is the `wits sesh` command.
var Sesh = &cobra.Command{
	Use:     "sesh <product> <amount>",
	Aliases: []string{"session"},
	Short:   "Record a session, drawing from the stash",
	Long: "Record a session: ground product comes out of that product's tin and\n" +
		"goes through a device. What is left of it becomes AVB, which is weighed\n" +
		"and credited separately.\n\n" +
		"With a temperature, this also reports which compounds that setting is\n" +
		"hot enough to release, and warns when it is hot enough to produce\n" +
		"benzene.",
	Example: "  wits sesh wedding-cake 0.3 --device volcano --temp 185\n" +
		"  wits sesh lemon 0.2 --date 2026-07-29",
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		grams, err := parseGrams(args[1])
		if err != nil {
			return err
		}
		at, err := parseDate(seshDate)
		if err != nil {
			return err
		}
		e, err := s.Recorder.Session(args[0], grams, at, seshDevice, seshTemp, seshNote)
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "[%s] sesh %.2fg %s, %.2fg left in the tin\n",
			shortHash(e.Hash), e.Grams, e.Product, s.Recorder.Available(e.Product, journal.Stash))
		if e.Temperature > 0 {
			writeReleased(out, e.Temperature)
		}
		return nil
	},
}

// writeReleased reports what a temperature is hot enough to volatilise.
func writeReleased(out io.Writer, celsius int) {
	released := catalog.ReleasedAt(celsius)
	if len(released) == 0 {
		fmt.Fprintf(out, "At %d°C nothing has reached its boiling point yet.\n", celsius)
		return
	}
	names := make([]string, 0, len(released))
	for _, r := range released {
		names = append(names, fmt.Sprintf("%s (%d°C)", r.Name, r.BoilingPoint))
	}
	fmt.Fprintf(out, "At %d°C this releases %s.\n", celsius, strings.Join(names, ", "))

	for _, h := range catalog.Hazards(celsius) {
		fmt.Fprintf(out, "⚠️  %d°C is at or above the %d°C boiling point of %s (%s).\n",
			celsius, h.BoilingPoint, h.Name, strings.Join(h.Effects, ", "))
	}
}

func init() {
	Sesh.Flags().StringVar(&seshDate, "date", "", "when the session was, defaults to now")
	Sesh.Flags().StringVar(&seshDevice, "device", "", "the device used")
	Sesh.Flags().IntVar(&seshTemp, "temp", 0, "the temperature in degrees Celsius")
	Sesh.Flags().StringVar(&seshNote, "note", "", "a note to keep with the entry")
}
