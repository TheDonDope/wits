package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInitialMenuModel(t *testing.T) {
	m := InitialMenuModel()
	if len(m.items) == 0 {
		t.Error("Expected initial menu items to be non-empty")
	}
}

func TestMenuNavigationDown(t *testing.T) {
	m := InitialMenuModel()
	// Simulate pressing the "down" key by setting Runes so that msg.String() returns "down".
	msg := tea.KeyMsg{Type: tea.KeyDown, Runes: []rune("down")}
	updatedModel, _ := m.Update(msg)
	newModel, ok := updatedModel.(MenuModel)
	if !ok {
		t.Fatal("Updated model is not of type MenuModel")
	}
	// After one down press, cursor should be 1.
	if newModel.cursor != 1 {
		t.Errorf("Expected cursor to be 1 after one down press, got %d", newModel.cursor)
	}
}

func TestMenuNavigationUpWrap(t *testing.T) {
	m := InitialMenuModel()
	// Set cursor to 0 then simulate pressing "up"
	m.cursor = 0
	msg := tea.KeyMsg{Type: tea.KeyUp, Runes: []rune("up")}
	updatedModel, _ := m.Update(msg)
	newModel, ok := updatedModel.(MenuModel)
	if !ok {
		t.Fatal("Updated model is not of type MenuModel")
	}
	expected := len(newModel.items) - 1
	if newModel.cursor != expected {
		t.Errorf("Expected cursor to wrap to %d when pressing up at 0, got %d", expected, newModel.cursor)
	}
}

func TestMenuViewHeader(t *testing.T) {
	m := InitialMenuModel()
	view := m.View()
	// Check that the header contains the text "Wits" (or whatever header text is set).
	if !strings.Contains(view, "Wits") {
		t.Error("Expected view to contain header with 'Wits'")
	}
}
