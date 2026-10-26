// Package api serves a repository over HTTP; this file weighs a jar short of
// the ledger and puts the difference down to something forgotten — a grind
// for what is sealed, sessions for what is ground — rather than to grams
// lost: what it would record, and recording it.
package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/record"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// forgottenPlan is what a reading short of the ledger would be put down to:
// the entries, not yet written, and the shortfall they make up. A reading
// that is not short has no entries.
type forgottenPlan struct {
	Expected float64 `json:"expected"`
	Short    float64 `json:"short"`
	Entries  []entry `json:"entries"`
	// Why is why a reading short of the ledger is not put down to anything
	// forgotten: more sessions than anybody forgets.
	Why string `json:"why,omitempty"`
	// SizedBy is what forgotten sessions are sized by: sessions, load or day.
	SizedBy string `json:"sized_by,omitempty"`
}

// forgottenAt answers what a reading would be put down to, recording nothing:
// ?product=&account=&weighed=.
func (s *Server) forgottenAt(w http.ResponseWriter, r *http.Request) {
	ws, err := s.snapshot()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	q := r.URL.Query()
	weighed, err := strconv.ParseFloat(q.Get("weighed"), 64)
	if err != nil {
		fail(w, http.StatusBadRequest, errors.New("weighed must be a number of grams"))
		return
	}
	account := journal.Account(q.Get("account"))
	expected, difference, err := ws.Recorder.Difference(q.Get("product"), account, weighed)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	out := forgottenPlan{Expected: expected, Short: ledger.Round(-difference), Entries: []entry{}}
	planned, err := ws.Recorder.Forgotten(q.Get("product"), account, weighed)
	if errors.Is(err, record.ErrNotShort) {
		out.Short = max(out.Short, 0)
		reply(w, http.StatusOK, out)
		return
	}
	if errors.Is(err, record.ErrTooMany) {
		out.Why = err.Error()
		reply(w, http.StatusOK, out)
		return
	}
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	for _, e := range planned {
		out.Entries = append(out.Entries, entryOf(e, ws, nil))
	}
	if account == journal.Stash {
		out.SizedBy = ws.Recorder.SizedBy()
	}
	reply(w, http.StatusOK, out)
}

// weighForgotten records what a reading short of the ledger is put down to.
func weighForgotten(ws *workspace.Workspace, r *http.Request) ([]journal.Event, string, error) {
	var req weighRequest
	if err := decode(r, &req); err != nil {
		return nil, "", err
	}
	events, err := ws.Recorder.RecordForgotten(req.Product, journal.Account(req.Account), req.Weighed)
	if err != nil {
		return nil, "", err
	}
	var grams journal.Centigrams
	for _, e := range events {
		grams += journal.CG(e.Grams)
	}
	name := ws.ProductName(req.Product)
	if len(events) == 1 && events[0].Type == journal.Grind {
		return events, fmt.Sprintf("a forgotten grind of %.2f g of %s", grams.Grams(), name), nil
	}
	sessions := "sessions"
	if len(events) == 1 {
		sessions = "session"
	}
	return events, fmt.Sprintf("%d forgotten %s, %.2f g of %s", len(events), sessions, grams.Grams(), name), nil
}
