package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/josh-wong/tegata/internal/config"
)

// settingsMenuItems lists the four settings menu options in order.
var settingsMenuItems = []string{
	"Tag management",
	"Change passphrase",
	"Export / import",
	"Config settings",
}

// resetSettingsOverlay resets all settings overlay state to defaults.
func (m *model) resetSettingsOverlay() {
	m.settingsMenuIdx = 0
	m.settingsSubFlow = ""
	m.settingsInput1.Reset()
	m.settingsInput1.Blur()
	m.settingsInput1.EchoMode = textinput.EchoNormal
	m.settingsInput2.Reset()
	m.settingsInput2.Blur()
	m.settingsMsg = ""
	m.settingsTagIdx = 0
	m.settingsEditMode = ""
}

// updateOverlaySettings handles key events in stateOverlaySettings.
func (m model) updateOverlaySettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.settingsSubFlow {
	case "tags":
		return m.updateSettingsTags(msg)
	case "passphrase":
		return m.updateSettingsPassphrase(msg)
	case "export":
		return m.updateSettingsExport(msg)
	case "import":
		return m.updateSettingsImport(msg)
	case "config":
		return m.updateSettingsConfig(msg)
	default:
		return m.updateSettingsMenu(msg)
	}
}

// updateSettingsMenu handles the top-level settings menu navigation.
func (m model) updateSettingsMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyEsc:
			m.resetSettingsOverlay()
			m.state = stateMainView
			return m, nil

		case msg.Type == tea.KeyDown || (len(msg.Runes) == 1 && msg.Runes[0] == 'j'):
			if m.settingsMenuIdx < len(settingsMenuItems)-1 {
				m.settingsMenuIdx++
			}
			return m, nil

		case msg.Type == tea.KeyUp || (len(msg.Runes) == 1 && msg.Runes[0] == 'k'):
			if m.settingsMenuIdx > 0 {
				m.settingsMenuIdx--
			}
			return m, nil

		case msg.Type == tea.KeyEnter:
			switch m.settingsMenuIdx {
			case 0:
				m.settingsSubFlow = "tags"
				m.settingsTagIdx = 0
				m.settingsInput1.Reset()
				m.settingsInput1.Placeholder = "New tag"
				m.settingsInput1.EchoMode = textinput.EchoNormal
				m.settingsInput1.Blur()
				m.settingsMsg = ""
			case 1:
				m.settingsSubFlow = "passphrase"
				m.settingsInput1.Reset()
				m.settingsInput1.Placeholder = "New passphrase"
				m.settingsInput1.EchoMode = textinput.EchoPassword
				m.settingsInput1.EchoCharacter = '·'
				m.settingsInput2.Reset()
				m.settingsInput2.Placeholder = "Confirm passphrase"
				m.settingsInput2.EchoMode = textinput.EchoPassword
				m.settingsInput2.EchoCharacter = '·'
				m.settingsInput1.Focus()
				m.settingsMsg = ""
			case 2:
				// Export / import: show sub-menu; we reuse settingsSubFlow to
				// track which branch the user picks. Start with "export" prompt.
				m.settingsSubFlow = "export"
				m.settingsInput1.Reset()
				m.settingsInput1.Placeholder = "Export file path"
				m.settingsInput1.EchoMode = textinput.EchoNormal
				m.settingsInput2.Reset()
				m.settingsInput2.Placeholder = "Export passphrase"
				m.settingsInput2.EchoMode = textinput.EchoPassword
				m.settingsInput2.EchoCharacter = '·'
				m.settingsInput1.Focus()
				m.settingsMsg = ""
			case 3:
				m.settingsSubFlow = "config"
				m.settingsMsg = ""
				m.settingsEditMode = ""
				m.settingsInput1.Reset()
				m.settingsInput1.Blur()
			}
			return m, nil
		}
	}
	return m, nil
}

