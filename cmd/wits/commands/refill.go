// Package commands holds `wits refill` here: when to order the next prescription.
package commands

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/repo"
	"github.com/spf13/cobra"
)

var (
	refillLead int
	refillICS  string
)

func init() {
	Refill.Flags().IntVar(&refillLead, "lead", repo.DefaultLead, "days the pharmacy takes; kept for next time")
	Refill.Flags().StringVar(&refillICS, "ics", "", "write a calendar reminder to this file")
}

// Refill is the `wits refill` command.
var Refill = &cobra.Command{
	Use:   "refill",
	Short: "Say when to order the next prescription",
	Long: "Say when the shelf runs out at the fill's rate — the supply projection's\n" +
		"empty day — and so, less the pharmacy's lead time, when to order the next\n" +
		"prescription. --lead sets the lead time and keeps it; it starts at 3 days.\n" +
		"--ics writes a calendar reminder for the day to order by.",
	Example: "  wits refill\n" +
		"  wits refill --lead 5 --ics refill.ics",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := open()
		if err != nil {
			return err
		}
		if cmd.Flags().Changed("lead") {
			if refillLead < 0 {
				return fmt.Errorf("--lead %d: a lead time is a number of days, 0 or more", refillLead)
			}
			if err := s.Repo.SetLead(refillLead); err != nil {
				return err
			}
		}
		plan, ok := ledger.PlanRefill(s.Cycle(), s.OpenedAt, s.Repo.Config.Lead())
		if !ok {
			return errors.New("no fill in progress with grinding to go by: nothing to project")
		}
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "%.2f g on the shelf, at %.2f g a day with grinding: empty on %s.\n",
			plan.Shelf, plan.Rate, plan.EmptyOn.Format("Mon 2 January"))
		fmt.Fprintln(out, orderLine(plan, s.OpenedAt))
		if refillICS == "" {
			return nil
		}
		if err := os.WriteFile(refillICS, []byte(calendar(plan, s.OpenedAt)), 0600); err != nil {
			return err
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Wrote a reminder for %s to %s\n", plan.OrderBy.Format(time.DateOnly), refillICS)
		return nil
	},
}

// orderLine says when to order, or that the day has passed.
func orderLine(plan ledger.Refill, now time.Time) string {
	lead := plural(plan.Lead, "day")
	if plan.Overdue(now) {
		return fmt.Sprintf("Order the next prescription now: with %s for the pharmacy, %s has passed.",
			lead, plan.OrderBy.Format("Mon 2 January"))
	}
	return fmt.Sprintf("Order the next prescription by %s, with %s for the pharmacy.",
		plan.OrderBy.Format("Mon 2 January"), lead)
}

// calendar is a reminder for the day to order by, as an iCalendar file: an
// all-day event with an alarm at nine that morning. Its UID names the day, so
// importing a later reminder for the same day replaces it.
func calendar(plan ledger.Refill, now time.Time) string {
	const day = "20060102"
	lines := []string{
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//wits//refill//EN", "CALSCALE:GREGORIAN",
		"BEGIN:VEVENT",
		"UID:refill-" + plan.OrderBy.Format(day) + "@wits",
		"DTSTAMP:" + now.UTC().Format("20060102T150405Z"),
		"DTSTART;VALUE=DATE:" + plan.OrderBy.Format(day),
		"DTEND;VALUE=DATE:" + plan.OrderBy.AddDate(0, 0, 1).Format(day),
		"SUMMARY:Order the next prescription",
		fmt.Sprintf("DESCRIPTION:The shelf runs out on %s at %.2f g a day\\, and the pharmacy takes %s.",
			plan.EmptyOn.Format("2 January"), plan.Rate, plural(plan.Lead, "day")),
		"BEGIN:VALARM", "ACTION:DISPLAY", "DESCRIPTION:Order the next prescription", "TRIGGER:PT9H", "END:VALARM",
		"END:VEVENT", "END:VCALENDAR",
	}
	return strings.Join(lines, "\r\n") + "\r\n"
}
