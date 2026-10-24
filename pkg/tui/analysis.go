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

	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/record"
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
		all = ledger.FoldAt(events, a.data.Now).Cycles
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
			Value: float64(c.Ground),
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
	cursor int // which of the recent sessions is selected, newest first
}

func newSessionsView() sessionsView { return sessionsView{} }

// recentShown is how many of the newest sessions are listed to be rated.
const recentShown = 8

type sessionKeys struct {
	keyMap
	Rate key.Binding
}

func (k sessionKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Rate, k.Help, k.Quit}
}

func (k sessionKeys) FullHelp() [][]key.Binding {
	return append(k.keyMap.FullHelp(), []key.Binding{k.Rate})
}

func (v sessionsView) keys(base keyMap) help.KeyMap {
	return sessionKeys{keyMap: base, Rate: withHelp(base.Edit, "rate")}
}

// Selected is the recent session under the cursor, or nil.
func (v sessionsView) Selected(a *App) *journal.Event {
	recent := recentSessions(a)
	if len(recent) == 0 {
		return nil
	}
	e := recent[min(v.cursor, len(recent)-1)]
	return &e
}

func (v sessionsView) Update(msg tea.Msg, a *App) (sessionsView, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(msg, a.keys.Up):
			v.cursor = max(min(v.cursor, len(recentSessions(a))-1)-1, 0)
		case key.Matches(msg, a.keys.Down):
			v.cursor = min(v.cursor+1, max(len(recentSessions(a))-1, 0))
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

// recentSessions are the newest sessions that stand — an undone one has
// nothing to rate — newest first, as many as are listed.
func recentSessions(a *App) []journal.Event {
	undone := record.Reverted(a.data.State.Events)
	var out []journal.Event
	events := a.data.State.Applied()
	for i := len(events) - 1; i >= 0 && len(out) < recentShown; i-- {
		if e := events[i]; e.Type == journal.Sesh && !undone[e.Hash] {
			out = append(out, e)
		}
	}
	return out
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
		t.Rule("Recent sessions", width),
		v.recent(a, width),
	}
	if works := v.whatWorks(a, width); works != "" {
		sections = append(sections, "", t.Rule("What works · overall, by product", width), works)
	}
	sections = append(sections,
		"",
		t.Rule("Seshed per day", width),
		v.perDay(a, events, width),
		"",
		t.Rule("By device", width),
		v.byDevice(a, events, width),
		"",
		t.Rule("Rhythm", width),
		v.rhythm(a, events, width),
	)

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

// recent lists the newest sessions with what each was rated, the selected one
// marked: e rates it, or rates it again.
func (v sessionsView) recent(a *App, width int) string {
	t := a.theme
	latest := effects.Latest(a.data.Ratings)
	recent := recentSessions(a)
	cursor := min(v.cursor, len(recent)-1)
	rows := make([]string, 0, len(recent))
	for i, e := range recent {
		marker, name := "  ", t.Value.Render(truncate(a.data.ProductName(e.Product), 30))
		if i == cursor {
			marker = lipgloss.NewStyle().Foreground(t.Accent).Render("│ ")
			name = lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Render(truncate(a.data.ProductName(e.Product), 30))
		}
		temp := ""
		if e.Temperature > 0 {
			temp = fmt.Sprintf("%d°C", e.Temperature)
		}
		rating := t.Dim.Render("not rated")
		if r, ok := latest[e.Hash]; ok {
			rating = lipgloss.NewStyle().Foreground(t.SeshC).Render(effects.Summary(r.Scores))
		}
		rows = append(rows, truncate(lipgloss.JoinHorizontal(lipgloss.Left,
			marker,
			t.Dim.Width(18).Render(e.OccurredAt.Format("Mon 02 Jan 15:04")),
			t.Value.Width(9).Render(fmt.Sprintf("%.2f g", e.Grams)),
			lipgloss.NewStyle().Width(32).Render(name),
			t.Label.Width(7).Render(temp),
			rating,
		), width))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// whatWorks is each product's mean overall rating, with how many ratings it
// stands on — one rating is a rating, not a finding, so the count is shown.
func (v sessionsView) whatWorks(a *App, width int) string {
	insights := effects.Analyse(a.data.Ratings, a.data.State.Applied(), a.data.ProductName)
	var bars []Bar
	for _, g := range insights.ByProduct {
		mean, ok := g.Means[effects.Overall]
		if !ok {
			continue
		}
		bars = append(bars, Bar{
			Label: g.Label,
			Value: mean,
			Note:  fmt.Sprintf("%.1f of 5 · %s", mean, plural(g.Counts[effects.Overall], "rating")),
			Color: a.theme.SeshC,
		})
	}
	if len(bars) == 0 {
		return ""
	}
	sort.SliceStable(bars, func(i, j int) bool { return bars[i].Value > bars[j].Value })
	return BarChart(bars, width, a.theme)
}
