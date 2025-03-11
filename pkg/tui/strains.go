package tui

import (
	"fmt"
	"time"

	can "github.com/TheDonDope/wits/pkg/cannabis"
	"github.com/TheDonDope/wits/pkg/service"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/google/uuid"
)

// geneticsOptions returns a list of genetic options for the user to choose from.
func geneticsOptions() []huh.Option[can.GeneticType] {
	var genetics []huh.Option[can.GeneticType]
	for k, v := range can.Genetics {
		genetics = append(genetics, huh.NewOption(v, k))
	}
	return genetics
}

// terpeneOptions returns a list of terpene options for the user to choose from.
func terpeneOptions() []huh.Option[*can.Terpene] {
	var terpenes []huh.Option[*can.Terpene]
	for _, t := range can.Terpenes {
		terpenes = append(terpenes, huh.NewOption(t.Name, t))
	}
	return terpenes
}

// NewStrainForm returns a form for creating a new strain.
func NewStrainForm() *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("strain").
				Title("Strain").
				Description("The product name"),

			huh.NewInput().
				Key("cultivar").
				Title("Cultivar").
				Description("The plant name"),

			huh.NewInput().
				Key("manufacturer").
				Title("Manufacturer").
				Description("The producing company"),

			huh.NewSelect[can.GeneticType]().
				Key("genetic").
				Options(geneticsOptions()...).
				Title("Genetic").
				Description("The phenotype"),

			huh.NewInput().
				Key("thc").
				Title("THC (%)").
				Description("The THC content"),

			huh.NewInput().
				Key("cbd").
				Title("CBD (%)").
				Description("The CBD content"),

			huh.NewMultiSelect[*can.Terpene]().
				Key("terpenes").
				Options(terpeneOptions()...).
				Title("Terpenes").
				Description("The contained terpenes"),

			huh.NewInput().
				Key("amount").
				Title("Amount (g)").
				Description("The weight"),
		),
	)
}

// NewStrainFromForm creates a new strain from the form data.
func NewStrainFromForm(form *huh.Form) *can.Strain {
	return &can.Strain{
		ID:           uuid.New(),
		Strain:       form.GetString("strain"),
		Cultivar:     form.GetString("cultivar"),
		Manufacturer: form.GetString("manufacturer"),
		Genetic:      form.Get("genetic").(can.GeneticType),
		THC:          form.Get("thc").(float64),
		CBD:          form.Get("cbd").(float64),
		Terpenes:     form.Get("terpenes").([]*can.Terpene),
		Amount:       form.Get("amount").(float64),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// StrainsListItem is a list item for strains.
type StrainsListItem struct {
	value *can.Strain
}

// FilterValue returns the filter value for the list item.
func (i StrainsListItem) FilterValue() string {
	return i.value.Cultivar
}

// Title returns the title for the list item.
func (i StrainsListItem) Title() string {
	return i.value.Strain
}

// Description returns the description for the list item.
func (i StrainsListItem) Description() string {
	return fmt.Sprintf("Genetic: %s, THC/CBD: %.1f%% %.1f%%", can.Genetics[i.value.Genetic], i.value.THC, i.value.CBD)
}

// StrainsListModel is a model for the strains list.
type StrainsListModel struct {
	list    list.Model
	service *service.StrainService
}

// NewStrainsListModel creates a new model for the strains list.
func NewStrainsListModel(s *service.StrainService) *StrainsListModel {
	items := []list.Item{}
	for _, strain := range s.GetStrains() {
		items = append(items, StrainsListItem{value: strain})
	}

	l := list.New(items, list.NewDefaultDelegate(), 60, 30)
	l.Title = "🌿 Strains"

	return &StrainsListModel{list: l, service: s}
}

// Init initializes the strains list model.
func (m StrainsListModel) Init() tea.Cmd {
	return nil
}

// Update updates the strains list model.
func (m StrainsListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View renders the strains list model.
func (m StrainsListModel) View() string {
	return m.list.View()
}
