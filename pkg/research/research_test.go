package research

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The pages below are cut down from what the sites answered on 2026-10-25.
const (
	// captcha is the page a search answers a client it takes for a bot with.
	captcha = `<html><body>Unfortunately, bots use DuckDuckGo too.</body></html>`

	results = `<html><body>
<div class="result"><a rel="nofollow" class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fshop.dransay.com%2Fproduct%2Flemon-tartz&amp;rut=1">Lemon Tartz - All Nations</a>
<a class="result__snippet" href="x">The main terpenes are <b>Limonen</b>, Linalool, β-Caryophyllen, Nerolidol.</a></div>
<div class="result"><a rel="nofollow" class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fstrainy.ca%2Fstrain%2Flemon-tartz&amp;rut=2">Lemon Tartz | Strainy</a>
<a class="result__snippet" href="x">The strain contains terpenes including nerolidol, limonene, linalool, and caryophyllene. 21% THC, 1% CBD.</a></div>
<div class="result"><a rel="nofollow" class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2F&amp;rut=3">Nothing here</a>
<a class="result__snippet" href="x">A hybrid with a creative effect.</a></div>
</body></html>`

	leaflySearch = `{"results":[{"entity":"strain","hits":[{"slug":"mac-11","name":"Mac 11"},{"slug":"mac-1","name":"Mac 1"}]}]}`

	leaflyPage = `<html><script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"strain":{"name":"Mac 1",
"terps":{"limonene":{"name":"Limonene","score":0.645},"pinene":{"name":"Pinene","score":0.378},
"myrcene":{"name":"Myrcene","score":0.142},"ocimene":{"name":"Ocimene","score":0.032}}}}}}</script></html>`
)

var ddg = Engine{Name: "test", Result: regexp.MustCompile(`class="result__a"`), Title: regexp.MustCompile(`(?s)^[^>]*>(.*?)</a>`)}

// web stands in for the sites: the first search turns the client away, the
// second answers, and Leafly knows MAC 1.
func web(t *testing.T) Sites {
	t.Helper()
	sites, _ := webAnd(t)
	return sites
}

// webAnd is web, with the handler that answers for the sites, for a stand-in
// relay to answer from without a request of its own.
func webAnd(t *testing.T) (Sites, http.Handler) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/captcha", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(captcha)) })
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "All Nations 21/1 Lemon Tartz terpene", r.URL.Query().Get("q"))
		_, _ = w.Write([]byte(results))
	})
	mux.HandleFunc("/leafly/search", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "MAC 1" {
			_, _ = w.Write([]byte(`{"results":[{"entity":"strain","hits":[]}]}`))
			return
		}
		_, _ = w.Write([]byte(leaflySearch))
	})
	mux.HandleFunc("/leafly/strains/mac-1", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(leaflyPage)) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	first, second := ddg, ddg
	first.URL, second.URL = srv.URL+"/captcha", srv.URL+"/search"
	return Sites{Engines: []Engine{first, second}, LeaflyAPI: srv.URL + "/leafly/search", LeaflyPages: srv.URL + "/leafly/strains/"}, mux
}

// TestLookSearchesOnPastACaptcha: a search that turns the client away is
// passed over for the next, and each result's terpenes are read in the order
// it names them, German or English, results naming none left out.
func TestLookSearchesOnPastACaptcha(t *testing.T) {
	f, err := Look(context.Background(), http.DefaultClient, web(t), Query{Name: "All Nations 21/1 Lemon Tartz", Strain: "Lemon Tartz"})
	require.NoError(t, err)
	require.Len(t, f.Sources, 2)
	assert.Equal(t, "shop.dransay.com", f.Sources[0].Site)
	assert.Equal(t, "https://shop.dransay.com/product/lemon-tartz", f.Sources[0].URL)
	assert.Equal(t, "Lemon Tartz - All Nations", f.Sources[0].Title)
	assert.Equal(t, []string{"Limonene", "Linalool", "Caryophyllene", "Nerolidol"}, f.Sources[0].Terpenes)
	assert.Equal(t, []string{"Nerolidol", "Limonene", "Linalool", "Caryophyllene"}, f.Sources[1].Terpenes)
	// Both name all four; Limonene is named earliest on average (0 and 1).
	assert.Equal(t, []string{"Limonene", "Linalool", "Nerolidol", "Caryophyllene"}, f.Suggested)
	assert.True(t, strings.HasSuffix(f.Search, "/search?q=All+Nations+21%2F1+Lemon+Tartz+terpene"))
}

