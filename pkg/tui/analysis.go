package tui

import (
	"fmt"
	"sort"
	"strings"
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
	scope  int // an index into scopes
	scroll int // lines scrolled past, since the long scopes overrun a screen
	sel    int // the day cursor, counted back from the last day of the scope
}

func newAnalysisView() analysisView { return analysisView{} }

var scopes = []string{"this cycle", "last 30 days", "last 90 days", "last 12 months", "all time"}

type analysisKeys struct {
	keyMap
	Scope key.Binding
	Slide key.Binding
}

// scopeKey is declared once so the help line and the dispatch cannot drift.
var scopeKey = key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "scope"))

func (k analysisKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Slide, k.Next, k.Scope, k.Help, k.Quit}
}

func (k analysisKeys) FullHelp() [][]key.Binding {
	return append(k.keyMap.FullHelp(), []key.Binding{k.Scope, k.Slide})
}

func (v analysisView) keys(base keyMap) help.KeyMap {
	return analysisKeys{keyMap: base, Scope: scopeKey, Slide: withHelp(slideKey, "day")}
}

func (v analysisView) Update(msg tea.Msg, a *App) (analysisView, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(msg, scopeKey):
			v.scope = (v.scope + 1) % len(scopes)
			v.scroll, v.sel = 0, 0
		case key.Matches(msg, slideKey):
			// Counted back from the last day: left walks into the past.
			switch msg.String() {
			case "left", "h":
				v.sel = min(v.sel+1, max(v.spanDays(a)-1, 0))
			default:
				v.sel = max(v.sel-1, 0)
			}
		case key.Matches(msg, a.keys.Up):
			v.scroll = max(v.scroll-1, 0)
		case key.Matches(msg, a.keys.Down):
			v.scroll++
		case key.Matches(msg, a.keys.PageUp):
			v.scroll = max(v.scroll-10, 0)
		case key.Matches(msg, a.keys.PgDown):
			v.scroll += 10
		case key.Matches(msg, a.keys.Top):
			v.scroll = 0
		}
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
	sections = append(sections, "", t.Rule("Product trails", width), v.trails(a, events, width))
	if v.scope != 0 {
		sections = append(sections, "", t.Rule("Rhythm", width), v.rhythm(a, events, width))
	}
	if v.scope >= 3 {
		sections = append(sections, "", t.Rule("Cycles", width), v.cycles(a, width))
	}
	// The long scopes overrun a screen, so the view scrolls rather than
	// silently cutting the calendar off at whatever height the terminal has.
	lines := strings.Split(lipgloss.JoinVertical(lipgloss.Left, sections...), "\n")
	visible := max(height-2, 1)
	offset := min(v.scroll, max(len(lines)-visible, 0))
	show := visible
	more := offset+visible < len(lines)
	if more {
		// The hint takes the last visible row, so the frame never overruns.
		show = max(visible-1, 1)
	}
	body := strings.Join(lines[offset:min(offset+show, len(lines))], "\n")
	if more {
		body += "\n" + t.Dim.Render("↓ more")
	}
	return lipgloss.NewStyle().Padding(1, 1).Render(body)
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
		return since(a, a.data.Now.AddDate(0, 0, -30))
	case 2:
		return since(a, a.data.Now.AddDate(0, 0, -90))
	case 3:
		return since(a, a.data.Now.AddDate(-1, 0, 0))
	default:
		return a.data.State.Events
	}
}

// spanDays is how many days the scope's grind chart covers, for clamping the
// day cursor.
func (v analysisView) spanDays(a *App) int {
	var first, last time.Time
	for _, e := range v.events(a) {
		if e.Type != journal.Grind {
			continue
		}
		if first.IsZero() || e.OccurredAt.Before(first) {
			first = e.OccurredAt
		}
		if e.OccurredAt.After(last) {
			last = e.OccurredAt
		}
	}
	if first.IsZero() {
		return 0
	}
	return daysBetween(first, last) + 1
}

// since returns the events that happened after the cutoff.
func since(a *App, cutoff time.Time) []journal.Event {
	var out []journal.Event
	for _, e := range a.data.State.Events {
		if e.OccurredAt.After(cutoff) {
			out = append(out, e)
		}
	}
	return out
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
	chart := AreaWithAxis(values, movingAverage(values, 7), width, 6, t, t.StashC, t.Alt)
	legend := lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Foreground(t.StashC).Render("⣿"), t.Dim.Render(" daily   "),
		lipgloss.NewStyle().Foreground(t.Alt).Render("⠒⠒"), t.Dim.Render(" 7-day average"),
		t.Dim.Render("   ←/→ walks the days"),
	)

	// The day cursor: its figure above the chart, its date lit in the axis.
	chartW := max(width-axisGutter, 12)
	dayIdx := max(len(values)-1-v.sel, 0)
	day := first.AddDate(0, 0, dayIdx)
	col := min(dayIdx*(chartW*2)/max(len(values), 1)/2, chartW-1)
	marker := strings.Repeat(" ", axisGutter) +
		pointerRow(t, chartW, col, fmt.Sprintf("%.2f g · %s", values[dayIdx], day.Format("Mon 02 Jan")))
	axis := strings.Repeat(" ", axisGutter) +
		dateAxisCursor(t, first, last, chartW, col, day.Format("02 Jan"))

	return lipgloss.JoinVertical(lipgloss.Left, marker, chart, axis, legend)
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

