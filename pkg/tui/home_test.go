package tui

import (
	"fmt"
	"regexp"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

// Test Models for component testing
type mockModel struct {
	view string
}

func (m mockModel) Init() tea.Cmd                           { return nil }
func (m mockModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (m mockModel) View() string                            { return m.view }

func TestInitialHomeModel(t *testing.T) {
	model := initialHomeModel()

	assert.Equal(t, maxWidth, model.width, "Should set default width")
	assert.Equal(t, homeTitle, model.title, "Should set default title")
	assert.NotNil(t, model.styles, "Should initialize styles")
	assert.IsType(t, &lipgloss.Renderer{}, model.lg, "Should create lipgloss renderer")
}

func TestHomeModel_Update(t *testing.T) {
	t.Run("QuitKey", func(t *testing.T) {
		model := initialHomeModel()
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}

		_, cmd := model.Update(msg)
		// Check if the command is the quit command
		if cmd != nil {
			assert.Equal(t, tea.Quit(), cmd())
		} else {
			t.Error("Expected quit command, got nil")
		}
	})

	t.Run("CtrlC", func(t *testing.T) {
		model := initialHomeModel()
		msg := tea.KeyMsg{Type: tea.KeyCtrlC}

		_, cmd := model.Update(msg)
		// Check if the command is the quit command
		if cmd != nil {
			assert.Equal(t, tea.Quit(), cmd())
		} else {
			t.Error("Expected quit command, got nil")
		}
	})
}

func TestHomeModel_View(t *testing.T) {
	t.Run("EmptyState", func(t *testing.T) {
		model := initialHomeModel()
		view := model.View()

		// Verify header exists
		assert.Contains(t, view, homeTitle, "Should render title")
		// Verify empty list area using regex for flexible whitespace
		assert.Regexp(t, regexp.MustCompile(`\n\s*\n\s*\n`), view, "Should render empty list area")
	})

	t.Run("WithComponents", func(t *testing.T) {
		model := initialHomeModel()
		model.List(mockModel{view: "LIST"})
		model.Bar(mockModel{view: "BAR"})
		model.Extras(mockModel{view: "EXTRAS"})
		model.Preview(mockModel{view: "PREVIEW"})

		view := model.View()
		// Verify component containment
		assert.Contains(t, view, "LIST", "Should render list")
		assert.Contains(t, view, "BAR", "Should render bar")
		assert.Contains(t, view, "EXTRAS", "Should render extras")
		assert.Contains(t, view, "PREVIEW", "Should render preview")
	})
}

func TestMarkedText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"NoMarker", "Test", "Test"},
		{"ValidMarker", "&Test", "\033[4mT\033[0mest"},
		{"EndMarker", "Test&", "Test&"},
		{"DoubleMarker", "T&es&t", "T\033[4me\033[0ms&t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := markedText(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBreadcrumbTitle(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{"Single", []string{"Home"}, "Home"},
		{"Multiple", []string{"Home", "Settings", "Profile"}, "Home > Settings > Profile"},
		{"EmptyNodes", []string{"", "Settings", ""}, " > Settings > "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := breadcrumbTitle(tt.input...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStyles(t *testing.T) {
	renderer := lipgloss.DefaultRenderer()
	styles := NewStyles(renderer)

	t.Run("BaseStyle", func(t *testing.T) {
		// Get all 4 padding values
		top, right, bottom, left := styles.Base.GetPadding()
		assert.Equal(t, "1 4 0 1", fmt.Sprintf("%d %d %d %d", top, right, bottom, left))
	})

	t.Run("HeaderText", func(t *testing.T) {
		assert.True(t, styles.HeaderText.GetBold())
		assert.Equal(t, indigo, styles.HeaderText.GetForeground())
	})
}

func TestHomeModelBuilder(t *testing.T) {
	model := initialHomeModel()

	t.Run("SetTitle", func(t *testing.T) {
		model.Title("New Title")
		assert.Equal(t, "New Title", model.title)
	})

	t.Run("SetComponents", func(t *testing.T) {
		list := mockModel{view: "list"}
		bar := mockModel{view: "bar"}
		extras := mockModel{view: "extras"}
		preview := mockModel{view: "preview"}

		model.List(list)
		model.Bar(bar)
		model.Extras(extras)
		model.Preview(preview)

		assert.Equal(t, list, model.listView)
		assert.Equal(t, bar, model.listBar)
		assert.Equal(t, extras, model.listExtras)
		assert.Equal(t, preview, model.preview)
	})
}
