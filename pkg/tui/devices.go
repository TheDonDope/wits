package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/journal"
)

// devicesView lists the vaporizers and what each one's temperature range
// actually reaches.
type devicesView struct {
	cursor int
}

func newDevicesView() devicesView { return devicesView{} }

type deviceKeys struct {
	keyMap
	Add, Edit, Remove key.Binding
}

func (k deviceKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Add, k.Edit, k.Remove, k.Help, k.Quit}
}

func (k deviceKeys) FullHelp() [][]key.Binding {
	return append(k.keyMap.FullHelp(), []key.Binding{k.Add, k.Edit, k.Remove})
}

func (v devicesView) keys(base keyMap) help.KeyMap {
	return deviceKeys{
		keyMap: base,
		Add:    base.Add,
		Edit:   base.Edit,
		Remove: withHelp(base.Delete, "remove"),
	}
}

func (v devicesView) Update(msg tea.Msg, a *App) (devicesView, tea.Cmd) {
	count := len(a.deviceList())
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(msg, a.keys.Up):
			v.cursor = max(v.cursor-1, 0)
		case key.Matches(msg, a.keys.Down):
			v.cursor = min(v.cursor+1, max(count-1, 0))
		}
	}
	return v, nil
}

// Selected returns the device under the cursor.
func (v devicesView) Selected(a *App) *catalog.Device {
	devices := a.deviceList()
	if v.cursor < 0 || v.cursor >= len(devices) {
		return nil
	}
	return devices[v.cursor]
}

func (v devicesView) View(a *App, height int) string {
	t := a.theme
	width := a.inner()
	devices := a.deviceList()

	if len(devices) == 0 {
		return lipgloss.NewStyle().Padding(1, 1).Render(
			t.Subtitle.Render("No devices yet.\n\nPress ") + t.Key.Render("a") +
				t.Subtitle.Render(" to add one. A device gives a session its temperature,\nand a temperature is what decides which compounds come off."))
	}

	var rows []string
	for i, d := range devices {
		selected := i == v.cursor
		name := t.Value.Render(truncate(d.Name, 28))
		if selected {
			name = lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Render(truncate(d.Name, 28))
		}
		marker := "  "
		if selected {
			marker = lipgloss.NewStyle().Foreground(t.Accent).Render("│ ")
		}

		kind := d.Kind
		if kind == "" {
			kind = "—"
		}
		temps := "—"
		if d.MaxTemp > 0 {
			temps = fmt.Sprintf("%d–%d°C", d.MinTemp, d.MaxTemp)
		}
		def := "—"
		if d.DefaultTemp > 0 {
			def = fmt.Sprintf("%d°C", d.DefaultTemp)
		}

		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Left,
			marker,
			lipgloss.NewStyle().Width(30).Render(name),
			t.Dim.Width(12).Render(kind),
			t.Label.Width(12).Render(temps),
			t.Value.Width(8).Render(def),
			t.Dim.Render(plural(a.sessionsWith(d.Slug), "session")),
		))

		// Under the selected device, say what its default setting releases: the
		// point of recording a temperature at all.
		if selected && d.DefaultTemp > 0 {
			rows = append(rows, "", t.Rule(fmt.Sprintf("At %d°C", d.DefaultTemp), width))
			rows = append(rows, releasedSummary(a, d.DefaultTemp, width), "")
		}
	}

	header := lipgloss.JoinHorizontal(lipgloss.Left,
		t.Label.Width(32).Render("DEVICE"),
		t.Label.Width(12).Render("KIND"),
		t.Label.Width(12).Render("RANGE"),
		t.Label.Width(8).Render("DEFAULT"),
		t.Label.Render("USE"),
	)
	return lipgloss.NewStyle().Padding(1, 1).Render(
		clip(lipgloss.JoinVertical(lipgloss.Left, append([]string{header, ""}, rows...)...), height-2))
}

// releasedSummary lists what a temperature reaches, and warns if it is hot
// enough to produce benzene.
func releasedSummary(a *App, celsius, width int) string {
	t := a.theme
	released := catalog.ReleasedAt(celsius)
	if len(released) == 0 {
		return t.Dim.Render("nothing has reached its boiling point yet")
	}
	names := make([]string, 0, len(released))
	for _, r := range released {
		names = append(names, r.Name)
	}
	body := t.Dim.Render(truncate(strings.Join(names, ", "), width-2))

	if hazards := catalog.Hazards(celsius); len(hazards) > 0 {
		body = lipgloss.JoinVertical(lipgloss.Left, body,
			t.Negative.Render(fmt.Sprintf("⚠  at or above the %d°C boiling point of %s",
				hazards[0].BoilingPoint, hazards[0].Name)))
	}
	return body
}

// plural renders a count with its noun, so a device used once does not read as
// "1 sessions".
func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// deviceList returns the device catalog as a slice.
func (a *App) deviceList() []*catalog.Device {
	if a.data.Devices == nil {
		return nil
	}
	return a.data.Devices.Devices
}

// sessionsWith counts how many sessions used a device, which is the only honest
// measure of whether it is worth keeping in the list.
func (a *App) sessionsWith(slug string) int {
	n := 0
	for _, e := range a.data.State.Events {
		if e.Type == journal.Sesh && e.Device == slug {
			n++
		}
	}
	return n
}

// deviceForm adds or edits a device.
type deviceForm struct {
	form    *huh.Form
	editing *catalog.Device
	remove  bool
	confirm bool

	name, kind    string
	min, max, def string
}

