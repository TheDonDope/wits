package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/record"
)

// journalView is the log: every entry, newest first, grouped under the day it
// happened on.
type journalView struct {
	view    viewport.Model
	cursor  int
	filter  journal.Type // empty shows everything
	showAll bool         // include corrected entries and their corrections
	rows    []row
}

func newJournalView() journalView {
	return journalView{view: viewport.New()}
}

// journalKeys are the bindings this screen adds.
type journalKeys struct {
	keyMap
	Slide, Filter, Reveal, Edit, Delete key.Binding
}

func (k journalKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Slide, k.Up, k.Down, k.Edit, k.Delete, k.Filter, k.Help, k.Quit}
}

func (k journalKeys) FullHelp() [][]key.Binding {
	return append(k.keyMap.FullHelp(), []key.Binding{k.Slide, k.Filter, k.Reveal, k.Edit, k.Delete})
}

func (v journalView) keys(base keyMap) help.KeyMap {
	return journalKeys{
		keyMap: base,
		Slide:  slideKey,
		Filter: filterKey,
		Reveal: revealKey,
		Edit:   withHelp(base.Edit, "amend"),
		Delete: withHelp(base.Delete, "undo"),
	}
}

// filters is the cycle the f key steps through.
var filters = []journal.Type{"", journal.Purchase, journal.Grind, journal.Sesh}

// filterKey and revealKey are declared once, shared by the help line and the
// dispatch, so neither can drift from the other.
var (
	filterKey = key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter"))
	revealKey = key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "corrections"))
)

// row is a rendered line: either a day heading or an entry.
type row struct {
	day       time.Time
	event     *journal.Event
	heading   bool
	corrected bool // superseded by a later correction
	isFix     bool // is itself a correction
}

func (v journalView) Update(msg tea.Msg, a *App) (journalView, tea.Cmd) {
	v.rows = v.build(a)

	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(msg, a.keys.Up):
			v.cursor = v.step(-1)
		case key.Matches(msg, a.keys.Down):
			v.cursor = v.step(+1)
		case key.Matches(msg, slideKey):
			v.cursor = v.slide(msg)
		case key.Matches(msg, a.keys.PageUp):
			for i := 0; i < 10; i++ {
				v.cursor = v.step(-1)
			}
		case key.Matches(msg, a.keys.PgDown):
			for i := 0; i < 10; i++ {
				v.cursor = v.step(+1)
			}
		case key.Matches(msg, a.keys.Top):
			v.cursor = v.firstEntry(0, +1)
		case key.Matches(msg, a.keys.Bottom):
			v.cursor = v.firstEntry(len(v.rows)-1, -1)
		case key.Matches(msg, filterKey):
			for i, f := range filters {
				if f == v.filter {
					v.filter = filters[(i+1)%len(filters)]
					break
				}
			}
			v.rows = v.build(a)
			v.cursor = v.firstEntry(0, +1)
		case key.Matches(msg, revealKey):
			v.showAll = !v.showAll
			v.rows = v.build(a)
			v.cursor = v.firstEntry(0, +1)
		}
	}
	return v, nil
}

// Selected returns the entry under the cursor, if there is one.
func (v journalView) Selected() *journal.Event {
	if v.cursor < 0 || v.cursor >= len(v.rows) {
		return nil
	}
	return v.rows[v.cursor].event
}

// step moves the cursor by one entry, skipping day headings, which are labels
// rather than things that can be acted on.
func (v journalView) step(delta int) int {
	i := v.cursor + delta
	for i >= 0 && i < len(v.rows) && v.rows[i].heading {
		i += delta
	}
	if i < 0 || i >= len(v.rows) {
		return v.cursor
	}
	return i
}

// firstEntry returns the first non-heading row from i, searching in direction.
func (v journalView) firstEntry(i, direction int) int {
	for i >= 0 && i < len(v.rows) && v.rows[i].heading {
		i += direction
	}
	if i < 0 || i >= len(v.rows) {
		return 0
	}
	return i
}

// build assembles the display list, newest first, inserting a heading whenever
// the day changes.
//
// A corrected entry and the correction that undid it are both left out unless
// asked for, so the log reads as what currently stands. They are hidden rather
// than removed: the journal keeps both, because that is the difference between
// a record that can be audited and one that cannot.
func (v journalView) build(a *App) []row {
	events := a.data.State.Events
	corrected := record.Reverted(events)

	var out []row
	var last string
	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		isFix := e.Reverts != ""
		if !v.showAll && corrected[e.Hash] {
			continue
		}
		if v.filter != "" && e.Type != v.filter {
			continue
		}
		day := e.OccurredAt.Format(time.DateOnly)
		if day != last {
			out = append(out, row{day: e.OccurredAt, heading: true})
			last = day
		}
		ev := e
		out = append(out, row{event: &ev, corrected: corrected[e.Hash] && !isFix, isFix: isFix})
	}
	return out
}

