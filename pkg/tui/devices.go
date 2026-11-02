package tui

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/record"
	"github.com/TheDonDope/wits/pkg/views"
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

	use := views.Devices(a.data.State.Standing)
	heatW := heatWidth(width)
	rows := []string{deviceHeader(a, heatW)}
	group := ""
	for i, d := range devices {
		if d.Is() != group {
			group = d.Is()
			rows = append(rows, "", t.Rule(categoryNames[group], width))
		}
		selected := i == v.cursor
		rows = append(rows, v.deviceLine(a, d, use[d.Slug], heatW, selected))
		// Under the selected device, say what its usual setting releases:
		// the point of recording a temperature at all.
		if at := usualTemp(d); selected && at > 0 {
			rows = append(rows, "", t.Rule(fmt.Sprintf("At %d°C, as it is usually set", at), width), releasedSummary(a, at, width), "")
		}
	}
	return lipgloss.NewStyle().Padding(1, 1).Render(clip(lipgloss.JoinVertical(lipgloss.Left, rows...), height-2))
}

// categoryNames are the groups the devices screen draws, as the browser names them.
var categoryNames = map[string]string{
	catalog.Vaporizer: "Vaporizers", catalog.Induction: "Induction heaters", catalog.Torch: "Torches",
	catalog.Grinder: "Grinders", catalog.Glass: "Glass",
}

// Column widths of the devices screen. HEAT takes what the others leave,
// so every column stands on screen from 100 columns up.
const (
	devNameW, devMakerW, devUseW, devGramW, devLastW = 21, 12, 12, 9, 13
	devFixedW                                        = 2 + devNameW + devMakerW + devUseW + devGramW + devLastW
)

// heatWidth is what the HEAT column has at a width.
func heatWidth(width int) int { return min(max(width-devFixedW, 12), 40) }

func deviceHeader(a *App, heatW int) string {
	t := a.theme
	return lipgloss.JoinHorizontal(lipgloss.Left, "  ",
		t.Label.Width(devNameW).Render("DEVICE"),
		t.Label.Width(devMakerW).Render("MAKER"),
		t.Label.Width(heatW).Render("HEAT"),
		t.Label.Width(devUseW).Align(lipgloss.Right).Render("USED"),
		t.Label.Width(devGramW).Align(lipgloss.Right).Render("GRAMS"),
		t.Label.Render("  LAST USED"),
	)
}

// deviceLine renders one device as a row: what it is, how it heats, and what
// it has done — sessions for what a session is had with, sessions heated for
// a heater — each figure from views.Devices, as the browser's cards read them.
func (v devicesView) deviceLine(a *App, d *catalog.Device, u *views.Use, heatW int, selected bool) string {
	t := a.theme
	if u == nil {
		u = &views.Use{}
	}
	style, marker := t.Value, "  "
	if d.Retired {
		style = t.Dim
	}
	if selected {
		style, marker = lipgloss.NewStyle().Foreground(t.Accent).Bold(true), lipgloss.NewStyle().Foreground(t.Accent).Render("│ ")
	}
	name := d.Name
	if d.Retired {
		name += " · retired"
	}
	used, grams := plural(u.Sessions, "session"), fmt.Sprintf("%.2f g", u.Grams.Grams())
	switch {
	case d.Heats():
		used, grams = fmt.Sprintf("heated %d", u.Heated), ""
	case d.Is() == catalog.Grinder:
		used, grams = "", ""
	}
	last := ""
	if !u.LastUsed.IsZero() {
		last = "  " + humanDay(u.LastUsed, a.data.Now)
	}
	return lipgloss.JoinHorizontal(lipgloss.Left,
		marker,
		style.Width(devNameW).Render(truncate(name, devNameW-1)),
		t.Dim.Width(devMakerW).Render(truncate(orDash(d.Maker), devMakerW-1)),
		t.Label.Width(heatW).Render(truncate(heatOf(d), heatW-1)),
		t.Value.Width(devUseW).Align(lipgloss.Right).Render(used),
		t.Dim.Width(devGramW).Align(lipgloss.Right).Render(grams),
		t.Dim.Render(last),
	)
}

