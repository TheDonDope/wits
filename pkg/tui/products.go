package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/journal"
)

// productsView lists what has been dispensed, what is left of each, and how much
// of it has been used.
type productsView struct {
	cursor  int
	allTime bool // show every product ever, not only those still held
}

func newProductsView() productsView { return productsView{} }

type productKeys struct {
	keyMap
	Reconcile, Edit, All key.Binding
}

func (k productKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Reconcile, k.Edit, k.All, k.Help, k.Quit}
}

func (k productKeys) FullHelp() [][]key.Binding {
	return append(k.keyMap.FullHelp(), []key.Binding{k.Reconcile, k.Edit, k.All})
}

func (v productsView) keys(base keyMap) help.KeyMap {
	return productKeys{
		keyMap:    base,
		Reconcile: base.Weigh,
		Edit:      base.Edit,
		All:       withHelp(base.Add, "all/held"),
	}
}

// productRow is a product with everything the screen shows about it.
type productRow struct {
	Product  *catalog.Product
	Slug     string
	Storage  float64
	Stash    float64
	AVB      float64
	Ground   float64
	Bought   float64
	LastSeen time.Time
}

// Held is what is still on the shelf and in the stash.
func (p productRow) Held() float64 { return round(p.Storage + p.Stash) }

func (v productsView) Update(msg tea.Msg, a *App) (productsView, tea.Cmd) {
	rows := v.rows(a)
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(msg, a.keys.Up):
			v.cursor = max(v.cursor-1, 0)
		case key.Matches(msg, a.keys.Down):
			v.cursor = min(v.cursor+1, max(len(rows)-1, 0))
		case key.Matches(msg, a.keys.Top):
			v.cursor = 0
		case key.Matches(msg, a.keys.Bottom):
			v.cursor = max(len(rows)-1, 0)
		case key.Matches(msg, a.keys.Add):
			v.allTime = !v.allTime
			v.cursor = 0
		}
	}
	return v, nil
}

// Selected returns the product under the cursor.
func (v productsView) Selected(a *App) *productRow {
	rows := v.rows(a)
	if v.cursor < 0 || v.cursor >= len(rows) {
		return nil
	}
	return &rows[v.cursor]
}

// rows folds the journal into one row per product.
//
// By default only products still holding something are listed: a catalog that
// remembers four years of prescriptions is a history, not a shelf, and the shelf
// is what a screen called Products should show first.
func (v productsView) rows(a *App) []productRow {
	byProduct := map[string]*productRow{}
	get := func(slug string) *productRow {
		if r, ok := byProduct[slug]; ok {
			return r
		}
		r := &productRow{Slug: slug}
		if a.data.Products != nil {
			if p, err := a.data.Products.Find(slug); err == nil {
				r.Product = p
			}
		}
		byProduct[slug] = r
		return r
	}

	for _, e := range a.data.State.Events {
		r := get(e.Product)
		if e.OccurredAt.After(r.LastSeen) {
			r.LastSeen = e.OccurredAt
		}
		switch e.Type {
		case journal.Purchase:
			r.Bought = round(r.Bought + e.Grams)
		case journal.Grind:
			r.Ground = round(r.Ground + e.Grams)
		}
	}
	for slug, b := range a.data.State.Balances {
		r := get(slug)
		r.Storage, r.Stash, r.AVB = b.Storage, b.Stash, b.AVB
	}

	rows := make([]productRow, 0, len(byProduct))
	for _, r := range byProduct {
		if !v.allTime && r.Held() <= 0 && r.AVB <= 0 {
			continue
		}
		rows = append(rows, *r)
	}
	// Most recently touched first: what was used yesterday is what will be
	// weighed today.
	sort.Slice(rows, func(i, j int) bool {
		if !rows[i].LastSeen.Equal(rows[j].LastSeen) {
			return rows[i].LastSeen.After(rows[j].LastSeen)
		}
		return rows[i].Slug < rows[j].Slug
	})
	return rows
}

