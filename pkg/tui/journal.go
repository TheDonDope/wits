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
	Filter, Reveal, Edit, Delete key.Binding
}

func (k journalKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Edit, k.Delete, k.Filter, k.Help, k.Quit}
}

func (k journalKeys) FullHelp() [][]key.Binding {
	return append(k.keyMap.FullHelp(), []key.Binding{k.Filter, k.Reveal, k.Edit, k.Delete})
}

func (v journalView) keys(base keyMap) help.KeyMap {
	return journalKeys{
		keyMap: base,
		Filter: key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter")),
		Reveal: key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "corrections")),
		Edit:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "amend")),
		Delete: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "undo")),
	}
}

// filters is the cycle the f key steps through.
var filters = []journal.Type{"", journal.Purchase, journal.Grind, journal.Sesh}

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
		case msg.String() == "f":
			for i, f := range filters {
				if f == v.filter {
					v.filter = filters[(i+1)%len(filters)]
					break
				}
			}
			v.rows = v.build(a)
			v.cursor = v.firstEntry(0, +1)
		case msg.String() == "v":
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
	if v.cursor >= len(v.rows) || v.rows[minInt(v.cursor, len(v.rows)-1)].heading {
		v.cursor = v.firstEntry(minInt(v.cursor, len(v.rows)-1), +1)
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

	body := maxInt(height-2, 1)
	v.view.SetWidth(width)
	v.view.SetHeight(body)
	v.view.SetContent(strings.Join(lines, "\n"))
	v.view.SetYOffset(scrollTo(v.cursor, v.view.YOffset(), body, len(lines)))

	return lipgloss.NewStyle().Padding(0, 1).Render(
		lipgloss.JoinVertical(lipgloss.Left, v.status(a, width), v.view.View()))
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
	return maxInt(0, minInt(offset, maxInt(total-height, 0)))
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
