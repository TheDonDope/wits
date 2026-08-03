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
	player
}

func newAnalysisView() analysisView { return analysisView{player: newPlayer()} }

// player is the replay transport shared by every screen that can play the
// ledger back: where the tape stands, whether it runs on its own, how fast.
// Everything on these screens is derived from an append-only log, so any of
// them can start from empty and grow the way the record grew.
type player struct {
	playhead int  // events applied; -1 means live, everything applied
	playing  bool // whether the playback is running on its own
	speed    int  // an index into playSpeeds
}

func newPlayer() player { return player{playhead: -1, speed: 2} }

// playSpeeds are the autoplay intervals, slowest first.
var playSpeeds = []time.Duration{
	800 * time.Millisecond, 400 * time.Millisecond, 200 * time.Millisecond,
	100 * time.Millisecond, 50 * time.Millisecond,
}

// playTickMsg advances a running playback by one event.
type playTickMsg struct{}

// playKey and speedKeys drive the playback on every screen that has one; the
// horizontal keys step it. Play sits on p rather than space, which stays free
// for marking wherever there are rows to mark.
var (
	playKey   = key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "play/pause"))
	speedKeys = key.NewBinding(key.WithKeys("+", "=", "-", "_"), key.WithHelp("+/-", "speed"))
)

var scopes = []string{"this cycle", "last 30 days", "last 90 days", "last 12 months", "all time"}

type analysisKeys struct {
	keyMap
	Scope key.Binding
	Play  key.Binding
	Step  key.Binding
	Speed key.Binding
}

// scopeKey is declared once so the help line and the dispatch cannot drift.
var scopeKey = key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "scope"))

func (k analysisKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Play, k.Step, k.Speed, k.Scope, k.Help, k.Quit}
}

func (k analysisKeys) FullHelp() [][]key.Binding {
	return append(k.keyMap.FullHelp(), []key.Binding{k.Play, k.Step, k.Speed, k.Scope})
}

func (v analysisView) keys(base keyMap) help.KeyMap {
	return analysisKeys{keyMap: base, Scope: scopeKey,
		Play: playKey, Step: withHelp(slideKey, "step"), Speed: speedKeys}
}

func (v analysisView) Update(msg tea.Msg, a *App) (analysisView, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case playTickMsg:
		v.player, cmd = v.player.advance(v.events(a))
		return v, cmd
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, scopeKey):
			v.scope = (v.scope + 1) % len(scopes)
			v.scroll, v.player = 0, newPlayer()
			v.speed = 2
		case key.Matches(msg, playKey):
			v.player, cmd = v.player.toggle(v.events(a))
			return v, cmd
		case key.Matches(msg, slideKey):
			v.player = v.player.step(msg, v.events(a))
		case key.Matches(msg, speedKeys):
			v.player = v.player.retune(msg)
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

// skippable reports whether an event rides along silently in a replay rather
// than getting a frame of its own. Adjustments are corrections — reconciled
// scales, cleaned history — and a replay narrating each one reads as spam
// while the bars barely move. They are still applied with the prefix, so the
// balances stay true; they just never take the stage.
func skippable(e journal.Event) bool { return e.Type == journal.Adjust }

// toggle starts the replay — from the beginning when the screen was live or
// already played out — or pauses one that is running.
func (p player) toggle(events []journal.Event) (player, tea.Cmd) {
	if p.playing {
		p.playing = false
		return p, nil
	}
	if len(events) == 0 {
		return p, nil
	}
	if p.playhead < 0 || p.playhead >= len(events) {
		p.playhead = 0
	}
	p.playing = true
	return p, p.tick()
}

// advance applies the next event of a running playback and schedules the one
// after it, swallowing the skippable ones so every frame shows a real event,
// and settling back to live when the record runs out.
func (p player) advance(events []journal.Event) (player, tea.Cmd) {
	if !p.playing {
		return p, nil
	}
	p.playhead++
	for p.playhead < len(events) && skippable(events[p.playhead-1]) {
		p.playhead++
	}
	if p.playhead >= len(events) {
		p.playhead, p.playing = -1, false
		return p, nil
	}
	return p, p.tick()
}