// updateSettingsTags handles the tag management sub-flow.
func (m model) updateSettingsTags(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// If the tag input is focused, handle typing and Enter/Esc.
		if m.settingsInput1.Focused() {
			switch msg.Type {
			case tea.KeyEsc:
				m.settingsInput1.Reset()
				m.settingsInput1.Blur()
				return m, nil
			case tea.KeyEnter:
				tag := strings.TrimSpace(m.settingsInput1.Value())
				if tag != "" && m.vaultMgr != nil {
					selected := m.credList.SelectedItem()
					if selected != nil {
						if item, ok := selected.(credItem); ok {
							cred := item.cred
							cred.Tags = append(cred.Tags, tag)
							if err := m.vaultMgr.UpdateCredential(&cred); err != nil {
								m.settingsMsg = fmt.Sprintf("Error: %v", err)
							} else {
								m.settingsMsg = fmt.Sprintf("Added tag %q", tag)
								m = refreshCredList(m)
							}
						}
					}
				}
				m.settingsInput1.Reset()
				m.settingsInput1.Blur()
				return m, nil
			}
			var cmd tea.Cmd
			m.settingsInput1, cmd = m.settingsInput1.Update(msg)
			return m, cmd
		}

		switch {
		case msg.Type == tea.KeyEsc:
			m.settingsSubFlow = ""
			m.settingsMsg = ""
			return m, nil

		case msg.Type == tea.KeyDown || (len(msg.Runes) == 1 && msg.Runes[0] == 'j'):
			maxIdx := 0
			if selected := m.credList.SelectedItem(); selected != nil {
				if item, ok := selected.(credItem); ok && len(item.cred.Tags) > 0 {
					maxIdx = len(item.cred.Tags) - 1
				}
			}
			if m.settingsTagIdx < maxIdx {
				m.settingsTagIdx++
			}
			return m, nil

		case msg.Type == tea.KeyUp || (len(msg.Runes) == 1 && msg.Runes[0] == 'k'):
			if m.settingsTagIdx > 0 {
				m.settingsTagIdx--
			}
			return m, nil

		case len(msg.Runes) == 1 && msg.Runes[0] == 'a':
			m.settingsInput1.Focus()
			return m, nil

		case len(msg.Runes) == 1 && msg.Runes[0] == 'd':
			if m.vaultMgr == nil {
				return m, nil
			}
			selected := m.credList.SelectedItem()
			if selected == nil {
				return m, nil
			}
			item, ok := selected.(credItem)
			if !ok {
				return m, nil
			}
			cred := item.cred
			if m.settingsTagIdx >= 0 && m.settingsTagIdx < len(cred.Tags) {
				removed := cred.Tags[m.settingsTagIdx]
				cred.Tags = append(cred.Tags[:m.settingsTagIdx], cred.Tags[m.settingsTagIdx+1:]...)
				if err := m.vaultMgr.UpdateCredential(&cred); err != nil {
					m.settingsMsg = fmt.Sprintf("Error: %v", err)
				} else {
					m.settingsMsg = fmt.Sprintf("Removed tag %q", removed)
					m = refreshCredList(m)
					if m.settingsTagIdx > 0 {
						m.settingsTagIdx--
					}
				}
			}
			return m, nil
		}
	}
	return m, nil
}

// updateSettingsPassphrase handles the change-passphrase sub-flow.
func (m model) updateSettingsPassphrase(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.settingsSubFlow = ""
			m.settingsInput1.Reset()
			m.settingsInput1.Blur()
			m.settingsInput2.Reset()
			m.settingsInput2.Blur()
			m.settingsMsg = ""
			return m, nil

		case tea.KeyTab:
			if m.settingsInput1.Focused() {
				m.settingsInput1.Blur()
				m.settingsInput2.Focus()
			} else {
				m.settingsInput2.Blur()
				m.settingsInput1.Focus()
			}
			return m, nil

		case tea.KeyEnter:
			pp1 := m.settingsInput1.Value()
			pp2 := m.settingsInput2.Value()

			if pp1 != pp2 {
				m.settingsMsg = "Passphrases do not match"
				return m, nil
			}
			if len(pp1) < 8 {
				m.settingsMsg = "Passphrase must be at least 8 characters"
				return m, nil
			}
			if m.vaultMgr == nil {
				m.settingsMsg = "Vault not unlocked"
				return m, nil
			}

			pp := []byte(pp1)
			defer zeroBytes(pp)

			if err := m.vaultMgr.ChangePassphrase(pp); err != nil {
				m.settingsMsg = fmt.Sprintf("Error: %v", err)
				return m, nil
			}
			if err := m.vaultMgr.Save(); err != nil {
				m.settingsMsg = fmt.Sprintf("Save error: %v", err)
				return m, nil
			}

			m.settingsInput1.Reset()
			m.settingsInput1.Blur()
			m.settingsInput2.Reset()
			m.settingsInput2.Blur()
			m.settingsMsg = "Passphrase changed."
			m.settingsSubFlow = ""
			return m, nil
		}
	}

	var cmd tea.Cmd
	if m.settingsInput1.Focused() {
		m.settingsInput1, cmd = m.settingsInput1.Update(msg)
	} else {
		m.settingsInput2, cmd = m.settingsInput2.Update(msg)
	}
	return m, cmd
}

