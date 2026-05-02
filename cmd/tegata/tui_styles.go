package main

import "github.com/charmbracelet/lipgloss"

// cinnabar is the primary brand color used throughout the TUI.
const cinnabar lipgloss.Color = "#E34234"

// darkRed is the darker red used for filenames and emphatic headings.
const darkRed lipgloss.Color = "#F04368"

// appName is the display name rendered in the TUI header.
const appName = "Tegata"

// titleStyle renders bold centered text for wizard and overlay titles.
var titleStyle = lipgloss.NewStyle().Bold(true).AlignHorizontal(lipgloss.Center)

// appNameStyle renders the "Tegata" app name in cinnabar (brand color), bold.
var appNameStyle = lipgloss.NewStyle().Foreground(cinnabar).Bold(true)

// errorStyle renders error messages in rose-red.
var errorStyle = lipgloss.NewStyle().Foreground(darkRed)

// helpBarStyle renders the bottom help bar in dim/faint text.
var helpBarStyle = lipgloss.NewStyle().Faint(true)

// warnStyle renders advisory warnings in amber.
var warnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#D97706"))

// tipStyle renders informational tips and selected items in cinnabar, bold.
var tipStyle = lipgloss.NewStyle().Foreground(cinnabar).Bold(true)

// categoryStyle styles category headings: bold with no foreground override so
// the terminal's default text color is used, matching the vault filename in
// the header.
var categoryStyle = lipgloss.NewStyle().Bold(true)

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

// spinnerStyle renders the spinner in amber during async operations.
var spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#D97706"))

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
