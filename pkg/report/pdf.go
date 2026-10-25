// Package report: the PDF, written here, not by a library — one page of text,
// lines and bars in the standard Helvetica every PDF reader carries, which is a
// few hundred lines and no dependency, the way the charts are drawn in-tree.
// Text is encoded as WinAnsi, which has the euro sign, the umlauts and the
// dashes a German report needs.

package report

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/seal"
)

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
	if r.Seal != nil {
		p.text(margin, margin, 7, false, fmt.Sprintf("Sealed %s by the key %s: `wits verify %s.seal` checks the seal.",
			long(r.Seal.At.Local()), seal.Fingerprint(r.Seal.Key), r.Seal.Name()))
	}
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
