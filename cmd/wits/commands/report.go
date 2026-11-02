// Package commands holds `wits report` here: the day, the costs, the year,
// the next order, and the page for the prescriber — each a reading of the
// journal, worked out by the same code the terminal and the browser read.
package commands

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/TheDonDope/wits/pkg/report"
	"github.com/TheDonDope/wits/pkg/version"
)

// Report is the `wits report` command, which only groups.
var Report = &cobra.Command{
	Use:   "report <today|costs|year|refill|prescriber>",
	Short: "Read the journal for a purpose: the day, the costs, the year, the next order, the prescriber",
	Args:  cobra.NoArgs,
}

var (
	reportFrom, reportTo, reportOut string
	reportSeal                      bool
)

var reportPrescriber = &cobra.Command{
	Use:     "prescriber",
	Aliases: []string{"pdf"},
	Short:   "Write one page for the prescriber, as a PDF",
	Long: "Write one page for the prescriber: the days asked for, grams a day, which\n" +
		"strains, what the sessions did by the effect log, and what is left of the\n" +
		"fill in progress, with when the next prescription is due. The last 90 days\n" +
		"unless --from and --to say otherwise. With --seal, the journal's head is\n" +
		"sealed first and the page names the seal, so the page can be checked\n" +
		"against the journal it was made from.",
	Example: "  wits report prescriber\n" +
		"  wits report prescriber --from 2026-08-01 --to 2026-10-31 --out appointment.pdf",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		w, err := reportWindow(s.OpenedAt)
		if err != nil {
			return err
		}
		if reportSeal {
			if _, _, err := sealHead(cmd, s); err != nil {
				return err
			}
		}
		out := reportOut
		if out == "" {
			out = "wits-report-" + w.to.Format(time.DateOnly) + ".pdf"
		}
		return toFile(cmd, out, func(f io.Writer) string {
			if err := report.PDF(f, report.Build(s.Workspace, w.from, w.to, version.String())); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "🚨 %v\n", err)
			}
			return "a report for " + w.String()
		})
	},
}

// reportWindow is the days a report covers: those asked for, filled in to
// the 90 days before today, or to today.
func reportWindow(now time.Time) (window, error) {
	w, err := parseWindow(reportFrom, reportTo, now)
	if err != nil {
		return w, err
	}
	if w.to.IsZero() {
		w.to = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	}
	if w.from.IsZero() {
		w.from = w.to.AddDate(0, 0, -89)
	}
	return w, nil
}

func init() {
	f := reportPrescriber.Flags()
	f.StringVar(&reportFrom, "from", "", "the first day, YYYY-MM-DD (default 90 days ago)")
	f.StringVar(&reportTo, "to", "", "the last day, YYYY-MM-DD (default today)")
	f.StringVar(&reportOut, "out", "", "the file to write (default wits-report-<last day>.pdf)")
	f.BoolVar(&reportSeal, "seal", false, "seal the journal's head first, and name the seal on the page")
	Report.AddCommand(Today, Costs, Review, Refill, reportPrescriber)
}
