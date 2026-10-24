package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/repo"
	"github.com/TheDonDope/wits/pkg/seal"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// server is a repository with a fill, a grind, a device and a session in it,
// served the way wits-server serves it.
func server(t *testing.T, ui string) (*Server, *repo.Repo) {
	t.Helper()
	dir := t.TempDir()
	r, err := repo.Init(dir)
	require.NoError(t, err)
	_, err = r.UpdateDevices(func(d *catalog.Devices) error {
		return d.Add(&catalog.Device{Name: "Volcano", MaxTemp: 230, DefaultTemp: 185})
	})
	require.NoError(t, err)
	ws, err := workspace.Read(r)
	require.NoError(t, err)
	start := time.Now().AddDate(0, 0, -5)
	_, _, _, err = ws.Recorder.Buy("Enua 22/1 Wedding Cake", "", 20, start)
	require.NoError(t, err)
	_, err = ws.Recorder.Grind("wcake-221", 2, start.AddDate(0, 0, 1))
	require.NoError(t, err)
	_, err = ws.Recorder.Session("wcake-221", 0.5, start.AddDate(0, 0, 2), "volcano", 0, "")
	require.NoError(t, err)
	return New(r, ui), r
}

// call makes a request the way a browser on this machine would.
func call(t *testing.T, s *Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "http://127.0.0.1:7341"+path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w
}

func decodeInto[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &v), w.Body.String())
	return v
}

func TestOverview(t *testing.T) {
	s, _ := server(t, "")

	w := call(t, s, "GET", "/api/overview", "")

	require.Equal(t, http.StatusOK, w.Code)
	o := decodeInto[overview](t, w)
	assert.True(t, o.ChainOK)
	require.NotNil(t, o.Fill)
	assert.Equal(t, 1, o.Fill.Number)
	assert.Equal(t, 20.0, o.Fill.Dispensed)
	assert.Equal(t, 18.0, o.Fill.Remaining, "The fill's own sealed grams")
	require.Len(t, o.Fill.Products, 1)
	assert.Equal(t, fillProduct{Slug: "wcake-221", Name: "Enua 22/1 Wedding Cake", Dispensed: 20, Sealed: 18, Stash: 1.5},
		o.Fill.Products[0], "The stash, as far as it is this fill's")
	assert.Equal(t, shelf{Storage: 18, Stash: 1.5, Jars: 1}, o.Shelf)
	assert.NotEmpty(t, o.Fill.ShelfProjection.History)
	assert.NotNil(t, o.Fill.ShelfProjection.EmptyOn, "A rate to project from")
	require.NotNil(t, o.Fill.Refill, "When to order, from the same projection")
	assert.Equal(t, o.Fill.ShelfProjection.EmptyOn.Format(time.DateOnly), o.Fill.Refill.EmptyOn,
		"The refill's empty day is the projection's")
	assert.Equal(t, 3, o.Fill.Refill.LeadDays, "The lead time starts at three days")
}

func TestReads(t *testing.T) {
	s, _ := server(t, "")

	journal := decodeInto[struct {
		Entries []entry `json:"entries"`
		Total   int     `json:"total"`
	}](t, call(t, s, "GET", "/api/journal?limit=2", ""))
	assert.Equal(t, 3, journal.Total)
	require.Len(t, journal.Entries, 2, "limit caps the entries")
	assert.Equal(t, "sesh", journal.Entries[0].Type, "newest first")

	jars := decodeInto[[]jar](t, call(t, s, "GET", "/api/jars", ""))
	require.Len(t, jars, 1)
	assert.Equal(t, 20.0, jars[0].Bought)
	assert.Equal(t, 2.0, jars[0].Ground)

	cycles := decodeInto[[]cycle](t, call(t, s, "GET", "/api/cycles", ""))
	require.Len(t, cycles, 1)
	assert.True(t, cycles[0].Open)
	assert.Equal(t, 18.0, cycles[0].Remaining)

	devices := decodeInto[[]device](t, call(t, s, "GET", "/api/devices", ""))
	require.Len(t, devices, 1)
	assert.Equal(t, 1, devices[0].Sessions)

	days := decodeInto[[]day](t, call(t, s, "GET", "/api/days", ""))
	require.Len(t, days, 3)
	assert.Equal(t, 2.0, days[1].Ground)
	assert.Equal(t, 0.5, days[2].Seshed)

	assert.Equal(t, http.StatusBadRequest, call(t, s, "GET", "/api/journal?limit=x", "").Code)
	assert.Equal(t, http.StatusNotFound, call(t, s, "GET", "/api/nothing", "").Code)
}

