// Package main is the entry point for the Wits TUI application.
package main

import (
	"fmt"
	"os"

	"github.com/TheDonDope/wits/pkg/service"
	"github.com/TheDonDope/wits/pkg/storage"
	"github.com/TheDonDope/wits/pkg/tui"
	tea "github.com/charmbracelet/bubbletea"
)

var strainStore *storage.StrainStore
var strainService *service.StrainService

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

// Init is the first function that will be called. It returns an optional
// initial command. To not perform an initial command return nil.
func (m model) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

// Update is called when a message is received. Use it to inspect messages
// and, in response, update the model and/or send a command.
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

// View renders the program's UI, which is just a string. The view is
// rendered after every Update.
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
		s += onSubmenuSelected(m)
	}
	s += "\nPress ctrl+c or q to quit."
	if m.menu != "main" {
		s += "\nPress esc to return to main menu."
	}
	return s
}

// onMenuSelected returns a model for the selected menu.
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
		switch m.cursor {
		case 0:
			return onStrainCreated(), nil
		case 1:
			return onStrainListed(), nil
		}
	}
	return m, nil
}

// onSubmenuSelected renders the selected submenu and its items.
func onSubmenuSelected(m model) string {
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

// onStrainCreated returns a model for creating a strain.
func onStrainCreated() tea.Model {
	form := tui.NewStrainForm()

	if err := form.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running strain creation form: %v\n", err)
		os.Exit(1)
	}
	s := tui.NewStrainFromForm(form)
	strainService.AddStrain(s)
	fmt.Printf("Strain model added: %v\n", s)
	return initialModel()
}

// onStrainListed returns a model for listing strains.
func onStrainListed() tea.Model {
	return tui.NewStrainsListModel(strainService)
}

func main() {
	strainStore = storage.NewStrainStore()
	strainService = service.NewStrainService(strainStore)
	_, err := tea.NewProgram(initialModel()).Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting program: %v", err)
		os.Exit(1)
	}
}
