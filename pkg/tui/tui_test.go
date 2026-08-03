package tui

import (
	"io"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// TestMain handles global test setup
func TestMain(m *testing.M) {
	// Disable log output during tests
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// sample builds a small repository state to render against.
func sample(t *testing.T) Data {
	t.Helper()
	at := time.Date(2026, time.July, 9, 10, 0, 0, 0, time.UTC)
	mk := func(typ journal.Type, product string, grams float64, day int) journal.Event {
		from, to, _ := journal.Flow(typ)
		return journal.Event{
			Type: typ, Product: product, Grams: grams, From: from, To: to,
			OccurredAt: at.AddDate(0, 0, day),
		}
	}
	events := []journal.Event{
		mk(journal.Purchase, "wcake", 20, 0),
		mk(journal.Purchase, "lcook", 10, 0),
		mk(journal.Grind, "wcake", 0.75, 1),
		mk(journal.Grind, "lcook", 1.25, 1),
		mk(journal.Sesh, "wcake", 0.30, 2),
		mk(journal.Grind, "wcake", 1.10, 3),
	}
	// Slugs are set to match the entries above: a catalog whose slugs disagree
	// with the journal is not a fixture, it is a bug being tested.
	products := &catalog.Catalog{}
	require.NoError(t, products.Add(product("Enua 22/1 Wedding Cake", "wcake")))
	require.NoError(t, products.Add(product("Cannamedical 28/1 Lemon Cookie", "lcook")))

	return Data{
		Workspace: &workspace.Workspace{
			Products: products,
			Devices:  &catalog.Devices{},
			State:    ledger.Fold(events),
		},
		Now: at.AddDate(0, 0, 4),
	}
}

// render drives the app to a size and returns the plain text of a screen.
func render(t *testing.T, data Data, s screen, w, h int) string {
	t.Helper()
	app := New(data)
	app.screen = s
	var m tea.Model = app
	m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return stripANSI(m.View().Content)
}

func TestDashboard(t *testing.T) {
	out := render(t, sample(t), dashboardScreen, 96, 44)

	assert.Contains(t, out, "Storage", "Should deal a storage card")
	assert.Contains(t, out, "26.90 g", "Should show grams remaining in the cycle")
	assert.Contains(t, out, "of 30 g", "Should show what the cycle started with")
	assert.Contains(t, out, "Stash", "Should deal a stash card")
	assert.Contains(t, out, "Enua 22/1 Wedding", "Should name products, not slugs, sized to the card")
	assert.Contains(t, out, "Sessions", "Should deal a sessions card")
	assert.Contains(t, out, "Devices", "Should deal a devices card")
	assert.Contains(t, out, "Supply projection", "Should project the decline")
	assert.Contains(t, out, "Rhythm", "Should deal the rhythm cards")
}

func TestJournalScreen(t *testing.T) {
	out := render(t, sample(t), journalScreen, 96, 30)

	assert.Contains(t, out, "ground", "Should read as a verb rather than an event type")
	assert.Contains(t, out, "sesh", "Should show sessions")
	assert.Contains(t, out, "Enua 22/1 Wedding Cake", "Should name products")
	assert.Contains(t, out, "all entries · 6", "Should count entries, not the day headings between them")
}

func TestAnalysisScreen(t *testing.T) {
	out := render(t, sample(t), analysisScreen, 96, 30)

	assert.Contains(t, out, "Showing this cycle", "Should say what is being shown")
	assert.Contains(t, out, "GROUND", "Should summarise the scope")
	assert.Contains(t, out, "By product", "Should break the total down")
}

func TestNavigation(t *testing.T) {
	app := New(sample(t))
	var m tea.Model = app
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 30})

	m, _ = m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	assert.Equal(t, journalScreen, app.screen, "Should move to the next tab")

	// On the journal, the horizontal keys belong to the cover slider; only
	// tab and shift+tab change screens from there.
	m, _ = m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	assert.Equal(t, journalScreen, app.screen, "h should slide the cover, not leave the journal")

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	assert.Equal(t, dashboardScreen, app.screen, "Should move back on shift+tab")

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	require.NotNil(t, cmd)
	assert.IsType(t, tea.QuitMsg{}, cmd(), "Should quit on q")
}

