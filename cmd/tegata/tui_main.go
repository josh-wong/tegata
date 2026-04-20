package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/auth"
	pkgmodel "github.com/josh-wong/tegata/pkg/model"
)

// credItem wraps a model.Credential for display in the bubbles/list sidebar.
type credItem struct{ cred pkgmodel.Credential }

// FilterValue implements list.Item for fuzzy filtering (disabled but required).
func (i credItem) FilterValue() string { return i.cred.Label }

// Title returns the sidebar label, truncated to 20 runes before any lipgloss
// styling is applied (ANSI-safe).
func (i credItem) Title() string { return truncateLabel(i.cred.Label, 20) }

// Description returns the credential type string shown below the label.
func (i credItem) Description() string { return string(i.cred.Type) }

// truncateLabel truncates s to at most max runes. If truncated, the last rune
// is replaced by an ellipsis so the total width stays within max.
func truncateLabel(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

// totpProgressBar renders a Unicode block progress bar proportional to the
// fraction of remaining/period. Colors: green when > half period, amber when
// <= half, red when <= quarter.
func totpProgressBar(remaining, period, width int) string {
	if period <= 0 || width <= 0 {
		return ""
	}
	filled := int(float64(remaining) / float64(period) * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)

	var color lipgloss.Color
	switch {
	case remaining > period/2:
		color = lipgloss.Color("#5FFF5F") // green
	case remaining > period/4:
		color = lipgloss.Color("#FFAF5F") // amber
	default:
		color = lipgloss.Color("#FF5F5F") // red
	}
	return lipgloss.NewStyle().Foreground(color).Render(bar)
}

// updateMainView handles all messages when the model is in stateMainView.
func (m model) updateMainView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		m.lastActivity = time.Now()

		// CR challenge input is active — delegate all typing to it.
		if m.crChallengeActive {
			switch msg.Type {
			case tea.KeyEsc:
				m.crChallengeActive = false
				m.crChallengeInput.Reset()
				m.crChallengeInput.Blur()
				return m, nil

			case tea.KeyEnter:
				return m.submitCRChallenge()
			}
			var cmd tea.Cmd
			m.crChallengeInput, cmd = m.crChallengeInput.Update(msg)
			return m, cmd
		}

		switch {
		case msg.Type == tea.KeyDown || (len(msg.Runes) == 1 && msg.Runes[0] == 'j'):
			if m.cursor < len(m.credList.Items())-1 {
				m.cursor++
			}
			m.statusMsg = ""
			m.errMsg = ""
			var cmd tea.Cmd
			m.credList, cmd = m.credList.Update(tea.KeyMsg{Type: tea.KeyDown})
			return m, cmd

		case msg.Type == tea.KeyUp || (len(msg.Runes) == 1 && msg.Runes[0] == 'k'):
			if m.cursor > 0 {
				m.cursor--
			}
			m.statusMsg = ""
			m.errMsg = ""
			var cmd tea.Cmd
			m.credList, cmd = m.credList.Update(tea.KeyMsg{Type: tea.KeyUp})
			return m, cmd

		case msg.Type == tea.KeyEnter:
			return m.handleCredentialAction()

		case len(msg.Runes) == 1 && msg.Runes[0] == 'a':
			m.state = stateOverlayAdd
			m.addFocusIdx = 0
			m.focusAddInput()
			return m, nil

		case len(msg.Runes) == 1 && msg.Runes[0] == 'r':
			m.state = stateOverlayRemove
			return m, nil

		case len(msg.Runes) == 1 && msg.Runes[0] == 's':
			m.state = stateOverlaySettings
			return m, nil

		case len(msg.Runes) == 1 && msg.Runes[0] == 'v':
			if !m.cfg.Audit.Enabled {
				m.errMsg = "Audit logging is not enabled"
				return m, nil
			}
			m.state = stateOverlayAudit
			return m, nil
		}
	}

	return m, nil
}

