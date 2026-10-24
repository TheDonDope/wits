package tui

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

// Theme holds every colour and style the interface uses.
//
// Lip Gloss v2 removed AdaptiveColor, so light and dark are resolved once here
// against the terminal's reported background rather than at each call site.
type Theme struct {
	// ID is the palette the theme was built from; autoTheme follows the
	// terminal and paints no background of its own.
	ID   string
	Dark bool

	// Palette
	Bg               tint
	Fg, Muted, Faint tint
	Accent, Alt      tint
	Good, Warn, Bad  tint
	Line, Selection  tint
	StorageC, StashC tint
	AVBC, SeshC      tint
	Heat             []tint

	// Chrome
	Title, Subtitle   lipgloss.Style
	Tab, TabActive    lipgloss.Style
	Panel, PanelTitle lipgloss.Style
	Help, Key         lipgloss.Style

	// Content
	Label, Value, Unit lipgloss.Style
	Big                lipgloss.Style
	Positive, Negative lipgloss.Style
	Dim                lipgloss.Style
}

// tint is a resolved terminal colour. Lip Gloss v2 makes Color a constructor
// function rather than a type, so colours are held as image/color values.
type tint = color.Color

// autoTheme is the id of wits' own palette, which follows the terminal's light
// or dark background instead of bringing one.
const autoTheme = "auto"

// NewTheme builds the theme for a light or dark terminal.
func NewTheme(dark bool) *Theme {
	pick := lipgloss.LightDark(dark)
	t := &Theme{ID: autoTheme, Dark: dark}

	t.Fg = pick(lipgloss.Color("#1A1A1A"), lipgloss.Color("#EEEEEE"))
	t.Muted = pick(lipgloss.Color("#6C6C6C"), lipgloss.Color("#9A9A9A"))
	t.Faint = pick(lipgloss.Color("#B0B0B0"), lipgloss.Color("#4A4A4A"))
	t.Accent = pick(lipgloss.Color("#00A36A"), lipgloss.Color("#02BF87"))
	t.Alt = pick(lipgloss.Color("#5A56E0"), lipgloss.Color("#7571F9"))
	t.Good = pick(lipgloss.Color("#02BA84"), lipgloss.Color("#02BF87"))
	t.Warn = pick(lipgloss.Color("#B7791F"), lipgloss.Color("#E3B341"))
	t.Bad = pick(lipgloss.Color("#D6336C"), lipgloss.Color("#FE5F86"))
	t.Line = pick(lipgloss.Color("#DDDDDD"), lipgloss.Color("#3A3A3A"))
	t.Selection = pick(lipgloss.Color("#EEF1F6"), lipgloss.Color("#22262E"))

	// One colour per account, used consistently by every chart and table so a
	// colour means the same thing wherever it appears.
	t.StorageC = pick(lipgloss.Color("#2F6FED"), lipgloss.Color("#6AA6FF"))
	t.StashC = t.Accent
	t.AVBC = pick(lipgloss.Color("#8B5E34"), lipgloss.Color("#C98A55"))
	t.SeshC = pick(lipgloss.Color("#9B51E0"), lipgloss.Color("#B57BEE"))

	// Heat is the intensity ramp the calendar and the heavier chart columns
	// use, dimmest first. The shades are the contribution greens GitHub uses,
	// which read on both backgrounds and already mean "more happened here" to
	// anyone who has looked at a profile page.
	if dark {
		t.Heat = []tint{
			lipgloss.Color("#0E4429"), lipgloss.Color("#006D32"),
			lipgloss.Color("#26A641"), lipgloss.Color("#39D353"),
		}
	} else {
		t.Heat = []tint{
			lipgloss.Color("#9BE9A8"), lipgloss.Color("#40C463"),
			lipgloss.Color("#30A14E"), lipgloss.Color("#216E39"),
		}
	}
	t.style()
	return t
}

