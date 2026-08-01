package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

func TestAreaChartDimensions(t *testing.T) {
	th := NewTheme(true)
	values := []float64{0, 1, 2, 3, 2, 1, 0.5}

	out := stripANSI(AreaChart(values, nil, 20, 4, th, th.StashC, th.Alt))
	lines := strings.Split(out, "\n")

	assert.Len(t, lines, 5, "Should be the chart rows plus the baseline")
	for _, line := range lines {
		assert.LessOrEqual(t, lipgloss.Width(line), 20, "Should stay inside the width")
	}
}

func TestAreaChartSaysWhenNothingHappened(t *testing.T) {
	th := NewTheme(true)

	out := stripANSI(AreaChart([]float64{0, 0, 0}, nil, 20, 4, th, th.StashC, th.Alt))

	assert.Contains(t, out, "nothing logged", "Should say a range is empty rather than drawing a flat line")
}

func TestAreaChartKeepsALightDayVisible(t *testing.T) {
	th := NewTheme(true)
	// One heavy day next to a very light one: the light one must still get a dot.
	out := stripANSI(AreaChart([]float64{100, 0.01}, nil, 1, 4, th, th.StashC, th.Alt))

	row := strings.Split(out, "\n")
	bottom := []rune(row[3])
	assert.NotEqual(t, brailleBlank, bottom[0]|brailleBlank, "The bottom row should carry both days")
}

func TestMovingAverage(t *testing.T) {
	avg := movingAverage([]float64{2, 4, 6}, 2)

	assert.Equal(t, []float64{2, 3, 5}, avg, "Should average over the window, and over what exists before it fills")
}

func TestResample(t *testing.T) {
	down := resample([]float64{1, 1, 3, 3}, 2)
	assert.Equal(t, []float64{1, 3}, down, "Squeezing should average the days a point covers")

	up := resample([]float64{1, 3}, 4)
	assert.Equal(t, []float64{1, 1, 3, 3}, up, "Stretching should repeat rather than invent")
}

func TestCalendarShowsMonthsAndLegend(t *testing.T) {
	th := NewTheme(true)
	perDay := map[string]float64{}
	from := time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 2, 0)
	for d := from; !d.After(to); d = d.AddDate(0, 0, 2) {
		perDay[d.Format(time.DateOnly)] = 1.5
	}

	out := stripANSI(Calendar(perDay, from, to, 60, th))

	assert.Contains(t, out, "Jan", "Should label the months")
	assert.Contains(t, out, "Mon", "Should label the weekdays")
	assert.Contains(t, out, "less", "Should carry the legend")
	assert.Contains(t, out, "■", "Should draw the days that hold something")
	assert.Contains(t, out, "·", "Should mark the empty days as absences")
}

func TestCalendarClampsToTheWidth(t *testing.T) {
	th := NewTheme(true)
	perDay := map[string]float64{}
	from := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	perDay[to.Format(time.DateOnly)] = 1

	out := stripANSI(Calendar(perDay, from, to, 40, th))

	for _, line := range strings.Split(out, "\n") {
		assert.LessOrEqual(t, lipgloss.Width(line), 40, "Six years must not overflow forty columns")
	}
}

func TestGradientGaugeWidth(t *testing.T) {
	th := NewTheme(true)

	assert.Equal(t, 10, lipgloss.Width(GradientGauge(10, 0, th.Good, th.Bad, th.Dim)), "Should be full width when empty")
	assert.Equal(t, 10, lipgloss.Width(GradientGauge(10, 0.5, th.Good, th.Bad, th.Dim)), "Should be full width when half full")
	assert.Equal(t, 10, lipgloss.Width(GradientGauge(10, 1, th.Good, th.Bad, th.Dim)), "Should be full width when full")
	assert.NotContains(t, stripANSI(GradientGauge(10, 1, th.Good, th.Bad, th.Dim)), "─", "Should have no track left when full")
}

func TestColumnChartHonoursAColumnColour(t *testing.T) {
	th := NewTheme(true)

	out := ColumnChart([]Column{
		{Value: 1, Color: th.Bad},
		{Value: 2},
	}, 2, th, th.Good)

	assert.NotEmpty(t, out, "Should render columns that carry their own colour")
}
