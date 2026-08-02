package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
)

// storageView is two tables: what still holds something, and the history of
// everything weighed down to zero, newest first. Jars can be ticked in either
// table and weighed together in one sitting.
type storageView struct {
	view   viewport.Model
	cursor int
	marked map[string]bool // slugs ticked for weighing
}

func newStorageView() storageView {
	return storageView{view: viewport.New(), marked: map[string]bool{}}
}

type storageKeys struct {
	keyMap
	Mark, Reconcile, Edit, Clean key.Binding
}

// markKey and cleanKey are declared once, shared by the help line and the
// dispatch.
var (
	markKey  = key.NewBinding(key.WithKeys("space", "x"), key.WithHelp("space", "mark"))
	cleanKey = key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "clean history"))
)

func (k storageKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Mark, k.Reconcile, k.Edit, k.Clean, k.Help, k.Quit}
}

func (k storageKeys) FullHelp() [][]key.Binding {
	return append(k.keyMap.FullHelp(), []key.Binding{k.Mark, k.Reconcile, k.Edit, k.Clean})
}

func (v storageView) keys(base keyMap) help.KeyMap {
	return storageKeys{
		keyMap:    base,
		Mark:      markKey,
		Reconcile: withHelp(base.Weigh, "weigh"),
		Edit:      base.Edit,
		Clean:     cleanKey,
	}
}

// staleStashes returns the jars whose storage is finished but whose stash
// still carries a remainder, outside the cycle in progress. The imported
// years never recorded sessions, so their stashes read as if the grams were
// still there; these are what `clean history` zeroes out.
func staleStashes(a *App) []string {
	current := map[string]bool{}
	if c := a.data.Cycle(); c != nil {
		for _, slug := range c.Products {
			current[slug] = true
		}
	}
	var out []string
	for _, slug := range a.data.State.Products() {
		b := a.data.State.Balances[slug]
		if b != nil && !current[slug] && b.Storage <= 0 && b.Stash > 0 {
			out = append(out, slug)
		}
	}
	return out
}

// productRow is a product with everything the screen shows about it.
type productRow struct {
	Product  *catalog.Product
	Slug     string
	Storage  float64
	Stash    float64
	AVB      float64
	Ground   float64
	Bought   float64
	LastSeen time.Time
}

// Held is what is still on the shelf and in the stash.
func (p productRow) Held() float64 { return ledger.Round(p.Storage + p.Stash) }

// gone reports whether the product has been weighed down to nothing.
func (p productRow) gone() bool { return p.Held() <= 0 && p.AVB <= 0 }

func (v storageView) Update(msg tea.Msg, a *App) (storageView, tea.Cmd) {
	rows := v.rows(a)
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(msg, a.keys.Up):
			v.cursor = max(v.cursor-1, 0)
		case key.Matches(msg, a.keys.Down):
			v.cursor = min(v.cursor+1, max(len(rows)-1, 0))
		case key.Matches(msg, a.keys.PageUp):
			v.cursor = max(v.cursor-10, 0)
		case key.Matches(msg, a.keys.PgDown):
			v.cursor = min(v.cursor+10, max(len(rows)-1, 0))
		case key.Matches(msg, a.keys.Top):
			v.cursor = 0
		case key.Matches(msg, a.keys.Bottom):
			v.cursor = max(len(rows)-1, 0)
		case key.Matches(msg, markKey):
			if v.cursor < len(rows) {
				slug := rows[v.cursor].Slug
				if v.marked[slug] {
					delete(v.marked, slug)
				} else {
					v.marked[slug] = true
				}
			}
		}
	}
	return v, nil
}

// Selected returns the product under the cursor, in either table.
func (v storageView) Selected(a *App) *productRow {
	rows := v.rows(a)
	if v.cursor < 0 || v.cursor >= len(rows) {
		return nil
	}
	return &rows[v.cursor]
}

// Marked returns the ticked slugs in the order the tables show them.
func (v storageView) Marked(a *App) []string {
	var out []string
	for _, r := range v.rows(a) {
		if v.marked[r.Slug] {
			out = append(out, r.Slug)
		}
	}
	return out
}

// ClearMarks unticks everything, for after a weighing has been recorded.
func (v *storageView) ClearMarks() { v.marked = map[string]bool{} }

