// Package tui holds the strain's rating form here: what a strain is thought of
// as a whole — overall, taste and aroma, whether it would be had again, and a
// line — apart from any one session, as `wits describe --rating` writes it.
package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/record"
)

// strainScores are the dimensions a strain is scored on, in the order shown.
var strainScores = []string{"overall", "taste", "aroma"}

// againChoices are "would you have it again?", stepped through with ← and →.
var againChoices = []catalog.Again{"", catalog.AgainYes, catalog.AgainNo}

// strainForm rates a strain as a whole. It opens on the strain's standing
// rating, so rating again starts from what was said; the newer rating stands.
// Drawn by hand, a line a question, as the session's rating form is, so the
// whole form and its enter stay on a short terminal.
type strainForm struct {
	product *catalog.Product
	scores  []int // one per strainScores, 0 for not given
	again   int   // an index into againChoices
	row     int   // the line under the cursor: the scores, again, then the note
	note    textinput.Model
}

// newStrainForm builds the strain's rating form for a product.
func newStrainForm(slug string, a *App) *strainForm {
	p, err := a.data.Products.Find(slug)
	if err != nil {
		p = &catalog.Product{Slug: slug, Name: slug}
	}
	f := &strainForm{product: p, scores: make([]int, len(strainScores)), note: textinput.New()}
	f.note.Placeholder = "a line about it — optional"
	f.note.Prompt = ""
	f.note.SetWidth(min(a.width-24, 50))
	if r := p.Rating; r != nil {
		f.scores = []int{r.Overall, r.Taste, r.Aroma}
		for i, c := range againChoices {
			if c == r.Again {
				f.again = i
			}
		}
		f.note.SetValue(r.Note)
	}
	return f
}

func (f *strainForm) init() tea.Cmd { return nil }
func (f *strainForm) dismiss(*App)  {}

func (f *strainForm) onAgain() bool { return f.row == len(f.scores) }
func (f *strainForm) onNote() bool  { return f.row == len(f.scores)+1 }

// update moves between the questions, answers them, and rates on enter.
func (f *strainForm) update(msg tea.Msg, a *App) (modal, tea.Cmd) {
	press, ok := msg.(tea.KeyPressMsg)
	if !ok || f.onNote() && !navigates(press.String()) {
		var cmd tea.Cmd
		f.note, cmd = f.note.Update(msg)
		return f, cmd
	}
	switch k := press.String(); {
	case k == "enter":
		what, err := f.commit(a)
		return nil, func() tea.Msg { return ratedMsg{what: what, err: err} }
	case k == "up" || k == "shift+tab" || k == "k":
		return f, f.move(-1)
	case k == "down" || k == "tab" || k == "j":
		return f, f.move(1)
	}
	f.answer(press.String())
	return f, nil
}

// navigates reports whether a key leaves the note rather than being typed in it.
func navigates(k string) bool {
	return k == "enter" || k == "up" || k == "down" || k == "tab" || k == "shift+tab"
}

// answer changes the line under the cursor for a key: a score a step either
// way or a digit straight to it, below 1 not given; again stepped round.
func (f *strainForm) answer(k string) {
	step := map[string]int{"right": 1, "l": 1, "left": -1, "h": -1}[k]
	if f.onAgain() {
		f.again = (f.again + step + len(againChoices)) % len(againChoices)
		return
	}
	switch {
	case step != 0:
		f.scores[f.row] = min(max(f.scores[f.row]+step, 0), effects.Highest)
	case len(k) == 1 && k[0] >= '0' && k[0] <= '5':
		f.scores[f.row] = int(k[0] - '0')
	}
}

// move steps the cursor, focusing the note when it lands there.
func (f *strainForm) move(by int) tea.Cmd {
	f.row = min(max(f.row+by, 0), len(f.scores)+1)
	if f.onNote() {
		return f.note.Focus()
	}
	f.note.Blur()
	return nil
}

// rating is what the form says, or nil when it says nothing at all: a rating
// cleared of every answer is taken away, as --unrate takes it.
func (f *strainForm) rating() *catalog.StrainRating {
	r := catalog.StrainRating{Overall: f.scores[0], Taste: f.scores[1], Aroma: f.scores[2],
		Again: againChoices[f.again], Note: strings.TrimSpace(f.note.Value())}
	if r.Overall == 0 && r.Taste == 0 && r.Aroma == 0 && r.Again == "" && r.Note == "" {
		return nil
	}
	return &r
}

// commit rates the strain through the recorder, or takes its rating away.
func (f *strainForm) commit(a *App) (string, error) {
	r := f.rating()
	if r == nil && f.product.Rating == nil {
		return "", fmt.Errorf("nothing rated — give it a score, say whether again, or write a line")
	}
	rec := record.New(a.data.Repo, a.data.Products, a.data.Devices, a.data.State)
	p, err := rec.RateStrain(f.product.Slug, r, time.Now())
	if err != nil {
		return "", err
	}
	if p.Rating == nil {
		return fmt.Sprintf("took the rating of %s away", p.Name), nil
	}
	return fmt.Sprintf("rated %s: %s", p.Name, p.Rating.Summary()), nil
}

// view draws a line a question, the cursor's marked, then the note.
func (f *strainForm) view(a *App) string {
	t := a.theme
	cursor := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	mark := func(i int, name string) string {
		if i == f.row {
			return cursor.Render("› ") + cursor.Width(10).Render(name)
		}
		return "  " + t.Label.Width(10).Render(name)
	}
	rows := []string{
		t.PanelTitle.Render("Rate the strain"),
		t.Dim.Render("↑/↓ choose · ←/→ or 1–5 score · 0 clears · enter rates · esc cancels"),
		"",
		t.Value.Render(f.product.Name),
		t.Dim.Render("as a whole, apart from any one session"),
		"",
	}
	for i, name := range strainScores {
		rows = append(rows, mark(i, name)+f.dots(a, f.scores[i]))
	}
	again := map[catalog.Again]string{"": "—", catalog.AgainYes: "yes", catalog.AgainNo: "no"}[againChoices[f.again]]
	rows = append(rows, mark(len(f.scores), "again")+t.Value.Render(again), "", mark(len(f.scores)+1, "note")+f.note.View())
	return t.Panel.Width(min(a.width-2, 80)).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// dots draws a score as filled and empty dots, and the figure.
func (f *strainForm) dots(a *App, v int) string {
	t := a.theme
	on := lipgloss.NewStyle().Foreground(t.SeshC)
	score := t.Dim.Render("—")
	if v > 0 {
		score = on.Render(strconv.Itoa(v))
	}
	return on.Render(strings.Repeat("● ", v)) + t.Dim.Render(strings.Repeat("○ ", effects.Highest-v)) + " " + score
}
