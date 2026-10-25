// Package research looks a product's terpenes up on the web: a web search
// for the product as the pharmacy sells it, whose results' words are read for
// terpene names, and Leafly's profile of the strain it is grown from. It
// writes nothing down — what it finds is offered, and a person keeps it or not.
package research

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"

	can "github.com/TheDonDope/wits/pkg/cannabis"
	"github.com/TheDonDope/wits/pkg/catalog"
)

// Query is what is looked up: the product's name as the pharmacy sells it,
// and the strain it is grown from, as prices.StrainOf names it.
type Query struct {
	Name   string
	Strain string
}

// Source is one page that names terpenes: a search result, or Leafly's
// profile of the strain. Terpenes are in the order the page gives them.
type Source struct {
	Site     string   `json:"site"`
	Title    string   `json:"title"`
	URL      string   `json:"url"`
	Terpenes []string `json:"terpenes"`
	// Strain is true when the page is about the strain, not this product:
	// another grower's harvest of it can run differently.
	Strain bool `json:"strain,omitempty"`
}

// Findings is what a lookup found: every source that named terpenes, and a
// profile from them, dominant first — the terpenes most sources name, ahead
// of those named earlier on fewer.
type Findings struct {
	Search    string   `json:"search"`
	Sources   []Source `json:"sources"`
	Suggested []string `json:"suggested"`
}

// Engine is a web search that answers in HTML: where it is asked, what
// opens each of its results on the page, and where a result's title is.
type Engine struct {
	Name   string
	URL    string
	Result *regexp.Regexp
	Title  *regexp.Regexp
}

// Sites are where a lookup asks: the web searches, tried in turn until one
// answers with results, and Leafly's strain search and strain pages. Tests
// point them at a stand-in.
//
// Relay, when set, is a reader every page is fetched through, its address
// put before the page's: a browser lets a page read another site only when
// that site allows it, and no search does, nor Leafly, so the phone's engine
// asks a reader that allows it and hands the page back.
type Sites struct {
	Engines     []Engine
	LeaflyAPI   string
	LeaflyPages string
	Relay       string
}

// Web is the sites themselves. Each search turns a client away now and then
// with a page asking it to prove it is a person; that page has no results,
// and the next search is asked.
var Web = Sites{
	Engines: []Engine{
		{Name: "Brave Search", URL: "https://search.brave.com/search",
			Result: regexp.MustCompile(`data-type="web"`),
			Title:  regexp.MustCompile(`(?s)class="title[^"]*"[^>]*>(.*?)</div>`)},
		{Name: "DuckDuckGo", URL: "https://html.duckduckgo.com/html/",
			Result: regexp.MustCompile(`class="result__a"`),
			Title:  regexp.MustCompile(`(?s)^[^>]*>(.*?)</a>`)},
	},
	LeaflyAPI:   "https://consumer-api.leafly.com/api/strains/v1/search",
	LeaflyPages: "https://www.leafly.com/strains/",
}

// Phone is the sites as the phone's engine reaches them, through Jina's
// Reader, which lets any page read what it fetches. Brave's results do not
// come through it, so the phone searches DuckDuckGo.
var Phone = Sites{
	Engines:     Web.Engines[1:],
	LeaflyAPI:   Web.LeaflyAPI,
	LeaflyPages: Web.LeaflyPages,
	Relay:       "https://r.jina.ai/",
}

// ErrUnreachable is a lookup that reached no site at all.
var ErrUnreachable = errors.New("the web could not be reached to look the terpenes up")

// most is how many terpenes a suggested profile keeps: a label lists three to
// five, and the tail of a longer list is traces.
const most = 5

// Look searches the web for the product, and Leafly for its strain, and
// reads the terpenes each names. Either may come back empty; only when
// neither site answered at all is it an error.
func Look(ctx context.Context, client *http.Client, sites Sites, q Query) (Findings, error) {
	out := Findings{Sources: []Source{}, Suggested: []string{}}
	f := fetcher{client: client, relay: sites.Relay}
	found, searched, errSearch := searchAll(ctx, f, sites.Engines, terms(q))
	out.Search = searched
	out.Sources = append(out.Sources, found...)
	strain, errLeafly := leafly(ctx, f, sites, q.Strain)
	if strain != nil {
		out.Sources = append(out.Sources, *strain)
	}
	if errSearch != nil && errLeafly != nil {
		return out, fmt.Errorf("%w: %w", ErrUnreachable, errSearch)
	}
	out.Suggested = Rank(out.Sources)
	return out, nil
}

// terms is what the web is searched for: the product as the pharmacy sells
// it, and the word its pages list terpenes under.
func terms(q Query) string {
	name := strings.TrimSpace(q.Name)
	if name == "" {
		name = q.Strain
	}
	return name + " terpene"
}

