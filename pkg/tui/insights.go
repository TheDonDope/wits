// Package tui holds the Insights screen here: what the command line and the
// browser show beyond the jars — the day so far, what it cost, the year on
// one card, the history and the prescriber's report, a trip packed, what is
// carried, and the map of the strains — one page at a time, each worked out
// by the package the others read it from.
package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/pack"
)

// insightsPage is one page of the Insights screen.
type insightsPage int

// The pages, in the order ←/→ steps through them.
const (
	pageToday insightsPage = iota
	pageCosts
	pageYear
	pageHistory
	pageTrip
	pageCarry
	pageStrains
)

var insightsPages = []string{"Today", "Costs", "Year", "History", "Trip", "Carry", "Strains"}

// insightsView is the Insights screen: the page in front, the year and the
// trip it is set to, and what was last drawn, kept until the journal or the
// settings change — the clock redraws every second, and a year folded again
// each time would be work for nothing.
type insightsView struct {
	page   insightsPage
	year   int // 0 is this year
	days   int
	oldest bool
	memo   *insightsMemo
}

// insightsMemo is the page last drawn and what it was drawn for, and the QR
// code a statement was signed into, until the page changes.
type insightsMemo struct {
	state *ledger.State
	key   string
	body  string
	qr    string
}

func newInsightsView() insightsView { return insightsView{days: 7, memo: &insightsMemo{}} }

// The screen's own keys, declared once for the help line and the dispatch.
var (
	pageKey   = key.NewBinding(key.WithKeys("left", "right", "h", "l"), key.WithHelp("←/→", "page"))
	stepKey   = key.NewBinding(key.WithKeys("-", "+", "="), key.WithHelp("-/+", "year, days"))
	oldestKey = key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "oldest first"))
	writeKey  = key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "write"))
)

type insightsKeys struct {
	keyMap
	Page, Step, Oldest, Write key.Binding
}

func (k insightsKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Next, k.Page, k.Step, k.Oldest, k.Write, k.Help, k.Quit}
}

func (k insightsKeys) FullHelp() [][]key.Binding {
	return append(k.keyMap.FullHelp(), []key.Binding{k.Page, k.Step, k.Oldest, k.Write})
}

// keys names w for what it does on the page in front.
func (v insightsView) keys(base keyMap) help.KeyMap {
	write := map[insightsPage]string{pageHistory: "write the report", pageTrip: "sign it", pageCarry: "sign it"}[v.page]
	k := insightsKeys{keyMap: base, Page: pageKey}
	if write != "" {
		k.Write = withHelp(writeKey, write)
	}
	if v.page == pageYear || v.page == pageTrip {
		k.Step = stepKey
	}
	if v.page == pageTrip {
		k.Oldest = oldestKey
	}
	return k
}

// Update turns the pages and their settings, and writes what w writes.
func (v insightsView) Update(msg tea.Msg, a *App) (insightsView, tea.Cmd) {
	press, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return v, nil
	}
	switch {
	case key.Matches(press, pageKey):
		step := 1
		if k := press.String(); k == "left" || k == "h" {
			step = -1
		}
		v.page = insightsPage((int(v.page) + step + len(insightsPages)) % len(insightsPages))
		v.memo.qr = ""
	case key.Matches(press, stepKey):
		v.step(press.String() == "-", a)
	case key.Matches(press, oldestKey) && v.page == pageTrip:
		v.oldest = !v.oldest
		v.memo.qr = ""
	case key.Matches(press, writeKey):
		v.write(a)
	}
	return v, nil
}

// step moves the year back or on, or the trip a day shorter or longer.
func (v *insightsView) step(back bool, a *App) {
	by := 1
	if back {
		by = -1
	}
	switch v.page {
	case pageYear:
		year := v.year
		if year == 0 {
			year = a.data.Now.Year()
		}
		v.year = min(year+by, a.data.Now.Year())
	case pageTrip:
		v.days = min(max(v.days+by, 1), 90)
		v.memo.qr = ""
	}
}

// View draws the page strip and the page, kept while nothing it shows changed.
func (v insightsView) View(a *App, height int) string {
	t := a.theme
	strip := make([]string, len(insightsPages))
	for i, name := range insightsPages {
		strip[i] = t.Dim.Render(name)
		if insightsPage(i) == v.page {
			strip[i] = lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Render(name)
		}
	}
	width := a.inner()
	key := fmt.Sprintf("%d·%d·%d·%t·%d·%s", v.page, v.year, v.days, v.oldest, width, t.ID)
	if v.memo.state != a.data.State || v.memo.key != key {
		v.memo.state, v.memo.key = a.data.State, key
		v.memo.body = v.render(a, width)
	}
	body := v.memo.body
	if v.memo.qr != "" {
		// The code alone, so none of it is cut off: a code cut off scans as nothing.
		body = lipgloss.JoinVertical(lipgloss.Left, t.Dim.Render("Scan it to check it; ←/→ goes back."), "", v.memo.qr)
	}
	return lipgloss.NewStyle().Padding(1, 1).Render(clip(lipgloss.JoinVertical(lipgloss.Left,
		strings.Join(strip, t.Dim.Render("  ·  ")), "", body), height-2))
}

// render draws the page in front.
func (v insightsView) render(a *App, width int) string {
	switch v.page {
	case pageCosts:
		return costsPage(a, width)
	case pageYear:
		return yearPage(a, v.year, width)
	case pageHistory:
		return historyPage(a, width)
	case pageTrip:
		return tripPage(a, v.days, v.mode(), width)
	case pageCarry:
		return carryPage(a, width)
	case pageStrains:
		return strainsPage(a, width)
	}
	return todayPage(a, width)
}

func (v insightsView) mode() pack.Mode {
	if v.oldest {
		return pack.Oldest
	}
	return pack.Even
}

// write is w: the report for the prescriber on History, a signed statement
// of what is packed or carried on Trip and Carry, its QR code drawn when the
// terminal has room for it. What was written, or why not, is the notice.
func (v insightsView) write(a *App) {
	var (
		said, qr string
		err      error
	)
	switch v.page {
	case pageHistory:
		said, err = writeReport(a)
	case pageTrip:
		said, qr, err = signTrip(a, v.days, v.mode())
	case pageCarry:
		said, qr, err = signCarry(a)
	default:
		return
	}
	if err != nil {
		a.notice, a.failed = err.Error(), true
		return
	}
	v.memo.qr = qr
	a.notice, a.failed = said, false
}

// day is a date as the pages write it.
func day(t time.Time) string { return t.Format("Mon 2 January") }