// step moves the playback by hand, pausing it: right applies the next event,
// left takes the last one back, both passing over the skippable ones the way
// the clock does. Stepping past the end settles on live.
func (p player) step(msg tea.KeyPressMsg, events []journal.Event) player {
	total := len(events)
	if total == 0 {
		return p
	}
	p.playing = false
	at := p.playhead
	if at < 0 {
		at = total
	}
	if msg.String() == "left" || msg.String() == "h" {
		at--
		for at > 0 && skippable(events[at-1]) {
			at--
		}
		at = max(at, 0)
	} else {
		at++
		for at < total && skippable(events[at-1]) {
			at++
		}
	}
	if at >= total {
		p.playhead = -1
		return p
	}
	p.playhead = at
	return p
}

// retune moves the autoplay speed a notch either way.
func (p player) retune(msg tea.KeyPressMsg) player {
	if msg.String() == "-" || msg.String() == "_" {
		p.speed = max(p.speed-1, 0)
	} else {
		p.speed = min(p.speed+1, len(playSpeeds)-1)
	}
	return p
}

// played returns the prefix of the events the playhead has applied, or all of
// them when the screen is live.
func (p player) played(events []journal.Event) []journal.Event {
	if p.playhead < 0 {
		return events
	}
	return events[:min(p.playhead, len(events))]
}

// tick schedules the next playback step at the current speed.
func (p player) tick() tea.Cmd {
	return tea.Tick(playSpeeds[p.speed], func(time.Time) tea.Msg { return playTickMsg{} })
}

// transport is the playback line: where the replay stands, how fast it runs,
// and the event it just applied — the ledger telling its own story.
func (p player) transport(a *App, events []journal.Event, width int) string {
	t := a.theme
	if p.playhead < 0 {
		return t.Dim.Render("live · press ") + t.Key.Render("p") +
			t.Dim.Render(" to replay the ledger from empty")
	}
	mark, state := "⏸", "paused"
	if p.playing {
		mark, state = "▶", "playing"
	}
	// The counter counts the events worth a frame; the adjustments riding
	// along silently are not part of the story being told.
	line := lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Render(mark+" ") +
		t.Value.Render(fmt.Sprintf("%d/%d", shown(events[:min(p.playhead, len(events))]), shown(events))) +
		t.Dim.Render(fmt.Sprintf(" · %s · %.1f events/s · p %s · ←/→ step · +/- speed",
			state, 1000/float64(playSpeeds[p.speed].Milliseconds()),
			map[bool]string{true: "pause", false: "play"}[p.playing]))
	// Narrate the newest event that took the stage, never a silent one.
	for at := min(p.playhead, len(events)); at > 0; at-- {
		if skippable(events[at-1]) {
			continue
		}
		e := events[at-1]
		line += "\n" + t.Dim.Render("→ ") +
			t.entryLine(e, a.data.ProductName(e.Product), width-2, false, false, false)
		break
	}
	return line
}

// shown counts the events of a run that a replay gives a frame to.
func shown(events []journal.Event) int {
	n := 0
	for _, e := range events {
		if !skippable(e) {
			n++
		}
	}
	return n
}

