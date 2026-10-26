package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/bundle"
	"github.com/TheDonDope/wits/pkg/ledger"
)

// demoBundle is the demo the interface ships, as committed.
func demoBundle(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../assets/demo-seed.wits")
	require.NoError(t, err)
	return string(b)
}

// sendDemo posts the demo's bundle as the interface does.
func sendDemo(t *testing.T, s *Server, body, kind string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7341/api/demo", strings.NewReader(body))
	req.Header.Set("Content-Type", kind)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w
}

// TestDemoBundle: the committed demo reads, its journal holds no entry that
// takes grams that were not there, and it carries what the demo is for —
// prices, devices, ratings, terpenes.
func TestDemoBundle(t *testing.T) {
	c, err := bundle.Read(strings.NewReader(demoBundle(t)))
	require.NoError(t, err)
	assert.Empty(t, ledger.Shortfalls(c.Events))
	assert.NotEmpty(t, c.Prices)
	assert.NotEmpty(t, c.Ratings)
	assert.Len(t, c.Devices.Devices, 15)
	with := 0
	for _, p := range c.Products.Products {
		if len(p.Terpenes) > 0 {
			with++
		}
	}
	assert.Greater(t, with, 40)
	for _, p := range c.Prices {
		assert.Empty(t, p.Note, "an invoice's number stays out of the public demo")
	}
}

// TestDemo: the demo is restored into wits-demo in the home folder and
// served; asked for again, the repository there is opened as it was left,
// and a folder by that name that is something else is left alone.
func TestDemo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	s, _ := server(t, "")
	body := demoBundle(t)

	assert.Equal(t, http.StatusUnsupportedMediaType, sendDemo(t, s, body, "text/plain").Code)

	w := sendDemo(t, s, body, bundleType)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	p := decodeInto[place](t, w)
	assert.Equal(t, filepath.Join(home, demoFolder), p.Path)
	assert.Greater(t, p.Entries, 2000)
	o := decodeInto[overview](t, call(t, s, http.MethodGet, "/api/overview", ""))
	assert.NotNil(t, o.Fill)

	// Opened again, not restored over: a grind made in the demo stays.
	require.Equal(t, http.StatusCreated, call(t, s, http.MethodPost, "/api/grind", `{"product":"nice-morning-sun-321","grams":0.5}`).Code)
	again := decodeInto[place](t, sendDemo(t, s, body, bundleType))
	assert.Equal(t, p.Entries+1, again.Entries)

	other := t.TempDir()
	t.Setenv("HOME", other)
	require.NoError(t, os.MkdirAll(filepath.Join(other, demoFolder), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(other, demoFolder, "notes.txt"), []byte("mine"), 0o600))
	assert.Equal(t, http.StatusConflict, sendDemo(t, s, body, bundleType).Code)
}
