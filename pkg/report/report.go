// Package report is the one page for the prescriber: the months asked for,
// grams a day, which strains, what the sessions did by the effect log, and
// what is left of the fill in progress — as a PDF.
//
// The PDF is written here, not by a library: one page of text, lines and bars
// in the standard Helvetica every PDF reader carries, which is a few hundred
// lines and no dependency, the way the charts are drawn in-tree. Text is
// encoded as WinAnsi, which has the euro sign, the umlauts and the dashes a
// German report needs.
package report

import (
	"bytes"
	"cmp"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// Report is what the page says, every figure for the days asked for unless it
// says otherwise: what is left now is the fill's and the shelf's, today.
type Report struct {
	From, To  time.Time
	Generated time.Time
	Version   string
	Days      int

	Ground       float64
	GrindDays    int
	PerGrindDay  float64
	MedianPerDay float64
	PerDay       float64
	Dispensed    float64
	Purchases    int
	Seshed       float64
	Sessions     int

	Products []Use
	Weeks    []Week
	Rated    int
	Effects  []Effect
	Best     []effects.Group
	Now      *Now

	Entries int
	Head    string
}

// Use is what one product went through in the days asked for.
type Use struct {
	Name               string
	THC, CBD           float64
	Dispensed, Ground  float64
	Seshed             float64
	SessionsWithRating int
}

// Week is what was ground in the seven days from Start.
type Week struct {
	Start  time.Time
	Ground float64
}

// Effect is one dimension's mean over the sessions rated on it.
type Effect struct {
	Dimension effects.Dimension
	Mean      float64
	Count     int
}

// Now is the fill in progress and the shelf, today.
type Now struct {
	Fill               int
	Since              time.Time
	Dispensed, Left    float64
	Shelf              float64
	EmptyOn, OrderBy   time.Time
	HasRefill          bool
	OlderJars, Carried float64
}

// Build gathers the report for the days from from to to, both included.
func Build(ws *workspace.Workspace, from, to time.Time, version string) Report {
	from, to = midnight(from), midnight(to)
	r := Report{From: from, To: to, Generated: ws.OpenedAt, Version: version,
		Days: int(to.Sub(from).Hours()/24) + 1, Entries: len(ws.State.Events)}
	if events, err := ws.Journal().Events(); err == nil && len(events) > 0 {
		r.Head = events[len(events)-1].Hash
	}
	var in []journal.Event
	for _, e := range ledger.Standing(ws.State.Events) {
		if !e.OccurredAt.Before(from) && e.OccurredAt.Before(to.AddDate(0, 0, 1)) {
			in = append(in, e)
		}
	}
	stats := ledger.Summarise(in)
	r.Ground, r.GrindDays, r.PerGrindDay, r.MedianPerDay = stats.Ground, stats.ActiveDays, stats.PerActiveDay, stats.MedianPerDay
	r.PerDay = ledger.Round(stats.Ground / float64(r.Days))
	r.Products = uses(ws, in, &r)
	r.Weeks = weeks(in, from, to)
	r.Rated, r.Effects = rated(ws.Ratings, in)
	insights := effects.Analyse(ws.Ratings, in, ws.ProductName)
	r.Best = best(insights.ByProduct)
	r.Now = now(ws)
	return r
}

// uses totals the range by product, the most ground first, and the range's
// purchases and sessions with them.
func uses(ws *workspace.Workspace, in []journal.Event, r *Report) []Use {
	by := map[string]*Use{}
	var dispensed, seshed journal.Centigrams
	for _, e := range in {
		if e.Product == "" {
			continue
		}
		u := by[e.Product]
		if u == nil {
			u = &Use{Name: ws.ProductName(e.Product)}
			if p, err := ws.Products.Find(e.Product); err == nil {
				u.THC, u.CBD = p.THC, p.CBD
			}
			by[e.Product] = u
		}
		switch e.Type {
		case journal.Purchase:
			u.Dispensed = ledger.Round(u.Dispensed + e.Grams)
			dispensed += journal.CG(e.Grams)
			r.Purchases++
		case journal.Grind:
			u.Ground = ledger.Round(u.Ground + e.Grams)
		case journal.Sesh:
			u.Seshed = ledger.Round(u.Seshed + e.Grams)
			seshed += journal.CG(e.Grams)
			r.Sessions++
		}
	}
	r.Dispensed, r.Seshed = dispensed.Grams(), seshed.Grams()
	out := make([]Use, 0, len(by))
	for _, u := range by {
		out = append(out, *u)
	}
	slices.SortFunc(out, func(a, b Use) int {
		return cmp.Or(cmp.Compare(b.Ground+b.Dispensed, a.Ground+a.Dispensed), strings.Compare(a.Name, b.Name))
	})
	return out
}

// weeks is the grinding in each seven days of the range, the last week cut
// short where the range ends.
func weeks(in []journal.Event, from, to time.Time) []Week {
	var out []Week
	for d := from; !d.After(to); d = d.AddDate(0, 0, 7) {
		out = append(out, Week{Start: d})
	}
	for _, e := range in {
		if e.Type != journal.Grind {
			continue
		}
		i := int(midnight(e.OccurredAt).Sub(from).Hours() / 24 / 7)
		if i >= 0 && i < len(out) {
			out[i].Ground = ledger.Round(out[i].Ground + e.Grams)
		}
	}
	return out
}

// rated is how many of the range's sessions were rated, and each dimension's
// mean over the ones rated on it.
func rated(ratings []effects.Rating, in []journal.Event) (int, []Effect) {
	latest := effects.Latest(ratings)
	sums := map[effects.Dimension][2]int{}
	n := 0
	for _, e := range in {
		r, ok := latest[e.Hash]
		if e.Type != journal.Sesh || !ok {
			continue
		}
		n++
		for d, v := range r.Scores {
			s := sums[d]
			sums[d] = [2]int{s[0] + v, s[1] + 1}
		}
	}
	var out []Effect
	for _, d := range effects.Dimensions {
		if s := sums[d]; s[1] > 0 {
			out = append(out, Effect{Dimension: d, Mean: float64(s[0]) / float64(s[1]), Count: s[1]})
		}
	}
	return n, out
}

// best is up to three products by their overall mean, the most rated first
// among equals.
func best(groups []effects.Group) []effects.Group {
	var out []effects.Group
	for _, g := range groups {
		if g.Counts[effects.Overall] > 0 {
			out = append(out, g)
		}
	}
	slices.SortFunc(out, func(a, b effects.Group) int {
		return cmp.Or(cmp.Compare(b.Means[effects.Overall], a.Means[effects.Overall]),
			cmp.Compare(b.Counts[effects.Overall], a.Counts[effects.Overall]))
	})
	return out[:min(3, len(out))]
}

// now is the fill in progress and the shelf, today, with when to order.
func now(ws *workspace.Workspace) *Now {
	c := ws.Cycle()
	if c == nil {
		return nil
	}
	n := &Now{Fill: c.Seq + 1, Since: c.Start, Dispensed: float64(c.Purchased), Left: float64(ws.State.FillOnShelf(c))}
	carried, _, _ := ws.State.CarriedOnShelf(c)
	n.Carried = float64(carried)
	for _, b := range ws.State.Balances {
		n.Shelf = ledger.Round(n.Shelf + b.Storage)
	}
	if plan, ok := ledger.PlanRefill(c, ws.OpenedAt, ws.Repo.Config.Lead()); ok {
		n.EmptyOn, n.OrderBy, n.HasRefill = plan.EmptyOn, plan.OrderBy, true
	}
	return n
}

func midnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// A4, in points, and the margin the page keeps.
const (
	pageW, pageH = 595.28, 841.89
	margin       = 50.0
)

// PDF writes the report as a one-page PDF.
func PDF(w io.Writer, r Report) error {
	p := &page{}
	y := p.header(r, pageH-margin)
	y = p.use(r, y-26)
	y = p.chart(r, y-18)
	y = p.strains(r, y-26)
	y = p.effects(r, y-22)
	p.now(r, y-22)
	p.footer(r)
	return p.write(w)
}

func (p *page) header(r Report, y float64) float64 {
	p.text(margin, y-18, 18, true, "Cannabis therapy: a report for the prescriber")
	p.text(margin, y-36, 10, false, fmt.Sprintf("%s to %s, %d days. Generated %s.",
		long(r.From), long(r.To), r.Days, long(r.Generated)))
	p.rule(y - 46)
	return y - 46
}

func (p *page) use(r Report, y float64) float64 {
	p.text(margin, y, 12, true, "Use")
	rows := [][2]string{
		{"Ground", fmt.Sprintf("%.2f g on %d of the %d days", r.Ground, r.GrindDays, r.Days)},
		{"Per day with grinding", fmt.Sprintf("%.2f g, median %.2f g", r.PerGrindDay, r.MedianPerDay)},
		{"Per calendar day", fmt.Sprintf("%.2f g", r.PerDay)},
		{"Dispensed", fmt.Sprintf("%.2f g in %s", r.Dispensed, count(r.Purchases, "purchase"))},
		{"Through a vaporizer", fmt.Sprintf("%.2f g in %s", r.Seshed, count(r.Sessions, "session"))},
	}
	for i, row := range rows {
		ry := y - 16 - float64(i)*14
		p.text(margin, ry, 10, false, row[0])
		p.text(margin+150, ry, 10, false, row[1])
	}
	return y - 16 - float64(len(rows))*14
}

func (p *page) chart(r Report, y float64) float64 {
	const h = 80.0
	p.text(margin, y, 9, false, "Grams ground per week")
	top := 0.0
	for _, w := range r.Weeks {
		top = max(top, w.Ground)
	}
	if top == 0 || len(r.Weeks) == 0 {
		p.text(margin, y-16, 9, false, "Nothing was ground in these days.")
		return y - 20
	}
	base, width := y-10-h, pageW-2*margin
	bar := width / float64(len(r.Weeks))
	for i, w := range r.Weeks {
		bh := w.Ground / top * h
		p.box(margin+float64(i)*bar+1, base, bar-2, bh, 0.36, 0.66, 0.53)
	}
	p.line(margin, base, pageW-margin, base, 0.5, 0.6)
	p.text(margin, base-11, 8, false, short(r.Weeks[0].Start))
	last := short(r.Weeks[len(r.Weeks)-1].Start)
	p.text(pageW-margin-p.width(last, 8, false), base-11, 8, false, last)
	peak := fmt.Sprintf("%.1f g", top)
	p.text(pageW-margin-p.width(peak, 8, false), y-10, 8, false, peak)
	return base - 14
}

func (p *page) strains(r Report, y float64) float64 {
	p.text(margin, y, 12, true, "Strains")
	cols := []float64{margin, margin + 280, margin + 360, margin + 430}
	head := []string{"Product", "THC / CBD %", "Dispensed", "Ground"}
	for i, h := range head {
		p.right(i > 0, cols[i], y-16, 9, true, h)
	}
	rows := r.Products[:min(len(r.Products), 8)]
	for i, u := range rows {
		ry := y - 30 - float64(i)*13
		p.text(cols[0], ry, 9, false, clip(u.Name, 52))
		p.right(true, cols[1], ry, 9, false, ratio(u.THC, u.CBD))
		p.right(true, cols[2], ry, 9, false, grams(u.Dispensed))
		p.right(true, cols[3], ry, 9, false, grams(u.Ground))
	}
	return y - 30 - float64(len(rows))*13
}

func (p *page) effects(r Report, y float64) float64 {
	p.text(margin, y, 12, true, "What the sessions did")
	if r.Rated == 0 {
		say := fmt.Sprintf("None of the %s was rated.", count(r.Sessions, "session"))
		if r.Sessions == 0 {
			say = "No session was recorded in these days, so the effect log has nothing to say."
		}
		p.text(margin, y-16, 10, false, say)
		return y - 20
	}
	p.text(margin, y-16, 10, false, fmt.Sprintf("%s rated, 1 (not at all) to 5 (very well). Each mean says how many ratings it stands on.",
		count(r.Rated, "session")))
	for i, e := range r.Effects {
		ry := y - 32 - float64(i/3)*13
		x := margin + float64(i%3)*165
		p.text(x, ry, 10, false, fmt.Sprintf("%s  %.1f  (%d)", title(string(e.Dimension)), e.Mean, e.Count))
	}
	y -= 32 + float64((len(r.Effects)+2)/3)*13
	for i, g := range r.Best {
		p.text(margin, y-float64(i)*13, 10, false, fmt.Sprintf("Overall, %s: %.1f (%d)",
			clip(g.Label, 60), g.Means[effects.Overall], g.Counts[effects.Overall]))
	}
	return y - float64(len(r.Best))*13
}

func (p *page) now(r Report, y float64) float64 {
	p.text(margin, y, 12, true, "Today")
	if r.Now == nil {
		p.text(margin, y-16, 10, false, "No fill in progress: storage is empty.")
		return y - 20
	}
	n := r.Now
	lines := []string{
		fmt.Sprintf("Fill %d, dispensed %s: %.2f g of its %.2f g left.", n.Fill, long(n.Since), n.Left, n.Dispensed),
		fmt.Sprintf("On the shelf, every jar: %.2f g, %.2f g of it from earlier fills.", n.Shelf, n.Carried),
	}
	if n.HasRefill {
		lines = append(lines, fmt.Sprintf("At the fill's rate the shelf is empty around %s; the next prescription is due by %s.",
			long(n.EmptyOn), long(n.OrderBy)))
	}
	for i, l := range lines {
		p.text(margin, y-16-float64(i)*14, 10, false, l)
	}
	return y - 16 - float64(len(lines))*14
}

func (p *page) footer(r Report) {
	p.rule(margin + 22)
	head := "an empty journal"
	if r.Head != "" {
		head = fmt.Sprintf("%d entries, whose last is %s", r.Entries, r.Head[:16])
	}
	p.text(margin, margin+10, 7, false, fmt.Sprintf("From the hash-chained journal kept by wits %s: %s. `wits fsck` checks it.", r.Version, head))
}

func long(t time.Time) string { return t.Format("2 January 2006") }

// grams is an amount, or a dash for none: a jar dispensed before the range
// was not dispensed in it.
func grams(g float64) string {
	if g == 0 {
		return "–"
	}
	return fmt.Sprintf("%.2f g", g)
}

// title capitalises a word's first letter.
func title(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
func short(t time.Time) string { return t.Format("2 Jan") }

func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func ratio(thc, cbd float64) string {
	if thc == 0 && cbd == 0 {
		return "–"
	}
	return fmt.Sprintf("%g / %g", thc, cbd)
}

// clip shortens text to n characters, with an ellipsis where it was cut.
func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// page is one PDF page being drawn: its content stream, in PDF's own
// operators, with y counted up from the bottom of the page.
type page struct{ content bytes.Buffer }

func (p *page) text(x, y, size float64, bold bool, s string) {
	font := "F1"
	if bold {
		font = "F2"
	}
	// Text is filled in the fill colour, which a bar may have left green.
	fmt.Fprintf(&p.content, "BT 0.1 g /%s %.1f Tf %.2f %.2f Td (%s) Tj ET\n", font, size, x, y, escape(winAnsi(s)))
}

// right draws text ending at x when it is a number column, starting there when not.
func (p *page) right(alignRight bool, x, y, size float64, bold bool, s string) {
	if alignRight {
		x += 60 - p.width(s, size, bold)
	}
	p.text(x, y, size, bold, s)
}

func (p *page) line(x1, y1, x2, y2, width, gray float64) {
	fmt.Fprintf(&p.content, "%.2f G %.2f w %.2f %.2f m %.2f %.2f l S\n", gray, width, x1, y1, x2, y2)
}

func (p *page) rule(y float64) { p.line(margin, y, pageW-margin, y, 0.5, 0.75) }

func (p *page) box(x, y, w, h, r, g, b float64) {
	fmt.Fprintf(&p.content, "%.2f %.2f %.2f rg %.2f %.2f %.2f %.2f re f\n", r, g, b, x, y, w, h)
}

// width is how wide text sets in Helvetica, in points.
func (p *page) width(s string, size float64, bold bool) float64 {
	var units int
	for _, c := range []byte(winAnsi(s)) {
		units += glyph(c, bold)
	}
	return float64(units) * size / 1000
}

// write lays the page out as a PDF file: the catalog, the page tree, the page,
// its content, and the two fonts, then the cross-reference table that says
// where each object starts.
func (p *page) write(w io.Writer) error {
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f] /Contents 4 0 R "+
			"/Resources << /Font << /F1 5 0 R /F2 6 0 R >> >> >>", pageW, pageH),
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", p.content.Len(), p.content.String()),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>",
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	offsets := make([]int, len(objects))
	for i, o := range objects {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	_, err := w.Write(out.Bytes())
	return err
}

// escape protects a PDF string's delimiters.
func escape(s string) string {
	return strings.NewReplacer(`\`, `\\`, "(", `\(`, ")", `\)`).Replace(s)
}

// winAnsi encodes text in the code page the standard fonts are set in: Latin-1,
// with the euro, the dashes, the ellipsis and the curly quotes where Windows
// put them. Anything else becomes a question mark.
func winAnsi(s string) string {
	special := map[rune]byte{'€': 0x80, '‚': 0x82, '„': 0x84, '…': 0x85, '‘': 0x91, '’': 0x92,
		'“': 0x93, '”': 0x94, '•': 0x95, '–': 0x96, '—': 0x97, '™': 0x99}
	out := make([]byte, 0, len(s))
	for _, r := range s {
		switch b, ok := special[r]; {
		case ok:
			out = append(out, b)
		case r < 0x80 || (r >= 0xA0 && r <= 0xFF):
			out = append(out, byte(r))
		default:
			out = append(out, '?')
		}
	}
	return string(out)
}

// helvetica and helveticaBold are the widths of the printable ASCII
// characters, space to tilde, in thousandths of the font size: Adobe's
// metrics for the two standard fonts the page is set in.
var (
	helvetica = [95]int{
		278, 278, 355, 556, 556, 889, 667, 191, 333, 333, 389, 584, 278, 333, 278, 278,
		556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 278, 278, 584, 584, 584, 556,
		1015, 667, 667, 722, 722, 667, 611, 778, 722, 278, 500, 667, 556, 833, 722, 778,
		667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 278, 278, 278, 469, 556,
		333, 556, 556, 500, 556, 556, 278, 556, 556, 222, 222, 500, 222, 833, 556, 556,
		556, 556, 333, 500, 278, 556, 500, 722, 500, 500, 500, 334, 260, 334, 584,
	}
	helveticaBold = [95]int{
		278, 333, 474, 556, 556, 889, 722, 238, 333, 333, 389, 584, 278, 333, 278, 278,
		556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 333, 333, 584, 584, 584, 611,
		975, 722, 722, 722, 722, 667, 611, 778, 722, 278, 556, 722, 611, 833, 722, 778,
		667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 333, 278, 333, 584, 556,
		333, 556, 611, 556, 611, 556, 333, 611, 611, 278, 278, 556, 278, 889, 611, 611,
		611, 611, 389, 556, 333, 611, 556, 778, 556, 556, 500, 389, 280, 389, 584,
	}
)

// glyph is a WinAnsi character's width, in thousandths of the font size. The
// few characters past ASCII the report sets have their own; the rest of
// Latin-1 is near enough the width of a digit.
func glyph(c byte, bold bool) int {
	if c >= 32 && c <= 126 {
		if bold {
			return helveticaBold[c-32]
		}
		return helvetica[c-32]
	}
	switch c {
	case 0x85, 0x97:
		return 1000
	case 0x95:
		return 350
	case 0xB0:
		return 400
	case 0xB7:
		return 278
	}
	return 556
}
