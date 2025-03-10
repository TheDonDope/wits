// Package main is the entry point for the Wits TUI application.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
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
			"🔮 Devices",
			"🔧 Settings",
			"📊 Stats",
		},
		menu: "main",
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
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
			return handleSelection(m)
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
		s += submenuView(m)
	}
	s += "\nPress q to quit."
	if m.menu != "main" {
		s += "\nPress esc to return to main menu."
	}
	return s
}

func handleSelection(m model) (tea.Model, tea.Cmd) {
	switch m.menu {
	case "main":
		switch m.cursor {
		case 0:
			return model{choices: []string{"➕ Add Strain", "🔍 View Strains", "✏️ Edit Strain", "❌ Remove Strain"}, menu: "strains"}, nil
		case 1:
			return model{choices: []string{"➕ Add Device", "📋 View Devices", "🛠 Manage Device", "❌ Remove Device"}, menu: "devices"}, nil
		case 2:
			return model{choices: []string{"🎨 Appearance", "⌨️ Keybindings", "🌍 Localization", "💾 Backup & Restore"}, menu: "settings"}, nil
		case 3:
			return model{choices: []string{"📅 Usage History", "📈 Trends", "🔢 Dosage Tracker"}, menu: "stats"}, nil
		}
	}
	return m, nil
}

func submenuView(m model) string {
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

func main() {
	p := tea.NewProgram(initialModel())
	if err := p.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting program: %v", err)
		os.Exit(1)
	}
}