func TestRendersAtAwkwardSizes(t *testing.T) {
	data := sample(t)
	for _, size := range [][2]int{{40, 10}, {60, 12}, {200, 60}, {96, 5}} {
		for _, s := range []screen{dashboardScreen, journalScreen, analysisScreen} {
			out := render(t, data, s, size[0], size[1])
			assert.NotEmpty(t, out, "Should render at %dx%d", size[0], size[1])
			for _, line := range strings.Split(out, "\n") {
				assert.LessOrEqual(t, lipgloss.Width(line), size[0]+1,
					"a line should not run past the terminal width at %dx%d: %q", size[0], size[1], line)
			}
		}
	}
}

func TestEmptyRepository(t *testing.T) {
	empty := Data{Workspace: &workspace.Workspace{
		Products: &catalog.Catalog{}, Devices: &catalog.Devices{}, State: ledger.Fold(nil),
	}, Now: time.Now()}

	out := render(t, empty, dashboardScreen, 96, 30)

	assert.Contains(t, out, "No cycle yet", "Should say so rather than showing zeroes")
	assert.Contains(t, out, "wits buy", "Should say what to do about it")
}

func TestGauge(t *testing.T) {
	th := NewTheme(true)

	assert.Equal(t, 10, lipgloss.Width(Gauge(10, 0, th.Good, th.Dim)), "Should be full width when empty")
	assert.Equal(t, 10, lipgloss.Width(Gauge(10, 1, th.Good, th.Dim)), "Should be full width when full")
	assert.NotContains(t, stripANSI(Gauge(10, 1, th.Good, th.Dim)), "─", "Should have no track left when full")
	assert.Contains(t, stripANSI(Gauge(10, 0.5, th.Good, th.Dim)), "─", "Should show the remaining track")
}

func TestEventLineHidesAMeaninglessTime(t *testing.T) {
	th := NewTheme(true)
	midnight := journal.Event{Type: journal.Grind, Product: "x", Grams: 1,
		OccurredAt: time.Date(2026, time.July, 9, 0, 0, 0, 0, time.UTC)}
	evening := journal.Event{Type: journal.Grind, Product: "x", Grams: 1,
		OccurredAt: time.Date(2026, time.July, 9, 21, 30, 0, 0, time.UTC)}

	assert.NotContains(t, stripANSI(th.EventLine(midnight, "X", 80, false)), "00:00",
		"A backfilled entry has no time of day, and should not claim one")
	assert.Contains(t, stripANSI(th.EventLine(evening, "X", 80, false)), "21:30",
		"A real time should be shown")
}

// product parses a display name and pins its slug.
func product(name, slug string) *catalog.Product {
	p := catalog.Parse(name)
	p.Slug = slug
	return p
}

// emptyData is a repository with nothing in it.
func emptyData(t *testing.T) Data {
	t.Helper()
	return Data{Workspace: &workspace.Workspace{
		Products: &catalog.Catalog{}, Devices: &catalog.Devices{}, State: ledger.Fold(nil),
	}, Now: time.Now()}
}

func TestJournalCoverSlider(t *testing.T) {
	out := render(t, sample(t), journalScreen, 96, 34)

	assert.Contains(t, out, "of 6 · newer →", "Should say where the slider stands")
	assert.Contains(t, out, "sesh", "Should tell the selected entry in full")

	// Sliding left selects the older neighbour; the list cursor follows.
	app := New(sample(t))
	app.screen = journalScreen
	var m tea.Model = app
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 34})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyHome})
	before := app.journal.Selected()
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	after := app.journal.Selected()
	require.NotNil(t, before)
	require.NotNil(t, after)
	assert.True(t, after.OccurredAt.Before(before.OccurredAt),
		"Left should slide to the older entry")
}

func TestDashboardStorageCardListsProducts(t *testing.T) {
	out := render(t, sample(t), dashboardScreen, 96, 44)

	assert.Contains(t, out, "Enua 22/1 Wedding", "The storage card should bar each product")
	assert.Contains(t, out, "Cannamedical 28/1", "all of them")
}

