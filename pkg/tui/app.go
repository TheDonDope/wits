package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/TheDonDope/wits/pkg/repo"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// Data is what the screens read: a workspace snapshot, plus the moment it was
// taken, so that nothing on screen calls time.Now for itself and disagrees with
// the rest of the frame.
type Data struct {
	*workspace.Workspace
	Now time.Time
}

// Load reads a repository into a Data.
func Load(r *repo.Repo) (Data, error) {
	ws, err := workspace.Read(r)
	if err != nil {
		return Data{}, err
	}
	return Data{Workspace: ws, Now: ws.OpenedAt}, nil
}

// screen is one of the views the tab bar switches between.
type screen int

const (
	dashboardScreen screen = iota
	journalScreen
	analysisScreen
	devicesScreen
)

// tabs are the screen names, in order.
var tabs = []string{"Dashboard", "Journal", "Analysis", "Devices"}

// keyMap is the global key bindings. Screens add their own on top.
type keyMap struct {
	Next, Prev     key.Binding
	Up, Down       key.Binding
	PageUp, PgDown key.Binding
	Top, Bottom    key.Binding
	Help, Quit     key.Binding
	New, Sesh, Buy key.Binding
}

func defaultKeys() keyMap {
	return keyMap{
		Next:   key.NewBinding(key.WithKeys("tab", "l", "right"), key.WithHelp("tab", "next")),
		Prev:   key.NewBinding(key.WithKeys("shift+tab", "h", "left"), key.WithHelp("⇧tab", "prev")),
		Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		PageUp: key.NewBinding(key.WithKeys("pgup", "ctrl+b"), key.WithHelp("pgup", "page up")),
		PgDown: key.NewBinding(key.WithKeys("pgdown", "ctrl+f"), key.WithHelp("pgdn", "page down")),
		Top:    key.NewBinding(key.WithKeys("home", "g"), key.WithHelp("g", "top")),
		Bottom: key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("G", "bottom")),
		New:    key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "grind")),
		Sesh:   key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sesh")),
		Buy:    key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "buy")),
		Help:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:   key.NewBinding(key.WithKeys("q", "ctrl+c", "esc"), key.WithHelp("q", "quit")),
	}
}

// ShortHelp implements help.KeyMap.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Next, k.New, k.Sesh, k.Buy, k.Help, k.Quit}
}

// FullHelp implements help.KeyMap.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Next, k.Prev},
		{k.Up, k.Down, k.PageUp, k.PgDown},
		{k.Top, k.Bottom},
		{k.New, k.Sesh, k.Buy},
		{k.Help, k.Quit},
	}
}

// App is the root model. It owns the window size, the theme and the data, and
// routes everything else to the screen in front.
type App struct {
	data   Data
	theme  *Theme
	keys   keyMap
	help   help.Model
	screen screen

	width, height int
	showHelp      bool

	// entry is the form in front, if any. While it is open it takes every key,
	// so navigation cannot fire underneath a half-filled entry.
	entry  *entryForm
	notice string
	failed bool

	dashboard dashboard
	journal   journalView
	analysis  analysisView
	devices   devicesView
	device    *deviceForm
}

