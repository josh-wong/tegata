package main

import (
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/josh-wong/tegata/internal/crypto"
	"github.com/josh-wong/tegata/internal/vault"
)

// createVaultResultMsg is returned by createVaultCmd when the async vault
// creation goroutine completes.
type createVaultResultMsg struct {
	recoveryKey string
	err         error
}

// createVaultCmd spawns an async tea.Cmd that calls vault.Create. The
// Argon2id derivation inside Create blocks for ~1-3s, so it must run off
// the event loop. The caller must zero the passphrase slice after this call.
func createVaultCmd(path string, passphrase []byte) tea.Cmd {
	return func() tea.Msg {
		recoveryKey, err := vault.Create(path, passphrase, crypto.DefaultParams)
		// Zero the local copy of passphrase bytes passed to this closure.
		for i := range passphrase {
			passphrase[i] = 0
		}
		return createVaultResultMsg{recoveryKey: recoveryKey, err: err}
	}
}

// vaultCreatePath returns the vault file path to use when creating a new vault
// during the setup wizard. If the model already has a vaultPath, it is used
// as-is. Otherwise, the vault is created as vault.tegata in the current working
// directory.
func (m model) vaultCreatePath() string {
	if m.vaultPath != "" {
		return m.vaultPath
	}
	cwd, _ := filepath.Abs(".")
	return filepath.Join(cwd, vaultFilename)
}

// updateWizard delegates Update messages to the correct wizard step handler.
func (m model) updateWizard(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateWizardWelcome:
		return m.updateWizardWelcome(msg)
	case stateWizardPassphrase:
		return m.updateWizardPassphrase(msg)
	case stateWizardRecoveryKey:
		return m.updateWizardRecoveryKey(msg)
	case stateWizardAddCredential:
		return m.updateWizardAddCredential(msg)
	}
	return m, nil
}

// updateWizardWelcome handles input on the welcome screen (step 1/4).
func (m model) updateWizardWelcome(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyEnter {
			m.state = stateWizardPassphrase
			m.passphraseInput.Focus()
			return m, m.spinner.Tick
		}
	}
	return m, nil
}

// updateWizardPassphrase handles input on the passphrase entry screen (step 2/4).
func (m model) updateWizardPassphrase(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.creating {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case createVaultResultMsg:
		m.creating = false
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("Error creating vault: %v", msg.err)
			return m, nil
		}
		m.recoveryKey = msg.recoveryKey
		m.errMsg = ""
		m.state = stateWizardRecoveryKey
		return m, nil

	case tea.KeyMsg:
		if m.creating {
			return m, nil // ignore input while creating
		}

		switch msg.Type {
		case tea.KeyTab:
			// Toggle focus between the two passphrase inputs.
			if m.passphraseInput.Focused() {
				m.passphraseInput.Blur()
				m.confirmInput.Focus()
			} else {
				m.confirmInput.Blur()
				m.passphraseInput.Focus()
			}
			return m, nil

		case tea.KeyEnter:
			if m.confirmInput.Focused() {
				pass := m.passphraseInput.Value()
				confirm := m.confirmInput.Value()
				if len(pass) < 8 {
					m.errMsg = "Passphrase must be at least 8 characters."
					return m, nil
				}
				if pass != confirm {
					m.errMsg = "Passphrases do not match."
					return m, nil
				}
				m.errMsg = ""

				// Copy passphrase to a byte slice that the async command owns.
				// The async command is responsible for zeroing this copy.
				pp := []byte(pass)

				// Reset the textinput fields immediately.
				m.passphraseInput.Reset()
				m.confirmInput.Reset()

				m.creating = true
				path := m.vaultCreatePath()
				return m, tea.Batch(m.spinner.Tick, createVaultCmd(path, pp))
			}
			return m, nil
		}

		// Delegate typing to the focused input.
		var cmd tea.Cmd
		if m.passphraseInput.Focused() {
			m.passphraseInput, cmd = m.passphraseInput.Update(msg)
		} else {
			m.confirmInput, cmd = m.confirmInput.Update(msg)
		}
		return m, cmd
	}

	// For non-key messages, update whichever input is focused.
	var cmd tea.Cmd
	if m.passphraseInput.Focused() {
		m.passphraseInput, cmd = m.passphraseInput.Update(msg)
	} else {
		m.confirmInput, cmd = m.confirmInput.Update(msg)
	}
	return m, cmd
}