// byProduct ranks the products by how much of them was ground, each under its
// full name — the name is what the jar is called, and the bars can shrink —
// with the share, the active days and the rate alongside.
func (v analysisView) byProduct(a *App, events []journal.Event, width int) string {
	t, data := a.theme, a.data
	totals := map[string]float64{}
	days := map[string]map[string]bool{}
	ground := 0.0
	for _, e := range events {
		if e.Type != journal.Grind {
			continue
		}
		totals[e.Product] += e.Grams
		ground += e.Grams
		if days[e.Product] == nil {
			days[e.Product] = map[string]bool{}
		}
		days[e.Product][e.OccurredAt.Format(time.DateOnly)] = true
	}
	slugs := make([]string, 0, len(totals))
	for s := range totals {
		slugs = append(slugs, s)
	}
	sort.Slice(slugs, func(i, j int) bool { return totals[slugs[i]] > totals[slugs[j]] })

	bars := make([]Bar, 0, len(slugs))
	for _, s := range slugs {
		active := len(days[s])
		bars = append(bars, Bar{
			Label: data.ProductName(s),
			Value: totals[s],
			Note: fmt.Sprintf("%.1f g · %2.0f%% · %s · %.2f g/day",
				totals[s], totals[s]/ground*100, plural(active, "day"),
				totals[s]/float64(max(active, 1))),
			Color: t.StashC,
		})
	}
	return BarChart(bars, width, t)
}

// trails is the per-product breakdown over time: the heaviest products, each
// with the shape of its own days across the scope.
func (v analysisView) trails(a *App, events []journal.Event, width int) string {
	t, data := a.theme, a.data
	perProduct := map[string]map[string]float64{}
	totals := map[string]float64{}
	var first, last time.Time
	for _, e := range events {
		if e.Type != journal.Grind {
			continue
		}
		if perProduct[e.Product] == nil {
			perProduct[e.Product] = map[string]float64{}
		}
		perProduct[e.Product][e.OccurredAt.Format(time.DateOnly)] += e.Grams
		totals[e.Product] += e.Grams
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
	slugs := make([]string, 0, len(totals))
	for s := range totals {
		slugs = append(slugs, s)
	}
	sort.Slice(slugs, func(i, j int) bool { return totals[slugs[i]] > totals[slugs[j]] })
	if len(slugs) > 4 {
		slugs = slugs[:4]
	}

	var lines []string
	for _, slug := range slugs {
		var values []float64
		for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
			values = append(values, perProduct[slug][d.Format(time.DateOnly)])
		}
		lines = append(lines,
			t.Value.Render(data.ProductName(slug))+t.Dim.Render(fmt.Sprintf("  %.1f g", totals[slug])),
			Sparkline(values, width, lipgloss.NewStyle().Foreground(t.StashC)))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
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

// sessionsView is where the sessions live: how much came out of the stash,
// when, through which device and at what temperature. It is the view the
// spreadsheet never had — the imported years recorded only grinding — so it
// grows as sessions are actually logged.
type sessionsView struct {
	scroll int
}

func newSessionsView() sessionsView { return sessionsView{} }

func (v sessionsView) keys(base keyMap) help.KeyMap { return base }

func (v sessionsView) Update(msg tea.Msg, a *App) (sessionsView, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(msg, a.keys.Up):
			v.scroll = max(v.scroll-1, 0)
		case key.Matches(msg, a.keys.Down):
			v.scroll++
		case key.Matches(msg, a.keys.PageUp):
			v.scroll = max(v.scroll-10, 0)
		case key.Matches(msg, a.keys.PgDown):
			v.scroll += 10
		case key.Matches(msg, a.keys.Top):
			v.scroll = 0
		}
	}
	return v, nil
}

// seshes returns the session events, oldest first.
func seshes(a *App) []journal.Event {
	var out []journal.Event
	for _, e := range a.data.State.Events {
		if e.Type == journal.Sesh {
			out = append(out, e)
		}
	}
	return out
}

func (v sessionsView) View(a *App, height int) string {
	t := a.theme
	width := a.inner()
	events := seshes(a)

	if len(events) == 0 {
		return lipgloss.NewStyle().Padding(1, 1).Render(
			t.Subtitle.Render("No sessions logged yet.\n\nPress ") + t.Key.Render("s") +
				t.Subtitle.Render(" to record one, or `wits sesh` from the shell.\nThe imported years only recorded grinding, so this view starts here."))
	}

	sections := []string{
		v.summary(a, events, width),
		"",
		t.Rule("Seshed per day", width),
		v.perDay(a, events, width),
		"",
		t.Rule("By device", width),
		v.byDevice(a, events, width),
		"",
		t.Rule("Rhythm", width),
		v.rhythm(a, events, width),
	}

	lines := strings.Split(lipgloss.JoinVertical(lipgloss.Left, sections...), "\n")
	visible := max(height-2, 1)
	offset := min(v.scroll, max(len(lines)-visible, 0))
	show := visible
	more := offset+visible < len(lines)
	if more {
		show = max(visible-1, 1)
	}
	body := strings.Join(lines[offset:min(offset+show, len(lines))], "\n")
	if more {
		body += "\n" + t.Dim.Render("↓ more")
	}
	return lipgloss.NewStyle().Padding(1, 1).Render(body)
}

