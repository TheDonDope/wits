package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits-tui/pkg/catalog"
	"github.com/TheDonDope/wits-tui/pkg/journal"
	"github.com/TheDonDope/wits-tui/pkg/ledger"
	"github.com/TheDonDope/wits-tui/pkg/record"
	"github.com/TheDonDope/wits-tui/pkg/repo"
)

// liveApp returns an app backed by a real repository on disk, so that entries
// made through a form actually land in a journal.
func liveApp(t *testing.T) *App {
	t.Helper()
	r, err := repo.Init(t.TempDir())
	require.NoError(t, err)

	products := &catalog.Catalog{}
	require.NoError(t, products.Add(catalog.Parse("Enua 22/1 Wedding Cake")))
	require.NoError(t, products.Save(r.ProductsPath()))

	_, err = r.Journal().Append(journal.Event{
		Type: journal.Purchase, Product: "enua-wedding-cake", Grams: 20,
		From: journal.External, To: journal.Storage, OccurredAt: time.Now(),
	})
	require.NoError(t, err)

	data, err := Load(r)
	require.NoError(t, err)
	app := New(data)
	var m tea.Model = app
	m.Update(tea.WindowSizeMsg{Width: 96, Height: 30})
	app.width, app.height = 96, 30
	return app
}

// send delivers a message and then drains whatever commands it produced, the
// way the runtime does. Without this a form never advances past its first
// field, because huh moves between fields by returning a command.
func send(m tea.Model, msg tea.Msg) (tea.Model, []tea.Msg) {
	var produced []tea.Msg
	queue := []tea.Msg{msg}
	for len(queue) > 0 && len(produced) < 64 {
		next := queue[0]
		queue = queue[1:]
		var cmd tea.Cmd
		m, cmd = m.Update(next)
		if cmd == nil {
			continue
		}
		out, ok := runCmd(cmd)
		if !ok || out == nil {
			continue
		}
		// Record it for the test to assert on, and still deliver it: an outcome
		// message is what makes the app reload, so swallowing it here would
		// leave the assertions looking at stale data.
		produced = append(produced, out)
		queue = append(queue, out)
	}
	return m, produced
}

