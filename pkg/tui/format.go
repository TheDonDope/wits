package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/TheDonDope/wits/pkg/journal"
)

// glyphs give each event type a shape, so the kind of an entry is legible
// before any of its text is read.
var glyphs = map[journal.Type]string{
	journal.Purchase:   "▲",
	journal.Grind:      "◆",
	journal.Sesh:       "●",
	journal.AVBCollect: "◇",
	journal.AVBUse:     "▽",
	journal.Adjust:     "±",
}

// verbs are how each event type reads in a sentence.
var verbs = map[journal.Type]string{
	journal.Purchase:   "picked up",
	journal.Grind:      "ground",
	journal.Sesh:       "sesh",
	journal.AVBCollect: "collected",
	journal.AVBUse:     "used AVB",
	journal.Adjust:     "adjusted",
}

// eventColor gives an event the colour of the account it moves grams into, so
// the palette means the same thing in the log as it does in the charts.
func (t *Theme) eventColor(typ journal.Type) tint {
	switch typ {
	case journal.Purchase:
		return t.StorageC
	case journal.Grind:
		return t.StashC
	case journal.Sesh:
		return t.SeshC
	case journal.AVBCollect, journal.AVBUse:
		return t.AVBC
	default:
		return t.Muted
	}
}

// Grams renders an amount with its unit set apart, so a column of them lines up
// on the decimal point and the units do not shout.
func (t *Theme) Grams(g float64) string {
	return t.Value.Render(fmt.Sprintf("%6.2f", g)) + t.Unit.Render("g")
}

// Day renders a date as a heading: "Thu 09 Jul 2026", with today and yesterday
// named rather than dated.
func (t *Theme) Day(d time.Time, now time.Time) string {
	switch days := daysBetween(d, now); days {
	case 0:
		return t.PanelTitle.Render("Today") + t.Dim.Render(" · "+d.Format("Mon 02 Jan"))
	case 1:
		return t.PanelTitle.Render("Yesterday") + t.Dim.Render(" · "+d.Format("Mon 02 Jan"))
	default:
		return t.PanelTitle.Render(d.Format("Mon 02 Jan 2006"))
	}
}

// EventLine renders one journal entry as a single row.
//
// The columns are fixed so that a run of entries reads as a table without being
// drawn as one: time, glyph, verb, amount, product, and whatever detail the
// entry carries. Everything optional is dimmed, so the eye falls on the amount.
func (t *Theme) EventLine(e journal.Event, name string, width int, selected bool) string {
	return t.entryLine(e, name, width, selected, false, false)
}

// entryLine renders one journal entry, marking it if it has been corrected or
// is itself a correction.
func (t *Theme) entryLine(e journal.Event, name string, width int, selected, corrected, isFix bool) string {
	colour := t.eventColor(e.Type)

	marker := " "
	if selected {
		marker = lipgloss.NewStyle().Foreground(colour).Render("│")
	}

	// Backfilled entries have no time of day, only a date. Printing 00:00 for
	// them would look like a precise fact rather than a missing one.
	clock := "     "
	if h, m, _ := e.OccurredAt.Clock(); h != 0 || m != 0 {
		clock = e.OccurredAt.Format("15:04")
	}

	glyph := glyphs[e.Type]
	verb := verbs[e.Type]
	value := t.Value
	if isFix {
		glyph, verb, colour = "↩", "undo", t.Muted
	}
	if corrected {
		// A superseded entry is shown struck through rather than removed, so the
		// correction reads as something that happened to it.
		value = t.Dim.Strikethrough(true)
	}

	parts := []string{
		marker,
		t.Dim.Render(clock),
		" ",
		lipgloss.NewStyle().Foreground(colour).Render(glyph),
		" ",
		t.Label.Width(10).Render(verb),
		value.Render(fmt.Sprintf("%6.2f", e.Grams)) + t.Unit.Render("g"),
		"  ",
		value.Render(name),
	}
	line := lipgloss.JoinHorizontal(lipgloss.Left, parts...)

	if detail := t.eventDetail(e); detail != "" {
		room := width - lipgloss.Width(line) - 1
		if room > 4 {
			line += " " + truncate(detail, room)
		}
	}
	if selected {
		return lipgloss.NewStyle().Background(t.selectionBg()).Render(pad(line, width))
	}
	return line
}

// eventDetail is the trailing, dimmed part of a log line: the device and
// temperature of a session, a note, or nothing at all.
func (t *Theme) eventDetail(e journal.Event) string {
	var bits []string
	if e.Device != "" {
		bits = append(bits, e.Device)
	}
	if e.Temperature > 0 {
		bits = append(bits, fmt.Sprintf("%d°C", e.Temperature))
	}
	if e.Note != "" {
		bits = append(bits, e.Note)
	}
	if len(bits) == 0 {
		return ""
	}
	return t.Dim.Render("· " + strings.Join(bits, " · "))
}

// selectionBg is the background for the highlighted row.
func (t *Theme) selectionBg() tint {
	if t.Dark {
		return lipgloss.Color("#22262E")
	}
	return lipgloss.Color("#EEF1F6")
}

// Metric renders a label above a large value, for the figures that should be
// readable from across the room.
func (t *Theme) Metric(label, value, note string) string {
	rows := []string{
		t.Label.Render(strings.ToUpper(label)),
		t.Big.Render(value),
	}
	if note != "" {
		rows = append(rows, t.Dim.Render(note))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// Rule draws a horizontal line, optionally with a title set into it.
func (t *Theme) Rule(title string, width int) string {
	if title == "" {
		return t.Dim.Render(strings.Repeat("─", maxInt(width, 0)))
	}
	head := t.PanelTitle.Render(title)
	rest := width - lipgloss.Width(head) - 2
	if rest < 0 {
		rest = 0
	}
	return head + " " + t.Dim.Render(strings.Repeat("─", rest))
}

// daysBetween counts whole days between two instants, by calendar day.
func daysBetween(a, b time.Time) int {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	from := time.Date(ay, am, ad, 0, 0, 0, 0, time.UTC)
	to := time.Date(by, bm, bd, 0, 0, 0, 0, time.UTC)
	return int(to.Sub(from).Hours() / 24)
}

// truncate shortens a rendered string to width, adding an ellipsis. It measures
// display width, so styled text and wide runes are not cut mid-cell.
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes))+1 > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

// pad extends a rendered string to width with spaces.
func pad(s string, width int) string {
	if gap := width - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}