func TestWrites(t *testing.T) {
	s, _ := server(t, "")

	w := call(t, s, "POST", "/api/grind", `{"product":"wcake","grams":1.25}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	ground := decodeInto[written](t, w)
	assert.Equal(t, "ground 1.25 g of Enua 22/1 Wedding Cake", ground.Message)

	w = call(t, s, "POST", "/api/undo", `{"hash":"`+ground.Entry.Hash+`"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, "undid the grind of 1.25 g of Enua 22/1 Wedding Cake", decodeInto[written](t, w).Message)

	w = call(t, s, "POST", "/api/buy", `{"name":"Cannamedical 28/1 Lemon Cookie","grams":10,"date":"2026-01-02"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Contains(t, decodeInto[written](t, w).Message, "new product")

	w = call(t, s, "POST", "/api/sesh", `{"product":"wcake","grams":0.3,"device":"volcano"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, 185, decodeInto[written](t, w).Entry.Temperature, "The device's default")

	w = call(t, s, "POST", "/api/weigh", `{"product":"wcake","account":"storage","weighed":17}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, "lost 1.00 g of Enua 22/1 Wedding Cake in storage", decodeInto[written](t, w).Message)
}

func TestWritesAreRefusedWhereTheCommandsRefuseThem(t *testing.T) {
	s, _ := server(t, "")

	for name, c := range map[string]struct{ path, body, says string }{
		"TooMuch":      {"/api/grind", `{"product":"wcake","grams":500}`, "cannot take"},
		"Finer":        {"/api/grind", `{"product":"wcake","grams":0.333}`, "finer than a centigram"},
		"BadDate":      {"/api/grind", `{"product":"wcake","grams":1,"date":"soon"}`, "not a date"},
		"UnknownField": {"/api/grind", `{"product":"wcake","grams":1,"gramz":2}`, "unknown field"},
		"PartialBuy":   {"/api/buy", `{"name":"Wedding","grams":1}`, "not a product's full name"},
	} {
		t.Run(name, func(t *testing.T) {
			w := call(t, s, "POST", c.path, c.body)
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), c.says)
		})
	}
}

func TestOnlyThisMachine(t *testing.T) {
	s, _ := server(t, "")

	for _, host := range []string{"evil.example", "192.168.1.20:7341", "wits.local"} {
		req := httptest.NewRequest("GET", "/api/overview", nil)
		req.Host = host
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code, "A page elsewhere reaching it through %s", host)
	}
	for _, host := range []string{"localhost:7341", "127.0.0.1:7341", "[::1]:7341"} {
		req := httptest.NewRequest("GET", "/api/overview", nil)
		req.Host = host
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, host)
	}

	req := httptest.NewRequest("POST", "http://127.0.0.1/api/grind", strings.NewReader("product=wcake&grams=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnsupportedMediaType, w.Code, "A form from another origin cannot write")
}

func TestServesTheInterface(t *testing.T) {
	ui := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(ui, "index.html"), []byte("<app-root>"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(ui, "main.js"), []byte("boot()"), 0600))
	s, _ := server(t, ui)

	assert.Equal(t, "boot()", call(t, s, "GET", "/main.js", "").Body.String())
	assert.Equal(t, "<app-root>", call(t, s, "GET", "/journal", "").Body.String(), "A client route reloads into the app")
	assert.Equal(t, http.StatusOK, call(t, s, "GET", "/api/overview", "").Code, "and the API is still the API")
}

func TestFlush(t *testing.T) {
	s, _ := server(t, "")

	w := call(t, s, "POST", "/api/flush", `{"products":[]}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	out := decodeInto[flushed](t, w)
	assert.Equal(t, "flushed 1 stash: 1.50 g recorded as consumed", out.Message)
	require.Len(t, out.Entries, 1)
	assert.Equal(t, "stash", out.Entries[0].From)

	o := decodeInto[overview](t, call(t, s, "GET", "/api/overview", ""))
	assert.Equal(t, 0.0, o.Shelf.Stash, "every stash is empty")

	w = call(t, s, "POST", "/api/flush", `{"products":["wcake"]}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "nothing in those stashes")
}

func TestEffects(t *testing.T) {
	s, _ := server(t, "")
	ov := decodeInto[struct {
		Unrated []entry `json:"unrated"`
	}](t, call(t, s, "GET", "/api/effects", ""))
	require.Len(t, ov.Unrated, 1, "the one session, waiting")

	w := call(t, s, "POST", "/api/rate", `{"session":"`+ov.Unrated[0].Hash+`","scores":{"overall":4,"sleep":5},"note":"slept"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "rated the session of 0.50 g of Enua 22/1 Wedding Cake")

	log := decodeInto[effectLog](t, call(t, s, "GET", "/api/effects", ""))
	assert.Empty(t, log.Unrated)
	require.Len(t, log.Rated, 1)
	assert.Equal(t, 5, log.Rated[0].Scores["sleep"])
	assert.Equal(t, 1, log.Insights.Rated)
	require.Len(t, log.Insights.ByTemp, 1)
	assert.Equal(t, "180–189", log.Insights.ByTemp[0].Key, "the device's default, 185")

	w = call(t, s, "POST", "/api/rate", `{"session":"`+ov.Unrated[0].Hash+`","scores":{"overall":7}}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAVB(t *testing.T) {
	s, _ := server(t, "")
	before := decodeInto[avb](t, call(t, s, "GET", "/api/avb", ""))
	require.Len(t, before.Waiting, 1, "the volcano's session, waiting to be collected")
	assert.Equal(t, waiting{Device: "volcano", Grams: 0.5, Sessions: 1}, before.Waiting[0])

	w := call(t, s, "POST", "/api/avb/collect", `{"grams":0.15}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, "collected 0.15 g of AVB from 0.50 g: 30% back", decodeInto[flushed](t, w).Message)

	after := decodeInto[avb](t, call(t, s, "GET", "/api/avb", ""))
	assert.Equal(t, 0.15, after.Held)
	assert.Empty(t, after.Waiting, "nothing waits once it is emptied")
	require.Len(t, after.Yields, 1)
	assert.InDelta(t, 0.3, after.Yields[0].Ratio, 1e-9)

	w = call(t, s, "POST", "/api/avb/use", `{"grams":0.5}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, "more than the jar holds")
	w = call(t, s, "POST", "/api/avb/use", `{"grams":0.1,"note":"butter"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, 0.05, decodeInto[avb](t, call(t, s, "GET", "/api/avb", "")).Held)
}

func TestPrices(t *testing.T) {
	s, _ := server(t, "")
	unpriced := decodeInto[[]entry](t, call(t, s, "GET", "/api/unpriced", ""))
	require.Len(t, unpriced, 1)

	w := call(t, s, "POST", "/api/price", `{"purchase":"`+unpriced[0].Hash+`","euros":"170,00"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "170.00 € for 20.00 g of Enua 22/1 Wedding Cake")
	w = call(t, s, "POST", "/api/price", `{"purchase":"`+unpriced[0].Hash+`","euros":"lots"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	c := decodeInto[costs](t, call(t, s, "GET", "/api/costs", ""))
	assert.Equal(t, int64(17000), c.Cents)
	assert.InDelta(t, 850, c.PerGram, 1e-9)
	assert.Equal(t, 0, c.Unpriced)
	assert.Empty(t, decodeInto[[]entry](t, call(t, s, "GET", "/api/unpriced?all=1", "")))
}

func TestReport(t *testing.T) {
	s, _ := server(t, "")

	w := call(t, s, "GET", "/api/report.pdf", "")

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
	assert.True(t, strings.HasPrefix(w.Body.String(), "%PDF-"))
	assert.Equal(t, http.StatusBadRequest, call(t, s, "GET", "/api/report.pdf?from=2026-10-31&to=2026-10-01", "").Code)
	assert.Equal(t, http.StatusBadRequest, call(t, s, "GET", "/api/report.pdf?from=soon", "").Code)
}

func TestCarry(t *testing.T) {
	s, _ := server(t, "")

	w := call(t, s, "GET", "/api/carry", "")

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	out := decodeInto[struct {
		Statement, URL, Fingerprint, QR string
		Items                           []carryItem
	}](t, w)
	require.Len(t, out.Items, 1)
	assert.Equal(t, 18.0, out.Items[0].Sealed)
	assert.True(t, strings.HasPrefix(out.QR, "<svg"))
	c, err := seal.ParseCarry(out.URL)
	require.NoError(t, err)
	assert.NoError(t, c.Verify(), "The address holds the signed statement")
}
