package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/repo"
)

func TestThemes(t *testing.T) {
	assert.Len(t, palettes, 20, "Should offer the twenty editor themes")
	assert.Equal(t, autoTheme, ThemeIDs()[0], "Should list wits' own theme first")

	seen := map[string]bool{}
	for _, id := range ThemeIDs() {
		assert.False(t, seen[id], "Should not list %s twice", id)
		seen[id] = true

		th, err := ThemeNamed(id, true)
		require.NoError(t, err)
		assert.Equal(t, id, th.ID)
		assert.Len(t, th.Heat, 4, "%s should have a four-step heat ramp", id)
		for name, c := range map[string]tint{
			"fg": th.Fg, "accent": th.Accent, "storage": th.StorageC, "avb": th.AVBC,
			"sesh": th.SeshC, "selection": th.Selection, "line": th.Line,
		} {
			assert.NotNil(t, c, "%s should set %s", id, name)
		}
		if id != autoTheme {
			assert.NotNil(t, th.Bg, "%s should bring its own background", id)
		}
	}

	_, err := ThemeNamed("vim-classic", true)
	assert.ErrorContains(t, err, "dracula", "Should name the themes it does know")
}

func TestThemePaintsTheTerminal(t *testing.T) {
	data := sample(t)
	assert.Nil(t, New(data).View().BackgroundColor, "The auto theme should leave the terminal alone")

	data.Repo = initRepo(t)
	require.NoError(t, data.Repo.SetTheme("nord"))
	app := New(data)
	app.Update(tea.WindowSizeMsg{Width: 96, Height: 40})
	app.Update(tea.BackgroundColorMsg{Color: nil})
	assert.Equal(t, "nord", app.theme.ID, "The terminal's own background should not override a chosen theme")
	assert.NotNil(t, app.View().BackgroundColor, "An editor theme should repaint the terminal")
}

func TestThemePicker(t *testing.T) {
	data := sample(t)
	data.Repo = initRepo(t)
	var m tea.Model = New(data)
	m, _ = send(m, tea.WindowSizeMsg{Width: 96, Height: 40})
	m, _ = send(m, tea.KeyPressMsg{Code: 't', Text: "t"})
	app := m.(*App)
	require.NotNil(t, app.picker, "t should open the picker")
	assert.Contains(t, stripANSI(app.View().Content), "Dark+ (VS Code)")

	m, _ = send(m, tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Equal(t, "dark-plus", app.theme.ID, "Moving the cursor should preview the theme")

	m, _ = send(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Equal(t, autoTheme, app.theme.ID, "esc should put the old theme back")
	assert.Empty(t, data.Repo.Config.Theme, "esc should save nothing")

	m, _ = send(m, tea.KeyPressMsg{Code: 't', Text: "t"})
	m, _ = send(m, tea.KeyPressMsg{Code: tea.KeyDown})
	m, _ = send(m, tea.KeyPressMsg{Code: tea.KeyDown})
	_, _ = send(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Nil(t, app.picker, "enter should close the picker")
	assert.Equal(t, "light-plus", app.theme.ID)

	reopened, err := repo.Discover(data.Repo.WorkTree())
	require.NoError(t, err)
	assert.Equal(t, "light-plus", reopened.Config.Theme, "enter should save the theme to the config")
}

// initRepo is a fresh repository to persist settings into.
func initRepo(t *testing.T) *repo.Repo {
	t.Helper()
	r, err := repo.Init(t.TempDir())
	require.NoError(t, err)
	return r
}
