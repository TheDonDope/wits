// Package today is the day so far: its sessions, each with the THC and CBD in
// the grams put through the device — grams × the strain's per cent × 10, in
// milligrams — and what else the day saw: grinding, a fill picked up, AVB. It
// is what was in the material, not what reached the body: a vaporizer
// releases part of it, and a lung takes part of that. A strain with no
// strength written down is counted, not guessed at.
package today

import (
	"cmp"
	"slices"
	"time"

	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// usualDays is how far back "a usual day" looks: the days with a session in it.
const usualDays = 30

// Today is one day's record.
type Today struct {
	Day      time.Time
	Sessions []Session
	// Seshed is the grams through a device; THC and CBD the milligrams in them.
	Seshed   float64
	THC, CBD float64
	// Unknown is how many sessions were of a strain with no THC written down.
	Unknown   int
	Ground    float64
	Grinds    []Amount
	Picked    []Amount
	Collected float64
	UsedAVB   float64
	Strains   []Share
	Devices   []Share
	Usual     Usual
}

// Session is one session of the day, and what was in it.
type Session struct {
	Hash     string
	At       time.Time
	Name     string
	Grams    float64
	THCPct   float64
	CBDPct   float64
	THC, CBD float64 // milligrams
	Known    bool
	Device   string
	Temp     int
	Mode     string
	// Overall is its overall rating, 0 when it has none.
	Overall int
}

// Amount is grams of a product.
type Amount struct {
	Name  string
	Grams float64
}

// Share is one strain's or one device's part of the day.
type Share struct {
	Name     string
	Sessions int
	Grams    float64
	THC      float64
}

// Usual is a usual day of the last thirty with a session: the mean of them.
type Usual struct {
	Days   int
	Seshed float64
	THC    float64
}

// Of is the day at's falls on, as the workspace stands.
func Of(ws *workspace.Workspace, at time.Time) Today {
	day := dateOf(at)
	t := Today{Day: midnight(at)}
	rated := effects.Latest(ws.Ratings)
	var seshed, ground, collected, used journal.Centigrams
	for _, e := range ws.State.Standing {
		if dateOf(e.OccurredAt) != day {
			continue
		}
		switch e.Type {
		case journal.Sesh:
			s := session(ws, e, rated)
			t.Sessions = append(t.Sessions, s)
			seshed += journal.CG(e.Grams)
		case journal.Grind:
			t.Grinds = append(t.Grinds, Amount{ws.ProductName(e.Product), e.Grams})
			ground += journal.CG(e.Grams)
		case journal.Purchase:
			t.Picked = append(t.Picked, Amount{ws.ProductName(e.Product), e.Grams})
		case journal.AVBCollect:
			collected += journal.CG(e.Grams)
		case journal.AVBUse:
			used += journal.CG(e.Grams)
		}
	}
	t.Seshed, t.Ground, t.Collected, t.UsedAVB = seshed.Grams(), ground.Grams(), collected.Grams(), used.Grams()
	sums(&t)
	t.Usual = usual(ws, day, at)
	return t
}

// session is one session with its strain's strength and its rating.
func session(ws *workspace.Workspace, e journal.Event, rated map[string]effects.Rating) Session {
	s := Session{Hash: e.Hash, At: e.OccurredAt, Name: ws.ProductName(e.Product), Grams: e.Grams, Temp: e.Temperature,
		Mode: e.Mode, Device: e.Device}
	if p, err := ws.Products.Find(e.Product); err == nil && p != nil && p.THC > 0 {
		s.THCPct, s.CBDPct, s.Known = p.THC, p.CBD, true
		s.THC, s.CBD = mg(e.Grams, p.THC), mg(e.Grams, p.CBD)
	}
	if d, err := ws.Devices.Find(e.Device); err == nil && d != nil {
		s.Device = d.Name
	}
	if r, ok := rated[e.Hash]; ok {
		s.Overall = r.Scores[effects.Overall]
	}
	return s
}

// mg is the milligrams of a cannabinoid in grams of a strain that is pct per cent of it.
func mg(grams, pct float64) float64 { return ledger.Round(grams * pct * 10) }

// sums totals the sessions, by strain and by device.
func sums(t *Today) {
	strains, devices := map[string]*Share{}, map[string]*Share{}
	add := func(by map[string]*Share, name string, s Session) {
		sh := by[name]
		if sh == nil {
			sh = &Share{Name: name}
			by[name] = sh
		}
		sh.Sessions++
		sh.Grams = ledger.Round(sh.Grams + s.Grams)
		sh.THC = ledger.Round(sh.THC + s.THC)
	}
	for _, s := range t.Sessions {
		t.THC, t.CBD = ledger.Round(t.THC+s.THC), ledger.Round(t.CBD+s.CBD)
		if !s.Known {
			t.Unknown++
		}
		add(strains, s.Name, s)
		device := s.Device
		if device == "" {
			device = "no device named"
		}
		add(devices, device, s)
	}
	// Strains are told by the THC in them, devices by the grams through them.
	t.Strains = sorted(strains, func(s Share) float64 { return s.THC })
	t.Devices = sorted(devices, func(s Share) float64 { return s.Grams })
}

// sorted is the shares, the most by first, then the most grams, then by name.
func sorted(in map[string]*Share, by func(Share) float64) []Share {
	out := make([]Share, 0, len(in))
	for _, s := range in {
		out = append(out, *s)
	}
	slices.SortFunc(out, func(a, b Share) int {
		return cmp.Or(cmp.Compare(by(b), by(a)), cmp.Compare(b.Grams, a.Grams), cmp.Compare(a.Name, b.Name))
	})
	return out
}

// usual is the mean of the last thirty days before today that had a session.
func usual(ws *workspace.Workspace, today string, at time.Time) Usual {
	since := dateOf(midnight(at).AddDate(0, 0, -usualDays))
	grams, thc := map[string]float64{}, map[string]float64{}
	for _, e := range ws.State.Standing {
		d := dateOf(e.OccurredAt)
		if e.Type != journal.Sesh || d >= today || d < since {
			continue
		}
		grams[d] += e.Grams
		if p, err := ws.Products.Find(e.Product); err == nil && p != nil {
			thc[d] += mg(e.Grams, p.THC)
		}
	}
	u := Usual{Days: len(grams)}
	if u.Days == 0 {
		return u
	}
	for d := range grams {
		u.Seshed += grams[d]
		u.THC += thc[d]
	}
	u.Seshed, u.THC = ledger.Round(u.Seshed/float64(u.Days)), ledger.Round(u.THC/float64(u.Days))
	return u
}

// dateOf is the calendar day an instant falls on where it was written.
func dateOf(t time.Time) string { return t.Format(time.DateOnly) }

func midnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
