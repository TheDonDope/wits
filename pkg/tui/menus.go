package tui

import (
	"fmt"

	"github.com/TheDonDope/wits/pkg/service"
	"github.com/TheDonDope/wits/pkg/storage"
	tea "github.com/charmbracelet/bubbletea"
)

var (
	strainStore   storage.StrainStore
	strainService service.StrainService
)

// MainMenu is a list of options for the main menu.
var MainMenu = []string{
	"🌿 Strains",
	"🚀 Devices",
	"🔧 Settings",
	"📊 Stats"}

// MenuModel is the model for the main menu.
type MenuModel struct {
	Cursor int
	Items  []string
	Menu   string
}

// InitialMenuModel returns the initial model for the main menu.
func InitialMenuModel(strainSrv service.StrainService, strainStr storage.StrainStore) MenuModel {
	strainStore = strainStr
	strainService = strainSrv
	return MenuModel{
		Items: MainMenu,
		Menu:  "main",
	}
}

// Init is the first function that will be called. It returns an optional
// initial command. To not perform an initial command return nil.
func (m MenuModel) Init() tea.Cmd {
	// return `nil`, which means "no I/O right now, please."
	return nil
}

// Update is called when a message is received. Use it to inspect messages
// and, in response, update the model and/or send a command.
func (m MenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			m.Cursor--
			if m.Cursor < 0 {
				m.Cursor = len(m.Menu) - 1 // Wrap to last item
			}
		case "down", "j":
			m.Cursor++
			if m.Cursor >= len(m.Menu) {
				m.Cursor = 0 // Wrap to first item
			}
		case "1", "2", "3", "4":
			idx := int(msg.String()[0] - '1') // Convert key to index
			if idx < len(m.Menu) {
				m.Cursor = idx // Jump to selected menu item
			}
		case "enter":
			return onMenuSelected(m)
		case "esc":
			return InitialMenuModel(strainService, strainStore), nil
		}
	}
	return m, nil
}

// View renders the program's UI, which is just a string. The view is
// rendered after every Update.
func (m MenuModel) View() string {
	s := "🥦 Welcome to Wits!\n\n"
	if m.Menu == "main" {
		for i, item := range m.Items {
			cursor := " "
			if m.Cursor == i {
				cursor = "➡ "
			}
			s += fmt.Sprintf("%s(%d): %s\n", cursor, i+1, item)
		}
	} else {
		s += onSubmenuSelected(m)
	}
	s += "\nPress ctrl+c or q to quit."
	if m.Menu != "main" {
		s += "\nPress esc to return to main menu."
	}
	return s
}

// onMenuSelected returns a model for the selected menu.
func onMenuSelected(m MenuModel) (tea.Model, tea.Cmd) {
	switch m.Menu {
	case "main":
		switch m.Cursor {
		case 0:
			return MenuModel{
				Items: StrainsSubmenu,
				Menu:  "strains"}, nil
		case 1:
			return MenuModel{
				Items: DevicesSubmenu,
				Menu:  "devices"}, nil
		case 2:
			return MenuModel{
				Items: SettingsSubmenu,
				Menu:  "settings"}, nil
		case 3:
			return MenuModel{
				Items: StatsSubmenu,
				Menu:  "stats"}, nil
		}
	case "strains":
		switch m.Cursor {
		case 0:
			return AddStrain(strainService), nil
		case 1:
			return ListStrains(strainService), nil
		}
	}
	return m, nil
}

// onSubmenuSelected renders the selected submenu and its items.
func onSubmenuSelected(m MenuModel) string {
	s := fmt.Sprintf("%s Menu:\n", m.Menu)
	for i, item := range m.Items {
		cursor := " "
		if m.Cursor == i {
			cursor = "➡ "
		}
		s += fmt.Sprintf("%s(%d): %s\n", cursor, i+1, item)
	}
	return s
}
