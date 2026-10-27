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
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/record"
	"github.com/TheDonDope/wits/pkg/repo"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// Data is what the screens read: a workspace snapshot, plus the moment it was
// taken, so that nothing on screen calls time.Now for itself and disagrees with
// the rest of the frame.
type Data struct {
	*workspace.Workspace
	Now time.Time

	// stamp is what the repository's files looked like when they were read,
	// so the interface can tell when something else has written to them.
	stamp string
}

// Load reads a repository into a Data. The files are stamped before they are
// read, so a write landing during the read shows as a change on the next look
// rather than being missed.
func Load(r *repo.Repo) (Data, error) {
	before := stamp(r)
	ws, err := workspace.Read(r)
	if err != nil {
		return Data{}, err
	}
	d := From(ws)
	d.stamp = before
	return d, nil
}

// stamp is workspace.Stamp: whether anything was written since a snapshot.
func stamp(r *repo.Repo) string { return workspace.Stamp(r) }

// Cycle returns the fill in progress at the frame's moment — the one every
// figure on screen is computed against.
func (d Data) Cycle() *ledger.Cycle { return d.State.CycleAt(d.Now) }

// From wraps an already-open workspace, so a caller that has one does not read
// the repository a second time.
func From(ws *workspace.Workspace) Data {
	d := Data{Workspace: ws, Now: ws.OpenedAt}
	if ws != nil && ws.Repo != nil {
		d.stamp = stamp(ws.Repo)
	}
	return d
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
	Weigh, Price   key.Binding
	Edit, Delete   key.Binding
	Add            key.Binding
	AVB, Rate      key.Binding
	Theme          key.Binding
}

func defaultKeys() keyMap {
	return keyMap{
		Next:   key.NewBinding(key.WithKeys("tab", "l", "right"), key.WithHelp("tab", "next")),
		Prev:   key.NewBinding(key.WithKeys("shift+tab", "h", "left"), key.WithHelp("⇧tab", "prev")),
		Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		PageUp: key.NewBinding(key.WithKeys("pgup", "ctrl+b"), key.WithHelp("pgup", "page up")),
		PgDown: key.NewBinding(key.WithKeys("pgdown", "ctrl+f"), key.WithHelp("pgdn", "page down")),
		Top:    key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "top")),
		Bottom: key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("G", "bottom")),
		// g grinds, mirroring the command's initial; n stays as a quiet alias
		// for the muscle memory the first weeks built. Jumping to the top moved
		// to home alone to make room.
		New:    key.NewBinding(key.WithKeys("g", "n"), key.WithHelp("g", "grind")),
		Sesh:   key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sesh")),
		Buy:    key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "buy")),
		Weigh:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "weigh")),
		Price:  key.NewBinding(key.WithKeys("$"), key.WithHelp("$", "price purchases")),
		Edit:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		Delete: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		Add:    key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
		// v is AVB, as in the browser, everywhere but the journal, where it
		// shows the corrections behind what stands.
		AVB:   key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "AVB")),
		Rate:  key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "rate strain")),
		Theme: key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "theme")),
		Help:  key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		// esc is not a way out: it closes a form or a picker, and one press
		// too many after closing one used to quit the whole interface.
		Quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

// withHelp is a binding with its description reworded for one screen: the same
// key, declared once, read differently where the action has a better name.
func withHelp(b key.Binding, desc string) key.Binding {
	return key.NewBinding(key.WithKeys(b.Keys()...), key.WithHelp(b.Help().Key, desc))
}

// ShortHelp implements help.KeyMap.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Next, k.Buy, k.New, k.Sesh, k.Weigh, k.Help, k.Quit}
}

