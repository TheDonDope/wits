package tui

import (
	"fmt"
	"os"
	"strconv"
	"time"

	can "github.com/TheDonDope/wits/pkg/cannabis"
	"github.com/TheDonDope/wits/pkg/service"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/google/uuid"
)

// StrainsSubmenu is a list of options for the strains submenu.
var StrainsSubmenu = []string{
	"➕ Add Strain",
	"📋 View Strains",
	"✏️ Edit Strain",
	"❌ Remove Strain"}

// geneticsOptions returns a list of genetic options for the user to choose from.
func geneticsOptions() []huh.Option[can.GeneticType] {
	var genetics []huh.Option[can.GeneticType]
	for k, v := range can.Genetics {
		genetics = append(genetics, huh.NewOption(v, k))
	}
	return genetics
}

// radiationOptions returns a list of options for radiation treatment for the user to choose from.
func radiationOptions() []huh.Option[bool] {
	var radiations []huh.Option[bool]
	radiations = append(radiations, huh.NewOption("yes", true))
	radiations = append(radiations, huh.NewOption("no", false))
	return radiations
}

// terpeneOptions returns a list of terpene options for the user to choose from.
func terpeneOptions() []huh.Option[*can.Terpene] {
	var terpenes []huh.Option[*can.Terpene]
	for _, t := range can.Terpenes {
		terpenes = append(terpenes, huh.NewOption(t.Name, t))
	}
	return terpenes
}

// newStrainForm returns a form for creating a new strain.
func newStrainForm() *huh.Form {
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

			huh.NewInput().
				Key("country").
				Title("Country").
				Description("The country of origin"),

			huh.NewSelect[can.GeneticType]().
				Key("genetic").
				Options(geneticsOptions()...).
				Title("Genetic").
				Description("The phenotype"),

			huh.NewSelect[bool]().
				Key("radiated").
				Options(radiationOptions()...).
				Title("Radiated").
				Description("If the plant was radiation treated"),

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

// newStrainFromForm creates a new strain from the form data.
func newStrainFromForm(form *huh.Form) *can.Strain {
	thc, err := strconv.ParseFloat(form.GetString("thc"), 64)
	if err != nil {
		thc = 0
	}
	cbd, err := strconv.ParseFloat(form.GetString("cbd"), 64)
	if err != nil {
		cbd = 0
	}
	amount, err := strconv.ParseFloat(form.GetString("amount"), 64)
	if err != nil {
		amount = 0
	}
	return &can.Strain{
		ID:           uuid.New(),
		Strain:       form.GetString("strain"),
		Cultivar:     form.GetString("cultivar"),
		Manufacturer: form.GetString("manufacturer"),
		Country:      form.GetString("country"),
		Genetic:      form.Get("genetic").(can.GeneticType),
		Radiated:     form.GetBool("radiated"),
		THC:          thc,
		CBD:          cbd,
		Terpenes:     form.Get("terpenes").([]*can.Terpene),
		Amount:       amount,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// AddStrain opens a form for adding a new strain and returns the created strain object.
func AddStrain(svc service.StrainService) tea.Model {
	form := newStrainForm()

	if err := form.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running strain creation form: %v\n", err)
		os.Exit(1)
	}
	strain := newStrainFromForm(form)
	svc.AddStrain(strain)
	return ListStrains(svc)
}

// StrainsListItem is a list item for strains.
type StrainsListItem struct {
	value *can.Strain
}

// FilterValue returns the filter value for the list item.
func (sli StrainsListItem) FilterValue() string {
	return sli.value.Cultivar
}

// Title returns the title for the list item.
func (sli StrainsListItem) Title() string {
	return sli.value.Strain
}

// Description returns the description for the list item.
func (sli StrainsListItem) Description() string {
	return fmt.Sprintf("Genetic: %s, THC/CBD: %.1f%% %.1f%%", can.Genetics[sli.value.Genetic], sli.value.THC, sli.value.CBD)
}

// StrainsListModel is a model for the strains list.
type StrainsListModel struct {
	list    list.Model
	service service.StrainService
}

// ListStrains creates a new model for the strains list.
func ListStrains(svc service.StrainService) *StrainsListModel {
	items := []list.Item{}
	for _, strain := range svc.GetStrains() {
		items = append(items, StrainsListItem{value: strain})
	}

	l := list.New(items, list.NewDefaultDelegate(), 60, 30)
	l.Title = "🌿 Strains"

	return &StrainsListModel{list: l, service: svc}
}

// Init initializes the strains list model.
func (slm StrainsListModel) Init() tea.Cmd {
	return nil
}

// Update updates the strains list model.
func (slm StrainsListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return slm, tea.Quit
		}
	}

	var cmd tea.Cmd
	slm.list, cmd = slm.list.Update(msg)
	return slm, cmd
}

// View renders the strains list model.
func (slm StrainsListModel) View() string {
	return slm.list.View()
}
