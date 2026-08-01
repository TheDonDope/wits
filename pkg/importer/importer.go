package importer

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/TheDonDope/wits-tui/pkg/catalog"
	"github.com/TheDonDope/wits-tui/pkg/journal"
	"github.com/xuri/excelize/v2"
)

// dateHeaders are the labels that mark the start of the daily table. The early
// sheets are in German.
var dateHeaders = map[string]bool{"date": true, "datum": true}

// balanceFormula matches a running-balance cell, which binds a strain label to
// the header row it subtracts from: =IF(B6="WW",B1-C6,B1)
var balanceFormula = regexp.MustCompile(`^IF\(B\d+\s*=\s*"([^"]*)"\s*,\s*B(\d+)\s*-\s*C\d+`)

// Product is one product named in a worksheet header.
type Product struct {
	Row       int     // the header row it sits on, which is what formulas bind to
	Name      string  // the display name as written
	Slug      string  // the stable identity derived from the name
	Purchased float64 // grams dispensed
	Ground    float64 // grams accounted for by the rows below
}

// Sheet is one worksheet, that is one prescription cycle.
type Sheet struct {
	Name      string
	Products  []*Product
	Events    []journal.Event
	Anomalies []string
}

// Result is everything a run of the importer found.
type Result struct {
	Sheets   []*Sheet
	Products []*catalog.Product
	Events   []journal.Event
}

// Anomalies returns every problem found, prefixed with the sheet it came from.
func (r *Result) Anomalies() []string {
	var all []string
	for _, s := range r.Sheets {
		for _, a := range s.Anomalies {
			all = append(all, fmt.Sprintf("%s: %s", s.Name, a))
		}
	}
	return all
}

// Grams returns the total grams purchased and ground across every sheet.
func (r *Result) Grams() (purchased, ground float64) {
	for _, s := range r.Sheets {
		for _, p := range s.Products {
			purchased += p.Purchased
			ground += p.Ground
		}
	}
	return purchased, ground
}