// TestDashboardBillsTheFillNotTheShelf pins the storage card to the fill's
// own jars. It once summed every jar on the shelf into a headline over
// fill-plus-carry-over, so the total never matched the rows drawn under it.
func TestDashboardBillsTheFillNotTheShelf(t *testing.T) {
	at := time.Date(2026, time.July, 9, 10, 0, 0, 0, time.UTC)
	mk := func(typ journal.Type, product string, grams float64, day int) journal.Event {
		from, to, _ := journal.Flow(typ)
		return journal.Event{
			Type: typ, Product: product, Grams: grams, From: from, To: to,
			OccurredAt: at.AddDate(0, 0, day),
		}
	}
	events := []journal.Event{
		mk(journal.Purchase, "old", 10, -40),
		mk(journal.Grind, "old", 2, -35),
		mk(journal.Purchase, "wcake", 20, 0),
		mk(journal.Grind, "wcake", 1, 1),
	}
	products := &catalog.Catalog{}
	require.NoError(t, products.Add(product("Khiron 20/1 Old Strain", "old")))
	require.NoError(t, products.Add(product("Enua 22/1 Wedding Cake", "wcake")))
	data := Data{
		Workspace: &workspace.Workspace{
			Products: products,
			Devices:  &catalog.Devices{},
			State:    ledger.Fold(events),
		},
		Now: at.AddDate(0, 0, 2),
	}

	out := render(t, data, dashboardScreen, 110, 44)

	assert.Contains(t, out, "19.00 g", "The headline sums the jars the card lists")
	assert.Contains(t, out, "of 20 g", "over what the fill dispensed")
	assert.NotContains(t, out, "of 28 g", "not the whole shelf")
	assert.Contains(t, out, "+ 8.00 g in 1 older jar",
		"with the previous cycle's remainder on its own line")
}

func TestDashboardWallClockAndCycleStart(t *testing.T) {
	app := New(sample(t))
	app.screen = dashboardScreen
	app.wall = time.Date(2026, 8, 3, 14, 30, 5, 0, time.UTC)
	var m tea.Model = app
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 44})

	out := stripANSI(m.View().Content)

	assert.Contains(t, out, "Mon 03 Aug 2026 · 14:30:05",
		"The quick-action line should carry the ticking wall clock")
	assert.Contains(t, out, "(started: ",
		"The storage card should date the cycle it counts")
}

func TestAnalysisScopes(t *testing.T) {
	app := New(sample(t))
	app.screen = analysisScreen
	var m tea.Model = app
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 40})

	m, _ = m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	out := stripANSI(m.View().Content)
	assert.Contains(t, out, "last 30 days", "The second scope is the last month")

	m, _ = m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	out = stripANSI(m.View().Content)
	assert.Contains(t, out, "last 90 days", "and the third is the quarter")
}

func TestAnalysisAxes(t *testing.T) {
	out := render(t, sample(t), analysisScreen, 96, 40)

	assert.Contains(t, out, "11 Jul", "The date axis should carry more than its ends")
	assert.Contains(t, out, "2.0 ┤", "The y axis should say what the peak weighs")
	assert.Contains(t, out, "0.0 ┴", "and where zero is")
}

func TestAnalysisPlayback(t *testing.T) {
	app := New(sample(t))
	app.screen = analysisScreen
	var m tea.Model = app
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 40})

	out := stripANSI(m.View().Content)
	assert.Contains(t, out, "p to replay", "Should offer the replay when live")

	// p starts the replay from empty and schedules the first tick.
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	require.True(t, app.analysis.playing, "p should start playing")
	assert.Equal(t, 0, app.analysis.playhead, "from the empty ledger")
	require.NotNil(t, cmd, "and schedule the first tick")

	out = stripANSI(m.View().Content)
	assert.Contains(t, out, "0/6", "The transport should count the events")
	assert.Contains(t, out, "playing", "and say it is playing")

	// Ticks apply events one at a time.
	m, _ = m.Update(playTickMsg{})
	assert.Equal(t, 1, app.analysis.playhead, "A tick should apply one event")
	out = stripANSI(m.View().Content)
	assert.Contains(t, out, "picked up", "and tell the event it applied")

	// Stepping by hand pauses; stepping past the end settles on live.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	assert.False(t, app.analysis.playing, "A hand step should pause the replay")
	assert.Equal(t, 2, app.analysis.playhead)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	assert.Equal(t, 1, app.analysis.playhead, "Left should take an event back")
	for i := 0; i < 9; i++ {
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	}
	assert.Equal(t, -1, app.analysis.playhead, "Past the end the screen is live again")

	// The speed keys move along the scale.
	m, _ = m.Update(tea.KeyPressMsg{Code: '+', Text: "+"})
	assert.Equal(t, 3, app.analysis.speed, "Plus should speed the replay up")
	m, _ = m.Update(tea.KeyPressMsg{Code: '-', Text: "-"})
	assert.Equal(t, 2, app.analysis.speed, "Minus should slow it down")
}