// style derives every style from the palette, so a theme is only its colours.
func (t *Theme) style() {
	t.Title = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
	t.Subtitle = lipgloss.NewStyle().Foreground(t.Muted)
	t.Tab = lipgloss.NewStyle().Foreground(t.Muted).Padding(0, 2)
	t.TabActive = lipgloss.NewStyle().Bold(true).Foreground(t.Accent).
		Padding(0, 2).Underline(true)
	t.Panel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).BorderForeground(t.Line).Padding(0, 1)
	t.PanelTitle = lipgloss.NewStyle().Bold(true).Foreground(t.Alt)
	t.Help = lipgloss.NewStyle().Foreground(t.Faint)
	t.Key = lipgloss.NewStyle().Foreground(t.Muted).Bold(true)

	t.Label = lipgloss.NewStyle().Foreground(t.Muted)
	t.Value = lipgloss.NewStyle().Foreground(t.Fg)
	t.Unit = lipgloss.NewStyle().Foreground(t.Faint)
	t.Big = lipgloss.NewStyle().Bold(true).Foreground(t.Fg)
	t.Positive = lipgloss.NewStyle().Foreground(t.Good)
	t.Negative = lipgloss.NewStyle().Foreground(t.Bad)
	t.Dim = lipgloss.NewStyle().Foreground(t.Faint)
}

// Level returns the colour for a proportion remaining: comfortable, getting
// low, or nearly out.
func (t *Theme) Level(fraction float64) tint {
	switch {
	case fraction <= 0.15:
		return t.Bad
	case fraction <= 0.35:
		return t.Warn
	default:
		return t.Good
	}
}

// palette is an editor theme as its authors publish it: hex colours, mapped
// onto the roles wits draws with. The accounts borrow the theme's own syntax
// colours — storage its blue, AVB its orange or brown, sessions its purple or
// pink — so the charts keep their meaning in every theme.
type palette struct {
	id, name string
	dark     bool

	bg, fg, muted, faint, line, sel string
	accent, alt, good, warn, bad    string
	storage, avb, sesh              string
}

