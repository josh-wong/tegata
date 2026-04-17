package main

import (
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

// unlockResultMsg is returned by unlockVaultCmd when the async vault unlock
// goroutine completes. On success, mgr is non-nil. On failure, err is set.
// The builder field carries the EventBuilder constructed while the passphrase
// was still available; it may be nil when audit is disabled or unavailable.
type unlockResultMsg struct {
	mgr     *vault.Manager
	err     error
	builder *audit.EventBuilder
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

		// Auto-start Docker audit stack if configured (D-09, D-10).
		// MaybeAutoStart is a no-op when DockerComposePath is empty (D-11).
		// Runs asynchronously — vault unlock is never blocked (D-10).
		// On failure, MaybeAutoStart logs to stderr and queues events (D-13).
		// Passes bundleFS so docker-compose.yml is synced on each run, keeping
		// the live stack config current after binary upgrades.
		bundleFS, _ := fs.Sub(dockerBundle, "docker-bundle")
		audit.MaybeAutoStart(cfg.Audit, bundleFS)

		builder, builderErr := newEventBuilder(cfg, filepath.Dir(path), passphrase)
		if builderErr != nil {
			// Non-fatal: TUI works without audit.
			_, _ = fmt.Fprintf(os.Stderr, "tegata: audit unavailable: %v\n", builderErr)
		}

		zeroBytes(passphrase)
		return unlockResultMsg{mgr: mgr, builder: builder}
	}
}

// loadCredentials populates m.credList from the unlocked vault (sorted by
// label) and loads configuration from the vault directory. It must be called
// after m.vaultMgr is set to a valid, unlocked Manager.
func loadCredentials(m model) model {
	m = refreshCredList(m)

	// Load config from vault directory; fall back to defaults on error.
	if cfg, err := config.Load(filepath.Dir(m.vaultPath)); err == nil {
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
		m.errMsg = fmt.Sprintf("Unlock failed: %v", msg.err)
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
	m.passphraseInput.Blur()
	m = loadCredentials(m)
	m.state = stateMainView
	m.prevState = stateMainView
	m.errMsg = ""
	m.lastActivity = time.Now()
	return m, tickCmd()
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
	var content string
	if m.unlocking {
		content = titleStyle.Render("Unlock Vault") + "\n\n" +
			m.spinner.View() + " Unlocking…\n"
	} else {
		content = titleStyle.Render("Unlock Vault") + "\n\n" +
			m.passphraseInput.View() + "\n"
		if m.errMsg != "" {
			content += "\n" + errorStyle.Render(m.errMsg) + "\n"
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
