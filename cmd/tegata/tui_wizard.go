package main

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/josh-wong/tegata/internal/crypto"
	"github.com/josh-wong/tegata/internal/vault"
)

// createVaultResultMsg is returned by createVaultCmd when the async vault
// creation goroutine completes. On success, mgr is a fully unlocked Manager
// ready for credential operations.
type createVaultResultMsg struct {
	recoveryKey string
	mgr         *vault.Manager
	err         error
}

// createVaultCmd spawns an async tea.Cmd that creates, opens, and unlocks the
// vault. Argon2id derivation inside Create+Unlock blocks for ~2-5s, so it must
// run off the event loop. The passphrase slice is zeroed when done.
func createVaultCmd(path string, passphrase []byte) tea.Cmd {
	return func() tea.Msg {
		recoveryKey, err := vault.Create(path, passphrase, crypto.DefaultParams)
		if err != nil {
			for i := range passphrase {
				passphrase[i] = 0
			}
			return createVaultResultMsg{err: err}
		}

		// Open and unlock the newly created vault so the TUI has a working
		// Manager immediately after the wizard completes.
		mgr, err := vault.Open(path)
		if err != nil {
			for i := range passphrase {
				passphrase[i] = 0
			}
			return createVaultResultMsg{recoveryKey: recoveryKey, err: fmt.Errorf("open after create: %w", err)}
		}
		if err := mgr.Unlock(passphrase); err != nil {
			for i := range passphrase {
				passphrase[i] = 0
			}
			mgr.Close()
			return createVaultResultMsg{recoveryKey: recoveryKey, err: fmt.Errorf("unlock after create: %w", err)}
		}

		for i := range passphrase {
			passphrase[i] = 0
		}
		return createVaultResultMsg{recoveryKey: recoveryKey, mgr: mgr}
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
		switch msg.Type {
		case tea.KeyEnter:
			m.state = stateWizardPassphrase
			m.passphraseInput.Focus()
			return m, m.spinner.Tick
		case tea.KeyEsc:
			if m.clipMgr != nil {
				m.clipMgr.Close()
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

// updateWizardPassphrase handles input on the passphrase entry screen (step 2/4).
//
// The screen has two fields: passphrase and confirm. Tab/Enter on the first field
// advances focus to the confirm field. Enter on the confirm field validates that
// both values match (and are non-empty) before proceeding.
//
// Design note: once both inputs match, the step uses an optimistic-advance
// pattern. The model immediately transitions to stateWizardRecoveryKey (so the UI
// remains responsive) and simultaneously dispatches createVaultCmd as an async
// command. The createVaultResultMsg updates m.recoveryKey when the Argon2id
// derivation completes (~1–3s). If vault creation fails, the model returns to
// stateWizardPassphrase with an error message.
func (m model) updateWizardPassphrase(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		// Spinner ticks while vault is being created in the background.
		if m.creating {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case createVaultResultMsg:
		m.creating = false
		if msg.err != nil {
			// Creation failed: return to passphrase step with error.
			m.errMsg = fmt.Sprintf("Error creating vault: %v", msg.err)
			m.state = stateWizardPassphrase
			m.confirmInput.Blur()
			m.passphraseInput.Focus()
			return m, nil
		}
		// Store the unlocked vault manager and recovery key.
		m.vaultMgr = msg.mgr
		m.vaultPath = m.vaultCreatePath()
		m.recoveryKey = msg.recoveryKey
		m.errMsg = ""
		return m, nil

	case tea.KeyMsg:
		if m.creating {
			return m, nil // ignore input while vault is being created
		}

		switch msg.Type {
		case tea.KeyEsc:
			if m.clipMgr != nil {
				m.clipMgr.Close()
			}
			return m, tea.Quit

		case tea.KeyTab, tea.KeyShiftTab:
			// Cycle focus between the two passphrase fields.
			if m.passphraseInput.Focused() {
				m.passphraseInput.Blur()
				m.confirmInput.Focus()
			} else {
				m.confirmInput.Blur()
				m.passphraseInput.Focus()
			}
			return m, nil

		case tea.KeyEnter:
			if m.passphraseInput.Focused() {
				// First field: advance focus to confirm field.
				m.passphraseInput.Blur()
				m.confirmInput.Focus()
				return m, nil
			}

			// Confirm field: validate that both values match and meet minimum length.
			pass := m.passphraseInput.Value()
			confirm := m.confirmInput.Value()
			if len(pass) < 8 {
				m.errMsg = "Passphrase must be at least 8 characters"
				m.confirmInput.Blur()
				m.passphraseInput.Focus()
				return m, nil
			}
			if pass != confirm {
				m.errMsg = "Passphrases do not match"
				m.confirmInput.Reset()
				m.confirmInput.Blur()
				m.passphraseInput.Focus()
				return m, nil
			}

			// Copy passphrase bytes so the async command owns the slice.
			// The async command zeroes this copy when done.
			pp := []byte(pass)

			// Zero and reset both inputs immediately.
			m.passphraseInput.Reset()
			m.confirmInput.Reset()
			m.confirmInput.Blur()
			m.errMsg = ""

			// Optimistic advance: show the recovery key screen immediately while
			// vault creation runs in the background. The recovery key will be
			// populated via createVaultResultMsg.
			m.creating = true
			m.state = stateWizardRecoveryKey
			path := m.vaultCreatePath()
			return m, tea.Batch(m.spinner.Tick, createVaultCmd(path, pp))
		}

		// Delegate typing to whichever input is focused.
		if m.passphraseInput.Focused() {
			var cmd tea.Cmd
			m.passphraseInput, cmd = m.passphraseInput.Update(msg)
			return m, cmd
		}
		var cmd tea.Cmd
		m.confirmInput, cmd = m.confirmInput.Update(msg)
		return m, cmd
	}

	// For non-key messages, delegate to whichever input is focused.
	if m.passphraseInput.Focused() {
		var cmd tea.Cmd
		m.passphraseInput, cmd = m.passphraseInput.Update(msg)
		return m, cmd
	}
	var cmd tea.Cmd
	m.confirmInput, cmd = m.confirmInput.Update(msg)
	return m, cmd
}

// updateWizardRecoveryKey handles input on the recovery key display screen (step 3/4).
func (m model) updateWizardRecoveryKey(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		// Forward spinner ticks while vault is being created.
		if m.creating {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case createVaultResultMsg:
		// Handle vault creation result that arrives while on the recovery screen.
		m.creating = false
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("Error creating vault: %v", msg.err)
			m.state = stateWizardPassphrase
			m.passphraseInput.Focus()
			return m, nil
		}
		// Store the unlocked vault manager and recovery key.
		m.vaultMgr = msg.mgr
		m.vaultPath = m.vaultCreatePath()
		m.recoveryKey = msg.recoveryKey
		m.errMsg = ""
		return m, nil

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
			m = loadCredentials(m)
			m.state = stateMainView
			m.lastActivity = time.Now()
			return m, tickCmd()
		case tea.KeyEnter:
			m = loadCredentials(m)
			m.lastActivity = time.Now()
			m.state = stateOverlayAdd
			m.addLabelInput.Focus()
			return m, tickCmd()
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
		helpBarStyle.Render("[Enter] Begin setup  [Esc] Quit")
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
		content += "\n" + helpBarStyle.Render("[Enter] Next field  [Tab] Switch field  [Esc] Quit")
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

// viewWizardRecoveryKey renders step 3/4.
func (m model) viewWizardRecoveryKey() string {
	var content string
	if m.creating || m.recoveryKey == "" {
		// Vault is still being created in the background; show a spinner.
		content = titleStyle.Render("Step 3/4: Recovery key") + "\n\n" +
			m.spinner.View() + " Creating vault… please wait.\n"
	} else {
		keyBox := overlayBoxStyle.Render(m.recoveryKey)
		content = titleStyle.Render("Step 3/4: Recovery key") + "\n\n" +
			keyBox + "\n\n" +
			errorStyle.Render("Write this down. You cannot recover your vault without it.") + "\n\n" +
			helpBarStyle.Render("[Enter] I have stored my recovery key")
	}
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
