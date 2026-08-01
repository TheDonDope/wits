package tui

import (
	"fmt"
	"sort"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
)

// analysisView is the long view: how this cycle is going, how it compares to
// the ones before it, and where the grams actually went.
type analysisView struct {
	scope int // 0 this cycle, 1 this year, 2 all time
}

func newAnalysisView() analysisView { return analysisView{} }

var scopes = []string{"this cycle", "last 12 months", "all time"}

type analysisKeys struct {
	keyMap
	Scope key.Binding
}

func (k analysisKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Next, k.Scope, k.Help, k.Quit}
}

func (k analysisKeys) FullHelp() [][]key.Binding {
	return append(k.keyMap.FullHelp(), []key.Binding{k.Scope})
}

func (v analysisView) keys(base keyMap) help.KeyMap {
	return analysisKeys{
		keyMap: base,
		Scope:  key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "scope")),
	}
}

func (v analysisView) Update(msg tea.Msg, _ *App) (analysisView, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok && msg.String() == "s" {
		v.scope = (v.scope + 1) % len(scopes)
	}
	return v, nil
}

func (v analysisView) View(a *App, height int) string {
	t := a.theme
	width := a.inner()
	events := v.events(a)

	if len(events) == 0 {
		return lipgloss.NewStyle().Padding(1, 1).Render(t.Subtitle.Render("Nothing to analyse yet."))
	}

	sections := []string{
		v.summary(a, events, width),
		"",
		t.Rule("Grams per day", width),
		v.perDay(a, events, width),
		"",
		t.Rule("By product", width),
		v.byProduct(a, events, width),
	}
	if v.scope != 0 {
		sections = append(sections,
			"", t.Rule("Rhythm", width), v.rhythm(a, events, width),
			"", t.Rule("Cycles", width), v.cycles(a, width))
	}
	return lipgloss.NewStyle().Padding(1, 1).Render(
		clip(lipgloss.JoinVertical(lipgloss.Left, sections...), height-2))
}

// events returns the events in the selected scope.
func (v analysisView) events(a *App) []journal.Event {
	switch v.scope {
	case 0:
		if c := a.data.Cycle(); c != nil {
			return c.Events
		}
		return nil
	case 1:
		cutoff := a.data.Now.AddDate(-1, 0, 0)
		var out []journal.Event
		for _, e := range a.data.State.Events {
			if e.OccurredAt.After(cutoff) {
				out = append(out, e)
			}
		}
		return out
	default:
		return a.data.State.Events
	}
}

// summary is the headline figures for the scope.
func (v analysisView) summary(a *App, events []journal.Event, width int) string {
	t := a.theme
	stats := ledger.Summarise(events)

	scope := t.Subtitle.Render("Showing ") + t.PanelTitle.Render(scopes[v.scope]) +
		t.Dim.Render("  ·  press s to change")

	cols := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(width/4).Render(
			t.Metric("ground", fmt.Sprintf("%.1f g", stats.Ground), "")),
		lipgloss.NewStyle().Width(width/4).Render(
			t.Metric("active days", fmt.Sprintf("%d", stats.ActiveDays),
				fmt.Sprintf("of %d elapsed", stats.ElapsedDays))),
		lipgloss.NewStyle().Width(width/4).Render(
			t.Metric("per active day", fmt.Sprintf("%.2f g", stats.PerActiveDay),
				fmt.Sprintf("median %.2f g", stats.MedianPerDay))),
		lipgloss.NewStyle().Width(width/4).Render(
			t.Metric("per elapsed day", fmt.Sprintf("%.2f g", stats.PerElapsedDay), "")),
	)
	return lipgloss.JoinVertical(lipgloss.Left, scope, "", cols)
}