// FullHelp implements help.KeyMap.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Next, k.Prev},
		{k.Up, k.Down, k.PageUp, k.PgDown},
		{k.Top, k.Bottom},
		{k.Buy, k.New, k.Sesh, k.Weigh, k.AVB, k.Price},
		{k.Theme, k.Help, k.Quit},
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

	// dark is what the terminal reported its background to be. Only the auto
	// theme reads it; the editor themes bring a background of their own.
	dark bool

	// wall is the ticking wall clock the dashboard shows. Data.Now stays the
	// frame every derived figure is computed against; the wall is only ever
	// read for display, so the two disagreeing cannot put a number on screen.
	wall time.Time

	// reloading is set while a reload is on its way, so a slow one is not
	// asked for again on every tick.
	reloading bool

	// front is the form in front, if any — an entry, a device, the theme
	// picker. While it is open it takes every key, so navigation cannot fire
	// underneath a half-filled entry.
	front  modal
	notice string
	failed bool

	dashboard dashboard
	journal   journalView
	analysis  analysisView
	storage   storageView
	stash     stashView
	sessions  sessionsView
	devices   devicesView
}

// modal is a form in front of the screens. There is one slot for it, so the
// rules — it takes every key, esc closes it, the notice waits behind it — are
// written once rather than once for each kind of form, where they were three
// nilable fields checked in turn in five places.
type modal interface {
	// init starts it.
	init() tea.Cmd
	// update advances it with any message, returning nil once it has closed.
	update(msg tea.Msg, a *App) (modal, tea.Cmd)
	// dismiss is esc: huh only aborts on ctrl+c, so the App closes the form
	// itself, and the form puts back anything it changed while open.
	dismiss(a *App)
	// view draws it.
	view(a *App) string
}

// show puts a form in front and starts it.
func (a *App) show(m modal) (bool, tea.Cmd) {
	a.front, a.notice = m, ""
	return true, m.init()
}

// New returns the root model for a repository.
func New(data Data) *App {
	a := &App{
		data:  data,
		theme: NewTheme(true),
		keys:  defaultKeys(),
		help:  help.New(),
		dark:  true,
	}
	if data.Workspace != nil && data.Repo != nil {
		a.useTheme(data.Repo.Config.Theme)
	}
	if data.Workspace != nil && data.Chain != nil {
		a.notice, a.failed = "the journal was changed outside wits; wits fsck says where", true
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
	return tea.Batch(tea.RequestBackgroundColor, tickWall())
}

// wallTickMsg carries the clock, once a second.
type wallTickMsg time.Time

// refresh moves the wall clock on, and reloads when the snapshot has gone
// stale: another process has written to the repository — a grind from the
// command line while the interface is open — or the day has turned, so that
// "today" and "day N" are no longer true. Before this the screen showed the
// repository as it was when it opened until something was entered in it.
func (a *App) refresh(now time.Time) tea.Cmd {
	a.wall = now
	if a.reloading || a.data.Workspace == nil || a.data.Repo == nil {
		return nil
	}
	sameDay := now.Format(time.DateOnly) == a.data.Now.Format(time.DateOnly)
	if sameDay && stamp(a.data.Repo) == a.data.stamp {
		return nil
	}
	a.reloading = true
	return a.reload()
}

// tickWall schedules the next reading of the wall clock.
func tickWall() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return wallTickMsg(t) })
}

// clock is the time the dashboard displays: the ticking wall once it has
// ticked, and the frame's own moment before that — a snapshot has no ticks,
// and a blank corner would read as a fault.
func (a *App) clock() time.Time {
	if a.wall.IsZero() {
		return a.data.Now
	}
	return a.wall
}

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.help.SetWidth(msg.Width)

	case tea.BackgroundColorMsg:
		a.dark = msg.IsDark()
		if a.theme.ID == autoTheme {
			a.theme = NewTheme(a.dark)
		}

	case wallTickMsg:
		return a, tea.Batch(a.refresh(time.Time(msg)), tickWall())

	case entryDoneMsg:
		return a.entryDone(msg)

	case deviceDoneMsg:
		return a.deviceDone(msg)

	case ratedMsg:
		return a.rated(msg)

	case reloadedMsg:
		if msg.err != nil {
			a.notice, a.failed, a.reloading = msg.err.Error(), true, false
			return a, nil
		}
		a.data, a.reloading = msg.data, false
		return a, nil

	case tea.KeyPressMsg:
		if handled, cmd := a.keyPress(msg); handled {
			return a, cmd
		}
	}

	// huh advances its fields through commands of its own, so the form in
	// front sees every message, not only keys.
	if a.front != nil {
		var cmd tea.Cmd
		a.front, cmd = a.front.update(msg, a)
		return a, cmd
	}

	return a, a.toScreen(msg)
}

