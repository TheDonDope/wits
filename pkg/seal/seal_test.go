package seal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const head = "f81bbd2cc8932aeb3a64c6f6b0d1a9e0c5e2f7d8a9b0c1d2e3f4a5b6c7d8e9f0"

func TestSealAndVerify(t *testing.T) {
	key, made, err := Key(filepath.Join(t.TempDir(), "seal.key"))
	require.NoError(t, err)
	assert.True(t, made, "Should make a key the first time")
	at := time.Date(2026, 10, 24, 20, 0, 0, 0, time.UTC)

	s := Make(key, head, 1429, at)

	require.NoError(t, s.Verify())
	parsed, err := Parse(s.String())
	require.NoError(t, err)
	assert.Equal(t, s, parsed, "Should read back as it was written")
	assert.Equal(t, "2026-10-24-f81bbd2cc893", s.Name())
	assert.True(t, strings.HasPrefix(Fingerprint(s.Key), "SHA256:"))

	changed, err := Parse(strings.Replace(s.String(), "entries 1429", "entries 1430", 1))
	require.NoError(t, err)
	assert.ErrorIs(t, changed.Verify(), ErrForged, "Should refuse a seal changed after signing")
	_, err = Parse("hello")
	assert.Error(t, err)
}

func TestKeyIsKept(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seal.key")
	first, _, err := Key(path)
	require.NoError(t, err)
	again, made, err := Key(path)
	require.NoError(t, err)
	assert.False(t, made)
	assert.Equal(t, first, again, "Should seal with the same key every time")
}

func TestStore(t *testing.T) {
	st := Store{Dir: filepath.Join(t.TempDir(), "seals")}
	require.NoError(t, st.Save(File{Name: "b", Text: "two"}))
	require.NoError(t, st.Save(File{Name: "a", Text: "one", OTS: []byte{1, 2}}))
	all, err := st.All()
	require.NoError(t, err)
	assert.Equal(t, []File{{Name: "a", Text: "one", OTS: []byte{1, 2}}, {Name: "b", Text: "two"}}, all)
}

// calendar answers like an OpenTimestamps calendar: a pending attestation
// behind an append and a hash, and, once asked again, a Bitcoin block.
func calendar(t *testing.T) *httptest.Server {
	t.Helper()
	nonce := []byte("nonce")
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b bytes.Buffer
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/digest":
			body, _ := io.ReadAll(r.Body)
			require.Len(t, body, 32, "Should send the digest and nothing else")
			b.WriteByte(opAppend)
			writeVarbytes(&b, nonce)
			b.WriteByte(opSHA256)
			b.WriteByte(0x00)
			b.Write(pendingTag)
			var uri bytes.Buffer
			writeVarbytes(&uri, []byte(srv.URL))
			writeVarbytes(&b, uri.Bytes())
		case strings.HasPrefix(r.URL.Path, "/timestamp/"):
			b.WriteByte(opPrepend)
			writeVarbytes(&b, []byte("block"))
			b.WriteByte(opSHA256)
			b.WriteByte(0x00)
			b.Write(bitcoinTag)
			writeVarbytes(&b, []byte{0xb4, 0xd6, 0x37}) // 912180, as a varuint
		default:
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(b.Bytes())
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestStampAndUpgrade(t *testing.T) {
	srv := calendar(t)
	digest := sha256.Sum256([]byte("a seal"))

	ots, answered, err := Stamp(context.Background(), srv.Client(), digest, []string{srv.URL, "http://127.0.0.1:1"})

	require.NoError(t, err, "One calendar answering is enough")
	assert.Equal(t, 1, answered)
	assert.True(t, bytes.HasPrefix(ots, otsMagic), "Should write the standard file")
	p, err := Describe(ots)
	require.NoError(t, err)
	assert.Equal(t, hex.EncodeToString(digest[:]), hex.EncodeToString(p.Digest))
	assert.Equal(t, []string{srv.URL}, p.Pending)
	assert.Empty(t, p.Blocks)

	upgraded, changed, err := Upgrade(context.Background(), srv.Client(), ots)
	require.NoError(t, err)
	assert.True(t, changed)
	p, err = Describe(upgraded)
	require.NoError(t, err)
	assert.Empty(t, p.Pending, "Should put the block in place of the promise")
	assert.Equal(t, []int{912180}, p.Blocks)

	again, changed, err := Upgrade(context.Background(), srv.Client(), upgraded)
	require.NoError(t, err)
	assert.False(t, changed, "Nothing left to ask")
	assert.Equal(t, upgraded, again, "Should write back what it read")
}

func TestCarry(t *testing.T) {
	key, _, err := Key(filepath.Join(t.TempDir(), "seal.key"))
	require.NoError(t, err)
	items := []Item{
		{Name: "Sumo 25/1 Papaya Bomb", Sealed: 12.94, Ground: 7.06, Fills: []Dispensed{{Day: "2026-08-29", Grams: 20}}},
		{Name: "Enua 36/1 Citrus Slap", Sealed: 2.4, Ground: 17.6, AVB: 0.21, Fills: []Dispensed{{Day: "2026-07-09", Grams: 20}}},
	}
	c := MakeCarry(key, head, 1429, time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC), items)

	require.NoError(t, c.Verify())
	for name, text := range map[string]string{"the statement": c.String(), "its address": c.URL()} {
		got, err := ParseCarry(text)
		require.NoError(t, err, name)
		assert.Equal(t, c, got, "Should read back %s as it was signed", name)
	}
	assert.True(t, strings.HasPrefix(c.URL(), VerifyPage+"z"), "Should deflate the statement, and say so")
	plain := len(VerifyPage) + base64.RawURLEncoding.EncodedLen(len(c.String()))
	assert.Less(t, len(c.URL()), plain, "Deflated, the address is shorter than the statement in plain base64")

	forged, err := ParseCarry(strings.Replace(c.String(), "12.94", "13.94", 1))
	require.NoError(t, err)
	assert.ErrorIs(t, forged.Verify(), ErrForged, "Should refuse a gram changed after signing")
	_, err = ParseCarry("wits-seal 1\n")
	assert.Error(t, err)
}