// New returns the root model for a repository.
func New(data Data) *App {
	a := &App{
		data:  data,
		theme: NewTheme(true),
		keys:  defaultKeys(),
		help:  help.New(),
	}
	a.dashboard = newDashboard()
	a.journal = newJournalView()
	a.analysis = newAnalysisView()
	a.devices = newDevicesView()
	return a
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd {
	// Ask the terminal what it is, so the theme can be resolved against the
	// actual background rather than assumed.
	return tea.RequestBackgroundColor
}

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.help.SetWidth(msg.Width)

	case tea.BackgroundColorMsg:
		a.theme = NewTheme(msg.IsDark())

	case entryDoneMsg:
		if errors.Is(msg.err, errCancelled) {
			a.notice = ""
			return a, nil
		}
		if msg.err != nil {
			a.notice, a.failed = msg.err.Error(), true
			return a, nil
		}
		a.notice, a.failed = fmt.Sprintf("recorded %s %.2fg %s",
			msg.event.Type, msg.event.Grams, msg.event.Product), false
		return a, a.reload()

	case deviceDoneMsg:
		if errors.Is(msg.err, errCancelled) {
			a.notice = ""
			return a, nil
		}
		if msg.err != nil {
			a.notice, a.failed = msg.err.Error(), true
			return a, nil
		}
		a.notice, a.failed = fmt.Sprintf("saved device %s", msg.name), false
		return a, a.reload()

	case reloadedMsg:
		if msg.err != nil {
			a.notice, a.failed = msg.err.Error(), true
			return a, nil
		}
		a.data = msg.data
		return a, nil

	case tea.KeyPressMsg:
		if a.device != nil {
			if msg.String() == "esc" {
				a.device, a.notice = nil, ""
				return a, nil
			}
			var cmd tea.Cmd
			a.device, cmd = a.device.Update(msg, a)
			return a, cmd
		}
		// A form in front owns the keyboard.
		if a.entry != nil {
			// huh only aborts on ctrl+c, so esc is handled here rather than
			// leaving the panel promising something that does not work.
			if msg.String() == "esc" {
				a.entry, a.notice = nil, ""
				return a, nil
			}
			var cmd tea.Cmd
			a.entry, cmd = a.entry.Update(msg, a)
			return a, cmd
		}
		switch {
		case msg.String() == "n":
			a.entry, a.notice = newEntryForm(entryGrind, a), ""
			return a, a.entry.form.Init()
		case msg.String() == "s" && a.screen != analysisScreen:
			a.entry, a.notice = newEntryForm(entrySesh, a), ""
			return a, a.entry.form.Init()
		case msg.String() == "b":
			a.entry, a.notice = newEntryForm(entryBuy, a), ""
			return a, a.entry.form.Init()
		case msg.String() == "a" && a.screen == devicesScreen:
			a.device, a.notice = newDeviceForm(nil, a), ""
			return a, a.device.form.Init()
		case msg.String() == "e" && a.screen == devicesScreen:
			if d := a.devices.Selected(a); d != nil {
				a.device, a.notice = newDeviceForm(d, a), ""
				return a, a.device.form.Init()
			}
			return a, nil
		case msg.String() == "d" && a.screen == devicesScreen:
			if d := a.devices.Selected(a); d != nil {
				a.device, a.notice = newDeviceRemoveForm(d, a), ""
				return a, a.device.form.Init()
			}
			return a, nil
		case msg.String() == "e" && a.screen == journalScreen:
			if e := a.journal.Selected(); e != nil {
				a.entry, a.notice = newAmendForm(e, a), ""
				return a, a.entry.form.Init()
			}
			return a, nil
		case msg.String() == "d" && a.screen == journalScreen:
			if e := a.journal.Selected(); e != nil {
				a.entry, a.notice = newUndoForm(e, a), ""
				return a, a.entry.form.Init()
			}
			return a, nil
		case key.Matches(msg, a.keys.Quit):
			return a, tea.Quit
		case key.Matches(msg, a.keys.Help):
			a.showHelp = !a.showHelp
			return a, nil
		case key.Matches(msg, a.keys.Next):
			a.screen = screen((int(a.screen) + 1) % len(tabs))
			return a, nil
		case key.Matches(msg, a.keys.Prev):
			a.screen = screen((int(a.screen) - 1 + len(tabs)) % len(tabs))
			return a, nil
		}
	}

	if a.device != nil {
		var cmd tea.Cmd
		a.device, cmd = a.device.Update(msg, a)
		return a, cmd
	}
	if a.entry != nil {
		var cmd tea.Cmd
		a.entry, cmd = a.entry.Update(msg, a)
		return a, cmd
	}

	// Whatever is left belongs to the screen in front. Each screen returns its
	// own updated value rather than replacing the root, so the chrome and the
	// data survive navigation.
	var cmd tea.Cmd
	switch a.screen {
	case dashboardScreen:
		a.dashboard, cmd = a.dashboard.Update(msg, a)
	case journalScreen:
		a.journal, cmd = a.journal.Update(msg, a)
	case analysisScreen:
		a.analysis, cmd = a.analysis.Update(msg, a)
	case devicesScreen:
		a.devices, cmd = a.devices.Update(msg, a)
	}
	return a, cmd
}

