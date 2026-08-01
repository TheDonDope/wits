package tui

import (
	"image/color"
	"math"
	"strings"

	"charm.land/lipgloss/v2"
)

// blocks are the eighths used to give a bar sub-character resolution, so a
// difference of a tenth of a gram is still visible on a narrow terminal.
var blocks = []rune{' ', '▏', '▎', '▍', '▌', '▋', '▊', '▉', '█'}

// sparks are the eighths used vertically, for one-line series.
var sparks = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Gauge renders a horizontal bar of the given width filled to fraction, in
// eighths of a cell.
func Gauge(width int, fraction float64, fill tint, track lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	fraction = clamp(fraction, 0, 1)
	exact := fraction * float64(width)
	full := int(exact)
	rem := exact - float64(full)

	var b strings.Builder
	b.WriteString(strings.Repeat("█", full))
	if full < width {
		if eighth := int(rem*8 + 0.5); eighth > 0 {
			b.WriteRune(blocks[eighth])
			full++
		}
	}
	filled := lipgloss.NewStyle().Foreground(fill).Render(b.String())
	if rest := width - full; rest > 0 {
		return filled + track.Render(strings.Repeat("─", rest))
	}
	return filled
}

// GradientGauge is Gauge with the fill shaded from one colour to another along
// its length. The dashboard uses it for the cycle bar, running from the
// comfortable green into whatever colour the remaining fraction has earned, so
// a draining cycle looks like one.
func GradientGauge(width int, fraction float64, from, to tint, track lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	fraction = clamp(fraction, 0, 1)
	exact := fraction * float64(width)
	full := int(exact)
	rem := exact - float64(full)

	var b strings.Builder
	cells := full
	partial := ' '
	if full < width {
		if eighth := int(rem*8 + 0.5); eighth > 0 {
			partial = blocks[eighth]
			cells++
		}
	}
	for i := 0; i < cells; i++ {
		r := "█"
		if i == cells-1 && partial != ' ' {
			r = string(partial)
		}
		shade := lerp(from, to, float64(i)/math.Max(float64(cells-1), 1))
		b.WriteString(lipgloss.NewStyle().Foreground(shade).Render(r))
	}
	if rest := width - cells; rest > 0 {
		b.WriteString(track.Render(strings.Repeat("─", rest)))
	}
	return b.String()
}

// lerp blends two colours. The blend is in plain RGB, which is crude next to a
// perceptual space but indistinguishable at the dozen steps a terminal bar has.
func lerp(from, to tint, t float64) tint {
	fr, fg, fb, _ := from.RGBA()
	tr, tg, tb, _ := to.RGBA()
	mix := func(a, b uint32) uint8 {
		return uint8(uint32(float64(a>>8)*(1-t)+float64(b>>8)*t) & 0xFF)
	}
	return color.RGBA{R: mix(fr, tr), G: mix(fg, tg), B: mix(fb, tb), A: 0xFF}
}

// Sparkline renders a series as a single line of block characters. It is used
// where the shape of a run of days matters more than the exact figures.
func Sparkline(values []float64, width int, style lipgloss.Style) string {
	if width <= 0 || len(values) == 0 {
		return ""
	}
	values = fit(values, width)
	peak := maxOf(values)
	if peak <= 0 {
		return style.Render(strings.Repeat("·", len(values)))
	}
	var b strings.Builder
	for _, v := range values {
		if v <= 0 {
			// A day with nothing logged is not a very short bar, it is an
			// absence, and should not read as a small amount.
			b.WriteRune('·')
			continue
		}
		i := int(math.Round(v / peak * float64(len(sparks)-1)))
		b.WriteRune(sparks[i])
	}
	return style.Render(b.String())
}

// Bar is one row of a horizontal bar chart.
type Bar struct {
	Label string
	Value float64
	Note  string
	Color tint
}

// BarChart renders labelled horizontal bars scaled to the largest value. The
// label column is padded to a common width so the bars line up and can be
// compared by eye, which is the only reason to draw them at all.
func BarChart(bars []Bar, width int, t *Theme) string {
	if len(bars) == 0 {
		return t.Dim.Render("nothing to show yet")
	}
	labelW, noteW, peak := 0, 0, 0.0
	for _, b := range bars {
		labelW = maxInt(labelW, lipgloss.Width(b.Label))
		noteW = maxInt(noteW, lipgloss.Width(b.Note))
		peak = math.Max(peak, b.Value)
	}
	barW := width - labelW - noteW - 3
	if barW < 4 {
		barW = 4
	}

	var rows []string
	for _, b := range bars {
		fraction := 0.0
		if peak > 0 {
			fraction = b.Value / peak
		}
		colour := b.Color
		if colour == nil {
			colour = t.Accent
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Left,
			t.Label.Width(labelW).Render(b.Label),
			" ",
			Gauge(barW, fraction, colour, t.Dim),
			" ",
			t.Value.Width(noteW).Align(lipgloss.Right).Render(b.Note),
		))
	}
	return strings.Join(rows, "\n")
}

