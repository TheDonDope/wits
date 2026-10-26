// Package api serves a repository over HTTP; this file answers a write asked
// with ?dry=1: the recorder rehearses it (record.Recorder.Dry) — built and
// checked as it would be, refused in the same words — and the answer is what
// it would record and what that would do to each jar, with nothing written.
// A form shows it before it is recorded, so the browser works out none of the
// ledger's arithmetic itself.
package api

import (
	"context"
	"net/http"
	"slices"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// preview is what a write would record, and what it would do.
type preview struct {
	Entries []entry     `json:"entries"`
	Message string      `json:"message"`
	Jars    []jarChange `json:"jars"`
	// Lines is a fill's strains, each with its price as it would be written.
	Lines []previewLine `json:"lines,omitempty"`
}

// jarChange is one jar's accounts before the write and after it — the jar's
// own grams, every fill of it.
type jarChange struct {
	Product string   `json:"product"`
	Name    string   `json:"name"`
	Before  accounts `json:"before"`
	After   accounts `json:"after"`
}

// accounts is what a jar holds in each account.
type accounts struct {
	Storage  float64 `json:"storage"`
	Stash    float64 `json:"stash"`
	Consumed float64 `json:"consumed"`
	AVB      float64 `json:"avb"`
}

// previewLine is a fill's strain as it would be recorded: the product it is,
// whether it is new, and its price in cents, with the VAT in it apart.
type previewLine struct {
	Product string `json:"product"`
	Added   bool   `json:"added"`
	Cents   int64  `json:"cents"`
	VAT     int64  `json:"vat_cents"`
}

// dryKey carries, in a rehearsed request's context, where a fill puts its lines.
type dryKey struct{}

// dry reports whether a write is to be rehearsed.
func dry(r *http.Request) bool { return r.URL.Query().Get("dry") == "1" }

// rehearsing readies a workspace and a request for a rehearsal: the
// recorder only rehearses, and a fill has somewhere to put its lines.
func rehearsing(ws *workspace.Workspace, r *http.Request) (*http.Request, *[]previewLine) {
	ws.Recorder = ws.Recorder.Dry()
	lines := &[]previewLine{}
	return r.WithContext(context.WithValue(r.Context(), dryKey{}, lines)), lines
}

// rehearsedLines is where a rehearsed fill puts its lines, or nil when the
// fill is recorded.
func rehearsedLines(r *http.Request) *[]previewLine {
	lines, _ := r.Context().Value(dryKey{}).(*[]previewLine)
	return lines
}

// previewOf is what a rehearsed batch would do: each jar it touches, before
// and after, folded from the journal as it stands with the batch on top.
func previewOf(ws *workspace.Workspace, batch []journal.Event, message string, lines []previewLine) preview {
	before := ws.State
	after := ledger.Fold(append(slices.Clip(before.Events), batch...))
	out := preview{Entries: make([]entry, 0, len(batch)), Message: message, Jars: []jarChange{}, Lines: lines}
	seen := map[string]bool{}
	for _, e := range batch {
		out.Entries = append(out.Entries, entryOf(e, ws, nil))
		if e.Product == "" || seen[e.Product] {
			continue
		}
		seen[e.Product] = true
		out.Jars = append(out.Jars, jarChange{Product: e.Product, Name: ws.ProductName(e.Product),
			Before: accountsOf(before.Balances[e.Product]), After: accountsOf(after.Balances[e.Product])})
	}
	return out
}

func accountsOf(b *ledger.Balance) accounts {
	if b == nil {
		return accounts{}
	}
	return accounts{Storage: b.Storage, Stash: b.Stash, Consumed: b.Consumed, AVB: b.AVB}
}
