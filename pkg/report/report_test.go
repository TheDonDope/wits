package report

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/repo"
	"github.com/TheDonDope/wits/pkg/seal"
	"github.com/TheDonDope/wits/pkg/workspace"
)

// fixture is a repository with a fill, three grinds a week apart, one before
// the range, and a rated session.
func fixture(t *testing.T) (*workspace.Workspace, time.Time) {
	t.Helper()
	r, err := repo.Init(t.TempDir())
	require.NoError(t, err)
	ws, err := workspace.Read(r)
	require.NoError(t, err)
	start := time.Now().AddDate(0, 0, -30)
	_, _, _, err = ws.Recorder.Buy("Enua 22/1 Wedding Cake", "", 20, start)
	require.NoError(t, err)
	for _, d := range []int{1, 8, 15} {
		_, err = ws.Recorder.Grind("wcake", 2, start.AddDate(0, 0, d))
		require.NoError(t, err)
	}
	s, err := ws.Recorder.Session("wcake", 0.5, start.AddDate(0, 0, 16), "", 0, "")
	require.NoError(t, err)
	_, err = ws.Recorder.Rate(s.Hash, map[effects.Dimension]int{effects.Overall: 4, effects.Sleep: 5}, "", time.Now())
	require.NoError(t, err)
	ws, err = workspace.Read(r)
	require.NoError(t, err)
	return ws, start
}

func TestBuild(t *testing.T) {
	ws, start := fixture(t)

	r := Build(ws, start.AddDate(0, 0, 5), start.AddDate(0, 0, 20), "test")

	assert.Equal(t, 16, r.Days, "Both ends counted")
	assert.Equal(t, 4.0, r.Ground, "Only the two grinds in the range")
	assert.Equal(t, 2, r.GrindDays)
	assert.Equal(t, 0.25, r.PerDay)
	assert.Zero(t, r.Purchases, "The fill came before the range")
	assert.Equal(t, 1, r.Sessions)
	assert.Equal(t, 1, r.Rated)
	require.Len(t, r.Effects, 2)
	assert.Equal(t, Effect{Dimension: effects.Overall, Mean: 4, Count: 1}, r.Effects[0])
	require.Len(t, r.Products, 1)
	assert.Equal(t, "Enua 22/1 Wedding Cake", r.Products[0].Name)
	assert.Equal(t, 22.0, r.Products[0].THC)
	require.NotNil(t, r.Now, "A fill is in progress")
	assert.Equal(t, 14.0, r.Now.Left, "What is left sealed of the fill today, as wits status says it, not of the range")
	assert.True(t, r.Now.HasRefill)
	assert.NotEmpty(t, r.Head)
	var total float64
	for _, w := range r.Weeks {
		total += w.Ground
	}
	assert.Equal(t, r.Ground, total, "The weeks add up to the range")
}

func TestPDF(t *testing.T) {
	ws, start := fixture(t)
	var buf bytes.Buffer

	require.NoError(t, PDF(&buf, Build(ws, start, time.Now(), "test")))

	out := buf.Bytes()
	assert.True(t, bytes.HasPrefix(out, []byte("%PDF-1.4\n")))
	assert.True(t, bytes.HasSuffix(out, []byte("%%EOF\n")))
	assert.Contains(t, buf.String(), "(Enua 22/1 Wedding Cake)", "Should set the strain's name")
	// Every object starts where the cross-reference table says it does, or a
	// reader has to repair the file before it can show it.
	xref := regexp.MustCompile(`startxref\n(\d+)`).FindSubmatch(out)
	require.NotNil(t, xref)
	at, _ := strconv.Atoi(string(xref[1]))
	require.True(t, bytes.HasPrefix(out[at:], []byte("xref\n")), "startxref names the table")
	offsets := regexp.MustCompile(`(\d{10}) 00000 n`).FindAllSubmatch(out, -1)
	require.Len(t, offsets, 6)
	for i, o := range offsets {
		off, _ := strconv.Atoi(string(o[1]))
		assert.True(t, bytes.HasPrefix(out[off:], fmt.Appendf(nil, "%d 0 obj", i+1)), "object %d", i+1)
	}
}

func TestWinAnsi(t *testing.T) {
	assert.Equal(t, "\x80 \xe4\xf6\xfc\xdf \x96 ?", winAnsi("€ äöüß – ✓"), "The euro, the umlauts and a dash; nothing else")
	assert.Equal(t, `a\(b\)\\`, escape(`a(b)\`))
	p := &page{}
	assert.InDelta(t, 6.67, p.width("A", 10, false), 0.001, "Helvetica's A is 667 units")
	assert.InDelta(t, 7.22, p.width("A", 10, true), 0.001, "and 722 in bold")
}

func TestTheReportNamesItsSeal(t *testing.T) {
	ws, start := fixture(t)
	assert.Nil(t, Build(ws, start, time.Now(), "test").Seal, "Nothing sealed yet")
	key, _, err := seal.Key(ws.Repo.SealKeyPath())
	require.NoError(t, err)
	events, err := ws.Journal().Events()
	require.NoError(t, err)
	s := seal.Make(key, events[len(events)-1].Hash, len(events), time.Now())
	require.NoError(t, (seal.Store{Dir: ws.Repo.SealsDir()}).Save(seal.File{Name: s.Name(), Text: s.String()}))

	r := Build(ws, start, time.Now(), "test")

	require.NotNil(t, r.Seal, "Should find the seal of its head")
	var buf bytes.Buffer
	require.NoError(t, PDF(&buf, r))
	assert.Contains(t, buf.String(), seal.Fingerprint(s.Key), "Should print the key's fingerprint")
}

func TestHoldings(t *testing.T) {
	ws, start := fixture(t)

	items := Holdings(ws, false)

	require.Len(t, items, 1)
	it := items[0]
	assert.Equal(t, "Enua 22/1 Wedding Cake", it.Name)
	assert.Equal(t, 14.0, it.Sealed)
	assert.Equal(t, 5.5, it.Ground, "Ground six, half a gram through a session")
	require.Len(t, it.Fills, 1)
	assert.Equal(t, seal.Dispensed{Day: start.Format(time.DateOnly), Grams: 20}, it.Fills[0], "The fill its sealed grams stand on")
}

func TestQR(t *testing.T) {
	svg, err := QR("https://thedondope.github.io/wits/verify.html#z")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(svg, `<svg xmlns="http://www.w3.org/2000/svg"`))
	assert.Contains(t, svg, "h1v1h-1z", "Should draw the dark modules")
}
