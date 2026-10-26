// Package api serves a repository over HTTP; this file serves the year on one
// card, as wits review gathers it, for Insights → Year.
package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/TheDonDope/wits/pkg/review"
)

// yearOut is the year as the client reads it: every figure the year's own,
// grams in grams and money in cents.
type yearOut struct {
	Year        int          `json:"year"`
	Years       []int        `json:"years"`
	From        string       `json:"from"`
	To          string       `json:"to"`
	Days        int          `json:"days"`
	Running     bool         `json:"running"`
	Ground      float64      `json:"ground"`
	GrindDays   int          `json:"grind_days"`
	PerGrindDay float64      `json:"per_grind_day"`
	Sessions    int          `json:"sessions"`
	Seshed      float64      `json:"seshed"`
	Dispensed   float64      `json:"dispensed"`
	Pickups     int          `json:"pickups"`
	Products    int          `json:"products"`
	Months      [12]float64  `json:"months"`
	Daily       []day        `json:"daily"`
	Strains     []yearUse    `json:"strains"`
	Busiest     *day         `json:"busiest"`
	Break       *yearBreak   `json:"break"`
	Best        *yearRated   `json:"best_temperature"`
	Liked       *yearRated   `json:"best_strain"`
	Device      *yearDevice  `json:"device"`
	Terpene     *yearTerp    `json:"terpene"`
	Paid        int64        `json:"paid"`
	Priced      float64      `json:"priced"`
	PerGram     float64      `json:"per_gram"`
	Dearest     *yearBought  `json:"dearest"`
	Cheapest    *yearBought  `json:"cheapest"`
	Bought      []yearBought `json:"bought"`
}

type yearUse struct {
	Name   string  `json:"name"`
	Ground float64 `json:"ground"`
	Share  float64 `json:"share"`
}

type yearBreak struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Days   int    `json:"days"`
	Breaks int    `json:"breaks"`
}

type yearRated struct {
	Label string  `json:"label"`
	Mean  float64 `json:"mean"`
	Count int     `json:"count"`
}

type yearDevice struct {
	Name     string  `json:"name"`
	Sessions int     `json:"sessions"`
	Seshed   float64 `json:"seshed"`
}

type yearTerp struct {
	Name    string   `json:"name"`
	Flavors []string `json:"flavors"`
	Share   float64  `json:"share"`
}

type yearBought struct {
	At      time.Time `json:"at"`
	Name    string    `json:"name"`
	Seller  string    `json:"seller"`
	Grams   float64   `json:"grams"`
	Cents   int64     `json:"cents"`
	PerGram float64   `json:"per_gram"`
}

// year is the year on one card: ?year=, this year when not given.
func (s *Server) year(w http.ResponseWriter, r *http.Request) {
	ws, err := s.snapshot()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	y := ws.OpenedAt.Year()
	if q := r.URL.Query().Get("year"); q != "" {
		if y, err = strconv.Atoi(q); err != nil {
			fail(w, http.StatusBadRequest, errors.New("?year= is a year, as 2025"))
			return
		}
	}
	reply(w, http.StatusOK, yearOf(review.Year(ws, y), review.Years(ws)))
}

// yearOf is the review as the client reads it.
func yearOf(rv review.Review, years []int) yearOut {
	out := yearOut{Year: rv.Year, Years: years, From: rv.From.Format(time.DateOnly), To: rv.To.Format(time.DateOnly),
		Days: rv.Days, Running: rv.Running, Ground: rv.Ground, GrindDays: rv.GrindDays, PerGrindDay: rv.PerGrindDay,
		Sessions: rv.Sessions, Seshed: rv.Seshed, Dispensed: rv.Dispensed, Pickups: rv.Pickups, Products: rv.Products,
		Months: rv.Months, Daily: make([]day, 0, len(rv.Daily)), Strains: make([]yearUse, 0, len(rv.Strains)),
		Paid: rv.Paid, Priced: rv.Priced, PerGram: rv.PerGram(), Bought: make([]yearBought, 0, len(rv.Bought))}
	if out.Years == nil {
		out.Years = []int{}
	}
	for _, d := range rv.Daily {
		out.Daily = append(out.Daily, day{Date: d.Day.Format(time.DateOnly), Ground: d.Ground, Seshed: d.Seshed})
	}
	for _, u := range rv.Strains {
		out.Strains = append(out.Strains, yearUse(u))
	}
	for _, b := range rv.Bought {
		out.Bought = append(out.Bought, yearBought(b))
	}
	if rv.Busiest.Ground > 0 {
		out.Busiest = &day{Date: rv.Busiest.Day.Format(time.DateOnly), Ground: rv.Busiest.Ground, Seshed: rv.Busiest.Seshed}
	}
	findingsOf(rv, &out)
	return out
}

// findingsOf is the year's single findings, those it has.
func findingsOf(rv review.Review, out *yearOut) {
	if b := rv.Break; b != nil {
		out.Break = &yearBreak{From: b.From.Format(time.DateOnly), To: b.To.Format(time.DateOnly), Days: b.Days, Breaks: b.Breaks}
	}
	if b := rv.Best; b != nil {
		out.Best = &yearRated{Label: b.Label, Mean: b.Mean, Count: b.Count}
	}
	if l := rv.Liked; l != nil {
		out.Liked = &yearRated{Label: l.Name, Mean: l.Mean, Count: l.Count}
	}
	if d := rv.Device; d != nil {
		out.Device = &yearDevice{Name: d.Name, Sessions: d.Sessions, Seshed: d.Seshed}
	}
	if t := rv.Terpene; t != nil {
		out.Terpene = &yearTerp{Name: t.Name, Flavors: t.Flavors, Share: t.Share}
	}
	if d := rv.Dearest; d != nil {
		b := yearBought(*d)
		out.Dearest = &b
	}
	if c := rv.Cheapest; c != nil {
		b := yearBought(*c)
		out.Cheapest = &b
	}
}
