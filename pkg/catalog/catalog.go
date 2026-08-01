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

	can "github.com/TheDonDope/wits-tui/pkg/cannabis"
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
	return os.WriteFile(path, data, 0600)
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
	needle := strings.ToLower(ref)

	for _, p := range c.Products {
		if p.Slug == ref || strings.EqualFold(p.Name, ref) {
			return p, nil
		}
	}

	var matches []*Product
	for _, p := range c.Products {
		if strings.Contains(strings.ToLower(p.Slug), needle) || strings.Contains(strings.ToLower(p.Name), needle) {
			matches = append(matches, p)
		}
	}
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

// Slugify turns a display name into a stable, typeable identifier.
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
	s = ratio.ReplaceAllString(s, "")
	s = nonAlphanumeric.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
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
