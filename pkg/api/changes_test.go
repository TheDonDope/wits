package api

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/workspace"
)

func TestChangesStreamsAnEventWhenTheRepositoryChanges(t *testing.T) {
	changesEvery = 10 * time.Millisecond
	s, r := server(t, "")
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	req, err := http.NewRequest("GET", srv.URL+"/api/changes", nil)
	require.NoError(t, err)
	req.Host = "127.0.0.1:7341"
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, "text/event-stream", res.Header.Get("Content-Type"))
	lines := bufio.NewScanner(res.Body)
	next := func() string {
		for lines.Scan() {
			if strings.HasPrefix(lines.Text(), "data: ") {
				return lines.Text()
			}
		}
		return ""
	}
	require.NotEmpty(t, next(), "Should say so at once")

	ws, err := workspace.Read(r)
	require.NoError(t, err)
	_, err = ws.Recorder.Grind("wcake", 0.5, time.Now())
	require.NoError(t, err)

	done := make(chan string, 1)
	go func() { done <- next() }()
	select {
	case got := <-done:
		assert.NotEmpty(t, got, "Should say when an entry is recorded")
	case <-time.After(3 * time.Second):
		t.Fatal("no event after an entry was recorded")
	}
}

func TestTheJournalFromWhereAPageLeftIt(t *testing.T) {
	s, r := server(t, "")
	all := decodeInto[journalSince](t, call(t, s, "GET", "/api/journal?after=", ""))
	assert.True(t, all.Reset, "Should start a page with nothing from the beginning")
	require.Len(t, all.Entries, 3)
	assert.Equal(t, "Enua 22/1 Wedding Cake", all.Names["wcake-221"])

	ws, err := workspace.Read(r)
	require.NoError(t, err)
	e, err := ws.Recorder.Grind("wcake", 0.5, time.Now().Add(-72*time.Hour))
	require.NoError(t, err)

	since := decodeInto[journalSince](t, call(t, s, "GET", "/api/journal?after="+all.Head, ""))
	assert.False(t, since.Reset)
	require.Len(t, since.Entries, 1, "Should give only what was recorded since, backdated or not")
	assert.Equal(t, e.Hash, since.Entries[0].Hash)
	assert.Equal(t, e.Hash, since.Head)

	gone := decodeInto[journalSince](t, call(t, s, "GET", "/api/journal?after=nothing-like-it", ""))
	assert.True(t, gone.Reset, "Should start again from an entry the journal does not hold")
	assert.Len(t, gone.Entries, 4)
}
