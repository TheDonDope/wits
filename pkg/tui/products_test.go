package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/record"
	"github.com/TheDonDope/wits/pkg/repo"
)

// shelved returns an app holding two products, one of them used up.
func shelved(t *testing.T) *App {
	t.Helper()
	app := liveApp(t)
	rec := record.New(app.data.Repo, app.data.Products, app.data.Devices, app.data.State)

	_, _, _, err := rec.Buy("Cannamedical 28/1 Lemon Cookie", "", 10, time.Now().AddDate(0, 0, -2))
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

func TestStorageScreenShowsBothTables(t *testing.T) {
	app := shelved(t)
	app.screen = storageScreen
	var m tea.Model = app

	out := stripANSI(m.View().Content)

	assert.Contains(t, out, "Storage", "Should title the shelf table")
	assert.Contains(t, out, "History", "Should title the history table")
	assert.Contains(t, out, "Enua 22/1 Wedding Cake", "Should name the product in full")
	assert.Contains(t, out, "22/1", "Should show its potency")
	assert.Contains(t, out, "18.00 g", "Should show what is in storage")
	assert.Contains(t, out, "2.00 g", "and what is in the stash")
	assert.Contains(t, out, "Cannamedical 28/1 Lemon Cookie",
		"A used-up product should stand in the history, not vanish")
	assert.Contains(t, out, "on the shelf · 1   history · 1", "Should count both tables")
}

func TestStorageDoesNotAbbreviateNames(t *testing.T) {
	app := liveApp(t)
	rec := record.New(app.data.Repo, app.data.Products, app.data.Devices, app.data.State)
	long := "Four Twenty Evolution CA Ice Cream Cake Especially Long 27/1"
	_, _, _, err := rec.Buy(long, "", 10, time.Now())
	require.NoError(t, err)
	app.data, err = Load(app.data.Repo)
	require.NoError(t, err)

	app.screen = storageScreen
	var m tea.Model = app
	m, _ = m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})

	out := stripANSI(m.View().Content)
	assert.Contains(t, out, long, "The full name is what the jar is called, and the column must hold it")
}

