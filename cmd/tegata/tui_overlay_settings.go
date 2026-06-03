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
	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/i18n"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/vault"
)

// settingsMenuItems returns the localised settings menu options in order.
// This is a function (not a var) so i18n strings are resolved at call time,
// after i18n.Init() has been called.
func settingsMenuItems() []string {
	return []string{
		i18n.T("tui.settings.menu.tags"),
		i18n.T("tui.settings.menu.passphrase"),
		i18n.T("tui.settings.menu.exportImport"),
		i18n.T("tui.settings.menu.config"),
		i18n.T("tui.settings.menu.verifyRecovery"),
	}
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
	m.settingsInput3.Reset()
	m.settingsInput3.Blur()
	m.settingsMsg = ""
	m.settingsTagIdx = 0
	m.settingsEditMode = ""
	m.settingsRecoveryOK = false
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
	case "recovery":
		return m.updateSettingsRecovery(msg)
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
			if m.settingsMenuIdx < len(settingsMenuItems())-1 {
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
				m.settingsInput1.Placeholder = i18n.T("tui.settings.placeholder.newTag")
				m.settingsInput1.EchoMode = textinput.EchoNormal
				m.settingsInput1.Blur()
				m.settingsMsg = ""
			case 1:
				m.settingsSubFlow = "passphrase"
				m.settingsInput1.Reset()
				m.settingsInput1.Placeholder = i18n.T("tui.settings.placeholder.currentPass")
				m.settingsInput1.EchoMode = textinput.EchoPassword
				m.settingsInput1.EchoCharacter = '·'
				m.settingsInput2.Reset()
				m.settingsInput2.Placeholder = i18n.T("tui.settings.placeholder.newPass")
				m.settingsInput2.EchoMode = textinput.EchoPassword
				m.settingsInput2.EchoCharacter = '·'
				m.settingsInput3.Reset()
				m.settingsInput3.Placeholder = i18n.T("tui.settings.placeholder.confirmNewPass")
				m.settingsInput3.EchoMode = textinput.EchoPassword
				m.settingsInput3.EchoCharacter = '·'
				m.settingsInput1.Focus()
				m.settingsMsg = ""
			case 2:
				// Export / import: show sub-menu; we reuse settingsSubFlow to
				// track which branch the user picks. Start with "export" prompt.
				m.settingsSubFlow = "export"
				m.settingsInput1.Reset()
				m.settingsInput1.Placeholder = i18n.T("tui.settings.placeholder.exportPath")
				m.settingsInput1.EchoMode = textinput.EchoNormal
				m.settingsInput2.Reset()
				m.settingsInput2.Placeholder = i18n.T("tui.settings.placeholder.exportPass")
				m.settingsInput2.EchoMode = textinput.EchoPassword
				m.settingsInput2.EchoCharacter = '·'
				m.settingsInput3.Reset()
				m.settingsInput3.Placeholder = i18n.T("tui.settings.placeholder.confirmPass")
				m.settingsInput3.EchoMode = textinput.EchoPassword
				m.settingsInput3.EchoCharacter = '·'
				m.settingsInput1.Focus()
				m.settingsMsg = ""
			case 3:
				m.settingsSubFlow = "config"
				m.settingsMsg = ""
				m.settingsEditMode = ""
				m.settingsInput1.Reset()
				m.settingsInput1.Blur()
			case 4:
				m.settingsSubFlow = "recovery"
				m.settingsInput1.Reset()
				m.settingsInput1.Placeholder = i18n.T("tui.settings.placeholder.recoveryKey")
				m.settingsInput1.EchoMode = textinput.EchoNormal
				m.settingsInput1.Focus()
				m.settingsMsg = ""
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
								m.settingsMsg = i18n.Tf("tui.settings.error.generic", map[string]any{"Err": err})
							} else {
								m.settingsMsg = i18n.Tf("tui.settings.tags.added", map[string]any{"Tag": tag})
								if m.builder != nil {
									if logErr := m.builder.LogEvent("credential-tag-update", cred.Label, cred.Issuer, audit.Hostname(), true); logErr != nil {
										_, _ = fmt.Fprintf(os.Stderr, "%s", i18n.Tf("tui.model.warn.auditFailed", map[string]any{"Err": logErr}))
									}
								}
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
					m.settingsMsg = i18n.Tf("tui.settings.error.generic", map[string]any{"Err": err})
				} else {
					m.settingsMsg = i18n.Tf("tui.settings.tags.removed", map[string]any{"Tag": removed})
					if m.builder != nil {
						if logErr := m.builder.LogEvent("credential-tag-update", cred.Label, cred.Issuer, audit.Hostname(), true); logErr != nil {
							_, _ = fmt.Fprintf(os.Stderr, "%s", i18n.Tf("tui.model.warn.auditFailed", map[string]any{"Err": logErr}))
						}
					}
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
// Input1 = current passphrase, Input2 = new passphrase, Input3 = confirm.
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
			m.settingsInput3.Reset()
			m.settingsInput3.Blur()
			m.settingsMsg = ""
			return m, nil

		case tea.KeyTab:
			if m.settingsInput1.Focused() {
				m.settingsInput1.Blur()
				m.settingsInput2.Focus()
			} else if m.settingsInput2.Focused() {
				m.settingsInput2.Blur()
				m.settingsInput3.Focus()
			} else {
				m.settingsInput3.Blur()
				m.settingsInput1.Focus()
			}
			return m, nil

		case tea.KeyEnter:
			current := m.settingsInput1.Value()
			newPP := m.settingsInput2.Value()
			confirm := m.settingsInput3.Value()

			if newPP != confirm {
				m.settingsMsg = i18n.T("tui.settings.error.passMismatch")
				return m, nil
			}
			if len(newPP) < 8 {
				m.settingsMsg = i18n.T("tui.settings.error.shortPass")
				return m, nil
			}
			if m.vaultMgr == nil {
				m.settingsMsg = i18n.T("tui.settings.error.vaultLocked")
				return m, nil
			}

			// Verify current passphrase.
			currentBytes := []byte(current)
			defer zeroBytes(currentBytes)
			verifier, err := vault.Open(m.vaultPath)
			if err != nil {
				m.settingsMsg = i18n.Tf("tui.settings.error.generic", map[string]any{"Err": err})
				return m, nil
			}
			if err := verifier.Unlock(currentBytes); err != nil {
				verifier.Close()
				m.settingsMsg = i18n.T("tui.settings.error.wrongPass")
				return m, nil
			}
			verifier.Close()

			pp := []byte(newPP)
			defer zeroBytes(pp)

			if err := m.vaultMgr.ChangePassphrase(pp); err != nil {
				m.settingsMsg = i18n.Tf("tui.settings.error.generic", map[string]any{"Err": err})
				return m, nil
			}
			if err := m.vaultMgr.Save(); err != nil {
				m.settingsMsg = i18n.Tf("tui.settings.error.saveError", map[string]any{"Err": err})
				return m, nil
			}

			if m.builder != nil {
				if logErr := m.builder.LogEvent("vault-passphrase-change", "", "", audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(os.Stderr, "%s", i18n.Tf("tui.model.warn.auditFailed", map[string]any{"Err": logErr}))
				}
			}

			m.settingsInput1.Reset()
			m.settingsInput1.Blur()
			m.settingsInput2.Reset()
			m.settingsInput2.Blur()
			m.settingsInput3.Reset()
			m.settingsInput3.Blur()
			m.settingsMsg = i18n.T("tui.settings.pass.success")
			m.settingsSubFlow = ""
			return m, nil
		}
	}

	var cmd tea.Cmd
	if m.settingsInput1.Focused() {
		m.settingsInput1, cmd = m.settingsInput1.Update(msg)
	} else if m.settingsInput2.Focused() {
		m.settingsInput2, cmd = m.settingsInput2.Update(msg)
	} else {
		m.settingsInput3, cmd = m.settingsInput3.Update(msg)
	}
	return m, cmd
}

