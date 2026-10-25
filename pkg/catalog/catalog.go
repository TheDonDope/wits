package catalog

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	can "github.com/TheDonDope/wits/pkg/cannabis"
	"github.com/TheDonDope/wits/pkg/journal"
	"gopkg.in/yaml.v3"
)

var (
	// ErrNotFound is returned when no product matches a reference.
	ErrNotFound = errors.New("no product matches that reference")
	// ErrAmbiguous is returned when a reference matches more than one product.
	ErrAmbiguous = errors.New("that reference matches more than one product")
	// ErrDuplicate is returned when adding a product that is already known.
	ErrDuplicate = errors.New("a product with that slug already exists")
)

// Product is one dispensed product, identified by a stable slug.
type Product struct {
	Slug         string          `yaml:"slug"`
	Name         string          `yaml:"name"`
	Manufacturer string          `yaml:"manufacturer,omitempty"`
	Cultivar     string          `yaml:"cultivar,omitempty"`
	Country      string          `yaml:"country,omitempty"`
	Genetic      can.GeneticType `yaml:"genetic,omitempty"`
	Radiated     bool            `yaml:"radiated,omitempty"`
	THC          float64         `yaml:"thc,omitempty"`
	CBD          float64         `yaml:"cbd,omitempty"`
	Terpenes     []string        `yaml:"terpenes,omitempty"`
	AddedAt      time.Time       `yaml:"added_at"`
}

// String returns the display name of the product.
func (p Product) String() string { return p.Name }

// Catalog is the set of known products.
type Catalog struct {
	Products []*Product `yaml:"products"`
}

// Load reads the catalog from path. A missing file is an empty catalog, but an
// unreadable or malformed one is an error rather than a silent reset.
func Load(path string) (*Catalog, error) {
	c := &Catalog{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("reading the product catalog: %w", err)
	}
	return c, nil
}

// Save writes the catalog to path.
func (c *Catalog) Save(path string) error {
	sort.Slice(c.Products, func(i, j int) bool { return c.Products[i].Slug < c.Products[j].Slug })
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	// Written whole, so a crash must leave the old catalog or the new one.
	return journal.WriteFile(path, data)
}

// Add puts a product in the catalog, refusing to shadow an existing slug.
func (c *Catalog) Add(p *Product) error {
	if p.Slug == "" {
		p.Slug = Slugify(p.Name)
	}
	for _, existing := range c.Products {
		if existing.Slug == p.Slug {
			return fmt.Errorf("%w: %s", ErrDuplicate, p.Slug)
		}
	}
	if p.AddedAt.IsZero() {
		p.AddedAt = time.Now()
	}
	// To the second, like the journal's timestamps: a bundle stores this as
	// whole seconds, so keeping nanoseconds here would mean a catalog did not
	// survive a round trip through one unchanged.
	p.AddedAt = p.AddedAt.Truncate(time.Second)
	c.Products = append(c.Products, p)
	return nil
}

// Find resolves a reference to a product. A reference is an exact slug, an
// exact display name, or an unambiguous prefix or substring of either, so that
// a daily entry can be typed as `wits grind cake 0.75`.
func (c *Catalog) Find(ref string) (*Product, error) {
	if ref == "" {
		return nil, ErrNotFound
	}
	if p := c.Exact(ref); p != nil {
		return p, nil
	}
	matches := c.Partial(ref)
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("%w: %q", ErrNotFound, ref)
	case 1:
		return matches[0], nil
	default:
		names := make([]string, 0, len(matches))
		for _, p := range matches {
			names = append(names, p.Slug)
		}
		return nil, fmt.Errorf("%w: %q matches %s", ErrAmbiguous, ref, strings.Join(names, ", "))
	}
}

// Exact returns the product whose slug or name is ref, ignoring case in the
// name, or nil. Find also accepts part of a name; a purchase must not, since
// a reference that matches no product in full adds one.
func (c *Catalog) Exact(ref string) *Product {
	for _, p := range c.Products {
		if ref != "" && (p.Slug == ref || strings.EqualFold(p.Name, ref)) {
			return p
		}
	}
	return nil
}