// Import reads the workbook at path and converts every worksheet into events.
// It never writes anything: the caller decides whether to commit the result.
func Import(path string) (*Result, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	result := &Result{}
	slugs := map[string]*catalog.Product{}
	for _, name := range f.GetSheetList() {
		sheet, err := importSheet(f, name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if sheet == nil {
			continue
		}
		for _, p := range sheet.Products {
			if _, seen := slugs[p.Slug]; !seen {
				product := catalog.Parse(p.Name)
				slugs[p.Slug] = product
				result.Products = append(result.Products, product)
			}
		}
		result.Sheets = append(result.Sheets, sheet)
		result.Events = append(result.Events, sheet.Events...)
	}

	// The journal is a chronology, but the worksheets are not in date order:
	// several are named for a month that starts a few days earlier, and one is
	// dated a year early outright.
	sort.SliceStable(result.Events, func(i, j int) bool {
		return result.Events[i].OccurredAt.Before(result.Events[j].OccurredAt)
	})
	return result, nil
}

// importSheet converts a single worksheet. It returns nil for a sheet that does
// not look like a tracking sheet at all.
func importSheet(f *excelize.File, name string) (*Sheet, error) {
	rows, err := f.GetRows(name, excelize.Options{RawCellValue: true})
	if err != nil {
		return nil, err
	}

	header := headerRow(rows)
	if header == -1 {
		return nil, nil
	}
	sheet := &Sheet{Name: name}
	sheet.Products = products(rows, header)
	if len(sheet.Products) == 0 {
		return nil, nil
	}
	byRow := map[int]*Product{}
	for _, p := range sheet.Products {
		byRow[p.Row] = p
	}

	labels, anomalies := labelBindings(f, name, header, byRow)
	sheet.Anomalies = append(sheet.Anomalies, anomalies...)

	// Grinds first, so that the fill can be dated from the earliest of them.
	type grind struct {
		at      time.Time
		product *Product
		grams   float64
	}
	var grinds []grind
	var earliest time.Time
	unattributed := map[string]float64{}

	for i := header + 1; i < len(rows); i++ {
		row := rows[i]
		at, ok := cellDate(row, 0)
		if !ok {
			continue
		}
		if earliest.IsZero() || at.Before(earliest) {
			earliest = at
		}
		grams := cellFloat(row, 2)
		if grams <= 0 {
			continue
		}
		label := strings.TrimSpace(cellString(row, 1))
		product, ok := labels[label]
		if !ok {
			// The grams are real but no balance column claims them, so the
			// spreadsheet never subtracted them from anything either.
			unattributed[label] += grams
			continue
		}
		product.Ground += grams
		grinds = append(grinds, grind{at: at, product: product, grams: grams})
	}

	for label, grams := range unattributed {
		sheet.Anomalies = append(sheet.Anomalies,
			fmt.Sprintf("%.2fg logged against %q, which no balance column matches, so it was never subtracted", grams, label))
	}
	if earliest.IsZero() {
		return nil, nil
	}

	for _, p := range sheet.Products {
		sheet.Events = append(sheet.Events, journal.Event{
			Type:       journal.Purchase,
			Product:    p.Slug,
			Grams:      p.Purchased,
			OccurredAt: earliest,
			Note:       "imported from " + name,
		})
	}
	for _, g := range grinds {
		sheet.Events = append(sheet.Events, journal.Event{
			Type:       journal.Grind,
			Product:    g.product.Slug,
			Grams:      g.grams,
			OccurredAt: g.at,
			Note:       "imported from " + name,
		})
	}
	years := map[int]int{}
	for _, g := range grinds {
		years[g.at.Year()]++
	}
	sheet.Anomalies = append(sheet.Anomalies, checkSheet(sheet, name, years)...)

	for i := range sheet.Events {
		from, to, _ := journal.Flow(sheet.Events[i].Type)
		sheet.Events[i].From, sheet.Events[i].To = from, to
	}
	return sheet, nil
}

// labelBindings reads the running-balance formulas to learn which strain label
// belongs to which header product.
func labelBindings(f *excelize.File, sheet string, header int, byRow map[int]*Product) (map[string]*Product, []string) {
	labels := map[string]*Product{}
	var anomalies []string

	// Balance columns start after Date, Strain and Amount, and come in pairs of
	// grams and percent.
	slot := 0
	for col := 4; col <= 12; col++ {
		ref, err := excelize.CoordinatesToCellName(col, header+2)
		if err != nil {
			continue
		}
		formula, err := f.GetCellFormula(sheet, ref)
		if err != nil || formula == "" {
			continue
		}
		m := balanceFormula.FindStringSubmatch(strings.TrimPrefix(formula, "="))
		if m == nil {
			continue
		}
		row, err := strconv.Atoi(m[2])
		if err != nil {
			continue
		}
		product, ok := byRow[row]
		if !ok {
			anomalies = append(anomalies, fmt.Sprintf("column %s subtracts from row %d, which names no product", ref, row))
			continue
		}
		slot++
		if product.Row != slot {
			anomalies = append(anomalies,
				fmt.Sprintf("balance column %d subtracts from header row %d, so the columns are not in header order", slot, product.Row))
		}
		if existing, clash := labels[m[1]]; clash && existing != product {
			anomalies = append(anomalies, fmt.Sprintf("label %q is claimed by two products", m[1]))
			continue
		}
		labels[m[1]] = product
	}
	return labels, anomalies
}

// checkSheet reconciles the events against what the sheet says about itself.
// years counts how many entries fall in each calendar year.
func checkSheet(sheet *Sheet, name string, years map[int]int) []string {
	var anomalies []string

	// A sheet whose entries fall in a different year than its name is a typo
	// that would otherwise be imported as fact. Compare against the year most
	// of the entries are in, not the earliest one: a cycle routinely starts a
	// few days before the month it is named for, and flagging that would bury
	// the one sheet that is genuinely a year out.
	if named, err := strconv.Atoi(strings.SplitN(name, "-", 2)[0]); err == nil {
		dominant, most := 0, 0
		for year, count := range years {
			if count > most {
				dominant, most = year, count
			}
		}
		if most > 0 && dominant != named {
			anomalies = append(anomalies,
				fmt.Sprintf("sheet is named for %d but %d of its %d entries are in %d",
					named, most, total(years), dominant))
		}
	}

	for _, p := range sheet.Products {
		if p.Ground > p.Purchased+0.005 {
			anomalies = append(anomalies,
				fmt.Sprintf("%s: %.2fg ground but only %.2fg dispensed", p.Slug, p.Ground, p.Purchased))
		}
		if p.Purchased > 0 && p.Purchased < 0.05 {
			anomalies = append(anomalies,
				fmt.Sprintf("%s: dispensed amount of %.2fg looks like a typo", p.Slug, p.Purchased))
		}
	}
	return anomalies
}

// headerRow returns the index of the row that starts the daily table.
func headerRow(rows [][]string) int {
	for i, row := range rows {
		if i > 8 {
			break
		}
		if len(row) > 0 && dateHeaders[strings.ToLower(strings.TrimSpace(row[0]))] {
			return i
		}
	}
	return -1
}

// products reads the header block above the daily table. A row counts when it
// names something and gives an amount, which skips the totals row.
func products(rows [][]string, header int) []*Product {
	var found []*Product
	for i := 0; i < header; i++ {
		name := strings.TrimSpace(cellString(rows[i], 0))
		if name == "" {
			continue
		}
		grams := cellFloat(rows[i], 1)
		if grams <= 0 {
			continue
		}
		display := strings.TrimSpace(strings.TrimSuffix(name, "(g)"))
		found = append(found, &Product{
			Row:       i + 1, // formulas use one-based rows
			Name:      display,
			Slug:      catalog.Slugify(display),
			Purchased: grams,
		})
	}
	return found
}

// cellString returns a cell as text, or empty when the row is short.
func cellString(row []string, col int) string {
	if col >= len(row) {
		return ""
	}
	return row[col]
}

// cellFloat returns a cell as a number, or zero when it is not one.
func cellFloat(row []string, col int) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(cellString(row, col)), 64)
	if err != nil {
		return 0
	}
	return v
}

// cellDate returns a cell as a date. Dates are read raw, as the serial numbers
// the file stores, so that no display format gets in the way.
func cellDate(row []string, col int) (time.Time, bool) {
	raw := strings.TrimSpace(cellString(row, col))
	if raw == "" {
		return time.Time{}, false
	}
	serial, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return time.Time{}, false
	}
	at, err := excelize.ExcelDateToTime(serial, false)
	if err != nil {
		return time.Time{}, false
	}
	return at, true
}

// total sums the values of a count.
func total(counts map[int]int) int {
	var n int
	for _, c := range counts {
		n += c
	}
	return n
}
