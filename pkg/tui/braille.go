package tui

import (
	"math"
	"strings"

	"charm.land/lipgloss/v2"
)

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

	// Heights in dots, from the bottom. A day with something logged is never
	// rounded away entirely: one dot is the difference between a light day and
	// an absence, and the absence is the one that must stay blank.
	subH := height * 4
	fillTop := make([]int, points)
	lineDot := make([]int, points)
	for i, v := range vals {
		if v > 0 {
			fillTop[i] = max(int(math.Round(v/peak*float64(subH))), 1)
		}
		lineDot[i] = -1
		if avg != nil && avg[i] > 0 {
			lineDot[i] = min(max(int(math.Round(avg[i]/peak*float64(subH))), 1), subH) - 1
		}
	}

	fillStyle := lipgloss.NewStyle().Foreground(fill)
	lineStyle := lipgloss.NewStyle().Foreground(line)

	var b strings.Builder
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			var filled, lined rune
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
			switch {
			case lined != 0:
				// The cell belongs to the line where the two meet: the average
				// is the annotation, and an annotation that vanishes into what
				// it annotates is not one.
				b.WriteString(lineStyle.Render(string(brailleBlank | lined | filled)))
			case filled != 0:
				b.WriteString(fillStyle.Render(string(brailleBlank + filled)))
			default:
				b.WriteByte(' ')
			}
		}
		b.WriteByte('\n')
	}
	b.WriteString(t.Dim.Render(strings.Repeat("─", width)))
	return b.String()
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