// updateSettingsExport handles the export sub-flow.
func (m model) updateSettingsExport(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.settingsSubFlow = ""
			m.settingsInput1.Reset()
			m.settingsInput1.Blur()
			m.settingsInput2.Reset()
			m.settingsInput2.Blur()
			m.settingsMsg = ""
			return m, nil

		case tea.KeyTab:
			if m.settingsInput1.Focused() {
				m.settingsInput1.Blur()
				m.settingsInput2.Focus()
			} else {
				m.settingsInput2.Blur()
				m.settingsInput1.Focus()
			}
			return m, nil

		// Switch between export and import.
		case tea.KeyF1:
			m.settingsSubFlow = "import"
			m.settingsInput1.Reset()
			m.settingsInput1.Placeholder = "Import file path"
			m.settingsInput1.EchoMode = textinput.EchoNormal
			m.settingsInput2.Reset()
			m.settingsInput2.Placeholder = "Import passphrase"
			m.settingsInput2.EchoMode = textinput.EchoPassword
			m.settingsInput2.EchoCharacter = '·'
			m.settingsInput2.Blur()
			m.settingsInput1.Focus()
			m.settingsMsg = ""
			return m, nil

		case tea.KeyEnter:
			path := m.settingsInput1.Value()
			if path == "" || m.settingsInput2.Value() == "" {
				m.settingsMsg = "File path and passphrase are required"
				return m, nil
			}
			if m.vaultMgr == nil {
				m.settingsMsg = "Vault not unlocked"
				return m, nil
			}

			pp := []byte(m.settingsInput2.Value())
			defer zeroBytes(pp)

			data, err := m.vaultMgr.ExportCredentials(pp)
			if err != nil {
				m.settingsMsg = fmt.Sprintf("Export failed: %v", err)
				return m, nil
			}

			if err := os.WriteFile(path, data, 0600); err != nil {
				m.settingsMsg = fmt.Sprintf("Write failed: %v", err)
				return m, nil
			}

			m.settingsInput1.Reset()
			m.settingsInput1.Blur()
			m.settingsInput2.Reset()
			m.settingsInput2.Blur()
			m.settingsMsg = "Exported to " + path
			m.settingsSubFlow = ""
			return m, nil
		}
	}

	var cmd tea.Cmd
	if m.settingsInput1.Focused() {
		m.settingsInput1, cmd = m.settingsInput1.Update(msg)
	} else {
		m.settingsInput2, cmd = m.settingsInput2.Update(msg)
	}
	return m, cmd
}

// updateSettingsImport handles the import sub-flow.
func (m model) updateSettingsImport(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.settingsSubFlow = ""
			m.settingsInput1.Reset()
			m.settingsInput1.Blur()
			m.settingsInput2.Reset()
			m.settingsInput2.Blur()
			m.settingsMsg = ""
			return m, nil

		case tea.KeyTab:
			if m.settingsInput1.Focused() {
				m.settingsInput1.Blur()
				m.settingsInput2.Focus()
			} else {
				m.settingsInput2.Blur()
				m.settingsInput1.Focus()
			}
			return m, nil

		case tea.KeyEnter:
			path := m.settingsInput1.Value()
			if path == "" || m.settingsInput2.Value() == "" {
				m.settingsMsg = "File path and passphrase are required"
				return m, nil
			}
			if m.vaultMgr == nil {
				m.settingsMsg = "Vault not unlocked"
				return m, nil
			}

			data, err := os.ReadFile(path)
			if err != nil {
				m.settingsMsg = fmt.Sprintf("Read failed: %v", err)
				return m, nil
			}

			pp := []byte(m.settingsInput2.Value())
			defer zeroBytes(pp)

			imported, skipped, err := m.vaultMgr.ImportCredentials(data, pp)
			if err != nil {
				m.settingsMsg = fmt.Sprintf("Import failed: %v", err)
				return m, nil
			}

			if err := m.vaultMgr.Save(); err != nil {
				m.settingsMsg = fmt.Sprintf("Save error: %v", err)
				return m, nil
			}

			m = refreshCredList(m)
			m.settingsInput1.Reset()
			m.settingsInput1.Blur()
			m.settingsInput2.Reset()
			m.settingsInput2.Blur()
			m.settingsMsg = fmt.Sprintf("Imported %d credentials (%d skipped)", imported, skipped)
			m.settingsSubFlow = ""
			return m, nil
		}
	}

	var cmd tea.Cmd
	if m.settingsInput1.Focused() {
		m.settingsInput1, cmd = m.settingsInput1.Update(msg)
	} else {
		m.settingsInput2, cmd = m.settingsInput2.Update(msg)
	}
	return m, cmd
}

