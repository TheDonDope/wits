// Package api serves a repository over HTTP; this file serves the day so far
// — its sessions with the THC and CBD in each, and what else it saw — for the
// dashboard's Today, as wits today says it.
package api

import (
	"net/http"
	"time"

	"github.com/TheDonDope/wits/pkg/today"
)

// todayOut is the day as the client reads it: grams in grams, THC and CBD in
// milligrams, in the material put through the device.
type todayOut struct {
	Day       string         `json:"day"`
	Sessions  []todaySession `json:"sessions"`
	Seshed    float64        `json:"seshed"`
	THC       float64        `json:"thc_mg"`
	CBD       float64        `json:"cbd_mg"`
	Unknown   int            `json:"unknown"`
	Ground    float64        `json:"ground"`
	Grinds    []todayAmount  `json:"grinds"`
	Picked    []todayAmount  `json:"picked"`
	Collected float64        `json:"avb_collected"`
	UsedAVB   float64        `json:"avb_used"`
	Strains   []todayShare   `json:"strains"`
	Devices   []todayShare   `json:"devices"`
	Usual     todayUsual     `json:"usual"`
}

type todaySession struct {
	Hash    string    `json:"hash"`
	At      time.Time `json:"at"`
	Name    string    `json:"name"`
	Grams   float64   `json:"grams"`
	THCPct  float64   `json:"thc_pct"`
	CBDPct  float64   `json:"cbd_pct"`
	THC     float64   `json:"thc_mg"`
	CBD     float64   `json:"cbd_mg"`
	Known   bool      `json:"known"`
	Device  string    `json:"device"`
	Temp    int       `json:"temperature"`
	Mode    string    `json:"mode"`
	Overall int       `json:"overall"`
}

type todayAmount struct {
	Name  string  `json:"name"`
	Grams float64 `json:"grams"`
}

type todayShare struct {
	Name     string  `json:"name"`
	Sessions int     `json:"sessions"`
	Grams    float64 `json:"grams"`
	THC      float64 `json:"thc_mg"`
}

type todayUsual struct {
	Days   int     `json:"days"`
	Seshed float64 `json:"seshed"`
	THC    float64 `json:"thc_mg"`
}

// today is the day so far, or ?day= another, YYYY-MM-DD.
func (s *Server) today(w http.ResponseWriter, r *http.Request) {
	ws, err := s.snapshot()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	at := ws.OpenedAt
	if q := r.URL.Query().Get("day"); q != "" {
		if at, err = time.ParseInLocation(time.DateOnly, q, at.Location()); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
	}
	reply(w, http.StatusOK, todayOf(today.Of(ws, at)))
}

func todayOf(d today.Today) todayOut {
	out := todayOut{Day: d.Day.Format(time.DateOnly), Seshed: d.Seshed, THC: d.THC, CBD: d.CBD, Unknown: d.Unknown,
		Ground: d.Ground, Collected: d.Collected, UsedAVB: d.UsedAVB, Usual: todayUsual(d.Usual),
		Sessions: make([]todaySession, 0, len(d.Sessions)), Grinds: amounts(d.Grinds), Picked: amounts(d.Picked),
		Strains: shares(d.Strains), Devices: shares(d.Devices)}
	for _, s := range d.Sessions {
		out.Sessions = append(out.Sessions, todaySession(s))
	}
	return out
}

func amounts(in []today.Amount) []todayAmount {
	out := make([]todayAmount, 0, len(in))
	for _, a := range in {
		out = append(out, todayAmount(a))
	}
	return out
}

func shares(in []today.Share) []todayShare {
	out := make([]todayShare, 0, len(in))
	for _, s := range in {
		out = append(out, todayShare(s))
	}
	return out
}
