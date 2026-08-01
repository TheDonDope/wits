package importer

import (
	"fmt"
	"sort"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/TheDonDope/wits-tui/pkg/catalog"
	"github.com/TheDonDope/wits-tui/pkg/journal"
	"github.com/TheDonDope/wits-tui/pkg/repo"
)

// Result is everything a run of the importer found. It holds no repository and
// has written nothing.
type Result struct {
	Sheets   []*Sheet
	Products []*catalog.Product
	Events   []journal.Event
	Merged   []Merge
}

// Merge is a product that several spellings in the workbook resolved to.
type Merge struct {
	Slug  string
	Names []string
}

// Anomalies returns every problem found, each prefixed with its sheet.
func (r *Result) Anomalies() []string {
	var all []string
	for _, s := range r.Sheets {
		for _, a := range s.Anomalies {
			all = append(all, fmt.Sprintf("%s: %s", s.Name, a))
		}
	}
	return all
}

// Grams returns the totals the sheets describe: what was dispensed, and what
// their rows account for.
func (r *Result) Grams() (purchased, ground float64) {
	for _, s := range r.Sheets {
		purchased += s.Purchased()
		ground += s.Ground()
	}
	return purchased, ground
}

// Span returns the dates of the first and last entry.
func (r *Result) Span() (first, last time.Time) {
	for _, e := range r.Events {
		if first.IsZero() || e.OccurredAt.Before(first) {
			first = e.OccurredAt
		}
		if e.OccurredAt.After(last) {
			last = e.OccurredAt
		}
	}
	return first, last
}

// Counts returns how many events of each type were produced.
func (r *Result) Counts() map[journal.Type]int {
	counts := map[journal.Type]int{}
	for _, e := range r.Events {
		counts[e.Type]++
	}
	return counts
}

// Read parses the workbook at path. It opens the file read-only and writes
// nothing, anywhere.
func Read(path string) (*Result, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	result := &Result{}
	seen := map[string]bool{}
	// Different spellings that resolve to the same product are worth saying out
	// loud: the slug drops the THC ratio, so the same cultivar from the same
	// manufacturer at two potencies becomes one product, and only the first
	// spelling's figures are kept.
	spellings := map[string]map[string]bool{}

	for _, name := range f.GetSheetList() {
		sheet, err := readSheet(f, name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if sheet == nil {
			continue
		}
		for _, p := range sheet.Products {
			if spellings[p.Slug] == nil {
				spellings[p.Slug] = map[string]bool{}
			}
			spellings[p.Slug][p.Name] = true
			if !seen[p.Slug] {
				seen[p.Slug] = true
				result.Products = append(result.Products, catalog.Parse(p.Name))
			}
		}
		result.Sheets = append(result.Sheets, sheet)
		result.Events = append(result.Events, events(sheet)...)
	}

	// The journal is a chronology, but the worksheets are not in date order:
	// several are named for a month that begins a few days earlier, and one is
	// dated a year early outright. A stable sort keeps a sheet's own fills ahead
	// of its first grind, which share a date.
	sort.SliceStable(result.Events, func(i, j int) bool {
		return result.Events[i].OccurredAt.Before(result.Events[j].OccurredAt)
	})
	result.Merged = merged(spellings)
	return result, nil
}

// merged lists the products that more than one spelling resolved to.
func merged(spellings map[string]map[string]bool) []Merge {
	var out []Merge
	for slug, names := range spellings {
		if len(names) < 2 {
			continue
		}
		m := Merge{Slug: slug}
		for name := range names {
			m.Names = append(m.Names, name)
		}
		sort.Strings(m.Names)
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

// events turns one sheet into the entries it describes. The fill is dated from
// the earliest row, because the sheet records no purchase date of its own.
func events(s *Sheet) []journal.Event {
	out := make([]journal.Event, 0, len(s.Products)+len(s.Entries))
	note := "imported from " + s.Name

	for _, p := range s.Products {
		out = append(out, journal.Event{
			Type:       journal.Purchase,
			Product:    p.Slug,
			Grams:      p.Purchased,
			OccurredAt: s.Opened,
			Note:       note,
		})
	}
	for _, e := range s.Entries {
		out = append(out, journal.Event{
			Type:       journal.Grind,
			Product:    e.Product.Slug,
			Grams:      e.Grams,
			OccurredAt: e.At,
			Note:       note,
		})
	}
	for i := range out {
		from, to, _ := journal.Flow(out[i].Type)
		out[i].From, out[i].To = from, to
	}
	return out
}

// ErrNotEmpty is returned when committing into a repository that already holds
// entries.
var ErrNotEmpty = fmt.Errorf("the journal already holds entries")

// Commit writes a result into a repository.
//
// The repository must be empty. Importing the same workbook twice would double
// every gram in it, and there is no way to tell a re-import from a genuine
// second helping of the same product on the same day.
func Commit(r *repo.Repo, result *Result) error {
	events, err := r.Journal().Events()
	if err != nil {
		return err
	}
	if len(events) > 0 {
		return fmt.Errorf("%w: %d of them; import into an empty repository", ErrNotEmpty, len(events))
	}

	products, err := catalog.Load(r.ProductsPath())
	if err != nil {
		return err
	}
	for _, p := range result.Products {
		if err := products.Add(p); err != nil {
			return err
		}
	}
	if err := products.Save(r.ProductsPath()); err != nil {
		return err
	}

	for i, e := range result.Events {
		if _, err := r.Journal().Append(e); err != nil {
			return fmt.Errorf("entry %d of %d, dated %s: %w",
				i+1, len(result.Events), e.OccurredAt.Format(time.DateOnly), err)
		}
	}
	return nil
}