// updateSettingsConfig handles the config settings sub-flow.
func (m model) updateSettingsConfig(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// If an edit input is active, handle typing and Enter/Esc.
		if m.settingsInput1.Focused() {
			switch msg.Type {
			case tea.KeyEsc:
				m.settingsInput1.Reset()
				m.settingsInput1.Blur()
				m.settingsEditMode = ""
				return m, nil
			case tea.KeyEnter:
				val, err := strconv.Atoi(m.settingsInput1.Value())
				if err != nil || val < 1 {
					m.settingsMsg = "Enter a positive integer (seconds)"
					m.settingsInput1.Reset()
					m.settingsInput1.Blur()
					m.settingsEditMode = ""
					return m, nil
				}
				switch m.settingsEditMode {
				case "clipboard":
					m.cfg.ClipboardTimeout = secondsDuration(val)
				case "idle":
					m.cfg.IdleTimeout = secondsDuration(val)
					m.idleTimeout = m.cfg.IdleTimeout
				}
				if err := writeConfigFile(m.vaultPath, m.cfg); err != nil {
					m.settingsMsg = fmt.Sprintf("Save error: %v", err)
				} else {
					m.settingsMsg = "Config saved."
				}
				m.settingsInput1.Reset()
				m.settingsInput1.Blur()
				m.settingsEditMode = ""
				return m, nil
			}
			var cmd tea.Cmd
			m.settingsInput1, cmd = m.settingsInput1.Update(msg)
			return m, cmd
		}

		switch {
		case msg.Type == tea.KeyEsc:
			m.settingsSubFlow = ""
			m.settingsMsg = ""
			return m, nil

		case len(msg.Runes) == 1 && msg.Runes[0] == 'c':
			m.settingsEditMode = "clipboard"
			m.settingsInput1.Reset()
			m.settingsInput1.Placeholder = fmt.Sprintf("Seconds (current: %d)", int(m.cfg.ClipboardTimeout.Seconds()))
			m.settingsInput1.EchoMode = textinput.EchoNormal
			m.settingsInput1.Focus()
			return m, nil

		case len(msg.Runes) == 1 && msg.Runes[0] == 'i':
			m.settingsEditMode = "idle"
			m.settingsInput1.Reset()
			m.settingsInput1.Placeholder = fmt.Sprintf("Seconds (current: %d)", int(m.cfg.IdleTimeout.Seconds()))
			m.settingsInput1.EchoMode = textinput.EchoNormal
			m.settingsInput1.Focus()
			return m, nil
		}
	}
	return m, nil
}

// viewOverlaySettings renders the settings overlay with the appropriate sub-flow.
func (m model) viewOverlaySettings() string {
	var content string
	switch m.settingsSubFlow {
	case "tags":
		content = m.viewSettingsTags()
	case "passphrase":
		content = m.viewSettingsPassphrase()
	case "export":
		content = m.viewSettingsExportImport("Export credentials", "Export file path", "Export passphrase",
			"[Tab] Next  [Enter] Export  [F1] Switch to Import  [Esc] Cancel")
	case "import":
		content = m.viewSettingsExportImport("Import credentials", "Import file path", "Import passphrase",
			"[Tab] Next  [Enter] Import  [Esc] Cancel")
	case "config":
		content = m.viewSettingsConfig()
	default:
		content = m.viewSettingsMenu()
	}

	overlay := overlayBoxStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay)
}

// viewSettingsMenu renders the top-level settings menu.
func (m model) viewSettingsMenu() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Settings"))
	lines = append(lines, "")
	for i, item := range settingsMenuItems {
		if i == m.settingsMenuIdx {
			lines = append(lines, successStyle.Render("> "+item))
		} else {
			lines = append(lines, "  "+item)
		}
	}
	if m.settingsMsg != "" {
		lines = append(lines, "")
		lines = append(lines, successStyle.Render(m.settingsMsg))
	}
	lines = append(lines, "")
	lines = append(lines, helpBarStyle.Render("[j/k] Navigate  [Enter] Select  [Esc] Close"))
	return strings.Join(lines, "\n")
}