// palettes are the twenty most popular editor themes, by installs across the
// VS Code marketplace and the JetBrains defaults, dark and light.
var palettes = []palette{
	{id: "dark-plus", name: "Dark+ (VS Code)", dark: true,
		bg: "#1E1E1E", fg: "#D4D4D4", muted: "#9D9D9D", faint: "#5A5A5A", line: "#3C3C3C", sel: "#264F78",
		accent: "#4EC9B0", alt: "#DCDCAA", good: "#89D185", warn: "#CCA700", bad: "#F14C4C",
		storage: "#569CD6", avb: "#CE9178", sesh: "#C586C0"},
	{id: "light-plus", name: "Light+ (VS Code)",
		bg: "#FFFFFF", fg: "#1F1F1F", muted: "#6E7681", faint: "#A0A0A0", line: "#E5E5E5", sel: "#ADD6FF",
		accent: "#267F99", alt: "#795E26", good: "#388A34", warn: "#BF8803", bad: "#E51400",
		storage: "#0070C1", avb: "#A31515", sesh: "#AF00DB"},
	{id: "one-dark-pro", name: "One Dark Pro", dark: true,
		bg: "#282C34", fg: "#ABB2BF", muted: "#7F848E", faint: "#5C6370", line: "#3E4452", sel: "#3E4451",
		accent: "#56B6C2", alt: "#E5C07B", good: "#98C379", warn: "#E5C07B", bad: "#E06C75",
		storage: "#61AFEF", avb: "#D19A66", sesh: "#C678DD"},
	{id: "dracula", name: "Dracula", dark: true,
		bg: "#282A36", fg: "#F8F8F2", muted: "#9EA3C0", faint: "#6272A4", line: "#44475A", sel: "#44475A",
		accent: "#50FA7B", alt: "#BD93F9", good: "#50FA7B", warn: "#F1FA8C", bad: "#FF5555",
		storage: "#8BE9FD", avb: "#FFB86C", sesh: "#FF79C6"},
	{id: "monokai", name: "Monokai", dark: true,
		bg: "#272822", fg: "#F8F8F2", muted: "#A59F85", faint: "#75715E", line: "#3E3D32", sel: "#49483E",
		accent: "#A6E22E", alt: "#F92672", good: "#A6E22E", warn: "#E6DB74", bad: "#F92672",
		storage: "#66D9EF", avb: "#FD971F", sesh: "#AE81FF"},
	{id: "darcula", name: "Darcula (JetBrains)", dark: true,
		bg: "#2B2B2B", fg: "#A9B7C6", muted: "#808080", faint: "#606366", line: "#3C3F41", sel: "#214283",
		accent: "#CC7832", alt: "#FFC66D", good: "#6A8759", warn: "#BBB529", bad: "#BC3F3C",
		storage: "#6897BB", avb: "#B5835A", sesh: "#9876AA"},
	{id: "solarized-dark", name: "Solarized Dark", dark: true,
		bg: "#002B36", fg: "#93A1A1", muted: "#839496", faint: "#586E75", line: "#073642", sel: "#073642",
		accent: "#2AA198", alt: "#6C71C4", good: "#859900", warn: "#B58900", bad: "#DC322F",
		storage: "#268BD2", avb: "#CB4B16", sesh: "#D33682"},
	{id: "solarized-light", name: "Solarized Light",
		bg: "#FDF6E3", fg: "#586E75", muted: "#657B83", faint: "#93A1A1", line: "#EEE8D5", sel: "#EEE8D5",
		accent: "#2AA198", alt: "#6C71C4", good: "#859900", warn: "#B58900", bad: "#DC322F",
		storage: "#268BD2", avb: "#CB4B16", sesh: "#D33682"},
	{id: "nord", name: "Nord", dark: true,
		bg: "#2E3440", fg: "#D8DEE9", muted: "#9AA3B5", faint: "#4C566A", line: "#3B4252", sel: "#434C5E",
		accent: "#88C0D0", alt: "#8FBCBB", good: "#A3BE8C", warn: "#EBCB8B", bad: "#BF616A",
		storage: "#81A1C1", avb: "#D08770", sesh: "#B48EAD"},
	{id: "gruvbox-dark", name: "Gruvbox Dark", dark: true,
		bg: "#282828", fg: "#EBDBB2", muted: "#A89984", faint: "#665C54", line: "#3C3836", sel: "#504945",
		accent: "#8EC07C", alt: "#FABD2F", good: "#B8BB26", warn: "#D79921", bad: "#FB4934",
		storage: "#83A598", avb: "#FE8019", sesh: "#D3869B"},
	{id: "tokyo-night", name: "Tokyo Night", dark: true,
		bg: "#1A1B26", fg: "#C0CAF5", muted: "#A9B1D6", faint: "#565F89", line: "#292E42", sel: "#283457",
		accent: "#73DACA", alt: "#7DCFFF", good: "#9ECE6A", warn: "#E0AF68", bad: "#F7768E",
		storage: "#7AA2F7", avb: "#FF9E64", sesh: "#BB9AF7"},
	{id: "catppuccin-mocha", name: "Catppuccin Mocha", dark: true,
		bg: "#1E1E2E", fg: "#CDD6F4", muted: "#A6ADC8", faint: "#6C7086", line: "#313244", sel: "#45475A",
		accent: "#94E2D5", alt: "#CBA6F7", good: "#A6E3A1", warn: "#F9E2AF", bad: "#F38BA8",
		storage: "#89B4FA", avb: "#FAB387", sesh: "#F5C2E7"},
	{id: "catppuccin-latte", name: "Catppuccin Latte",
		bg: "#EFF1F5", fg: "#4C4F69", muted: "#6C6F85", faint: "#9CA0B0", line: "#CCD0DA", sel: "#BCC0CC",
		accent: "#179299", alt: "#8839EF", good: "#40A02B", warn: "#DF8E1D", bad: "#D20F39",
		storage: "#1E66F5", avb: "#FE640B", sesh: "#EA76CB"},
	{id: "github-dark", name: "GitHub Dark", dark: true,
		bg: "#0D1117", fg: "#E6EDF3", muted: "#8B949E", faint: "#484F58", line: "#30363D", sel: "#21262D",
		accent: "#3FB950", alt: "#D2A8FF", good: "#3FB950", warn: "#D29922", bad: "#F85149",
		storage: "#79C0FF", avb: "#FFA657", sesh: "#F778BA"},
	{id: "github-light", name: "GitHub Light",
		bg: "#FFFFFF", fg: "#1F2328", muted: "#59636E", faint: "#8C959F", line: "#D0D7DE", sel: "#DDF4FF",
		accent: "#1A7F37", alt: "#8250DF", good: "#1A7F37", warn: "#9A6700", bad: "#CF222E",
		storage: "#0969DA", avb: "#BC4C00", sesh: "#BF3989"},
	{id: "night-owl", name: "Night Owl", dark: true,
		bg: "#011627", fg: "#D6DEEB", muted: "#8BA4BE", faint: "#4B6479", line: "#1D3B53", sel: "#1D3B53",
		accent: "#7FDBCA", alt: "#C792EA", good: "#ADDB67", warn: "#FFEB95", bad: "#EF5350",
		storage: "#82AAFF", avb: "#ECC48D", sesh: "#F78C6C"},
	{id: "palenight", name: "Material Palenight", dark: true,
		bg: "#292D3E", fg: "#A6ACCD", muted: "#959DCB", faint: "#676E95", line: "#3A3F58", sel: "#444267",
		accent: "#C3E88D", alt: "#C792EA", good: "#C3E88D", warn: "#FFCB6B", bad: "#FF5370",
		storage: "#82AAFF", avb: "#F78C6C", sesh: "#FF9CAC"},
	{id: "ayu-dark", name: "Ayu Dark", dark: true,
		bg: "#0B0E14", fg: "#BFBDB6", muted: "#8A8986", faint: "#565B66", line: "#1F2430", sel: "#273747",
		accent: "#AAD94C", alt: "#95E6CB", good: "#7FD962", warn: "#E6B450", bad: "#D95757",
		storage: "#59C2FF", avb: "#FF8F40", sesh: "#D2A6FF"},
	{id: "cobalt2", name: "Cobalt2", dark: true,
		bg: "#193549", fg: "#FFFFFF", muted: "#A0B5C9", faint: "#627E99", line: "#1F4662", sel: "#0050A4",
		accent: "#FFC600", alt: "#9EFFFF", good: "#3AD900", warn: "#FFEE80", bad: "#FF628C",
		storage: "#0088FF", avb: "#FF9D00", sesh: "#FB94FF"},
	{id: "rose-pine", name: "Rosé Pine", dark: true,
		bg: "#191724", fg: "#E0DEF4", muted: "#908CAA", faint: "#6E6A86", line: "#26233A", sel: "#403D52",
		accent: "#EBBCBA", alt: "#C4A7E7", good: "#9CCFD8", warn: "#F6C177", bad: "#EB6F92",
		storage: "#3E8FB0", avb: "#F6C177", sesh: "#C4A7E7"},
}

