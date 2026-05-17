package main

import (
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/vault"
)

// auditAutoStartMsg is returned by maybeAutoStartCmd when the background
// auto-start attempt finishes. err is nil when the stack started successfully
// or when auto-start is not configured; non-nil on failure.
type auditAutoStartMsg struct{ err error }

// maybeAutoStartCmd runs the Docker audit stack auto-start as a tea.Cmd so
// errors route through the TUI message loop instead of being written directly
// to stderr, which would corrupt the alt-screen renderer while the TUI is active.
func maybeAutoStartCmd(cfg config.AuditConfig) tea.Cmd {
	return func() tea.Msg {
		bundleFS, _ := fs.Sub(dockerBundle, "docker-bundle")
		err := audit.RunAutoStart(cfg, bundleFS)
		return auditAutoStartMsg{err: err}
	}
}

// unlockResultMsg is returned by unlockVaultCmd when the async vault unlock
// goroutine completes. On success, mgr is non-nil. On failure, err is set.
// The builder field carries the EventBuilder constructed while the passphrase
// was still available; it may be nil when audit is disabled or unavailable.
// auditCfg is populated when auto-start is configured so handleUnlockResult
// can kick off maybeAutoStartCmd without re-reading the passphrase.
type unlockResultMsg struct {
	mgr      *vault.Manager
	err      error
	builder  *audit.EventBuilder
	auditCfg config.AuditConfig
}

// unlockVaultCmd spawns an async tea.Cmd that opens and unlocks the vault.
// Argon2id derivation inside Unlock blocks for ~1-3s, so it runs off the
// event loop. The caller must zero the passphrase slice after this call.
//
// The EventBuilder is constructed here — while the passphrase is still
// available — so the queue encryption key can be derived before zeroing.
func unlockVaultCmd(path string, passphrase []byte) tea.Cmd {
	return func() tea.Msg {
		mgr, err := vault.Open(path)
		if err != nil {
			zeroBytes(passphrase)
			return unlockResultMsg{err: err}
		}
		if err := mgr.Unlock(passphrase); err != nil {
			mgr.Close()
			zeroBytes(passphrase)
			return unlockResultMsg{err: err}
		}

		// Build EventBuilder while passphrase is available (AUDT-02).
		cfg, _ := config.Load(filepath.Dir(path))

		// Load HMAC secret from vault (encrypted storage) and inject into config.
		secretFromVault := mgr.GetSecret("audit.secret_key")

		// Migration: if the vault doesn't have the secret but tegata.toml does,
		// store it in the vault now.
		if secretFromVault == "" && cfg.Audit.SecretKey != "" {
			if vaultErr := mgr.SetSecret("audit.secret_key", cfg.Audit.SecretKey); vaultErr != nil {
				// Migration failed — keep cfg.Audit.SecretKey so audit can still
				// function using the plaintext value from tegata.toml this session.
				_, _ = fmt.Fprintf(os.Stderr, "tegata: audit secret migration failed: %v\n", vaultErr)
				secretFromVault = cfg.Audit.SecretKey
			} else {
				secretFromVault = cfg.Audit.SecretKey

				// Cleanup: rewrite tegata.toml to remove the plaintext secret_key field
				// now that it has been safely stored in the vault.
				if writeErr := config.WriteAuditSection(filepath.Dir(path), cfg.Audit); writeErr != nil {
					_, _ = fmt.Fprintf(os.Stderr, "tegata: audit secret cleanup failed: %v\n", writeErr)
				}
			}
			cfg.Audit.SecretKey = ""
		}

		// Use the secret (from vault or after migration).
		if secretFromVault != "" {
			cfg.Audit.SecretKey = secretFromVault
		}

		// Load the ledger volume key from vault (encrypted storage) so the
		// encrypted data directory can be decrypted and mounted for postgres.
		volumeKeyHex := mgr.GetSecret("audit.ledger_volume_key")
		if volumeKeyHex != "" {
			if volumeKey, hexErr := hex.DecodeString(volumeKeyHex); hexErr == nil {
				cfg.Audit.LedgerVolumeKey = volumeKey
			}
		}

		// Auto-start is deferred to a tea.Cmd (maybeAutoStartCmd) issued by
		// handleUnlockResult so any error routes through the TUI message loop
		// instead of being written to stderr, which corrupts the alt-screen
		// renderer. auditCfg is carried in the result for that purpose.

		builder, builderErr := newEventBuilder(cfg, filepath.Dir(path), passphrase)
		if builderErr != nil {
			// Non-fatal: TUI works without audit.
			_, _ = fmt.Fprintf(os.Stderr, "Warning: Audit unavailable: %v\n", builderErr)
		}

		zeroBytes(passphrase)
		return unlockResultMsg{mgr: mgr, builder: builder, auditCfg: cfg.Audit}
	}
}

