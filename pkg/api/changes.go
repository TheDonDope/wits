// Package api serves a repository over HTTP; this file tells a page when the
// repository changed — an entry made with the command line, the terminal
// interface or another page — as a stream of server-sent events, so the page
// reads again only then rather than asking every few seconds; and answers the
// journal from where a page left it, so what it reads again is what is new.
package api

import (
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// How often the repository's files are looked at, and how often a stream
// says it is still there when nothing changed, so no proxy or browser
// closes it as idle.
var (
	changesEvery = time.Second
	changesAlive = 25 * time.Second
)

// changes streams an event each time the repository's files change: the
// first at once, then one per change, until the page goes away.
func (s *Server) changes(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		fail(w, http.StatusNotImplemented, fmt.Errorf("this server cannot stream"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	look := time.NewTicker(changesEvery)
	defer look.Stop()
	alive := time.NewTicker(changesAlive)
	defer alive.Stop()
	last := ""
	for {
		if stamp := workspace.Stamp(s.current()); stamp != last {
			last = stamp
			fmt.Fprintf(w, "data: %d\n\n", time.Now().UnixMilli())
			flusher.Flush()
		}
		select {
		case <-r.Context().Done():
			return
		case <-alive.C:
			fmt.Fprint(w, ": still here\n\n")
			flusher.Flush()
		case <-look.C:
		}
	}
}

// journalSince is the journal from where a page left it: the entries
// recorded after the one named, in the order they were recorded, and every
// product's name as it is now, since a rename changes what an old entry is
// called. A page that names none, or one the journal no longer holds, is
// given every entry and told to start again.
type journalSince struct {
	Entries []entry           `json:"entries"`
	Head    string            `json:"head"`
	Reset   bool              `json:"reset"`
	Total   int               `json:"total"`
	Names   map[string]string `json:"names"`
}

func journalAfter(ws *workspace.Workspace, after string) (journalSince, error) {
	chain, err := ws.Repo.Journal().Events()
	if err != nil {
		return journalSince{}, err
	}
	from := slices.IndexFunc(chain, func(e journal.Event) bool { return e.Hash == after })
	out := journalSince{Entries: []entry{}, Reset: after == "" || from < 0, Total: len(chain), Names: map[string]string{}}
	if out.Reset {
		from = -1
	}
	undone := ledger.Undone(chain)
	for _, e := range chain[from+1:] {
		out.Entries = append(out.Entries, entryOf(e, ws, undone))
	}
	if n := len(chain); n > 0 {
		out.Head = chain[n-1].Hash
	}
	for _, p := range ws.Products.Products {
		out.Names[p.Slug] = p.Name
	}
	return out, nil
}