// handleCredentialAction dispatches Enter based on the selected credential type.
func (m model) handleCredentialAction() (tea.Model, tea.Cmd) {
	selected := m.credList.SelectedItem()
	if selected == nil {
		return m, nil
	}
	item, ok := selected.(credItem)
	if !ok {
		return m, nil
	}
	cred := item.cred

	switch cred.Type {
	case pkgmodel.CredentialTOTP:
		secret, err := decodeBase32Secret(cred.Secret)
		if err != nil {
			m.errMsg = fmt.Sprintf("Invalid TOTP secret: %v", err)
			return m, nil
		}
		defer zeroBytes(secret)
		period := cred.Period
		if period <= 0 {
			period = 30
		}
		digits := cred.Digits
		if digits <= 0 {
			digits = 6
		}
		code, _ := auth.GenerateTOTP(secret, m.now, period, digits, cred.Algorithm)
		if m.builder != nil {
			_ = m.builder.LogEvent("totp", cred.Label, cred.Issuer, audit.Hostname(), true)
		}
		if m.clipMgr != nil {
			if err := m.clipMgr.CopyWithAutoClear(code, m.cfg.ClipboardTimeout); err != nil {
				m.statusMsg = fmt.Sprintf("Code: %s  (clipboard unavailable — select to copy)", code)
				m.errMsg = ""
				return m, nil
			}
			m.statusMsg = fmt.Sprintf("Copied! (auto-clear in %ds)", int(m.cfg.ClipboardTimeout.Seconds()))
		} else {
			m.statusMsg = fmt.Sprintf("Code: %s  (clipboard unavailable)", code)
		}
		m.errMsg = ""

	case pkgmodel.CredentialHOTP:
		secret, err := decodeBase32Secret(cred.Secret)
		if err != nil {
			m.errMsg = fmt.Sprintf("Invalid HOTP secret: %v", err)
			return m, nil
		}
		defer zeroBytes(secret)
		digits := cred.Digits
		if digits <= 0 {
			digits = 6
		}
		code := auth.GenerateHOTP(secret, cred.Counter, digits, cred.Algorithm)
		// Advance counter before displaying so HOTP state stays correct
		// regardless of clipboard outcome.
		cred.Counter++
		if m.vaultMgr != nil {
			if err := m.vaultMgr.UpdateCredential(&cred); err != nil {
				m.errMsg = fmt.Sprintf("Counter save failed: %v", err)
				return m, nil
			}
			m = refreshCredList(m, cred.Label)
		}
		if m.builder != nil {
			_ = m.builder.LogEvent("hotp", cred.Label, cred.Issuer, audit.Hostname(), true)
		}
		if m.clipMgr != nil {
			if err := m.clipMgr.CopyWithAutoClear(code, m.cfg.ClipboardTimeout); err != nil {
				m.statusMsg = fmt.Sprintf("Code: %s  (clipboard unavailable — select to copy)", code)
				m.errMsg = ""
				return m, nil
			}
			m.statusMsg = fmt.Sprintf("Copied! (auto-clear in %ds)", int(m.cfg.ClipboardTimeout.Seconds()))
		} else {
			m.statusMsg = fmt.Sprintf("Code: %s  (clipboard unavailable)", code)
		}
		m.errMsg = ""

	case pkgmodel.CredentialStatic:
		password, err := auth.GetStaticPassword(&cred)
		if err != nil {
			m.errMsg = fmt.Sprintf("Error: %v", err)
			return m, nil
		}
		defer zeroBytes(password)
		if m.builder != nil {
			_ = m.builder.LogEvent("static", cred.Label, cred.Issuer, audit.Hostname(), true)
		}
		if m.clipMgr != nil {
			if err := m.clipMgr.CopyWithAutoClear(string(password), m.cfg.ClipboardTimeout); err != nil {
				m.statusMsg = fmt.Sprintf("Password: %s  (clipboard unavailable — select to copy)", password)
				m.errMsg = ""
				return m, nil
			}
			m.statusMsg = fmt.Sprintf("Copied! (auto-clear in %ds)", int(m.cfg.ClipboardTimeout.Seconds()))
		} else {
			m.statusMsg = fmt.Sprintf("Password: %s  (clipboard unavailable)", password)
		}
		m.errMsg = ""

	case pkgmodel.CredentialChallengeResponse:
		// Enter challenge-input mode.
		m.crChallengeActive = true
		m.crChallengeInput.Reset()
		m.crChallengeInput.Focus()
		return m, nil
	}

	return m, nil
}