// updateSettingsExport handles the export sub-flow.
// Input1 = file path, Input2 = export passphrase, Input3 = confirm passphrase.
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
			m.settingsInput3.Reset()
			m.settingsInput3.Blur()
			m.settingsMsg = ""
			return m, nil

		case tea.KeyTab:
			if m.settingsInput1.Focused() {
				m.settingsInput1.Blur()
				m.settingsInput2.Focus()
			} else if m.settingsInput2.Focused() {
				m.settingsInput2.Blur()
				m.settingsInput3.Focus()
			} else {
				m.settingsInput3.Blur()
				m.settingsInput1.Focus()
			}
			return m, nil

		// Switch between export and import.
		case tea.KeyF1:
			m.settingsSubFlow = "import"
			m.settingsInput1.Reset()
			m.settingsInput1.Placeholder = i18n.T("tui.settings.placeholder.importPath")
			m.settingsInput1.EchoMode = textinput.EchoNormal
			m.settingsInput2.Reset()
			m.settingsInput2.Placeholder = i18n.T("tui.settings.placeholder.importPass")
			m.settingsInput2.EchoMode = textinput.EchoPassword
			m.settingsInput2.EchoCharacter = '·'
			m.settingsInput2.Blur()
			m.settingsInput3.Reset()
			m.settingsInput3.Blur()
			m.settingsInput1.Focus()
			m.settingsMsg = ""
			return m, nil

		case tea.KeyEnter:
			path := m.settingsInput1.Value()
			passVal := m.settingsInput2.Value()
			confirmVal := m.settingsInput3.Value()

			if path == "" || passVal == "" {
				m.settingsMsg = i18n.T("tui.settings.error.exportRequired")
				return m, nil
			}
			if len(passVal) < 8 {
				m.settingsMsg = i18n.T("tui.settings.error.exportShortPass")
				return m, nil
			}
			if passVal != confirmVal {
				m.settingsMsg = i18n.T("tui.settings.error.exportMismatch")
				return m, nil
			}
			if m.vaultMgr == nil {
				m.settingsMsg = i18n.T("tui.settings.error.vaultLocked")
				return m, nil
			}

			pp := []byte(passVal)
			defer zeroBytes(pp)

			data, err := m.vaultMgr.ExportCredentials(pp)
			if err != nil {
				m.settingsMsg = i18n.Tf("tui.settings.error.exportFailed", map[string]any{"Err": err})
				return m, nil
			}
			defer zeroBytes(data)

			if err := os.WriteFile(path, data, 0600); err != nil {
				m.settingsMsg = i18n.Tf("tui.settings.error.writeFailed", map[string]any{"Err": err})
				return m, nil
			}

			if m.builder != nil {
				if logErr := m.builder.LogEvent("credential-export", "", "", audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(os.Stderr, "%s", i18n.Tf("tui.model.warn.auditFailed", map[string]any{"Err": logErr}))
				}
			}

			m.settingsInput1.Reset()
			m.settingsInput1.Blur()
			m.settingsInput2.Reset()
			m.settingsInput2.Blur()
			m.settingsInput3.Reset()
			m.settingsInput3.Blur()
			m.settingsMsg = i18n.Tf("tui.settings.export.success", map[string]any{"Path": path})
			m.settingsSubFlow = ""
			return m, nil
		}
	}

	var cmd tea.Cmd
	if m.settingsInput1.Focused() {
		m.settingsInput1, cmd = m.settingsInput1.Update(msg)
	} else if m.settingsInput2.Focused() {
		m.settingsInput2, cmd = m.settingsInput2.Update(msg)
	} else {
		m.settingsInput3, cmd = m.settingsInput3.Update(msg)
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
				m.settingsMsg = i18n.T("tui.settings.error.exportRequired")
				return m, nil
			}
			if m.vaultMgr == nil {
				m.settingsMsg = i18n.T("tui.settings.error.vaultLocked")
				return m, nil
			}

			const maxImportSize = 10 << 20
			info, err := os.Stat(path)
			if err != nil {
				m.settingsMsg = i18n.Tf("tui.settings.error.readFailed", map[string]any{"Err": err})
				return m, nil
			}
			if info.Size() > maxImportSize {
				m.settingsMsg = i18n.Tf("tui.settings.error.fileTooLarge", map[string]any{"Size": info.Size(), "Max": maxImportSize})
				return m, nil
			}

			data, err := os.ReadFile(path)
			if err != nil {
				m.settingsMsg = i18n.Tf("tui.settings.error.readFailed", map[string]any{"Err": err})
				return m, nil
			}

			pp := []byte(m.settingsInput2.Value())
			defer zeroBytes(pp)

			imported, skipped, err := m.vaultMgr.ImportCredentials(data, pp)
			if err != nil {
				m.settingsMsg = i18n.Tf("tui.settings.error.importFailed", map[string]any{"Err": err})
				return m, nil
			}

			if m.builder != nil && imported > 0 {
				if logErr := m.builder.LogEvent("credential-import", "", "", audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(os.Stderr, "%s", i18n.Tf("tui.model.warn.auditFailed", map[string]any{"Err": logErr}))
				}
			}

			if err := m.vaultMgr.Save(); err != nil {
				m.settingsMsg = i18n.Tf("tui.settings.error.saveError", map[string]any{"Err": err})
				return m, nil
			}

			m = refreshCredList(m)
			m.settingsInput1.Reset()
			m.settingsInput1.Blur()
			m.settingsInput2.Reset()
			m.settingsInput2.Blur()
			m.settingsMsg = i18n.Tf("tui.settings.import.success", map[string]any{"Imported": imported, "Skipped": skipped})
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
					m.settingsMsg = i18n.T("tui.settings.error.invalidSeconds")
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
					m.settingsMsg = i18n.Tf("tui.settings.error.saveError", map[string]any{"Err": err})
				} else {
					m.settingsMsg = i18n.T("tui.settings.config.saved")
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
			m.settingsInput1.Placeholder = i18n.Tf("tui.settings.placeholder.seconds", map[string]any{"Current": int(m.cfg.ClipboardTimeout.Seconds())})
			m.settingsInput1.EchoMode = textinput.EchoNormal
			m.settingsInput1.Focus()
			return m, nil

		case len(msg.Runes) == 1 && msg.Runes[0] == 'i':
			m.settingsEditMode = "idle"
			m.settingsInput1.Reset()
			m.settingsInput1.Placeholder = i18n.Tf("tui.settings.placeholder.seconds", map[string]any{"Current": int(m.cfg.IdleTimeout.Seconds())})
			m.settingsInput1.EchoMode = textinput.EchoNormal
			m.settingsInput1.Focus()
			return m, nil

		case len(msg.Runes) == 1 && msg.Runes[0] == 'a':
			if !m.cfg.Audit.Enabled {
				return m, nil // no-op when audit not configured
			}
			m.cfg.Audit.AutoStart = !m.cfg.Audit.AutoStart
			dir := filepath.Dir(m.vaultPath)
			if err := config.WriteAuditSection(dir, m.cfg.Audit); err != nil {
				m.settingsMsg = fmt.Sprintf("Could not save audit setting: %v", err)
			} else {
				label := i18n.T("tui.settings.autoStart.disabled")
				if m.cfg.Audit.AutoStart {
					label = i18n.T("tui.settings.autoStart.enabled")
				}
				m.settingsMsg = i18n.Tf("tui.settings.autoStart.toggled", map[string]any{"State": label})
			}
			return m, nil

		case len(msg.Runes) == 1 && msg.Runes[0] == 'l':
			// Cycle to the next supported language, save to config, re-init i18n.
			next := nextLanguage(m.cfg.Language)
			m.cfg.Language = next
			if err := config.WriteLanguage(filepath.Dir(m.vaultPath), next); err != nil {
				m.settingsMsg = i18n.Tf("tui.settings.error.saveError", map[string]any{"Err": err})
			} else {
				i18n.Init(next)
				m.settingsMsg = i18n.Tf("tui.settings.lang.toggled", map[string]any{"Lang": next})
			}
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
		content = m.viewSettingsExport()
	case "import":
		content = m.viewSettingsImport()
	case "config":
		content = m.viewSettingsConfig()
	case "recovery":
		content = m.viewSettingsRecovery()
	default:
		content = m.viewSettingsMenu()
	}

	overlay := overlayBoxStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay)
}

