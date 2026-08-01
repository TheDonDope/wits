package tui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/record"
)

// entryKind is which entry the form is collecting.
type entryKind int

const (
	entryGrind entryKind = iota
	entrySesh
	entryBuy
)

func (k entryKind) String() string {
	switch k {
	case entrySesh:
		return "Session"
	case entryBuy:
		return "Prescription fill"
	case entryAmend:
		return "Amend entry"
	case entryUndo:
		return "Undo entry"
	case entryReconcile:
		return "Weigh and reconcile"
	default:
		return "Grind"
	}
}

// entryForm collects one entry.
//
// The form is a model driven by Update, not a call to Run. Running it would
// block the event loop and start a second program inside the one already
// holding the terminal, which is how the previous interface did it.
type entryForm struct {
	kind entryKind
	form *huh.Form

	product string
	amount  string
	device  string
	temp    string
	note    string
	name    string
	confirm bool

	// target is the entry being corrected, for the amend and undo forms.
	target *journal.Event

	err error
}

// newEntryForm builds the form for a kind of entry against the current data.
func newEntryForm(kind entryKind, a *App) *entryForm {
	f := &entryForm{kind: kind}

	switch kind {
	case entryBuy:
		f.form = huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Product").
				Description("As written on the label, e.g. Enua 22/1 Wedding Cake").
				Value(&f.name).
				Validate(required("a product name")),
			huh.NewInput().Title("Amount").Description("Grams dispensed").
				Value(&f.amount).Validate(validGrams),
		))
	case entryGrind:
		f.form = huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().Title("Product").
				Description("Taken out of storage").
				Options(productOptions(a, journal.Storage)...).
				Value(&f.product),
			huh.NewInput().Title("Amount").Description("Grams ground into the tin").
				Value(&f.amount).Validate(validGrams),
		))
	case entrySesh:
		fields := []huh.Field{
			huh.NewSelect[string]().Title("Product").
				Description("Taken out of the tin").
				Options(productOptions(a, journal.Stash)...).
				Value(&f.product),
			huh.NewInput().Title("Amount").Description("Grams through the device").
				Value(&f.amount).Validate(validGrams),
		}
		if opts := deviceOptions(a); len(opts) > 0 {
			fields = append(fields,
				huh.NewSelect[string]().Title("Device").Options(opts...).Value(&f.device),
				huh.NewInput().Title("Temperature").Description("°C, blank for the device default").
					Value(&f.temp).Validate(optionalInt),
			)
		}
		fields = append(fields, huh.NewInput().Title("Note").Description("Optional").Value(&f.note))
		f.form = huh.NewForm(huh.NewGroup(fields...))
	}

	f.form = f.form.WithShowHelp(true).WithWidth(minInt(a.inner(), 72))
	return f
}

// productOptions lists the products that actually have something in the given
// account, with the amount alongside, so the form cannot offer a choice that is
// bound to be refused.
func productOptions(a *App, account journal.Account) []huh.Option[string] {
	var opts []huh.Option[string]
	for _, slug := range a.data.State.Products() {
		b := a.data.State.Balances[slug]
		if b == nil {
			continue
		}
		var have float64
		switch account {
		case journal.Storage:
			have = b.Storage
		case journal.Stash:
			have = b.Stash
		}
		if have <= 0 {
			continue
		}
		label := fmt.Sprintf("%s  (%.2f g)", truncate(a.data.ProductName(slug), 40), have)
		opts = append(opts, huh.NewOption(label, slug))
	}
	return opts
}

// deviceOptions lists the known devices.
func deviceOptions(a *App) []huh.Option[string] {
	if a.data.Devices == nil {
		return nil
	}
	var opts []huh.Option[string]
	for _, d := range a.data.Devices.Devices {
		opts = append(opts, huh.NewOption(d.Name, d.Slug))
	}
	return opts
}

// required rejects an empty value.
func required(what string) func(string) error {
	return func(s string) error {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("needs %s", what)
		}
		return nil
	}
}