// summary is the headline figures: how many sessions, how much they drew, and
// what a typical one looks like.
func (v sessionsView) summary(a *App, events []journal.Event, width int) string {
	t := a.theme
	total, days := 0.0, map[string]bool{}
	temps, tempCount := 0, 0
	for _, e := range events {
		total += e.Grams
		days[e.OccurredAt.Format(time.DateOnly)] = true
		if e.Temperature > 0 {
			temps += e.Temperature
			tempCount++
		}
	}
	perSession := total / float64(len(events))
	avgTemp := "—"
	if tempCount > 0 {
		avgTemp = fmt.Sprintf("%d°C", temps/tempCount)
	}

	cols := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(width/4).Render(
			t.Metric("sessions", fmt.Sprintf("%d", len(events)),
				fmt.Sprintf("across %s", plural(len(days), "day")))),
		lipgloss.NewStyle().Width(width/4).Render(
			t.Metric("seshed", fmt.Sprintf("%.1f g", total), "out of the stash")),
		lipgloss.NewStyle().Width(width/4).Render(
			t.Metric("per session", fmt.Sprintf("%.2f g", perSession), "")),
		lipgloss.NewStyle().Width(width/4).Render(
			t.Metric("avg temp", avgTemp, "where one was set")),
	)
	return cols
}

// perDay draws the grams seshed per day as the braille area the analysis view
// uses, with the same seven-day average riding over it.
func (v sessionsView) perDay(a *App, events []journal.Event, width int) string {
	t := a.theme
	perDay := map[string]float64{}
	var first, last time.Time
	for _, e := range events {
		perDay[e.OccurredAt.Format(time.DateOnly)] += e.Grams
		if first.IsZero() || e.OccurredAt.Before(first) {
			first = e.OccurredAt
		}
		if e.OccurredAt.After(last) {
			last = e.OccurredAt
		}
	}
	var values []float64
	for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
		values = append(values, perDay[d.Format(time.DateOnly)])
	}
	chart := AreaChart(values, movingAverage(values, 7), width, 6, t, t.SeshC, t.Alt)
	return lipgloss.JoinVertical(lipgloss.Left, chart,
		axisLabels(t, first.Format("02 Jan 06"), last.Format("02 Jan 06"), width))
}

// deviceUsage is what the sessions drew through one device.
type deviceUsage struct {
	grams float64
	count int
	temps int
	tSum  int
}

// usageByDevice aggregates the sessions per device, heaviest first. Sessions
// without a device are owned rather than dropped: they happened.
func usageByDevice(events []journal.Event) ([]string, map[string]*deviceUsage) {
	byDev := map[string]*deviceUsage{}
	for _, e := range events {
		name := e.Device
		if name == "" {
			name = "no device"
		}
		u, ok := byDev[name]
		if !ok {
			u = &deviceUsage{}
			byDev[name] = u
		}
		u.grams += e.Grams
		u.count++
		if e.Temperature > 0 {
			u.tSum += e.Temperature
			u.temps++
		}
	}
	names := make([]string, 0, len(byDev))
	for n := range byDev {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool { return byDev[names[i]].grams > byDev[names[j]].grams })
	return names, byDev
}

// byDevice ranks the devices by the grams drawn through them, with how often
// and how hot alongside — which is what device usage actually consists of.
func (v sessionsView) byDevice(a *App, events []journal.Event, width int) string {
	t := a.theme
	names, byDev := usageByDevice(events)
	bars := make([]Bar, 0, len(names))
	for _, n := range names {
		u := byDev[n]
		note := plural(u.count, "session")
		if u.temps > 0 {
			note = fmt.Sprintf("%s · avg %d°C", note, u.tSum/u.temps)
		}
		label := n
		if a.data.Devices != nil {
			if d, err := a.data.Devices.Find(n); err == nil {
				label = d.Name
			}
		}
		bars = append(bars, Bar{
			Label: label,
			Value: u.grams,
			Note:  fmt.Sprintf("%.2f g · %s", u.grams, note),
			Color: t.SeshC,
		})
	}
	return BarChart(bars, width, t)
}

// rhythm is the calendar of session days, in the same heat the analysis
// screen reads consumption in.
func (v sessionsView) rhythm(a *App, events []journal.Event, width int) string {
	t := a.theme
	perDay := map[string]float64{}
	var first, last time.Time
	for _, e := range events {
		perDay[e.OccurredAt.Format(time.DateOnly)] += e.Grams
		if first.IsZero() || e.OccurredAt.Before(first) {
			first = e.OccurredAt
		}
		if e.OccurredAt.After(last) {
			last = e.OccurredAt
		}
	}
	return Calendar(perDay, first, last, width, t)
}