// viewSettingsMenu renders the top-level settings menu.
func (m model) viewSettingsMenu() string {
	var lines []string
	lines = append(lines, titleStyle.Render(i18n.T("tui.settings.title.main")))
	lines = append(lines, "")
	for i, item := range settingsMenuItems() {
		if i == m.settingsMenuIdx {
			lines = append(lines, tipStyle.Render("> "+item))
		} else {
			lines = append(lines, "  "+item)
		}
	}
	if m.settingsMsg != "" {
		lines = append(lines, "")
		lines = append(lines, tipStyle.Render(m.settingsMsg))
	}
	lines = append(lines, "")
	lines = append(lines, helpBarStyle.Render(i18n.T("tui.settings.helpBar.main")))
	return strings.Join(lines, "\n")
}

// viewSettingsTags renders the tag management sub-flow.
func (m model) viewSettingsTags() string {
	var lines []string
	lines = append(lines, titleStyle.Render(i18n.T("tui.settings.title.tags")))
	lines = append(lines, "")

	var tags []string
	if selected := m.credList.SelectedItem(); selected != nil {
		if item, ok := selected.(credItem); ok {
			tags = item.cred.Tags
		}
	}

	if len(tags) == 0 {
		lines = append(lines, "  "+i18n.T("tui.settings.tags.empty"))
	} else {
		for i, tag := range tags {
			if i == m.settingsTagIdx {
				lines = append(lines, tipStyle.Render("> "+tag))
			} else {
				lines = append(lines, "  "+tag)
			}
		}
	}

	lines = append(lines, "")
	if m.settingsInput1.Focused() {
		lines = append(lines, i18n.T("tui.settings.tags.newLabel")+m.settingsInput1.View())
	}

	if m.settingsMsg != "" {
		lines = append(lines, "")
		lines = append(lines, tipStyle.Render(m.settingsMsg))
	}
	lines = append(lines, "")
	lines = append(lines, helpBarStyle.Render(i18n.T("tui.settings.helpBar.tags")))
	return strings.Join(lines, "\n")
}