// loadCredentials populates m.credList from the unlocked vault (sorted by
// label) and loads configuration from the vault directory. It must be called
// after m.vaultMgr is set to a valid, unlocked Manager.
func loadCredentials(m model) model {
	m = refreshCredList(m)

	// Load config from vault directory; fall back to defaults on error.
	if cfg, err := config.Load(filepath.Dir(m.vaultPath)); err == nil {
		// Load HMAC secret from vault (encrypted storage) and inject into config.
		if m.vaultMgr != nil {
			secretFromVault := m.vaultMgr.GetSecret("audit.secret_key")
			if secretFromVault != "" {
				cfg.Audit.SecretKey = secretFromVault
			}
		}
		m.cfg = cfg
		m.idleTimeout = cfg.IdleTimeout
	}

	return m
}

// handleUnlockResult processes the unlockResultMsg at the root Update level.
// It is called regardless of current state so the async result is never lost.
func (m model) handleUnlockResult(msg unlockResultMsg) (tea.Model, tea.Cmd) {
	m.unlocking = false
	if msg.err != nil {
		m.errMsg = "Unlock failed: " + humanizeError(msg.err)
		m.passphraseInput.Reset()
		m.passphraseInput.Focus()
		m.state = stateUnlock
		return m, nil
	}
	m.vaultMgr = msg.mgr
	if msg.mgr != nil {
		m.vaultID = msg.mgr.VaultID()
	}
	m.builder = msg.builder

	// Wire OnHashStored so submitted audit event hashes are persisted to the
	// vault for independent verification (D-15).
	if m.builder != nil && m.vaultMgr != nil {
		mgr := m.vaultMgr
		m.builder.OnHashStored = func(eventID, hashValue string) {
			if err := mgr.SetAuditHash(eventID, hashValue); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Warning: Failed to store audit hash: %v\n", err)
			}
		}
		if logErr := m.builder.LogEvent("vault-unlock", "", "", audit.Hostname(), true); logErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: Audit log failed: %v\n", logErr)
		}
	}
	m.passphraseInput.Blur()
	m = loadCredentials(m)
	m.state = stateMainView
	m.prevState = stateMainView
	m.errMsg = ""
	m.lastActivity = time.Now()
	return m, tea.Batch(tickCmd(), maybeAutoStartCmd(msg.auditCfg))
}

// updateUnlock handles key events in stateUnlock and stateLockedIdle.
func (m model) updateUnlock(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateUnlock:
		return m.updateUnlockScreen(msg)
	case stateLockedIdle:
		return m.updateLockedIdle(msg)
	}
	return m, nil
}

// updateUnlockScreen handles input on the passphrase entry screen.
func (m model) updateUnlockScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.unlocking {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		if m.unlocking {
			return m, nil // ignore input while unlock is in progress
		}

		switch msg.Type {
		case tea.KeyEnter:
			if m.passphraseInput.Value() == "" {
				return m, nil
			}
			// Copy passphrase bytes; the async command zeroes this slice after use.
			pp := []byte(m.passphraseInput.Value())
			m.passphraseInput.Reset()
			m.errMsg = ""
			m.unlocking = true
			return m, tea.Batch(m.spinner.Tick, unlockVaultCmd(m.vaultPath, pp))

		case tea.KeyEsc:
			return m.quit()
		}

		// Delegate typing to the passphrase input.
		var cmd tea.Cmd
		m.passphraseInput, cmd = m.passphraseInput.Update(msg)
		return m, cmd
	}

	// For non-key messages, delegate to the passphrase input.
	var cmd tea.Cmd
	m.passphraseInput, cmd = m.passphraseInput.Update(msg)
	return m, cmd
}

// updateLockedIdle handles input on the idle-locked screen.
func (m model) updateLockedIdle(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			m.state = stateUnlock
			m.errMsg = ""
			m.passphraseInput.Focus()
			return m, nil
		case tea.KeyEsc:
			return m.quit()
		}
	}
	return m, nil
}

// viewUnlock renders the unlock screen or the locked-idle notice.
func (m model) viewUnlock() string {
	switch m.state {
	case stateLockedIdle:
		return m.viewLockedIdle()
	default:
		return m.viewUnlockScreen()
	}
}

// viewUnlockScreen renders the passphrase entry UI.
func (m model) viewUnlockScreen() string {
	appNameHeader := appNameStyle.Render(appName)
	var content string
	if m.unlocking {
		content = appNameHeader + "\n\n" +
			titleStyle.Render("Unlock Vault") + "\n\n" +
			m.spinner.View() + " Unlocking…\n"
	} else {
		content = appNameHeader + "\n\n" +
			titleStyle.Render("Unlock Vault") + "\n\n" +
			m.passphraseInput.View() + "\n"
		if m.errMsg != "" {
			content += "\n" + renderErrMsg(m.errMsg, m.width) + "\n"
		}
		content += "\n" + helpBarStyle.Render("[Enter] Unlock  [Esc] Quit")
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

// viewLockedIdle renders the idle-locked notice.
func (m model) viewLockedIdle() string {
	content := titleStyle.Render("Vault Locked") + "\n\n" +
		"Vault locked due to inactivity. Press Enter to unlock.\n\n" +
		helpBarStyle.Render("[Enter] Unlock  [Esc] Quit")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
