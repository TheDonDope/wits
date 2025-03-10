// Package main is the entry point for the Wits TUI application.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	huh "github.com/charmbracelet/huh"
)

type model struct {
	cursor  int
	choices []string
	menu    string
}

func initialModel() model {
	return model{
		choices: []string{
			"🌱 Strains",
			"🚀 Devices",
			"🔧 Settings",
			"📊 Stats",
		},
		menu: "main",
	}
}

func (m model) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter":
			return onMenuSelected(m)
		case "esc":
			return initialModel(), nil
		}
	}
	return m, nil
}

func (m model) View() string {
	s := "🥦 Welcome to Wits!\n\n"
	if m.menu == "main" {
		for i, choice := range m.choices {
			cursor := " "
			if m.cursor == i {
				cursor = "➡ "
			}
			s += fmt.Sprintf("%s(%d): %s\n", cursor, i+1, choice)
		}
	} else {
		s += renderSubmenu(m)
	}
	s += "\nPress ctrl+c or q to quit."
	if m.menu != "main" {
		s += "\nPress esc to return to main menu."
	}
	return s
}

func onMenuSelected(m model) (tea.Model, tea.Cmd) {
	switch m.menu {
	case "main":
		switch m.cursor {
		case 0:
			return model{
				choices: []string{
					"➕ Add Strain",
					"📋 View Strains",
					"✏️ Edit Strain",
					"❌ Remove Strain"},
				menu: "strains"}, nil
		case 1:
			return model{
				choices: []string{
					"➕ Register Device",
					"📋 View Devices",
					"✏️ Edit Device",
					"❌ Remove Device"},
				menu: "devices"}, nil
		case 2:
			return model{
				choices: []string{
					"🎨 Appearance",
					"⌨️ Keybindings",
					"🌍 Localization",
					"💾 Backup & Restore"},
				menu: "settings"}, nil
		case 3:
			return model{
				choices: []string{
					"📅 Usage History",
					"📈 Trends",
					"🔢 Dosage Tracker"},
				menu: "stats"}, nil
		}
	case "strains":
		if m.cursor == 0 {
			return onStrainCreated(), nil
		}
	}
	return m, nil
}

func renderSubmenu(m model) string {
	s := fmt.Sprintf("%s Menu:\n", m.menu)
	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = "➡ "
		}
		s += fmt.Sprintf("%s(%d): %s\n", cursor, i+1, choice)
	}
	return s
}

func onStrainCreated() tea.Model {
	var strain, cultivar, manufacturer, genetic, thc, cbd, amount string
	var terpenes []string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Strain").
				Value(&strain),
			huh.NewInput().
				Title("Cultivar").
				Value(&cultivar),
			huh.NewInput().
				Title("Manufacturer").
				Value(&manufacturer),
			huh.NewSelect[string]().
				Title("Genetic").
				Options(
					huh.NewOption("Sativa", "sativa"),
					huh.NewOption("Indica", "indica"),
					huh.NewOption("Sativa-hybrid", "sativa-hybrid"),
					huh.NewOption("Indica-hybrid", "indica-hybrid"),
					huh.NewOption("Hybrid", "hybrid")).
				Value(&genetic),
			huh.NewInput().
				Title("THC (%)").
				Value(&thc),
			huh.NewInput().Title("CBD (%)").Value(&cbd),
			huh.NewMultiSelect[string]().
				Title("Terpenes").
				Options(
					huh.NewOption("Carophyllen", "carophyllen"),
					huh.NewOption("Humulen", "humulen"),
					huh.NewOption("Myrcen", "myrcen"),
					huh.NewOption("Limonen", "limonen"),
					huh.NewOption("Linalool", "linalool"),
					huh.NewOption("Ocimen", "ocimen"),
					huh.NewOption("Alpha-Pinen", "alpha-pinen"),
					huh.NewOption("Garniol", "garniol"),
					huh.NewOption("Terpinolen", "terpinolen"),
					huh.NewOption("Farnesene", "farnesene"),
				).
				Value(&terpenes),
			huh.NewInput().
				Title("Amount (grams)").
				Value(&amount),
		),
	)

	if err := form.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running form: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Strain added: %s, Cultivar: %s, Manufacturer: %s, Genetic: %s, THC: %s%%, CBD: %s%%, Terpenes: %v, Amount: %sg\n", strain, cultivar, manufacturer, genetic, thc, cbd, terpenes, amount)
	return initialModel()
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting program: %v", err)
		os.Exit(1)
	}
}
