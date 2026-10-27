// Package tui holds the AVB form here: the AVB weighed out of a device, shared
// between the products its sessions put in, or AVB used up from the jar, as
// `wits avb collect` and `wits avb use` record them.
package tui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/record"
	"github.com/TheDonDope/wits/pkg/views"
)

// The two things the form records.
const (
	avbCollect = "collect"
	avbUse     = "use"
)

// avbForm records AVB collected out of a device, or used up.
type avbForm struct {
	form   *huh.Form
	mode   string
	amount string
	device string
	note   string
}

// newAVBForm builds the AVB form: what is recorded, how much, the device it
// came out of when there is more than one with sessions waiting, and a note.
func newAVBForm(a *App) *avbForm {
	f := &avbForm{mode: avbCollect}
	held := views.AVBHeld(a.data.State.Balances)
	waiting := avbWaiting(a)
	f.form = a.dress(huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().Title("AVB").
				Description(fmt.Sprintf("The AVB jar holds %.2f g", held.Grams())).
				Options(huh.NewOption("Collected out of a device", avbCollect), huh.NewOption("Used up — butter, a tincture", avbUse)).
				Value(&f.mode),
			huh.NewInput().Title("Grams").Description("As the scale reads it").Value(&f.amount).Validate(validGrams),
		),
		huh.NewGroup(
			huh.NewSelect[string]().Title("Out of").
				Description("The sessions put through it since it was last emptied share it").
				Options(waiting...).Value(&f.device),
		).WithHideFunc(func() bool { return f.mode != avbCollect || len(waiting) < 2 }),
		huh.NewGroup(
			huh.NewInput().Title("Note").Description("Optional").Value(&f.note),
		),
	))
	return f
}

// avbWaiting is each device with sessions put through it since its last
// collection, as options: what the AVB weighed out now could have come from.
func avbWaiting(a *App) []huh.Option[string] {
	pending := ledger.Uncollected(a.data.State.Events, a.data.Now)
	slugs := make([]string, 0, len(pending))
	for slug := range pending {
		slugs = append(slugs, slug)
	}
	slices.Sort(slugs)
	opts := make([]huh.Option[string], 0, len(slugs))
	for _, slug := range slugs {
		var g journal.Centigrams
		for _, e := range pending[slug] {
			g += journal.CG(e.Grams)
		}
		name := "no device named"
		if d, err := a.data.Devices.Find(slug); err == nil && slug != "" {
			name = d.Name
		}
		opts = append(opts, huh.NewOption(fmt.Sprintf("%s — %s, %.2f g", name, plural(len(pending[slug]), "session"), g.Grams()), slug))
	}
	return opts
}

func (f *avbForm) init() tea.Cmd { return f.form.Init() }
func (f *avbForm) dismiss(*App)  {}

// view draws the form in a panel, as the entry forms are drawn.
func (f *avbForm) view(a *App) string {
	t := a.theme
	return t.Panel.Width(min(a.width-2, 74)).Render(lipgloss.JoinVertical(lipgloss.Left,
		t.PanelTitle.Render("AVB"),
		t.Dim.Render("enter to move on · esc to cancel"),
		"",
		f.form.View(),
	))
}

// update advances the form and records once it completes.
func (f *avbForm) update(msg tea.Msg, a *App) (modal, tea.Cmd) {
	model, cmd := f.form.Update(msg)
	if form, ok := model.(*huh.Form); ok {
		f.form = form
	}
	switch f.form.State {
	case huh.StateAborted:
		return nil, nil
	case huh.StateCompleted:
		what, err := f.commit(a)
		return nil, func() tea.Msg { return ratedMsg{what: what, err: err} }
	}
	return f, cmd
}

// commit records through the recorder, which refuses what the jar cannot
// give and a collection with no session behind it.
func (f *avbForm) commit(a *App) (string, error) {
	grams, err := ledger.ParseGrams(f.amount)
	if err != nil {
		return "", err
	}
	note := strings.TrimSpace(f.note)
	rec := record.New(a.data.Repo, a.data.Products, a.data.Devices, a.data.State)
	if f.mode == avbUse {
		if _, err := rec.UseAVB(grams, time.Now(), note); err != nil {
			return "", err
		}
		return fmt.Sprintf("used %.2f g of AVB", grams), nil
	}
	events, err := rec.CollectAVB(f.device, grams, time.Now(), note)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("collected %.2f g of AVB, shared between %s", grams, plural(len(events), "product")), nil
}
