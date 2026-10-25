// Package invoice reads a pharmacy's invoice for a fill from its text: each
// product with its grams, its price a gram and the line's total, the date it
// was ordered, the order's number, and the totals with and without VAT.
//
// It reads text, not PDF: the browser takes the text out of the file with
// pdf.js, on the device, and sends only that — as the label's photograph is
// read on the device and only its text sent. The text is a line per row of the
// invoice, its cells apart by whitespace. The words looked for are a German
// pharmacy's (Menge, Gesamtbetrag, USt), since those are the invoices there are.
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

// Invoice is what the text of one says about a fill.
type Invoice struct {
	Date   time.Time // the day ordered; zero when none was found
	Number string
	Lines  []Line
	Net    int64 // cents, without VAT; zero when not printed
	VAT    int64 // cents
	Rate   int   // percent
	Gross  int64 // cents, with VAT; zero when not printed
}

// Sum is the lines' totals added up, which on a German pharmacy's invoice is
// the total with VAT.
func (inv Invoice) Sum() int64 {
	var sum int64
	for _, l := range inv.Lines {
		sum += l.Total
	}
	return sum
}

// euro is an amount as printed: 1.234,56 or 5,84, with the sign or without.
const euro = `(\d{1,3}(?:\.\d{3})*,\d{2}|\d+,\d{2})\s*€?`

var (
	// A product line ends in its amount, its price a gram and its total:
	// "NICE 32/1 MOS Morning Sun   20 g   5,80 €   116,00 €".
	product = regexp.MustCompile(`^(.+?)\s+(\d+(?:[.,]\d+)?)\s*g\s+` + euro + `\s+` + euro + `\s*$`)
	net     = regexp.MustCompile(`(?i)(?:exkl|ohne|netto).*?\s` + euro + `\s*$`)
	gross   = regexp.MustCompile(`(?i)(?:inkl|brutto).*?\s` + euro + `\s*$`)
	vat     = regexp.MustCompile(`(?i)^(?:USt|MwSt)\.?\s*(\d{1,2})\s*%\s+` + euro + `\s*$`)
	day     = regexp.MustCompile(`^(\d{2})\.(\d{2})\.(\d{4})$`)
	// The date and the number stand on the line under their heading.
	dateHead   = regexp.MustCompile(`(?i)^(?:bestell|rechnungs|liefer)?datum$`)
	numberHead = regexp.MustCompile(`(?i)^(?:bestell|rechnungs|auftrags)-?\s*(?:nr|nummer)\.?$`)
)

// Read reads an invoice's text. What it cannot find it leaves zero; an
// invoice with no product lines is not one wits can record a fill from.
func Read(text string) Invoice {
	var inv Invoice
	var prev string
	for _, raw := range strings.Split(text, "\n") {
		line := strings.Join(strings.Fields(raw), " ")
		if line == "" {
			continue
		}
		inv.read(line, prev)
		prev = line
	}
	return inv
}

// read takes what one line says, given the one before it.
func (inv *Invoice) read(line, prev string) {
	switch {
	case dateHead.MatchString(prev) && day.MatchString(line) && inv.Date.IsZero():
		m := day.FindStringSubmatch(line)
		inv.Date, _ = time.ParseInLocation("02.01.2006", m[1]+"."+m[2]+"."+m[3], time.Local)
	case numberHead.MatchString(prev) && inv.Number == "":
		inv.Number = line
	case vat.MatchString(line):
		m := vat.FindStringSubmatch(line)
		inv.Rate, _ = strconv.Atoi(m[1])
		inv.VAT = cents(m[2])
	case gross.MatchString(line):
		inv.Gross = cents(gross.FindStringSubmatch(line)[1])
	case net.MatchString(line):
		inv.Net = cents(net.FindStringSubmatch(line)[1])
	case product.MatchString(line):
		m := product.FindStringSubmatch(line)
		grams, err := strconv.ParseFloat(strings.Replace(m[2], ",", ".", 1), 64)
		if err != nil || grams <= 0 {
			return
		}
		inv.Lines = append(inv.Lines, Line{Name: m[1], Grams: grams, PerGram: cents(m[3]), Total: cents(m[4])})
	}
}

// cents reads an amount as printed into cents: 1.234,56 is 123456.
func cents(s string) int64 {
	whole, frac, _ := strings.Cut(strings.ReplaceAll(s, ".", ""), ",")
	w, _ := strconv.ParseInt(whole, 10, 64)
	f, _ := strconv.ParseInt(frac, 10, 64)
	return w*100 + f
}
