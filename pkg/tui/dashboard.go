package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
)

// dashboard is the screen you land on: how much is left, how fast it is going,
// and whether it will last.
type dashboard struct{}

func newDashboard() dashboard { return dashboard{} }

func (d dashboard) Update(tea.Msg, *App) (dashboard, tea.Cmd) { return d, nil }

func (d dashboard) View(a *App, height int) string {
	t, data := a.theme, a.data
	width := a.inner()

	cycle := data.Cycle()
	if cycle == nil {
		return lipgloss.NewStyle().Padding(1, 1).Render(
			t.Subtitle.Render("No cycle yet.\n\nRecord a prescription fill with ") +
				t.Key.Render("wits buy") + t.Subtitle.Render(" to get started."))
	}

	stats := ledger.Summarise(cycle.Events)
	sections := []string{
		d.headline(a, cycle, stats, width),
		"",
		d.actions(a, width),
		"",
		t.Rule("Supply", width),
		d.products(a, cycle, width),
		"",
		t.Rule("Last 30 days", width),
		d.recent(a, width),
	}
	return lipgloss.NewStyle().Padding(1, 1).Render(
		clip(lipgloss.JoinVertical(lipgloss.Left, sections...), height-2))
}

// headline is the three figures worth reading first, with the cycle's own
// progress bar underneath.
func (d dashboard) headline(a *App, c *ledger.Cycle, stats ledger.Stats, width int) string {
	t := a.theme
	remaining := c.Remaining()
	fraction := c.RemainingPct()
	daysLeft := stats.DaysLeft(remaining)

	left := t.Metric("remaining",
		fmt.Sprintf("%.2f g", remaining),
		fmt.Sprintf("of %.0f g", c.Held()))

	rate := t.Metric("per active day",
		fmt.Sprintf("%.2f g", stats.PerActiveDay),
		fmt.Sprintf("median %.2f g", stats.MedianPerDay))

	runway := "—"
	note := "nothing ground yet"
	if daysLeft > 0 {
		runway = fmt.Sprintf("%.0f days", daysLeft)
		note = "at the observed rate"
	}
	last := t.Metric("supply lasts", runway, note)

	cols := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(width/3).Render(left),
		lipgloss.NewStyle().Width(width/3).Render(rate),
		lipgloss.NewStyle().Width(width/3).Render(last),
	)

	bar := GradientGauge(width, fraction, t.Good, t.Level(fraction), t.Dim)
	scale := lipgloss.JoinHorizontal(lipgloss.Left,
		t.Dim.Render(c.Start.Format("02 Jan")),
		strings.Repeat(" ", max(width-12-lipgloss.Width(fmt.Sprintf("%.0f%%", fraction*100)), 1)),
		lipgloss.NewStyle().Foreground(t.Level(fraction)).Render(fmt.Sprintf("%.0f%%", fraction*100)),
	)
	return lipgloss.JoinVertical(lipgloss.Left, cols, "", bar, scale)
}

// actions is the row of things worth doing from here, spelled out rather than
// left in the help line. Recording the day's grind is the whole point of opening
// this, and a key hint at the bottom of the screen is easy to never read.
func (d dashboard) actions(a *App, width int) string {
	t := a.theme
	type action struct{ key, label string }
	actions := []action{
		{"n", "grind"},
		{"s", "sesh"},
		{"b", "fill"},
		{"r", "weigh"},
	}

	cells := make([]string, 0, len(actions))
	for _, act := range actions {
		cells = append(cells, lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).BorderForeground(t.Line).
			Padding(0, 1).Render(
			lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Render(act.key)+
				t.Dim.Render("  "+act.label)))
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
	if lipgloss.Width(row) > width {
		// Too narrow for the boxes; the help line still carries the same keys.
		return ""
	}
	return row
}

// products shows each product of this cycle as a stacked bar: what is still
// sealed, what is in its tin, and what has come back as AVB.
func (d dashboard) products(a *App, c *ledger.Cycle, width int) string {
	t, data := a.theme, a.data

	labelW := 0
	for _, slug := range c.Products {
		labelW = max(labelW, len(truncate(data.ProductName(slug), 28)))
	}

	var rows []string
	for _, slug := range c.Products {
		b := data.State.Balances[slug]
		if b == nil {
			continue
		}
		// Carry-over included, so grinding down last month's remainder does
		// not push a product past 100%.
		held := c.HeldOf(slug)
		fraction := 0.0
		if held > 0 {
			fraction = b.Storage / held
		}
		barW := max(width-labelW-24, 8)

		rows = append(rows,
			lipgloss.JoinHorizontal(lipgloss.Left,
				t.Value.Width(labelW).Render(truncate(data.ProductName(slug), 28)),
				" ",
				Stack(barW, []Bar{
					{Value: b.Storage, Color: t.StorageC},
					{Value: b.Stash, Color: t.StashC},
					{Value: b.AVB, Color: t.AVBC},
				}, held, t),
				" ",
				t.Grams(b.Storage),
				" ",
				lipgloss.NewStyle().Foreground(t.Level(fraction)).Render(fmt.Sprintf("%3.0f%%", fraction*100)),
			))
	}
	legend := lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Foreground(t.StorageC).Render("█"), t.Dim.Render(" storage  "),
		lipgloss.NewStyle().Foreground(t.StashC).Render("█"), t.Dim.Render(" stash  "),
		lipgloss.NewStyle().Foreground(t.AVBC).Render("█"), t.Dim.Render(" avb"),
	)
	return lipgloss.JoinVertical(lipgloss.Left, append(rows, "", legend)...)
}

// recent draws the last 30 days as columns, so the rhythm of the month is
// visible: the heavy days, the gaps, the trend.
func (d dashboard) recent(a *App, _ int) string {
	t, data := a.theme, a.data

	const span = 30
	end := data.Now
	perDay := map[string]float64{}
	for _, e := range data.State.Events {
		if e.Type == journal.Grind {
			perDay[e.OccurredAt.Format(time.DateOnly)] += e.Grams
		}
	}
	peak := 0.0
	for i := 0; i < span; i++ {
		if v := perDay[end.AddDate(0, 0, -i).Format(time.DateOnly)]; v > peak {
			peak = v
		}
	}
	cols := make([]Column, 0, span)
	for i := span - 1; i >= 0; i-- {
		v := perDay[end.AddDate(0, 0, -i).Format(time.DateOnly)]
		c := Column{Value: v}
		// Heavier days burn hotter, so the week that got away with itself is
		// visible before the axis is read.
		if peak > 0 && v > 0 {
			c.Color = heatAt(t, v/peak)
		}
		cols = append(cols, c)
	}

	chart := ColumnChart(cols, 6, t, t.StashC)
	return lipgloss.JoinVertical(lipgloss.Left, chart,
		axisLabels(t, end.AddDate(0, 0, -span+1).Format("02 Jan"), end.Format("02 Jan"), span))
}

// clip trims rendered content to a number of lines.
func clip(s string, lines int) string {
	if lines <= 0 {
		return ""
	}
	rows := strings.Split(s, "\n")
	if len(rows) <= lines {
		return s
	}
	return strings.Join(rows[:lines], "\n")
}
