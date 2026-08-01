package tui

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

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

	// The month labels go where a month begins, so the columns can be dated
	// without an axis. A label is skipped when the previous one would still be
	// under it: a cramped chart with legible labels beats a complete ruler.
	months := make([]rune, weeks)
	for i := range months {
		months[i] = ' '
	}
	lastLabel := -3
	for week := 0; week < weeks; week++ {
		monday := start.AddDate(0, 0, week*7)
		if (week == 0 || monday.Day() <= 7) && week-lastLabel >= 3 {
			for i, r := range monday.Format("Jan") {
				if week+i < weeks {
					months[week+i] = r
				}
			}
			lastLabel = week
		}
	}

	rows := []string{strings.Repeat(" ", gutter) + t.Dim.Render(string(months))}
	labels := map[int]string{0: "Mon", 2: "Wed", 4: "Fri"}
	for weekday := 0; weekday < 7; weekday++ {
		var b strings.Builder
		b.WriteString(t.Dim.Render(padTo(labels[weekday], gutter)))
		for week := 0; week < weeks; week++ {
			day := start.AddDate(0, 0, week*7+weekday)
			switch v := perDay[day.Format(time.DateOnly)]; {
			case day.After(to) || day.Before(from):
				b.WriteByte(' ')
			case v <= 0:
				b.WriteString(t.Dim.Render("·"))
			default:
				b.WriteString(lipgloss.NewStyle().Foreground(heatAt(t, v/peak)).Render("■"))
			}
		}
		rows = append(rows, b.String())
	}

	legend := t.Dim.Render("less ")
	for _, shade := range t.Heat {
		legend += lipgloss.NewStyle().Foreground(shade).Render("■")
	}
	legend += t.Dim.Render(" more")
	rows = append(rows, "", legend)

	return strings.Join(rows, "\n")
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
