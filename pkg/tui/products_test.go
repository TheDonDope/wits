package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/record"
)

// shelved returns an app holding two products, one of them used up.
func shelved(t *testing.T) *App {
	t.Helper()
	app := liveApp(t)
	rec := record.New(app.data.Repo, app.data.Products, app.data.Devices, app.data.State)

	_, _, _, err := rec.Buy("Cannamedical 28/1 Lemon Cookie", 10, time.Now().AddDate(0, 0, -2))
	require.NoError(t, err)
	_, err = rec.Grind("lemon", 10, time.Now().AddDate(0, 0, -1))
	require.NoError(t, err)
	_, err = rec.Session("lemon", 10, time.Now().AddDate(0, 0, -1), "", 0, "")
	require.NoError(t, err)
	_, err = rec.Grind("wedding", 2, time.Now())
	require.NoError(t, err)

	app.data, err = Load(app.data.Repo)
	require.NoError(t, err)
	return app
}

func TestProductsScreen(t *testing.T) {
	app := shelved(t)
	app.screen = productsScreen
	var m tea.Model = app

	out := stripANSI(m.View().Content)

	assert.Contains(t, out, "Enua 22/1 Wedding Cake", "Should name the product")
	assert.Contains(t, out, "22/1", "Should show its potency")
	assert.Contains(t, out, "18.00 g", "Should show what is in storage")
	assert.Contains(t, out, "2.00 g", "and what is in the tin")
	assert.Contains(t, out, "on the shelf", "Should say what it is listing")
}

func TestProductsListsOnlyWhatIsHeld(t *testing.T) {
	app := shelved(t)
	app.screen = productsScreen
	var m tea.Model = app

	held := stripANSI(m.View().Content)
	assert.NotContains(t, held, "Lemon Cookie",
		"A product with nothing left is history, not shelf")
	assert.Contains(t, held, "on the shelf · 1", "Should count only what is held")

	// `a` widens it to everything ever dispensed.
	m, _ = send(m, tea.KeyPressMsg{Code: 'a', Text: "a"})
	all := stripANSI(m.View().Content)
	assert.Contains(t, all, "Lemon Cookie", "Should bring the used-up product back")
	assert.Contains(t, all, "every product ever dispensed", "and say that is what it is showing")
}

func TestProductsGaugeAgreesWithItsCaption(t *testing.T) {
	app := shelved(t)
	app.screen = productsScreen

	rows := app.products.rows(app)
	require.NotEmpty(t, rows)
	r := rows[0]

	// The bar is filled from held-over-dispensed, which is what the text beside
	// it states. Filling it from grams ground would disagree with itself as soon
	// as anything is seshed or adjusted.
	assert.InDelta(t, 20.0, r.Bought, 0.001)
	assert.InDelta(t, 20.0, r.Held(), 0.001, "nothing has left the product yet, only moved tin-wards")
	assert.InDelta(t, 2.0, r.Ground, 0.001, "though 2 g has been ground")
}

func TestProductsCursorAndSelection(t *testing.T) {
	app := shelved(t)
	app.screen = productsScreen
	app.products.allTime = true
	var m tea.Model = app

	first := app.products.Selected(app)
	require.NotNil(t, first)

	m, _ = send(m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	second := app.products.Selected(app)
	require.NotNil(t, second)
	assert.NotEqual(t, first.Slug, second.Slug, "Should move to the next product")

	for i := 0; i < 8; i++ {
		m, _ = send(m, tea.KeyPressMsg{Code: 'k', Text: "k"})
	}
	assert.Equal(t, first.Slug, app.products.Selected(app).Slug, "Should stop at the top")
}

func TestProductsEmpty(t *testing.T) {
	app := New(emptyData(t))
	app.screen = productsScreen
	var m tea.Model = app
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 30})

	out := stripANSI(m.View().Content)

	assert.Contains(t, out, "Nothing on the shelf", "Should say so rather than show an empty table")
	assert.Contains(t, out, "wits buy", "and say what to do about it")
}

func TestWeighPicksTheFullestJarOffTheProductsScreen(t *testing.T) {
	app := shelved(t)
	app.screen = dashboardScreen

	// Away from the products screen there is no cursor, so it offers the jar
	// with the most left — the one most worth checking.
	slug := app.weighable()

	assert.Equal(t, "enua-wedding-cake", slug, "Should pick the fullest jar of the cycle")
}

func TestReconcileFromTheInterface(t *testing.T) {
	app := shelved(t)
	app.screen = productsScreen
	var m tea.Model = app

	before := app.data.State.Balances["enua-wedding-cake"].Storage
	require.InDelta(t, 18.0, before, 0.001)

	m, _ = send(m, tea.KeyPressMsg{Code: 'r', Text: "r"})
	require.NotNil(t, app.entry, "r should open the weighing form")

	// Storage is preselected; type what the scale says and submit.
	m, _ = send(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m = typeText(m, "17.6")
	_, msgs := confirmThrough(m, 6)

	done, ok := findDone(msgs)
	require.True(t, ok, "Should report the outcome, got %v", msgs)
	require.NoError(t, done.err)

	assert.Equal(t, journal.Adjust, done.event.Type, "Should record an adjustment")
	assert.InDelta(t, 0.40, done.event.Grams, 0.001, "of the difference, not the weight")
	assert.InDelta(t, 17.6, app.data.State.Balances["enua-wedding-cake"].Storage, 0.001,
		"and storage should now agree with the scale")
}

func TestDashboardShowsQuickActions(t *testing.T) {
	app := shelved(t)
	app.screen = dashboardScreen
	var m tea.Model = app

	out := stripANSI(m.View().Content)

	for _, want := range []string{"grind", "sesh", "fill", "weigh"} {
		assert.Contains(t, out, want, "the home view should offer %q without reading the help line", want)
	}
}

func TestQuickActionsFoldAwayWhenNarrow(t *testing.T) {
	app := shelved(t)
	app.screen = dashboardScreen
	var m tea.Model = app
	m, _ = m.Update(tea.WindowSizeMsg{Width: 40, Height: 24})

	out := stripANSI(m.View().Content)

	for _, line := range strings.Split(out, "\n") {
		// Display cells, not bytes: the box-drawing characters are three bytes each.
		assert.LessOrEqual(t, lipgloss.Width(line), 41,
			"the boxes should not push the frame sideways: %q", line)
	}
}
