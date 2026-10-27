// Package tui holds the Insights pages that leave the screen here: a trip
// packed and what is carried, signed as `wits pack` and `wits carry` sign
// them, their QR code drawn in the terminal; the report for the prescriber;
// and the map of the strains.
package tui

import (
	"crypto/ed25519"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"rsc.io/qr"

	"github.com/TheDonDope/wits/pkg/pack"
	"github.com/TheDonDope/wits/pkg/report"
	"github.com/TheDonDope/wits/pkg/seal"
	"github.com/TheDonDope/wits/pkg/version"
)

// tripPage is what to take for days away, as `wits pack` plans it.
func tripPage(a *App, days int, mode pack.Mode, _ int) string {
	t := a.theme
	p, err := pack.Make(a.data.Workspace, days, pack.DefaultMargin, mode)
	if err != nil {
		return t.Dim.Render(err.Error())
	}
	head := t.Big.Render(fmt.Sprintf("%.2f g", p.Packed)) +
		t.Dim.Render(fmt.Sprintf("  for %s at %.2f g a day, with %d %% to spare", plural(p.Days, "day"), p.Rate, p.Margin))
	if p.Short {
		head = t.Big.Render(fmt.Sprintf("%.2f g", p.Packed)) + t.Dim.Render(fmt.Sprintf("  — all the shelf holds, %.1f days of the %d asked",
			p.Packed/p.Rate, p.Days))
	}
	rows := make([][]string, 0, len(p.Lines))
	for _, l := range p.Lines {
		rows = append(rows, []string{truncate(l.Name, 34), fmt.Sprintf("%.2f g", l.Stash), fmt.Sprintf("%.2f g", l.Sealed),
			fmt.Sprintf("%.2f g", l.Taken()), fmt.Sprintf("%.1f", l.Days)})
	}
	how := "evenly, about the same from every jar"
	if p.Mode == pack.Oldest {
		how = "oldest fill first"
	}
	return strings.Join([]string{head, "", table(a, []string{"PRODUCT", "GROUND", "SEALED", "TAKE", "DAYS"}, rows), "",
		t.Dim.Render("Drawn " + how + ", ground before sealed. Nothing is recorded.")}, "\n")
}

// carryPage is what is held now, the fills that dispensed it, as `wits
// carry` lists it before signing.
func carryPage(a *App, _ int) string {
	t := a.theme
	items := report.Holdings(a.data.Workspace, false)
	if len(items) == 0 {
		return t.Dim.Render("Nothing is held: there is nothing to show.")
	}
	rows := make([][]string, 0, len(items))
	var total float64
	for _, it := range items {
		fills := make([]string, 0, len(it.Fills))
		for _, f := range it.Fills {
			fills = append(fills, fmt.Sprintf("%.2f g on %s", f.Grams, f.Day))
		}
		rows = append(rows, []string{truncate(it.Name, 30), fmt.Sprintf("%.2f g", it.Sealed), fmt.Sprintf("%.2f g", it.Ground),
			fmt.Sprintf("%.2f g", it.AVB), strings.Join(fills, ", ")})
		total += it.Sealed + it.Ground + it.AVB
	}
	return strings.Join([]string{
		t.Big.Render(fmt.Sprintf("%.2f g", math.Round(total*100)/100)) + t.Dim.Render(" held, on the shelf"), "",
		table(a, []string{"PRODUCT", "SEALED", "GROUND", "AVB", "DISPENSED"}, rows), "",
		t.Dim.Render("w signs it with the sealing key; its QR code opens a page any phone checks it on."),
	}, "\n")
}

// signTrip signs what a trip packs and writes its QR code.
func signTrip(a *App, days int, mode pack.Mode) (said, code string, err error) {
	p, err := pack.Make(a.data.Workspace, days, pack.DefaultMargin, mode)
	if err != nil {
		return "", "", err
	}
	return sign(a, p.Statement(), "trip", fmt.Sprintf("signed for the %.2f g packed", p.Packed))
}

// signCarry signs what is held and writes its QR code.
func signCarry(a *App) (said, code string, err error) {
	items := report.Holdings(a.data.Workspace, false)
	if len(items) == 0 {
		return "", "", fmt.Errorf("nothing is held: there is nothing to show")
	}
	return sign(a, items, "carry", "signed for what is held")
}

// sign signs a statement of the items against the journal's head, as the
// command line signs one — the sealing key made the first time — writes its
// QR code as wits-<what>-<day>.svg, and draws it when the terminal has room.
func sign(a *App, items []seal.Item, what, signed string) (said, code string, err error) {
	events, err := a.data.Journal().Events()
	if err != nil || len(events) == 0 {
		return "", "", fmt.Errorf("the journal is empty: there is nothing to sign against")
	}
	key, made, err := seal.Key(a.data.Repo.SealKeyPath())
	if err != nil {
		return "", "", err
	}
	c := seal.MakeCarry(key, events[len(events)-1].Hash, len(events), a.data.Now, items)
	svg, err := report.QR(c.URL())
	if err != nil {
		return "", "", err
	}
	name := fmt.Sprintf("wits-%s-%s.svg", what, a.data.Now.Format(time.DateOnly))
	if err := os.WriteFile(name, []byte(svg), 0600); err != nil {
		return "", "", err
	}
	said = fmt.Sprintf("Wrote %s, %s, by %s", name, signed, seal.Fingerprint(key.Public().(ed25519.PublicKey)))
	if made {
		said += " — a new sealing key"
	}
	code = terminalQR(c.URL(), a.inner(), a.height-10)
	if code == "" {
		said += "; the terminal is too small to draw its code"
	}
	return said, code, nil
}