// rows is the cursor's world: the shelf first, then the history, both newest
// first, so one index walks both tables.
func (v storageView) rows(a *App) []productRow {
	shelf, history := v.tables(a)
	return append(shelf, history...)
}

// tables folds the journal into one row per product and splits the result
// into what still holds something and what has been weighed down to zero.
func (v storageView) tables(a *App) (shelf, history []productRow) {
	byProduct := map[string]*productRow{}
	get := func(slug string) *productRow {
		if r, ok := byProduct[slug]; ok {
			return r
		}
		r := &productRow{Slug: slug}
		if a.data.Products != nil {
			if p, err := a.data.Products.Find(slug); err == nil {
				r.Product = p
			}
		}
		byProduct[slug] = r
		return r
	}

	for _, e := range a.data.State.Events {
		r := get(e.Product)
		if e.OccurredAt.After(r.LastSeen) {
			r.LastSeen = e.OccurredAt
		}
		switch e.Type {
		case journal.Purchase:
			r.Bought = ledger.Round(r.Bought + e.Grams)
		case journal.Grind:
			r.Ground = ledger.Round(r.Ground + e.Grams)
		}
	}
	for slug, b := range a.data.State.Balances {
		r := get(slug)
		r.Storage, r.Stash, r.AVB = b.Storage, b.Stash, b.AVB
	}

	all := make([]productRow, 0, len(byProduct))
	for _, r := range byProduct {
		all = append(all, *r)
	}
	// Most recently touched first in both tables: what was used yesterday is
	// what will be weighed today, and the history reads newest to oldest.
	sort.Slice(all, func(i, j int) bool {
		if !all[i].LastSeen.Equal(all[j].LastSeen) {
			return all[i].LastSeen.After(all[j].LastSeen)
		}
		return all[i].Slug < all[j].Slug
	})
	for _, r := range all {
		if r.gone() {
			history = append(history, r)
		} else {
			shelf = append(shelf, r)
		}
	}
	return shelf, history
}

// Column widths for everything except the product name, which takes whatever
// the longest name needs. Names are not abbreviated: they are what the jars
// are called, and the numbers can afford to be narrow.
const (
	markW   = 4 // cursor bar and checkbox
	potW    = 8
	gramW   = 10
	groundW = 11
	lastW   = 13
)

// nameWidth returns the width of the product column: the longest full name,
// shrunk only when the terminal physically cannot hold it.
func nameWidth(a *App, rows []productRow, width int) int {
	nameW := lipgloss.Width("PRODUCT")
	for _, r := range rows {
		nameW = max(nameW, lipgloss.Width(a.data.ProductName(r.Slug)))
	}
	fixed := markW + potW + 3*gramW + groundW
	if nameW > width-fixed {
		nameW = max(width-fixed, 12)
	}
	return nameW
}

func (v storageView) View(a *App, height int) string {
	t := a.theme
	width := a.inner()
	shelf, history := v.tables(a)
	rows := append(append([]productRow{}, shelf...), history...)

	if len(rows) == 0 {
		return lipgloss.NewStyle().Padding(1, 1).Render(
			t.Subtitle.Render("Nothing in storage yet.\n\nRecord a prescription fill with ") +
				t.Key.Render("b") + t.Subtitle.Render(" or `wits buy`."))
	}
	if v.cursor >= len(rows) {
		v.cursor = len(rows) - 1
	}
	nameW := nameWidth(a, rows, width)

	lines := []string{v.status(a, shelf, history, width), ""}
	cursorLine := 0

	section, at := v.shelfSection(a, shelf, nameW, width, len(lines))
	lines, cursorLine = append(lines, section...), max(cursorLine, at)

	section, at = v.historySection(a, history, len(shelf), nameW, width, len(lines))
	lines, cursorLine = append(lines, section...), max(cursorLine, at)

	body := max(height-1, 1)
	v.view.SetWidth(width)
	v.view.SetHeight(body)
	v.view.SetContent(strings.Join(lines, "\n"))
	v.view.SetYOffset(scrollTo(cursorLine, v.view.YOffset(), body, len(lines)))
	return lipgloss.NewStyle().Padding(0, 1).Render(v.view.View())
}