func TestStorageCursorWalksIntoHistory(t *testing.T) {
	app := shelved(t)
	app.screen = storageScreen
	var m tea.Model = app

	first := app.storage.Selected(app)
	require.NotNil(t, first)
	assert.False(t, first.gone(), "The cursor should start on the shelf")

	m, _ = send(m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	second := app.storage.Selected(app)
	require.NotNil(t, second)
	assert.True(t, second.gone(), "Should walk from the shelf into the history")

	for i := 0; i < 8; i++ {
		m, _ = send(m, tea.KeyPressMsg{Code: 'k', Text: "k"})
	}
	assert.Equal(t, first.Slug, app.storage.Selected(app).Slug, "Should stop at the top")
}

func TestStorageMarksJars(t *testing.T) {
	app := shelved(t)
	app.screen = storageScreen
	var m tea.Model = app

	m, _ = send(m, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	assert.Len(t, app.storage.Marked(app), 1, "Space should tick the jar under the cursor")

	out := stripANSI(m.View().Content)
	assert.Contains(t, out, "☑", "A ticked jar should look ticked")
	assert.Contains(t, out, "1 jar marked", "and the status line should count it")

	m, _ = send(m, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	assert.Empty(t, app.storage.Marked(app), "Space again should untick it")
}

func TestWeighMarkedJarsTogether(t *testing.T) {
	app := shelved(t)
	app.screen = storageScreen
	var m tea.Model = app

	// Tick the shelf jar and the history jar, then weigh.
	m, _ = send(m, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	m, _ = send(m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	m, _ = send(m, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	require.Len(t, app.storage.Marked(app), 2)

	m, _ = send(m, tea.KeyPressMsg{Code: 'r', Text: "r"})
	require.NotNil(t, app.entry, "r should open the weighing form for the marked jars")
	assert.Equal(t, entryWeighMany, app.entry.kind)
	assert.Contains(t, stripANSI(app.entry.View(app, 92)), "Weighing 2 jars", "Should say how many")
}

func TestCommitManyRecordsTheReadings(t *testing.T) {
	app := shelved(t)

	f := &entryForm{
		kind:     entryWeighMany,
		slugs:    []string{"wcake", "lemon"},
		readings: []string{"17.6", ""},
		account:  string(journal.Storage),
	}
	summary, err := f.commitMany(app)

	require.NoError(t, err)
	assert.Contains(t, summary, "adjusted 1 jar", "Should count the recorded adjustment")
	assert.Contains(t, summary, "1 skipped", "and the blank reading")

	reloaded, err := Load(app.data.Repo)
	require.NoError(t, err)
	assert.InDelta(t, 17.6, reloaded.State.Balances["wcake"].Storage, 0.001,
		"Storage should agree with the scale afterwards")
}

func TestCommitManyCountsAMatchAsAMatch(t *testing.T) {
	app := shelved(t)

	f := &entryForm{
		kind:     entryWeighMany,
		slugs:    []string{"wcake"},
		readings: []string{"18"},
		account:  string(journal.Storage),
	}
	summary, err := f.commitMany(app)

	require.NoError(t, err)
	assert.Contains(t, summary, "already matched", "A scale agreeing with the ledger is not an error")
}

// staleApp returns an app with a finished cycle whose stash was never worked
// down — the shape four imported years of grind-only records leave behind —
// plus a fresh cycle in progress. The old jar is recorded before the new fill,
// the way it happened, because cycles fold in journal order.
func staleApp(t *testing.T) *App {
	t.Helper()
	r, err := repo.Init(t.TempDir())
	require.NoError(t, err)
	data, err := Load(r)
	require.NoError(t, err)
	rec := record.New(r, data.Products, data.Devices, data.State)

	_, _, _, err = rec.Buy("Cannamedical 28/1 Lemon Cookie", "lemon", 10, time.Now().AddDate(0, 0, -40))
	require.NoError(t, err)
	_, err = rec.Grind("lemon", 10, time.Now().AddDate(0, 0, -39))
	require.NoError(t, err)
	_, err = rec.Session("lemon", 7, time.Now().AddDate(0, 0, -38), "", 0, "")
	require.NoError(t, err)
	_, _, _, err = rec.Buy("Enua 22/1 Wedding Cake", "wcake", 20, time.Now())
	require.NoError(t, err)

	data, err = Load(r)
	require.NoError(t, err)
	app := New(data)
	app.width, app.height = 96, 30
	return app
}

func TestStaleStashes(t *testing.T) {
	app := staleApp(t)

	stale := staleStashes(app)

	require.Len(t, stale, 1, "Should find the finished jar with a remainder")
	assert.Equal(t, "lemon", stale[0], "The old cycle's stash is stale")
	assert.NotContains(t, stale, "wcake", "The cycle in progress is being worked, not stale")
}

func TestCleanHistoryOffersAndRecords(t *testing.T) {
	app := staleApp(t)
	app.screen = storageScreen
	var m tea.Model = app

	out := stripANSI(m.View().Content)
	assert.Contains(t, out, "stash remainder from earlier cycles",
		"Should offer the clean-up where the history is")

	m, _ = send(m, tea.KeyPressMsg{Code: 'c', Text: "c"})
	require.NotNil(t, app.entry, "c should open the confirmation")
	assert.Equal(t, entryCleanHistory, app.entry.kind)
	assert.Contains(t, stripANSI(app.entry.View(app, 92)), "3.00 g",
		"Should say how much the clean-up records as consumed")

	// The commit itself, confirmed.
	app.entry.confirm = true
	summary, err := app.entry.commitClean(app)
	require.NoError(t, err)
	assert.Contains(t, summary, "cleaned 1 stash", "Should count what it cleaned")
	assert.Contains(t, summary, "3.00 g", "and the grams it recorded as consumed")

	reloaded, err := Load(app.data.Repo)
	require.NoError(t, err)
	assert.Zero(t, reloaded.State.Balances["lemon"].Stash, "The stash should be zero afterwards")
}

// TestCleanHistoryManyJars is the imported-workbook shape: dozens of stale
// stashes. The dialog must keep its confirmation on screen — listing every
// jar once pushed it below the fold, where enter answered with the default
// Keep and the clean-up looked like it did nothing.
func TestCleanHistoryManyJars(t *testing.T) {
	r, err := repo.Init(t.TempDir())
	require.NoError(t, err)
	data, err := Load(r)
	require.NoError(t, err)
	rec := record.New(r, data.Products, data.Devices, data.State)

	for i := 0; i < 40; i++ {
		slug := fmt.Sprintf("old%02d", i)
		_, _, _, err = rec.Buy(fmt.Sprintf("Maker %d/1 Old Strain %d", 20+i%10, i), slug, 10, time.Now().AddDate(0, 0, -400+i*2))
		require.NoError(t, err)
		_, err = rec.Grind(slug, 10, time.Now().AddDate(0, 0, -399+i*2))
		require.NoError(t, err)
	}
	_, _, _, err = rec.Buy("Enua 22/1 Wedding Cake", "wcake", 20, time.Now())
	require.NoError(t, err)

	data, err = Load(r)
	require.NoError(t, err)
	app := New(data)
	app.screen = storageScreen
	app.width, app.height = 120, 40
	var m tea.Model = app

	m, _ = send(m, tea.KeyPressMsg{Code: 'c', Text: "c"})
	require.NotNil(t, app.entry, "c should open the confirmation")

	view := stripANSI(app.entry.View(app, 118))
	lines := strings.Count(view, "\n") + 1
	assert.LessOrEqual(t, lines, app.height-2, "The dialog must fit under the tab bar")
	assert.Contains(t, view, "Record them as consumed?", "with the question on screen")
	assert.Contains(t, view, "more,", "and the jars that did not fit folded into one line")
	assert.Contains(t, view, "400.00 g", "counting every jar in the total, listed or folded")

	// Choose Clean and submit, the way a user would.
	m, _ = send(m, tea.KeyPressMsg{Code: tea.KeyLeft})
	_, msgs := send(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	done, ok := findDone(msgs)
	require.True(t, ok, "enter should submit the confirmation")
	require.NoError(t, done.err)
	assert.Contains(t, done.summary, "cleaned 40 stashes")

	reloaded, err := Load(r)
	require.NoError(t, err)
	assert.Zero(t, reloaded.State.Balances["old17"].Stash, "The stale stashes are zeroed")
}

func TestCleanHistoryDeclined(t *testing.T) {
	app := staleApp(t)

	f := newCleanHistoryForm(staleStashes(app), app)
	f.confirm = false
	_, err := f.commitClean(app)

	assert.ErrorIs(t, err, errCancelled, "Keeping the remainder should write nothing")

	reloaded, err2 := Load(app.data.Repo)
	require.NoError(t, err2)
	assert.InDelta(t, 3.0, reloaded.State.Balances["lemon"].Stash, 0.001, "and the stash should be untouched")
}

func TestCleanHistoryWithNothingStale(t *testing.T) {
	app := shelved(t)
	app.screen = storageScreen
	var m tea.Model = app

	m, _ = send(m, tea.KeyPressMsg{Code: 'c', Text: "c"})

	assert.Nil(t, app.entry, "Should not open a form over nothing")
	assert.Contains(t, app.notice, "no stale stashes", "Should say why")
}

func TestStorageEmpty(t *testing.T) {
	app := New(emptyData(t))
	app.screen = storageScreen
	var m tea.Model = app
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 30})

	out := stripANSI(m.View().Content)

	assert.Contains(t, out, "Nothing in storage yet", "Should say so rather than show empty tables")
	assert.Contains(t, out, "wits buy", "and say what to do about it")
}

func TestWeighPicksTheFullestJarOffTheStorageScreen(t *testing.T) {
	app := shelved(t)
	app.screen = dashboardScreen

	// Away from the storage screen there is no cursor, so it offers the jar
	// with the most left — the one most worth checking.
	slug := app.weighable()

	assert.Equal(t, "wcake", slug, "Should pick the fullest jar of the cycle")
}

func TestReconcileFromTheInterface(t *testing.T) {
	app := shelved(t)
	app.screen = storageScreen
	var m tea.Model = app

	before := app.data.State.Balances["wcake"].Storage
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
	assert.InDelta(t, 17.6, app.data.State.Balances["wcake"].Storage, 0.001,
		"and storage should now agree with the scale")
}

func TestDashboardShowsQuickActions(t *testing.T) {
	app := shelved(t)
	app.screen = dashboardScreen
	var m tea.Model = app

	out := stripANSI(m.View().Content)

	for _, want := range []string{"buy", "grind", "sesh", "weigh"} {
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
		assert.LessOrEqual(t, lipgloss.Width(line), 41,
			"the boxes should not push the frame sideways: %q", line)
	}
}

func TestEditProductFromTheScreen(t *testing.T) {
	app := shelved(t)
	app.screen = storageScreen
	var m tea.Model = app

	m, _ = send(m, tea.KeyPressMsg{Code: 'e', Text: "e"})
	require.NotNil(t, app.entry, "e should open the edit form")
	assert.Contains(t, stripANSI(app.entry.View(app, 92)), "Edit product", "Should say what it is")
	assert.Contains(t, stripANSI(app.entry.View(app, 92)), "slug stays as it is",
		"and why the slug is not on offer")

	// The name is prefilled; appending to it is enough to change it.
	m = typeText(m, " II")
	_, msgs := confirmThrough(m, 10)

	done, ok := findDone(msgs)
	require.True(t, ok, "Should report the outcome, got %v", msgs)
	require.NoError(t, done.err)

	p, err := app.data.Products.Find("wcake")
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(p.Name, " II"), "Should have written the corrected name, got %q", p.Name)
	assert.Equal(t, "wcake", p.Slug, "and left the slug the journal refers to alone")
}

func TestEditingAProductDoesNotOrphanItsEntries(t *testing.T) {
	app := shelved(t)
	before := len(app.data.State.Events)

	rec := record.New(app.data.Repo, app.data.Products, app.data.Devices, app.data.State)
	_, err := rec.Rename("wcake", "Renamed Entirely")
	require.NoError(t, err)

	reloaded, err := Load(app.data.Repo)
	require.NoError(t, err)

	assert.Len(t, reloaded.State.Events, before, "Renaming records nothing and loses nothing")
	assert.Equal(t, "Renamed Entirely", reloaded.ProductName("wcake"),
		"and the entries still resolve to the product, under its new name")
}

// seshed returns an app with one finished stash, one holding, and sessions
// through two devices — enough for the stash and sessions screens to have
// something true to say.
func seshed(t *testing.T) *App {
	t.Helper()
	r, err := repo.Init(t.TempDir())
	require.NoError(t, err)
	data, err := Load(r)
	require.NoError(t, err)
	rec := record.New(r, data.Products, data.Devices, data.State)

	require.NoError(t, data.Devices.Add(&catalog.Device{Name: "Volcano Hybrid", MaxTemp: 230, DefaultTemp: 185}))
	require.NoError(t, data.Devices.Save(r.DevicesPath()))

	_, _, _, err = rec.Buy("Cannamedical 28/1 Lemon Cookie", "lemon", 10, day10(-10))
	require.NoError(t, err)
	_, err = rec.Grind("lemon", 3, day10(-9))
	require.NoError(t, err)
	_, err = rec.Session("lemon", 2, day10(-8), "volcano", 190, "")
	require.NoError(t, err)
	_, err = rec.Session("lemon", 1, day10(-7), "volcano", 0, "")
	require.NoError(t, err)

	_, _, _, err = rec.Buy("Enua 22/1 Wedding Cake", "wcake", 20, day10(0))
	require.NoError(t, err)
	_, err = rec.Grind("wcake", 2, day10(0))
	require.NoError(t, err)
	_, err = rec.Session("wcake", 0.5, day10(0), "", 0, "")
	require.NoError(t, err)

	data, err = Load(r)
	require.NoError(t, err)
	app := New(data)
	app.width, app.height = 96, 30
	return app
}

// day10 is a stable timestamp n days from now, at ten in the morning.
func day10(n int) time.Time {
	return time.Now().AddDate(0, 0, n).Truncate(24 * time.Hour).Add(10 * time.Hour)
}

func TestStashHistory(t *testing.T) {
	app := seshed(t)

	stats := stashHistoryOf(app.data.State.Events)

	lemon := stats["lemon"]
	require.NotNil(t, lemon)
	assert.InDelta(t, 3.0, lemon.Through, 0.001, "Should total everything ground into the stash")
	assert.Equal(t, 2, lemon.Sessions, "Should count the sessions drawn from it")
	assert.False(t, lemon.EmptiedAt.IsZero(), "A stash worked to zero should know when it ended")

	wcake := stats["wcake"]
	require.NotNil(t, wcake)
	assert.True(t, wcake.EmptiedAt.IsZero(), "A stash still holding something has not ended")
}

func TestStashHistoryRefillForgetsTheOldEnding(t *testing.T) {
	app := seshed(t)
	rec := record.New(app.data.Repo, app.data.Products, app.data.Devices, app.data.State)

	// The lemon stash ended; grinding into it again reopens the story.
	_, err := rec.Grind("lemon", 1, day10(0))
	require.NoError(t, err)
	var err2 error
	app.data, err2 = Load(app.data.Repo)
	require.NoError(t, err2)

	stats := stashHistoryOf(app.data.State.Events)
	assert.True(t, stats["lemon"].EmptiedAt.IsZero(), "A refilled stash is active again, not history")
}

func TestStashScreenShowsBothTables(t *testing.T) {
	app := seshed(t)
	app.screen = stashScreen
	var m tea.Model = app

	out := stripANSI(m.View().Content)

	assert.Contains(t, out, "Stash", "Should title the active table")
	assert.Contains(t, out, "Consumed", "Should title the finished table")
	assert.Contains(t, out, "Enua 22/1 Wedding Cake", "The holding stash sits above")
	assert.Contains(t, out, "Cannamedical 28/1 Lemon Cookie", "The finished stash sits below")
	assert.Contains(t, out, "holding · 1   finished · 1", "Should count both tables")
	assert.Contains(t, out, "2 sessions", "Should say how many sessions finished it")

	// The finished stash is grouped under the day it was consumed.
	ended := stashHistoryOf(app.data.State.Events)["lemon"].EmptiedAt.Format("Mon 02 Jan 2006")
	assert.Contains(t, out, ended, "Should head the group with the day it was consumed")
}

func TestStashMarkAndWeigh(t *testing.T) {
	app := seshed(t)
	app.screen = stashScreen
	var m tea.Model = app

	m, _ = send(m, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	require.Len(t, app.stash.Marked(app), 1, "Space should tick the stash under the cursor")

	m, _ = send(m, tea.KeyPressMsg{Code: 'r', Text: "r"})
	require.NotNil(t, app.entry, "r should open the weighing form")
	assert.Equal(t, entryWeighMany, app.entry.kind)
	assert.Equal(t, string(journal.Stash), app.entry.account, "The stash screen weighs stashes first")
}

func TestSessionsScreenEmpty(t *testing.T) {
	app := liveApp(t)
	app.screen = sessionsScreen
	var m tea.Model = app

	out := stripANSI(m.View().Content)

	assert.Contains(t, out, "No sessions logged yet", "Should say so rather than draw empty charts")
	assert.Contains(t, out, "wits sesh", "and say how to start")
}

func TestSessionsScreenShowsTheStory(t *testing.T) {
	app := seshed(t)
	app.screen = sessionsScreen
	var m tea.Model = app
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	out := stripANSI(m.View().Content)

	assert.Contains(t, out, "SESSIONS", "Should count the sessions")
	assert.Contains(t, out, "3", "all three of them")
	assert.Contains(t, out, "SESHED", "Should total the grams drawn")
	assert.Contains(t, out, "By device", "Should break usage down by device")
	assert.Contains(t, out, "Volcano Hybrid", "naming the device properly")
	assert.Contains(t, out, "avg 187°C",
		"with the temperatures it ran at, the device default filled in where none was set")
	assert.Contains(t, out, "no device", "and owning the sessions that had none")
	assert.Contains(t, out, "Rhythm", "Should draw the calendar")
}

func TestStorageReplay(t *testing.T) {
	app := seshed(t)
	app.screen = storageScreen
	var m tea.Model = app

	out := stripANSI(m.View().Content)
	assert.Contains(t, out, "p to replay", "Should offer the replay when live")

	// p starts from the empty ledger: no jars yet.
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	require.True(t, app.storage.playing, "p should start playing")
	require.NotNil(t, cmd, "and schedule a tick")
	out = stripANSI(m.View().Content)
	assert.Contains(t, out, "0/7", "The transport should count the whole ledger")
	assert.Contains(t, out, "nothing on the shelf", "and the shelf should start empty")

	// One purchase in, the first jar appears.
	m, _ = m.Update(playTickMsg{})
	out = stripANSI(m.View().Content)
	assert.Contains(t, out, "Cannamedical 28/1 Lemon Cookie", "The first fill should appear on the shelf")
	assert.Contains(t, out, "picked up", "and the transport should narrate it")

	// Stepping past the end settles on live.
	for i := 0; i < 8; i++ {
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	}
	assert.Equal(t, -1, app.storage.playhead, "Past the end the screen is live again")
}

func TestStashReplay(t *testing.T) {
	app := seshed(t)
	app.screen = stashScreen
	var m tea.Model = app

	_, _ = m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	require.True(t, app.stash.playing)

	// Play until the lemon stash has been ground but not yet worked down: it
	// stands in the active table, not the history.
	for i := 0; i < 2; i++ {
		m, _ = m.Update(playTickMsg{})
	}
	out := stripANSI(m.View().Content)
	assert.Contains(t, out, "holding · 1", "The ground stash should be active mid-replay")
	assert.Contains(t, out, "finished · 0", "and nothing finished yet")

	// Two sessions later it is consumed and moves to the history.
	for i := 0; i < 2; i++ {
		m, _ = m.Update(playTickMsg{})
	}
	out = stripANSI(m.View().Content)
	assert.Contains(t, out, "finished · 1", "The emptied stash should move into the history as it empties")
}
