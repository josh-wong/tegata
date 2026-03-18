package main

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/vault"
)

// appState identifies which screen the TUI is currently showing.
type appState int

const (
	stateWizardWelcome      appState = iota // First-time setup: welcome screen
	stateWizardPassphrase                    // First-time setup: passphrase entry
	stateWizardRecoveryKey                   // First-time setup: recovery key display
	stateWizardAddCredential                 // First-time setup: optional first credential
	stateUnlock                              // Vault found at launch; enter passphrase to unlock
	stateLockedIdle                          // Vault was unlocked but idle timeout expired
	stateMainView                            // Normal authenticated view
	stateOverlayAdd                          // Add-credential overlay
	stateOverlayRemove                       // Remove-credential confirmation overlay
	stateOverlaySettings                     // Settings overlay
	stateTerminalTooNarrow                   // Terminal is narrower than 80 columns
)

// tickMsg is fired every second to update the TOTP countdown and idle timer.
type tickMsg struct{ t time.Time }

// model is the root Bubbletea model. It holds all application state for the
// duration of a `tegata ui` session.
type model struct {
	// Core state machine
	state     appState
	prevState appState // restores state when terminal becomes wide enough again

	// Terminal dimensions
	width  int
	height int

	// Vault lifecycle
	vaultPath string
	vaultMgr  *vault.Manager // nil until unlocked

	// Configuration (loaded at startup)
	cfg config.Config

	// Time tracking for TOTP and idle timeout
	now          time.Time
	lastActivity time.Time
	idleTimeout  time.Duration

	// Wizard: holds the generated recovery key during display step
	recoveryKey string

	// Wizard: async vault creation flag
	creating bool

	// Display messages
	errMsg    string
	statusMsg string

	// Cursor position for credential list navigation (Plans 03/04)
	cursor int

	// Sub-models
	passphraseInput textinput.Model
	confirmInput    textinput.Model
	credList        list.Model
	spinner         spinner.Model
}

// newPassphraseInput returns a textinput configured for masked passphrase entry.
func newPassphraseInput(placeholder string) textinput.Model {
	t := textinput.New()
	t.Placeholder = placeholder
	t.EchoMode = textinput.EchoPassword
	t.EchoCharacter = '·'
	return t
}

// initialModel constructs a new model. If vaultPath is non-empty the vault file
// exists and the TUI starts at the unlock screen; otherwise it starts the
// first-time setup wizard.
func initialModel(vaultPath string) model {
	credList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	credList.DisableQuitKeybindings()
	credList.SetFilteringEnabled(false)
	credList.SetShowHelp(false)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	cfg := config.DefaultConfig()

	m := model{
		vaultPath:       vaultPath,
		cfg:             cfg,
		idleTimeout:     cfg.IdleTimeout,
		now:             time.Now(),
		lastActivity:    time.Now(),
		passphraseInput: newPassphraseInput("Passphrase"),
		confirmInput:    newPassphraseInput("Confirm passphrase"),
		credList:        credList,
		spinner:         sp,
	}

	if vaultPath == "" {
		m.state = stateWizardWelcome
	} else {
		m.state = stateUnlock
	}

	return m
}

// Init satisfies the tea.Model interface. The TOTP ticker is started when
// entering stateMainView (Plan 03). Return nil here so there is no initial
// command before a vault is open.
func (m model) Init() tea.Cmd {
	return nil
}

