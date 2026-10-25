package api

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/research"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// TestJarDetail: one jar in full — its fill and pickup with the price paid,
// its months, and its session on the device it was taken with, rated.
func TestJarDetail(t *testing.T) {
	s, r := server(t, "")
	ws, err := workspace.Read(r)
	require.NoError(t, err)
	var buy, sesh string
	for _, e := range ws.State.Events {
		switch e.Type {
		case "purchase":
			buy = e.Hash
		case "sesh":
			sesh = e.Hash
		}
	}
	_, err = ws.Recorder.Price(buy, 20000, "", time.Now())
	require.NoError(t, err)
	_, err = ws.Recorder.Rate(sesh, map[effects.Dimension]int{effects.Overall: 4, effects.Sleep: 5}, "", time.Now())
	require.NoError(t, err)

	w := call(t, s, http.MethodGet, "/api/jars/wcake-221", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	j := decodeInto[jarDetail](t, w)
	assert.Equal(t, "Enua 22/1 Wedding Cake", j.Name)
	assert.Equal(t, "Wedding Cake", j.Strain)
	assert.Empty(t, j.Terpenes)
	require.Len(t, j.Fills, 1)
	f := j.Fills[0]
	assert.Equal(t, 1, f.Number)
	assert.True(t, f.Open)
	assert.InDelta(t, 20, f.Dispensed, 1e-9)
	assert.InDelta(t, 18, f.Sealed, 1e-9)
	require.Len(t, f.Pickups, 1)
	assert.Equal(t, int64(20000), f.Pickups[0].Cents)
	assert.InDelta(t, 1000, f.Pickups[0].PerGram, 1e-9)
	assert.Equal(t, jarPrice{Cents: 20000, Priced: 20, PerGram: 1000}, j.Price)
	assert.InDelta(t, 0.5, j.Seshed, 1e-9)
	assert.NotEmpty(t, j.Months)

	assert.Equal(t, 1, j.Sessions.Count)
	assert.Equal(t, 1, j.Sessions.Rated)
	assert.Equal(t, []jarDevice{{Slug: "volcano", Name: "Volcano", Sessions: 1, Grams: 0.5}}, j.Sessions.Devices)
	assert.InDelta(t, 4, j.Sessions.Scores[effects.Overall], 1e-9)
	require.Len(t, j.Sessions.Recent, 1)
	assert.Equal(t, 5, j.Sessions.Recent[0].Scores[effects.Sleep])

	assert.Equal(t, http.StatusNotFound, call(t, s, http.MethodGet, "/api/jars/nothing", "").Code)
}

// TestTerpenesWritten: a profile written down is the jar's, on the list and
// in full; an empty one clears it.
func TestTerpenesWritten(t *testing.T) {
	s, _ := server(t, "")
	w := call(t, s, http.MethodPut, "/api/jars/wcake-221/terpenes", `{"terpenes":["Limonene"," Myrcene ",""]}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, []string{"Limonene", "Myrcene"}, decodeInto[jarDetail](t, call(t, s, http.MethodGet, "/api/jars/wcake-221", "")).Terpenes)
	assert.Equal(t, []string{"Limonene", "Myrcene"}, decodeInto[[]jar](t, call(t, s, http.MethodGet, "/api/jars", ""))[0].Terpenes)

	require.Equal(t, http.StatusCreated, call(t, s, http.MethodPut, "/api/jars/wcake-221/terpenes", `{"terpenes":[]}`).Code)
	assert.Empty(t, decodeInto[jarDetail](t, call(t, s, http.MethodGet, "/api/jars/wcake-221", "")).Terpenes)
	assert.Equal(t, http.StatusNotFound, call(t, s, http.MethodPut, "/api/jars/nothing/terpenes", `{"terpenes":[]}`).Code)
}

// TestResearch: the lookup asks the web for the product by name, and
// answers what it found without writing anything down.
func TestResearch(t *testing.T) {
	s, _ := server(t, "")
	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") == "Enua 22/1 Wedding Cake terpene" {
			_, _ = w.Write([]byte(`<a class="result__a" href="https://shop.example/wc">Wedding Cake</a> Terpene: Limonen, Myrcen`))
			return
		}
		http.NotFound(w, r)
	}))
	defer web.Close()
	s.sites = research.Sites{
		Engines:   []research.Engine{{URL: web.URL, Result: regexp.MustCompile(`class="result__a"`), Title: regexp.MustCompile(`^[^>]*>(.*?)</a>`)}},
		LeaflyAPI: web.URL + "/leafly", LeaflyPages: web.URL + "/strains/",
	}
	w := call(t, s, http.MethodGet, "/api/jars/wcake-221/research", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	f := decodeInto[research.Findings](t, w)
	assert.Equal(t, []string{"Limonene", "Myrcene"}, f.Suggested)
	assert.Empty(t, decodeInto[jarDetail](t, call(t, s, http.MethodGet, "/api/jars/wcake-221", "")).Terpenes)

	web.Close()
	assert.Equal(t, http.StatusBadGateway, call(t, s, http.MethodGet, "/api/jars/wcake-221/research", "").Code)
}

// TestOverviewEveryOpenFill: a fill whose jar outlives the next one's
// arrival stands on the dashboard beside it, newest first, each with its
// own grams.
func TestOverviewEveryOpenFill(t *testing.T) {
	s, r := server(t, "")
	ws, err := workspace.Read(r)
	require.NoError(t, err)
	_, _, _, err = ws.Recorder.Buy("Enua 30/1 Lemonade Haze", "", 10, time.Now().Add(-time.Hour))
	require.NoError(t, err)

	o := decodeInto[overview](t, call(t, s, http.MethodGet, "/api/overview", ""))
	require.Len(t, o.Fills, 2)
	assert.Equal(t, 2, o.Fills[0].Number)
	assert.Equal(t, o.Fill.Number, o.Fills[0].Number)
	assert.Equal(t, 1, o.Fills[1].Number)
	assert.InDelta(t, 18, o.Fills[1].Remaining, 1e-9)
	assert.Empty(t, o.Fills[1].ShelfProjection.History, "the projection is the fill in progress's")
	assert.Nil(t, o.Fills[1].Refill)
}

// TestSitesFor: the phone's engine looks up through the reader; a computer
// asks the sites themselves.
func TestSitesFor(t *testing.T) {
	assert.Equal(t, research.Phone.Relay, sitesFor("js").Relay)
	assert.NotEmpty(t, sitesFor("js").Relay)
	assert.Empty(t, sitesFor("linux").Relay)
}