// errNoResults is a search that answered with no results: nothing found, or
// the page asking to prove a person is searching.
var errNoResults = errors.New("the search answered with no results")

// searchAll asks each search in turn, until one answers with results, and
// says which address it asked last.
func searchAll(ctx context.Context, f fetcher, engines []Engine, terms string) ([]Source, string, error) {
	err := errNoResults
	asked := ""
	for _, e := range engines {
		asked = e.URL + "?" + url.Values{"q": {terms}}.Encode()
		var found []Source
		if found, err = search(ctx, f, e, asked); err == nil {
			return found, asked, nil
		}
	}
	return nil, asked, err
}

// Rank is a profile from every source, dominant first: the terpenes named by
// the most sources lead, and among those named as often, the one named
// earlier on average. It keeps the first five.
func Rank(sources []Source) []string {
	type tally struct {
		name  string
		votes int
		place int
	}
	by := map[string]*tally{}
	for _, s := range sources {
		for i, t := range s.Terpenes {
			k := catalog.TerpeneKey(t)
			if by[k] == nil {
				by[k] = &tally{name: t}
			}
			by[k].votes++
			by[k].place += i
		}
	}
	all := make([]*tally, 0, len(by))
	for _, t := range by {
		all = append(all, t)
	}
	sort.Slice(all, func(i, j int) bool {
		a, b := all[i], all[j]
		if a.votes != b.votes {
			return a.votes > b.votes
		}
		// place/votes compared without dividing: a.place/a.votes < b.place/b.votes.
		if a.place*b.votes != b.place*a.votes {
			return a.place*b.votes < b.place*a.votes
		}
		return a.name < b.name
	})
	out := []string{}
	for _, t := range all[:min(len(all), most)] {
		out = append(out, t.name)
	}
	return out
}

// fetcher fetches pages, directly or through a relay.
type fetcher struct {
	client *http.Client
	relay  string
}

// get fetches a page, refusing anything but a 200. Through a relay, it asks
// for the page as HTML — or, for an API's JSON, as the text it is — rather
// than the reader's own rendering of it.
func (f fetcher) get(ctx context.Context, u string, asText bool) ([]byte, error) {
	if f.relay != "" {
		u = f.relay + u
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if f.relay != "" {
		format := "html"
		if asText {
			format = "text"
		}
		req.Header.Set("X-Return-Format", format)
	} else {
		// The search answers a browser; a client that names itself gets nothing.
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) wits")
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %s", req.URL.Host, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 4<<20))
}

var (
	tag     = regexp.MustCompile(`<[^>]*>`)
	spacing = regexp.MustCompile(`\s+`)
	href    = regexp.MustCompile(`href="((?:https?:)?//[^"]+)"`)
)

// search reads the terpenes each result of a web search names, anywhere in
// its title or snippet; results that name none are left out. A page with no
// results at all is errNoResults.
func search(ctx context.Context, f fetcher, e Engine, u string) ([]Source, error) {
	page, err := f.get(ctx, u, false)
	if err != nil {
		return nil, err
	}
	blocks := e.Result.Split(string(page), -1)[1:]
	if len(blocks) == 0 {
		return nil, errNoResults
	}
	var out []Source
	for _, b := range blocks[:min(len(blocks), 12)] {
		link := href.FindStringSubmatch(b)
		terps := Named(text(b))
		if link == nil || len(terps) == 0 {
			continue
		}
		to := target(html.UnescapeString(link[1]))
		title := site(to)
		if m := e.Title.FindStringSubmatch(b); m != nil {
			title = text(m[1])
		}
		out = append(out, Source{Site: site(to), Title: title, URL: to, Terpenes: terps})
	}
	return out, nil
}

// text is a fragment of HTML as words.
func text(fragment string) string {
	return strings.TrimSpace(spacing.ReplaceAllString(html.UnescapeString(tag.ReplaceAllString(fragment, " ")), " "))
}

// target is where a result's link leads: the search wraps it in a redirect
// of its own, the address in its uddg parameter.
func target(link string) string {
	if strings.HasPrefix(link, "//") {
		link = "https:" + link
	}
	u, err := url.Parse(link)
	if err != nil {
		return link
	}
	if to := u.Query().Get("uddg"); to != "" {
		return to
	}
	return link
}

// site is a link's host, without its www.
func site(link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return link
	}
	return strings.TrimPrefix(u.Hostname(), "www.")
}