// submitCRChallenge reads the challenge input, signs it, and copies the result.
func (m model) submitCRChallenge() (tea.Model, tea.Cmd) {
	selected := m.credList.SelectedItem()
	if selected == nil {
		m.crChallengeActive = false
		return m, nil
	}
	item, ok := selected.(credItem)
	if !ok {
		m.crChallengeActive = false
		return m, nil
	}
	cred := item.cred

	challenge := []byte(m.crChallengeInput.Value())
	secret, err := decodeBase32Secret(cred.Secret)
	if err != nil {
		// Fall back to raw bytes for plain text shared keys.
		secret = []byte(cred.Secret)
	}
	defer zeroBytes(secret)

	response, err := auth.SignChallenge(&cred, secret, challenge)
	if err != nil {
		m.errMsg = fmt.Sprintf("Sign error: %v", err)
		m.crChallengeActive = false
		m.crChallengeInput.Reset()
		m.crChallengeInput.Blur()
		return m, nil
	}

	if m.builder != nil {
		_ = m.builder.LogEvent("challenge-response", cred.Label, cred.Issuer, audit.Hostname(), true)
	}

	if m.clipMgr != nil {
		if err := m.clipMgr.CopyWithAutoClear(response, m.cfg.ClipboardTimeout); err != nil {
			m.statusMsg = fmt.Sprintf("Response: %s  (clipboard unavailable — select to copy)", response)
			m.errMsg = ""
			m.crChallengeActive = false
			m.crChallengeInput.Reset()
			m.crChallengeInput.Blur()
			return m, nil
		}
		m.statusMsg = fmt.Sprintf("Signed response copied (auto-clear in %ds)", int(m.cfg.ClipboardTimeout.Seconds()))
	} else {
		m.statusMsg = fmt.Sprintf("Response: %s  (clipboard unavailable)", response)
	}
	m.errMsg = ""
	m.crChallengeActive = false
	m.crChallengeInput.Reset()
	m.crChallengeInput.Blur()
	return m, nil
}

// viewMainView renders the two-column credential list + detail panel layout.
func (m model) viewMainView() string {
	// Sidebar (fixed width 30).
	sidebarContent := m.credList.View()
	sidebar := sidebarStyle.Width(28).Height(m.height - 4).Render(sidebarContent)

	// Panel (remaining width).
	panelWidth := m.width - 32
	if panelWidth < 20 {
		panelWidth = 20
	}
	panel := m.renderDetailPanel(panelWidth)

	// Join horizontally.
	columns := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, panel)

	// Help bar at the bottom.
	helpText := "↑↓ Navigate  Enter Copy/Act  a Add  r Remove  s Settings  q Quit"
	if m.cfg.Audit.Enabled {
		helpText = "↑↓ Navigate  Enter Copy/Act  a Add  r Remove  s Settings  v Audit  q Quit"
	}
	help := helpBarStyle.Render(helpText)

	return columns + "\n" + help
}