// validGrams accepts a positive amount, in the shapes a scale reads out.
func validGrams(s string) error {
	v, err := strconv.ParseFloat(strings.Replace(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "g")), ",", ".", 1), 64)
	if err != nil {
		return fmt.Errorf("not an amount in grams")
	}
	if v <= 0 {
		return fmt.Errorf("must be more than zero")
	}
	return nil
}

// optionalInt accepts a whole number or nothing at all.
func optionalInt(s string) error {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	if _, err := strconv.Atoi(strings.TrimSpace(s)); err != nil {
		return fmt.Errorf("not a whole number")
	}
	return nil
}

// entryDoneMsg is sent when an entry has been written, so the app can reload.
type entryDoneMsg struct {
	event journal.Event
	err   error
}

// Update advances the form and, once it is complete, writes the entry.
func (f *entryForm) Update(msg tea.Msg, a *App) (*entryForm, tea.Cmd) {
	model, cmd := f.form.Update(msg)
	if form, ok := model.(*huh.Form); ok {
		f.form = form
	}

	switch f.form.State {
	case huh.StateAborted:
		return nil, nil
	case huh.StateCompleted:
		e, err := f.commit(a)
		return nil, func() tea.Msg { return entryDoneMsg{event: e, err: err} }
	}
	return f, cmd
}

// commit writes the collected entry through the recorder, so the interface
// applies the same checks the commands do.
func (f *entryForm) commit(a *App) (journal.Event, error) {
	rec := record.New(a.data.Repo, a.data.Products, a.data.Devices, a.data.State)
	at := time.Now()

	var grams float64
	if f.kind != entryUndo && f.kind != entryReconcile {
		var err error
		if grams, err = parseGrams(f.amount); err != nil {
			return journal.Event{}, err
		}
	}

	switch f.kind {
	case entryReconcile:
		weighed, err := strconv.ParseFloat(
			strings.Replace(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(f.amount), "g")), ",", ".", 1), 64)
		if err != nil {
			return journal.Event{}, err
		}
		return rec.Reconcile(f.product, journal.Account(f.device), weighed, strings.TrimSpace(f.note))
	case entryUndo:
		if !f.confirm {
			return journal.Event{}, errCancelled
		}
		return rec.Revert(f.target.Hash, strings.TrimSpace(f.note))
	case entryAmend:
		return rec.Amend(f.target.Hash, grams, strings.TrimSpace(f.note))
	case entryBuy:
		e, _, _, err := rec.Buy(strings.TrimSpace(f.name), grams, at)
		return e, err
	case entryGrind:
		return rec.Grind(f.product, grams, at)
	default:
		temp, _ := strconv.Atoi(strings.TrimSpace(f.temp))
		return rec.Session(f.product, grams, at, f.device, temp, strings.TrimSpace(f.note))
	}
}

// View renders the form inside a titled panel.
func (f *entryForm) View(a *App, width int) string {
	t := a.theme
	body := f.form.View()
	if f.err != nil {
		body = lipgloss.JoinVertical(lipgloss.Left, body, "", t.Negative.Render("✗ "+f.err.Error()))
	}
	panel := lipgloss.JoinVertical(lipgloss.Left,
		t.PanelTitle.Render(f.kind.String()),
		t.Dim.Render("enter to move on · esc to cancel"),
		"",
		body,
	)
	return t.Panel.Width(minInt(width-2, 74)).Render(panel)
}

// errCancelled reports a form the user answered in the negative, which is not a
// failure and should not be shown as one.
var errCancelled = errors.New("cancelled")

// parseGrams reads an amount written as "0.75", "0.75g" or "0,75 g".
func parseGrams(s string) (float64, error) {
	trimmed := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.ToLower(s)), "g"))
	grams, err := strconv.ParseFloat(strings.Replace(trimmed, ",", ".", 1), 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not an amount in grams", s)
	}
	if grams <= 0 {
		return 0, fmt.Errorf("amount must be positive, got %v", grams)
	}
	return grams, nil
}