// Partial returns the products whose slug or name contains ref, ignoring case.
func (c *Catalog) Partial(ref string) []*Product {
	needle := strings.ToLower(ref)
	var matches []*Product
	for _, p := range c.Products {
		if strings.Contains(strings.ToLower(p.Slug), needle) || strings.Contains(strings.ToLower(p.Name), needle) {
			matches = append(matches, p)
		}
	}
	return matches
}

var (
	nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)
	// ratio matches the THC/CBD ratio a dispensed product is labelled with,
	// for example the "22/1" in "Enua 22/1 Wedding Cake".
	ratio = regexp.MustCompile(`(\d+(?:[.,]\d+)?)\s*/\s*(\d+(?:[.,]\d+)?)`)
)

// labelPrefixes are column headings that ended up in front of a product name
// in the spreadsheet, in both languages it was kept in. They say how much was
// dispensed, not what it was, so they are not part of the product's identity.
var labelPrefixes = []string{"quantity delivered", "quantity", "liefermenge"}

// Slugify turns a display name into a stable, typeable identifier, ending in
// the THC/CBD ratio.
func Slugify(name string) string {
	s := strings.ToLower(name)
	s = strings.TrimSuffix(strings.TrimSpace(s), "(g)")
	s = strings.TrimSpace(s)
	for _, prefix := range labelPrefixes {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimSpace(strings.TrimPrefix(s, prefix))
			break
		}
	}
	// The ratio comes off wherever it was written and goes back on the end, so
	// that a name carrying it in the middle and one carrying it at the end
	// still slug the same way.
	potency := potencyIn(s)
	s = ratio.ReplaceAllString(s, "")
	s = nonAlphanumeric.ReplaceAllString(s, "-")
	return withPotency(strings.Trim(s, "-"), potency)
}

// potencyIn reads a THC/CBD ratio out of a name as the digits a slug carries:
// "25/1" becomes "251".
func potencyIn(name string) string {
	m := ratio.FindStringSubmatch(name)
	if m == nil {
		return ""
	}
	return decimalPoints.Replace(m[1]) + decimalPoints.Replace(m[2])
}

// potencyOf is potencyIn for a product that has already been parsed.
func potencyOf(p *Product) string {
	if p == nil || (p.THC == 0 && p.CBD == 0) {
		return ""
	}
	return decimalPoints.Replace(strconv.FormatFloat(p.THC, 'f', -1, 64)) +
		decimalPoints.Replace(strconv.FormatFloat(p.CBD, 'f', -1, 64))
}

// decimalPoints strips the separator out of a ratio, so that 22,5/1 reads as
// 2251 rather than growing a dash in the middle of the suffix.
var decimalPoints = strings.NewReplacer(".", "", ",", "")

// withPotency appends the potency suffix, which is what keeps one cultivar from
// one manufacturer at two strengths from collapsing into a single product. Two
// of those were found in four years of real records, and each was a pair of
// genuinely different prescriptions.
func withPotency(slug, potency string) string {
	switch {
	case potency == "":
		return slug
	case slug == "":
		return potency
	default:
		return slug + "-" + potency
	}
}

// Parse makes a best effort at pulling structure out of a dispensed product
// name, which is conventionally "{Manufacturer} {THC}/{CBD} {Cultivar}".
//
// The convention is not reliable enough to trust blind: the cultivar sometimes
// comes before the ratio, product lines are inlined, and there are loose
// country and batch codes. Whatever this gets wrong is meant to be corrected by
// hand afterwards, so it never guesses beyond what the name plainly says.
func Parse(name string) *Product {
	p := &Product{Name: strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(name), "(g)"))}
	p.Name = strings.TrimSpace(p.Name)
	p.Slug = Slugify(p.Name)

	loc := ratio.FindStringSubmatchIndex(p.Name)
	if loc == nil {
		return p
	}
	m := ratio.FindStringSubmatch(p.Name)
	p.THC = parseFloat(m[1])
	p.CBD = parseFloat(m[2])
	p.Manufacturer = strings.TrimSpace(strings.Trim(p.Name[:loc[0]], " :,-"))
	p.Cultivar = strings.TrimSpace(strings.Trim(p.Name[loc[1]:], " :,-."))
	return p
}