// viewSettingsPassphrase renders the change-passphrase sub-flow.
func (m model) viewSettingsPassphrase() string {
	currentLabel := i18n.T("tui.settings.field.currentPass")
	newLabel     := i18n.T("tui.settings.field.newPass")
	confirmLabel := i18n.T("tui.settings.field.confirmPass")

	col := lipgloss.Width(currentLabel)
	for _, l := range []string{newLabel, confirmLabel} {
		if w := lipgloss.Width(l); w > col {
			col = w
		}
	}
	col += 1
	pad := func(label string) string {
		return label + strings.Repeat(" ", col-lipgloss.Width(label))
	}

	var lines []string
	lines = append(lines, titleStyle.Render(i18n.T("tui.settings.title.passphrase")))
	lines = append(lines, "")
	lines = append(lines, pad(currentLabel)+m.settingsInput1.View())
	lines = append(lines, pad(newLabel)+m.settingsInput2.View())
	if newPP := m.settingsInput2.Value(); len(newPP) >= 8 {
		ppBytes := []byte(newPP)
		lines = append(lines, strings.Repeat(" ", col)+tuiStrengthLabel(ppBytes))
		zeroBytes(ppBytes)
	}
	lines = append(lines, pad(confirmLabel)+m.settingsInput3.View())
	if m.settingsMsg != "" {
		lines = append(lines, "")
		lines = append(lines, errorStyle.Render(m.settingsMsg))
	}
	lines = append(lines, "")
	lines = append(lines, helpBarStyle.Render(i18n.T("tui.settings.helpBar.passphrase")))
	return strings.Join(lines, "\n")
}

