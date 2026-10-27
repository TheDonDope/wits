package tui

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rsc.io/qr"
)

// insights opens the Insights screen on the seeded repository, at a size
// the pages and a QR code fit.
func insights(t *testing.T, width, height int) (*App, tea.Model) {
	t.Helper()
	app := seshed(t)
	app.screen = insightsScreen
	var m tea.Model = app
	m, _ = m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return app, m
}

// TestInsightsPages: ←/→ steps through every page, each drawing what the
// command line prints for the same repository.
func TestInsightsPages(t *testing.T) {
	_, m := insights(t, 100, 40)
	right := tea.KeyPressMsg{Code: tea.KeyRight}
	want := []string{
		"a usual day",                    // Today
		"No purchase has a price yet",    // Costs: the seed has none
		"in wits",                        // Year
		"A year of grinding, day by day", // History
		"for 7 days at",                  // Trip
		"held, on the shelf",             // Carry
		"The map needs terpenes",         // Strains: the seed has none
	}
	for i, w := range want {
		out := stripANSI(m.View().Content)
		assert.Contains(t, out, w, "page %s", insightsPages[i])
		m, _ = send(m, right)
	}
	assert.Contains(t, stripANSI(m.View().Content), "a usual day", "past the last page is the first again")
}

// TestInsightsTripSteps: - and + change the days away, o draws oldest first.
func TestInsightsTripSteps(t *testing.T) {
	app, m := insights(t, 100, 40)
	app.insights.page = pageTrip
	m = typeText(m, "+++")
	out := stripANSI(m.View().Content)
	assert.Contains(t, out, "for 10 days at")
	m = typeText(m, "o")
	assert.Contains(t, stripANSI(m.View().Content), "oldest fill first")
}

// TestInsightsWrite: w writes the report on History, and signs a trip on
// Trip, its QR code drawn when the terminal has room for it.
func TestInsightsWrite(t *testing.T) {
	app, m := insights(t, 120, 70)
	t.Chdir(t.TempDir())

	app.insights.page = pageHistory
	m = typeText(m, "w")
	name := reportName(app.data.Now)
	pdf, err := os.ReadFile(name)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(pdf), "%PDF"), "the report is a PDF")
	assert.Contains(t, stripANSI(m.View().Content), "Wrote "+name)

	app.insights.page = pageTrip
	m = typeText(m, "w")
	out := stripANSI(m.View().Content)
	assert.Contains(t, out, "Scan it to check it", "the code drawn in place of the page")
	assert.Contains(t, out, "signed for the")
	_, err = os.Stat(app.data.Repo.SealKeyPath())
	require.NoError(t, err, "the sealing key is made the first time")
	matches, _ := os.ReadDir(".")
	assert.Len(t, matches, 2, "the report and the trip's QR code")

	m, _ = send(m, tea.KeyPressMsg{Code: tea.KeyRight})
	assert.NotContains(t, stripANSI(m.View().Content), "Scan it to check it", "turning the page puts the code away")
}

// TestTerminalQRReadsBack: the half blocks drawn are the code's modules, two
// rows a line, with the quiet zone around them; too small a terminal draws
// nothing rather than a code cut off.
func TestTerminalQRReadsBack(t *testing.T) {
	const text = "https://thedondope.github.io/wits/verify.html#a-statement"
	code, err := qr.Encode(text, qr.L)
	require.NoError(t, err)
	drawn := strings.Split(stripANSI(terminalQR(text, 200, 200)), "\n")
	require.Len(t, drawn, (code.Size+4+1)/2)
	dark := func(x, y int) bool {
		r := []rune(drawn[(y+2)/2])[x+2]
		if (y+2)%2 == 0 {
			return r == '█' || r == '▀'
		}
		return r == '█' || r == '▄'
	}
	for y := range code.Size {
		for x := range code.Size {
			require.Equal(t, code.Black(x, y), dark(x, y), "module %d,%d", x, y)
		}
	}
	assert.Empty(t, terminalQR(text, code.Size, 200), "no room for the quiet zone")
}

// TestDashboardSaysToday: the sessions card ends on the day so far.
func TestDashboardSaysToday(t *testing.T) {
	app := seshed(t)
	var m tea.Model = app
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 60})
	assert.Contains(t, stripANSI(m.View().Content), "today: ")
}
