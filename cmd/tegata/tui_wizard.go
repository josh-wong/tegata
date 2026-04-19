package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
		defer zeroBytes(passphrase)
		recoveryKey, err := vault.Create(path, passphrase, crypto.DefaultParams)
		if err != nil {
			return createVaultResultMsg{err: err}
		}

		// Open and unlock the newly created vault so the TUI has a working
		// Manager immediately after the wizard completes.
		mgr, err := vault.Open(path)
		if err != nil {
			return createVaultResultMsg{recoveryKey: recoveryKey, err: fmt.Errorf("open after create: %w", err)}
		}
		if err := mgr.Unlock(passphrase); err != nil {
			mgr.Close()
			return createVaultResultMsg{recoveryKey: recoveryKey, err: fmt.Errorf("unlock after create: %w", err)}
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
	case stateWizardAuditOptIn:
		return m.updateWizardAuditOptIn(msg)
	}
	return m, nil
}

// updateWizardWelcome handles input on the welcome screen (step 1/5).
//
// The screen has a single vault path input. Enter advances with the following
// logic: if the typed path resolves to an existing vault file, the model
// transitions directly to stateUnlock so the user can enter their passphrase.
// If the path does not exist yet it is stored as the new vault location and the
// model advances to stateWizardPassphrase for vault creation. An empty path
// uses the default location (vault.tegata in the current working directory).
func (m model) updateWizardWelcome(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			raw := strings.TrimSpace(m.vaultPathInput.Value())

			if raw != "" {
				resolved, err := resolvePathArg(raw)
				if err != nil {
					m.errMsg = "Invalid path: " + humanizeError(err)
					return m, nil
				}
				if info, statErr := os.Stat(resolved); statErr == nil && !info.IsDir() {
					// Existing vault file: switch to unlock flow.
					m.vaultPathInput.Reset()
					m.vaultPathInput.Blur()
					m.localVaultWarn = false
					m.vaultPath = resolved
					m.state = stateUnlock
					m.passphraseInput.Focus()
					return m, nil
				}
				// Non-existing path: warn if not on a removable drive.
				if !m.localVaultWarn && !isRemovablePath(resolved) {
					m.localVaultWarn = true
					return m, nil
				}
				m.vaultPath = resolved
			} else if !m.localVaultWarn && !isRemovablePath(".") {
				// Blank input uses cwd; warn if cwd is not removable.
				m.localVaultWarn = true
				return m, nil
			}

			// Advance to passphrase creation (first Enter when removable, or
			// second Enter after the user has acknowledged the warning).
			m.vaultPathInput.Reset()
			m.vaultPathInput.Blur()
			m.localVaultWarn = false
			m.state = stateWizardPassphrase
			m.passphraseInput.Focus()
			return m, m.spinner.Tick

		case tea.KeyEsc:
			return m.quit()
		}

		// Any typing clears the pending local-drive warning so it re-evaluates
		// against the new path when the user presses Enter again.
		m.localVaultWarn = false
		m.errMsg = ""

		// Delegate typing to the vault path input.
		var cmd tea.Cmd
		m.vaultPathInput, cmd = m.vaultPathInput.Update(msg)
		return m, cmd
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
			m.errMsg = "Vault creation failed: " + humanizeError(msg.err)
			m.state = stateWizardPassphrase
			m.confirmInput.Blur()
			m.passphraseInput.Focus()
			return m, nil
		}
		// Store the unlocked vault manager and recovery key.
		m.vaultMgr = msg.mgr
		m.vaultPath = m.vaultCreatePath()
		if msg.mgr != nil {
			m.vaultID = msg.mgr.VaultID()
		}
		m.recoveryKey = msg.recoveryKey
		m.errMsg = ""
		return m, nil

	case tea.KeyMsg:
		if m.creating {
			return m, nil // ignore input while vault is being created
		}

		switch msg.Type {
		case tea.KeyEsc:
			return m.quit()

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
			m.passphraseInput.Blur()
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
			m.errMsg = "Vault creation failed: " + humanizeError(msg.err)
			m.state = stateWizardPassphrase
			m.passphraseInput.Focus()
			return m, nil
		}
		// Store the unlocked vault manager and recovery key.
		m.vaultMgr = msg.mgr
		m.vaultPath = m.vaultCreatePath()
		if msg.mgr != nil {
			m.vaultID = msg.mgr.VaultID()
		}
		m.recoveryKey = msg.recoveryKey
		m.errMsg = ""
		return m, nil

	case tea.KeyMsg:
		// Block Enter while vault creation is still in progress. The view
		// hides the "[Enter]" hint during creation, but we must also guard
		// the keypress itself so a fast user cannot advance with no vault.
		if msg.Type == tea.KeyEnter && !m.creating {
			m.recoveryKey = ""
			m.state = stateWizardAddCredential
		}
	}
	return m, nil
}