// View implements tea.Model.
func (a *App) View() tea.View {
	// In v2 the terminal state is declared by the view rather than set once at
	// program start, so every frame says what it wants.
	view := tea.NewView("")
	view.AltScreen = true
	view.WindowTitle = "wits"
	if a.width == 0 {
		return view
	}
	header := a.header()
	footer := a.footer()
	var body string
	bodyHeight := a.height - lipgloss.Height(header) - lipgloss.Height(footer)
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	if a.entry != nil || a.device != nil {
		panel := ""
		if a.entry != nil {
			panel = a.entry.View(a, a.width)
		} else {
			panel = a.device.View(a, a.width)
		}
		body = lipgloss.NewStyle().Padding(1, 1).Render(panel)
		view.SetContent(lipgloss.NewStyle().MaxWidth(a.width).MaxHeight(a.height).
			Render(lipgloss.JoinVertical(lipgloss.Left, header, body, footer)))
		return view
	}

	switch a.screen {
	case dashboardScreen:
		body = a.dashboard.View(a, bodyHeight)
	case journalScreen:
		body = a.journal.View(a, bodyHeight)
	case analysisScreen:
		body = a.analysis.View(a, bodyHeight)
	case devicesScreen:
		body = a.devices.View(a, bodyHeight)
	}

	// Clamp the finished frame. Inner layout tries to fit, but a narrow terminal
	// will still overflow somewhere, and lipgloss pads every line to the longest
	// one — so a single long line would push the whole frame sideways.
	frame := lipgloss.NewStyle().MaxWidth(a.width).MaxHeight(a.height).
		Render(lipgloss.JoinVertical(lipgloss.Left, header, body, footer))
	view.SetContent(frame)
	return view
}

// header draws the title, the tab bar and a summary of the cycle in progress.
func (a *App) header() string {
	t := a.theme
	title := t.Title.Render("🥦 wits")

	var rendered []string
	for i, name := range tabs {
		if screen(i) == a.screen {
			rendered = append(rendered, t.TabActive.Render(name))
		} else {
			rendered = append(rendered, t.Tab.Render(name))
		}
	}
	bar := lipgloss.JoinHorizontal(lipgloss.Bottom, rendered...)

	left := lipgloss.JoinHorizontal(lipgloss.Bottom, title, "  ", bar)

	// The cycle summary is the first thing to go when the terminal is narrow:
	// the tabs are how you navigate, so they matter more.
	right := ""
	if c := a.data.Cycle(); c != nil {
		candidate := t.Dim.Render(fmt.Sprintf("cycle %d · day %d",
			len(a.data.State.Cycles), daysBetween(c.Start, a.data.Now)+1))
		if lipgloss.Width(left)+lipgloss.Width(candidate)+3 <= a.width {
			right = candidate
		}
	}

	gap := a.width - lipgloss.Width(left) - lipgloss.Width(right) - 1
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + right
	return lipgloss.JoinVertical(lipgloss.Left, " "+line, t.Rule("", a.width))
}

// reloadedMsg carries a freshly folded repository.
type reloadedMsg struct {
	data Data
	err  error
}

// reload re-reads the journal after something has been written to it, so every
// screen sees the new entry at once.
func (a *App) reload() tea.Cmd {
	repository := a.data.Repo
	return func() tea.Msg {
		data, err := Load(repository)
		return reloadedMsg{data: data, err: err}
	}
}

// footer draws the key hints, or the outcome of the last entry.
func (a *App) footer() string {
	if a.notice != "" && a.entry == nil && a.device == nil {
		style := a.theme.Positive
		mark := "✓ "
		if a.failed {
			style, mark = a.theme.Negative, "✗ "
		}
		return lipgloss.JoinVertical(lipgloss.Left,
			a.theme.Rule("", a.width),
			" "+style.Render(mark+a.notice),
		)
	}
	return a.helpFooter()
}

// helpFooter draws the key hints.
func (a *App) helpFooter() string {
	a.help.Styles.ShortKey = a.theme.Key
	a.help.Styles.ShortDesc = a.theme.Help
	a.help.Styles.FullKey = a.theme.Key
	a.help.Styles.FullDesc = a.theme.Help
	a.help.ShowAll = a.showHelp

	keys := a.screenKeys()
	return lipgloss.JoinVertical(lipgloss.Left,
		a.theme.Rule("", a.width),
		" "+a.help.View(keys),
	)
}

// screenKeys returns the bindings relevant to the screen in front.
func (a *App) screenKeys() help.KeyMap {
	switch a.screen {
	case journalScreen:
		return a.journal.keys(a.keys)
	case analysisScreen:
		return a.analysis.keys(a.keys)
	case devicesScreen:
		return a.devices.keys(a.keys)
	default:
		return a.keys
	}
}

// inner returns the width available inside the frame.
func (a *App) inner() int { return maxInt(a.width-2, 10) }
