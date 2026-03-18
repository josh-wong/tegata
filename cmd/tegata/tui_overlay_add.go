package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/josh-wong/tegata/internal/auth"
	pkgmodel "github.com/josh-wong/tegata/pkg/model"
)

// credTypeNames maps addTypeIdx to display labels and CredentialType constants.
var credTypeNames = []struct {
	label string
	ctype pkgmodel.CredentialType
}{
	{"TOTP", pkgmodel.CredentialTOTP},
	{"HOTP", pkgmodel.CredentialHOTP},
	{"Static", pkgmodel.CredentialStatic},
	{"Challenge-Resp", pkgmodel.CredentialChallengeResponse},
}

// resetAddOverlay clears all add-overlay input fields and resets indices.
func (m *model) resetAddOverlay() {
	m.addLabelInput.Reset()
	m.addLabelInput.Blur()
	m.addIssuerInput.Reset()
	m.addIssuerInput.Blur()
	m.addSecretInput.Reset()
	m.addSecretInput.Blur()
	m.addTypeIdx = 0
	m.addFocusIdx = 0
	m.errMsg = ""
}

// focusAddInput blurs all add inputs, then focuses the one at addFocusIdx.
func (m *model) focusAddInput() {
	m.addLabelInput.Blur()
	m.addIssuerInput.Blur()
	m.addSecretInput.Blur()
	switch m.addFocusIdx {
	case 0:
		m.addLabelInput.Focus()
	case 1:
		m.addIssuerInput.Focus()
	case 2:
		m.addSecretInput.Focus()
	}
}

// updateOverlayAdd handles key events in stateOverlayAdd.
func (m model) updateOverlayAdd(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyEsc:
			m.resetAddOverlay()
			m.state = stateMainView
			return m, nil

		case msg.Type == tea.KeyTab:
			m.addFocusIdx = (m.addFocusIdx + 1) % 3
			m.focusAddInput()
			return m, nil

		case len(msg.Runes) == 1 && msg.Runes[0] >= '1' && msg.Runes[0] <= '4':
			if !m.addLabelInput.Focused() && !m.addIssuerInput.Focused() && !m.addSecretInput.Focused() {
				m.addTypeIdx = int(msg.Runes[0]-'1')
				return m, nil
			}
			// Digit is being typed into an active text field — fall through to delegate.

		case msg.Type == tea.KeyEnter:
			labelVal := m.addLabelInput.Value()

			// URI auto-populate: if label starts with "otpauth://", parse and fill fields.
			if strings.HasPrefix(labelVal, "otpauth://") {
				cred, err := auth.ParseOTPAuthURI(labelVal)
				if err != nil {
					m.errMsg = fmt.Sprintf("Invalid URI: %v", err)
					return m, nil
				}
				m.addLabelInput.SetValue(cred.Label)
				m.addIssuerInput.SetValue(cred.Issuer)
				m.addSecretInput.SetValue(cred.Secret)
				for i, ct := range credTypeNames {
					if ct.ctype == cred.Type {
						m.addTypeIdx = i
						break
					}
				}
				m.errMsg = ""
				return m, nil
			}

			// Validate required fields.
			if labelVal == "" || m.addSecretInput.Value() == "" {
				m.errMsg = "Label and secret are required"
				return m, nil
			}

			// Build credential from inputs.
			ct := credTypeNames[m.addTypeIdx]
			cred := pkgmodel.Credential{
				Label:     labelVal,
				Issuer:    m.addIssuerInput.Value(),
				Type:      ct.ctype,
				Secret:    m.addSecretInput.Value(),
				Algorithm: "SHA1",
				Digits:    6,
				Period:    30,
			}

			if m.vaultMgr == nil {
				m.errMsg = "Vault not unlocked"
				return m, nil
			}

			if _, err := m.vaultMgr.AddCredential(cred); err != nil {
				m.errMsg = fmt.Sprintf("Add failed: %v", err)
				return m, nil
			}

			// Refresh credential list.
			m = refreshCredList(m)

			label := labelVal
			m.resetAddOverlay()
			m.state = stateMainView
			m.statusMsg = fmt.Sprintf("Added %q", label)
			return m, nil
		}
	}

	// Delegate to the focused text input.
	var cmd tea.Cmd
	switch m.addFocusIdx {
	case 0:
		m.addLabelInput, cmd = m.addLabelInput.Update(msg)
	case 1:
		m.addIssuerInput, cmd = m.addIssuerInput.Update(msg)
	case 2:
		m.addSecretInput, cmd = m.addSecretInput.Update(msg)
	}
	return m, cmd
}