// shelfSection renders the table of jars still holding something, reporting
// the line the cursor sits on, or -1 when it is elsewhere.
func (v storageView) shelfSection(a *App, shelf []productRow, nameW, width, offset int) ([]string, int) {
	t := a.theme
	cursorLine := -1
	lines := []string{t.Rule("Storage", width), v.shelfHeader(a, nameW)}
	if len(shelf) == 0 {
		lines = append(lines, t.Dim.Render("  nothing on the shelf"))
	}
	for i, r := range shelf {
		if i == v.cursor {
			cursorLine = offset + len(lines)
		}
		lines = append(lines, v.shelfLine(a, r, nameW, i == v.cursor))
		if i == v.cursor {
			lines = append(lines, v.detail(a, r, width))
		}
	}
	return lines, cursorLine
}

// historySection renders the finished jars, and offers the clean-up: the
// stale jars sit in the table above — storage done, a stash remainder nobody
// will grind again — but cleaning them is what fills this table, so the offer
// is made here.
func (v storageView) historySection(a *App, history []productRow, shelved, nameW, width, offset int) ([]string, int) {
	t := a.theme
	cursorLine := -1
	lines := []string{"", t.Rule("History", width), v.historyHeader(a, nameW)}
	if stale := staleStashes(a); len(stale) > 0 {
		lines = append(lines, t.Dim.Render(fmt.Sprintf(
			"  %s with a stash remainder from earlier cycles — press ", plural(len(stale), "jar")))+
			t.Key.Render("c")+t.Dim.Render(" to record it as consumed"))
	}
	if len(history) == 0 {
		lines = append(lines, t.Dim.Render("  nothing weighed down to zero yet"))
	}
	for i, r := range history {
		at := shelved + i
		if at == v.cursor {
			cursorLine = offset + len(lines)
		}
		lines = append(lines, v.historyLine(a, r, nameW, at == v.cursor))
		if at == v.cursor {
			lines = append(lines, v.detail(a, r, width))
		}
	}
	return lines, cursorLine
}

// status is the one-line summary above the tables.
func (v storageView) status(a *App, shelf, history []productRow, width int) string {
	t := a.theme
	left := fmt.Sprintf("on the shelf · %d   history · %d", len(shelf), len(history))
	if n := len(v.Marked(a)); n > 0 {
		left += fmt.Sprintf("   %s marked — press r to weigh them", plural(n, "jar"))
	}
	return t.Dim.Render(truncate(left, width))
}

// prefix renders the cursor bar and the checkbox in front of a row.
func (v storageView) prefix(a *App, slug string, selected bool) string {
	t := a.theme
	bar := " "
	if selected {
		bar = lipgloss.NewStyle().Foreground(t.Accent).Render("│")
	}
	box := t.Dim.Render("☐")
	if v.marked[slug] {
		box = lipgloss.NewStyle().Foreground(t.Accent).Render("☑")
	}
	return bar + box + " "
}

func (v storageView) shelfHeader(a *App, nameW int) string {
	t := a.theme
	return lipgloss.JoinHorizontal(lipgloss.Left,
		strings.Repeat(" ", markW-1),
		t.Label.Width(nameW).Render("PRODUCT"),
		t.Label.Width(potW).Align(lipgloss.Right).Render("THC/CBD"),
		t.Label.Width(gramW).Align(lipgloss.Right).Render("STORAGE"),
		t.Label.Width(gramW).Align(lipgloss.Right).Render("STASH"),
		t.Label.Width(gramW).Align(lipgloss.Right).Render("AVB"),
		t.Label.Width(groundW).Align(lipgloss.Right).Render("GROUND"),
	)
}

// shelfLine renders one product still holding something.
func (v storageView) shelfLine(a *App, r productRow, nameW int, selected bool) string {
	t := a.theme
	name := truncate(a.data.ProductName(r.Slug), nameW)
	label := t.Value.Width(nameW).Render(name)
	if selected {
		label = lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Width(nameW).Render(name)
	}
	return lipgloss.JoinHorizontal(lipgloss.Left,
		v.prefix(a, r.Slug, selected),
		label,
		t.Dim.Width(potW).Align(lipgloss.Right).Render(potency(r)),
		grams(t, r.Storage, t.StorageC, gramW),
		grams(t, r.Stash, t.StashC, gramW),
		grams(t, r.AVB, t.AVBC, gramW),
		t.Dim.Width(groundW).Align(lipgloss.Right).Render(fmt.Sprintf("%.2f g", r.Ground)),
	)
}

