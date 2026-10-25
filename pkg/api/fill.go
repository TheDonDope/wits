// Package api serves a repository over HTTP; this file records a fill of
// several products with their prices, and reads a pharmacy's invoice into
// one. Both are the phone's as much as the computer's.
package api

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/TheDonDope/wits/pkg/invoice"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/prices"
	"github.com/TheDonDope/wits/pkg/record"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// germanVAT is the VAT a fill's prices are given before, in percent: medical
// cannabis is taxed at the full rate in Germany.
const germanVAT = 19

// fillLine is one product of a fill as the form sends it. Its price is either
// the line's total, as an invoice prints it — already with VAT — or a price a
// gram, which VAT is added to when the fill says so. Neither: no price.
type fillLine struct {
	Name    string  `json:"name"`
	Slug    string  `json:"slug"`
	Grams   float64 `json:"grams"`
	PerGram string  `json:"per_gram"`
	Total   string  `json:"total"`
	Note    string  `json:"note"`
}

type fillRequest struct {
	Lines []fillLine `json:"lines"`
	VAT   bool       `json:"vat"`
	Date  string     `json:"date"`
	Note  string     `json:"note"`
}

// recordFill records a fill of one product or several, each with its price.
func recordFill(ws *workspace.Workspace, r *http.Request) ([]journal.Event, string, error) {
	var req fillRequest
	if err := decode(r, &req); err != nil {
		return nil, "", err
	}
	at, err := when(req.Date)
	if err != nil {
		return nil, "", err
	}
	lines := make([]record.FillLine, len(req.Lines))
	for i, l := range req.Lines {
		cents, err := lineCents(l, req.VAT)
		if err != nil {
			return nil, "", fmt.Errorf("line %d, %s: %w", i+1, l.Name, err)
		}
		lines[i] = record.FillLine{Name: l.Name, Slug: l.Slug, Grams: l.Grams, Cents: cents, Note: l.Note}
	}
	filled, err := ws.Recorder.Fill(lines, at, req.Note)
	if len(filled) == 0 || filled[0].Event.Hash == "" {
		return nil, "", err
	}
	events := make([]journal.Event, len(filled))
	for i, f := range filled {
		events[i] = f.Event
	}
	// A price that failed after the purchases were written is said, not
	// refused: the fill stands.
	msg := fillMessage(filled)
	if err != nil {
		msg += " — " + err.Error()
	}
	return events, msg, nil
}

// lineCents is what a line cost: its total as given, or its grams at its
// price a gram, with VAT on top when asked for. Zero is no price.
func lineCents(l fillLine, vat bool) (int64, error) {
	if strings.TrimSpace(l.Total) != "" {
		return prices.ParseEuros(l.Total)
	}
	if strings.TrimSpace(l.PerGram) == "" {
		return 0, nil
	}
	perGram, err := prices.ParseEuros(l.PerGram)
	if err != nil {
		return 0, err
	}
	cents := float64(perGram) * l.Grams
	if vat {
		cents *= 1 + germanVAT/100.0
	}
	return int64(math.Round(cents)), nil
}

// fillMessage says what a fill recorded: the grams, of how many products, and
// what they cost where that is known.
func fillMessage(filled []record.Filled) string {
	var grams journal.Centigrams
	var cents int64
	priced, added := 0, 0
	for _, f := range filled {
		grams += journal.CG(f.Event.Grams)
		cents += f.Cents
		if f.Cents > 0 {
			priced++
		}
		if f.Added {
			added++
		}
	}
	msg := fmt.Sprintf("picked up %.2f g", grams.Grams())
	switch {
	case len(filled) == 1 && added == 1:
		msg += " of " + filled[0].Product.Name + ", a new product"
	case len(filled) == 1:
		msg += " of " + filled[0].Product.Name
	case added > 0:
		msg += fmt.Sprintf(" of %d products, %d of them new", len(filled), added)
	default:
		msg += fmt.Sprintf(" of %d products", len(filled))
	}
	switch {
	case priced == len(filled):
		msg += ", for " + prices.Format(cents)
	case priced > 0:
		msg += fmt.Sprintf(", %s for the %d with a price", prices.Format(cents), priced)
	}
	return msg
}

// invoiceReading is what an invoice says, as the fill form is filled from it:
// each line's product named as the jar it already is, where it is one.
type invoiceReading struct {
	Date   string        `json:"date,omitempty"`
	Number string        `json:"number,omitempty"`
	Lines  []invoiceLine `json:"lines"`
	Net    int64         `json:"net_cents"`
	VAT    int64         `json:"vat_cents"`
	Rate   int           `json:"rate"`
	Gross  int64         `json:"gross_cents"`
	Sum    int64         `json:"sum_cents"`
}

type invoiceLine struct {
	Name    string  `json:"name"`
	Slug    string  `json:"slug,omitempty"`
	Printed string  `json:"printed"`
	Grams   float64 `json:"grams"`
	PerGram int64   `json:"per_gram_cents"`
	Total   int64   `json:"total_cents"`
}

// readInvoice reads the text the browser took out of an invoice. It records
// nothing.
func (s *Server) readInvoice(w http.ResponseWriter, r *http.Request) {
	if !jsonBody(w, r) {
		return
	}
	var req struct {
		Text string `json:"text"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	ws, err := s.snapshot()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	inv := invoice.Read(req.Text)
	if len(inv.Lines) == 0 {
		fail(w, http.StatusUnprocessableEntity, errors.New("no product lines found: is it a pharmacy's invoice for a fill?"))
		return
	}
	out := invoiceReading{Number: inv.Number, Lines: make([]invoiceLine, len(inv.Lines)),
		Net: inv.Net, VAT: inv.VAT, Rate: inv.Rate, Gross: inv.Gross, Sum: inv.Sum()}
	if !inv.Date.IsZero() {
		out.Date = inv.Date.Format("2006-01-02")
	}
	for i, l := range inv.Lines {
		line := invoiceLine{Name: l.Name, Printed: l.Name, Grams: l.Grams, PerGram: l.PerGram, Total: l.Total}
		if p := ws.Products.Matching(l.Name); p != nil {
			line.Name, line.Slug = p.Name, p.Slug
		}
		out.Lines[i] = line
	}
	reply(w, http.StatusOK, out)
}