func (v analysisView) View(a *App, height int) string {
	t := a.theme
	width := a.inner()
	events := v.events(a)

	if len(events) == 0 {
		return lipgloss.NewStyle().Padding(1, 1).Render(t.Subtitle.Render("Nothing to analyse yet."))
	}

	played := v.played(events)

	sections := []string{
		v.summary(a, played, width),
		v.transport(a, events, width),
		"",
		t.Rule("Grams per day", width),
		v.perDay(a, played, width),
		"",
	}
	sections = append(sections, v.byProductSections(a, played, width)...)
	if v.scope != 0 {
		sections = append(sections, "", t.Rule("Rhythm", width), v.rhythm(a, played, width))
	}
	if v.scope >= 3 {
		sections = append(sections, "", t.Rule("Cycles", width), v.cycles(a, played, width))
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

	label := scopes[v.scope]
	if v.scope == 0 {
		if c := a.data.Cycle(); c != nil {
			label = fmt.Sprintf("%s (started: %s)", label, c.Start.Format("2006-01-02"))
		}
	}
	scope := t.Subtitle.Render("Showing ") + t.PanelTitle.Render(label) +
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
	)
	return lipgloss.JoinVertical(lipgloss.Left, chart,
		strings.Repeat(" ", axisGutter)+dateAxis(t, first, last, max(width-axisGutter, 12)),
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

// byProduct ranks the products by how much of them was ground, each under its
// full name — the name is what the jar is called, and the bars can shrink —
// with the share, the active days and the rate alongside.
// byProductSections lays the breakdown out. In the cycle scope the fill gets
// its own graph, and every older cycle whose jars were ground during the
// window gets one of its own beneath it, newest first — the window is honest
// about all of them, and nobody files under another fill's heading.
func (v analysisView) byProductSections(a *App, events []journal.Event, width int) []string {
	t := a.theme
	current, olderBy, seqs := v.byProductBars(a, events)
	if len(seqs) == 0 {
		return []string{t.Rule("By product", width), BarChart(current, width, t)}
	}
	sections := []string{t.Rule("By product · this cycle", width), BarChart(current, width, t)}
	for _, seq := range seqs {
		title := "By product · older cycles"
		if seq >= 0 {
			title = fmt.Sprintf("By product · cycle %d", seq+1)
		}
		sections = append(sections, "", t.Rule(title, width), BarChart(olderBy[seq], width, t))
	}
	return sections
}

// grindTotals sums the grams and active days ground per product, and ranks
// the products heaviest first.
func grindTotals(events []journal.Event) (totals map[string]float64, days map[string]map[string]bool, ground float64, slugs []string) {
	totals, days = map[string]float64{}, map[string]map[string]bool{}
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
	for s := range totals {
		slugs = append(slugs, s)
	}
	sort.Slice(slugs, func(i, j int) bool { return totals[slugs[i]] > totals[slugs[j]] })
	return totals, days, ground, slugs
}

// byProductBars ranks the products by how much of them was ground, each bar
// carrying the share, the active days and the rate. In the cycle scope the
// jars of older cycles group under the cycle that filled them, returned as
// bars per cycle with the cycles newest first.
func (v analysisView) byProductBars(a *App, events []journal.Event) (current []Bar, olderBy map[int][]Bar, seqs []int) {
	t, data := a.theme, a.data
	totals, days, ground, slugs := grindTotals(events)

	own := map[string]bool{}
	if v.scope == 0 {
		if c := a.data.Cycle(); c != nil {
			for _, slug := range c.Products {
				own[slug] = true
			}
		}
	}
	olderBy = map[int][]Bar{}
	for _, s := range slugs {
		active := len(days[s])
		bar := Bar{
			Label: data.ProductName(s),
			Value: totals[s],
			Note: fmt.Sprintf("%.1f g · %2.0f%% · %s · %.2f g/day",
				totals[s], totals[s]/ground*100, plural(active, "day"),
				totals[s]/float64(max(active, 1))),
			Color: t.StashC,
		}
		if len(own) > 0 && !own[s] {
			bar.Color = t.AVBC
			olderBy[fillSeqOf(a, s)] = append(olderBy[fillSeqOf(a, s)], bar)
			continue
		}
		current = append(current, bar)
	}
	for seq := range olderBy {
		seqs = append(seqs, seq)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(seqs)))
	return current, olderBy, seqs
}

// fillSeqOf returns the Seq of the cycle whose jar a grind drew on: the last
// one to fill the product before the current cycle. A jar no fill ever
// stocked — an imported grind with no purchase — returns -1 and stays a
// nameless older cycle.
func fillSeqOf(a *App, slug string) int {
	cycles := a.data.State.Cycles
	for i := len(cycles) - 2; i >= 0; i-- {
		for _, p := range cycles[i].Products {
			if p == slug {
				return i
			}
		}
	}
	return -1
}

// cycles compares whole cycles: how much was dispensed, how long it lasted, and
// the rate it went at. Under playback the cycles are folded from what has been
// applied so far, so they appear the way they appeared.
func (v analysisView) cycles(a *App, events []journal.Event, width int) string {
	t := a.theme
	all := a.data.State.Cycles
	if v.playhead >= 0 {
		all = ledger.Fold(events).Cycles
	}
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

// The Séance is the tab where the ledger is summoned back: the same replay
// transport the other screens use, but staged. Each event takes the table as
// a playing card with a figurine on its face and the full record on its back,
// while the shelf and the stashes fill and drain below it. The frame chooses
// which stretch of the past is called up — the whole ledger, one prescription
// cycle, or a window picked by hand.
type seanceView struct {
	player
	frame    int       // 0 the whole ledger, 1..N one cycle each, -1 a window picked by hand
	from, to time.Time // the hand-picked window, half-open, when frame is -1
	flipped  bool      // the card on the table shows its back
	scroll   int
}

func newSeanceView() seanceView { return seanceView{player: newPlayer()} }

// The séance's own keys: turning the frame, flipping the card, and picking
// the window by hand.
var (
	frameKey = key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "frame"))
	flipKey  = key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "flip card"))
	datesKey = key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "dates"))
)

