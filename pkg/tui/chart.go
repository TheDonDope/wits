package tui

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"time"

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
		labelW = max(labelW, lipgloss.Width(b.Label))
		noteW = max(noteW, lipgloss.Width(b.Note))
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
			w = min(w, width-used)
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

// The area chart is drawn in braille rather than blocks. A braille cell is a
// 2×4 grid of dots, so each column of text carries two points of the series at
// four times the vertical resolution of a block character — enough for the
// shape of a whole cycle to survive a narrow terminal.

// brailleBits maps a dot at (row, column) inside one cell to its bit in the
// braille block, rows top to bottom. The layout is historical rather than
// obvious: dots 1–3 and 7 stack in the left column, 4–6 and 8 in the right.
var brailleBits = [4][2]rune{
	{0x01, 0x08},
	{0x02, 0x10},
	{0x04, 0x20},
	{0x40, 0x80},
}

const brailleBlank = rune(0x2800)

// AreaChart renders a series as a filled braille area, with an optional second
// series — conventionally a moving average — drawn over it as a line. The two
// are scaled together, so the line sits where it belongs against the fill.
func AreaChart(values, average []float64, width, height int, t *Theme, fill, line tint) string {
	if width <= 0 || height <= 0 || len(values) == 0 {
		return t.Dim.Render("nothing to show yet")
	}
	points := width * 2
	vals := resample(values, points)
	var avg []float64
	if len(average) > 0 {
		avg = resample(average, points)
	}
	peak := math.Max(maxOf(vals), maxOf(avg))
	if peak <= 0 {
		return t.Dim.Render("nothing logged in this range")
	}
	fillTop, lineDot := brailleHeights(vals, avg, peak, height*4)

	fillStyle := lipgloss.NewStyle().Foreground(fill)
	lineStyle := lipgloss.NewStyle().Foreground(line)
	var b strings.Builder
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			filled, lined := brailleCell(row, col, height, fillTop, lineDot)
			switch {
			case lined != 0:
				// The cell belongs to the line where the two meet: the average
				// is the annotation, and an annotation that vanishes into what
				// it annotates is not one.
				b.WriteString(lineStyle.Render(string(brailleBlank | lined | filled)))
			case filled != 0:
				b.WriteString(fillStyle.Render(string(brailleBlank | filled)))
			default:
				b.WriteByte(' ')
			}
		}
		b.WriteByte('\n')
	}
	b.WriteString(t.Dim.Render(strings.Repeat("─", width)))
	return b.String()
}

// brailleHeights turns the two series into dot heights from the chart floor. A
// day with something logged is never rounded away entirely: one dot is the
// difference between a light day and an absence, and the absence is the one
// that must stay blank.
func brailleHeights(vals, avg []float64, peak float64, subH int) (fillTop, lineDot []int) {
	fillTop = make([]int, len(vals))
	lineDot = make([]int, len(vals))
	for i, v := range vals {
		if v > 0 {
			fillTop[i] = max(int(math.Round(v/peak*float64(subH))), 1)
		}
		lineDot[i] = -1
		if avg != nil && avg[i] > 0 {
			lineDot[i] = min(max(int(math.Round(avg[i]/peak*float64(subH))), 1), subH) - 1
		}
	}
	return fillTop, lineDot
}

// brailleCell collects the dots of one cell: the bits of the area fill and the
// bits of the average line crossing it.
func brailleCell(row, col, height int, fillTop, lineDot []int) (filled, lined rune) {
	for sub := 0; sub < 4; sub++ {
		// The dot's height counted from the chart floor.
		fromBottom := (height-1-row)*4 + (3 - sub)
		for side := 0; side < 2; side++ {
			i := col*2 + side
			if fromBottom < fillTop[i] {
				filled |= brailleBits[sub][side]
			}
			if fromBottom == lineDot[i] {
				lined |= brailleBits[sub][side]
			}
		}
	}
	return filled, lined
}

// movingAverage smooths a series over a trailing window. The first days of a
// range are averaged over what exists rather than padded with zeroes, which
// would invent a ramp-up that never happened.
func movingAverage(values []float64, window int) []float64 {
	if window <= 1 || len(values) == 0 {
		return values
	}
	out := make([]float64, len(values))
	sum := 0.0
	for i, v := range values {
		sum += v
		if i >= window {
			sum -= values[i-window]
		}
		out[i] = sum / float64(min(i+1, window))
	}
	return out
}