// typeText sends each character of s to the model as a key press.
func typeText(m tea.Model, s string) tea.Model {
	for _, r := range s {
		m, _ = send(m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	return m
}

// runCmd runs a command, giving up if it does not return promptly.
//
// Some commands are meant to block: a cursor blink waits on a timer, and
// running one on the test goroutine would hang forever. Those are exactly the
// ones a test has no interest in.
func runCmd(cmd tea.Cmd) (tea.Msg, bool) {
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		return msg, true
	case <-time.After(50 * time.Millisecond):
		return nil, false
	}
}

// confirmThrough presses enter until the form reports an outcome, or gives up.
// A form has as many fields as it has, and a test should not have to know how
// many enters that is.
func confirmThrough(m tea.Model, presses int) (tea.Model, []tea.Msg) {
	var all []tea.Msg
	for i := 0; i < presses; i++ {
		var msgs []tea.Msg
		m, msgs = send(m, tea.KeyPressMsg{Code: tea.KeyEnter})
		all = append(all, msgs...)
		if _, done := findDone(all); done {
			break
		}
	}
	return m, all
}

// findDone returns the entry outcome among a batch of messages.
func findDone(msgs []tea.Msg) (entryDoneMsg, bool) {
	for _, m := range msgs {
		if d, ok := m.(entryDoneMsg); ok {
			return d, true
		}
	}
	return entryDoneMsg{}, false
}

func TestGrindForm(t *testing.T) {
	app := liveApp(t)
	var m tea.Model = app

	m, _ = send(m, tea.KeyPressMsg{Code: 'n', Text: "n"})
	require.NotNil(t, app.entry, "n should open the grind form")
	assert.Equal(t, entryGrind, app.entry.kind, "Should be a grind")

	// The product select is first; enter accepts it, then the amount is typed.
	m, _ = send(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m = typeText(m, "0.75")
	_, msgs := send(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	done, ok := findDone(msgs)
	require.True(t, ok, "Should report the entry, got %v", msgs)
	require.NoError(t, done.err)

	assert.Equal(t, journal.Grind, done.event.Type, "Should have recorded a grind")
	assert.Equal(t, 0.75, done.event.Grams, "Should have recorded the amount typed")
	assert.Nil(t, app.entry, "The form should close once it is done")

	// And it is really in the journal, not only in the message.
	events, err := app.data.Repo.Journal().Events()
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, journal.Grind, events[1].Type, "Should be appended to the journal")
}

func TestFormRefusesAnOverdraw(t *testing.T) {
	app := liveApp(t)
	var m tea.Model = app

	m, _ = send(m, tea.KeyPressMsg{Code: 'n', Text: "n"})
	m, _ = send(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m = typeText(m, "999")
	_, msgs := send(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	done, ok := findDone(msgs)
	require.True(t, ok, "Should report the outcome, got %v", msgs)

	assert.Error(t, done.err, "Should refuse to draw storage below zero, the same as the command does")
	events, err := app.data.Repo.Journal().Events()
	require.NoError(t, err)
	assert.Len(t, events, 1, "Should not have written anything")
}

func TestFormCancels(t *testing.T) {
	app := liveApp(t)
	var m tea.Model = app

	m, _ = send(m, tea.KeyPressMsg{Code: 'n', Text: "n"})
	require.NotNil(t, app.entry)

	send(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	assert.Nil(t, app.entry, "esc should close the form")
	events, err := app.data.Repo.Journal().Events()
	require.NoError(t, err)
	assert.Len(t, events, 1, "Should not have written anything")
}

func TestFormTakesTheKeyboard(t *testing.T) {
	app := liveApp(t)
	var m tea.Model = app
	m, _ = send(m, tea.KeyPressMsg{Code: 'n', Text: "n"})

	// While a form is open, keys that would otherwise navigate must not.
	before := app.screen
	send(m, tea.KeyPressMsg{Code: 'l', Text: "l"})

	assert.Equal(t, before, app.screen, "Navigation should not fire under an open form")
	assert.NotNil(t, app.entry, "and the form should still be open")
}

func TestProductOptionsOnlyOfferWhatIsThere(t *testing.T) {
	app := liveApp(t)

	storage := productOptions(app, journal.Storage)
	stash := productOptions(app, journal.Stash)

	assert.Len(t, storage, 1, "Should offer the product that has storage")
	assert.Empty(t, stash, "Should offer nothing from an empty tin, rather than a choice bound to be refused")
}

func TestNoticeAfterAnEntry(t *testing.T) {
	app := liveApp(t)
	var m tea.Model = app

	m.Update(entryDoneMsg{event: journal.Event{Type: journal.Grind, Grams: 0.75, Product: "enua-wedding-cake"}})

	assert.Contains(t, app.notice, "0.75", "Should confirm what was recorded")
	assert.False(t, app.failed, "Should not read as a failure")

	m.Update(entryDoneMsg{err: assertError{}})
	assert.True(t, app.failed, "Should mark a failure")
}

// assertError is a stand-in error.
type assertError struct{}

func (assertError) Error() string { return "nope" }

var _ = ledger.Fold

func TestUndoDefaultsToKeeping(t *testing.T) {
	app := liveApp(t)
	rec := record.New(app.data.Repo, app.data.Products, app.data.Devices, app.data.State)
	_, err := rec.Grind("wedding", 2, time.Now())
	require.NoError(t, err)
	app.data, err = Load(app.data.Repo)
	require.NoError(t, err)

	var m tea.Model = app
	app.screen = journalScreen
	app.journal.rows = app.journal.build(app)
	app.journal.cursor = app.journal.firstEntry(0, +1)
	m, _ = send(m, tea.KeyPressMsg{Code: 'd', Text: "d"})
	require.NotNil(t, app.entry)

	// Pressing through without choosing must not undo anything.
	_, msgs := confirmThrough(m, 6)
	done, ok := findDone(msgs)
	require.True(t, ok)
	assert.ErrorIs(t, done.err, errCancelled, "Should default to keeping the entry")

	events, err := app.data.Repo.Journal().Events()
	require.NoError(t, err)
	assert.Len(t, events, 2, "Should not have recorded a correction")
}

func TestUndoFromTheJournal(t *testing.T) {
	app := liveApp(t)
	var m tea.Model = app
	m, _ = send(m, tea.KeyPressMsg{Code: 'n', Text: "n"})
	m, _ = send(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m = typeText(m, "2")
	m, _ = send(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Equal(t, 2.0, app.data.State.Balances["enua-wedding-cake"].Stash)

	// Move to the journal, put the cursor on the grind, and undo it.
	m, _ = send(m, tea.KeyPressMsg{Code: 'l', Text: "l"})
	require.Equal(t, journalScreen, app.screen)
	app.journal.rows = app.journal.build(app)
	app.journal.cursor = app.journal.firstEntry(0, +1)
	require.NotNil(t, app.journal.Selected(), "the cursor should be on an entry")

	m, _ = send(m, tea.KeyPressMsg{Code: 'd', Text: "d"})
	require.NotNil(t, app.entry, "d should open the undo form")

	// The form opens on the confirm, since a note takes no focus. It defaults to
	// keeping the entry, which is the right default for something destructive,
	// so undoing has to be chosen.
	m, _ = send(m, tea.KeyPressMsg{Code: 'y', Text: "y"})
	_, msgs := confirmThrough(m, 6)

	done, ok := findDone(msgs)
	require.True(t, ok, "Should report the outcome, got %v", msgs)
	require.NoError(t, done.err)

	assert.Zero(t, app.data.State.Balances["enua-wedding-cake"].Stash, "The grams should be back in storage")
	events, err := app.data.Repo.Journal().Events()
	require.NoError(t, err)
	assert.Len(t, events, 3, "Nothing should have been removed: the correction is appended")
	assert.NotEmpty(t, events[2].Reverts, "and it should name the entry it undid")
}

func TestJournalHidesCorrectedEntries(t *testing.T) {
	app := liveApp(t)
	rec := record.New(app.data.Repo, app.data.Products, app.data.Devices, app.data.State)
	grind, err := rec.Grind("wedding", 2, time.Now())
	require.NoError(t, err)
	_, err = rec.Revert(grind.Hash, "")
	require.NoError(t, err)
	app.data, err = Load(app.data.Repo)
	require.NoError(t, err)

	v := newJournalView()
	assert.Equal(t, 1, countEvents(v.build(app)),
		"A corrected entry and its correction should both be out of the way, leaving the fill")

	v.showAll = true
	assert.Equal(t, 3, countEvents(v.build(app)), "and v should bring them back")
}

func TestJournalCursorSkipsHeadings(t *testing.T) {
	app := liveApp(t)
	rec := record.New(app.data.Repo, app.data.Products, app.data.Devices, app.data.State)
	_, err := rec.Grind("wedding", 1, time.Now().AddDate(0, 0, -1))
	require.NoError(t, err)
	app.data, err = Load(app.data.Repo)
	require.NoError(t, err)

	v := newJournalView()
	v.rows = v.build(app)
	v.cursor = v.firstEntry(0, +1)

	for i := 0; i < len(v.rows); i++ {
		require.False(t, v.rows[v.cursor].heading, "the cursor should never land on a day heading")
		v.cursor = v.step(+1)
	}
}

func TestDeviceForm(t *testing.T) {
	app := liveApp(t)
	var m tea.Model = app
	app.screen = devicesScreen

	m, _ = send(m, tea.KeyPressMsg{Code: 'a', Text: "a"})
	require.NotNil(t, app.device, "a should open the device form")

	m = typeText(m, "Volcano Hybrid")
	m, _ = send(m, tea.KeyPressMsg{Code: tea.KeyEnter}) // name -> kind
	m = typeText(m, "desktop")
	m, _ = send(m, tea.KeyPressMsg{Code: tea.KeyEnter}) // kind -> min
	m = typeText(m, "40")
	m, _ = send(m, tea.KeyPressMsg{Code: tea.KeyEnter}) // min -> max
	m = typeText(m, "230")
	m, _ = send(m, tea.KeyPressMsg{Code: tea.KeyEnter}) // max -> default
	m = typeText(m, "185")
	_, msgs := send(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	var saved bool
	for _, msg := range msgs {
		if d, ok := msg.(deviceDoneMsg); ok {
			require.NoError(t, d.err)
			saved = true
		}
	}
	require.True(t, saved, "Should have saved the device, got %v", msgs)

	devices, err := catalog.LoadDevices(app.data.Repo.DevicesPath())
	require.NoError(t, err)
	require.Len(t, devices.Devices, 1)
	assert.Equal(t, "Volcano Hybrid", devices.Devices[0].Name, "Should have written it to the catalog")
	assert.Equal(t, 230, devices.Devices[0].MaxTemp, "Should keep the range")
	assert.Equal(t, 185, devices.Devices[0].DefaultTemp, "Should keep the default")
}

func TestDeviceTemperaturesAreChecked(t *testing.T) {
	for name, d := range map[string]*catalog.Device{
		"MinAboveMax":     {Name: "x", MinTemp: 220, MaxTemp: 200},
		"DefaultAboveMax": {Name: "x", MaxTemp: 200, DefaultTemp: 250},
		"DefaultBelowMin": {Name: "x", MinTemp: 100, DefaultTemp: 40},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Error(t, validTemps(d), "Should refuse a range that cannot be set")
		})
	}
	assert.NoError(t, validTemps(&catalog.Device{Name: "x", MinTemp: 40, MaxTemp: 230, DefaultTemp: 185}),
		"Should accept a sensible range")
}
