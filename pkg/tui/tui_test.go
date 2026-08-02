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

// stripANSI removes escape sequences so assertions can be made on the text.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
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

func TestColumnChartIsBlankWhenNothingHappened(t *testing.T) {
	th := NewTheme(true)

	out := stripANSI(ColumnChart([]Column{{Value: 0}, {Value: 0}}, 4, th, th.Good))

	assert.Contains(t, out, "nothing logged", "Should say a range is empty rather than drawing a flat line")
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
	m, _ = m.Update(tea.KeyPressMsg{Code: 'g', Text: "g"})
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