func (v journalView) View(a *App, height int) string {
	t := a.theme
	width := a.inner()
	v.rows = v.build(a)

	if len(v.rows) == 0 {
		return lipgloss.NewStyle().Padding(1, 1).Render(
			t.Subtitle.Render("Nothing logged yet.  Press n to record a grind."))
	}
	if v.cursor >= len(v.rows) || v.rows[min(v.cursor, len(v.rows)-1)].heading {
		v.cursor = v.firstEntry(min(v.cursor, len(v.rows)-1), +1)
	}

	var lines []string
	for i, r := range v.rows {
		switch {
		case r.heading:
			if i > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, t.Day(r.day, a.data.Now))
		default:
			lines = append(lines, t.entryLine(*r.event, a.data.ProductName(r.event.Product),
				width, i == v.cursor, r.corrected, r.isFix))
		}
	}

	cover := v.cover(a, width)
	body := max(height-3-lipgloss.Height(cover), 1)
	v.view.SetWidth(width)
	v.view.SetHeight(body)
	v.view.SetContent(strings.Join(lines, "\n"))
	v.view.SetYOffset(scrollTo(v.cursor, v.view.YOffset(), body, len(lines)))

	return lipgloss.NewStyle().Padding(0, 1).Render(
		lipgloss.JoinVertical(lipgloss.Left, cover, "", v.status(a, width), v.view.View()))
}

// scrollTo returns the offset that keeps line in view, moving as little as it
// can, so the list does not jump under a cursor that only moved one row.
func scrollTo(line, offset, height, total int) int {
	if line < offset {
		offset = line
	}
	if line >= offset+height {
		offset = line - height + 1
	}
	return max(0, min(offset, max(total-height, 0)))
}

// status is the one-line summary above the log.
func (v journalView) status(a *App, width int) string {
	t := a.theme
	what := "all entries"
	if v.filter != "" {
		what = string(v.filter) + " only"
	}
	if v.showAll {
		what += " · corrections shown"
	}
	total := countEvents(v.rows)
	position := 0
	for i := 0; i <= v.cursor && i < len(v.rows); i++ {
		if !v.rows[i].heading {
			position++
		}
	}

	left := t.Dim.Render(fmt.Sprintf("%s · %d", what, total))
	right := t.Dim.Render(fmt.Sprintf("%d/%d", position, total))
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// countEvents counts the entries in a row list, ignoring the day headings.
func countEvents(rows []row) int {
	n := 0
	for _, r := range rows {
		if !r.heading {
			n++
		}
	}
	return n
}

// slideKey moves the cover slider: the arrows and their vi spellings. On this
// screen they browse entries; tab and shift+tab still change screens.
var slideKey = key.NewBinding(key.WithKeys("left", "right", "h", "l"), key.WithHelp("←/→", "browse"))

// slide moves the cursor the way the slider reads: left toward older entries,
// right toward newer ones. The list is newest first, so the directions cross.
func (v journalView) slide(msg tea.KeyPressMsg) int {
	switch msg.String() {
	case "left", "h":
		return v.step(+1)
	default:
		return v.step(-1)
	}
}

// cover is the slider above the list: the selected entry as a card between
// its neighbours, older to the left, newer to the right.
func (v journalView) cover(a *App, width int) string {
	t := a.theme
	var entries []int
	at := -1
	for i, r := range v.rows {
		if r.heading {
			continue
		}
		if i == v.cursor {
			at = len(entries)
		}
		entries = append(entries, i)
	}
	if at < 0 || len(entries) == 0 {
		return ""
	}

	midW := min(46, max(width-32, 28))
	sideW := max((width-midW-2)/2, 14)

	older, newer := blankCard(sideW), blankCard(sideW)
	if at+1 < len(entries) {
		older = v.sideCard(a, v.rows[entries[at+1]].event, sideW)
	}
	if at > 0 {
		newer = v.sideCard(a, v.rows[entries[at-1]].event, sideW)
	}
	mid := v.midCard(a, v.rows[entries[at]].event, midW)

	strip := lipgloss.JoinHorizontal(lipgloss.Center, older, " ", mid, " ", newer)
	return lipgloss.JoinVertical(lipgloss.Left, strip,
		t.Dim.Render(fmt.Sprintf(" ← older · %d of %d · newer →", len(entries)-at, len(entries))))
}

// midCard is the selected entry, told in full.
func (v journalView) midCard(a *App, e *journal.Event, w int) string {
	t := a.theme
	colour := t.eventColor(e.Type)
	head := lipgloss.NewStyle().Foreground(colour).Render(glyphs[e.Type]+" ") +
		t.Value.Render(verbs[e.Type]) +
		t.Big.Render(fmt.Sprintf("  %.2f g", e.Grams))
	lines := []string{
		head,
		t.Value.Render(truncate(a.data.ProductName(e.Product), w-4)),
		t.Dim.Render(e.OccurredAt.Format("Mon 02 Jan 2006 · 15:04")),
	}
	if detail := t.eventDetail(*e); detail != "" {
		lines = append(lines, truncate(detail, w-4))
	}
	lines = append(lines, t.Dim.Render(shortHash(e.Hash)))
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).BorderForeground(t.Accent).
		Padding(0, 1).Width(w).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// sideCard is a neighbour, told small and dim.
func (v journalView) sideCard(a *App, e *journal.Event, w int) string {
	t := a.theme
	lines := []string{
		t.Dim.Render(fmt.Sprintf("%s %.2f g", verbs[e.Type], e.Grams)),
		t.Dim.Render(truncate(a.data.ProductName(e.Product), w-4)),
		t.Dim.Render(e.OccurredAt.Format("02 Jan")),
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).BorderForeground(t.Line).
		Padding(0, 1).Width(w).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// blankCard holds a neighbour's place at the edges of the journal, so the
// selected card does not wander as the slider reaches either end.
func blankCard(w int) string {
	return lipgloss.NewStyle().Width(w).Render("")
}

// shortHash abbreviates an event hash the way git abbreviates a commit.
func shortHash(h string) string {
	if len(h) > 7 {
		return h[:7]
	}
	return h
}
