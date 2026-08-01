package tui

import (
	"fmt"
	"sort"
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
		fmt.Sprintf("of %.0f g", c.Purchased))

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

	bar := Gauge(width, fraction, t.Level(fraction), t.Dim)
	scale := lipgloss.JoinHorizontal(lipgloss.Left,
		t.Dim.Render(c.Start.Format("02 Jan")),
		strings.Repeat(" ", maxInt(width-12-lipgloss.Width(fmt.Sprintf("%.0f%%", fraction*100)), 1)),
		lipgloss.NewStyle().Foreground(t.Level(fraction)).Render(fmt.Sprintf("%.0f%%", fraction*100)),
	)
	return lipgloss.JoinVertical(lipgloss.Left, cols, "", bar, scale)
}

// products shows each product of this cycle as a stacked bar: what is still
// sealed, what is in its tin, and what has come back as AVB.
func (d dashboard) products(a *App, c *ledger.Cycle, width int) string {
	t, data := a.theme, a.data

	labelW := 0
	for _, slug := range c.Products {
		labelW = maxInt(labelW, len(truncate(data.ProductName(slug), 28)))
	}

	var rows []string
	for _, slug := range c.Products {
		b := data.State.Balances[slug]
		if b == nil {
			continue
		}
		purchased := purchasedOf(c, slug)
		fraction := 0.0
		if purchased > 0 {
			fraction = b.Storage / purchased
		}
		barW := maxInt(width-labelW-24, 8)

		rows = append(rows,
			lipgloss.JoinHorizontal(lipgloss.Left,
				t.Value.Width(labelW).Render(truncate(data.ProductName(slug), 28)),
				" ",
				Stack(barW, []Bar{
					{Value: b.Storage, Color: t.StorageC},
					{Value: b.Stash, Color: t.StashC},
					{Value: b.AVB, Color: t.AVBC},
				}, purchased, t),
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
	cols := make([]Column, 0, span)
	for i := span - 1; i >= 0; i-- {
		day := end.AddDate(0, 0, -i)
		cols = append(cols, Column{Value: perDay[day.Format(time.DateOnly)]})
	}

	chart := ColumnChart(cols, 6, t, t.StashC)
	return lipgloss.JoinVertical(lipgloss.Left, chart,
		axisLabels(t, end.AddDate(0, 0, -span+1).Format("02 Jan"), end.Format("02 Jan"), span))
}

// purchasedOf returns how many grams of a product a cycle began with.
func purchasedOf(c *ledger.Cycle, slug string) float64 {
	var grams float64
	for _, e := range c.Events {
		if e.Product == slug && e.Type == journal.Purchase {
			grams += e.Grams
		}
	}
	return grams
}

// sortedSlugs returns product slugs in a stable order.
func sortedSlugs(m map[string]float64) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return m[out[i]] > m[out[j]] })
	return out
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
