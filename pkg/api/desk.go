//go:build !js

// Package api serves a repository over HTTP; this is its desk, what only a
// computer's server does — open another repository, browse the folders to find
// one, and import a workbook sent from the browser. The phone has one repository and no workbooks to read, and the
// spreadsheet reader would weigh on its engine, so none of this is built for
// it (desk_js.go).
package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/importer"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/repo"
)

// xlsx is the type a workbook upload must be sent as. A page on another
// origin cannot send it without a preflight this server never grants, as with
// JSON for every other write.
const xlsx = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

// maxWorkbook is the most an upload may be: the workbook is 0.35 MB.
const maxWorkbook = 32 << 20

func (s *Server) desk(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/repo", s.where)
	mux.HandleFunc("POST /api/repo", s.open)
	mux.HandleFunc("GET /api/dirs", s.dirs)
	mux.HandleFunc("POST /api/import", s.importBook)
}

// place is the repository being served, as a client reads it.
type place struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Entries int    `json:"entries"`
	Home    string `json:"home"`
}

func (s *Server) where(w http.ResponseWriter, _ *http.Request) {
	p, err := placeOf(s.current())
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	reply(w, http.StatusOK, p)
}

func placeOf(r *repo.Repo) (place, error) {
	events, err := r.Journal().Events()
	if err != nil {
		return place{}, err
	}
	home, _ := os.UserHomeDir()
	return place{Path: r.WorkTree(), Name: filepath.Base(r.WorkTree()), Entries: len(events), Home: home}, nil
}

// open serves another repository: the one in the folder named, or a new one
// made there when asked. It must be that folder's own — a folder inside a
// repository is not one, though wits commands run there would find it.
func (s *Server) open(w http.ResponseWriter, r *http.Request) {
	if !jsonBody(w, r) {
		return
	}
	var req struct {
		Path   string `json:"path"`
		Create bool   `json:"create"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	dir, err := folder(req.Path)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	next, err := openOrInit(dir, req.Create)
	if err != nil {
		fail(w, http.StatusConflict, err)
		return
	}
	s.serve(next)
	s.where(w, r)
}

// openOrInit opens the repository in dir, or makes one there.
func openOrInit(dir string, create bool) (*repo.Repo, error) {
	if create {
		return repo.Init(dir)
	}
	if !isRepo(dir) {
		return nil, fmt.Errorf("no repository in %s: create one there, or pick a folder marked wits", dir)
	}
	return repo.Discover(dir)
}

// folder checks that a path names a directory, and gives it absolute and
// without "..". It confines nothing, and is not meant to: any folder of this
// computer is the browser's to look in, as it is the user's in a terminal —
// the server is theirs, and answers no other machine.
func folder(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("not an absolute path: %q", path)
	}
	dir, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a folder: %s", dir)
	}
	return dir, nil
}

// isRepo reports whether dir holds a repository of its own.
func isRepo(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, repo.Dir))
	return err == nil && info.IsDir()
}

// listing is a folder as the browser shows it: where it is, the one above,
// and the folders in it, each saying whether it is a repository.
type listing struct {
	Path   string        `json:"path"`
	Parent string        `json:"parent,omitempty"`
	Repo   bool          `json:"repo"`
	Dirs   []folderEntry `json:"dirs"`
}

type folderEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Repo bool   `json:"repo"`
}

// dirs lists the folders in a folder: the one asked for, or the home folder.
// Hidden ones are left out, as a file picker leaves them.
func (s *Server) dirs(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path, _ = os.UserHomeDir()
	}
	dir, err := folder(path)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	found, err := os.ReadDir(dir)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	out := listing{Path: dir, Repo: isRepo(dir), Dirs: []folderEntry{}}
	if parent := filepath.Dir(dir); parent != dir {
		out.Parent = parent
	}
	for _, f := range found {
		if !f.IsDir() || strings.HasPrefix(f.Name(), ".") {
			continue
		}
		p := filepath.Join(dir, f.Name())
		out.Dirs = append(out.Dirs, folderEntry{Name: f.Name(), Path: p, Repo: isRepo(p)})
	}
	sort.Slice(out.Dirs, func(i, j int) bool {
		return strings.ToLower(out.Dirs[i].Name) < strings.ToLower(out.Dirs[j].Name)
	})
	reply(w, http.StatusOK, out)
}

// verdict is what a workbook holds, as wits import reports it, and whether
// it was written.
type verdict struct {
	Sheets    int      `json:"sheets"`
	Products  int      `json:"products"`
	Entries   int      `json:"entries"`
	Dispensed float64  `json:"dispensed"`
	Ground    float64  `json:"ground"`
	Fills     int      `json:"fills"`
	Grinds    int      `json:"grinds"`
	First     string   `json:"first,omitempty"`
	Last      string   `json:"last,omitempty"`
	Merged    []merge  `json:"merged"`
	Anomalies []string `json:"anomalies"`
	Committed bool     `json:"committed"`
}

type merge struct {
	Slug  string   `json:"slug"`
	Names []string `json:"names"`
}

// importBook reads a workbook sent as the request's body. It writes nothing
// unless ?commit=1 — a dry run is the default, as with wits import — and then
// only into an empty journal, as importer.Commit insists.
func (s *Server) importBook(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != xlsx {
		fail(w, http.StatusUnsupportedMediaType, errors.New("send the workbook as "+xlsx))
		return
	}
	result, err := importer.ReadFrom(io.LimitReader(http.MaxBytesReader(w, r.Body, maxWorkbook), maxWorkbook))
	if err != nil {
		fail(w, http.StatusBadRequest, fmt.Errorf("not a workbook wits can read: %w", err))
		return
	}
	out := verdictOf(result)
	if r.URL.Query().Get("commit") == "1" {
		if err := importer.Commit(s.current(), result, nil); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, importer.ErrNotEmpty) {
				status = http.StatusConflict
			}
			fail(w, status, err)
			return
		}
		out.Committed = true
	}
	reply(w, http.StatusOK, out)
}

func verdictOf(result *importer.Result) verdict {
	dispensed, ground := result.Grams()
	counts := result.Counts()
	out := verdict{
		Sheets: len(result.Sheets), Products: len(result.Products), Entries: len(result.Events),
		Dispensed: dispensed, Ground: ground,
		Fills: counts[journal.Purchase], Grinds: counts[journal.Grind],
		Merged: []merge{}, Anomalies: result.Anomalies(),
	}
	if first, last := result.Span(); !first.IsZero() {
		out.First, out.Last = first.Format(time.DateOnly), last.Format(time.DateOnly)
	}
	for _, m := range result.Merged {
		out.Merged = append(out.Merged, merge{Slug: m.Slug, Names: m.Names})
	}
	if out.Anomalies == nil {
		out.Anomalies = []string{}
	}
	return out
}
