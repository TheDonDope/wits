package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/repo"
)

func TestDevicesAreAddedChangedAndUsed(t *testing.T) {
	s, r := server(t, "")
	add := func(body string) *http.Response {
		t.Helper()
		return call(t, s, "POST", "/api/devices", body).Result()
	}
	require.Equal(t, http.StatusCreated, add(`{"name": "Vestratto Tornado", "category": "vaporizer", "maker": "Vestratto",
		"model": "Tornado", "power": "manual", "signature": "convection", "use": "on-demand", "modes": [{"name": "half soak"}, {"name": "full soak"}],
		"cents": 20800, "shop": "vestratto.com", "bought_on": "2026-03-01", "notes": "the copper coil, 3250 J"}`).StatusCode)
	require.Equal(t, http.StatusCreated, add(`{"name": "YLL 3.0", "category": "induction-heater", "power": "electric",
		"modes": [{"name": "30 W"}, {"name": " 45 W "}, {"name": ""}]}`).StatusCode)
	require.Equal(t, http.StatusCreated, add(`{"name": "Limelight Herb Frolic", "category": "vaporizer", "power": "electric",
		"use": "both", "auto_off": 300, "modes": [{"name": "On demand", "signature": "convection", "temp": 165},
		{"name": "Session", "signature": "hybrid", "temp": 185, "timer": 420}]}`).StatusCode)
	assert.Equal(t, http.StatusBadRequest, add(`{"name": "Odd 2", "modes": [{"name": "x", "signature": "radiation"}]}`).StatusCode,
		"A mode heats in a way there is")
	require.Equal(t, http.StatusCreated, add(`{"name": "Brilliant Cut", "category": "grinder", "maker": "Grinders For Life", "country": "Canada"}`).StatusCode)
	assert.Equal(t, http.StatusBadRequest, add(`{"name": "Wand", "category": "wand"}`).StatusCode, "No such category")
	assert.Equal(t, http.StatusBadRequest, add(`{"name": "Odd", "signature": "radiation"}`).StatusCode, "No such way of heating")
	assert.Equal(t, http.StatusBadRequest, add(`{"name": "YLL 3.0", "category": "induction-heater"}`).StatusCode, "Already there")

	sessionsWithDevices(t, s, r)
}

// sessionsWithDevices records sessions with the devices added, and refuses
// what the devices say cannot be.
func sessionsWithDevices(t *testing.T, s *Server, r *repo.Repo) {
	t.Helper()
	sesh := func(body string) int {
		t.Helper()
		return call(t, s, "POST", "/api/sesh", body).Code
	}
	assert.Equal(t, http.StatusCreated, sesh(`{"product": "wcake-221", "grams": 0.1, "device": "vestratto-tornado",
		"mode": "full soak", "heater": "yll-3-0", "setting": "45 W"}`))
	assert.Equal(t, http.StatusBadRequest, sesh(`{"product": "wcake-221", "grams": 0.1, "device": "vestratto-tornado", "mode": "boost"}`),
		"Not one of its modes")
	assert.Equal(t, http.StatusBadRequest, sesh(`{"product": "wcake-221", "grams": 0.1, "device": "vestratto-tornado", "heater": "brilliant-cut"}`),
		"A grinder heats nothing")
	assert.Equal(t, http.StatusBadRequest, sesh(`{"product": "wcake-221", "grams": 0.1, "device": "brilliant-cut"}`),
		"Nor is a session had with one")

	require.Equal(t, http.StatusCreated, sesh(`{"product": "wcake-221", "grams": 0.1, "device": "limelight-herb-frolic", "mode": "Session"}`))
	ws := must(t, s.snapshot)
	frolic := ws.State.Events[len(ws.State.Events)-1]
	assert.Equal(t, 185, frolic.Temperature, "The mode's usual temperature, none given")
	assert.Equal(t, 420, frolic.Timer, "and its timer")
	last := ws.State.Events[len(ws.State.Events)-2]
	assert.Equal(t, "full soak", last.Mode)
	assert.Equal(t, "yll-3-0", last.Heater)
	assert.Equal(t, "45 W", last.Setting)

	devices := decodeInto[[]device](t, call(t, s, "GET", "/api/devices", ""))
	byslug := map[string]device{}
	for _, d := range devices {
		byslug[d.Slug] = d
	}
	assert.Equal(t, []mode{{Name: "30 W"}, {Name: "45 W"}}, byslug["yll-3-0"].Modes, "Modes trimmed, the empty one gone")
	assert.Equal(t, mode{Name: "Session", Signature: "hybrid", Temp: 185, Timer: 420}, byslug["limelight-herb-frolic"].Modes[1],
		"A mode heats its own way, at its own temperature, for its own time")
	assert.Equal(t, 300, byslug["limelight-herb-frolic"].AutoOff)
	assert.Equal(t, 1, byslug["yll-3-0"].Heated, "It heated one session")
	assert.Equal(t, 1, byslug["vestratto-tornado"].Sessions)
	assert.Equal(t, "vaporizer", byslug["volcano"].Category, "A device from before categories is a vaporizer")

	w := call(t, s, "PUT", "/api/devices/vestratto-tornado", `{"name": "Vestratto Tornado", "category": "vaporizer",
		"power": "manual", "modes": [{"name": "half soak"}, {"name": "full soak"}], "retired": true, "cents": 19900}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	devs, err := catalog.LoadDevices(r.DevicesPath())
	require.NoError(t, err)
	tornado, err := devs.Find("vestratto-tornado")
	require.NoError(t, err)
	assert.True(t, tornado.Retired)
	assert.Equal(t, int64(19900), tornado.Cents)
	assert.Equal(t, http.StatusBadRequest, call(t, s, "PUT", "/api/devices/nothing", `{"name": "x"}`).Code)
}
