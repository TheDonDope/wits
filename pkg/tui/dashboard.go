package tui

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
)

// dashboard is the screen you land on: a grid of cards, one per corner of the
// system, each summarising what its own screen knows in a few lines — plus
// where the supply is heading, which no single screen says.
type dashboard struct {
	scroll int
}

func newDashboard() dashboard { return dashboard{} }

func (d dashboard) Update(msg tea.Msg, a *App) (dashboard, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(msg, a.keys.Up):
			d.scroll = max(d.scroll-1, 0)
		case key.Matches(msg, a.keys.Down):
			d.scroll++
		case key.Matches(msg, a.keys.PageUp):
			d.scroll = max(d.scroll-10, 0)
		case key.Matches(msg, a.keys.PgDown):
			d.scroll += 10
		case key.Matches(msg, a.keys.Top):
			d.scroll = 0
		}
	}
	return d, nil
}

func (d dashboard) View(a *App, height int) string {
	t, data := a.theme, a.data
	width := a.inner()

	cycle := data.Cycle()
	if cycle == nil {
		return lipgloss.NewStyle().Padding(1, 1).Render(
			t.Subtitle.Render("No cycle yet.\n\nRecord a prescription fill with ") +
				t.Key.Render("wits buy") + t.Subtitle.Render(" to get started."))
	}

	sections := []string{d.quickActions(a, width)}
	sections = append(sections, d.grid(a, cycle, width)...)

	lines := strings.Split(lipgloss.JoinVertical(lipgloss.Left, sections...), "\n")
	visible := max(height-2, 1)
	offset := min(d.scroll, max(len(lines)-visible, 0))
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

// grid lays the cards out two abreast where the terminal allows, and single
// file where it does not.
func (d dashboard) grid(a *App, c *ledger.Cycle, width int) []string {
	colW := (width - 1) / 2
	pair := func(left, right string) string {
		return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	}
	if width < 84 {
		colW = width
		pair = func(left, right string) string {
			return lipgloss.JoinVertical(lipgloss.Left, left, right)
		}
	}
	inner := colW - 4 // the card's border and padding

	return []string{
		pair(card(a, "Storage", d.storageCard(a, c, inner), colW),
			card(a, "Stash", d.stashCard(a, inner), colW)),
		pair(card(a, "Sessions", d.sessionsCard(a, inner), colW),
			card(a, "Devices", d.devicesCard(a, inner), colW)),
		card(a, "Supply projection", d.projectionCard(a, c, width-4), width),
		pair(card(a, "Rhythm · grinding", rhythmCard(a, journal.Grind, inner), colW),
			card(a, "Rhythm · sessions", rhythmCard(a, journal.Sesh, inner), colW)),
	}
}

// card frames a body the way every card is framed, so the grid reads as one
// thing rather than five styles.
func card(a *App, title, body string, w int) string {
	t := a.theme
	return t.Panel.Width(w).Render(lipgloss.JoinVertical(lipgloss.Left,
		t.PanelTitle.Render(title), body))
}

// quickActions is the row of things worth doing from here, spelled out rather
// than left in the help line.
func (d dashboard) quickActions(a *App, width int) string {
	t := a.theme
	actions := []struct{ key, label string }{
		{"n", "grind"}, {"s", "sesh"}, {"b", "fill"}, {"r", "weigh"},
	}
	cells := make([]string, 0, len(actions))
	for _, act := range actions {
		cells = append(cells, lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).BorderForeground(t.Line).
			Padding(0, 1).Render(
			lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Render(act.key)+
				t.Dim.Render("  "+act.label)))
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
	if lipgloss.Width(row) > width {
		// Too narrow for the boxes; the help line still carries the same keys.
		return ""
	}
	return row
}

// storageCard is the cycle at a glance: what remains, how it is going, and how
// long it lasts.
func (d dashboard) storageCard(a *App, c *ledger.Cycle, w int) string {
	t := a.theme
	stats := ledger.Summarise(c.Events)
	remaining := c.Remaining()
	fraction := c.RemainingPct()

	headline := t.Big.Render(fmt.Sprintf("%.2f g", remaining)) +
		t.Dim.Render(fmt.Sprintf("  of %.0f g · cycle %d, day %d",
			c.Held(), len(a.data.State.Cycles), daysBetween(c.Start, a.data.Now)+1))
	bar := GradientGauge(w, fraction, t.Good, t.Level(fraction), t.Dim)

	runway := "nothing ground yet, no rate to project"
	if days := stats.DaysLeft(remaining); days > 0 {
		runway = fmt.Sprintf("lasts ~%.0f days at %.2f g per active day", days, stats.PerActiveDay)
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		headline,
		bar,
		t.Dim.Render(fmt.Sprintf("%s on the shelf · %s", plural(len(c.Products), "product"), runway)),
	)
}

// stashCard is the ground product: how much sits in the stashes, and where.
func (d dashboard) stashCard(a *App, w int) string {
	t, data := a.theme, a.data
	total, peak := 0.0, 0.0
	var tins []stashShare
	for slug, b := range data.State.Balances {
		if b.Stash > 0 {
			tins = append(tins, stashShare{slug, b.Stash})
			total += b.Stash
			peak = math.Max(peak, b.Stash)
		}
	}
	sort.Slice(tins, func(i, j int) bool {
		if tins[i].grams != tins[j].grams {
			return tins[i].grams > tins[j].grams
		}
		return tins[i].slug < tins[j].slug
	})
	if len(tins) > 3 {
		tins = tins[:3]
	}

	lines := []string{t.Big.Render(fmt.Sprintf("%.2f g", total)) +
		t.Dim.Render(fmt.Sprintf("  ground, across %s", plural(len(tins), "stash")))}
	for _, tn := range tins {
		labelW := min(w/2, 24)
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
			t.Value.Width(labelW).Render(truncate(data.ProductName(tn.slug), labelW)),
			" ",
			Gauge(max(w-labelW-10, 6), tn.grams/peak, t.StashC, t.Dim),
			t.Dim.Render(fmt.Sprintf(" %.2f g", tn.grams)),
		))
	}
	if len(lines) == 1 {
		lines = append(lines, t.Dim.Render("every stash is empty — n grinds into one"))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// stashShare is one stash and what it holds, for the card's mini bars.
type stashShare struct {
	slug  string
	grams float64
}

// sessionsCard is the sessions at a glance: how many, how much, and the shape
// of the last two weeks.
func (d dashboard) sessionsCard(a *App, w int) string {
	t := a.theme
	events := seshes(a)
	if len(events) == 0 {
		return t.Dim.Render("no sessions logged yet — s records one")
	}
	total := 0.0
	perDay := map[string]float64{}
	for _, e := range events {
		total += e.Grams
		perDay[e.OccurredAt.Format(time.DateOnly)] += e.Grams
	}
	last := events[len(events)-1]
	lastLine := fmt.Sprintf("last: %.2f g %s", last.Grams, truncate(a.data.ProductName(last.Product), 20))
	if last.Device != "" {
		lastLine += " · " + deviceName(a, last.Device)
	}

	values := make([]float64, 0, 14)
	for i := 13; i >= 0; i-- {
		values = append(values, perDay[a.data.Now.AddDate(0, 0, -i).Format(time.DateOnly)])
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		t.Big.Render(fmt.Sprintf("%d", len(events)))+
			t.Dim.Render(fmt.Sprintf("  %s · %.1f g through, %.2f g each",
				nounOnly(len(events), "session"), total, total/float64(len(events)))),
		lipgloss.NewStyle().Foreground(t.SeshC).Render(Sparkline(values, min(w, 28), lipgloss.NewStyle()))+
			t.Dim.Render("  last 14 days"),
		t.Dim.Render(lastLine),
	)
}

// devicesCard is the devices at a glance: which one does the work, and how hot.
func (d dashboard) devicesCard(a *App, w int) string {
	t := a.theme
	devices := a.deviceList()
	if len(devices) == 0 {
		return t.Dim.Render("no devices yet — a adds one on the devices screen")
	}
	names, byDev := usageByDevice(seshes(a))
	lines := []string{t.Big.Render(fmt.Sprintf("%d", len(devices))) +
		t.Dim.Render("  "+plural(len(devices), "device")[2:]+" on the shelf")}
	shown := 0
	for _, n := range names {
		if n == "no device" || shown == 2 {
			continue
		}
		u := byDev[n]
		note := fmt.Sprintf("%.2f g · %s", u.grams, plural(u.count, "session"))
		if u.temps > 0 {
			note += fmt.Sprintf(" · avg %d°C", u.tSum/u.temps)
		}
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
			t.Value.Width(min(w/2, 20)).Render(truncate(deviceName(a, n), min(w/2, 20))),
			t.Dim.Render(" "+note)))
		shown++
	}
	if shown == 0 {
		lines = append(lines, t.Dim.Render("none used in a session yet"))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// nounOnly is plural without the number, for a card that has already said it
// big.
func nounOnly(n int, noun string) string {
	return strings.TrimLeft(plural(n, noun), "0123456789 ")
}

// deviceName resolves a device slug to its display name.
func deviceName(a *App, slug string) string {
	if a.data.Devices != nil {
		if d, err := a.data.Devices.Find(slug); err == nil {
			return d.Name
		}
	}
	return slug
}

// projectionCard draws the cycle's storage as it declined, and where it is
// heading: the braille area is what was, the line is what will be at the
// observed rate, falling to the day the jar runs out.
func (d dashboard) projectionCard(a *App, c *ledger.Cycle, w int) string {
	t := a.theme
	hist, proj, emptyAt := storageProjection(a, c)
	if len(hist) == 0 {
		return t.Dim.Render("nothing ground yet — the projection starts with the first grind")
	}
	chart := AreaChart(hist, proj, w, 5, t, t.StorageC, t.Warn)
	caption := t.Dim.Render("no rate to project from yet")
	if !emptyAt.IsZero() {
		caption = t.Dim.Render("empty around ") +
			lipgloss.NewStyle().Foreground(t.Warn).Render(emptyAt.Format("02 Jan")) +
			t.Dim.Render(" at the observed rate")
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		chart,
		axisLabels(t, c.Start.Format("02 Jan"), emptyDate(emptyAt, a.data.Now), w),
		caption,
	)
}

// emptyDate labels the right end of the projection: the projected empty day,
// or today when there is nothing to project.
func emptyDate(emptyAt time.Time, now time.Time) string {
	if emptyAt.IsZero() {
		return now.Format("02 Jan")
	}
	return emptyAt.Format("02 Jan")
}

// storageProjection builds the two series the projection card draws: the
// remaining grams per day so far, and the projected decline from today to
// empty. The history is padded with zeroes under the projection's tail, which
// the area chart reads as absence, so the future is line and no fill.
func storageProjection(a *App, c *ledger.Cycle) (hist, proj []float64, emptyAt time.Time) {
	perDay := map[string]float64{}
	for _, e := range c.Events {
		if e.Type == journal.Grind {
			perDay[e.OccurredAt.Format(time.DateOnly)] += e.Grams
		}
	}
	remaining := c.Held()
	for d := c.Start; !d.After(a.data.Now); d = d.AddDate(0, 0, 1) {
		remaining = ledger.Round(remaining - perDay[d.Format(time.DateOnly)])
		hist = append(hist, math.Max(remaining, 0))
	}

	rate := ledger.Summarise(c.Events).PerActiveDay
	if rate <= 0 || remaining <= 0 {
		return hist, nil, time.Time{}
	}
	days := min(int(math.Ceil(remaining/rate)), 90)
	proj = make([]float64, len(hist)+days)
	// Zeroes under the history keep the line out of the past; the area chart
	// draws no dot at zero.
	copy(proj, make([]float64, len(hist)-1))
	level := remaining
	proj[len(hist)-1] = level
	for i := 0; i < days; i++ {
		level = math.Max(level-rate, 0.01)
		proj[len(hist)+i] = level
	}
	hist = append(hist, make([]float64, days)...)
	return hist, proj, a.data.Now.AddDate(0, 0, days)
}

// rhythmCard is a small calendar of the last twelve weeks of one event type,
// so the habit is readable from the front page.
func rhythmCard(a *App, typ journal.Type, w int) string {
	t := a.theme
	perDay := map[string]float64{}
	logged := false
	for _, e := range a.data.State.Events {
		if e.Type == typ {
			perDay[e.OccurredAt.Format(time.DateOnly)] += e.Grams
			logged = true
		}
	}
	if !logged {
		return t.Dim.Render("nothing here yet")
	}
	from := a.data.Now.AddDate(0, 0, -12*7)
	return Calendar(perDay, from, a.data.Now, w, t)
}

// clip trims rendered content to a number of lines.
func clip(s string, lines int) string {
	if lines <= 0 {
		return ""
	}
	rows := strings.Split(s, "\n")
	if len(rows) <= lines {
		return s
	}
	return strings.Join(rows[:lines], "\n")
}
