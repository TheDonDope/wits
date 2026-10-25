// Package invoice reads a pharmacy's invoice for a fill from its text: each
// product with its grams, its price a gram and the line's total, the date it
// was ordered, the order's number, and the totals with and without VAT.
//
// It reads text, not PDF: the browser takes the text out of the file with
// pdf.js, on the device, and sends only that — as the label's photograph is
// read on the device and only its text sent. The text is a line per row of the
// invoice, its cells apart by whitespace. The words looked for are a German
// pharmacy's (Menge, Gesamtbetrag, USt), since those are the invoices there are.
// Two pharmacies' layouts are read: a product on one row, as most print it,
// and a product over several rows under its PZN (block.go).
package invoice

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Line is one product on the invoice. Its prices are as printed: with VAT
// when the invoice's totals say the lines include it, as a German
// pharmacy's do.
type Line struct {
	Name    string
	Grams   float64
	PerGram int64 // cents
	Total   int64 // cents
}

// Other is something on the invoice priced but not flower by the gram — a
// vape cartridge by the piece, shipping. It is left out of the fill, and
// said, so the invoice still adds up.
type Other struct {
	Name  string
	Total int64 // cents, as printed
}

// Invoice is what the text of one says about a fill.
type Invoice struct {
	Date   time.Time // the day ordered; zero when none was found
	Number string
	Lines  []Line
	Other  []Other
	Net    int64 // cents, without VAT; zero when not printed
	VAT    int64 // cents
	Rate   int   // percent
	Gross  int64 // cents, with VAT; zero when not printed
}

// Sum is the lines' totals added up, which on a German pharmacy's invoice is
// the total with VAT, less what was left out.
func (inv Invoice) Sum() int64 {
	var sum int64
	for _, l := range inv.Lines {
		sum += l.Total
	}
	return sum
}

// LeftOut is what the invoice charged for besides the flower.
func (inv Invoice) LeftOut() int64 {
	var sum int64
	for _, o := range inv.Other {
		sum += o.Total
	}
	return sum
}

// euro is an amount as printed: 1.234,56 or 5,84, or 12.50 as one pharmacy
// prints it, with the sign or without.
const euro = `(\d{1,3}(?:\.\d{3})*,\d{2}|\d+,\d{2}|\d+\.\d{2})\s*€?`

var (
	// A product line ends in its amount, its price a gram and its total:
	// "NICE 32/1 MOS Morning Sun   20 g   5,80 €   116,00 €".
	product = regexp.MustCompile(`^(.+?)\s+(\d+(?:[.,]\d+)?)\s*g\s+` + euro + `\s+` + euro + `\s*$`)
	// A row priced but not by the gram: "Curaleaf … 4 St. 49,20 € 196,80 €".
	priced = regexp.MustCompile(`^(.+?)\s+(?:\d+(?:[.,]\d+)?\s*(?:St\.?|Stk\.?|x)?\s+)?(?:` + euro + `\s+)?` + euro + `\s*$`)
	net    = regexp.MustCompile(`(?i)(?:exkl|ohne|netto|zwischensumme).*?\s` + euro + `\s*$`)
	gross  = regexp.MustCompile(`(?i)(?:inkl|brutto).*?\s` + euro + `\s*$`)
	vat    = regexp.MustCompile(`(?i)^(?:USt|MwSt)\.?\s*(\d{1,2})\s*%\s+` + euro + `\s*$`)
	day    = regexp.MustCompile(`^(\d{2})[./](\d{2})[./](\d{4})$`)
	// The date and the number stand on the line under their heading, or
	// after it on the same row: "Reservierungsdatum: 09.06.2025",
	// "Rechnung Nr. 2606-MNB-1637". A birth date (Geb.-Datum) is neither.
	dateHead     = regexp.MustCompile(`(?i)^(?:bestell|rechnungs|liefer)?datum$`)
	numberHead   = regexp.MustCompile(`(?i)^(?:bestell|rechnungs|auftrags)-?\s*(?:nr|nummer)\.?$`)
	dateInline   = regexp.MustCompile(`(?i)(?:bestell|rechnungs|reservierungs|liefer)datum:?\s+(\d{2})[./](\d{2})[./](\d{4})`)
	numberInline = regexp.MustCompile(`(?i)(?:rechnung|bestell|reservierungs|auftrags)[ -]?(?:nr\.?|nummer|id):?\s+([A-Z0-9][A-Z0-9-]{3,})`)
	// The table of products starts at its heading: "Name Menge Preis", "Produkt Anzahl (g)".
	tableHead = regexp.MustCompile(`(?i)^(?:produkt|name|artikel)\s.*(?:menge|anzahl)`)
	// Rows in the table that are sums, not things bought.
	sumWords = regexp.MustCompile(`(?i)betrag|bezahlt|summe|gesamt`)
)

