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

// warnStyle renders advisory warnings in dark orange.
var warnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#D4660A"))

// tipStyle renders informational tips in green, bold.
var tipStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6BA55E")).Bold(true)

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

// renderErrMsg renders msg with errorStyle, wrapping at terminal width minus
// padding so long error strings (e.g. file paths) do not overflow narrow
// terminals. Subtracts 4 columns for left/right margin, with a minimum of 40
// to ensure readability even on very narrow terminals (< 44 columns).
func renderErrMsg(msg string, termWidth int) string {
	w := termWidth - 4 // Reserve 4 columns for margins/padding
	if w < 40 {        // Minimum width for readability
		w = 40
	}
	return errorStyle.Width(w).Render(msg)
}