// viewSettingsTags renders the tag management sub-flow.
func (m model) viewSettingsTags() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Tag management"))
	lines = append(lines, "")

	var tags []string
	if selected := m.credList.SelectedItem(); selected != nil {
		if item, ok := selected.(credItem); ok {
			tags = item.cred.Tags
		}
	}

	if len(tags) == 0 {
		lines = append(lines, "  (no tags)")
	} else {
		for i, tag := range tags {
			if i == m.settingsTagIdx {
				lines = append(lines, successStyle.Render("> "+tag))
			} else {
				lines = append(lines, "  "+tag)
			}
		}
	}

	lines = append(lines, "")
	if m.settingsInput1.Focused() {
		lines = append(lines, "New tag: "+m.settingsInput1.View())
	}

	if m.settingsMsg != "" {
		lines = append(lines, "")
		lines = append(lines, successStyle.Render(m.settingsMsg))
	}
	lines = append(lines, "")
	lines = append(lines, helpBarStyle.Render("[a] Add  [d] Remove  [Esc] Done"))
	return strings.Join(lines, "\n")
}

// viewSettingsPassphrase renders the change-passphrase sub-flow.
func (m model) viewSettingsPassphrase() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Change passphrase"))
	lines = append(lines, "")
	lines = append(lines, "New passphrase:     "+m.settingsInput1.View())
	lines = append(lines, "Confirm passphrase: "+m.settingsInput2.View())
	if m.settingsMsg != "" {
		lines = append(lines, "")
		lines = append(lines, errorStyle.Render(m.settingsMsg))
	}
	lines = append(lines, "")
	lines = append(lines, helpBarStyle.Render("[Tab] Next  [Enter] Confirm  [Esc] Cancel"))
	return strings.Join(lines, "\n")
}

// viewSettingsExportImport renders the export or import sub-flow form.
func (m model) viewSettingsExportImport(title, label1, label2, help string) string {
	var lines []string
	lines = append(lines, titleStyle.Render(title))
	lines = append(lines, "")
	lines = append(lines, label1+": "+m.settingsInput1.View())
	lines = append(lines, label2+": "+m.settingsInput2.View())
	if m.settingsMsg != "" {
		lines = append(lines, "")
		lines = append(lines, successStyle.Render(m.settingsMsg))
	}
	lines = append(lines, "")
	lines = append(lines, helpBarStyle.Render(help))
	return strings.Join(lines, "\n")
}

// viewSettingsConfig renders the config settings sub-flow.
func (m model) viewSettingsConfig() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Config settings"))
	lines = append(lines, "")
	clipSec := int(m.cfg.ClipboardTimeout.Seconds())
	idleSec := int(m.cfg.IdleTimeout.Seconds())
	lines = append(lines, fmt.Sprintf("Clipboard timeout: %ds  [c to edit]", clipSec))
	lines = append(lines, fmt.Sprintf("Idle timeout:      %ds  [i to edit]", idleSec))
	if m.settingsInput1.Focused() {
		lines = append(lines, "")
		lines = append(lines, m.settingsInput1.View())
	}
	if m.settingsMsg != "" {
		lines = append(lines, "")
		lines = append(lines, successStyle.Render(m.settingsMsg))
	}
	lines = append(lines, "")
	lines = append(lines, helpBarStyle.Render("[c] Edit clipboard  [i] Edit idle  [Esc] Back"))
	return strings.Join(lines, "\n")
}

// writeConfigFile writes the effective clipboard and idle timeouts to tegata.toml.
func writeConfigFile(vaultPath string, cfg config.Config) error {
	dir := filepath.Dir(vaultPath)
	content := fmt.Sprintf("[clipboard]\ntimeout = %d\n\n[vault]\nidle_timeout = %d\n",
		int(cfg.ClipboardTimeout.Seconds()),
		int(cfg.IdleTimeout.Seconds()))
	return os.WriteFile(filepath.Join(dir, "tegata.toml"), []byte(content), 0600)
}

// secondsDuration converts an integer number of seconds to a time.Duration.
func secondsDuration(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}