func (v storageView) historyHeader(a *App, nameW int) string {
	t := a.theme
	return lipgloss.JoinHorizontal(lipgloss.Left,
		strings.Repeat(" ", markW-1),
		t.Label.Width(nameW).Render("PRODUCT"),
		t.Label.Width(potW).Align(lipgloss.Right).Render("THC/CBD"),
		t.Label.Width(gramW).Align(lipgloss.Right).Render("BOUGHT"),
		t.Label.Width(groundW).Align(lipgloss.Right).Render("GROUND"),
		t.Label.Width(lastW).Align(lipgloss.Right).Render("LAST USED"),
	)
}

// historyLine renders one finished product. Its balances are all zero, so the
// row says what it was and when it ended rather than repeating three zeroes.
func (v storageView) historyLine(a *App, r productRow, nameW int, selected bool) string {
	t := a.theme
	name := truncate(a.data.ProductName(r.Slug), nameW)
	label := t.Dim.Width(nameW).Render(name)
	if selected {
		label = lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Width(nameW).Render(name)
	}
	last := "—"
	if !r.LastSeen.IsZero() {
		last = humanDay(r.LastSeen, a.data.Now)
	}
	return lipgloss.JoinHorizontal(lipgloss.Left,
		v.prefix(a, r.Slug, selected),
		label,
		t.Dim.Width(potW).Align(lipgloss.Right).Render(potency(r)),
		t.Dim.Width(gramW).Align(lipgloss.Right).Render(fmt.Sprintf("%.2f g", r.Bought)),
		t.Dim.Width(groundW).Align(lipgloss.Right).Render(fmt.Sprintf("%.2f g", r.Ground)),
		t.Dim.Width(lastW).Align(lipgloss.Right).Render(last),
	)
}

// potency renders the THC/CBD ratio, or a dash where none is known.
func potency(r productRow) string {
	if r.Product != nil && r.Product.THC > 0 {
		return fmt.Sprintf("%g/%g", r.Product.THC, r.Product.CBD)
	}
	return "—"
}

// grams renders an amount in an account's colour, dimmed when it is empty so
// that a column of zeroes does not compete with the figures that matter.
func grams(t *Theme, g float64, colour tint, width int) string {
	style := lipgloss.NewStyle().Foreground(colour)
	if g <= 0 {
		style = t.Dim
	}
	return style.Width(width).Align(lipgloss.Right).Render(fmt.Sprintf("%.2f g", g))
}

// detail is the block under the selected product.
func (v storageView) detail(a *App, r productRow, width int) string {
	t := a.theme

	var facts []string
	if r.Product != nil {
		if r.Product.Manufacturer != "" {
			facts = append(facts, r.Product.Manufacturer)
		}
		if r.Product.Cultivar != "" {
			facts = append(facts, r.Product.Cultivar)
		}
	}
	if !r.LastSeen.IsZero() {
		facts = append(facts, "last used "+humanDay(r.LastSeen, a.data.Now))
	}

	rows := []string{"  " + t.Dim.Render(strings.Join(facts, " · "))}
	// The bar is what is still held against what was dispensed, which is what
	// the label beside it says. A finished jar has nothing to fill a bar with.
	if r.Bought > 0 && r.Held() > 0 {
		remaining := clamp(r.Held()/r.Bought, 0, 1)
		bar := Gauge(max(width-46, 10), remaining, t.Level(remaining), t.Dim)
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Left,
			"  ", bar, " ",
			t.Dim.Render(fmt.Sprintf("%.2f g of %.2f g dispensed still held", r.Held(), r.Bought)),
		))
	}
	rows = append(rows,
		"  "+t.Dim.Render("press ")+t.Key.Render("space")+t.Dim.Render(" to mark, ")+
			t.Key.Render("r")+t.Dim.Render(" to weigh, ")+
			t.Key.Render("e")+t.Dim.Render(" to edit"),
		"")
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// humanDay says today or yesterday where it can, and a date otherwise.
func humanDay(d, now time.Time) string {
	switch daysBetween(d, now) {
	case 0:
		return "today"
	case 1:
		return "yesterday"
	default:
		return d.Format("02 Jan 2006")
	}
}