// ThemeIDs lists every theme by id, wits' own first.
func ThemeIDs() []string {
	ids := []string{autoTheme}
	for _, p := range palettes {
		ids = append(ids, p.id)
	}
	return ids
}

// ThemeNamed builds a theme by id. The auto theme needs the terminal's
// background to resolve; the editor themes bring their own and ignore it.
func ThemeNamed(id string, dark bool) (*Theme, error) {
	if id == "" || id == autoTheme {
		return NewTheme(dark), nil
	}
	for _, p := range palettes {
		if p.id == id {
			return p.theme(), nil
		}
	}
	return nil, fmt.Errorf("no theme called %q; the themes are %s", id, strings.Join(ThemeIDs(), ", "))
}

// theme resolves a palette into a Theme.
func (p palette) theme() *Theme {
	c := lipgloss.Color
	t := &Theme{ID: p.id, Dark: p.dark}
	t.Bg, t.Fg, t.Muted, t.Faint = c(p.bg), c(p.fg), c(p.muted), c(p.faint)
	t.Line, t.Selection = c(p.line), c(p.sel)
	t.Accent, t.Alt = c(p.accent), c(p.alt)
	t.Good, t.Warn, t.Bad = c(p.good), c(p.warn), c(p.bad)
	t.StorageC, t.StashC, t.AVBC, t.SeshC = c(p.storage), t.Accent, c(p.avb), c(p.sesh)

	// The heat ramp climbs from the background to the theme's green, starting
	// two fifths of the way so the dimmest shade still shows against the page.
	t.Heat = lipgloss.Blend1D(6, t.Bg, t.Good)[2:]
	t.style()
	return t
}

