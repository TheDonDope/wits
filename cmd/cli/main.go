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

func main() {
	strainStore := storage.NewStrainStoreInMemory()
	strainService := service.NewStrainService(strainStore)
	_, err := tea.NewProgram(tui.InitialMenuModel(strainService, strainStore)).Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting program: %v", err)
		os.Exit(1)
	}
}
