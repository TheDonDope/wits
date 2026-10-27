// Package tui draws the Insights pages here, each from the package the
// command line and the browser read the same figures from: today, prices,
// review, views and report.
package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/prices"
	"github.com/TheDonDope/wits/pkg/review"
	"github.com/TheDonDope/wits/pkg/today"
	"github.com/TheDonDope/wits/pkg/views"
)

// table lines up rows of cells under a header, each column as wide as its
// widest cell; the header is drawn as labels.
func table(a *App, header []string, rows [][]string) string {
	widths := make([]int, len(header))
	for _, r := range append([][]string{header}, rows...) {
		for i, c := range r {
			widths[i] = max(widths[i], lipgloss.Width(c))
		}
	}
	line := func(r []string, style lipgloss.Style) string {
		cells := make([]string, len(r))
		for i, c := range r {
			cells[i] = style.Render(c + strings.Repeat(" ", widths[i]-lipgloss.Width(c)))
		}
		return strings.Join(cells, "  ")
	}
	out := []string{line(header, a.theme.Label)}
	for _, r := range rows {
		out = append(out, line(r, a.theme.Value))
	}
	return strings.Join(out, "\n")
}

// todayPage is the day so far, as `wits today` says it: every session with
// the THC and CBD in the material, their sum against a usual day.
func todayPage(a *App, width int) string {
	t := a.theme
	d := today.Of(a.data.Workspace, a.data.Now)
	head := t.Big.Render(fmt.Sprintf("%.0f mg", d.THC)) + t.Dim.Render(" THC in the material  ·  ") + todaySummary(d)
	rows := make([][]string, 0, len(d.Sessions))
	for _, s := range d.Sessions {
		rows = append(rows, []string{s.At.Format("15:04"), truncate(s.Name, 32), deviceAt(s), fmt.Sprintf("%.2f g", s.Grams),
			milligrams(s.THC, s.Known), milligrams(s.CBD, s.Known), stars(s.Overall)})
	}
	out := []string{head, ""}
	if len(rows) > 0 {
		out = append(out, table(a, []string{"TIME", "PRODUCT", "DEVICE", "GRAMS", "THC", "CBD", "RATED"}, rows), "")
	}
	if u := d.Usual; u.Days > 0 {
		out = append(out, Gauge(min(width-30, 40), min(d.THC/max(u.THC, 1), 1), t.SeshC, t.Dim)+
			t.Dim.Render(fmt.Sprintf("  a usual day: %.2f g, %.0f mg THC (%s of the last 30)", u.Seshed, u.THC, plural(u.Days, "day"))))
	} else {
		out = append(out, t.Dim.Render("No session in the 30 days before to compare with."))
	}
	if d.Unknown > 0 {
		out = append(out, t.Dim.Render(plural(d.Unknown, "session")+" of a strain with no THC written down; e on storage gives it one."))
	}
	return strings.Join(append(out, t.Dim.Render("What was in the material, not what reached the body.")), "\n")
}

// todaySummary is the day in one line, for the dashboard too.
func todaySummary(d today.Today) string {
	if len(d.Sessions) == 0 {
		return "no session yet today"
	}
	out := fmt.Sprintf("%s, %.2f g through a device", plural(len(d.Sessions), "session"), d.Seshed)
	if d.Ground > 0 {
		out += fmt.Sprintf(", %.2f g ground", d.Ground)
	}
	return out
}

func deviceAt(s today.Session) string {
	switch {
	case s.Device == "":
		return "—"
	case s.Temp > 0:
		return fmt.Sprintf("%s at %d°C", truncate(s.Device, 22), s.Temp)
	}
	return truncate(s.Device, 22)
}

func milligrams(v float64, known bool) string {
	if !known {
		return "?"
	}
	return fmt.Sprintf("%.1f mg", v)
}

func stars(n int) string {
	if n == 0 {
		return "—"
	}
	return fmt.Sprintf("%d of 5", n)
}

