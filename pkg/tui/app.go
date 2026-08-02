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

	"github.com/TheDonDope/wits/pkg/journal"
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
	return From(ws), nil
}

// From wraps an already-open workspace, so a caller that has one does not read
// the repository a second time.
func From(ws *workspace.Workspace) Data {
	return Data{Workspace: ws, Now: ws.OpenedAt}
}

// screen is one of the views the tab bar switches between.
type screen int

const (
	dashboardScreen screen = iota
	journalScreen
	analysisScreen
	storageScreen
	stashScreen
	sessionsScreen
	devicesScreen
)

// tabs are the screen names, in order.
var tabs = []string{"Dashboard", "Journal", "Analysis", "Storage", "Stash", "Sessions", "Devices"}

// keyMap is the global key bindings. Screens add their own on top, borrowing
// from here where the key is shared, so that a binding is declared once and
// the help line can never advertise a key the dispatch does not honour.
type keyMap struct {
	Next, Prev     key.Binding
	Up, Down       key.Binding
	PageUp, PgDown key.Binding
	Top, Bottom    key.Binding
	Help, Quit     key.Binding
	New, Sesh, Buy key.Binding
	Weigh          key.Binding
	Edit, Delete   key.Binding
	Add            key.Binding
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
		Weigh:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "weigh")),
		Edit:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		Delete: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		Add:    key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
		Help:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:   key.NewBinding(key.WithKeys("q", "ctrl+c", "esc"), key.WithHelp("q", "quit")),
	}
}

// withHelp is a binding with its description reworded for one screen: the same
// key, declared once, read differently where the action has a better name.
func withHelp(b key.Binding, desc string) key.Binding {
	return key.NewBinding(key.WithKeys(b.Keys()...), key.WithHelp(b.Help().Key, desc))
}

// ShortHelp implements help.KeyMap.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Next, k.New, k.Sesh, k.Buy, k.Weigh, k.Help, k.Quit}
}

// FullHelp implements help.KeyMap.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Next, k.Prev},
		{k.Up, k.Down, k.PageUp, k.PgDown},
		{k.Top, k.Bottom},
		{k.New, k.Sesh, k.Buy, k.Weigh},
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
	storage   storageView
	stash     stashView
	sessions  sessionsView
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
	a.storage = newStorageView()
	a.stash = newStashView()
	a.sessions = newSessionsView()
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
		return a.entryDone(msg)

	case deviceDoneMsg:
		return a.deviceDone(msg)

	case reloadedMsg:
		if msg.err != nil {
			a.notice, a.failed = msg.err.Error(), true
			return a, nil
		}
		a.data = msg.data
		return a, nil

	case tea.KeyPressMsg:
		if handled, cmd := a.formKey(msg); handled {
			return a, cmd
		}
		if handled, cmd := a.entryKey(msg); handled {
			return a, cmd
		}
		if handled, cmd := a.correctionKey(msg); handled {
			return a, cmd
		}
		switch {
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
	case storageScreen:
		a.storage, cmd = a.storage.Update(msg, a)
	case stashScreen:
		a.stash, cmd = a.stash.Update(msg, a)
	case sessionsScreen:
		a.sessions, cmd = a.sessions.Update(msg, a)
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
	case storageScreen:
		body = a.storage.View(a, bodyHeight)
	case stashScreen:
		body = a.stash.View(a, bodyHeight)
	case sessionsScreen:
		body = a.sessions.View(a, bodyHeight)
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

// entryDone turns a finished entry form into the notice the footer shows.
func (a *App) entryDone(msg entryDoneMsg) (tea.Model, tea.Cmd) {
	if errors.Is(msg.err, errCancelled) {
		a.notice = ""
		return a, nil
	}
	if msg.err != nil {
		a.notice, a.failed = msg.err.Error(), true
		return a, nil
	}
	if msg.summary != "" {
		// A weighing session: several adjustments summed into one line, and
		// the ticks have served their purpose.
		a.storage.ClearMarks()
		a.stash.ClearMarks()
		a.notice, a.failed = msg.summary, false
		return a, a.reload()
	}
	if msg.event.Type == "" {
		// A product was edited rather than an entry recorded.
		a.notice, a.failed = fmt.Sprintf("renamed %s to %s", msg.event.Product, msg.event.Note), false
		return a, a.reload()
	}
	a.notice, a.failed = fmt.Sprintf("recorded %s %.2fg %s",
		msg.event.Type, msg.event.Grams, msg.event.Product), false
	return a, a.reload()
}

// deviceDone is entryDone for the device forms.
func (a *App) deviceDone(msg deviceDoneMsg) (tea.Model, tea.Cmd) {
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
}

// formKey routes a key to whatever form is in front, which owns the keyboard
// while it is open, so navigation cannot fire underneath a half-filled entry.
func (a *App) formKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if a.device != nil {
		if msg.String() == "esc" {
			a.device, a.notice = nil, ""
			return true, nil
		}
		var cmd tea.Cmd
		a.device, cmd = a.device.Update(msg, a)
		return true, cmd
	}
	if a.entry != nil {
		// huh only aborts on ctrl+c, so esc is handled here rather than
		// leaving the panel promising something that does not work.
		if msg.String() == "esc" {
			a.entry, a.notice = nil, ""
			return true, nil
		}
		var cmd tea.Cmd
		a.entry, cmd = a.entry.Update(msg, a)
		return true, cmd
	}
	return false, nil
}