// keyPress offers a key to the form in front, then to whatever opens a form,
// then to navigation; a key none of them takes goes on to the screen.
func (a *App) keyPress(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	// A notice lasts until the next key, which then does what it always
	// does — otherwise the outcome of the last entry hides the key hints.
	if a.front == nil {
		a.notice = ""
	}
	for _, handle := range []func(tea.KeyPressMsg) (bool, tea.Cmd){a.formKey, a.entryKey, a.correctionKey, a.navKey} {
		if handled, cmd := handle(msg); handled {
			return true, cmd
		}
	}
	return false, nil
}

// toScreen hands a message to the screen in front. Each screen returns its
// own updated value rather than replacing the root, so the chrome and the
// data survive navigation.
func (a *App) toScreen(msg tea.Msg) tea.Cmd {
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
	return cmd
}

// dialog renders the form in front, if one is.
func (a *App) dialog() (string, bool) {
	if a.front == nil {
		return "", false
	}
	return a.front.view(a), true
}

// View implements tea.Model.
func (a *App) View() tea.View {
	// In v2 the terminal state is declared by the view rather than set once at
	// program start, so every frame says what it wants.
	view := tea.NewView("")
	view.AltScreen = true
	view.WindowTitle = "wits"
	// An editor theme repaints the terminal itself, the way the editor would;
	// Bubble Tea puts the terminal's own colours back on the way out.
	if a.theme.Bg != nil {
		view.BackgroundColor, view.ForegroundColor = a.theme.Bg, a.theme.Fg
	}
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

	if panel, open := a.dialog(); open {
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
			c.Seq+1, daysBetween(c.Start, a.data.Now)+1))
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

// navKey is quitting, help, and the tab bar. On the journal the horizontal
// keys belong to the cover slider, so only tab and shift+tab change screens
// from there — an unhandled key falls through to the screen.
func (a *App) navKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	switch {
	case key.Matches(msg, a.keys.Quit):
		return true, tea.Quit
	case key.Matches(msg, a.keys.Help):
		a.showHelp = !a.showHelp
		return true, nil
	case key.Matches(msg, a.keys.Theme):
		return a.show(newThemePicker(a))
	case key.Matches(msg, a.keys.Next):
		if a.slides() && msg.String() != "tab" {
			return false, nil
		}
		a.screen = screen((int(a.screen) + 1) % len(tabs))
		return true, nil
	case key.Matches(msg, a.keys.Prev):
		if a.slides() && msg.String() != "shift+tab" {
			return false, nil
		}
		a.screen = screen((int(a.screen) - 1 + len(tabs)) % len(tabs))
		return true, nil
	}
	return false, nil
}

// slides reports whether the screen in front reads the horizontal keys for
// itself — the journal's cover slider and the replay transports — leaving
// only tab and shift+tab to change screens.
func (a *App) slides() bool {
	switch a.screen {
	case journalScreen, analysisScreen, storageScreen, stashScreen:
		return true
	}
	return false
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

// rated reports a rating, or why it was refused.
func (a *App) rated(msg ratedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		a.notice, a.failed = msg.err.Error(), true
		return a, nil
	}
	a.notice, a.failed = msg.what, false
	return a, a.reload()
}