func TestAnalysisByProductKeepsFullNames(t *testing.T) {
	app := seshed(t)
	app.screen = analysisScreen
	app.analysis.scope = 4 // all time
	var m tea.Model = app
	m, _ = m.Update(tea.WindowSizeMsg{Width: 110, Height: 44})

	out := stripANSI(m.View().Content)
	assert.Contains(t, out, "Cannamedical 28/1 Lemon Cookie",
		"By product must never shorten a name")
	assert.Contains(t, out, "g/day", "and should rate each product")
	assert.Contains(t, out, "%", "and give its share")
}

func TestSnapshot(t *testing.T) {
	shot, err := Snapshot(sample(t), "storage", 96, 30, nil)
	require.NoError(t, err)
	assert.Contains(t, shot, "Enua 22/1 Wedding Cake", "Should render the screen it was asked for")

	// The playback can be photographed mid-run, tick by tick.
	shot, err = Snapshot(sample(t), "analysis", 96, 40, []string{"p", "tick"})
	require.NoError(t, err)
	assert.Contains(t, shot, "1/6", "Should apply the presses before the picture")

	_, err = Snapshot(sample(t), "garage", 96, 30, nil)
	assert.ErrorContains(t, err, "no screen called", "Should name the screens that exist")

	_, err = Snapshot(sample(t), "journal", 96, 30, []string{"ctrl+alt+del"})
	assert.ErrorContains(t, err, "cannot press", "Should refuse a key it cannot spell")
}

func TestReplaySkipsAdjustments(t *testing.T) {
	// A record with a correction in the middle: purchase, grind, adjust,
	// grind. The adjustment must ride along silently.
	at := time.Date(2026, time.July, 9, 10, 0, 0, 0, time.UTC)
	mk := func(typ journal.Type, grams float64, day int) journal.Event {
		from, to, _ := journal.Flow(typ)
		return journal.Event{Type: typ, Product: "wcake", Grams: grams, From: from, To: to,
			OccurredAt: at.AddDate(0, 0, day)}
	}
	events := []journal.Event{
		mk(journal.Purchase, 20, 0),
		mk(journal.Grind, 1, 1),
		mk(journal.Adjust, 0.5, 2),
		mk(journal.Grind, 1, 3),
	}
	products := &catalog.Catalog{}
	require.NoError(t, products.Add(product("Enua 22/1 Wedding Cake", "wcake")))
	data := Data{Workspace: &workspace.Workspace{
		Products: products, Devices: &catalog.Devices{}, State: ledger.Fold(events),
	}, Now: at.AddDate(0, 0, 4)}

	app := New(data)
	app.screen = analysisScreen
	app.analysis.scope = 4 // all time
	var m tea.Model = app
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 40})

	m, _ = m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	out := stripANSI(m.View().Content)
	assert.Contains(t, out, "0/3", "The counter should not count the adjustment")

	// Two ticks: purchase, then grind.
	m, _ = m.Update(playTickMsg{})
	m, _ = m.Update(playTickMsg{})
	assert.Equal(t, 2, app.analysis.playhead, "Two ticks apply the first two events")
	out = stripANSI(m.View().Content)
	assert.Contains(t, out, "2/3", "still not counting the adjustment")
	assert.Contains(t, out, "ground", "and narrating the grind")
	assert.NotContains(t, out, "adjusted", "never the adjustment")

	m, _ = m.Update(playTickMsg{})
	assert.Equal(t, -1, app.analysis.playhead,
		"The next tick swallows the adjustment, reaches the end, and settles on live")

	// Stepping back from live passes over the adjustment the same way.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	assert.Equal(t, 2, app.analysis.playhead, "Left from live lands before the adjustment, not on it")
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	assert.Equal(t, 1, app.analysis.playhead, "and keeps walking real events")
}

func TestSeanceScreen(t *testing.T) {
	out := render(t, sample(t), seanceScreen, 110, 46)

	assert.Contains(t, out, "Summoning the whole ledger", "Should name the frame on the table")
	assert.Contains(t, out, "apparitions", "Should count what the frame holds")
	assert.Contains(t, out, "The shelf", "Should draw the storage jars")
	assert.Contains(t, out, "The stashes", "Should draw the stash tins")
	assert.Contains(t, out, "═╩═", "The card wears a figurine on its pedestal")
	assert.Contains(t, out, "Enua 22/1 Wedding Cake", "The product keeps its whole name")
	assert.Contains(t, out, "live · press p", "Should offer the replay while live")
}

