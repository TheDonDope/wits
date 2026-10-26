package report

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	can "github.com/TheDonDope/wits/pkg/cannabis"
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

func TestStrains(t *testing.T) {
	r, err := repo.Init(t.TempDir())
	require.NoError(t, err)
	ws, err := workspace.Read(r)
	require.NoError(t, err)
	at := time.Now().AddDate(0, 0, -10)
	for _, name := range []string{"Enua 22/1 Wedding Cake", "Khiron 20/1 Wedding Cake", "Cantourage 25/1 MAC1+",
		"Barongo 27/1 MAC3", "Sumo 25/1 Papaya Bomb"} {
		_, _, _, err = ws.Recorder.Buy(name, "", 10, at)
		require.NoError(t, err)
	}
	ws, err = workspace.Read(r)
	require.NoError(t, err)
	// like is what a product is most like, by name, first.
	like := func(ws *workspace.Workspace, name string) (Strain, string) {
		all := Strains(ws)
		i := slices.IndexFunc(all, func(s Strain) bool { return s.Name == name })
		require.GreaterOrEqual(t, i, 0, name)
		j := slices.IndexFunc(all, func(s Strain) bool { return s.Slug == all[i].Like[0].Slug })
		return all[i], all[j].Name
	}

	require.Len(t, Strains(ws), 5)
	wcake, first := like(ws, "Enua 22/1 Wedding Cake")
	assert.Equal(t, "Khiron 20/1 Wedding Cake", first, "The other Wedding Cake first")
	assert.False(t, wcake.Placed, "No terpenes, no place on the map")
	_, first = like(ws, "Cantourage 25/1 MAC1+")
	assert.Equal(t, "Barongo 27/1 MAC3", first, "MAC1+ and MAC3 are one family")

	// A shared terpene profile brings together two products that share no
	// word, past one nearer in THC whose terpenes are not known.
	_, first = like(ws, "Sumo 25/1 Papaya Bomb")
	require.Equal(t, "Cantourage 25/1 MAC1+", first, "At first the nearest THC, 25 against 25")
	for _, name := range []string{"Sumo 25/1 Papaya Bomb", "Khiron 20/1 Wedding Cake"} {
		_, err = ws.Recorder.Terpenes(name, []string{"Myrcen", "Limonen", "Caryophyllen"})
		require.NoError(t, err)
	}
	ws, err = workspace.Read(r)
	require.NoError(t, err)
	sumo, first := like(ws, "Sumo 25/1 Papaya Bomb")
	assert.Equal(t, "Khiron 20/1 Wedding Cake", first, "Alike in terpenes, alike on the map")
	assert.Equal(t, []string{"Myrcen", "Limonen", "Caryophyllen"}, sumo.Like[0].Shared)
	assert.Greater(t, sumo.Like[0].Score, 0.85, "the same terpenes, by another name and strength")
	assert.Equal(t, "Musk, citrus and herbs", sumo.Taste)
}

// TestLikeLeansOnTerpenes: the profile decides what a strain is like, not
// its strength — a match in terpenes five points of THC apart comes before a
// strain of the same THC that tastes of something else — and the terpene
// only some profiles name tells more than the one nearly all do.
func TestLikeLeansOnTerpenes(t *testing.T) {
	base := []Strain{
		{Slug: "a", THC: 22, Terpenes: []string{"Limonene", "Caryophyllene", "Terpinolene"}},
		{Slug: "b", THC: 27, Terpenes: []string{"Limonene", "Terpinolene", "Caryophyllene"}},
		{Slug: "c", THC: 22, Terpenes: []string{"Myrcene", "Pinene", "Humulene"}},
		{Slug: "d", THC: 22, Terpenes: []string{"Limonene", "Caryophyllene", "Myrcene"}},
		{Slug: "e", THC: 25, Terpenes: []string{"Limonene", "Caryophyllene", "Linalool"}},
	}
	for i := range base {
		base[i].Aroma = can.Aroma(base[i].Terpenes)
	}
	v := terpeneVectors(base)
	none := map[string]bool{}
	sim := func(i, j int) float64 { return similarity(base[i], base[j], v[i], v[j], none, none) }
	assert.Greater(t, sim(0, 1), sim(0, 2), "the same terpenes, five points of THC apart, before the same THC")
	assert.Greater(t, sim(0, 1), sim(0, 3), "sharing the rare Terpinolene tells more than sharing Myrcene")
	assert.Greater(t, sim(0, 3), sim(0, 2))

	place(base, v)
	for _, s := range base {
		assert.True(t, s.Placed, s.Slug)
	}
}

// TestTerpenes: each terpene the shelf names, the most named first, its
// taste from what wits knows, and its rating weighed by where it stands.
func TestTerpenes(t *testing.T) {
	all := Terpenes([]Strain{
		{Terpenes: []string{"Limonen", "Myrcene"}, Ratings: 4, Overall: 4},
		{Terpenes: []string{"Myrcen", "Limonene", "Ocimene"}, Ratings: 2, Overall: 2},
	})
	require.Len(t, all, 3)
	assert.Equal(t, "Limonen", all[0].Name)
	assert.Equal(t, 2, all[0].Products)
	assert.Equal(t, 1, all[0].Leads)
	// Limonene leads the first (4 sessions at 4, weight 4) and is second in
	// the second (2 at 2, weight 1): (16 + 2) / 5.
	assert.InDelta(t, 3.6, all[0].Overall, 1e-9)
	assert.Equal(t, 6, all[0].Rated)
	assert.Equal(t, can.Citrus, all[0].Family)
	ocimene := all[slices.IndexFunc(all, func(x Terpene) bool { return x.Name == "Ocimene" })]
	assert.Equal(t, can.Sweet, ocimene.Family, "a minor terpene, tasted all the same")
	assert.Zero(t, ocimene.BoilingPoint)
}