// vocabulary is every terpene wits knows, keyed as catalog.TerpeneKey keys
// a name, with the name to write it down as. The cannabinoids are not
// terpenes, and a word like "CBD" must not read as one.
var vocabulary = func() map[string]string {
	out := map[string]string{}
	var names []string
	for typ, t := range can.Terpenes {
		if typ != can.Apigenin && typ != can.Quercetin && typ != can.CannaflavinA && typ != can.BetaSitosterol {
			names = append(names, t.Name)
		}
	}
	// Last, so Terpinene is the name Terpinen-4-ol's key writes, not "Terpinen".
	names = append(names, "Ocimene", "Farnesene", "Valencene", "Guaiol", "Fenchol", "Terpinene", "Bergamotene")
	for _, n := range names {
		w := longestWord(n)
		out[catalog.TerpeneKey(w)] = w
	}
	return out
}()

// longestWord is a name's longest run of letters: "β-Caryophyllene" is
// written down as Caryophyllene, "Δ-3-Carene" as Carene.
func longestWord(name string) string {
	return slices.MaxFunc(words(name), func(a, b string) int { return len(a) - len(b) })
}

// words splits text into its runs of letters.
func words(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) })
}

// Named is every terpene a text names, in the order it first names each,
// written as wits writes it: "Limonen, β-Caryophyllen" is Limonene,
// Caryophyllene.
func Named(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, w := range words(s) {
		k := catalog.TerpeneKey(w)
		if name, ok := vocabulary[k]; ok && !seen[k] {
			seen[k] = true
			out = append(out, name)
		}
	}
	return out
}

// leafly finds the strain on Leafly and reads its terpene profile; nil
// when there is no strain to ask about or Leafly does not know it.
func leafly(ctx context.Context, f fetcher, sites Sites, strain string) (*Source, error) {
	if strings.TrimSpace(strain) == "" {
		return nil, nil
	}
	// The search finds "Mac 1" by "MAC 1", not by "MAC1".
	asked := numbered.ReplaceAllString(strain, "$1 $2")
	body, err := f.get(ctx, sites.LeaflyAPI+"?"+url.Values{"q": {asked}}.Encode(), true)
	if err != nil {
		return nil, err
	}
	slug, name := leaflyHit(body, strain)
	if slug == "" {
		return nil, nil
	}
	page, err := f.get(ctx, sites.LeaflyPages+url.PathEscape(slug), false)
	if err != nil {
		return nil, err
	}
	terps := leaflyTerpenes(page)
	if len(terps) == 0 {
		return nil, nil
	}
	return &Source{Site: "leafly.com", Title: name + " on Leafly", URL: sites.LeaflyPages + slug,
		Terpenes: terps, Strain: true}, nil
}

// numbered is a letter run straight into a number, as MAC1 is.
var numbered = regexp.MustCompile(`(\pL)(\d)`)

// leaflyHit is the strain search's hit named as the strain is, or none: a
// near name is another strain ("Mac 11" is not "MAC 1").
func leaflyHit(body []byte, strain string) (slug, name string) {
	var res struct {
		Results []struct {
			Hits []struct {
				Slug string `json:"slug"`
				Name string `json:"name"`
			} `json:"hits"`
		} `json:"results"`
	}
	if json.Unmarshal(body, &res) != nil {
		return "", ""
	}
	for _, r := range res.Results {
		for _, h := range r.Hits {
			if squash(h.Name) == squash(strain) {
				return h.Slug, h.Name
			}
		}
	}
	return "", ""
}

// squash is a name with only its letters and digits, lowercase: "MAC1" and
// "Mac 1" are one.
func squash(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, s)
}

// nextData is the page data a Leafly strain page carries.
var nextData = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__" type="application/json">(.*?)</script>`)

// minScore is the least a terpene scores on Leafly to count in a profile:
// below it is a trace.
const minScore = 0.1

// leaflyTerpenes is a strain page's terpenes, strongest first.
func leaflyTerpenes(page []byte) []string {
	m := nextData.FindSubmatch(page)
	if m == nil {
		return nil
	}
	var data struct {
		Props struct {
			PageProps struct {
				Strain struct {
					Terps map[string]struct {
						Name  string  `json:"name"`
						Score float64 `json:"score"`
					} `json:"terps"`
				} `json:"strain"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if json.Unmarshal(m[1], &data) != nil {
		return nil
	}
	type scored struct {
		name  string
		score float64
	}
	var all []scored
	for _, t := range data.Props.PageProps.Strain.Terps {
		if names := Named(t.Name); len(names) == 1 && t.Score >= minScore {
			all = append(all, scored{names[0], t.Score})
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].score > all[j].score })
	out := make([]string, 0, len(all))
	for _, s := range all[:min(len(all), most)] {
		out = append(out, s.name)
	}
	return out
}
