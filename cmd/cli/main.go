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
			m.cursor--
			if m.cursor < 0 {
				m.cursor = len(m.menu) - 1 // Wrap to last item
			}
		case "down", "j":
			m.cursor++
			if m.cursor >= len(m.menu) {
				m.cursor = 0 // Wrap to first item
			}
		case "1", "2", "3", "4":
			idx := int(msg.String()[0] - '1') // Convert key to index
			if idx < len(m.menu) {
				m.cursor = idx // Jump to selected menu item
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
				choices: tui.StrainsSubmenu,
				menu:    "strains"}, nil
		case 1:
			return model{
				choices: tui.DevicesSubmenu,
				menu:    "devices"}, nil
		case 2:
			return model{
				choices: tui.SettingsSubmenu,
				menu:    "settings"}, nil
		case 3:
			return model{
				choices: tui.StatsSubmenu,
				menu:    "stats"}, nil
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
	return tui.AddStrain(strainService)
}

// onStrainListed returns a model for listing strains.
func onStrainListed() tea.Model {
	return tui.ListStrains(strainService)
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
