package importer

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/TheDonDope/wits-tui/pkg/catalog"
)

// dateHeaders are the labels that mark the start of the daily table. Early
// sheets were kept in German.
var dateHeaders = map[string]bool{"date": true, "datum": true}

// balanceFormula matches a running-balance cell, which binds a strain label to
// the header row it subtracts from: =IF(B6="WW",B1-C6,B1)
var balanceFormula = regexp.MustCompile(`^IF\(B\d+\s*=\s*"([^"]*)"\s*,\s*B(\d+)\s*-\s*C\d+`)

// firstBalanceColumn is where the running balances begin, after Date, Strain
// and Amount.
const firstBalanceColumn = 4

// lastBalanceColumn is as far right as a balance pair is looked for.
const lastBalanceColumn = 14

// Product is one product named in a worksheet header.
type Product struct {
	Row       int     // the header row, which is what the formulas bind to
	Name      string  // the display name as written
	Slug      string  // the identity derived from it
	Purchased float64 // grams dispensed
	Ground    float64 // grams the rows below account for
}

// Entry is one day's grinding of one product.
type Entry struct {
	At      time.Time
	Product *Product
	Grams   float64
	Row     int
}

// Sheet is one worksheet, that is one prescription cycle.
type Sheet struct {
	Name      string
	Products  []*Product
	Entries   []Entry
	Opened    time.Time // the earliest date on the sheet, when the fill was collected
	Anomalies []string
}

// Purchased returns the grams the sheet says were dispensed.
func (s *Sheet) Purchased() float64 {
	var total float64
	for _, p := range s.Products {
		total += p.Purchased
	}
	return total
}

// Ground returns the grams the sheet's rows account for.
func (s *Sheet) Ground() float64 {
	var total float64
	for _, p := range s.Products {
		total += p.Ground
	}
	return total
}

// readSheet parses one worksheet. It returns nil for anything that does not look
// like a tracking sheet, so unrelated tabs in the workbook are simply skipped.
func readSheet(f *excelize.File, name string) (*Sheet, error) {
	rows, err := f.GetRows(name, excelize.Options{RawCellValue: true})
	if err != nil {
		return nil, err
	}
	header := headerRow(rows)
	if header == -1 {
		return nil, nil
	}

	sheet := &Sheet{Name: name}
	sheet.Products = readProducts(rows, header)
	if len(sheet.Products) == 0 {
		return nil, nil
	}

	labels, anomalies := readBindings(f, name, header, sheet.Products)
	sheet.Anomalies = append(sheet.Anomalies, anomalies...)

	orphans := map[string]float64{}
	for i := header + 1; i < len(rows); i++ {
		at, ok := cellDate(rows[i], 0)
		if !ok {
			continue
		}
		if sheet.Opened.IsZero() || at.Before(sheet.Opened) {
			sheet.Opened = at
		}
		grams := cellFloat(rows[i], 2)
		if grams <= 0 {
			continue
		}
		label := strings.TrimSpace(cellString(rows[i], 1))
		product, known := labels[label]
		if !known {
			// Real grams that no balance column claims, which means the
			// spreadsheet never subtracted them from anything either.
			orphans[label] += grams
			continue
		}
		product.Ground += grams
		sheet.Entries = append(sheet.Entries, Entry{At: at, Product: product, Grams: grams, Row: i + 1})
	}

	for label, grams := range orphans {
		sheet.Anomalies = append(sheet.Anomalies, fmt.Sprintf(
			"%.2f g logged against %q, which no balance column matches, so it was never subtracted",
			grams, label))
	}
	if sheet.Opened.IsZero() {
		return nil, nil
	}
	sheet.Anomalies = append(sheet.Anomalies, sheet.check()...)
	return sheet, nil
}

// check reconciles a sheet against what it says about itself.
func (s *Sheet) check() []string {
	var out []string

	// A sheet whose entries fall in a different year than its name is a typo
	// that would otherwise be imported as fact. The comparison is against the
	// year most entries are in: a cycle routinely starts a few days before the
	// month it is named for, and flagging that would bury a sheet genuinely a
	// year out.
	if named, err := strconv.Atoi(strings.SplitN(s.Name, "-", 2)[0]); err == nil {
		years := map[int]int{}
		for _, e := range s.Entries {
			years[e.At.Year()]++
		}
		dominant, most, total := 0, 0, 0
		for year, n := range years {
			total += n
			if n > most {
				dominant, most = year, n
			}
		}
		if most > 0 && dominant != named {
			out = append(out, fmt.Sprintf("named for %d but %d of its %d entries fall in %d",
				named, most, total, dominant))
		}
	}

	for _, p := range s.Products {
		if p.Ground > p.Purchased+0.005 {
			out = append(out, fmt.Sprintf("%s: %.2f g ground but only %.2f g dispensed",
				p.Slug, p.Ground, p.Purchased))
		}
		if p.Purchased > 0 && p.Purchased < 0.05 {
			out = append(out, fmt.Sprintf("%s: a dispensed amount of %.2f g looks like a typo",
				p.Slug, p.Purchased))
		}
	}
	return out
}

// readProducts reads the header block above the daily table. A row counts when
// it names something and gives an amount, which skips the totals row.
func readProducts(rows [][]string, header int) []*Product {
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
			Row:       i + 1, // formulas count rows from one
			Name:      display,
			Slug:      catalog.Slugify(display),
			Purchased: grams,
		})
	}
	return found
}

// readBindings reads the running-balance formulas to learn which strain label
// belongs to which header product.
func readBindings(f *excelize.File, sheet string, header int, products []*Product) (map[string]*Product, []string) {
	byRow := make(map[int]*Product, len(products))
	for _, p := range products {
		byRow[p.Row] = p
	}

	labels := map[string]*Product{}
	var anomalies []string
	slot := 0

	for col := firstBalanceColumn; col <= lastBalanceColumn; col++ {
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
			anomalies = append(anomalies, fmt.Sprintf(
				"column %s subtracts from row %d, which names no product", ref, row))
			continue
		}
		slot++
		if product.Row != slot {
			anomalies = append(anomalies, fmt.Sprintf(
				"balance column %d subtracts from header row %d, so the columns are not in header order",
				slot, product.Row))
		}
		if existing, clash := labels[m[1]]; clash && existing != product {
			anomalies = append(anomalies, fmt.Sprintf("label %q is claimed by two products", m[1]))
			continue
		}
		labels[m[1]] = product
	}
	return labels, anomalies
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

// cellDate returns a cell as a date. Cells are read raw, as the serial numbers
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
