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
		return t.Dim.Render(strings.Repeat("─", max(width, 0)))
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

// The Séance renders events as playing cards, and a playing card wants a
// figure on it. These are the figurines: one emblem per event type, each
// standing on the same pedestal so the deck reads as a set. They are built
// from the event glyphs the rest of the interface already speaks.
var figurines = map[journal.Type][]string{
	journal.Purchase:   {"  ▲  ", " ▲ ▲ ", "▲ ▲ ▲", " ═╩═ "},
	journal.Grind:      {" ◆◆◆ ", "◆ ◇ ◆", " ◆◆◆ ", " ═╩═ "},
	journal.Sesh:       {"  )  ", " ( ( ", " ▄●▄ ", " ═╩═ "},
	journal.AVBCollect: {"◇ ◇ ◇", " ◇ ◇ ", "  ◇  ", " ═╩═ "},
	journal.AVBUse:     {"  ▽  ", " ▽ ▽ ", "▽ ▽ ▽", " ═╩═ "},
	journal.Adjust:     {"◢─┴─◣", "▽   ▽", "  │  ", " ═╩═ "},
}

// Card geometry. Every card in the séance is cut to the same size, front and
// back, so flipping one does not make the table twitch. The width and height
// are the whole card, border included; cardWellW is the writable well inside
// the border and its padding.
const (
	cardW     = 27
	cardH     = 13
	cardWellW = cardW - 4
	cardWellH = cardH - 2
)

// cardFrame is the border every card shares, coloured to the event it carries.
func (t *Theme) cardFrame(colour tint) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colour).
		Width(cardW).
		Height(cardH).
		Padding(0, 1)
}

// padRows stretches a card face to the well's height, so the front and the
// back always cut to the same size whatever the record left off the back.
func padRows(rows []string, height int) []string {
	for len(rows) < height {
		rows = append(rows, "")
	}
	return rows[:height]
}

// wrapName breaks a product name over up to two lines rather than cutting it
// short: a product is never abbreviated, only folded.
func wrapName(t *Theme, name string, width int) []string {
	if lipgloss.Width(name) <= width {
		return []string{center(t.Value.Render(name), width)}
	}
	words := strings.Fields(name)
	first, rest := "", ""
	for _, w := range words {
		if rest == "" && lipgloss.Width(strings.TrimSpace(first+" "+w)) <= width {
			first = strings.TrimSpace(first + " " + w)
			continue
		}
		rest = strings.TrimSpace(rest + " " + w)
	}
	return []string{
		center(t.Value.Render(first), width),
		center(truncate(t.Value.Render(rest), width), width),
	}
}

// spread sets two strings at opposite ends of a row, the way a playing card
// wears its corner pips.
func spread(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// CardFront renders the face of an event card: the corner pips, the figurine,
// the amount, and the product, in the colour of the account the grams moved to.
func (t *Theme) CardFront(e journal.Event, name string) string {
	colour := t.eventColor(e.Type)
	ink := lipgloss.NewStyle().Foreground(colour)
	w := cardWellW

	rows := []string{
		spread(ink.Render(glyphs[e.Type])+" "+t.Label.Render(verbs[e.Type]),
			t.Dim.Render(fmt.Sprintf("#%d", e.Seq)), w),
		"",
	}
	for _, line := range figurines[e.Type] {
		rows = append(rows, center(ink.Render(line), w))
	}
	rows = append(rows, "", center(t.Big.Render(fmt.Sprintf("%.2f", e.Grams))+t.Unit.Render(" g"), w))
	rows = append(rows, wrapName(t, name, w)...)
	// The date pins to the bottom edge, wherever the name folding left off.
	rows = append(padRows(rows, cardWellH-1),
		spread(t.Dim.Render(e.OccurredAt.Format("Mon 02 Jan")),
			ink.Render(glyphs[e.Type]), w))
	return t.cardFrame(colour).Render(strings.Join(rows, "\n"))
}

// CardBack renders the reverse of an event card: the record itself, with the
// timestamps, the accounts the grams moved between, and the tail of the hash
// that chains the entry to the one before it.
func (t *Theme) CardBack(e journal.Event, name string) string {
	colour := t.eventColor(e.Type)
	w := cardWellW
	// Each field spreads label and value to the card's edges, so a long value
	// borrows the gap a short label leaves rather than clipping at a column.
	field := func(label, value string) string {
		head := t.Label.Render(label)
		return spread(head, truncate(t.Value.Render(value), w-lipgloss.Width(head)-1), w)
	}

	rows := []string{
		spread(t.PanelTitle.Render("the record"), t.Dim.Render(fmt.Sprintf("#%d", e.Seq)), w),
		t.Dim.Render(strings.Repeat("─", w)),
		field("occurred", stamp(e.OccurredAt)),
		field("recorded", stamp(e.RecordedAt)),
		field("moved", fmt.Sprintf("%s→%s", e.From, e.To)),
	}
	// The product keeps its whole name, folded over the card's full width
	// rather than squeezed into the field column.
	rows = append(rows, wrapName(t, name, w)...)
	if e.Device != "" {
		rows = append(rows, field("device", e.Device))
	}
	if e.Temperature > 0 {
		rows = append(rows, field("temp", fmt.Sprintf("%d°C", e.Temperature)))
	}
	if e.Note != "" {
		rows = append(rows, field("note", e.Note))
	}
	rows = append(rows, field("hash", e.Hash[:min(12, len(e.Hash))]+"…"))
	return t.cardFrame(colour).Render(strings.Join(padRows(rows, cardWellH), "\n"))
}

// CardGhost renders a neighbouring card small and dim: enough to show the
// séance has more to say, not enough to say it.
func (t *Theme) CardGhost(e journal.Event) string {
	colour := t.eventColor(e.Type)
	ink := lipgloss.NewStyle().Foreground(colour)
	w := 9
	rows := []string{
		center(ink.Render(glyphs[e.Type]), w),
		center(t.Dim.Render(verbs[e.Type]), w),
		"",
		center(t.Dim.Render(fmt.Sprintf("%.1fg", e.Grams)), w),
		center(t.Dim.Render(e.OccurredAt.Format("02 Jan")), w),
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Muted).
		Width(w+4).Padding(0, 1).
		Render(strings.Join(rows, "\n"))
}

// CardSleeping is the face-down card shown before the séance has summoned
// anything: a patterned back and an invitation.
func (t *Theme) CardSleeping() string {
	w := cardWellW
	lace := t.Dim.Render(strings.Repeat("░▒", w/2))
	rows := []string{lace, lace, lace, "",
		center(t.Subtitle.Render("the ledger sleeps"), w),
		"",
		center(t.Dim.Render("press p to summon"), w),
		center(t.Dim.Render("what has been"), w),
		"", lace, lace, lace,
	}
	return t.cardFrame(t.Muted).Render(strings.Join(rows, "\n"))
}

// center pads a rendered string on both sides to sit mid-row.
func center(s string, width int) string {
	gap := width - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}
	left := gap / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", gap-left)
}

// stamp writes a moment compactly enough for a card's field column, dropping
// a midnight clock the way the journal does: a backfilled entry has a day,
// not a time, and printing 00:00 would claim a precision it never had.
func stamp(at time.Time) string {
	if h, m, _ := at.Clock(); h == 0 && m == 0 {
		return at.Format("02.01.2006")
	}
	return at.Format("02.01.06 15:04")
}