func TestSeanceFlipAndSleep(t *testing.T) {
	app := New(sample(t))
	app.screen = seanceScreen
	var m tea.Model = app
	m, _ = m.Update(tea.WindowSizeMsg{Width: 110, Height: 46})

	m, _ = m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	out := stripANSI(m.View().Content)
	assert.Contains(t, out, "the record", "x should turn the card over")
	assert.Contains(t, out, "occurred", "and show when it happened")
	assert.Contains(t, out, "storage→stash", "and the accounts the grams moved between")

	m, _ = m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	out = stripANSI(m.View().Content)
	assert.NotContains(t, out, "the record", "x again should turn it back face up")

	m, _ = m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	out = stripANSI(m.View().Content)
	assert.Contains(t, out, "the ledger sleeps",
		"Before the first tick nothing has been summoned yet")
}

func TestSeanceFrames(t *testing.T) {
	app := New(sample(t))
	app.screen = seanceScreen
	var m tea.Model = app
	m, _ = m.Update(tea.WindowSizeMsg{Width: 110, Height: 46})

	m, _ = m.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})
	out := stripANSI(m.View().Content)
	assert.Contains(t, out, "cycle 1 of 1", "f should turn the frame to the first cycle")

	m, _ = m.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})
	out = stripANSI(m.View().Content)
	assert.Contains(t, out, "the whole ledger", "and back around to everything")

	// A window picked by hand: the first two days of the sample.
	from := time.Date(2026, time.July, 9, 0, 0, 0, 0, time.UTC)
	m, _ = m.Update(seanceDatesMsg{from: from, to: from.AddDate(0, 0, 2)})
	out = stripANSI(m.View().Content)
	assert.Contains(t, out, "09 Jul 2026 → 10 Jul 2026", "Should frame the window by its days")
	assert.Contains(t, out, "4 apparitions", "and hold only the events inside it")

	// The séance stakes a claim on the horizontal keys, so only tab moves on.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	assert.Equal(t, seanceScreen, app.screen, "Right steps the replay rather than the tab bar")
}

func TestSeanceWindowForm(t *testing.T) {
	app := New(sample(t))
	app.screen = seanceScreen
	var m tea.Model = app
	m, _ = m.Update(tea.WindowSizeMsg{Width: 110, Height: 46})

	m, _ = m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	require.NotNil(t, app.entry, "d should open the window form")
	assert.Equal(t, entrySeance, app.entry.kind)
	assert.Equal(t, "2026-07-09", app.entry.from, "prefilled with the ledger's first day")
	out := stripANSI(m.View().Content)
	assert.Contains(t, out, "Séance window")

	// The parser reads the shapes people write dates in, and refuses a window
	// that ends before it starts.
	f := &entryForm{kind: entrySeance, from: "2026-07-01", to: "31.07.2026"}
	from, to, err := f.window()
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC), from)
	assert.Equal(t, time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC), to,
		"The last day is inclusive, so the window runs to the close of it")

	f = &entryForm{kind: entrySeance, from: "2026-07-01", to: "2026-06-01"}
	_, _, err = f.window()
	assert.ErrorContains(t, err, "ends before it starts")

	assert.Error(t, validDay("soon"), "Should refuse a date it cannot read")
	assert.NoError(t, validDay("02 Jan 2026"), "and accept one written out")
}

func TestSeanceReplayGrowsTheShelf(t *testing.T) {
	app := New(sample(t))
	app.screen = seanceScreen
	var m tea.Model = app
	m, _ = m.Update(tea.WindowSizeMsg{Width: 110, Height: 46})

	m, _ = m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	out := stripANSI(m.View().Content)
	assert.Contains(t, out, "nothing on the shelf yet", "The shelf starts bare")

	m, _ = m.Update(playTickMsg{})
	out = stripANSI(m.View().Content)
	assert.Contains(t, out, "picked up", "The first tick deals the first purchase")
	assert.NotContains(t, out, "nothing on the shelf yet", "and its jar takes the shelf")
	assert.Contains(t, out, "nothing ground yet", "while the tins wait for the first grind")
}