// updateWizardAddCredential handles input on the optional first-credential
// screen (step 4/5). Esc advances to the audit opt-in step.
func (m model) updateWizardAddCredential(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m = loadCredentials(m)
			m.state = stateWizardAuditOptIn
			return m, nil
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
	case stateWizardAuditOptIn:
		return m.viewWizardAuditOptIn()
	}
	return ""
}

// viewWizardWelcome renders step 1/5.
func (m model) viewWizardWelcome() string {
	content := titleStyle.Render("Welcome to Tegata") + "\n\n" +
		"Tegata is a portable authenticator that stores your\n" +
		"two-factor authentication codes in an encrypted vault.\n\n" +
		tipStyle.Render("💡Tip:") + " Store your vault on a USB or microSD for security\n" +
		"and portability. Install Tegata on any device to access it.\n\n" +
		m.vaultPathInput.View() + "\n"

	if m.localVaultWarn {
		content += "\n" + warnStyle.Render("Warning: this path is on a system drive, not a removable drive.") + "\n" +
			"For better security, use a USB drive or microSD card — physical\n" +
			"separation keeps your vault safe if your computer is compromised.\n"
	}
	if m.errMsg != "" {
		content += "\n" + renderErrMsg(m.errMsg, m.width) + "\n"
	}
	if m.localVaultWarn {
		content += "\n" + helpBarStyle.Render("[Enter] Proceed anyway  [Esc] Quit")
	} else {
		content += "\n" + helpBarStyle.Render("[Enter] Continue  [Esc] Quit")
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

// viewWizardPassphrase renders step 2/4.
func (m model) viewWizardPassphrase() string {
	ppBytes := []byte(m.passphraseInput.Value())
	strength := strengthLabel(ppBytes)
	zeroBytes(ppBytes)
	content := titleStyle.Render("Step 2/5: Set passphrase") + "\n\n" +
		m.passphraseInput.View() + "\n" +
		m.confirmInput.View() + "\n\n" +
		"Strength: " + strength + "\n"

	if m.errMsg != "" {
		content += "\n" + renderErrMsg(m.errMsg, m.width) + "\n"
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
		content = titleStyle.Render("Step 3/5: Recovery key") + "\n\n" +
			m.spinner.View() + " Creating vault… please wait.\n"
	} else {
		keyBox := overlayBoxStyle.Render(m.recoveryKey)
		content = titleStyle.Render("Step 3/5: Recovery key") + "\n\n" +
			keyBox + "\n\n" +
			errorStyle.Render("Write this down. You cannot recover your vault without it.") + "\n\n" +
			helpBarStyle.Render("[Enter] I have stored my recovery key")
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

// viewWizardAddCredential renders step 4/4.
func (m model) viewWizardAddCredential() string {
	content := titleStyle.Render("Step 4/5: Add your first credential") + "\n\n" +
		"Add a TOTP, HOTP, challenge-response, or static password credential\n" +
		"to get started. You can add more credentials later from the main view.\n\n" +
		helpBarStyle.Render("[Enter] Add credential  [Esc] Skip")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

// updateWizardAuditOptIn handles input on the audit opt-in screen (step 5/5).
// Pressing y enables audit logging by writing Enabled=true and AutoStart=true
// to config and updating m.cfg in memory. Pressing n or Esc skips.
func (m model) updateWizardAuditOptIn(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyEsc || (len(msg.Runes) == 1 && (msg.Runes[0] == 'n' || msg.Runes[0] == 'N')):
			m.lastActivity = time.Now()
			m.state = stateMainView
			return m, tickCmd()
		case len(msg.Runes) == 1 && (msg.Runes[0] == 'y' || msg.Runes[0] == 'Y'):
			m.auditSubFlow = "start"
			m.auditLoading = true
			m.auditMsg = ""
			m.lastActivity = time.Now()
			m.state = stateOverlayAudit
			return m, tea.Batch(tickCmd(), auditStartCmd(m.cfg, m.vaultPath, m.vaultID))
		}
	}
	return m, nil
}

// viewWizardAuditOptIn renders step 5/5.
func (m model) viewWizardAuditOptIn() string {
	content := titleStyle.Render("Step 5/5: Audit logging") + "\n\n" +
		"Tegata can log every authentication event to a\n" +
		"tamper-evident ledger. This requires Docker to be\n" +
		"installed on the host machine.\n\n" +
		helpBarStyle.Render("[y] Enable audit logging  [n] Skip for now")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

// strengthLabel returns a styled passphrase strength label using the shared
// strengthLevel scoring. Informational only; no minimum is enforced here.
func strengthLabel(pass []byte) string {
	_, label := strengthLevel(pass)
	switch label {
	case "Strong":
		return successStyle.Render(label)
	case "Weak", "Too short":
		return errorStyle.Render(label)
	default:
		return label
	}
}
