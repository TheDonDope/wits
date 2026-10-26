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

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/prices"
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
	case entryReconcile, entryWeighMany:
		return "Weigh and reconcile"
	case entryCleanHistory:
		return "Clean history"
	case entryDescribe:
		return "Edit product"
	case entryPrices:
		return "Price purchases"
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

	// The fields below belong to single forms. They used to be borrowed from
	// the ones above — the account rode in device, the CBD in temp — which
	// worked until anyone had to read commit and say what device meant there.
	account      string // reconcile: which account is on the scale
	forgot       bool   // reconcile: a short reading is a grind or sessions forgotten
	manufacturer string // describe
	cultivar     string // describe
	thc, cbd     string // describe
	terpenes     string // describe

	// target is the entry being corrected, for the amend and undo forms.
	target *journal.Event

	// slugs and readings belong to the weigh-many form: one reading per
	// ticked jar, collected in order. The price form keeps its answers in
	// readings too, one per purchase.
	slugs     []string
	readings  []string
	purchases []journal.Event

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
			huh.NewInput().Title("Amount").Description("Grams ground into the stash").
				Value(&f.amount).Validate(validGrams),
		))
	case entrySesh:
		fields := []huh.Field{
			huh.NewSelect[string]().Title("Product").
				Description("Taken out of the stash").
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

	f.form = a.dress(f.form)
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

// deviceOptions lists the devices a session is had with: the vaporizers and
// the glass still in use, not the grinders, heaters and torches.
func deviceOptions(a *App) []huh.Option[string] {
	if a.data.Devices == nil {
		return nil
	}
	var opts []huh.Option[string]
	for _, d := range a.data.Devices.Devices {
		if d.SessionWith() && !d.Retired {
			opts = append(opts, huh.NewOption(d.Name, d.Slug))
		}
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
	v, err := ledger.ParseGrams(s)
	if err != nil {
		return err
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
// A summary stands in for the event where one line cannot: a weighing session
// records several adjustments, and the notice should say what they added up to.
type entryDoneMsg struct {
	event   journal.Event
	summary string
	err     error
}

// init starts the form.
func (f *entryForm) init() tea.Cmd { return f.form.Init() }

// view draws the form at the terminal's width.
func (f *entryForm) view(a *App) string { return f.View(a, a.width) }

// dismiss is esc; nothing is written until the form completes, so there is
// nothing to put back.
func (f *entryForm) dismiss(*App) {}

// update is Update as a modal: a closed form is a nil modal, not a modal
// holding a nil form.
func (f *entryForm) update(msg tea.Msg, a *App) (modal, tea.Cmd) {
	next, cmd := f.Update(msg, a)
	if next == nil {
		return nil, cmd
	}
	return next, cmd
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
		switch f.kind {
		case entryWeighMany:
			summary, err := f.commitMany(a)
			return nil, func() tea.Msg { return entryDoneMsg{summary: summary, err: err} }
		case entryCleanHistory:
			summary, err := f.commitClean(a)
			return nil, func() tea.Msg { return entryDoneMsg{summary: summary, err: err} }
		case entryPrices:
			summary, err := f.commitPrices(a)
			return nil, func() tea.Msg { return entryDoneMsg{summary: summary, err: err} }
		case entryReconcile:
			if f.forgot && len(f.plan(a, f.product, f.amount)) > 0 {
				summary, err := f.commitForgotten(a)
				return nil, func() tea.Msg { return entryDoneMsg{summary: summary, err: err} }
			}
		}
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
	if f.kind != entryUndo && f.kind != entryReconcile && f.kind != entryDescribe {
		var err error
		if grams, err = parseGrams(f.amount); err != nil {
			return journal.Event{}, err
		}
	}

	switch f.kind {
	case entryDescribe:
		p, err := rec.Describe(f.product, catalog.Product{
			Name:         f.name,
			Manufacturer: f.manufacturer,
			Cultivar:     f.cultivar,
			THC:          parseFloatOrZero(f.thc),
			CBD:          parseFloatOrZero(f.cbd),
			Terpenes:     catalog.ParseTerpenes(f.terpenes),
		})
		if err != nil {
			return journal.Event{}, err
		}
		return journal.Event{Product: p.Slug, Note: p.Name}, nil
	case entryReconcile:
		weighed, err := ledger.ParseGrams(f.amount)
		if err != nil {
			return journal.Event{}, err
		}
		return rec.Reconcile(f.product, journal.Account(f.account), weighed, strings.TrimSpace(f.note))
	case entryUndo:
		if !f.confirm {
			return journal.Event{}, errCancelled
		}
		return rec.Revert(f.target.Hash, strings.TrimSpace(f.note))
	case entryAmend:
		return rec.Amend(f.target.Hash, grams, strings.TrimSpace(f.note))
	case entryBuy:
		// The interface never asks for a slug: a generated handle is the
		// point, and `wits buy --slug` exists for anyone who wants their own.
		e, _, _, err := rec.Buy(strings.TrimSpace(f.name), "", grams, at)
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
	return t.Panel.Width(min(width-2, 74)).Render(panel)
}

// errCancelled reports a form the user answered in the negative, which is not a
// failure and should not be shown as one.
var errCancelled = errors.New("cancelled")

// parseGrams reads an amount written as "0.75", "0.75g" or "0,75 g".
func parseGrams(s string) (float64, error) {
	grams, err := ledger.ParseGrams(s)
	if err != nil {
		return 0, err
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
	f.form = a.dress(huh.NewForm(huh.NewGroup(
		huh.NewNote().Title(describe(e, a)).
			Description("The original stays in the journal. Amending records a\ncorrection alongside it."),
		huh.NewInput().Title("Corrected amount").Description("Grams").
			Value(&f.amount).Validate(validGrams),
		huh.NewInput().Title("Why").Description("Optional").Value(&f.note),
	)))
	return f
}

// newUndoForm confirms undoing an entry.
func newUndoForm(e *journal.Event, a *App) *entryForm {
	f := &entryForm{kind: entryUndo, target: e}
	f.form = a.dress(huh.NewForm(huh.NewGroup(
		huh.NewNote().Title(describe(e, a)).
			Description("Nothing is deleted. Undoing moves the grams back the way\nthey came and records that alongside the original."),
		huh.NewConfirm().Title("Undo this entry?").Affirmative("Undo").Negative("Keep").
			Value(&f.confirm),
		huh.NewInput().Title("Why").Description("Optional").Value(&f.note),
	)))
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
func newReconcileForm(slug string, a *App, account journal.Account) *entryForm {
	f := &entryForm{kind: entryReconcile, product: slug}
	b := a.data.State.Balances[slug]
	if b == nil {
		b = &ledger.Balance{}
	}

	accounts := []huh.Option[string]{
		huh.NewOption(fmt.Sprintf("Storage — ledger says %.2f g", b.Storage), string(journal.Storage)),
		huh.NewOption(fmt.Sprintf("The stash — ledger says %.2f g", b.Stash), string(journal.Stash)),
	}
	if b.AVB > 0 {
		accounts = append(accounts,
			huh.NewOption(fmt.Sprintf("AVB — ledger says %.2f g", b.AVB), string(journal.AVB)))
	}
	f.account = string(account)

	f.form = a.dress(huh.NewForm(huh.NewGroup(
		huh.NewNote().Title(a.data.ProductName(slug)).
			Description("Nothing in the past is edited. The difference is recorded\nas an adjustment, and the account agrees with the jar again."),
		huh.NewSelect[string]().Title("Weigh which").Options(accounts...).Value(&f.account),
		huh.NewInput().Title("On the scale").Description("Grams").
			Value(&f.amount).Validate(nonNegativeGrams),
		huh.NewInput().Title("Why").Description("Optional — spilled, unlogged, misread").Value(&f.note),
	), forgottenGroup(f, func() string {
		if planned := f.plan(a, slug, f.amount); len(planned) > 0 {
			return record.ForgottenWords(planned, a.data.Devices)
		}
		return ""
	})))
	return f
}

// forgottenGroup asks whether a short reading was a grind — or, weighing
// the ground, sessions — never logged, and says what would be recorded. It
// shows only when there is a shortfall to put down to one; yes is the
// default, because grams do not vanish into thin air.
//
// The words are set when the form asks whether to show the question, the
// moment the readings are in, rather than through huh's TitleFunc, which
// answers on a later command and showed a blank question until it did; and
// the group is made as tall as they are, since it was sized when it was
// made, with nothing to say yet.
func forgottenGroup(f *entryForm, words func() string) *huh.Group {
	f.forgot = true
	ask := huh.NewConfirm().Affirmative("Forgotten").Negative("Lost").Value(&f.forgot)
	group := huh.NewGroup(ask)
	return group.WithHideFunc(func() bool {
		said := words()
		if said == "" {
			return true
		}
		title := "Less than the ledger says — was a grind forgotten?"
		if f.account == string(journal.Stash) {
			title = "Less than the ledger says — were sessions forgotten?"
		}
		ask.Title(title).Description("Forgotten records " + said + ". Lost records the grams as gone.")
		group.WithHeight(lipgloss.Height(group.Content()) + 2)
		return false
	})
}

// plan is what a reading of one jar would be put down to, or nothing when
// it is not short — or the account is not drawn on by a grind or a session.
func (f *entryForm) plan(a *App, slug, reading string) []journal.Event {
	weighed, err := ledger.ParseGrams(reading)
	if strings.TrimSpace(reading) == "" || err != nil {
		return nil
	}
	rec := record.New(a.data.Repo, a.data.Products, a.data.Devices, a.data.State)
	planned, err := rec.Forgotten(slug, journal.Account(f.account), weighed)
	if err != nil {
		return nil
	}
	return planned
}

// commitForgotten records what a short reading is put down to, and says so;
// after a grind, it asks for the ground to be weighed next.
func (f *entryForm) commitForgotten(a *App) (string, error) {
	weighed, err := ledger.ParseGrams(f.amount)
	if err != nil {
		return "", err
	}
	rec := record.New(a.data.Repo, a.data.Products, a.data.Devices, a.data.State)
	written, err := rec.RecordForgotten(f.product, journal.Account(f.account), weighed)
	if err != nil {
		return "", err
	}
	said := record.ForgottenWords(written, a.data.Devices)
	if f.account == string(journal.Storage) {
		return "recorded " + said + " — weigh the ground next", nil
	}
	return "recorded " + said, nil
}

// nonNegativeGrams accepts zero, because an empty jar is a real reading.
func nonNegativeGrams(s string) error {
	v, err := ledger.ParseGrams(s)
	if err != nil {
		return err
	}
	if v < 0 {
		return fmt.Errorf("a weight cannot be negative")
	}
	return nil
}

// entryDescribe corrects what a product is called and what is known about it.
const entryDescribe entryKind = iota + 300

// entryWeighMany weighs the jars ticked on the storage screen in one sitting.
const entryWeighMany entryKind = iota + 400

// newWeighManyForm asks which account is on the scale, then for each ticked
// jar's reading in turn, the way `wits reconcile` does. A blank reading skips
// a jar, and nothing is written until every question is answered.
func newWeighManyForm(slugs []string, a *App, account journal.Account) *entryForm {
	f := &entryForm{kind: entryWeighMany, slugs: slugs, readings: make([]string, len(slugs))}
	f.account = string(account)

	groups := []*huh.Group{huh.NewGroup(
		huh.NewNote().Title(fmt.Sprintf("Weighing %s", plural(len(slugs), "jar"))).
			Description("Nothing in the past is edited. Each difference is recorded\nas an adjustment, and the accounts agree with the jars again."),
		huh.NewSelect[string]().Title("Weigh which account").
			Options(
				huh.NewOption("Storage — sealed product", string(journal.Storage)),
				huh.NewOption("The stash — ground product", string(journal.Stash)),
			).
			Value(&f.account),
	)}
	for i, slug := range slugs {
		b := a.data.State.Balances[slug]
		if b == nil {
			b = &ledger.Balance{}
		}
		groups = append(groups, huh.NewGroup(
			huh.NewInput().
				Title(a.data.ProductName(slug)).
				Description(fmt.Sprintf("On the scale, in grams — ledger says storage %.2f g · stash %.2f g.\nBlank to skip.", b.Storage, b.Stash)).
				Value(&f.readings[i]).
				Validate(optionalWeight),
		))
	}
	groups = append(groups, forgottenGroup(f, func() string { return f.plans(a) }))
	f.form = a.dress(huh.NewForm(groups...))
	return f
}

// plans says what every short reading of the weigh-many form would be put
// down to, a line a jar, or nothing when none is short.
func (f *entryForm) plans(a *App) string {
	var lines []string
	for i, slug := range f.slugs {
		if planned := f.plan(a, slug, f.readings[i]); len(planned) > 0 {
			lines = append(lines, "\n· "+a.data.ProductName(slug)+": "+record.ForgottenWords(planned, a.data.Devices))
		}
	}
	return strings.Join(lines, "")
}

// commitMany records the collected readings and sums up what changed. A jar
// that already matches is counted rather than turned into an error, because a
// scale agreeing with the ledger is a good day, not a failure.
func (f *entryForm) commitMany(a *App) (string, error) {
	rec := record.New(a.data.Repo, a.data.Products, a.data.Devices, a.data.State)
	account := journal.Account(f.account)

	adjusted, matched, skipped, forgotten := 0, 0, 0, 0
	for i, slug := range f.slugs {
		reading := strings.TrimSpace(f.readings[i])
		if reading == "" {
			skipped++
			continue
		}
		weighed, err := ledger.ParseGrams(reading)
		if err != nil {
			return "", err
		}
		if f.forgot && len(f.plan(a, slug, reading)) > 0 {
			if _, err := rec.RecordForgotten(slug, account, weighed); err != nil {
				return "", err
			}
			forgotten++
			continue
		}
		_, err = rec.Reconcile(slug, account, weighed, "")
		if errors.Is(err, record.ErrNothingToReconcile) {
			matched++
			continue
		}
		if err != nil {
			return "", err
		}
		adjusted++
	}

	parts := []string{fmt.Sprintf("adjusted %s", plural(adjusted, "jar"))}
	if forgotten > 0 {
		what := "grind"
		if account == journal.Stash {
			what = "sessions"
		}
		parts = append(parts, fmt.Sprintf("%d put down to a forgotten %s", forgotten, what))
	}
	if matched > 0 {
		parts = append(parts, fmt.Sprintf("%d already matched", matched))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", skipped))
	}
	return "weighed " + string(account) + ": " + strings.Join(parts, ", "), nil
}

// optionalWeight accepts a scale reading, zero included, or nothing at all.
func optionalWeight(s string) error {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return nonNegativeGrams(s)
}

// entryCleanHistory zeroes out the stale stash remainders in one confirmed go.
const entryCleanHistory entryKind = iota + 500

// newCleanHistoryForm confirms recording the stale stashes as consumed. The
// imported years never logged sessions, so a stash whose storage is long gone
// reads as if the grams were still in the jar; a zero here says they were
// consumed at some point, which is the only honest reading of an empty jar.
func newCleanHistoryForm(slugs []string, a *App) *entryForm {
	f := &entryForm{kind: entryCleanHistory, slugs: slugs}

	// The confirmation has to stay on screen. A workbook import leaves dozens
	// of stale jars, and listing them all pushed the question below the fold —
	// the form answered enter with its default, Keep, and looked like it did
	// nothing. What does not fit folds into one line; the storage screen is
	// the full listing.
	room := max(3, a.height-20)
	var listed []string
	total, folded, foldedGrams := 0.0, 0, 0.0
	for _, slug := range slugs {
		b := a.data.State.Balances[slug]
		if b == nil {
			continue
		}
		total += b.Stash
		if len(listed) < room {
			listed = append(listed, fmt.Sprintf("%s — %.2f g", a.data.ProductName(slug), b.Stash))
		} else {
			folded++
			foldedGrams += b.Stash
		}
	}
	if folded > 0 {
		listed = append(listed, fmt.Sprintf("… and %d more, %.2f g together", folded, foldedGrams))
	}
	f.form = a.dress(huh.NewForm(huh.NewGroup(
		huh.NewNote().Title(fmt.Sprintf("Clean history — %s, %.2f g", plural(len(slugs), "jar"), total)).
			Description(strings.Join(listed, "\n")+
				"\n\nEach stash is reconciled to zero: the grams were consumed at\nsome point, the sessions were never logged. Nothing is deleted."),
		huh.NewConfirm().Title("Record them as consumed?").Affirmative("Clean").Negative("Keep").
			Value(&f.confirm),
	)))
	return f
}

// commitClean reconciles every stale stash to zero and sums up what left.
func (f *entryForm) commitClean(a *App) (string, error) {
	if !f.confirm {
		return "", errCancelled
	}
	rec := record.New(a.data.Repo, a.data.Products, a.data.Devices, a.data.State)
	cleaned, grams := 0, 0.0
	for _, slug := range f.slugs {
		b := a.data.State.Balances[slug]
		if b == nil || b.Stash <= 0 {
			continue
		}
		was := b.Stash
		if _, err := rec.Reconcile(slug, journal.Stash, 0, "clean history: consumed at some point"); err != nil {
			return "", err
		}
		cleaned++
		grams += was
	}
	return fmt.Sprintf("cleaned %s: %.2f g recorded as consumed", plural(cleaned, "stash"), grams), nil
}

// newDescribeForm edits a product's details.
//
// The slug is shown but not offered for editing. It is the name every entry in
// the journal refers to, and changing it would leave those entries pointing at
// a product that no longer exists. What a jar is called can be corrected; which
// jar it is cannot.
func newDescribeForm(slug string, a *App) *entryForm {
	f := &entryForm{kind: entryDescribe, product: slug}

	p, err := a.data.Products.Find(slug)
	if err != nil {
		p = &catalog.Product{Slug: slug, Name: slug}
	}
	f.name = p.Name
	f.manufacturer = p.Manufacturer
	f.cultivar = p.Cultivar
	f.thc = trimZero(p.THC)
	f.cbd = trimZero(p.CBD)
	f.terpenes = strings.Join(p.Terpenes, ", ")

	// The terpenes are a page of their own: seven fields on one would push
	// the form past a terminal's height, and a form taller than the terminal
	// submits what it holds.
	f.form = a.dress(huh.NewForm(huh.NewGroup(
		huh.NewNote().Title(slug).
			Description("The slug stays as it is: every entry in the journal refers\nto it, and renaming it would orphan them."),
		huh.NewInput().Title("Name").Description("As it should read").
			Value(&f.name).Validate(required("a name")),
		huh.NewInput().Title("Manufacturer").Value(&f.manufacturer),
		huh.NewInput().Title("Cultivar").Value(&f.cultivar),
		huh.NewInput().Title("THC %").Value(&f.thc).Validate(optionalFloat),
		huh.NewInput().Title("CBD %").Value(&f.cbd).Validate(optionalFloat),
	), huh.NewGroup(
		huh.NewInput().Title("Terpenes").
			Description("Dominant first, as the label lists them, e.g. Myrcen, Limonen, Caryophyllen").
			Value(&f.terpenes),
	)))
	return f
}

// trimZero renders a percentage, or nothing when it is unset.
func trimZero(f float64) string {
	if f == 0 {
		return ""
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// optionalFloat accepts a percentage or nothing at all.
func optionalFloat(s string) error {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	if _, err := strconv.ParseFloat(strings.Replace(strings.TrimSpace(s), ",", ".", 1), 64); err != nil {
		return fmt.Errorf("not a percentage")
	}
	return nil
}

// parseFloatOrZero reads a percentage, treating anything unreadable as unset.
func parseFloatOrZero(s string) float64 {
	v, err := strconv.ParseFloat(strings.Replace(strings.TrimSpace(s), ",", ".", 1), 64)
	if err != nil {
		return 0
	}
	return v
}

// entryPrices writes down what purchases cost, one question a purchase.
const entryPrices entryKind = iota + 600

// newPriceForm asks what each purchase was paid, in turn, the way `wits
// reconcile purchases` does: a blank answer skips one, and nothing is written
// until every question is answered.
func newPriceForm(purchases []journal.Event, a *App) *entryForm {
	f := &entryForm{kind: entryPrices, purchases: purchases, readings: make([]string, len(purchases))}
	groups := make([]*huh.Group, 0, len(purchases))
	for i, e := range purchases {
		groups = append(groups, huh.NewGroup(
			huh.NewInput().
				Title(fmt.Sprintf("%s · %.2f g of %s", e.OccurredAt.Format("02 Jan 2006"), e.Grams, a.data.ProductName(e.Product))).
				Description(fmt.Sprintf("%d of %d. What was paid, in euros. Nothing is edited;\nthe price goes beside the journal. Blank to skip.", i+1, len(purchases))).
				Value(&f.readings[i]).
				Validate(optionalEuros),
		))
	}
	f.form = a.dress(huh.NewForm(groups...))
	return f
}

// optionalEuros accepts a blank answer or an amount of euros.
func optionalEuros(s string) error {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	_, err := prices.ParseEuros(s)
	return err
}

// commitPrices writes down the prices answered and sums up what it wrote.
func (f *entryForm) commitPrices(a *App) (string, error) {
	rec := record.New(a.data.Repo, a.data.Products, a.data.Devices, a.data.State)
	var written int
	var cents int64
	for i, e := range f.purchases {
		answer := strings.TrimSpace(f.readings[i])
		if answer == "" {
			continue
		}
		c, err := prices.ParseEuros(answer)
		if err != nil {
			return "", err
		}
		if _, err := rec.Price(e.Hash, c, "", time.Now()); err != nil {
			return "", err
		}
		written++
		cents += c
	}
	return fmt.Sprintf("priced %s, %s in all", plural(written, "purchase"), prices.Format(cents)), nil
}