// Read reads an invoice's text. What it cannot find it leaves zero; an
// invoice with no product lines is not one wits can record a fill from.
func Read(text string) Invoice {
	var inv Invoice
	var b *block
	table := false
	prev := ""
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\x00", " "), "\n") {
		line := strings.Join(strings.Fields(raw), " ")
		if line == "" {
			continue
		}
		inv.head(line, prev)
		prev = line
		switch {
		case pzn.MatchString(line):
			inv.close(b)
			b = &block{head: pzn.FindStringSubmatch(line)[1]}
		case b != nil && b.takes(line):
			b.add(line)
		default:
			inv.close(b)
			b = nil
			table = inv.body(line, table)
		}
	}
	inv.close(b)
	return inv
}

// head takes the date and the number from a line, given the one before it.
func (inv *Invoice) head(line, prev string) {
	if inv.Date.IsZero() {
		if m := dateInline.FindStringSubmatch(line); m != nil {
			inv.Date = date(m[1], m[2], m[3])
		} else if m := day.FindStringSubmatch(line); m != nil && dateHead.MatchString(prev) {
			inv.Date = date(m[1], m[2], m[3])
		}
	}
	if inv.Number == "" {
		if m := numberInline.FindStringSubmatch(line); m != nil && strings.ContainsAny(m[1], "0123456789") {
			inv.Number = m[1]
		} else if numberHead.MatchString(prev) {
			inv.Number = line
		}
	}
}

// body takes what one line says about the products and the totals; table
// is whether the line stands among the products, and what it returns is
// whether the next one does.
func (inv *Invoice) body(line string, table bool) bool {
	switch {
	case tableHead.MatchString(line):
		return true
	case vat.MatchString(line):
		m := vat.FindStringSubmatch(line)
		inv.Rate, _ = strconv.Atoi(m[1])
		inv.VAT = cents(m[2])
		return false
	case gross.MatchString(line):
		inv.Gross = cents(gross.FindStringSubmatch(line)[1])
		return false
	case net.MatchString(line):
		inv.Net = cents(net.FindStringSubmatch(line)[1])
		return false
	case product.MatchString(line):
		m := product.FindStringSubmatch(line)
		inv.add(m[1], m[2], m[3], m[4])
	case table && priced.MatchString(line) && !sumWords.MatchString(line):
		m := priced.FindStringSubmatch(line)
		if total := cents(m[3]); total > 0 {
			inv.Other = append(inv.Other, Other{Name: strings.TrimSpace(m[1]), Total: total})
		}
	}
	return table
}

// add takes a product line as printed: its name, grams, price a gram and total.
func (inv *Invoice) add(name, grams, perGram, total string) {
	g, err := strconv.ParseFloat(strings.Replace(grams, ",", ".", 1), 64)
	if err != nil || g <= 0 {
		return
	}
	inv.Lines = append(inv.Lines, Line{Name: name, Grams: g, PerGram: cents(perGram), Total: cents(total)})
}

func date(d, m, y string) time.Time {
	t, _ := time.ParseInLocation("02.01.2006", d+"."+m+"."+y, time.Local)
	return t
}

// cents reads an amount as printed into cents: 1.234,56 is 123456, and so is
// 1234.56 where the point is the decimal one.
func cents(s string) int64 {
	var whole, frac string
	if strings.Contains(s, ",") {
		whole, frac, _ = strings.Cut(strings.ReplaceAll(s, ".", ""), ",")
	} else {
		whole, frac, _ = strings.Cut(s, ".")
	}
	w, _ := strconv.ParseInt(whole, 10, 64)
	f, _ := strconv.ParseInt(frac, 10, 64)
	return w*100 + f
}