// Update is the root message dispatcher. It handles terminal resize and global
// quit keys, then delegates to the per-state sub-handler.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if msg.Width < 80 {
			if m.state != stateTerminalTooNarrow {
				m.prevState = m.state
			}
			m.state = stateTerminalTooNarrow
		} else if m.state == stateTerminalTooNarrow {
			m.state = m.prevState
		}
		return m, nil

	case tea.KeyMsg:
		// Global quit keys — only when not inside a text input field.
		if !m.isInputFocused() {
			switch msg.Type {
			case tea.KeyCtrlC:
				if m.vaultMgr != nil {
					m.vaultMgr.Close()
					m.vaultMgr = nil
				}
				return m, tea.Quit
			}
			if len(msg.Runes) == 1 && msg.Runes[0] == 'q' {
				if m.state == stateMainView {
					if m.vaultMgr != nil {
						m.vaultMgr.Close()
						m.vaultMgr = nil
					}
					return m, tea.Quit
				}
			}
		}

	case tickMsg:
		m.now = msg.t
		// Idle-lock check — delegated to Plan 03; stub here.
		if m.vaultMgr != nil && time.Since(m.lastActivity) >= m.idleTimeout {
			m.vaultMgr.Close()
			m.vaultMgr = nil
			m.state = stateLockedIdle
			return m, nil
		}
		return m, tickCmd()
	}

	// Delegate to per-state handlers.
	switch m.state {
	case stateWizardWelcome,
		stateWizardPassphrase,
		stateWizardRecoveryKey,
		stateWizardAddCredential:
		return m.updateWizard(msg)

	case stateUnlock:
		return m.updateUnlock(msg)

	case stateMainView:
		return m.updateMainView(msg)

	case stateOverlayAdd, stateOverlayRemove, stateOverlaySettings:
		return m.updateOverlay(msg)
	}

	return m, nil
}

// View renders the current screen. Unimplemented states return a placeholder.
func (m model) View() string {
	switch m.state {
	case stateTerminalTooNarrow:
		content := "Terminal too narrow (minimum 80 columns)\n\nPlease resize your terminal."
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)

	case stateWizardWelcome,
		stateWizardPassphrase,
		stateWizardRecoveryKey,
		stateWizardAddCredential:
		return m.viewWizard()

	case stateUnlock:
		return "[state not yet implemented: unlock]"

	case stateMainView:
		return "[state not yet implemented: main view]"

	case stateLockedIdle:
		return "[state not yet implemented: locked idle]"

	case stateOverlayAdd, stateOverlayRemove, stateOverlaySettings:
		return m.viewOverlay()
	}

	return fmt.Sprintf("[unknown state: %d]", m.state)
}

// isInputFocused returns true when a text input sub-model currently has focus,
// which suppresses the global 'q' quit binding.
func (m model) isInputFocused() bool {
	return m.passphraseInput.Focused() || m.confirmInput.Focused()
}

// tickCmd returns a tea.Cmd that fires a tickMsg after one second.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg{t: t}
	})
}

// --- Stub handlers for states implemented in later plans ---

// updateUnlock handles key events in the stateUnlock state (Plan 03).
func (m model) updateUnlock(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

// updateMainView handles key events in the stateMainView state (Plan 03).
func (m model) updateMainView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case len(msg.Runes) == 1 && msg.Runes[0] == 'j':
			m.cursor++
			m.lastActivity = time.Now()
		case len(msg.Runes) == 1 && msg.Runes[0] == 'k':
			if m.cursor > 0 {
				m.cursor--
			}
			m.lastActivity = time.Now()
		case len(msg.Runes) == 1 && msg.Runes[0] == 'a':
			m.state = stateOverlayAdd
			m.lastActivity = time.Now()
		case len(msg.Runes) == 1 && msg.Runes[0] == 'r':
			m.state = stateOverlayRemove
			m.lastActivity = time.Now()
		case len(msg.Runes) == 1 && msg.Runes[0] == 's':
			m.state = stateOverlaySettings
			m.lastActivity = time.Now()
		}
	}
	return m, nil
}

// updateOverlay handles key events in overlay states (Plan 04).
func (m model) updateOverlay(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyEsc {
			m.state = stateMainView
		}
	}
	return m, nil
}

// viewOverlay renders overlay states (Plan 04).
func (m model) viewOverlay() string {
	switch m.state {
	case stateOverlaySettings:
		content := titleStyle.Render("Settings") + "\n\n" +
			"Tag management\n" +
			"Change passphrase\n" +
			"Export\n" +
			"Config settings\n"
		overlay := overlayBoxStyle.Render(content)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay)
	}
	return "[overlay not yet implemented]"
}
