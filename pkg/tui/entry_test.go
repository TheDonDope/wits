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
		produced = append(produced, out)
		// Anything the app itself raises is reported rather than re-delivered,
		// so a test can assert on it.
		if _, ours := out.(entryDoneMsg); !ours {
			queue = append(queue, out)
		}
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