// updateWizardRecoveryKey handles input on the recovery key display screen (step 3/4).
func (m model) updateWizardRecoveryKey(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyEnter {
			m.state = stateWizardAddCredential
		}
	}
	return m, nil
}

// updateWizardAddCredential handles input on the optional first-credential
// screen (step 4/4). Esc skips directly to the main view.
func (m model) updateWizardAddCredential(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.state = stateMainView
		case tea.KeyEnter:
			// Full add overlay is Plan 04; for now skip is fine.
			m.state = stateMainView
		}
	}
	return m, nil
}

// viewWizard composes the correct wizard screen view.
func (m model) viewWizard() string {
	switch m.state {
	case stateWizardWelcome:
		return m.viewWizardWelcome()
	case stateWizardPassphrase:
		return m.viewWizardPassphrase()
	case stateWizardRecoveryKey:
		return m.viewWizardRecoveryKey()
	case stateWizardAddCredential:
		return m.viewWizardAddCredential()
	}
	return ""
}

// viewWizardWelcome renders step 1/4.
func (m model) viewWizardWelcome() string {
	content := titleStyle.Render("Welcome to Tegata") + "\n\n" +
		"Tegata is a portable authenticator that stores encrypted\n" +
		"credentials on USB drives or microSD cards.\n\n" +
		"This wizard will guide you through creating your vault.\n\n" +
		helpBarStyle.Render("[Enter] Begin setup  [q] Quit")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

// viewWizardPassphrase renders step 2/4.
func (m model) viewWizardPassphrase() string {
	strength := strengthLabel(len(m.passphraseInput.Value()))
	content := titleStyle.Render("Step 2/4: Set passphrase") + "\n\n" +
		m.passphraseInput.View() + "\n" +
		m.confirmInput.View() + "\n\n" +
		"Strength: " + strength + "\n"

	if m.errMsg != "" {
		content += "\n" + errorStyle.Render(m.errMsg) + "\n"
	}
	if m.creating {
		content += "\n" + m.spinner.View() + " Creating vault…\n"
	} else {
		content += "\n" + helpBarStyle.Render("[Tab] Switch field  [Enter] Confirm  [q] Quit")
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

// viewWizardRecoveryKey renders step 3/4.
func (m model) viewWizardRecoveryKey() string {
	keyBox := overlayBoxStyle.Render(m.recoveryKey)
	content := titleStyle.Render("Step 3/4: Recovery key") + "\n\n" +
		keyBox + "\n\n" +
		errorStyle.Render("Write this down. You cannot recover your vault without it.") + "\n\n" +
		helpBarStyle.Render("[Enter] I have stored my recovery key")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

// viewWizardAddCredential renders step 4/4.
func (m model) viewWizardAddCredential() string {
	content := titleStyle.Render("Step 4/4: Add your first credential") + "\n\n" +
		"Add a TOTP, HOTP, challenge-response, or static password credential\n" +
		"to get started. You can add more credentials later from the main view.\n\n" +
		helpBarStyle.Render("[Enter] Add credential  [Esc] Skip")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

// strengthLabel returns a human-readable passphrase strength label based on
// character count. Informational only; no minimum is enforced here.
func strengthLabel(length int) string {
	switch {
	case length >= 20:
		return successStyle.Render("Strong")
	case length >= 12:
		return "Fair"
	default:
		return errorStyle.Render("Weak")
	}
}