type seanceKeys struct {
	keyMap
	Play, Step, Flip, Frame, Dates, Speed key.Binding
}

func (k seanceKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Play, k.Step, k.Flip, k.Frame, k.Dates, k.Speed, k.Quit}
}

func (k seanceKeys) FullHelp() [][]key.Binding {
	return append(k.keyMap.FullHelp(), []key.Binding{k.Play, k.Step, k.Speed, k.Frame, k.Flip, k.Dates})
}

func (v seanceView) keys(base keyMap) help.KeyMap {
	return seanceKeys{keyMap: base, Play: playKey, Step: withHelp(slideKey, "step"),
		Flip: flipKey, Frame: frameKey, Dates: datesKey, Speed: speedKeys}
}

func (v seanceView) Update(msg tea.Msg, a *App) (seanceView, tea.Cmd) {
	if _, ok := msg.(playTickMsg); ok {
		var cmd tea.Cmd
		v.player, cmd = v.player.advance(v.events(a))
		return v, cmd
	}
	if press, ok := msg.(tea.KeyPressMsg); ok {
		return v.press(press, a)
	}
	return v, nil
}

// press handles the séance's keys. Turning the frame or picking a new window
// resets the sitting: the tape rewinds and the card turns face up again.
func (v seanceView) press(msg tea.KeyPressMsg, a *App) (seanceView, tea.Cmd) {
	var cmd tea.Cmd
	switch {
	case key.Matches(msg, frameKey):
		v.frame = (v.frame + 1) % (len(a.data.State.Cycles) + 1)
		v.player, v.flipped, v.scroll = newPlayer(), false, 0
	case key.Matches(msg, datesKey):
		_, open := a.open(newSeanceDatesForm(a))
		return v, open
	case key.Matches(msg, flipKey):
		v.flipped = !v.flipped
	case key.Matches(msg, playKey):
		v.player, cmd = v.player.toggle(v.events(a))
	case key.Matches(msg, slideKey):
		v.player = v.player.step(msg, v.events(a))
	case key.Matches(msg, speedKeys):
		v.player = v.player.retune(msg)
	case key.Matches(msg, a.keys.Up):
		v.scroll = max(v.scroll-1, 0)
	case key.Matches(msg, a.keys.Down):
		v.scroll++
	case key.Matches(msg, a.keys.Top):
		v.scroll = 0
	}
	return v, cmd
}