// formStyles dresses a dialog like the screen behind it. The auto theme keeps
// huh's own, which it was drawn beside.
func (t *Theme) formStyles(dark bool) *huh.Styles {
	if t.ID == autoTheme {
		return huh.ThemeCharm(dark)
	}
	s := huh.ThemeBase(t.Dark)
	f := &s.Focused
	f.Base = f.Base.BorderForeground(t.Line)
	f.Card = f.Base
	f.Title = f.Title.Foreground(t.Alt).Bold(true)
	f.NoteTitle = f.NoteTitle.Foreground(t.Alt).Bold(true).MarginBottom(1)
	f.Description = f.Description.Foreground(t.Muted)
	f.ErrorIndicator = f.ErrorIndicator.Foreground(t.Bad)
	f.ErrorMessage = f.ErrorMessage.Foreground(t.Bad)
	f.SelectSelector = f.SelectSelector.Foreground(t.Accent)
	f.NextIndicator = f.NextIndicator.Foreground(t.Accent)
	f.PrevIndicator = f.PrevIndicator.Foreground(t.Accent)
	f.Option = f.Option.Foreground(t.Fg)
	f.SelectedOption = f.SelectedOption.Foreground(t.Accent)
	f.UnselectedOption = f.UnselectedOption.Foreground(t.Fg)
	f.FocusedButton = f.FocusedButton.Foreground(t.Bg).Background(t.Accent)
	f.Next = f.FocusedButton
	f.BlurredButton = f.BlurredButton.Foreground(t.Fg).Background(t.Selection)
	f.TextInput.Cursor = f.TextInput.Cursor.Foreground(t.Accent)
	f.TextInput.Placeholder = f.TextInput.Placeholder.Foreground(t.Faint)
	f.TextInput.Prompt = f.TextInput.Prompt.Foreground(t.Accent)
	f.TextInput.Text = f.TextInput.Text.Foreground(t.Fg)

	s.Blurred = s.Focused
	s.Blurred.Base = s.Focused.Base.BorderStyle(lipgloss.HiddenBorder())
	s.Blurred.Card = s.Blurred.Base
	s.Blurred.NextIndicator = lipgloss.NewStyle()
	s.Blurred.PrevIndicator = lipgloss.NewStyle()
	s.Group.Title = f.Title
	s.Group.Description = f.Description
	return s
}

// dress gives a form the house layout and the theme. huh themes a field only
// once, so the form is handed a theme that asks the app for its current one at
// every frame — the picker previews themes on a form that is already open.
func (a *App) dress(f *huh.Form) *huh.Form {
	live := huh.ThemeFunc(func(dark bool) *huh.Styles { return a.theme.formStyles(dark) })
	return f.WithShowHelp(true).WithWidth(min(a.inner(), 72)).WithTheme(live)
}

// themePicker chooses a theme, previewing each one as the cursor lands on it.
type themePicker struct {
	form   *huh.Form
	choice string
	// was is the theme in force when the picker opened, restored on esc.
	was string
}

// newThemePicker lists every theme, the current one under the cursor. The
// list scrolls rather than standing taller than the terminal, so the dialog's
// confirmation can never fall off the bottom of the screen.
func newThemePicker(a *App) *themePicker {
	p := &themePicker{choice: a.theme.ID, was: a.theme.ID}
	opts := []huh.Option[string]{huh.NewOption("wits (follows the terminal)", autoTheme)}
	for _, pal := range palettes {
		opts = append(opts, huh.NewOption(pal.name, pal.id))
	}
	height := max(4, min(len(opts), a.height-12))
	p.form = a.dress(huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Theme").
			Description("The screen follows the cursor; enter keeps it, esc puts it back").
			Options(opts...).Height(height).Value(&p.choice),
	)))
	return p
}

// themeKey takes every key while the picker is open. huh only aborts on
// ctrl+c, so esc is caught here and puts back the theme the picker opened on.
func (a *App) themeKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if msg.String() == "esc" {
		a.useTheme(a.picker.was)
		a.picker, a.notice = nil, ""
		return true, nil
	}
	return true, a.pickTheme(msg)
}

// pickTheme drives the picker: every move previews, and completing it saves
// the choice to the repository's config. huh completes a form through a
// command of its own, so this sees every message, not only keys.
func (a *App) pickTheme(msg tea.Msg) tea.Cmd {
	p := a.picker
	model, cmd := p.form.Update(msg)
	if form, ok := model.(*huh.Form); ok {
		p.form = form
	}
	switch p.form.State {
	case huh.StateAborted:
		a.useTheme(p.was)
		a.picker = nil
		return nil
	case huh.StateCompleted:
		a.picker = nil
		a.useTheme(p.choice)
		a.notice, a.failed = fmt.Sprintf("theme %s", p.choice), false
		if a.data.Workspace == nil || a.data.Repo == nil {
			return nil
		}
		if err := a.data.Repo.SetTheme(p.choice); err != nil {
			a.notice, a.failed = err.Error(), true
		}
		return nil
	}
	a.useTheme(p.choice)
	return cmd
}

// useTheme switches to a theme by id, keeping the current one if the id is
// unknown — a config naming a theme this build lacks still opens.
func (a *App) useTheme(id string) {
	if t, err := ThemeNamed(id, a.dark); err == nil {
		a.theme = t
	}
}