// TestLookLeaflyStrain: Leafly is asked for the strain spaced as it spells
// it, only the hit of that very name counts ("Mac 11" is not "MAC1"), and
// traces below its threshold are left out.
func TestLookLeaflyStrain(t *testing.T) {
	sites := web(t)
	sites.Engines = sites.Engines[:1]
	f, err := Look(context.Background(), http.DefaultClient, sites, Query{Name: "Cantourage 25/1 MAC 1+", Strain: "MAC1"})
	require.NoError(t, err)
	require.Len(t, f.Sources, 1)
	assert.True(t, f.Sources[0].Strain)
	assert.Equal(t, "leafly.com", f.Sources[0].Site)
	assert.Equal(t, []string{"Limonene", "Pinene", "Myrcene"}, f.Sources[0].Terpenes)
	assert.Equal(t, []string{"Limonene", "Pinene", "Myrcene"}, f.Suggested)
}

// TestLookUnreachable: only when no site answers at all is it an error.
func TestLookUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	e := ddg
	e.URL = srv.URL
	_, err := Look(context.Background(), http.DefaultClient,
		Sites{Engines: []Engine{e}, LeaflyAPI: srv.URL, LeaflyPages: srv.URL}, Query{Name: "x", Strain: "y"})
	require.ErrorIs(t, err, ErrUnreachable)
}

// TestNamed reads terpene names in any of the spellings labels use, each
// once, and nothing that is not a terpene.
func TestNamed(t *testing.T) {
	assert.Equal(t, []string{"Myrcene", "Caryophyllene", "Pinene", "Carene", "Ocimene"},
		Named("Terpene: β-Myrcen, Trans-Caryophyllen, Beta-Caryophyllene, Alpha-Pinen, Δ-3-Carene; ocimene. THC 22 %, CBD < 1 %"))
	assert.Empty(t, Named("A hybrid with apigenin and quercetin, CBD and THC."))
}

// TestRank: the terpene most sources name leads; equal votes go to the one
// named earlier; five at most.
func TestRank(t *testing.T) {
	got := Rank([]Source{
		{Terpenes: []string{"Myrcene", "Limonene", "Pinene", "Humulene", "Linalool", "Terpinolene"}},
		{Terpenes: []string{"Limonen", "Myrcen"}},
		{Terpenes: []string{"Limonene"}},
	})
	assert.Equal(t, []string{"Limonene", "Myrcene", "Pinene", "Humulene", "Linalool"}, got)
	assert.Empty(t, Rank(nil))
}

// TestLookThroughRelay: through a relay every page is asked for by the
// relay's address before its own — the search as HTML, Leafly's API as its
// text — with no User-Agent of its own, which a browser would have to clear
// with the relay first.
func TestLookThroughRelay(t *testing.T) {
	sites, answer := webAnd(t)
	formats := map[string]string{}
	// The stand-in reader answers from the stand-in sites in this process:
	// the page asked for is the path after its host.
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.NotContains(t, r.Header.Get("User-Agent"), "wits")
		page := strings.SplitN(strings.TrimPrefix(r.RequestURI, "/http://"), "/", 2)[1]
		formats[page] = r.Header.Get("X-Return-Format")
		answer.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/"+page, nil))
	}))
	defer relay.Close()
	sites.Relay = relay.URL + "/"
	sites.Engines = sites.Engines[1:]
	f, err := Look(context.Background(), http.DefaultClient, sites, Query{Name: "All Nations 21/1 Lemon Tartz", Strain: "MAC1"})
	require.NoError(t, err)
	assert.Equal(t, []string{"Limonene", "Linalool", "Nerolidol", "Caryophyllene", "Pinene"}, f.Suggested)
	assert.Equal(t, "html", formats["search?q=All+Nations+21%2F1+Lemon+Tartz+terpene"])
	assert.Equal(t, "text", formats["leafly/search?q=MAC+1"])
	assert.Equal(t, "html", formats["leafly/strains/mac-1"])
}

// TestPhoneSites: the phone asks through the reader, and DuckDuckGo only —
// Brave's results do not come through it.
func TestPhoneSites(t *testing.T) {
	assert.Equal(t, "https://r.jina.ai/", Phone.Relay)
	require.Len(t, Phone.Engines, 1)
	assert.Equal(t, "DuckDuckGo", Phone.Engines[0].Name)
	assert.Empty(t, Web.Relay)
}
