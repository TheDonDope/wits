package tui

import (
	"time"

	can "github.com/TheDonDope/wits/pkg/cannabis"
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
