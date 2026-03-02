// Copyright (c) 2026 DarkOneiroi
// All rights reserved.
// This source code is proprietary and confidential.
// Unauthorized copying of this file, via any medium, is strictly prohibited.

package ui

import "github.com/charmbracelet/lipgloss"

// Colors used throughout the application
var (
	Pink   = lipgloss.Color("205")
	Blue   = lipgloss.Color("39")
	Gray   = lipgloss.Color("241")
	Muted  = lipgloss.Color("245")
	White  = lipgloss.Color("15")
	Black  = lipgloss.Color("0")
	Orange = lipgloss.Color("208")
	Red    = lipgloss.Color("1")
)

// Styles encapsulates all pre-defined styles for the TUI.
// Pre-defining styles prevents repeated allocations during the render loop.
type Styles struct {
	Header           lipgloss.Style
	ActiveTab        lipgloss.Style
	InactiveTab      lipgloss.Style
	Legend           lipgloss.Style
	Status           lipgloss.Style
	Title            lipgloss.Style
	ActiveTitle      lipgloss.Style
	Subtitle         lipgloss.Style
	Meta             lipgloss.Style
	MetaHighlight    lipgloss.Style
	Cursor           lipgloss.Style
	Confirmation     lipgloss.Style
	ProgressBarEmpty lipgloss.Style
	ProgressBarFull  lipgloss.Style
	WorkerActive     lipgloss.Style
	WorkerPaused     lipgloss.Style
}

func DefaultStyles() Styles {
	s := Styles{}

	s.Header = lipgloss.NewStyle().MarginBottom(1)
	s.ActiveTab = lipgloss.NewStyle().
		Padding(0, 1).
		Bold(true).
		Background(Pink).
		Foreground(White)
	s.InactiveTab = lipgloss.NewStyle().
		Padding(0, 1)

	s.Legend = lipgloss.NewStyle().
		Foreground(Gray).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(lipgloss.Color("238")).
		PaddingTop(1)

	s.Status = lipgloss.NewStyle().Foreground(Pink)
	
	s.Title = lipgloss.NewStyle().Bold(true)
	s.ActiveTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(Pink)
	
	s.Subtitle = lipgloss.NewStyle().Foreground(Muted)
	s.Meta = lipgloss.NewStyle().Foreground(Muted)
	s.MetaHighlight = lipgloss.NewStyle().Foreground(Blue)
	
	s.Cursor = lipgloss.NewStyle().
		Foreground(Pink).
		Bold(true)

	s.Confirmation = lipgloss.NewStyle().
		Bold(true).
		Foreground(White).
		Background(Red).
		Align(lipgloss.Center, lipgloss.Center)

	s.WorkerActive = lipgloss.NewStyle().Foreground(Blue)
	s.WorkerPaused = lipgloss.NewStyle().Foreground(Orange)

	return s
}