// parseFloat reads a decimal that may use a comma as its separator.
func parseFloat(s string) float64 {
	v, err := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64)
	if err != nil {
		return 0
	}
	return v
}

// Handle length bounds for a generated slug.
const (
	minHandle = 3
	maxHandle = 5
)

// validHandle is what a slug given by hand is allowed to look like. Slugs are
// typed daily and matched by prefix, so they stay lowercase and free of
// anything that needs quoting on a command line.
var validHandle = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,23}$`)

// ErrBadSlug is returned for a slug that cannot be used.
var ErrBadSlug = errors.New("a slug must be 2 to 24 characters of lowercase letters, digits or dashes")

// CheckSlug reports whether a slug given by hand can be used.
func CheckSlug(slug string) error {
	if !validHandle.MatchString(slug) {
		return fmt.Errorf("%w: %q", ErrBadSlug, slug)
	}
	return nil
}

// NewHandle returns a short, typeable slug for a product: three to five
// characters naming the cultivar, then the THC/CBD ratio.
//
// The long form derived from a display name — enua-wedding-cake-221 — is precise
// but nobody wants to type it every evening. This produces something closer to
// what a person would abbreviate it to themselves, preferring the cultivar over
// the manufacturer, because the cultivar is what distinguishes one jar from the
// next.
//
// The ratio rides along because the same cultivar at two strengths is two
// prescriptions: wcake-221 and wcake-251 are legible as a pair, where the
// numbering that would otherwise separate them — wcake and wcak2 — is not.
func NewHandle(p *Product, existing []string) string {
	potency := potencyOf(p)
	// The length bounds are on the memorable part. The suffix is carried on top
	// of it, and the nearness rule judges the whole slug, since that is what
	// gets typed and resolved by prefix.
	near := notNear(existing)
	free := func(base string) bool { return near(withPotency(base, potency)) }
	words := handleBasis(p)

	for _, candidate := range handleCandidates(words) {
		if len(candidate) >= minHandle && len(candidate) <= maxHandle && free(candidate) {
			return withPotency(candidate, potency)
		}
	}
	return withPotency(numberedHandle(words, free), potency)
}

// notNear reports whether a candidate is far enough from every slug already in
// use.
//
// Not merely different: a handle that is a prefix of another, or has another as
// its prefix, is one keystroke from the wrong jar, and references are resolved
// by prefix. wcake and wcak must not both exist.
func notNear(existing []string) func(string) bool {
	return func(candidate string) bool {
		for _, slug := range existing {
			if strings.HasPrefix(candidate, slug) || strings.HasPrefix(slug, candidate) {
				return false
			}
		}
		return true
	}
}

// handleBasis reduces a product to the words worth abbreviating, preferring the
// cultivar, since that is what distinguishes one jar from the next.
func handleBasis(p *Product) []string {
	basis := p.Cultivar
	if strings.TrimSpace(basis) == "" {
		basis = p.Name
	}
	words := handleWords(basis)
	full := handleWords(p.Name)
	switch {
	case len(words) == 0:
		return full
	case len(words) == 1 && len(words[0]) < minHandle:
		// One short word on its own has nothing to shorten; the rest of the
		// name is what is left to borrow from.
		return append(words, full...)
	default:
		return words
	}
}

// numberedHandle is the last resort, when everything memorable is taken. The
// point is then to be short and unique: somebody with three Wedding Cakes open
// at once needs to tell them apart more than they need a mnemonic.
//
// Shorter stems come first within each number so the digit stays visible: wedd
// being taken gives wed2, not wedd2 — which the nearness rule rejects anyway.
func numberedHandle(words []string, free func(string) bool) string {
	stem := "p"
	if len(words) > 0 {
		stem = words[0]
	}
	for n := 2; ; n++ {
		suffix := strconv.Itoa(n)
		for keep := maxHandle - len(suffix); keep >= minHandle-len(suffix); keep-- {
			if keep <= 0 || keep > len(stem) {
				continue
			}
			candidate := stem[:keep] + suffix
			if len(candidate) >= minHandle && len(candidate) <= maxHandle && free(candidate) {
				return candidate
			}
		}
	}
}

// handleCandidates offers abbreviations in the order a person would reach for
// them: the initials, then a contraction, then the beginning of the first word.
func handleCandidates(words []string) []string {
	if len(words) == 0 {
		return nil
	}
	var out []string

	initials := ""
	for _, w := range words {
		initials += w[:1]
	}
	out = append(out, initials)

	if len(words) > 1 {
		last := words[len(words)-1]
		// One letter of the first word and the head of the last: "Wedding Cake"
		// reads as wcake, which is what it gets called anyway.
		for _, n := range []int{4, 3, 2} {
			if len(last) >= n {
				out = append(out, words[0][:1]+last[:n])
			}
		}
	}
	// Four characters before five: "muns" is how a word gets shortened by hand,
	// "munso" is how a field gets truncated by a program.
	for _, n := range []int{4, maxHandle, minHandle} {
		if len(words[0]) >= n {
			out = append(out, words[0][:n])
		}
	}
	// A word too short to abbreviate borrows a letter rather than a number,
	// which would otherwise read as though something had collided.
	if len(words[0]) < minHandle {
		if len(words) > 1 {
			out = append(out, words[0]+words[1])
		}
		for _, w := range words[1:] {
			out = append(out, words[0]+w[:1])
		}
	}
	return out
}

// handleWords reduces a name to lowercase alphanumeric words, dropping the
// THC/CBD ratio and anything too short to abbreviate from.
func handleWords(s string) []string {
	s = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(s), "(g)"))
	s = ratio.ReplaceAllString(s, " ")
	var words []string
	for _, w := range strings.FieldsFunc(s, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	}) {
		if w != "" {
			words = append(words, w)
		}
	}
	return words
}

// Handles returns every slug in the catalog.
func (c *Catalog) Handles() []string {
	out := make([]string, 0, len(c.Products))
	for _, p := range c.Products {
		out = append(out, p.Slug)
	}
	return out
}

// Taken reports whether a slug is already in the catalog. Unlike the check a
// generated handle is held to, this is exact: a slug given by hand is the
// caller's business as long as it is not literally somebody else's.
func (c *Catalog) Taken(slug string) bool {
	for _, p := range c.Products {
		if p.Slug == slug {
			return true
		}
	}
	return false
}

// ParseTerpenes reads terpenes as a label lists them, dominant first, by
// commas or semicolons: "Myrcen, Limonen; β-Caryophyllen".
func ParseTerpenes(s string) []string {
	var out []string
	for _, t := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' }) {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// TerpeneKey is a terpene's name as two labels agree on it: lowercase, without
// its α or β, and without the e German leaves off — "β-Myrcen" and "myrcene"
// are one terpene.
func TerpeneKey(name string) string {
	k := strings.ToLower(strings.TrimSpace(name))
	for _, prefix := range []string{"α-", "β-", "γ-", "δ-", "alpha-", "beta-", "alpha ", "beta ", "a-", "b-"} {
		k = strings.TrimPrefix(k, prefix)
	}
	return strings.TrimSuffix(strings.TrimSpace(k), "e")
}

// Label is what a pharmacy label says about a fill, as far as it can be read
// from the text an OCR made of a photograph of it. Anything not found is left
// empty; the form it fills is checked by a person before it is recorded.
type Label struct {
	// Name is the product line, the way a purchase names a product.
	Name string
	// THC and CBD are the ratio: off the product line, or off the label's
	// percentages when no line carries one.
	THC, CBD float64
	// Grams is what was dispensed, zero when no amount was found.
	Grams journal.Centigrams
}

var (
	// percentage is a THC or CBD content as a label prints it: "THC: 22 %",
	// "CBD < 1,0 %".
	percentage = regexp.MustCompile(`(?i)\b(thc|cbd)\b[^0-9<%]{0,12}(<\s*)?(\d+(?:[.,]\d+)?)\s*%`)
	// amount is a weight in grams — "10 mg" does not match, since the unit
	// follows the digits — or the number a column headed "(g)" gives.
	amount = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*(?:g|gr|gramm)\b|\(g\)[\s:]*(\d+(?:[.,]\d+)?)`)
	// bareAmount is a number standing alone, for the line that names the
	// quantity without its unit: "Liefermenge (g): 10".
	bareAmount = regexp.MustCompile(`(?:^|[^/\d.,])(\d+(?:[.,]\d+)?)\s*$`)
	// quantityWords mark the line that says how much was dispensed.
	quantityWords = regexp.MustCompile(`(?i)menge|inhalt|quantity|netto|gewicht|packungsgr`)
	// labelNoise is what a label prints on the product line that is not the
	// product: what kind of thing it is, and how much of it there is.
	labelNoise = regexp.MustCompile(`(?i)^(?:liefermenge|quantity delivered|quantity)\b[\s:]*|\b(?:medizinal-?cannabisblüten|cannabisblüten|cannabis flos|blüten|unzerkleinert)\b,?|\s*\(g\)[\s:]*(?:\d+(?:[.,]\d+)?)?|\b\d+(?:[.,]\d+)?\s*(?:g|gr|gramm)\b|\b(?:thc|cbd)\b[^0-9<%]{0,12}(?:<\s*)?\d+(?:[.,]\d+)?\s*%`)
	// addressWords mark a line of the pharmacy's address, whose house number
	// can look like a ratio: "Hauptstr. 12/1".
	addressWords = regexp.MustCompile(`(?i)stra(?:ß|ss)e|str\.|platz\b|\btel\b|telefon|\bfax\b`)
)

