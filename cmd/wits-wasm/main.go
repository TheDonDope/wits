//go:build js

// Command wits-wasm is wits for a phone: the same repository, recorder and
// API the server runs, compiled to WebAssembly and run inside the page, so
// the phone refuses the same things in the same words as the terminal.
//
// The repository lives in the browser, at /wits on the file system the page
// hands Go (wits-ui's data/memfs.ts, kept in IndexedDB). Nothing is sent
// anywhere: the phone syncs by opening the remote's wits.age from the shared
// folder and saving it back, and the remote package does the rest exactly as
// wits pull and wits push do.
//
// It sets globalThis.witsEngine, with
//
//	request(method, url, body) → Promise<{status, type, body, bytes?}>  the API
//	sync(file, key)            → Promise<{pulled, pushed, file}>
//	key()                      → Promise<string>           the remote's key
//
// and calls globalThis.witsReady once they are there.
package main

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall/js"

	"github.com/TheDonDope/wits/pkg/api"
	"github.com/TheDonDope/wits/pkg/remote"
	"github.com/TheDonDope/wits/pkg/repo"
)

const (
	// root is where the repository lives on the page's file system.
	root = "/wits"
	// folder stands in for the shared folder: the file the phone opens is put
	// here before a pull, and what a push writes here is handed back to save.
	folder = "/remote"
)

func main() {
	r, err := open()
	if err != nil {
		js.Global().Get("console").Call("error", "wits: "+err.Error())
		return
	}
	handler := api.New(r, "").Handler()
	js.Global().Set("witsEngine", map[string]any{
		"request": promised(func(args []js.Value) (any, error) { return request(handler, args) }),
		"sync":    promised(func(args []js.Value) (any, error) { return sync(r, args) }),
		"key":     promised(func([]js.Value) (any, error) { return key(r) }),
	})
	if ready := js.Global().Get("witsReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}
	select {}
}

// open finds the repository, making it on the first run, and the temporary
// folder a key brought in passes through: the page's file system starts empty.
func open() (*repo.Repo, error) {
	if err := os.MkdirAll(os.TempDir(), 0700); err != nil {
		return nil, err
	}
	r, err := repo.Discover(root)
	if errors.Is(err, repo.ErrNotARepo) {
		if err := os.MkdirAll(root, 0700); err != nil {
			return nil, err
		}
		return repo.Init(root)
	}
	return r, err
}

// promised wraps work as a JavaScript function returning a Promise, the work
// run on its own goroutine: Go blocks on the file system, and the page's
// thread must not.
func promised(work func([]js.Value) (any, error)) js.Func {
	return js.FuncOf(func(_ js.Value, args []js.Value) any {
		return js.Global().Get("Promise").New(js.FuncOf(func(_ js.Value, settle []js.Value) any {
			go func() {
				out, err := work(args)
				if err != nil {
					settle[1].Invoke(js.Global().Get("Error").New(err.Error()))
					return
				}
				settle[0].Invoke(out)
			}()
			return nil
		}))
	})
}

// request answers one API call the way wits-server would: from this machine,
// a write as JSON.
func request(h http.Handler, args []js.Value) (any, error) {
	if len(args) < 3 {
		return nil, errors.New("request takes a method, a URL and a body")
	}
	method, url, body := args[0].String(), args[1].String(), args[2].String()
	req, err := http.NewRequest(method, "http://localhost"+url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := &recorder{header: http.Header{}, code: http.StatusOK}
	h.ServeHTTP(rec, req)
	out := map[string]any{"status": rec.code, "type": rec.header.Get("Content-Type")}
	// A string crossing into JavaScript is read as UTF-8, which a PDF is not:
	// anything but JSON goes across as bytes.
	if t := rec.header.Get("Content-Type"); t != "" && !strings.HasPrefix(t, "application/json") {
		out["bytes"] = jsBytes([]byte(rec.body.String()))
		out["body"] = ""
	} else {
		out["body"] = rec.body.String()
	}
	return out, nil
}

// recorder keeps a response in memory. It is httptest's ResponseRecorder, all
// that is needed of it: httptest brings the testing flags and doubles the
// download.
type recorder struct {
	header http.Header
	code   int
	body   strings.Builder
	wrote  bool
}

func (r *recorder) Header() http.Header { return r.header }

func (r *recorder) WriteHeader(code int) {
	if !r.wrote {
		r.code, r.wrote = code, true
	}
}

func (r *recorder) Write(b []byte) (int, error) {
	r.wrote = true
	return r.body.Write(b)
}

// sync is a pull from the opened file, when there is one, and a push: what
// the phone saves back to the shared folder. A key given is the remote's key,
// brought from the first device; without one, the key kept here is used, or a
// new one made when the phone is the first device.
func sync(r *repo.Repo, args []js.Value) (any, error) {
	if len(args) < 2 {
		return nil, errors.New("sync takes the opened file, or null, and a key, or an empty string")
	}
	if err := os.MkdirAll(folder, 0700); err != nil {
		return nil, err
	}
	opened := !args[0].IsNull() && !args[0].IsUndefined()
	if opened {
		if err := os.WriteFile(filepath.Join(folder, remote.File), bytesOf(args[0]), 0600); err != nil {
			return nil, err
		}
	}
	if err := attach(r, args[1].String()); err != nil {
		return nil, err
	}
	var pulled remote.Pulled
	if opened {
		var err error
		if pulled, err = remote.Pull(r); err != nil {
			return nil, err
		}
	}
	pushed, err := remote.Push(r)
	if err != nil {
		return nil, err
	}
	// Nothing recorded and nothing opened is nothing to save.
	file := js.Null()
	if data, err := os.ReadFile(filepath.Join(folder, remote.File)); err == nil {
		file = jsBytes(data)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return map[string]any{"pulled": report(pulled), "pushed": map[string]any{
		"events": pushed.Events, "ratings": pushed.Ratings, "prices": pushed.Prices}, "file": file}, nil
}

// attach points the repository at the stand-in folder, with the key given or
// the one it has.
func attach(r *repo.Repo, keyText string) error {
	keyFile := ""
	if keyText = strings.TrimSpace(keyText); keyText != "" {
		keyFile = filepath.Join(os.TempDir(), "remote.key")
		if err := os.WriteFile(keyFile, []byte(keyText+"\n"), 0600); err != nil {
			return err
		}
		defer os.Remove(keyFile)
	} else if r.Config.Remote == folder {
		return nil
	}
	return remote.Add(r, folder, keyFile)
}

// report is a pull as the page reads it.
func report(p remote.Pulled) map[string]any {
	clashes := make([]any, 0, len(p.Clashes))
	for _, c := range p.Clashes {
		clashes = append(clashes, map[string]any{"hash": c.Event.Hash, "text": c.String()})
	}
	return map[string]any{
		"fetched": p.Fetched, "rechained": p.Rechained, "ratings": p.Ratings, "prices": p.Prices,
		"products": strings.Join(p.Products, ","), "devices": strings.Join(p.Devices, ","),
		"clashes": clashes,
	}
}

// key is the remote's key, for bringing to another device.
func key(r *repo.Repo) (any, error) {
	id, err := remote.Key(r)
	if err != nil {
		return nil, err
	}
	return id.String(), nil
}

// bytesOf copies a Uint8Array into Go.
func bytesOf(v js.Value) []byte {
	b := make([]byte, v.Get("length").Int())
	js.CopyBytesToGo(b, v)
	return b
}

// jsBytes copies bytes out to a new Uint8Array.
func jsBytes(b []byte) js.Value {
	out := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(out, b)
	return out
}