// formKey routes a key to the form in front, which owns the keyboard while it
// is open, so navigation cannot fire underneath a half-filled entry.
func (a *App) formKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if a.front == nil {
		return false, nil
	}
	if msg.String() == "esc" {
		a.front.dismiss(a)
		a.front, a.notice = nil, ""
		return true, nil
	}
	var cmd tea.Cmd
	a.front, cmd = a.front.update(msg, a)
	return true, cmd
}

// open puts an entry form in front and starts it.
func (a *App) open(f *entryForm) (bool, tea.Cmd) { return a.show(f) }

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
	case key.Matches(msg, a.keys.AVB) && a.screen != journalScreen:
		return a.show(newAVBForm(a))
	case key.Matches(msg, a.keys.Price):
		return a.priceKey()
	}
	return false, nil
}

// priceKey walks the purchases still in storage without a price, the way
// `wits reconcile purchases` does.
func (a *App) priceKey() (bool, tea.Cmd) {
	if a.data.Repo == nil {
		return false, nil
	}
	rec := record.New(a.data.Repo, a.data.Products, a.data.Devices, a.data.State)
	unpriced, err := rec.Unpriced(false)
	if err != nil {
		a.notice, a.failed = err.Error(), true
		return true, nil
	}
	if len(unpriced) == 0 {
		a.notice, a.failed = "every purchase in storage has its price", false
		return true, nil
	}
	return a.open(newPriceForm(unpriced, a))
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
	var action string
	switch {
	case key.Matches(msg, a.keys.Edit):
		action = "edit"
	case key.Matches(msg, a.keys.Delete):
		action = "delete"
	case key.Matches(msg, a.keys.Add):
		action = "add"
	case key.Matches(msg, a.keys.Rate):
		action = "rate"
	default:
		return false, nil
	}
	form, ok := corrections[a.screen][action]
	if !ok {
		return false, nil
	}
	if m := form(a); m != nil {
		return a.show(m)
	}
	return true, nil // the key belongs here, but nothing is selected
}

// corrections is which form each screen opens on edit, delete and add, built
// from what is selected there — or nil when nothing is. A key a screen does
// not list falls through to navigation.
var corrections = map[screen]map[string]func(a *App) modal{
	storageScreen: {
		"edit": func(a *App) modal {
			if r := a.storage.Selected(a); r != nil {
				return newDescribeForm(r.Slug, a)
			}
			return nil
		},
		"rate": func(a *App) modal {
			if r := a.storage.Selected(a); r != nil {
				return newStrainForm(r.Slug, a)
			}
			return nil
		},
	},
	stashScreen: {
		"edit": func(a *App) modal {
			if slug := a.stash.Selected(a); slug != "" {
				return newDescribeForm(slug, a)
			}
			return nil
		},
		"rate": func(a *App) modal {
			if slug := a.stash.Selected(a); slug != "" {
				return newStrainForm(slug, a)
			}
			return nil
		},
	},
	journalScreen: {
		"edit": func(a *App) modal {
			if e := a.journal.Selected(); e != nil {
				return newAmendForm(e, a)
			}
			return nil
		},
		"delete": func(a *App) modal {
			if e := a.journal.Selected(); e != nil {
				return newUndoForm(e, a)
			}
			return nil
		},
	},
	sessionsScreen: {"edit": func(a *App) modal {
		if e := a.sessions.Selected(a); e != nil {
			return newRateForm(e, a)
		}
		return nil
	}},
	devicesScreen: {
		"add": func(a *App) modal { return newDeviceForm(nil, a) },
		"edit": func(a *App) modal {
			if d := a.devices.Selected(a); d != nil {
				return newDeviceForm(d, a)
			}
			return nil
		},
		"delete": func(a *App) modal {
			if d := a.devices.Selected(a); d != nil {
				return newDeviceRemoveForm(d, a)
			}
			return nil
		},
	},
}

// reloadedMsg carries a freshly folded repository.
type reloadedMsg struct {
	data Data
	err  error
}