// entryAmend and entryUndo correct an entry that is already in the journal.
const (
	entryAmend entryKind = iota + 100
	entryUndo
)

// newAmendForm asks for the amount an entry should have had.
func newAmendForm(e *journal.Event, a *App) *entryForm {
	f := &entryForm{kind: entryAmend, target: e, amount: fmt.Sprintf("%.2f", e.Grams)}
	f.form = huh.NewForm(huh.NewGroup(
		huh.NewNote().Title(describe(e, a)).
			Description("The original stays in the journal. Amending records a\ncorrection alongside it."),
		huh.NewInput().Title("Corrected amount").Description("Grams").
			Value(&f.amount).Validate(validGrams),
		huh.NewInput().Title("Why").Description("Optional").Value(&f.note),
	)).WithShowHelp(true).WithWidth(minInt(a.inner(), 72))
	return f
}

// newUndoForm confirms undoing an entry.
func newUndoForm(e *journal.Event, a *App) *entryForm {
	f := &entryForm{kind: entryUndo, target: e}
	f.form = huh.NewForm(huh.NewGroup(
		huh.NewNote().Title(describe(e, a)).
			Description("Nothing is deleted. Undoing moves the grams back the way\nthey came and records that alongside the original."),
		huh.NewConfirm().Title("Undo this entry?").Affirmative("Undo").Negative("Keep").
			Value(&f.confirm),
		huh.NewInput().Title("Why").Description("Optional").Value(&f.note),
	)).WithShowHelp(true).WithWidth(minInt(a.inner(), 72))
	return f
}

// describe summarises the entry being corrected.
func describe(e *journal.Event, a *App) string {
	return fmt.Sprintf("%s  %.2f g  %s  ·  %s",
		e.Type, e.Grams, a.data.ProductName(e.Product), e.OccurredAt.Format("Mon 02 Jan 2006"))
}

// entryReconcile makes an account agree with the scale.
const entryReconcile entryKind = iota + 200

// newReconcileForm asks what an account actually weighs.
//
// The ledger's own figure is put in front of the field rather than left to be
// remembered, because the point of weighing is to compare, and a number typed
// from memory is not a reconciliation.
func newReconcileForm(slug string, a *App) *entryForm {
	f := &entryForm{kind: entryReconcile, product: slug}
	b := a.data.State.Balances[slug]
	if b == nil {
		b = &ledger.Balance{}
	}

	accounts := []huh.Option[string]{
		huh.NewOption(fmt.Sprintf("Storage — ledger says %.2f g", b.Storage), string(journal.Storage)),
		huh.NewOption(fmt.Sprintf("The tin — ledger says %.2f g", b.Stash), string(journal.Stash)),
	}
	if b.AVB > 0 {
		accounts = append(accounts,
			huh.NewOption(fmt.Sprintf("AVB — ledger says %.2f g", b.AVB), string(journal.AVB)))
	}
	f.device = string(journal.Storage) // reused as the account, defaulting to storage

	f.form = huh.NewForm(huh.NewGroup(
		huh.NewNote().Title(a.data.ProductName(slug)).
			Description("Nothing in the past is edited. The difference is recorded\nas an adjustment, and the account agrees with the jar again."),
		huh.NewSelect[string]().Title("Weigh which").Options(accounts...).Value(&f.device),
		huh.NewInput().Title("On the scale").Description("Grams").
			Value(&f.amount).Validate(nonNegativeGrams),
		huh.NewInput().Title("Why").Description("Optional — spilled, unlogged, misread").Value(&f.note),
	)).WithShowHelp(true).WithWidth(minInt(a.inner(), 72))
	return f
}

// nonNegativeGrams accepts zero, because an empty jar is a real reading.
func nonNegativeGrams(s string) error {
	v, err := strconv.ParseFloat(strings.Replace(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "g")), ",", ".", 1), 64)
	if err != nil {
		return fmt.Errorf("not a weight in grams")
	}
	if v < 0 {
		return fmt.Errorf("a weight cannot be negative")
	}
	return nil
}