// viewOverlayAdd renders the add-credential overlay.
func (m model) viewOverlayAdd() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Add Credential"))
	lines = append(lines, "")
	lines = append(lines, "Label:   "+m.addLabelInput.View())
	lines = append(lines, "Issuer:  "+m.addIssuerInput.View()+" (optional)")
	lines = append(lines, "Secret:  "+m.addSecretInput.View())
	lines = append(lines, "")

	// Type selector row.
	var typeRow []string
	for i, ct := range credTypeNames {
		key := fmt.Sprintf("[%d]", i+1)
		if m.addTypeIdx == i {
			typeRow = append(typeRow, successStyle.Render(key+" "+ct.label))
		} else {
			typeRow = append(typeRow, key+" "+ct.label)
		}
	}
	lines = append(lines, "Type:    "+strings.Join(typeRow, "  "))

	if m.errMsg != "" {
		lines = append(lines, "")
		lines = append(lines, errorStyle.Render(m.errMsg))
	}

	lines = append(lines, "")
	lines = append(lines, helpBarStyle.Render("[Tab] Next field  [Enter] Save  [Esc] Cancel"))

	content := strings.Join(lines, "\n")
	overlay := overlayBoxStyle.Render(content)
	bg := m.viewMainView()
	return overlayOnBackground(bg, overlay, m.width, m.height)
}

// updateOverlayRemove handles key events in stateOverlayRemove.
func (m model) updateOverlayRemove(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyEsc ||
			(len(msg.Runes) == 1 && msg.Runes[0] == 'n'):
			m.state = stateMainView
			return m, nil

		case msg.Type == tea.KeyEnter ||
			(len(msg.Runes) == 1 && msg.Runes[0] == 'y'):
			selected := m.credList.SelectedItem()
			if selected == nil {
				m.state = stateMainView
				return m, nil
			}
			item, ok := selected.(credItem)
			if !ok {
				m.state = stateMainView
				return m, nil
			}

			if m.vaultMgr != nil {
				if err := m.vaultMgr.RemoveCredential(item.cred.ID); err != nil {
					m.errMsg = fmt.Sprintf("Remove failed: %v", err)
					m.state = stateMainView
					return m, nil
				}
			}

			m = refreshCredList(m)
			m.state = stateMainView
			m.statusMsg = "Removed"
			return m, nil
		}
	}
	return m, nil
}

// viewOverlayRemove renders the remove-confirmation overlay.
func (m model) viewOverlayRemove() string {
	label := "(none selected)"
	if selected := m.credList.SelectedItem(); selected != nil {
		if item, ok := selected.(credItem); ok {
			label = item.cred.Label
		}
	}

	var lines []string
	lines = append(lines, titleStyle.Render("Remove credential?"))
	lines = append(lines, "")
	lines = append(lines, "Credential: "+label)
	lines = append(lines, "")
	lines = append(lines, helpBarStyle.Render("[y] Remove  [n] Cancel"))

	content := strings.Join(lines, "\n")
	overlay := overlayBoxStyle.Render(content)
	bg := m.viewMainView()
	return overlayOnBackground(bg, overlay, m.width, m.height)
}

// refreshCredList rebuilds the credential list from the vault manager.
func refreshCredList(m model) model {
	if m.vaultMgr == nil {
		return m
	}
	creds := m.vaultMgr.ListCredentials()
	items := make([]list.Item, 0, len(creds))
	for _, c := range creds {
		items = append(items, credItem{cred: c})
	}
	m.credList.SetItems(items)
	if len(creds) > 0 {
		m.credList.Title = fmt.Sprintf("%d credentials", len(creds))
	} else {
		m.credList.Title = "No credentials"
	}
	m.cursor = 0
	return m
}

// overlayOnBackground places an overlay box centered on top of the background.
func overlayOnBackground(bg, overlay string, width, height int) string {
	_ = bg // background is rendered behind; lipgloss.Place handles centering
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, overlay)
}