// Column is one bar of a vertical chart. A column may carry its own colour —
// the dashboard tints heavier days hotter — and falls back to the chart's fill
// when it does not.
type Column struct {
	Value float64
	Label string
	Color tint
}

// ColumnChart renders a vertical bar chart, which is how a run of daily amounts
// is easiest to read: time runs left to right, and the shape of a week is
// visible at a glance.
func ColumnChart(cols []Column, height int, t *Theme, fill tint) string {
	if len(cols) == 0 || height <= 0 {
		return t.Dim.Render("nothing to show yet")
	}
	peak := 0.0
	for _, c := range cols {
		peak = math.Max(peak, c.Value)
	}
	if peak <= 0 {
		return t.Dim.Render("nothing logged in this range")
	}

	styles := make([]lipgloss.Style, len(cols))
	for i, c := range cols {
		colour := c.Color
		if colour == nil {
			colour = fill
		}
		styles[i] = lipgloss.NewStyle().Foreground(colour)
	}
	grid := make([][]string, height)
	for row := range grid {
		grid[row] = make([]string, len(cols))
		// Rows are filled from the top down, so this row represents the band
		// between these two fractions of the maximum.
		upper := float64(height-row) / float64(height)
		lower := float64(height-row-1) / float64(height)
		for i, c := range cols {
			style := styles[i]
			fraction := c.Value / peak
			switch {
			case fraction >= upper:
				grid[row][i] = style.Render("█")
			case fraction <= lower:
				grid[row][i] = " "
			default:
				eighth := int((fraction-lower)*float64(height)*8 + 0.5)
				if eighth < 1 {
					eighth = 1
				}
				if eighth > 8 {
					eighth = 8
				}
				grid[row][i] = style.Render(string(sparks[eighth-1]))
			}
		}
	}

	var b strings.Builder
	for row := range grid {
		b.WriteString(strings.Join(grid[row], ""))
		b.WriteByte('\n')
	}
	b.WriteString(t.Dim.Render(strings.Repeat("─", len(cols))))
	return b.String()
}

// Stack renders one bar split between accounts, scaled against total rather
// than against its own sum. Scaling each bar to itself would make every row the
// full width, which says nothing: the point is to compare one product's
// remaining supply against another's.
func Stack(width int, parts []Bar, total float64, t *Theme) string {
	if width <= 0 {
		return ""
	}
	if total <= 0 {
		return t.Dim.Render(strings.Repeat("─", width))
	}
	var b strings.Builder
	used := 0
	for _, p := range parts {
		w := int(p.Value / total * float64(width))
		if w <= 0 || used+w > width {
			w = minInt(w, width-used)
		}
		if w <= 0 {
			continue
		}
		used += w
		b.WriteString(lipgloss.NewStyle().Foreground(p.Color).Render(strings.Repeat("█", w)))
	}
	if rest := width - used; rest > 0 {
		b.WriteString(t.Dim.Render(strings.Repeat("─", rest)))
	}
	return b.String()
}

// fit reduces a series to at most width points by averaging neighbours, so a
// long cycle still reads as a shape rather than being cut off.
func fit(values []float64, width int) []float64 {
	if len(values) <= width {
		return values
	}
	out := make([]float64, width)
	per := float64(len(values)) / float64(width)
	for i := range out {
		start := int(float64(i) * per)
		end := int(float64(i+1) * per)
		if end > len(values) {
			end = len(values)
		}
		if end <= start {
			end = start + 1
		}
		var sum float64
		for _, v := range values[start:end] {
			sum += v
		}
		out[i] = sum / float64(end-start)
	}
	return out
}

func maxOf(values []float64) float64 {
	m := 0.0
	for _, v := range values {
		m = math.Max(m, v)
	}
	return m
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp(v, lo, hi float64) float64 {
	return math.Min(math.Max(v, lo), hi)
}

// axisLabels puts the start and end of a range at either end of a chart, or on
// one line together when the chart is too narrow to separate them. A fixed gap
// would bunch them up under a short range and read as one date.
func axisLabels(t *Theme, left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return t.Dim.Render(left + " → " + right)
	}
	return t.Dim.Render(left) + strings.Repeat(" ", gap) + t.Dim.Render(right)
}

// round trims to centigrams, matching what the ledger stores and what a
// jeweller's scale reads.
func round(g float64) float64 { return math.Round(g*100) / 100 }
