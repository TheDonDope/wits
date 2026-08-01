package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Theme holds every colour and style the interface uses.
//
// Lip Gloss v2 removed AdaptiveColor, so light and dark are resolved once here
// against the terminal's reported background rather than at each call site.
type Theme struct {
	Dark bool

	// Palette
	Fg, Muted, Faint tint
	Accent, Alt      tint
	Good, Warn, Bad  tint
	Line             tint
	StorageC, StashC tint
	AVBC, SeshC      tint
	Heat             []tint

	// Chrome
	Title, Subtitle   lipgloss.Style
	Tab, TabActive    lipgloss.Style
	TabGap            lipgloss.Style
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

// NewTheme builds the theme for a light or dark terminal.
func NewTheme(dark bool) *Theme {
	pick := lipgloss.LightDark(dark)
	t := &Theme{Dark: dark}

	t.Fg = pick(lipgloss.Color("#1A1A1A"), lipgloss.Color("#EEEEEE"))
	t.Muted = pick(lipgloss.Color("#6C6C6C"), lipgloss.Color("#9A9A9A"))
	t.Faint = pick(lipgloss.Color("#B0B0B0"), lipgloss.Color("#4A4A4A"))
	t.Accent = pick(lipgloss.Color("#00A36A"), lipgloss.Color("#02BF87"))
	t.Alt = pick(lipgloss.Color("#5A56E0"), lipgloss.Color("#7571F9"))
	t.Good = pick(lipgloss.Color("#02BA84"), lipgloss.Color("#02BF87"))
	t.Warn = pick(lipgloss.Color("#B7791F"), lipgloss.Color("#E3B341"))
	t.Bad = pick(lipgloss.Color("#D6336C"), lipgloss.Color("#FE5F86"))
	t.Line = pick(lipgloss.Color("#DDDDDD"), lipgloss.Color("#3A3A3A"))

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

	t.Title = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
	t.Subtitle = lipgloss.NewStyle().Foreground(t.Muted)
	t.Tab = lipgloss.NewStyle().Foreground(t.Muted).Padding(0, 2)
	t.TabActive = lipgloss.NewStyle().Bold(true).Foreground(t.Accent).
		Padding(0, 2).Underline(true)
	t.TabGap = lipgloss.NewStyle().Foreground(t.Line)
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
	return t
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