// open puts an entry form in front and starts it.
func (a *App) open(f *entryForm) (bool, tea.Cmd) {
	a.entry, a.notice = f, ""
	return true, a.entry.form.Init()
}

// entryKey opens the forms that record something new: a grind, a session, a
// fill, a weighing or the history clean-up.
func (a *App) entryKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	switch {
	case key.Matches(msg, a.keys.New):
		return a.open(newEntryForm(entryGrind, a))
	case key.Matches(msg, a.keys.Sesh) && a.screen != analysisScreen:
		return a.open(newEntryForm(entrySesh, a))
	case key.Matches(msg, a.keys.Buy):
		return a.open(newEntryForm(entryBuy, a))
	case key.Matches(msg, cleanKey) && a.screen == storageScreen:
		if stale := staleStashes(a); len(stale) > 0 {
			return a.open(newCleanHistoryForm(stale, a))
		}
		a.notice, a.failed = "no stale stashes to clean", true
		return true, nil
	case key.Matches(msg, a.keys.Weigh) && a.screen != journalScreen:
		return a.weighKey()
	}
	return false, nil
}

// weighKey opens the right weighing form for where the cursor is: the ticked
// jars together, the one under the cursor, or the fullest one. The stash
// screen weighs stashes, so its forms start on that account.
func (a *App) weighKey() (bool, tea.Cmd) {
	if a.screen == storageScreen {
		if marked := a.storage.Marked(a); len(marked) > 0 {
			return a.open(newWeighManyForm(marked, a, journal.Storage))
		}
	}
	if a.screen == stashScreen {
		if marked := a.stash.Marked(a); len(marked) > 0 {
			return a.open(newWeighManyForm(marked, a, journal.Stash))
		}
		if slug := a.stash.Selected(a); slug != "" {
			return a.open(newReconcileForm(slug, a, journal.Stash))
		}
	}
	if slug := a.weighable(); slug != "" {
		return a.open(newReconcileForm(slug, a, journal.Storage))
	}
	a.notice, a.failed = "nothing on the shelf to weigh", true
	return true, nil
}

// correctionKey opens the forms that correct what already exists: a product's
// details, a journal entry, or a device.
func (a *App) correctionKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	edit := key.Matches(msg, a.keys.Edit)
	del := key.Matches(msg, a.keys.Delete)
	switch {
	case edit && a.screen == storageScreen:
		if r := a.storage.Selected(a); r != nil {
			return a.open(newDescribeForm(r.Slug, a))
		}
		return true, nil
	case edit && a.screen == stashScreen:
		if slug := a.stash.Selected(a); slug != "" {
			return a.open(newDescribeForm(slug, a))
		}
		return true, nil
	case edit && a.screen == journalScreen:
		if e := a.journal.Selected(); e != nil {
			return a.open(newAmendForm(e, a))
		}
		return true, nil
	case del && a.screen == journalScreen:
		if e := a.journal.Selected(); e != nil {
			return a.open(newUndoForm(e, a))
		}
		return true, nil
	case key.Matches(msg, a.keys.Add) && a.screen == devicesScreen:
		a.device, a.notice = newDeviceForm(nil, a), ""
		return true, a.device.form.Init()
	case edit && a.screen == devicesScreen:
		if d := a.devices.Selected(a); d != nil {
			a.device, a.notice = newDeviceForm(d, a), ""
			return true, a.device.form.Init()
		}
		return true, nil
	case del && a.screen == devicesScreen:
		if d := a.devices.Selected(a); d != nil {
			a.device, a.notice = newDeviceRemoveForm(d, a), ""
			return true, a.device.form.Init()
		}
		return true, nil
	}
	return false, nil
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
	case storageScreen:
		return a.storage.keys(a.keys)
	case stashScreen:
		return a.stash.keys(a.keys)
	case sessionsScreen:
		return a.sessions.keys(a.keys)
	case devicesScreen:
		return a.devices.keys(a.keys)
	default:
		return a.keys
	}
}

// weighable returns the product r should weigh: the one under the cursor on the
// products screen, or otherwise whichever of the current cycle's products has
// the most left, since that is the jar most worth checking.
func (a *App) weighable() string {
	if a.screen == storageScreen {
		if r := a.storage.Selected(a); r != nil {
			return r.Slug
		}
	}
	best, most := "", -1.0
	cycle := a.data.Cycle()
	if cycle == nil {
		return ""
	}
	for _, slug := range cycle.Products {
		if b := a.data.State.Balances[slug]; b != nil && b.Total() > most {
			best, most = slug, b.Total()
		}
	}
	return best
}

// inner returns the width available inside the frame.
func (a *App) inner() int { return max(a.width-2, 10) }
