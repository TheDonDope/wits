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
	player
}

func newStorageView() storageView {
	return storageView{view: viewport.New(), marked: map[string]bool{}, player: newPlayer()}
}

type storageKeys struct {
	keyMap
	Mark, Reconcile, Edit, Clean, Play key.Binding
}

// markKey and cleanKey are declared once, shared by the help line and the
// dispatch.
var (
	markKey  = key.NewBinding(key.WithKeys("space", "x"), key.WithHelp("space", "mark"))
	cleanKey = key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "clean history"))
)

func (k storageKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Mark, k.Reconcile, k.Edit, k.Clean, k.Play, k.Help, k.Quit}
}

func (k storageKeys) FullHelp() [][]key.Binding {
	return append(k.keyMap.FullHelp(), []key.Binding{k.Mark, k.Reconcile, k.Edit, k.Clean, k.Play})
}

func (v storageView) keys(base keyMap) help.KeyMap {
	return storageKeys{
		keyMap:    base,
		Mark:      markKey,
		Reconcile: withHelp(base.Weigh, "weigh"),
		Edit:      base.Edit,
		Clean:     cleanKey,
		Play:      playKey,
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
	var cmd tea.Cmd
	if _, ok := msg.(playTickMsg); ok {
		v.player, cmd = v.player.advance(a.data.State.Events)
		return v, cmd
	}
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(msg, playKey):
			v.player, cmd = v.player.toggle(a.data.State.Events)
			return v, cmd
		case key.Matches(msg, slideKey):
			v.player = v.player.step(msg, a.data.State.Events)
		case key.Matches(msg, speedKeys):
			v.player = v.player.retune(msg)
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
// Under playback only the applied prefix exists, so the shelf fills and the
// history writes itself the way it happened.
func (v storageView) tables(a *App) (shelf, history []productRow) {
	events := v.played(a.data.State.Events)
	balances := a.data.State.Balances
	if v.playhead >= 0 {
		balances = ledger.Fold(events).Balances
	}

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

	for _, e := range events {
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
	for slug, b := range balances {
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

	if len(rows) == 0 && v.playhead < 0 {
		return lipgloss.NewStyle().Padding(1, 1).Render(
			t.Subtitle.Render("Nothing in storage yet.\n\nRecord a prescription fill with ") +
				t.Key.Render("b") + t.Subtitle.Render(" or `wits buy`."))
	}
	if v.cursor >= len(rows) {
		v.cursor = len(rows) - 1
	}
	nameW := nameWidth(a, rows, width)

	lines := []string{v.status(a, shelf, history, width),
		v.transport(a, a.data.State.Events, width), ""}
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
	if stale := staleStashes(a); len(stale) > 0 && v.playhead < 0 {
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

// The stash screen is the storage screen's drill-down: the tins that hold
// ground product now, and under them the story of every stash already worked
// down to nothing, grouped under the day it was finished.

// stashStats is what the stash screen knows about one product's stash beyond
// its current balance: everything that ever passed through it, how many
// sessions drew on it, and when it last reached zero.
type stashStats struct {
	Slug      string
	Through   float64   // grams ever ground into the stash
	Sessions  int       // sessions drawn from it
	LastTouch time.Time // the last event that moved its stash
	EmptiedAt time.Time // when the stash last reached zero; zero while it holds
}

// stashHistoryOf replays a run of events watching only the stash account, so
// the screen can say when a stash was finished rather than only that it is
// empty — and, given a prefix of the ledger, replay it.
func stashHistoryOf(events []journal.Event) map[string]*stashStats {
	out := map[string]*stashStats{}
	bal := map[string]float64{}
	get := func(slug string) *stashStats {
		if s, ok := out[slug]; ok {
			return s
		}
		s := &stashStats{Slug: slug}
		out[slug] = s
		return s
	}
	for _, e := range events {
		touched := false
		if e.To == journal.Stash {
			s := get(e.Product)
			bal[e.Product] = ledger.Round(bal[e.Product] + e.Grams)
			s.Through = ledger.Round(s.Through + e.Grams)
			if bal[e.Product] > 0 {
				// Refilled: the old ending no longer ends the story.
				s.EmptiedAt = time.Time{}
			}
			touched = true
		}
		if e.From == journal.Stash {
			s := get(e.Product)
			bal[e.Product] = ledger.Round(bal[e.Product] - e.Grams)
			if bal[e.Product] <= 0 {
				s.EmptiedAt = e.OccurredAt
			}
			touched = true
		}
		if touched {
			s := get(e.Product)
			if e.OccurredAt.After(s.LastTouch) {
				s.LastTouch = e.OccurredAt
			}
			if e.Type == journal.Sesh {
				s.Sessions++
			}
		}
	}
	return out
}

// stashView is two tables: the stashes holding something, and the finished
// ones grouped under the day they were consumed.
type stashView struct {
	view   viewport.Model
	cursor int
	marked map[string]bool
	player
}

func newStashView() stashView {
	return stashView{view: viewport.New(), marked: map[string]bool{}, player: newPlayer()}
}

type stashKeys struct {
	keyMap
	Mark, Reconcile, Edit, Play key.Binding
}

func (k stashKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Mark, k.Reconcile, k.Edit, k.Play, k.Help, k.Quit}
}

func (k stashKeys) FullHelp() [][]key.Binding {
	return append(k.keyMap.FullHelp(), []key.Binding{k.Mark, k.Reconcile, k.Edit, k.Play})
}

func (v stashView) keys(base keyMap) help.KeyMap {
	return stashKeys{
		keyMap:    base,
		Mark:      markKey,
		Reconcile: withHelp(base.Weigh, "weigh"),
		Edit:      base.Edit,
		Play:      playKey,
	}
}

// balances returns the account balances the screen reads: the live fold, or
// one over only what the playback has applied.
func (v stashView) balances(a *App) map[string]*ledger.Balance {
	if v.playhead < 0 {
		return a.data.State.Balances
	}
	return ledger.Fold(v.played(a.data.State.Events)).Balances
}

// active returns the stashes holding something, fullest first: the tin most
// worth weighing is the one with the most in it.
func (v stashView) active(a *App) []*stashStats {
	stats := stashHistoryOf(v.played(a.data.State.Events))
	balances := v.balances(a)
	var out []*stashStats
	for slug, b := range balances {
		if b.Stash > 0 {
			out = append(out, stats[slug])
		}
	}
	sort.Slice(out, func(i, j int) bool {
		bi, bj := balances[out[i].Slug], balances[out[j].Slug]
		if bi.Stash != bj.Stash {
			return bi.Stash > bj.Stash
		}
		return out[i].Slug < out[j].Slug
	})
	return out
}

// finished returns the emptied stashes, newest ending first, so the history
// reads back the way the journal does.
func (v stashView) finished(a *App) []*stashStats {
	stats := stashHistoryOf(v.played(a.data.State.Events))
	balances := v.balances(a)
	var out []*stashStats
	for slug, s := range stats {
		b := balances[slug]
		if s.Through > 0 && (b == nil || b.Stash <= 0) {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].EmptiedAt.Equal(out[j].EmptiedAt) {
			return out[i].EmptiedAt.After(out[j].EmptiedAt)
		}
		return out[i].Slug < out[j].Slug
	})
	return out
}

// slugsInOrder is the cursor's world: active stashes, then the finished ones.
func (v stashView) slugsInOrder(a *App) []string {
	var out []string
	for _, s := range v.active(a) {
		out = append(out, s.Slug)
	}
	for _, s := range v.finished(a) {
		out = append(out, s.Slug)
	}
	return out
}

func (v stashView) Update(msg tea.Msg, a *App) (stashView, tea.Cmd) {
	slugs := v.slugsInOrder(a)
	var cmd tea.Cmd
	if _, ok := msg.(playTickMsg); ok {
		v.player, cmd = v.player.advance(a.data.State.Events)
		return v, cmd
	}
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(msg, playKey):
			v.player, cmd = v.player.toggle(a.data.State.Events)
			return v, cmd
		case key.Matches(msg, slideKey):
			v.player = v.player.step(msg, a.data.State.Events)
		case key.Matches(msg, speedKeys):
			v.player = v.player.retune(msg)
		case key.Matches(msg, a.keys.Up):
			v.cursor = max(v.cursor-1, 0)
		case key.Matches(msg, a.keys.Down):
			v.cursor = min(v.cursor+1, max(len(slugs)-1, 0))
		case key.Matches(msg, a.keys.PageUp):
			v.cursor = max(v.cursor-10, 0)
		case key.Matches(msg, a.keys.PgDown):
			v.cursor = min(v.cursor+10, max(len(slugs)-1, 0))
		case key.Matches(msg, a.keys.Top):
			v.cursor = 0
		case key.Matches(msg, a.keys.Bottom):
			v.cursor = max(len(slugs)-1, 0)
		case key.Matches(msg, markKey):
			if v.cursor < len(slugs) {
				slug := slugs[v.cursor]
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

// Selected returns the slug under the cursor.
func (v stashView) Selected(a *App) string {
	slugs := v.slugsInOrder(a)
	if v.cursor < 0 || v.cursor >= len(slugs) {
		return ""
	}
	return slugs[v.cursor]
}

// Marked returns the ticked slugs in display order.
func (v stashView) Marked(a *App) []string {
	var out []string
	for _, slug := range v.slugsInOrder(a) {
		if v.marked[slug] {
			out = append(out, slug)
		}
	}
	return out
}

// ClearMarks unticks everything.
func (v *stashView) ClearMarks() { v.marked = map[string]bool{} }

// Stash column widths beyond the product name.
const (
	stashGramW = 10
	throughW   = 11
	seshCountW = 12
	lastSeshW  = 13
)

func (v stashView) View(a *App, height int) string {
	t := a.theme
	width := a.inner()
	active, finished := v.active(a), v.finished(a)

	if len(active) == 0 && len(finished) == 0 && v.playhead < 0 {
		return lipgloss.NewStyle().Padding(1, 1).Render(
			t.Subtitle.Render("Nothing has been ground yet.\n\nPress ") + t.Key.Render("n") +
				t.Subtitle.Render(" to grind into a stash, or record it with `wits grind`."))
	}
	slugs := v.slugsInOrder(a)
	if v.cursor >= len(slugs) {
		v.cursor = len(slugs) - 1
	}
	nameW := stashNameWidth(a, slugs, width)

	marked := ""
	if n := len(v.Marked(a)); n > 0 {
		marked = fmt.Sprintf("   %s marked — press r to weigh them", plural(n, "stash"))
	}
	lines := []string{t.Dim.Render(truncate(fmt.Sprintf(
		"holding · %d   finished · %d%s", len(active), len(finished), marked), width)),
		v.transport(a, a.data.State.Events, width), ""}
	cursorLine := 0

	section, at := v.activeSection(a, active, nameW, width, len(lines))
	lines, cursorLine = append(lines, section...), max(cursorLine, at)

	section, at = v.finishedSection(a, finished, len(active), nameW, width, len(lines))
	lines, cursorLine = append(lines, section...), max(cursorLine, at)

	body := max(height-1, 1)
	v.view.SetWidth(width)
	v.view.SetHeight(body)
	v.view.SetContent(strings.Join(lines, "\n"))
	v.view.SetYOffset(scrollTo(cursorLine, v.view.YOffset(), body, len(lines)))
	return lipgloss.NewStyle().Padding(0, 1).Render(v.view.View())
}

// stashNameWidth is nameWidth for the stash tables' narrower fixed columns.
func stashNameWidth(a *App, slugs []string, width int) int {
	nameW := lipgloss.Width("PRODUCT")
	for _, slug := range slugs {
		nameW = max(nameW, lipgloss.Width(a.data.ProductName(slug)))
	}
	fixed := markW + potW + stashGramW + throughW + seshCountW + lastSeshW
	if nameW > width-fixed {
		nameW = max(width-fixed, 12)
	}
	return nameW
}

// activeSection renders the stashes holding something.
func (v stashView) activeSection(a *App, active []*stashStats, nameW, width, offset int) ([]string, int) {
	t := a.theme
	cursorLine := -1
	lines := []string{t.Rule("Stash", width), lipgloss.JoinHorizontal(lipgloss.Left,
		strings.Repeat(" ", markW-1),
		t.Label.Width(nameW).Render("PRODUCT"),
		t.Label.Width(potW).Align(lipgloss.Right).Render("THC/CBD"),
		t.Label.Width(stashGramW).Align(lipgloss.Right).Render("STASH"),
		t.Label.Width(throughW).Align(lipgloss.Right).Render("THROUGH"),
		t.Label.Width(seshCountW).Align(lipgloss.Right).Render("SESSIONS"),
		t.Label.Width(lastSeshW).Align(lipgloss.Right).Render("LAST USED"),
	)}
	if len(active) == 0 {
		lines = append(lines, t.Dim.Render("  every stash is empty"))
	}
	for i, s := range active {
		if i == v.cursor {
			cursorLine = offset + len(lines)
		}
		lines = append(lines, v.activeLine(a, s, nameW, i == v.cursor))
	}
	return lines, cursorLine
}

// activeLine renders one stash still holding something.
func (v stashView) activeLine(a *App, s *stashStats, nameW int, selected bool) string {
	t := a.theme
	b := v.balances(a)[s.Slug]
	name := truncate(a.data.ProductName(s.Slug), nameW)
	label := t.Value.Width(nameW).Render(name)
	if selected {
		label = lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Width(nameW).Render(name)
	}
	last := "—"
	if !s.LastTouch.IsZero() {
		last = humanDay(s.LastTouch, a.data.Now)
	}
	return lipgloss.JoinHorizontal(lipgloss.Left,
		v.prefix(a, s.Slug, selected),
		label,
		t.Dim.Width(potW).Align(lipgloss.Right).Render(stashPotency(a, s.Slug)),
		grams(t, b.Stash, t.StashC, stashGramW),
		t.Dim.Width(throughW).Align(lipgloss.Right).Render(fmt.Sprintf("%.2f g", s.Through)),
		t.Dim.Width(seshCountW).Align(lipgloss.Right).Render(fmt.Sprintf("%d", s.Sessions)),
		t.Dim.Width(lastSeshW).Align(lipgloss.Right).Render(last),
	)
}

// finishedSection renders the emptied stashes under the day each was finished.
func (v stashView) finishedSection(a *App, finished []*stashStats, activeCount, nameW, width, offset int) ([]string, int) {
	t := a.theme
	cursorLine := -1
	lines := []string{"", t.Rule("Consumed", width)}
	if len(finished) == 0 {
		lines = append(lines, t.Dim.Render("  no stash finished yet"))
	}
	lastDay := ""
	for i, s := range finished {
		day := "earlier"
		if !s.EmptiedAt.IsZero() {
			day = s.EmptiedAt.Format(time.DateOnly)
		}
		if day != lastDay {
			heading := "earlier"
			if !s.EmptiedAt.IsZero() {
				heading = s.EmptiedAt.Format("Mon 02 Jan 2006")
			}
			lines = append(lines, t.PanelTitle.Render(heading))
			lastDay = day
		}
		at := activeCount + i
		if at == v.cursor {
			cursorLine = offset + len(lines)
		}
		lines = append(lines, v.finishedLine(a, s, nameW, at == v.cursor))
	}
	return lines, cursorLine
}

// finishedLine renders one emptied stash: what it was, what went through it,
// and how many sessions that took.
func (v stashView) finishedLine(a *App, s *stashStats, nameW int, selected bool) string {
	t := a.theme
	name := truncate(a.data.ProductName(s.Slug), nameW)
	label := t.Dim.Width(nameW).Render(name)
	if selected {
		label = lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Width(nameW).Render(name)
	}
	return lipgloss.JoinHorizontal(lipgloss.Left,
		v.prefix(a, s.Slug, selected),
		label,
		t.Dim.Width(potW).Align(lipgloss.Right).Render(stashPotency(a, s.Slug)),
		t.Dim.Width(throughW).Align(lipgloss.Right).Render(fmt.Sprintf("%.2f g", s.Through)),
		t.Dim.Width(seshCountW).Align(lipgloss.Right).Render(plural(s.Sessions, "session")),
	)
}

// prefix renders the cursor bar and checkbox, shared with the storage screen's
// idea of marking.
func (v stashView) prefix(a *App, slug string, selected bool) string {
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

// stashPotency is potency by slug rather than by row.
func stashPotency(a *App, slug string) string {
	if a.data.Products != nil {
		if p, err := a.data.Products.Find(slug); err == nil && p.THC > 0 {
			return fmt.Sprintf("%g/%g", p.THC, p.CBD)
		}
	}
	return "—"
}