// reframe points the séance at a window picked by hand and starts the sitting
// over.
func (v seanceView) reframe(from, to time.Time) seanceView {
	v.frame, v.from, v.to = -1, from, to
	v.player, v.flipped, v.scroll = newPlayer(), false, 0
	return v
}

// cycleAt returns the cycle the frame points at, or nil when it points at the
// whole ledger or past the end after a reload shrank the record.
func (v seanceView) cycleAt(a *App) *ledger.Cycle {
	cycles := a.data.State.Cycles
	if v.frame >= 1 && v.frame <= len(cycles) {
		return &cycles[v.frame-1]
	}
	return nil
}

// events returns the stretch of the ledger the frame has called up.
func (v seanceView) events(a *App) []journal.Event {
	if v.frame < 0 {
		var out []journal.Event
		for _, e := range a.data.State.Events {
			if !e.OccurredAt.Before(v.from) && e.OccurredAt.Before(v.to) {
				out = append(out, e)
			}
		}
		return out
	}
	if c := v.cycleAt(a); c != nil {
		return c.Events
	}
	return a.data.State.Events
}

// frameName says which stretch of the past is on the table.
func (v seanceView) frameName(a *App) string {
	if v.frame < 0 {
		return fmt.Sprintf("%s → %s", v.from.Format("02 Jan 2006"),
			v.to.AddDate(0, 0, -1).Format("02 Jan 2006"))
	}
	if c := v.cycleAt(a); c != nil {
		return fmt.Sprintf("cycle %d of %d · %s", v.frame,
			len(a.data.State.Cycles), c.Start.Format("Jan 2006"))
	}
	return "the whole ledger"
}

func (v seanceView) View(a *App, height int) string {
	t := a.theme
	width := a.inner()
	events := v.events(a)
	if len(events) == 0 {
		return lipgloss.NewStyle().Padding(1, 1).Render(
			t.Subtitle.Render("The séance finds nothing in this frame."))
	}
	played := v.played(events)

	sections := []string{
		v.banner(a, events, width),
		v.transport(a, events, width),
		"",
		v.table(a, events, width),
		"",
		t.Rule("The shelf", width),
		v.shelf(a, played, width),
		"",
		t.Rule("The stashes", width),
		v.stashes(a, played, width),
	}
	// The stage overruns a small terminal, so the view scrolls the way the
	// analysis does rather than cutting the stashes off silently.
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

// banner names the frame and counts the apparitions it holds.
func (v seanceView) banner(a *App, events []journal.Event, _ int) string {
	t := a.theme
	return t.Subtitle.Render("Summoning ") + t.PanelTitle.Render(v.frameName(a)) +
		t.Dim.Render(fmt.Sprintf("  ·  %d apparitions  ·  f frame · d dates · x flip", shown(events)))
}

// table lays the cards out: the newest apparition face up — or face down to
// its record when flipped — with its neighbours as ghosts either side.
func (v seanceView) table(a *App, events []journal.Event, width int) string {
	t := a.theme
	cur := lastShown(v.played(events))
	if cur < 0 {
		return lipgloss.PlaceHorizontal(width, lipgloss.Center, t.CardSleeping())
	}
	e := events[cur]
	card := t.CardFront(e, a.data.ProductName(e.Product))
	if v.flipped {
		card = t.CardBack(e, a.data.ProductName(e.Product))
	}
	var pieces []string
	if prev := lastShown(events[:cur]); prev >= 0 {
		pieces = append(pieces, t.CardGhost(events[prev]), "  ")
	}
	pieces = append(pieces, card)
	if next := nextShown(events, cur); next >= 0 {
		pieces = append(pieces, "  ", t.CardGhost(events[next]))
	}
	row := lipgloss.JoinHorizontal(lipgloss.Center, pieces...)
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, row)
}

