// Package api serves a repository over HTTP; this file opens the demo: the
// bundle the interface ships (assets/demo-seed.wits), restored into a
// repository of its own in the home folder and served — or, when that
// repository already holds the demo, opened again as it was left.
package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/TheDonDope/wits/pkg/bundle"
	"github.com/TheDonDope/wits/pkg/repo"
)

// bundleType is the type the demo's bundle must be sent as. Like a workbook
// upload, a page on another origin cannot send it without a preflight this
// server never grants.
const bundleType = "application/x-wits-bundle"

// demoFolder is where the demo is restored: beside the user's own
// repositories in the home folder, never into one of them.
const demoFolder = "wits-demo"

// maxBundle is the most a bundle sent may be: the demo's is 0.17 MB.
const maxBundle = 16 << 20

// errNotDemo is a folder by the demo's name that is something else.
var errNotDemo = errors.New("a folder named " + demoFolder + " is in the home folder and is not a wits repository; move it, and the demo can be made there")

// demo restores the bundle sent into the demo's repository and serves it,
// or serves the demo already restored there.
func (s *Server) demo(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != bundleType {
		fail(w, http.StatusUnsupportedMediaType, errors.New("send the demo as "+bundleType))
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	next, err := demoRepo(filepath.Join(home, demoFolder), http.MaxBytesReader(w, r.Body, maxBundle))
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, errNotDemo) {
			status = http.StatusConflict
		}
		fail(w, status, err)
		return
	}
	s.serve(next)
	s.where(w, r)
}

// demoRepo is the demo's repository in dir: the one there when it holds
// entries, as it was left, or a new one with the bundle restored into it.
func demoRepo(dir string, sent io.Reader) (*repo.Repo, error) {
	r, err := demoFolderRepo(dir)
	if err != nil {
		return nil, err
	}
	events, err := r.Journal().Events()
	if err != nil {
		return nil, err
	}
	if len(events) > 0 {
		return r, nil
	}
	contents, err := bundle.Read(sent)
	if err != nil {
		return nil, fmt.Errorf("the demo's bundle: %w", err)
	}
	if err := bundle.Restore(r, contents, func(string) {}); err != nil {
		return nil, fmt.Errorf("restoring the demo: %w", err)
	}
	return r, nil
}

// demoFolderRepo is the repository in the demo's folder, made when the folder
// is not there or empty; a folder holding anything else is left alone.
func demoFolderRepo(dir string) (*repo.Repo, error) {
	if isRepo(dir) {
		return repo.Discover(dir)
	}
	found, err := os.ReadDir(dir)
	if err == nil && len(found) > 0 {
		return nil, errNotDemo
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return repo.Init(dir)
}