// perDay draws the daily amounts as columns across the whole scope.
func (v analysisView) perDay(a *App, events []journal.Event, width int) string {
	t := a.theme
	perDay := map[string]float64{}
	var first, last time.Time
	for _, e := range events {
		if e.Type != journal.Grind {
			continue
		}
		perDay[e.OccurredAt.Format(time.DateOnly)] += e.Grams
		if first.IsZero() || e.OccurredAt.Before(first) {
			first = e.OccurredAt
		}
		if e.OccurredAt.After(last) {
			last = e.OccurredAt
		}
	}
	if first.IsZero() {
		return t.Dim.Render("nothing ground in this range")
	}

	var values []float64
	for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
		values = append(values, perDay[d.Format(time.DateOnly)])
	}

	// The braille area carries two days per cell and the whole range is
	// resampled to the width, so a four-year scope keeps its shape rather than
	// being cut off. The seven-day average rides over it as a line: the daily
	// spikes are the record, the line is the habit.
	chart := AreaChart(values, movingAverage(values, 7), width, 6, t, t.StashC, t.Alt)
	legend := lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Foreground(t.StashC).Render("⣿"), t.Dim.Render(" daily   "),
		lipgloss.NewStyle().Foreground(t.Alt).Render("⠒⠒"), t.Dim.Render(" 7-day average"),
	)
	return lipgloss.JoinVertical(lipgloss.Left, chart,
		axisLabels(t, first.Format("02 Jan 06"), last.Format("02 Jan 06"), width),
		legend)
}

// rhythm draws the longer scopes as a calendar heatmap, one cell per day. The
// per-day chart says how much; this says when — the pauses, the heavy weeks,
// whether weekends differ — which is what a year of habit actually consists of.
func (v analysisView) rhythm(a *App, events []journal.Event, width int) string {
	t := a.theme
	perDay := map[string]float64{}
	var first, last time.Time
	for _, e := range events {
		if e.Type != journal.Grind {
			continue
		}
		perDay[e.OccurredAt.Format(time.DateOnly)] += e.Grams
		if first.IsZero() || e.OccurredAt.Before(first) {
			first = e.OccurredAt
		}
		if e.OccurredAt.After(last) {
			last = e.OccurredAt
		}
	}
	if first.IsZero() {
		return t.Dim.Render("nothing ground in this range")
	}
	return Calendar(perDay, first, last, width, t)
}

// byProduct ranks the products by how much of them was ground.
func (v analysisView) byProduct(a *App, events []journal.Event, width int) string {
	t, data := a.theme, a.data
	totals := map[string]float64{}
	for _, e := range events {
		if e.Type == journal.Grind {
			totals[e.Product] += e.Grams
		}
	}
	slugs := make([]string, 0, len(totals))
	for s := range totals {
		slugs = append(slugs, s)
	}
	sort.Slice(slugs, func(i, j int) bool { return totals[slugs[i]] > totals[slugs[j]] })
	if len(slugs) > 8 {
		slugs = slugs[:8]
	}

	bars := make([]Bar, 0, len(slugs))
	for _, s := range slugs {
		bars = append(bars, Bar{
			Label: truncate(data.ProductName(s), 30),
			Value: totals[s],
			Note:  fmt.Sprintf("%.1f g", totals[s]),
			Color: t.StashC,
		})
	}
	return BarChart(bars, width, t)
}

// cycles compares whole cycles: how much was dispensed, how long it lasted, and
// the rate it went at.
func (v analysisView) cycles(a *App, width int) string {
	t := a.theme
	all := a.data.State.Cycles
	if len(all) == 0 {
		return t.Dim.Render("no cycles yet")
	}
	if len(all) > 12 {
		all = all[len(all)-12:]
	}

	bars := make([]Bar, 0, len(all))
	for _, c := range all {
		stats := ledger.Summarise(c.Events)
		bars = append(bars, Bar{
			Label: c.Start.Format("Jan 2006"),
			Value: c.Ground,
			Note:  fmt.Sprintf("%.0f g · %.2f g/day", c.Ground, stats.PerActiveDay),
			Color: t.StorageC,
		})
	}
	return BarChart(bars, width, t)
}