// reloadAtOnce reads the repository again inside Update rather than in a
// command. The tests set it: they run a command the way the runtime does but
// give up on one that takes longer than a timer would, and a reload on a slow
// machine under the race detector can take longer than that.
var reloadAtOnce = false

// reload re-reads the journal after something has been written to it, so every
// screen sees the new entry at once.
func (a *App) reload() tea.Cmd {
	repository := a.data.Repo
	if reloadAtOnce {
		data, err := Load(repository)
		return func() tea.Msg { return reloadedMsg{data: data, err: err} }
	}
	return func() tea.Msg {
		data, err := Load(repository)
		return reloadedMsg{data: data, err: err}
	}
}

// footer draws the key hints, or the outcome of the last entry.
func (a *App) footer() string {
	if a.notice != "" && a.front == nil {
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

// ScreenNames lists the screens a Snapshot can render, in tab order.
func ScreenNames() []string {
	out := make([]string, len(tabs))
	for i, name := range tabs {
		out[i] = strings.ToLower(name)
	}
	return out
}

// Snapshot renders one screen to plain text, no terminal required. It is the
// seam the tooling photographs through — pull request captures, quick visual
// checks, future clients that want to see what the interface would say —
// driving the same model the running program drives, keystroke by keystroke.
func Snapshot(data Data, screenName string, width, height int, presses []string) (string, error) {
	at := -1
	for i, name := range tabs {
		if strings.EqualFold(name, screenName) {
			at = i
		}
	}
	if at < 0 {
		return "", fmt.Errorf("no screen called %q; the screens are %s",
			screenName, strings.Join(ScreenNames(), ", "))
	}

	app := New(data)
	app.screen = screen(at)
	var m tea.Model = app
	m = deliver(m, tea.WindowSizeMsg{Width: width, Height: height})
	for _, press := range presses {
		msg, err := pressFor(press)
		if err != nil {
			return "", err
		}
		m = deliver(m, msg)
	}
	return stripANSI(m.View().Content), nil
}

// deliver sends a message and drains the commands it produces, the way the
// runtime does. huh moves between fields by returning a command, so a snapshot
// that dropped them would photograph every form frozen before its first field.
func deliver(m tea.Model, msg tea.Msg) tea.Model {
	msgs := []tea.Msg{msg}
	for steps := 0; len(msgs) > 0 && steps < 128; steps++ {
		next := msgs[0]
		msgs = msgs[1:]
		if batch, ok := next.(tea.BatchMsg); ok {
			for _, cmd := range batch {
				if out := promptly(cmd); out != nil {
					msgs = append(msgs, out)
				}
			}
			continue
		}
		var cmd tea.Cmd
		m, cmd = m.Update(next)
		if cmd != nil {
			if out := promptly(cmd); out != nil {
				msgs = append(msgs, out)
			}
		}
	}
	return m
}

// promptly runs a command, abandoning one that does not answer at once — a
// cursor blink waits on a timer, and a snapshot has no interest in it.
func promptly(cmd tea.Cmd) tea.Msg {
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		return msg
	case <-time.After(50 * time.Millisecond):
		return nil
	}
}

// pressFor turns a spelled-out key into the message the runtime would send.
// "tick" is the playback clock, so a replay can be photographed mid-run.
func pressFor(press string) (tea.Msg, error) {
	switch press {
	case "tick":
		return playTickMsg{}, nil
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}, nil
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}, nil
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}, nil
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}, nil
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}, nil
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}, nil
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}, nil
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}, nil
	}
	runes := []rune(press)
	if len(runes) != 1 {
		return nil, fmt.Errorf("cannot press %q; use a single key, tick, or a named key", press)
	}
	return tea.KeyPressMsg{Code: runes[0], Text: press}, nil
}

// stripANSI removes the escape sequences from a rendered frame, leaving the
// text a capture or an assertion can read.
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