// resample stretches or squeezes a series to exactly n points. Squeezing
// averages the days a point covers, so a long history keeps its shape;
// stretching repeats them, so a short one reads as steps rather than gaining
// detail it does not have.
func resample(values []float64, n int) []float64 {
	if n <= 0 || len(values) == 0 {
		return nil
	}
	out := make([]float64, n)
	per := float64(len(values)) / float64(n)
	for i := range out {
		start := int(float64(i) * per)
		end := int(float64(i+1) * per)
		if end <= start {
			end = start + 1
		}
		if end > len(values) {
			end = len(values)
		}
		if start >= len(values) {
			start = len(values) - 1
		}
		sum := 0.0
		for _, v := range values[start:end] {
			sum += v
		}
		out[i] = sum / float64(end-start)
	}
	return out
}

// Calendar renders a run of days as a heatmap: one column per week, one row
// per weekday, colour carrying the amount. It is the shape a year of habit is
// easiest to read in — the heavy weeks, the pauses, whether weekends differ
// from weekdays — none of which a bar per day can say once the days outnumber
// the columns of the terminal.
func Calendar(perDay map[string]float64, from, to time.Time, width int, t *Theme) string {
	const gutter = 4 // room for the weekday labels
	if width <= gutter+1 || to.Before(from) {
		return t.Dim.Render("nothing to show yet")
	}

	start := mondayOf(from)
	weeks := daysBetween(start, to)/7 + 1
	if room := width - gutter; weeks > room {
		// Too long to fit: the most recent weeks win, since they are the ones
		// a decision would be made from.
		weeks = room
		start = mondayOf(to).AddDate(0, 0, -7*(weeks-1))
	}

	peak := 0.0
	for d := start; !d.After(to); d = d.AddDate(0, 0, 1) {
		if v := perDay[d.Format(time.DateOnly)]; v > peak {
			peak = v
		}
	}
	if peak <= 0 {
		return t.Dim.Render("nothing logged in this range")
	}

	rows := []string{strings.Repeat(" ", gutter) + t.Dim.Render(monthLabels(start, weeks))}
	labels := map[int]string{0: "Mon", 2: "Wed", 4: "Fri"}
	for weekday := 0; weekday < 7; weekday++ {
		var b strings.Builder
		b.WriteString(t.Dim.Render(padTo(labels[weekday], gutter)))
		for week := 0; week < weeks; week++ {
			day := start.AddDate(0, 0, week*7+weekday)
			b.WriteString(calendarCell(t, perDay[day.Format(time.DateOnly)], peak, day, from, to))
		}
		rows = append(rows, b.String())
	}
	rows = append(rows, "", heatLegend(t))
	return strings.Join(rows, "\n")
}

// monthLabels writes each month's name and its two-digit year where the month
// begins — "Jan 26", so two Januaries a year apart cannot be confused. A label
// is skipped when the previous one would still be under it: a cramped chart
// with legible labels beats a complete ruler.
func monthLabels(start time.Time, weeks int) string {
	months := make([]rune, weeks)
	for i := range months {
		months[i] = ' '
	}
	lastLabel := -8
	for week := 0; week < weeks; week++ {
		monday := start.AddDate(0, 0, week*7)
		if (week == 0 || monday.Day() <= 7) && week-lastLabel >= 8 {
			for i, r := range monday.Format("Jan 06") {
				if week+i < weeks {
					months[week+i] = r
				}
			}
			lastLabel = week
		}
	}
	return string(months)
}

// calendarCell renders one day: blank outside the range, a dot for an absence,
// and a heat-shaded block for anything logged.
func calendarCell(t *Theme, v, peak float64, day, from, to time.Time) string {
	switch {
	case day.After(to) || day.Before(from):
		return " "
	case v <= 0:
		return t.Dim.Render("·")
	default:
		return lipgloss.NewStyle().Foreground(heatAt(t, v/peak)).Render("■")
	}
}

// heatLegend is the less-to-more ramp under the calendar.
func heatLegend(t *Theme) string {
	legend := t.Dim.Render("less ")
	for _, shade := range t.Heat {
		legend += lipgloss.NewStyle().Foreground(shade).Render("■")
	}
	return legend + t.Dim.Render(" more")
}

// heatAt picks the shade for a share of the peak, in even steps. The scale is
// linear because the readings are: a 2 g day is twice a 1 g day, and the chart
// should not editorialise beyond that.
func heatAt(t *Theme, fraction float64) tint {
	i := int(fraction * float64(len(t.Heat)))
	if i >= len(t.Heat) {
		i = len(t.Heat) - 1
	}
	if i < 0 {
		i = 0
	}
	return t.Heat[i]
}