func (v productsView) View(a *App, height int) string {
	t := a.theme
	width := a.inner()
	rows := v.rows(a)

	if len(rows) == 0 {
		message := "Nothing on the shelf.\n\nRecord a prescription fill with "
		if v.allTime {
			message = "No products yet.\n\nRecord a prescription fill with "
		}
		return lipgloss.NewStyle().Padding(1, 1).Render(
			t.Subtitle.Render(message) + t.Key.Render("b") + t.Subtitle.Render(" or `wits buy`."))
	}
	if v.cursor >= len(rows) {
		v.cursor = len(rows) - 1
	}

	scope := "on the shelf"
	if v.allTime {
		scope = "every product ever dispensed"
	}
	head := lipgloss.JoinHorizontal(lipgloss.Left,
		t.Label.Width(34).Render("PRODUCT"),
		t.Label.Width(10).Render("THC/CBD"),
		t.Label.Width(10).Align(lipgloss.Right).Render("STORAGE"),
		t.Label.Width(9).Align(lipgloss.Right).Render("STASH"),
		t.Label.Width(9).Align(lipgloss.Right).Render("AVB"),
		t.Label.Width(11).Align(lipgloss.Right).Render("GROUND"),
	)

	lines := []string{t.Dim.Render(fmt.Sprintf("%s · %d", scope, len(rows))), "", head}
	for i, r := range rows {
		lines = append(lines, v.line(a, r, i == v.cursor))
		if i == v.cursor {
			lines = append(lines, v.detail(a, r, width))
		}
	}
	return lipgloss.NewStyle().Padding(0, 1).Render(
		clip(lipgloss.JoinVertical(lipgloss.Left, lines...), height-1))
}

// line renders one product as a row.
func (v productsView) line(a *App, r productRow, selected bool) string {
	t := a.theme
	name := truncate(a.data.ProductName(r.Slug), 32)
	label := t.Value.Width(32).Render(name)
	marker := "  "
	if selected {
		marker = lipgloss.NewStyle().Foreground(t.Accent).Render("│ ")
		label = lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Width(32).Render(name)
	}

	potency := "—"
	if r.Product != nil && r.Product.THC > 0 {
		potency = fmt.Sprintf("%g/%g", r.Product.THC, r.Product.CBD)
	}
	return lipgloss.JoinHorizontal(lipgloss.Left,
		marker, label,
		t.Dim.Width(10).Render(potency),
		grams(t, r.Storage, t.StorageC, 10),
		grams(t, r.Stash, t.StashC, 9),
		grams(t, r.AVB, t.AVBC, 9),
		t.Dim.Width(11).Align(lipgloss.Right).Render(fmt.Sprintf("%.2f g", r.Ground)),
	)
}

// grams renders an amount in an account's colour, dimmed when it is empty so
// that a column of zeroes does not compete with the figures that matter.
func grams(t *Theme, g float64, colour tint, width int) string {
	style := lipgloss.NewStyle().Foreground(colour)
	if g <= 0 {
		style = t.Dim
	}
	return style.Width(width).Align(lipgloss.Right).Render(fmt.Sprintf("%.2f g", g))
}

// detail is the block under the selected product.
func (v productsView) detail(a *App, r productRow, width int) string {
	t := a.theme

	var facts []string
	if r.Product != nil {
		if r.Product.Manufacturer != "" {
			facts = append(facts, r.Product.Manufacturer)
		}
		if r.Product.Cultivar != "" {
			facts = append(facts, r.Product.Cultivar)
		}
	}
	if !r.LastSeen.IsZero() {
		facts = append(facts, "last used "+humanDay(r.LastSeen, a.data.Now))
	}

	// The bar is what is still held against what was dispensed, which is what
	// the label beside it says. Filling it from grams ground instead would
	// disagree with its own caption whenever anything was adjusted or seshed.
	remaining := 0.0
	if r.Bought > 0 {
		remaining = clamp(r.Held()/r.Bought, 0, 1)
	}
	bar := Gauge(max(width-46, 10), remaining, t.Level(remaining), t.Dim)

	return lipgloss.JoinVertical(lipgloss.Left,
		"  "+t.Dim.Render(strings.Join(facts, " · ")),
		lipgloss.JoinHorizontal(lipgloss.Left,
			"  ", bar, " ",
			t.Dim.Render(fmt.Sprintf("%.2f g of %.2f g dispensed still held", r.Held(), r.Bought)),
		),
		"  "+t.Dim.Render("press ")+t.Key.Render("r")+t.Dim.Render(" to weigh it, ")+
			t.Key.Render("e")+t.Dim.Render(" to correct its name"),
		"",
	)
}

// humanDay says today or yesterday where it can, and a date otherwise.
func humanDay(d, now time.Time) string {
	switch daysBetween(d, now) {
	case 0:
		return "today"
	case 1:
		return "yesterday"
	default:
		return d.Format("02 Jan 2006")
	}
}
