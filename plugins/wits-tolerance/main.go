// Package main is wits-tolerance, a plugin run as `wits tolerance`: the
// grams a session takes, month by month, and whether the last three months
// take more than the three before them — tolerance creeping up.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/TheDonDope/wits/plugins/internal/wits"
)

func main() {
	months := flag.Int("months", 12, "how many months back, this one included")
	threshold := flag.Float64("threshold", 15, "the rise, in percent, that counts as creeping")
	flag.Parse()
	if *months < 1 {
		wits.Fail(fmt.Errorf("--months %d: one or more", *months))
	}
	var sessions []wits.Session
	if err := wits.Get(&sessions, "sessions"); err != nil {
		wits.Fail(err)
	}
	render(os.Stdout, tally(sessions, time.Now(), *months), *threshold)
}

// month is what one calendar month's sessions took.
type month struct {
	start    time.Time
	sessions int
	grams    float64
}

// perSession is the month's grams a session, or 0 when it had none.
func (m month) perSession() float64 {
	if m.sessions == 0 {
		return 0
	}
	return m.grams / float64(m.sessions)
}

// tally sums the sessions of the last n months, oldest first.
func tally(sessions []wits.Session, now time.Time, n int) []month {
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local).AddDate(0, -(n - 1), 0)
	ms := make([]month, n)
	for i := range ms {
		ms[i].start = first.AddDate(0, i, 0)
	}
	for _, s := range sessions {
		at := s.At.In(time.Local)
		i := (at.Year()-first.Year())*12 + int(at.Month()) - int(first.Month())
		if i < 0 || i >= n {
			continue
		}
		ms[i].sessions++
		ms[i].grams += s.Grams
	}
	return ms
}

// render prints a row a month and then the verdict.
func render(out io.Writer, ms []month, threshold float64) {
	widest := 0.0
	for _, m := range ms {
		widest = max(widest, m.perSession())
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "MONTH\tSESSIONS\tGRAMS\tG/SESSION\tCHANGE")
	for i, m := range ms {
		change := "-"
		if i > 0 && ms[i-1].perSession() > 0 && m.sessions > 0 {
			change = fmt.Sprintf("%+.0f%%", rise(ms[i-1].perSession(), m.perSession()))
		}
		fmt.Fprintf(w, "%s\t%d\t%.2f g\t%.3f g\t%s\t%s\n", m.start.Format("2006-01"), m.sessions, m.grams,
			m.perSession(), change, bar(m.perSession(), widest))
	}
	w.Flush()
	fmt.Fprintln(out)
	fmt.Fprintln(out, verdict(ms, threshold))
}

// bar draws a month's grams a session against the largest.
func bar(v, widest float64) string {
	if widest == 0 {
		return ""
	}
	return strings.Repeat("█", int(v/widest*20+0.5))
}

// rise is how much b is above a, in percent.
func rise(a, b float64) float64 { return (b/a - 1) * 100 }

// verdict compares the grams a session of the last three months with the
// three before them.
func verdict(ms []month, threshold float64) string {
	if len(ms) < 6 {
		return "Six months or more are needed to compare three with three: --months 6."
	}
	recent, before := pooled(ms[len(ms)-3:]), pooled(ms[len(ms)-6:len(ms)-3])
	if recent == 0 || before == 0 {
		return "Too few sessions in the last six months to compare."
	}
	r := rise(before, recent)
	switch {
	case r >= threshold:
		return fmt.Sprintf("A session takes %.0f%% more than three months before (%.3f g, was %.3f g): tolerance may be creeping up.", r, recent, before)
	case r <= -threshold:
		return fmt.Sprintf("A session takes %.0f%% less than three months before (%.3f g, was %.3f g).", -r, recent, before)
	}
	return fmt.Sprintf("Steady: a session takes %.3f g, %+.0f%% on three months before.", recent, r)
}

// pooled is the grams a session over several months together.
func pooled(ms []month) float64 {
	var all month
	for _, m := range ms {
		all.sessions += m.sessions
		all.grams += m.grams
	}
	return all.perSession()
}
