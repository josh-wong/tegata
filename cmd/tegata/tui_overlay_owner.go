package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/josh-wong/tegata/internal/audit"
	tegerrors "github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/internal/i18n"
)

// ownerClearToken is the value a user types in an owner identifier field to
// clear that identifier. A blank field keeps the current value.
const ownerClearToken = "-"

// ownerFieldInput converts a TUI owner field value into the pointer form used
// by audit.UpdateOwner: nil keeps the current hash, "" clears it, and any
// other value replaces it.
func ownerFieldInput(v string) *string {
	v = strings.TrimSpace(v)
	switch v {
	case "":
		return nil
	case ownerClearToken:
		empty := ""
		return &empty
	default:
		return &v
	}
}

// updateSettingsOwner handles the owner identifier sub-flow.
// Input1 = employee ID, Input2 = email address.
func (m model) updateSettingsOwner(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.Type {
		case tea.KeyEsc:
			m.settingsSubFlow = ""
			m.settingsInput1.Reset()
			m.settingsInput1.Blur()
			m.settingsInput2.Reset()
			m.settingsInput2.Blur()
			m.settingsMsg, m.settingsMsgOK = "", false
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
			if m.vaultMgr == nil {
				m.settingsMsg, m.settingsMsgOK = i18n.T("tui.settings.error.vaultLocked"), false
				return m, nil
			}
			// With an unreadable tegata.toml the organization salt is unknown, so
			// a save would hash unsalted and never match the organization's hashes.
			if m.configLoadFailed {
				m.settingsMsg, m.settingsMsgOK = i18n.T("tui.settings.owner.configUnreadable"), false
				return m, nil
			}
			idIn := ownerFieldInput(m.settingsInput1.Value())
			emailIn := ownerFieldInput(m.settingsInput2.Value())
			if idIn == nil && emailIn == nil {
				m.settingsMsg, m.settingsMsgOK = i18n.T("tui.settings.owner.unchanged"), true
				return m, nil
			}
			next, changed, err := audit.UpdateOwner(m.vaultMgr, m.builder, idIn, emailIn, m.cfg.Organization.IdentifierSalt)
			// Clear the inputs on every path so raw identifiers do not linger
			// in the text input buffers.
			m.settingsInput1.Reset()
			m.settingsInput2.Reset()
			// Return focus to the first field so the next edit starts from a
			// predictable place.
			m.settingsInput2.Blur()
			m.settingsInput1.Focus()
			if err != nil {
				if tegerrors.Is(err, audit.ErrOwnerSaltChanged) {
					m.settingsMsg, m.settingsMsgOK = i18n.T("tui.settings.owner.saltChanged"), false
				} else {
					m.settingsMsg, m.settingsMsgOK = i18n.Tf("tui.settings.owner.saveFailed", map[string]any{"Err": ownerErrorMessage(err)}), false
				}
				return m, nil
			}
			if !changed {
				m.settingsMsg, m.settingsMsgOK = i18n.T("tui.settings.owner.unchanged"), true
				return m, nil
			}
			audit.ApplyOwner(&m.cfg.Audit, next)
			m.settingsMsg, m.settingsMsgOK = i18n.T("tui.settings.owner.updated"), true
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

// viewSettingsOwner renders the owner identifier sub-flow. Only short hash
// prefixes of the stored identifiers are shown.
func (m model) viewSettingsOwner() string {
	idLabel := i18n.T("tui.settings.field.ownerEmployeeID")
	emailLabel := i18n.T("tui.settings.field.ownerEmail")
	col := lipgloss.Width(idLabel)
	if w := lipgloss.Width(emailLabel); w > col {
		col = w
	}
	col++
	pad := func(label string) string {
		return label + strings.Repeat(" ", col-lipgloss.Width(label))
	}

	notSet := i18n.T("cmd.owner.show.notSet")
	current := func(h string) string {
		if h == "" {
			return notSet
		}
		return audit.ShortOwnerHash(h)
	}
	var idHash, emailHash string
	if m.vaultMgr != nil {
		if o := m.vaultMgr.Owner(); o != nil {
			idHash, emailHash = o.EmployeeIDHash, o.EmailHash
		}
	}

	var lines []string
	lines = append(lines, titleStyle.Render(i18n.T("tui.settings.title.owner")))
	lines = append(lines, "")
	lines = append(lines, tipStyle.Render(i18n.T("tui.settings.owner.intro")))
	lines = append(lines, "")
	// Show the "> " prompt only on the focused field so the user can tell
	// which field receives input, including when color is disabled.
	in1, in2 := m.settingsInput1, m.settingsInput2
	if !in1.Focused() {
		in1.Prompt = strings.Repeat(" ", lipgloss.Width(in1.Prompt))
	}
	if !in2.Focused() {
		in2.Prompt = strings.Repeat(" ", lipgloss.Width(in2.Prompt))
	}
	lines = append(lines, pad(idLabel)+helpBarStyle.Render(current(idHash)))
	lines = append(lines, strings.Repeat(" ", col)+in1.View())
	lines = append(lines, pad(emailLabel)+helpBarStyle.Render(current(emailHash)))
	lines = append(lines, strings.Repeat(" ", col)+in2.View())
	if m.settingsMsg != "" {
		lines = append(lines, "")
		if m.settingsMsgOK {
			lines = append(lines, tipStyle.Render(m.settingsMsg))
		} else {
			lines = append(lines, errorStyle.Render(m.settingsMsg))
		}
	}
	lines = append(lines, "")
	lines = append(lines, helpBarStyle.Render(i18n.T("tui.settings.helpBar.owner")))
	return strings.Join(lines, "\n")
}