// newDeviceForm builds the add or edit form. A nil device means adding.
func newDeviceForm(d *catalog.Device, a *App) *deviceForm {
	f := &deviceForm{editing: d}
	if d != nil {
		f.name, f.kind = d.Name, d.Kind
		f.min, f.max, f.def = itoaBlank(d.MinTemp), itoaBlank(d.MaxTemp), itoaBlank(d.DefaultTemp)
	}
	f.form = huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Name").Value(&f.name).Validate(required("a name")),
		huh.NewInput().Title("Kind").Description("desktop, portable, …").Value(&f.kind),
		huh.NewInput().Title("Lowest temperature").Description("°C").Value(&f.min).Validate(optionalInt),
		huh.NewInput().Title("Highest temperature").Description("°C").Value(&f.max).Validate(optionalInt),
		huh.NewInput().Title("Default temperature").Description("°C, used when a session gives none").
			Value(&f.def).Validate(optionalInt),
	)).WithShowHelp(true).WithWidth(min(a.inner(), 72))
	return f
}

// newDeviceRemoveForm confirms removing a device.
func newDeviceRemoveForm(d *catalog.Device, a *App) *deviceForm {
	f := &deviceForm{editing: d, remove: true}
	used := a.sessionsWith(d.Slug)
	note := "No sessions have used it."
	if used > 0 {
		// The journal keeps naming the device on those sessions whatever the
		// catalog says, so removing it loses the description, not the history.
		note = fmt.Sprintf("%s used it. Those entries keep the name;\nonly the device's details are forgotten.", plural(used, "session"))
	}
	f.form = huh.NewForm(huh.NewGroup(
		huh.NewNote().Title(d.Name).Description(note),
		huh.NewConfirm().Title("Remove this device?").Affirmative("Remove").Negative("Keep").
			Value(&f.confirm),
	)).WithShowHelp(true).WithWidth(min(a.inner(), 72))
	return f
}

// Update advances the device form and applies it once complete.
func (f *deviceForm) Update(msg tea.Msg, a *App) (*deviceForm, tea.Cmd) {
	model, cmd := f.form.Update(msg)
	if form, ok := model.(*huh.Form); ok {
		f.form = form
	}
	switch f.form.State {
	case huh.StateAborted:
		return nil, nil
	case huh.StateCompleted:
		err := f.commit(a)
		return nil, func() tea.Msg { return deviceDoneMsg{err: err, name: f.name} }
	}
	return f, cmd
}

// deviceDoneMsg reports the outcome of a device change.
type deviceDoneMsg struct {
	name string
	err  error
}

// commit writes the catalog back.
func (f *deviceForm) commit(a *App) error {
	devices := a.data.Devices
	switch {
	case f.remove:
		if !f.confirm {
			return errCancelled
		}
		kept := devices.Devices[:0]
		for _, d := range devices.Devices {
			if d.Slug != f.editing.Slug {
				kept = append(kept, d)
			}
		}
		devices.Devices = kept
		f.name = f.editing.Name

	case f.editing != nil:
		f.editing.Name = strings.TrimSpace(f.name)
		f.editing.Kind = strings.TrimSpace(f.kind)
		f.editing.MinTemp, f.editing.MaxTemp, f.editing.DefaultTemp = atoiBlank(f.min), atoiBlank(f.max), atoiBlank(f.def)
		if err := validTemps(f.editing); err != nil {
			return err
		}

	default:
		d := &catalog.Device{
			Name:        strings.TrimSpace(f.name),
			Kind:        strings.TrimSpace(f.kind),
			MinTemp:     atoiBlank(f.min),
			MaxTemp:     atoiBlank(f.max),
			DefaultTemp: atoiBlank(f.def),
		}
		if err := validTemps(d); err != nil {
			return err
		}
		if err := devices.Add(d); err != nil {
			return err
		}
	}
	return devices.Save(a.data.Repo.DevicesPath())
}

// validTemps rejects a range that cannot be set, which would otherwise only
// surface later as a refused session.
func validTemps(d *catalog.Device) error {
	if d.MinTemp > 0 && d.MaxTemp > 0 && d.MinTemp > d.MaxTemp {
		return fmt.Errorf("the lowest temperature is above the highest")
	}
	if d.DefaultTemp > 0 && d.MaxTemp > 0 && d.DefaultTemp > d.MaxTemp {
		return fmt.Errorf("the default is above what the device reaches")
	}
	if d.DefaultTemp > 0 && d.MinTemp > 0 && d.DefaultTemp < d.MinTemp {
		return fmt.Errorf("the default is below what the device reaches")
	}
	return nil
}

// View renders the device form in a panel.
func (f *deviceForm) View(a *App, width int) string {
	t := a.theme
	title := "Add device"
	switch {
	case f.remove:
		title = "Remove device"
	case f.editing != nil:
		title = "Edit device"
	}
	return t.Panel.Width(min(width-2, 74)).Render(lipgloss.JoinVertical(lipgloss.Left,
		t.PanelTitle.Render(title),
		t.Dim.Render("enter to move on · esc to cancel"),
		"",
		f.form.View(),
	))
}

// itoaBlank renders zero as an empty string, since zero means unset here.
func itoaBlank(v int) string {
	if v == 0 {
		return ""
	}
	return strconv.Itoa(v)
}

// atoiBlank reads an empty string as zero.
func atoiBlank(s string) int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return v
}