// renderDetailPanel renders the right-side credential detail for the selected item.
func (m model) renderDetailPanel(width int) string {
	selected := m.credList.SelectedItem()
	if selected == nil {
		content := "No credential selected.\n\nUse ↑↓ to navigate."
		return panelStyle.Width(width - 2).Render(content)
	}
	item, ok := selected.(credItem)
	if !ok {
		return panelStyle.Width(width - 2).Render("")
	}
	cred := item.cred

	var content string
	switch cred.Type {
	case pkgmodel.CredentialTOTP:
		content = m.renderTOTPPanel(cred, width)
	case pkgmodel.CredentialHOTP:
		content = m.renderHOTPPanel(cred)
	case pkgmodel.CredentialStatic:
		content = m.renderStaticPanel(cred)
	case pkgmodel.CredentialChallengeResponse:
		content = m.renderCRPanel(cred)
	default:
		content = fmt.Sprintf("Label: %s\nType: %s", cred.Label, cred.Type)
	}

	if m.statusMsg != "" {
		content += "\n\n" + successStyle.Render(m.statusMsg)
	}
	if m.errMsg != "" {
		content += "\n\n" + errorStyle.Render(m.errMsg)
	}

	return panelStyle.Width(width - 2).Render(content)
}

// renderTOTPPanel renders the live TOTP detail view with countdown.
func (m model) renderTOTPPanel(cred pkgmodel.Credential, width int) string {
	period := cred.Period
	if period <= 0 {
		period = 30
	}
	digits := cred.Digits
	if digits <= 0 {
		digits = 6
	}

	secret, err := decodeBase32Secret(cred.Secret)
	var code string
	var remaining int
	if err != nil {
		code = "??????"
		remaining = 0
	} else {
		code, remaining = auth.GenerateTOTP(secret, m.now, period, digits, cred.Algorithm)
		zeroBytes(secret)
	}

	barWidth := width - 10
	if barWidth < 5 {
		barWidth = 5
	}
	bar := totpProgressBar(remaining, period, barWidth)

	var lines []string
	lines = append(lines, titleStyle.Render(cred.Label))
	if cred.Issuer != "" {
		lines = append(lines, cred.Issuer)
	}
	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render(code))
	lines = append(lines, bar)
	lines = append(lines, fmt.Sprintf("%ds remaining", remaining))

	return strings.Join(lines, "\n")
}

// renderHOTPPanel renders the HOTP credential detail.
func (m model) renderHOTPPanel(cred pkgmodel.Credential) string {
	var lines []string
	lines = append(lines, titleStyle.Render(cred.Label))
	if cred.Issuer != "" {
		lines = append(lines, cred.Issuer)
	}
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Counter: %d", cred.Counter))
	lines = append(lines, "")
	lines = append(lines, helpBarStyle.Render("[Enter] Generate & advance counter"))
	return strings.Join(lines, "\n")
}

// renderStaticPanel renders the static password credential detail.
func (m model) renderStaticPanel(cred pkgmodel.Credential) string {
	var lines []string
	lines = append(lines, titleStyle.Render(cred.Label))
	if cred.Issuer != "" {
		lines = append(lines, cred.Issuer)
	}
	lines = append(lines, "")
	lines = append(lines, helpBarStyle.Render("[Enter] Copy password"))
	return strings.Join(lines, "\n")
}

// renderCRPanel renders the challenge-response credential detail.
func (m model) renderCRPanel(cred pkgmodel.Credential) string {
	var lines []string
	lines = append(lines, titleStyle.Render(cred.Label))
	if cred.Issuer != "" {
		lines = append(lines, cred.Issuer)
	}
	lines = append(lines, "")
	if m.crChallengeActive {
		lines = append(lines, "Challenge: "+m.crChallengeInput.View())
		lines = append(lines, "")
		lines = append(lines, helpBarStyle.Render("[Enter] Sign  [Esc] Cancel"))
	} else {
		lines = append(lines, "Press Enter to sign a challenge.")
	}
	return strings.Join(lines, "\n")
}

// ensure credItem satisfies list.Item at compile time.
var _ list.Item = credItem{}
