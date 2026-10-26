// Package api serves a repository over HTTP; this file serves a trip's
// packing — the grams to take, from which jars — signed as a statement of
// exactly what is packed, as wits pack makes it, for the Trip page.
package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/TheDonDope/wits/pkg/pack"
	"github.com/TheDonDope/wits/pkg/report"
	"github.com/TheDonDope/wits/pkg/seal"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// packOut is a plan as the client reads it, with its signed statement.
type packOut struct {
	Days        int        `json:"days"`
	Margin      int        `json:"margin"`
	Mode        pack.Mode  `json:"mode"`
	Rate        float64    `json:"rate"`
	Need        float64    `json:"need"`
	Packed      float64    `json:"packed"`
	Shelf       float64    `json:"shelf"`
	Short       bool       `json:"short"`
	Lines       []packLine `json:"lines"`
	Statement   string     `json:"statement"`
	URL         string     `json:"url"`
	Fingerprint string     `json:"fingerprint"`
	QR          string     `json:"qr"`
}

// packLine is what one jar gives the trip, and what it held before.
type packLine struct {
	Name       string      `json:"name"`
	Stash      float64     `json:"stash"`
	Sealed     float64     `json:"sealed"`
	HeldStash  float64     `json:"held_stash"`
	HeldSealed float64     `json:"held_sealed"`
	Days       float64     `json:"days"`
	Fills      []carryFill `json:"fills"`
}

// pack plans ?days= away with ?margin= per cent on top, ?mode=even or
// oldest, and signs it. Nothing is recorded.
func (s *Server) pack(w http.ResponseWriter, r *http.Request) {
	ws, err := s.snapshot()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	q := r.URL.Query()
	days, margin, err := packArgs(q.Get("days"), q.Get("margin"))
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	p, err := pack.Make(ws, days, margin, pack.Mode(q.Get("mode")))
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	out := packOut{Days: p.Days, Margin: p.Margin, Mode: p.Mode, Rate: p.Rate, Need: p.Need, Packed: p.Packed,
		Shelf: p.Shelf, Short: p.Short, Lines: make([]packLine, 0, len(p.Lines))}
	for _, l := range p.Lines {
		pl := packLine{Name: l.Name, Stash: l.Stash, Sealed: l.Sealed, HeldStash: l.HeldStash, HeldSealed: l.HeldSealed,
			Days: l.Days, Fills: []carryFill{}}
		for _, f := range l.Fills {
			pl.Fills = append(pl.Fills, carryFill(f))
		}
		out.Lines = append(out.Lines, pl)
	}
	if err := signPack(ws, &out, p); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	reply(w, http.StatusOK, out)
}

// packArgs reads the days and the margin, the margin the default when absent.
func packArgs(days, margin string) (int, int, error) {
	d, err := strconv.Atoi(days)
	if err != nil || d < 1 || d > 366 {
		return 0, 0, errors.New("?days= is how many days away, 1 to 366")
	}
	m := pack.DefaultMargin
	if margin != "" {
		if m, err = strconv.Atoi(margin); err != nil || m < 0 || m > 200 {
			return 0, 0, errors.New("?margin= is a per cent on top, 0 to 200")
		}
	}
	return d, m, nil
}

// signPack signs the plan's statement with the sealing key, made the first
// time, and draws its QR code.
func signPack(ws *workspace.Workspace, out *packOut, p pack.Plan) error {
	events, err := ws.Journal().Events()
	if err != nil {
		return err
	}
	if len(events) == 0 || len(p.Lines) == 0 {
		return nil
	}
	key, _, err := seal.Key(ws.Repo.SealKeyPath())
	if err != nil {
		return err
	}
	c := seal.MakeCarry(key, events[len(events)-1].Hash, len(events), ws.OpenedAt, p.Statement())
	out.QR, err = report.QR(c.URL())
	out.Statement, out.URL, out.Fingerprint = c.String(), c.URL(), seal.Fingerprint(c.Key)
	return err
}