// tuiStrengthLabel returns a strength label for the passphrase, using the
// shared strengthLevel scoring to stay in sync with the CLI meter.
func tuiStrengthLabel(pass []byte) string {
	bar, label := strengthLevel(pass)
	return bar + " " + label
}

// viewSettingsExport renders the export sub-flow with passphrase confirmation
// and strength meter.
func (m model) viewSettingsExport() string {
	pathLabel    := i18n.T("tui.settings.field.exportPathLabel")
	passLabel    := i18n.T("tui.settings.field.exportPassLabel")
	confirmLabel := i18n.T("tui.settings.field.confirmPassLabel")

	col := lipgloss.Width(pathLabel)
	for _, l := range []string{passLabel, confirmLabel} {
		if w := lipgloss.Width(l); w > col {
			col = w
		}
	}
	col += 1
	pad := func(label string) string {
		return label + strings.Repeat(" ", col-lipgloss.Width(label))
	}

	var lines []string
	lines = append(lines, titleStyle.Render(i18n.T("tui.settings.title.export")))
	lines = append(lines, "")
	lines = append(lines, pad(pathLabel)+m.settingsInput1.View())
	lines = append(lines, pad(passLabel)+m.settingsInput2.View())
	if pp := m.settingsInput2.Value(); len(pp) >= 8 {
		ppBytes := []byte(pp)
		lines = append(lines, strings.Repeat(" ", col)+tuiStrengthLabel(ppBytes))
		zeroBytes(ppBytes)
	}
	lines = append(lines, pad(confirmLabel)+m.settingsInput3.View())
	if m.settingsMsg != "" {
		lines = append(lines, "")
		lines = append(lines, errorStyle.Render(m.settingsMsg))
	}
	lines = append(lines, "")
	lines = append(lines, helpBarStyle.Render(i18n.T("tui.settings.helpBar.export")))
	return strings.Join(lines, "\n")
}