// mondayOf returns the Monday on or before a day, which is where a calendar
// column starts.
func mondayOf(d time.Time) time.Time {
	shift := (int(d.Weekday()) + 6) % 7
	y, m, day := d.AddDate(0, 0, -shift).Date()
	return time.Date(y, m, day, 0, 0, 0, 0, d.Location())
}

// padTo extends a label with spaces to a fixed width.
func padTo(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
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

// dateAxis writes a run of dates under a chart, evenly spaced: the ends
// anchored left and right, the middles centred, and any label that would
// collide with its neighbour dropped rather than smudged. Two dates say how
// long a chart is; four or five say where in it a spike lives.
func dateAxis(t *Theme, first, last time.Time, width int) string {
	return t.Dim.Render(string(dateAxisRunes(first, last, width)))
}

// dateAxisRunes is the unstyled tick row, shared by the plain axis and the
// one carrying a cursor.
func dateAxisRunes(first, last time.Time, width int) []rune {
	span := int(last.Sub(first).Hours() / 24)
	layout := "02 Jan"
	if span > 270 {
		layout = "Jan 06"
	}
	ticks := min(max(width/22, 2), 6)

	row := []rune(strings.Repeat(" ", width))
	place := func(label string, at int) {
		for i, r := range label {
			if p := at + i; p >= 0 && p < width && row[p] == ' ' &&
				(p == 0 || row[p-1] == ' ' || i > 0) {
				row[p] = r
			} else if i == 0 {
				return
			}
		}
	}
	for i := 0; i < ticks; i++ {
		day := first.AddDate(0, 0, span*i/max(ticks-1, 1))
		label := day.Format(layout)
		at := (width - len(label)) * i / max(ticks-1, 1)
		place(label, at)
	}
	return row
}

// axisGutter is the width of the y-axis labels beside a scaled area chart.
const axisGutter = 8

// AreaWithAxis is AreaChart with a y-axis: the peak at the top, the midpoint
// halfway down, zero on the baseline — so a spike has a size, not only a
// shape. The date row under it should be indented by the same gutter.
func AreaWithAxis(values, average []float64, width, height int, t *Theme, fill, line tint) string {
	chartW := max(width-axisGutter, 12)
	chart := AreaChart(values, average, chartW, height, t, fill, line)
	peak := math.Max(maxOf(resample(values, chartW*2)), maxOf(resample(average, chartW*2)))
	if peak <= 0 {
		return chart
	}

	rows := strings.Split(chart, "\n")
	label := func(v float64, joint string) string {
		return t.Dim.Render(fmt.Sprintf("%6.1f %s", v, joint))
	}
	for i := range rows {
		switch {
		case i == 0:
			rows[i] = label(peak, "┤") + rows[i]
		case i == len(rows)-1:
			rows[i] = label(0, "┴") + rows[i]
		case i == (len(rows)-1)/2:
			rows[i] = label(peak*float64(len(rows)-1-i)/float64(len(rows)-1), "┤") + rows[i]
		default:
			rows[i] = strings.Repeat(" ", axisGutter-1) + t.Dim.Render("│") + rows[i]
		}
	}
	return strings.Join(rows, "\n")
}

// Jar renders a container filled to a fraction, the way a jar on a shelf
// holds what is left of it: walls, a lid line, and the contents rising from
// the bottom in eighths of a row, so a tenth of a gram still shows. The
// fraction is clamped rather than trusted, because a reconciliation can put
// more into an account than was ever bought into it.
func Jar(width, height int, fraction float64, fill tint, t *Theme) []string {
	if width < 4 {
		width = 4
	}
	if height < 1 {
		height = 1
	}
	fraction = clamp(fraction, 0, 1)
	inner := width - 2
	eighths := int(math.Round(fraction * float64(height) * 8))
	style := lipgloss.NewStyle().Foreground(fill)

	rows := make([]string, 0, height+2)
	rows = append(rows, t.Dim.Render("╭"+strings.Repeat("─", inner)+"╮"))
	for row := height - 1; row >= 0; row-- {
		// The eighths this row holds: full below the level, empty above it,
		// and one partial row where the surface sits.
		have := min(max(eighths-row*8, 0), 8)
		var body string
		switch {
		case have == 8:
			body = style.Render(strings.Repeat("█", inner))
		case have > 0:
			body = style.Render(strings.Repeat(string(sparks[have-1]), inner))
		default:
			body = strings.Repeat(" ", inner)
		}
		rows = append(rows, t.Dim.Render("│")+body+t.Dim.Render("│"))
	}
	rows = append(rows, t.Dim.Render("╰"+strings.Repeat("─", inner)+"╯"))
	return rows
}
