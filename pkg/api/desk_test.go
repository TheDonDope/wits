package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/repo"
)

func TestOpensAnotherRepository(t *testing.T) {
	s, first := server(t, "")
	other := t.TempDir()
	_, err := repo.Init(other)
	require.NoError(t, err)

	w := call(t, s, "POST", "/api/repo", `{"path": "`+other+`"}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	p := decodeInto[place](t, w)
	assert.Equal(t, other, p.Path)
	assert.Equal(t, 0, p.Entries)
	o := decodeInto[overview](t, call(t, s, "GET", "/api/overview", ""))
	assert.Nil(t, o.Fill, "The overview is the new repository's, and it is empty")

	w = call(t, s, "POST", "/api/repo", `{"path": "`+first.WorkTree()+`"}`)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 3, decodeInto[place](t, w).Entries, "and back again")
}

func TestOpenRefusesWhatIsNotARepository(t *testing.T) {
	s, _ := server(t, "")
	bare := t.TempDir()

	assert.Equal(t, http.StatusConflict, call(t, s, "POST", "/api/repo", `{"path": "`+bare+`"}`).Code,
		"A folder without one")
	assert.Equal(t, http.StatusBadRequest, call(t, s, "POST", "/api/repo", `{"path": "relative/wits"}`).Code)
	assert.Equal(t, http.StatusBadRequest, call(t, s, "POST", "/api/repo", `{"path": "`+bare+`/missing"}`).Code)

	w := call(t, s, "POST", "/api/repo", `{"path": "`+bare+`", "create": true}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.DirExists(t, filepath.Join(bare, repo.Dir), "Asked to, it makes one")
	assert.Equal(t, http.StatusConflict, call(t, s, "POST", "/api/repo", `{"path": "`+bare+`", "create": true}`).Code,
		"but never over one")

	w = call(t, s, "POST", "/api/repo", `{"path": "`+bare+`/new", "create": true}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.DirExists(t, filepath.Join(bare, "new", repo.Dir), "A new folder is made for it")
	assert.Equal(t, http.StatusBadRequest,
		call(t, s, "POST", "/api/repo", `{"path": "`+bare+`/no/such", "create": true}`).Code,
		"but only in a folder that is there")
}

func TestDirs(t *testing.T) {
	s, _ := server(t, "")
	root := t.TempDir()
	for _, d := range []string{"b-plain", "A-wits", ".hidden"} {
		require.NoError(t, os.Mkdir(filepath.Join(root, d), 0o755))
	}
	_, err := repo.Init(filepath.Join(root, "A-wits"))
	require.NoError(t, err)

	w := call(t, s, "GET", "/api/dirs?path="+root, "")

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	l := decodeInto[listing](t, w)
	assert.Equal(t, root, l.Path)
	assert.Equal(t, filepath.Dir(root), l.Parent)
	assert.False(t, l.Repo)
	assert.Equal(t, []folderEntry{
		{Name: "A-wits", Path: filepath.Join(root, "A-wits"), Repo: true},
		{Name: "b-plain", Path: filepath.Join(root, "b-plain")},
	}, l.Dirs, "Sorted without regard to case, the hidden one left out")
	assert.Equal(t, http.StatusBadRequest, call(t, s, "GET", "/api/dirs?path=nowhere", "").Code)
}

// upload sends a file the way the browser sends a workbook.
func upload(t *testing.T, s *Server, path, contentType string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "http://127.0.0.1:7341"+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w
}

func TestImportsAnUploadedWorkbook(t *testing.T) {
	book, err := os.ReadFile("../../assets/Tracking.2022.cleaned.xlsx")
	require.NoError(t, err)
	s, _ := server(t, "")
	empty := t.TempDir()
	require.Equal(t, http.StatusOK, call(t, s, "POST", "/api/repo", `{"path": "`+empty+`", "create": true}`).Code)

	w := upload(t, s, "/api/import", xlsx, book)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	v := decodeInto[verdict](t, w)
	assert.Equal(t, 53, v.Products)
	assert.Equal(t, 1429, v.Entries)
	assert.False(t, v.Committed)
	assert.Equal(t, 0, decodeInto[place](t, call(t, s, "GET", "/api/repo", "")).Entries, "A dry run writes nothing")

	w = upload(t, s, "/api/import?commit=1", xlsx, book)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.True(t, decodeInto[verdict](t, w).Committed)
	assert.Equal(t, 1429, decodeInto[place](t, call(t, s, "GET", "/api/repo", "")).Entries)

	assert.Equal(t, http.StatusConflict, upload(t, s, "/api/import?commit=1", xlsx, book).Code,
		"Never twice: it would double every gram")
}

func TestImportRefusesWhatIsNotAWorkbook(t *testing.T) {
	s, _ := server(t, "")

	assert.Equal(t, http.StatusUnsupportedMediaType, upload(t, s, "/api/import", "text/plain", []byte("x")).Code,
		"A type a page elsewhere could send without asking")
	assert.Equal(t, http.StatusBadRequest, upload(t, s, "/api/import", xlsx, []byte("not a zip")).Code)
}