// viewSettingsImport renders the import sub-flow.
func (m model) viewSettingsImport() string {
	pathLabel := i18n.T("tui.settings.field.importPathLabel")
	passLabel := i18n.T("tui.settings.field.importPassLabel")

	col := lipgloss.Width(pathLabel)
	if w := lipgloss.Width(passLabel); w > col {
		col = w
	}
	col += 1
	pad := func(label string) string {
		return label + strings.Repeat(" ", col-lipgloss.Width(label))
	}

	var lines []string
	lines = append(lines, titleStyle.Render(i18n.T("tui.settings.title.import")))
	lines = append(lines, "")
	lines = append(lines, pad(pathLabel)+m.settingsInput1.View())
	lines = append(lines, pad(passLabel)+m.settingsInput2.View())
	if m.settingsMsg != "" {
		lines = append(lines, "")
		lines = append(lines, tipStyle.Render(m.settingsMsg))
	}
	lines = append(lines, "")
	lines = append(lines, helpBarStyle.Render(i18n.T("tui.settings.helpBar.import")))
	return strings.Join(lines, "\n")
}

// updateSettingsRecovery handles the recovery key verification sub-flow.
func (m model) updateSettingsRecovery(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.settingsSubFlow = ""
			m.settingsInput1.Reset()
			m.settingsInput1.Blur()
			m.settingsMsg = ""
			return m, nil

		case tea.KeyEnter:
			keyStr := strings.TrimSpace(m.settingsInput1.Value())
			if keyStr == "" {
				m.settingsMsg = i18n.T("tui.settings.error.recoveryKeyRequired")
				return m, nil
			}
			rawKey, err := decodeBase32Secret(keyStr)
			if err != nil {
				m.settingsMsg = i18n.T("tui.settings.error.recoveryKeyDecode")
				return m, nil
			}
			defer zeroBytes(rawKey)

			if m.vaultMgr == nil {
				m.settingsMsg = i18n.T("tui.settings.error.vaultLocked")
				return m, nil
			}
			ok, err := m.vaultMgr.VerifyRecoveryKey(rawKey)
			if err != nil {
				m.settingsMsg = i18n.Tf("tui.settings.error.generic", map[string]any{"Err": err})
				return m, nil
			}
			m.settingsRecoveryOK = ok
			if ok {
				m.settingsMsg = i18n.T("tui.settings.recoveryKey.valid")
			} else {
				m.settingsMsg = i18n.T("tui.settings.recoveryKey.invalid")
			}
			m.settingsInput1.Reset()
			m.settingsInput1.Blur()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.settingsInput1, cmd = m.settingsInput1.Update(msg)
	return m, cmd
}

