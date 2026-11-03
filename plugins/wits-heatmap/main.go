// Package main is wits-heatmap, a plugin run as `wits heatmap`: the
// sessions of the last weeks as a calendar, a column a week and a row a
// weekday, each day shaded by the grams it took.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/TheDonDope/wits/plugins/internal/wits"
)

func main() {
	weeks := flag.Int("weeks", 52, "how many weeks back, this one included")
	count := flag.Bool("count", false, "shade each day by its sessions rather than its grams")
	flag.Parse()
	if *weeks < 1 {
		wits.Fail(fmt.Errorf("--weeks %d: one or more", *weeks))
	}
	var sessions []wits.Session
	if err := wits.Get(&sessions, "sessions"); err != nil {
		wits.Fail(err)
	}
	render(os.Stdout, sessions, time.Now(), *weeks, *count)
}

// shades go from no session to the busiest quarter of days.
var shades = []string{"·", "░", "▒", "▓", "█"}

// calendar is the window the map covers: from a Monday to today.
type calendar struct {
	start, today time.Time
	weeks        int
	day          map[string]float64
	// sessions and grams are the window's, whichever the map is shaded by.
	sessions int
	grams    float64
}

// day names a date the way the calendar keys it.
func day(t time.Time) string { return t.Format(time.DateOnly) }

// newCalendar sums each day of the window, by grams or by sessions.
func newCalendar(sessions []wits.Session, now time.Time, weeks int, byCount bool) calendar {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	monday := today.AddDate(0, 0, -((int(today.Weekday()) + 6) % 7))
	c := calendar{start: monday.AddDate(0, 0, -7*(weeks-1)), today: today, weeks: weeks, day: map[string]float64{}}
	for _, s := range sessions {
		at := s.At.In(time.Local)
		if at.Before(c.start) || at.After(now) {
			continue
		}
		c.sessions++
		c.grams += s.Grams
		if byCount {
			c.day[day(at)]++
		} else {
			c.day[day(at)] += s.Grams
		}
	}
	return c
}

// quartiles are the bounds of the shades, from the days that had a session.
func (c calendar) quartiles() [3]float64 {
	var used []float64
	for _, v := range c.day {
		used = append(used, v)
	}
	if len(used) == 0 {
		return [3]float64{}
	}
	slices.Sort(used)
	at := func(q float64) float64 { return used[int(q*float64(len(used)-1))] }
	return [3]float64{at(0.25), at(0.5), at(0.75)}
}

// shade is the cell for one day's amount.
func shade(v float64, q [3]float64) string {
	switch {
	case v == 0:
		return shades[0]
	case v <= q[0]:
		return shades[1]
	case v <= q[1]:
		return shades[2]
	case v <= q[2]:
		return shades[3]
	}
	return shades[4]
}

// months is the line above the map: a month's name over the week it starts.
func (c calendar) months() string {
	line := []rune(strings.Repeat(" ", c.weeks+3))
	free := 0
	for w := range c.weeks {
		monday := c.start.AddDate(0, 0, 7*w)
		// A month is named over its first week; the first column only
		// when that is where the month begins, or the next would crowd it.
		starts := monday.AddDate(0, 0, -7).Month() != monday.Month()
		if w < free || (w == 0 && monday.Day() > 7) || (w > 0 && !starts) {
			continue
		}
		copy(line[w:], []rune(monday.Format("Jan")))
		free = w + 4
	}
	return strings.TrimRight(string(line), " ")
}

// render prints the map, its legend and what it adds up to.
func render(out io.Writer, sessions []wits.Session, now time.Time, weeks int, byCount bool) {
	c := newCalendar(sessions, now, weeks, byCount)
	q := c.quartiles()
	fmt.Fprintf(out, "    %s\n", c.months())
	for wd, label := range []string{"Mon", "", "Wed", "", "Fri", "", "Sun"} {
		var row strings.Builder
		for w := range c.weeks {
			d := c.start.AddDate(0, 0, 7*w+wd)
			if d.After(c.today) {
				break
			}
			row.WriteString(shade(c.day[day(d)], q))
		}
		fmt.Fprintf(out, "%-3s %s\n", label, row.String())
	}
	unit := "grams"
	if byCount {
		unit = "sessions"
	}
	fmt.Fprintf(out, "\n    less %s more (%s a day)\n\n", strings.Join(shades, " "), unit)
	summary(out, c)
}

// summary says what the map adds up to: sessions, grams, the days used and
// the longest run of days without a session.
func summary(out io.Writer, c calendar) {
	days, used, gap, longest := 0, 0, 0, 0
	for d := c.start; !d.After(c.today); d = d.AddDate(0, 0, 1) {
		days++
		if c.day[day(d)] > 0 {
			used, gap = used+1, 0
			continue
		}
		gap++
		longest = max(longest, gap)
	}
	fmt.Fprintf(out, "%d sessions, %.2f g in %d weeks: a session on %d of %d days; the longest break, %d days.\n",
		c.sessions, c.grams, c.weeks, used, days, longest)
}
