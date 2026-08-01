package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/TheDonDope/wits-tui/pkg/journal"
)

// journalView is the log: every entry, newest first, grouped under the day it
// happened on.
type journalView struct {
	cursor int
	offset int
	filter journal.Type // empty shows everything
}

func newJournalView() journalView { return journalView{} }

// journalKeys are the bindings this screen adds.
type journalKeys struct {
	keyMap
	Filter key.Binding
}

func (k journalKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Next, k.Up, k.Down, k.Filter, k.New, k.Help, k.Quit}
}

func (k journalKeys) FullHelp() [][]key.Binding {
	return append(k.keyMap.FullHelp(), []key.Binding{k.Filter})
}

func (v journalView) keys(base keyMap) help.KeyMap {
	return journalKeys{
		keyMap: base,
		Filter: key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter")),
	}
}

// filters is the cycle the f key steps through.
var filters = []journal.Type{"", journal.Purchase, journal.Grind, journal.Sesh}

func (v journalView) Update(msg tea.Msg, a *App) (journalView, tea.Cmd) {
	rows := v.rows(a)
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, a.keys.Up):
			v.cursor = maxInt(v.cursor-1, 0)
		case key.Matches(msg, a.keys.Down):
			v.cursor = minInt(v.cursor+1, maxInt(len(rows)-1, 0))
		case key.Matches(msg, a.keys.PageUp):
			v.cursor = maxInt(v.cursor-10, 0)
		case key.Matches(msg, a.keys.PgDown):
			v.cursor = minInt(v.cursor+10, maxInt(len(rows)-1, 0))
		case key.Matches(msg, a.keys.Top):
			v.cursor = 0
		case key.Matches(msg, a.keys.Bottom):
			v.cursor = maxInt(len(rows)-1, 0)
		case msg.String() == "f":
			for i, f := range filters {
				if f == v.filter {
					v.filter = filters[(i+1)%len(filters)]
					break
				}
			}
			v.cursor, v.offset = 0, 0
		}
	}
	return v, nil
}

// row is a rendered line: either a day heading or an event.
type row struct {
	day     time.Time
	event   *journal.Event
	heading bool
}

// rows builds the display list, newest first, inserting a heading whenever the
// day changes. Headings are part of the list so that scrolling never leaves a
// run of entries without the date they belong to.
func (v journalView) rows(a *App) []row {
	events := a.data.State.Events
	var out []row
	var last string
	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		if v.filter != "" && e.Type != v.filter {
			continue
		}
		day := e.OccurredAt.Format(time.DateOnly)
		if day != last {
			out = append(out, row{day: e.OccurredAt, heading: true})
			last = day
		}
		ev := e
		out = append(out, row{event: &ev})
	}
	return out
}

func (v journalView) View(a *App, height int) string {
	t := a.theme
	width := a.inner()
	rows := v.rows(a)

	if len(rows) == 0 {
		return lipgloss.NewStyle().Padding(1, 1).Render(t.Subtitle.Render("Nothing logged yet."))
	}

	body := height - 3
	if body < 1 {
		body = 1
	}
	// Keep the cursor in view, scrolling only as far as it has to.
	if v.cursor < v.offset {
		v.offset = v.cursor
	}
	if v.cursor >= v.offset+body {
		v.offset = v.cursor - body + 1
	}
	end := minInt(v.offset+body, len(rows))

	var lines []string
	for i := v.offset; i < end; i++ {
		r := rows[i]
		switch {
		case r.heading:
			lines = append(lines, "")
			lines = append(lines, t.Day(r.day, a.data.Now))
		default:
			lines = append(lines, t.EventLine(*r.event, a.data.ProductName(r.event.Product), width, i == v.cursor))
		}
	}

	return lipgloss.NewStyle().Padding(0, 1).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			v.status(a, countEvents(rows), width),
			lipgloss.JoinVertical(lipgloss.Left, lines...),
		))
}

// status is the one-line summary above the log: what is being shown, and how
// far down it you are.
func (v journalView) status(a *App, total, width int) string {
	t := a.theme
	what := "all entries"
	if v.filter != "" {
		what = string(v.filter) + " only"
	}
	left := t.Dim.Render(fmt.Sprintf("%s · %d", what, total))
	right := t.Dim.Render(fmt.Sprintf("%d/%d", minInt(v.cursor+1, total), total))
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