// viewSettingsRecovery renders the recovery key verification sub-flow.
func (m model) viewSettingsRecovery() string {
	var lines []string
	lines = append(lines, titleStyle.Render(i18n.T("tui.settings.title.verifyRecovery")))
	lines = append(lines, "")
	lines = append(lines, i18n.T("tui.settings.field.recoveryKey")+m.settingsInput1.View())
	if m.settingsMsg != "" {
		lines = append(lines, "")
		if m.settingsRecoveryOK {
			lines = append(lines, tipStyle.Render(m.settingsMsg))
		} else {
			lines = append(lines, errorStyle.Render(m.settingsMsg))
		}
	}
	lines = append(lines, "")
	lines = append(lines, helpBarStyle.Render(i18n.T("tui.settings.helpBar.verifyRecovery")))
	return strings.Join(lines, "\n")
}

// viewSettingsConfig renders the config settings sub-flow.
func (m model) viewSettingsConfig() string {
	// Compute column width from the widest translated label so values align
	// correctly in any language (Japanese labels are wider than English ones).
	langLabel     := i18n.T("tui.settings.config.langLabel")
	clipLabel     := i18n.T("tui.settings.config.clipboardLabel")
	idleLabel     := i18n.T("tui.settings.config.idleLabel")
	autoLabel     := i18n.T("tui.settings.config.autoStartLabel")

	col := lipgloss.Width(langLabel)
	for _, l := range []string{clipLabel, idleLabel, autoLabel} {
		if w := lipgloss.Width(l); w > col {
			col = w
		}
	}
	col += 2 // two-space gap between label and value

	pad := func(label string) string {
		return label + strings.Repeat(" ", col-lipgloss.Width(label))
	}

	clipSec := int(m.cfg.ClipboardTimeout.Seconds())
	idleSec := int(m.cfg.IdleTimeout.Seconds())

	var lines []string
	lines = append(lines, titleStyle.Render(i18n.T("tui.settings.title.config")))
	lines = append(lines, "")
	lines = append(lines, pad(langLabel)+m.cfg.Language+"  "+helpBarStyle.Render(i18n.T("tui.settings.config.langHint")))
	lines = append(lines, fmt.Sprintf("%s%ds  %s", pad(clipLabel), clipSec, helpBarStyle.Render(i18n.T("tui.settings.config.clipboardHint"))))
	lines = append(lines, fmt.Sprintf("%s%ds  %s", pad(idleLabel), idleSec, helpBarStyle.Render(i18n.T("tui.settings.config.idleHint"))))
	if m.cfg.Audit.Enabled {
		autoStartVal := i18n.T("tui.settings.autoStart.disabled")
		if m.cfg.Audit.AutoStart {
			autoStartVal = i18n.T("tui.settings.autoStart.enabled")
		}
		lines = append(lines, fmt.Sprintf("%s%s  %s", pad(autoLabel), autoStartVal, helpBarStyle.Render(i18n.T("tui.settings.config.autoStartHint"))))
	}
	if m.settingsInput1.Focused() {
		lines = append(lines, "")
		lines = append(lines, m.settingsInput1.View())
	}
	if m.settingsMsg != "" {
		lines = append(lines, "")
		lines = append(lines, tipStyle.Render(m.settingsMsg))
	}
	lines = append(lines, "")
	helpText := i18n.T("tui.settings.helpBar.config")
	if m.cfg.Audit.Enabled {
		helpText = i18n.T("tui.settings.helpBar.configAudit")
	}
	lines = append(lines, helpBarStyle.Render(helpText))
	return strings.Join(lines, "\n")
}

// writeConfigFile writes the effective clipboard and idle timeouts to tegata.toml,
// preserving any other existing sections such as [audit].
func writeConfigFile(vaultPath string, cfg config.Config) error {
	dir := filepath.Dir(vaultPath)
	return config.WriteClipboardVaultSections(dir,
		int(cfg.ClipboardTimeout.Seconds()),
		int(cfg.IdleTimeout.Seconds()))
}

// secondsDuration converts an integer number of seconds to a time.Duration.
func secondsDuration(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}

// nextLanguage cycles through supported languages, returning the one after current.
func nextLanguage(current string) string {
	return i18n.NextLanguage(current)
}