// terminalQR draws a QR code in half blocks, two modules a line, dark on
// light whatever the theme, with the quiet zone a reader wants; nothing when
// the terminal is too small to hold it, since a code cut off scans as nothing.
func terminalQR(text string, width, height int) string {
	c, err := qr.Encode(text, qr.L)
	if err != nil {
		return ""
	}
	const quiet = 2
	n := c.Size + 2*quiet
	if n > width || (n+1)/2 > height {
		return ""
	}
	dark := func(x, y int) bool { return x >= 0 && y >= 0 && x < c.Size && y < c.Size && c.Black(x, y) }
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#ffffff"))
	lines := make([]string, 0, (n+1)/2)
	for y := -quiet; y < c.Size+quiet; y += 2 {
		var b strings.Builder
		for x := -quiet; x < c.Size+quiet; x++ {
			b.WriteRune([]rune{' ', '▄', '▀', '█'}[boolInt(dark(x, y))*2+boolInt(dark(x, y+1))])
		}
		lines = append(lines, style.Render(b.String()))
	}
	return strings.Join(lines, "\n")
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// writeReport writes the page for the prescriber, the 90 days to today, as
// `wits report` writes it by default.
func writeReport(a *App) (string, error) {
	to := time.Date(a.data.Now.Year(), a.data.Now.Month(), a.data.Now.Day(), 0, 0, 0, 0, time.Local)
	from := to.AddDate(0, 0, -89)
	name := reportName(to)
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", err
	}
	if err := report.PDF(f, report.Build(a.data.Workspace, from, to, version.String())); err != nil {
		_ = f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return fmt.Sprintf("Wrote %s, a report for %s to %s", name, from.Format(time.DateOnly), to.Format(time.DateOnly)), nil
}

// strainsPage is the map of the strains, as the browser's Effects page and
// `wits like` place them by their terpenes: every strain a dot, those on the
// shelf numbered and named under it.
func strainsPage(a *App, width int) string {
	t := a.theme
	all := report.Strains(a.data.Workspace)
	var placed []report.Strain
	for _, s := range all {
		if s.Placed {
			placed = append(placed, s)
		}
	}
	if len(placed) < 2 {
		return t.Dim.Render("The map needs terpenes: e on storage writes a strain's down, as `wits describe --terpenes` does.")
	}
	w, h := min(width-2, 72), 16
	grid, shelf := strainGrid(a, placed, w, h)
	lines := []string{t.PanelTitle.Render("The strains, placed by their terpenes"), t.Dim.Render(strings.Repeat("─", w))}
	for _, row := range grid {
		lines = append(lines, strings.Join(row, ""))
	}
	lines = append(lines, t.Dim.Render(strings.Repeat("─", w)))
	for i, s := range shelf {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.Accent).Render(fmt.Sprintf("%d", i+1))+"  "+
			t.Value.Render(truncate(s.Name, 36))+t.Dim.Render("  "+s.Taste))
	}
	return strings.Join(append(lines, t.Dim.Render(fmt.Sprintf("%d of %d strains placed; those near each other taste alike.",
		len(placed), len(all)))), "\n")
}

// strainGrid lays the strains on a grid of cells: a dot each, the jars on
// the shelf as their number instead, up to nine of them.
func strainGrid(a *App, placed []report.Strain, w, h int) ([][]string, []report.Strain) {
	t := a.theme
	minX, maxX, minY, maxY := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
	for _, s := range placed {
		minX, maxX, minY, maxY = math.Min(minX, s.X), math.Max(maxX, s.X), math.Min(minY, s.Y), math.Max(maxY, s.Y)
	}
	grid := make([][]string, h)
	for i := range grid {
		grid[i] = strings.Split(strings.Repeat(" ", w), "")
	}
	cell := func(s report.Strain) (int, int) {
		x := int(math.Round((s.X - minX) / math.Max(maxX-minX, 1e-9) * float64(w-1)))
		y := int(math.Round((maxY - s.Y) / math.Max(maxY-minY, 1e-9) * float64(h-1)))
		return x, y
	}
	var shelf []report.Strain
	for _, s := range placed {
		x, y := cell(s)
		if b := a.data.State.Balances[s.Slug]; b != nil && b.Storage+b.Stash > 0 && len(shelf) < 9 {
			shelf = append(shelf, s)
			grid[y][x] = lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Render(fmt.Sprint(len(shelf)))
		} else if grid[y][x] == " " {
			grid[y][x] = t.Dim.Render("·")
		}
	}
	return grid, shelf
}
