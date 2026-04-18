package main

import "github.com/charmbracelet/lipgloss"

// titleStyle renders bold centered text for wizard and overlay titles.
var titleStyle = lipgloss.NewStyle().Bold(true).AlignHorizontal(lipgloss.Center)

// errorStyle renders error messages in red.
var errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F5F"))

// successStyle renders success or info messages in green.
var successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#5FFF5F"))

// helpBarStyle renders the bottom help bar in dim/faint text.
var helpBarStyle = lipgloss.NewStyle().Faint(true)

// overlayBoxStyle is a centered bordered box used for overlay modals.
var overlayBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder()).
	Padding(1).
	Width(60)

// sidebarStyle is the fixed-width credential list sidebar.
var sidebarStyle = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder()).
	Width(30)

// panelStyle is the main content panel that fills remaining width.
var panelStyle = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder())

// spinnerStyle renders the spinner in cyan during async operations.
var spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FFFF"))

// renderErrMsg renders msg with errorStyle, wrapping at termWidth-4 columns so
// long error strings (e.g. file paths) do not overflow narrow terminals.
func renderErrMsg(msg string, termWidth int) string {
	w := termWidth - 4
	if w < 40 {
		w = 40
	}
	return errorStyle.Width(w).Render(msg)
}