// Most a prescription holds, and the strongest ratio a flower is sold at; an
// amount or a ratio past them is something else that matched — a date, a
// batch number.
const (
	mostGrams = 100
	mostTHC   = 45
	mostCBD   = 30
)

// ReadLabel reads the product line and the grams off a label's text. The
// product line is the first that carries a THC/CBD ratio; the grams are the
// amount on the line that names the quantity, else on the product line, else
// the first plausible amount anywhere.
func ReadLabel(text string) Label {
	lines := labelLines(text)
	var l Label
	at := -1
	for i, line := range lines {
		if thc, cbd, ok := ratioIn(line); ok {
			l.THC, l.CBD, at = thc, cbd, i
			l.Name = productLine(lines, i)
			break
		}
	}
	if at < 0 {
		l.THC, l.CBD = percentages(lines)
	}
	l.Grams = labelGrams(lines, at)
	return l
}

// labelLines splits a label's text into its lines, each with its runs of
// whitespace made one space, dropping the empty ones.
func labelLines(text string) []string {
	var out []string
	for line := range strings.SplitSeq(text, "\n") {
		if line = strings.Join(strings.Fields(line), " "); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// ratioIn reads a THC/CBD ratio off a line, refusing one that is part of a
// date or out of the range a flower is sold at.
func ratioIn(line string) (thc, cbd float64, ok bool) {
	loc := ratio.FindStringSubmatchIndex(line)
	if loc == nil || addressWords.MatchString(line) {
		return 0, 0, false
	}
	// 03/2027, 12/10/2026: a date goes on, or came from, another separator.
	if strings.ContainsAny(after(line, loc[1]), "/.") || strings.ContainsAny(before(line, loc[0]), "/.") {
		return 0, 0, false
	}
	thc, cbd = parseFloat(line[loc[2]:loc[3]]), parseFloat(line[loc[4]:loc[5]])
	return thc, cbd, thc > 0 && thc <= mostTHC && cbd <= mostCBD
}

// after is the character following position i in s, if it is a separator
// followed by a digit; before is the same for the character preceding it.
func after(s string, i int) string {
	if i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9' {
		return s[i : i+1]
	}
	return ""
}

func before(s string, i int) string {
	if i >= 2 && s[i-2] >= '0' && s[i-2] <= '9' {
		return s[i-1 : i]
	}
	return ""
}

// productLine is the product a label's line i names, without the words around
// it that say it is cannabis and how much. A line that is nothing but the
// ratio is the end of the line above it.
func productLine(lines []string, i int) string {
	name := strings.TrimSpace(labelNoise.ReplaceAllString(lines[i], " "))
	name = strings.Join(strings.Fields(strings.Trim(name, " :,-")), " ")
	if ratio.ReplaceAllString(name, "") == "" && i > 0 {
		name = strings.TrimSpace(labelNoise.ReplaceAllString(lines[i-1], " ")) + " " + name
	}
	return unshout(name)
}

// unshout gives a line printed all in capitals the case a name is kept in:
// words of four letters or more are capitalised, shorter ones — CA, DNK,
// MAC — are codes and stay as they are.
func unshout(name string) string {
	if strings.ToUpper(name) != name || strings.ToLower(name) == name {
		return name
	}
	words := strings.Fields(name)
	for i, w := range words {
		if r := []rune(w); len(r) >= 4 && strings.IndexFunc(w, func(c rune) bool { return c >= '0' && c <= '9' }) < 0 {
			words[i] = string(r[0]) + strings.ToLower(string(r[1:]))
		}
	}
	return strings.Join(words, " ")
}

// percentages reads the ratio off a label's THC and CBD contents, the way a
// product is named for them: "CBD < 1 %" is the 1 in 22/1.
func percentages(lines []string) (thc, cbd float64) {
	for _, line := range lines {
		for _, m := range percentage.FindAllStringSubmatch(line, -1) {
			v := parseFloat(m[3])
			switch {
			case strings.EqualFold(m[1], "thc") && thc == 0 && v <= mostTHC:
				thc = v
			case strings.EqualFold(m[1], "cbd") && cbd == 0 && v <= mostCBD:
				cbd = max(v, 1)
			}
		}
	}
	return thc, cbd
}

// labelGrams is the amount dispensed: on the line naming the quantity, then on
// the product line at, then the first plausible amount anywhere.
func labelGrams(lines []string, at int) journal.Centigrams {
	for i, line := range lines {
		// The product line's heading can be a quantity's — "Quantity ...
		// Gorilla Glue #4" — and the number ending it is not grams.
		if !quantityWords.MatchString(line) {
			continue
		}
		if g := gramsIn(line, i != at); g > 0 {
			return g
		}
	}
	if at >= 0 {
		if g := gramsIn(lines[at], false); g > 0 {
			return g
		}
	}
	for _, line := range lines {
		if g := gramsIn(line, false); g > 0 {
			return g
		}
	}
	return 0
}

// gramsIn is the first plausible weight on a line, or a number ending a line
// that names the quantity without a unit.
func gramsIn(line string, quantity bool) journal.Centigrams {
	var found []string
	for _, m := range amount.FindAllStringSubmatch(line, -1) {
		found = append(found, m[1]+m[2])
	}
	if m := bareAmount.FindStringSubmatch(line); quantity && m != nil {
		found = append(found, m[1])
	}
	for _, s := range found {
		if g := parseFloat(s); g > 0 && g <= mostGrams {
			return journal.CG(g)
		}
	}
	return 0
}

// Matching is the product already in the catalog that a label's product line
// names, or nil: by its name or slug, by its name with the ratio written
// elsewhere or a column heading in front, or — at the same ratio and from the
// same manufacturer — by every word of its name but one, when no other product
// comes as close.
func (c *Catalog) Matching(name string) *Product {
	if name == "" {
		return nil
	}
	if p := c.Exact(name); p != nil {
		return p
	}
	slug := Slugify(name)
	for _, p := range c.Products {
		if Slugify(p.Name) == slug {
			return p
		}
	}
	return c.closest(Parse(name), handleWords(slug))
}

// closest is the one product at the label's ratio whose name shares its first
// word — the manufacturer — and every other word but one with the label's.
// The words are a slug's, so a column heading in front counts for nothing and
// the ratio counts as one of them.
func (c *Catalog) closest(label *Product, words []string) *Product {
	have := make(map[string]bool, len(words))
	for _, w := range words {
		have[w] = true
	}
	var best *Product
	bestShared, tied := 0, false
	for _, p := range c.Products {
		own := handleWords(Slugify(p.Name))
		if p.THC != label.THC || p.CBD != label.CBD || len(own) == 0 || !have[own[0]] {
			continue
		}
		shared := 0
		for _, w := range own {
			if have[w] {
				shared++
			}
		}
		if shared < 3 || shared < len(own)-1 {
			continue
		}
		switch {
		case shared > bestShared:
			best, bestShared, tied = p, shared, false
		case shared == bestShared:
			tied = true
		}
	}
	if tied {
		return nil
	}
	return best
}