// costsPage is what the prescriptions cost, as `wits costs` says it: in all
// and a gram, the newest fills, and month by month.
func costsPage(a *App, width int) string {
	t := a.theme
	c := prices.Analyse(a.data.State, prices.Latest(a.data.Prices), a.data.Ratings)
	if c.Priced == 0 {
		return t.Dim.Render(fmt.Sprintf("No purchase has a price yet; %s without one. $ walks them.", plural(c.Unpriced, "purchase")))
	}
	head := t.Big.Render(prices.Format(c.Cents)) + t.Dim.Render(fmt.Sprintf("  for %.2f g with a price, %s a gram",
		c.Priced, prices.Format(int64(math.Round(c.PerGram())))))
	if c.Unpriced > 0 {
		head += t.Dim.Render(fmt.Sprintf("; %s without one", plural(c.Unpriced, "purchase")))
	}
	var fills [][]string
	for i := len(c.Fills) - 1; i >= 0 && len(fills) < 8; i-- {
		f := c.Fills[i]
		paid, per := "—", "—"
		if f.Priced > 0 {
			paid, per = prices.Format(f.Cents), prices.Format(int64(math.Round(f.PerGram())))
		}
		fills = append(fills, []string{fmt.Sprint(f.Seq + 1), f.Start.Format(time.DateOnly), fmt.Sprintf("%.2f g", f.Dispensed), paid, per})
	}
	out := []string{head, "", table(a, []string{"FILL", "DISPENSED", "GRAMS", "PAID", "A GRAM"}, fills), "", monthsPaid(a, c, width)}
	if c.Helped > 0 {
		out = append(out, "", t.Dim.Render(fmt.Sprintf("%s that helped (4 or more overall), %s each on average.",
			plural(c.Helped, "session"), prices.Format(int64(math.Round(c.PerHelped()))))))
	}
	return strings.Join(out, "\n")
}

// monthsPaid is the last twelve months' payments as bars.
func monthsPaid(a *App, c prices.Costs, width int) string {
	months := c.Months[max(len(c.Months)-12, 0):]
	var top int64
	for _, m := range months {
		top = max(top, m.Cents)
	}
	lines := []string{a.theme.Label.Render("MONTH")}
	for i := len(months) - 1; i >= 0; i-- {
		m := months[i]
		lines = append(lines, fmt.Sprintf("%s  %s  %s", m.Month,
			Gauge(min(width-30, 36), float64(m.Cents)/float64(max(top, 1)), a.theme.Accent, a.theme.Dim), prices.Format(m.Cents)))
	}
	return strings.Join(lines, "\n")
}

// yearPage is the year on one card, as `wits review` draws it: the
// headline, the months as bars, and the year's findings.
func yearPage(a *App, year, width int) string {
	t := a.theme
	if year == 0 {
		year = a.data.Now.Year()
	}
	r := review.Year(a.data.Workspace, year)
	if r.Ground == 0 && r.Sessions == 0 && r.Dispensed == 0 {
		return t.Dim.Render(fmt.Sprintf("Nothing was recorded in %d; - steps back a year.", year))
	}
	title := fmt.Sprintf("%d in wits", r.Year)
	if r.Running {
		title += ", to " + r.To.Format("2 January")
	}
	out := []string{
		t.PanelTitle.Render(title),
		t.Big.Render(fmt.Sprintf("%.2f g", r.Ground)) + t.Dim.Render(fmt.Sprintf(" ground on %d of %d days · %.2f g an active day",
			r.GrindDays, r.Days, r.PerGrindDay)),
		t.Dim.Render(fmt.Sprintf("%s · %s · %s", plural(r.Pickups, "prescription"), plural(r.Sessions, "session"), plural(r.Products, "strain"))),
		"", yearMonths(a, r, width), "",
	}
	rows := make([][]string, 0, 12)
	for _, l := range r.Findings() {
		rows = append(rows, []string{l[0], l[1]})
	}
	return strings.Join(append(out, table(a, []string{"", ""}, rows)), "\n")
}

var monthNames = [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// yearMonths is the grams ground month by month, the busiest marked.
func yearMonths(a *App, r review.Review, width int) string {
	top, last := 0.0, 12
	for _, g := range r.Months {
		top = max(top, g)
	}
	if r.Running {
		last = int(r.To.Month())
	}
	lines := make([]string, 0, last)
	for i := range last {
		mark := ""
		if g := r.Months[i]; g == top && g > 0 {
			mark = a.theme.Dim.Render("  ← the most")
		}
		lines = append(lines, fmt.Sprintf("%s  %s %6.2f g%s", monthNames[i],
			Gauge(min(width-30, 36), r.Months[i]/max(top, 0.01), a.theme.StashC, a.theme.Dim), r.Months[i], mark))
	}
	return strings.Join(lines, "\n")
}

// historyPage is the last year as a calendar of what was ground each day,
// and the report for the prescriber that w writes.
func historyPage(a *App, width int) string {
	t := a.theme
	ground := views.Daily(a.data.State.Standing, journal.Grind)
	perDay := make(map[string]float64, len(ground))
	for d, g := range ground {
		perDay[d] = g.Grams()
	}
	to := a.data.Now
	return strings.Join([]string{
		t.PanelTitle.Render("A year of grinding, day by day"),
		Calendar(perDay, to.AddDate(-1, 0, 1), to, width, t),
		"",
		t.Dim.Render(fmt.Sprintf("w writes the report for the prescriber, the last 90 days: %s", reportName(to))),
	}, "\n")
}

// reportName is the report's file, named for its last day as `wits report` names it.
func reportName(to time.Time) string { return "wits-report-" + to.Format(time.DateOnly) + ".pdf" }