// heatOf says how a device heats: its range, its usual setting and its
// modes, those it has, or a dash for what does not heat.
func heatOf(d *catalog.Device) string {
	var parts []string
	if d.MaxTemp > 0 {
		parts = append(parts, fmt.Sprintf("%d–%d°C", d.MinTemp, d.MaxTemp))
	}
	if d.DefaultTemp > 0 {
		parts = append(parts, fmt.Sprintf("usually %d°C", d.DefaultTemp))
	}
	if len(d.Modes) > 0 {
		parts = append(parts, strings.Join(d.ModeNames(), ", "))
	}
	if len(parts) == 0 {
		return "—"
	}
	return strings.Join(parts, " · ")
}

// usualTemp is the temperature a device is usually set to: its default, or
// its first mode's.
func usualTemp(d *catalog.Device) int {
	if d.DefaultTemp > 0 {
		return d.DefaultTemp
	}
	for _, m := range d.Modes {
		if m.Temp > 0 {
			return m.Temp
		}
	}
	return 0
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
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

// plural renders a count with its noun, so a device used once does not read
// as "1 sessions" — and two stashes never read as "2 stashs".
func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	if strings.HasSuffix(noun, "sh") || strings.HasSuffix(noun, "s") {
		return fmt.Sprintf("%d %ses", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// deviceList is the devices in the order the screen draws them: by what they
// are, as catalog.Categories orders them, then by name, so the cursor and the
// rows agree.
func (a *App) deviceList() []*catalog.Device {
	if a.data.Devices == nil {
		return nil
	}
	out := slices.Clone(a.data.Devices.Devices)
	rank := func(d *catalog.Device) int { return slices.Index(catalog.Categories, d.Is()) }
	slices.SortStableFunc(out, func(x, y *catalog.Device) int {
		if c := cmp.Compare(rank(x), rank(y)); c != 0 {
			return c
		}
		return cmp.Compare(strings.ToLower(x.Name), strings.ToLower(y.Name))
	})
	return out
}

// sessionsWith counts how many sessions used a device, which is the only honest
// measure of whether it is worth keeping in the list.
func (a *App) sessionsWith(slug string) int {
	n := 0
	for _, e := range a.data.State.Standing {
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
	load          string
}

// newDeviceForm builds the add or edit form. A nil device means adding.
func newDeviceForm(d *catalog.Device, a *App) *deviceForm {
	f := &deviceForm{editing: d}
	if d != nil {
		f.name, f.kind = d.Name, d.Kind
		f.min, f.max, f.def = itoaBlank(d.MinTemp), itoaBlank(d.MaxTemp), itoaBlank(d.DefaultTemp)
		if d.Load > 0 {
			f.load = strconv.FormatFloat(d.Load, 'f', -1, 64)
		}
	}
	f.form = a.dress(huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Name").Value(&f.name).Validate(required("a name")),
		huh.NewInput().Title("Kind").Description("desktop, portable, …").Value(&f.kind),
		huh.NewInput().Title("Lowest temperature").Description("°C").Value(&f.min).Validate(optionalInt),
		huh.NewInput().Title("Highest temperature").Description("°C").Value(&f.max).Validate(optionalInt),
		huh.NewInput().Title("Default temperature").Description("°C, used when a session gives none").
			Value(&f.def).Validate(optionalInt),
		huh.NewInput().Title("A load holds").Description("Grams, as usually filled — sizes a session you forgot to record").
			Value(&f.load).Validate(optionalLoad),
	)))
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
	f.form = a.dress(huh.NewForm(huh.NewGroup(
		huh.NewNote().Title(d.Name).Description(note),
		huh.NewConfirm().Title("Remove this device?").Affirmative("Remove").Negative("Keep").
			Value(&f.confirm),
	)))
	return f
}

// init starts the form.
func (f *deviceForm) init() tea.Cmd { return f.form.Init() }

// view draws the form at the terminal's width.
func (f *deviceForm) view(a *App) string { return f.View(a, a.width) }

// dismiss is esc; nothing is written until the form completes, so there is
// nothing to put back.
func (f *deviceForm) dismiss(*App) {}

// update is Update as a modal: a closed form is a nil modal, not a modal
// holding a nil form.
func (f *deviceForm) update(msg tea.Msg, a *App) (modal, tea.Cmd) {
	next, cmd := f.Update(msg, a)
	if next == nil {
		return nil, cmd
	}
	return next, cmd
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

// commit writes the change to the device catalog as it stands on disk, under
// its lock, and adopts what was written. The form was filled in against the
// screen's copy; writing that copy back would undo a device added by wits
// device add in another terminal meanwhile.
func (f *deviceForm) commit(a *App) error {
	if f.remove && !f.confirm {
		return errCancelled
	}
	fresh, err := a.data.Repo.UpdateDevices(f.apply)
	if err != nil {
		return err
	}
	*a.data.Devices = *fresh
	return nil
}

// apply makes the form's change to a device catalog. The device being edited
// or removed is found again by its slug, since the catalog is a fresh read.
func (f *deviceForm) apply(devices *catalog.Devices) error {
	switch {
	case f.remove:
		f.name = f.editing.Name
		_, err := devices.Remove(f.editing.Slug)
		return err

	case f.editing != nil:
		d, err := devices.Find(f.editing.Slug)
		if err != nil {
			return err
		}
		d.Name = strings.TrimSpace(f.name)
		d.Kind = strings.TrimSpace(f.kind)
		d.MinTemp, d.MaxTemp, d.DefaultTemp = atoiBlank(f.min), atoiBlank(f.max), atoiBlank(f.def)
		d.Load = loadOf(f.load)
		return validTemps(d)

	default:
		d := &catalog.Device{
			Name:        strings.TrimSpace(f.name),
			Kind:        strings.TrimSpace(f.kind),
			MinTemp:     atoiBlank(f.min),
			MaxTemp:     atoiBlank(f.max),
			DefaultTemp: atoiBlank(f.def),
			Load:        loadOf(f.load),
		}
		if err := validTemps(d); err != nil {
			return err
		}
		return devices.Add(d)
	}
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

// rateForm rates what a session did: a score from 1 to 5, or none, on each
// dimension, and a line. It opens on the session's standing rating, so rating
// again starts from what was said before; the effect log keeps both. It is
// drawn by hand rather than with huh, whose fields take three lines each: a
// line a dimension keeps the whole form, and its enter, on a short terminal.
type rateForm struct {
	session *journal.Event
	scores  []int // one per effects.Dimensions, 0 for not rated
	row     int   // the dimension under the cursor; len(scores) is the note
	note    textinput.Model
}

// newRateForm builds the rating form for a session.
func newRateForm(e *journal.Event, a *App) *rateForm {
	f := &rateForm{session: e, scores: make([]int, len(effects.Dimensions)), note: textinput.New()}
	f.note.Placeholder = "a line about it — optional"
	f.note.Prompt = ""
	f.note.SetWidth(min(a.width-24, 50))
	if r, ok := effects.Latest(a.data.Ratings)[e.Hash]; ok {
		for i, d := range effects.Dimensions {
			f.scores[i] = r.Scores[d]
		}
		f.note.SetValue(r.Note)
	}
	return f
}

func (f *rateForm) init() tea.Cmd { return nil }
func (f *rateForm) dismiss(*App)  {}

// update moves between the dimensions, scores them, and rates on enter.
func (f *rateForm) update(msg tea.Msg, a *App) (modal, tea.Cmd) {
	press, ok := msg.(tea.KeyPressMsg)
	if !ok {
		if f.onNote() {
			var cmd tea.Cmd
			f.note, cmd = f.note.Update(msg)
			return f, cmd
		}
		return f, nil
	}
	switch k := press.String(); {
	case k == "enter":
		what, err := f.commit(a)
		return nil, func() tea.Msg { return ratedMsg{what: what, err: err} }
	case k == "up" || k == "shift+tab" || (k == "k" && !f.onNote()):
		return f, f.move(-1)
	case k == "down" || k == "tab" || (k == "j" && !f.onNote()):
		return f, f.move(1)
	case f.onNote():
		var cmd tea.Cmd
		f.note, cmd = f.note.Update(msg)
		return f, cmd
	}
	f.score(press.String())
	return f, nil
}

// score changes the selected dimension's score for a key: a step either way,
// or a digit straight to it. Below 1 is not rated — the dimension is left
// out, not scored low.
func (f *rateForm) score(k string) {
	switch {
	case k == "right" || k == "l":
		f.scores[f.row] = min(f.scores[f.row]+1, effects.Highest)
	case k == "left" || k == "h":
		f.scores[f.row] = max(f.scores[f.row]-1, 0)
	case len(k) == 1 && k[0] >= '0' && k[0] <= '5':
		f.scores[f.row] = int(k[0] - '0')
	}
}

func (f *rateForm) onNote() bool { return f.row == len(f.scores) }

// move steps the cursor, focusing the note when it lands there.
func (f *rateForm) move(by int) tea.Cmd {
	f.row = min(max(f.row+by, 0), len(f.scores))
	if f.onNote() {
		return f.note.Focus()
	}
	f.note.Blur()
	return nil
}

// view draws a line a dimension, the cursor's marked, then the note.
func (f *rateForm) view(a *App) string {
	t := a.theme
	on := lipgloss.NewStyle().Foreground(t.SeshC)
	cursor := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	rows := []string{
		t.PanelTitle.Render("Rate session"),
		t.Dim.Render("↑/↓ choose · ←/→ or 1–5 score · 0 clears · enter rates · esc cancels"),
		"",
		t.Value.Render(describe(f.session, a)),
		"",
	}
	for i, d := range effects.Dimensions {
		marker, label := "  ", t.Label.Width(10).Render(string(d))
		if i == f.row {
			marker, label = cursor.Render("› "), cursor.Width(10).Render(string(d))
		}
		v := f.scores[i]
		dots := on.Render(strings.Repeat("● ", v)) + t.Dim.Render(strings.Repeat("○ ", effects.Highest-v))
		score := t.Dim.Render("—")
		if v > 0 {
			score = on.Render(strconv.Itoa(v))
		}
		rows = append(rows, marker+label+dots+" "+score)
	}
	marker, label := "  ", t.Label.Width(10).Render("note")
	if f.onNote() {
		marker, label = cursor.Render("› "), cursor.Width(10).Render("note")
	}
	rows = append(rows, "", marker+label+f.note.View())
	return t.Panel.Width(min(a.width-2, 80)).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// ratedMsg reports the outcome of a rating.
type ratedMsg struct {
	what string
	err  error
}

// commit rates the session through the recorder, which refuses what the
// command refuses.
func (f *rateForm) commit(a *App) (string, error) {
	scores := map[effects.Dimension]int{}
	for i, d := range effects.Dimensions {
		if v := f.scores[i]; v > 0 {
			scores[d] = v
		}
	}
	if len(scores) == 0 {
		return "", fmt.Errorf("nothing rated — give at least one a score")
	}
	rec := record.New(a.data.Repo, a.data.Products, a.data.Devices, a.data.State)
	r, err := rec.Rate(f.session.Hash, scores, f.note.Value(), time.Now())
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("rated %.2f g of %s: %s", f.session.Grams,
		a.data.ProductName(f.session.Product), effects.Summary(r.Scores)), nil
}

// optionalLoad accepts a load in grams, or nothing.
func optionalLoad(s string) error {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return nonNegativeGrams(s)
}

// loadOf reads a load typed in grams, to the centigram; blank is none.
func loadOf(s string) float64 {
	g, err := ledger.ParseGrams(s)
	if strings.TrimSpace(s) == "" || err != nil {
		return 0
	}
	return journal.CG(g).Grams()
}