// lastShown finds the newest event of a run that a replay would give a frame
// to, or -1 when the run holds none.
func lastShown(events []journal.Event) int {
	for i := len(events) - 1; i >= 0; i-- {
		if !skippable(events[i]) {
			return i
		}
	}
	return -1
}

// nextShown finds the next event after the given one that would take a frame.
func nextShown(events []journal.Event, after int) int {
	for i := after + 1; i < len(events); i++ {
		if !skippable(events[i]) {
			return i
		}
	}
	return -1
}

// seanceTotals sums what each product has had bought and ground within the
// played prefix, which is what the jars measure their fill against.
func seanceTotals(events []journal.Event) (bought, ground map[string]float64) {
	bought, ground = map[string]float64{}, map[string]float64{}
	for _, e := range events {
		switch e.Type {
		case journal.Purchase:
			bought[e.Product] += e.Grams
		case journal.Grind:
			ground[e.Product] += e.Grams
		}
	}
	return bought, ground
}

// appearance lists the products in the order the played prefix first moved
// grams into them by the given event type, so jars appear on the shelf the
// moment the replay buys them.
func appearance(events []journal.Event, typ journal.Type) []string {
	var order []string
	seen := map[string]bool{}
	for _, e := range events {
		if e.Type == typ && e.Product != "" && !seen[e.Product] {
			seen[e.Product] = true
			order = append(order, e.Product)
		}
	}
	return order
}

// shelf draws the storage jars: one per product the replay has bought so far,
// filled to what is left in storage against everything bought into it.
func (v seanceView) shelf(a *App, played []journal.Event, width int) string {
	t := a.theme
	bought, _ := seanceTotals(played)
	return jarRow(a, played, appearance(played, journal.Purchase), bought,
		journal.Storage, t.StorageC, width, "nothing on the shelf yet")
}

// stashes draws the stash tins the same way, measured against everything
// ground into them.
func (v seanceView) stashes(a *App, played []journal.Event, width int) string {
	t := a.theme
	_, ground := seanceTotals(played)
	return jarRow(a, played, appearance(played, journal.Grind), ground,
		journal.Stash, t.StashC, width, "nothing ground yet")
}

// jarRow renders a row of labelled jars, each filled to what its account holds
// against what has passed through it. When the shelf outgrows the terminal the
// newest jars keep their place, since they are the ones the replay just moved.
func jarRow(a *App, played []journal.Event, order []string, through map[string]float64,
	account journal.Account, fill tint, width int, empty string) string {
	t := a.theme
	if len(order) == 0 {
		return t.Dim.Render(empty)
	}
	balances := ledger.Fold(played).Balances
	if room := max(width/15, 1); len(order) > room {
		order = order[len(order)-room:]
	}
	var cols []string
	for _, slug := range order {
		have := heldIn(balances[slug], account)
		// A reconciliation can leave more in the jar than ever passed through
		// it on the books, so the fuller of the two sets the scale.
		denom := through[slug]
		if have > denom {
			denom = have
		}
		fraction := 0.0
		if denom > 0 {
			fraction = have / denom
		}
		// The label wraps rather than truncates: a product keeps its whole
		// name in this house, even under a jar three fingers wide.
		const jw = 13
		col := append(Jar(jw, 4, fraction, fill, t),
			center(t.Dim.Render(fmt.Sprintf("%.1f g", have)), jw),
			lipgloss.NewStyle().Width(jw).Align(lipgloss.Center).
				Render(t.Value.Render(a.data.ProductName(slug))))
		cols = append(cols, strings.Join(col, "\n"), "  ")
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cols...)
}

// heldIn reads one account out of a balance that may not exist yet.
func heldIn(b *ledger.Balance, account journal.Account) float64 {
	switch {
	case b == nil:
		return 0
	case account == journal.Storage:
		return b.Storage
	default:
		return b.Stash
	}
}
