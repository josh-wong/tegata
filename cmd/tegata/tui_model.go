package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/clipboard"
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
	stateWizardAuditOptIn                    // First-time setup: optional audit logging
	stateUnlock                              // Vault found at launch; enter passphrase to unlock
	stateLockedIdle                          // Vault was unlocked but idle timeout expired
	stateMainView                            // Normal authenticated view
	stateOverlayAdd                          // Add-credential overlay
	stateOverlayRemove                       // Remove-credential confirmation overlay
	stateOverlaySettings                     // Settings overlay
	stateOverlayAudit                        // Audit history/verify overlay
	stateTerminalTooNarrow                   // Terminal is narrower than 80 columns
)

// tickMsg is fired every second to update the TOTP countdown and idle timer.
type tickMsg struct{ t time.Time }

// sigTermMsg is sent to the model when SIGTERM or SIGHUP is received, so the
// normal quit() path runs and vault-lock is logged before the process exits.
type sigTermMsg struct{}

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
	vaultID   string         // stable vault UUID, captured from Manager.VaultID() at unlock time (D-04)
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

	// Unlock: true while Argon2id derivation is in progress
	unlocking bool

	// Display messages
	errMsg    string
	statusMsg string

	// Cursor position for credential list navigation
	cursor int

	// Wizard: vault path input on the welcome screen
	vaultPathInput textinput.Model

	// Wizard: true after the user selects a non-removable path; requires a
	// second Enter to confirm before advancing past the welcome screen.
	localVaultWarn bool

	// Challenge-response inline input
	crChallengeInput  textinput.Model
	crChallengeActive bool

	// Add-credential overlay inputs
	addLabelInput  textinput.Model // label or otpauth:// URI
	addIssuerInput textinput.Model // issuer (optional)
	addSecretInput textinput.Model // secret (masked)
	addTypeIdx     int             // 0=TOTP, 1=HOTP, 2=Static, 3=CR
	addFocusIdx    int             // which add-overlay slot has focus
	addPeriodInput textinput.Model // period in seconds (TOTP only)
	addTagsInput   textinput.Model // comma-separated tags
	addAlgoIdx     int             // 0=SHA1, 1=SHA256, 2=SHA512
	addDigitsIdx   int             // 0=6, 1=8

	// Settings overlay state
	settingsMenuIdx  int          // 0-3 menu selection
	settingsSubFlow  string       // ""|"tags"|"passphrase"|"export"|"import"|"config"
	settingsInput1   textinput.Model
	settingsInput2   textinput.Model
	settingsInput3   textinput.Model
	settingsMsg      string
	settingsTagIdx   int          // selected tag index in tag management
	settingsEditMode string       // "clipboard"|"idle"|"" for config edit mode

	// Audit overlay state
	auditMenuIdx    int             // 0=History, 1=Verify, 2=Start
	auditSubFlow    string          // ""|"history"|"verify"|"start"
	auditMsg        string          // result/status message
	auditRecords    []historyRecord // fetched records
	auditLoading    bool            // true while async gRPC call is in progress
	auditCursor     int             // selected row index in history view
	auditScrollOff  int             // first visible row index in history view
	auditMsgTime    time.Time       // time when auditMsg was set (for auto-dismiss)

	// Audit event builder (nil when audit disabled or vault locked)
	builder *audit.EventBuilder

	// Sub-models
	passphraseInput textinput.Model
	confirmInput    textinput.Model
	credList        list.Model
	spinner         spinner.Model
	clipMgr         *clipboard.Manager
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
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(cinnabar).
		Foreground(cinnabar).
		Padding(0, 0, 0, 1)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(cinnabar).
		Foreground(cinnabar).
		Padding(0, 0, 0, 1)

	credList := list.New([]list.Item{}, delegate, 0, 0)
	credList.DisableQuitKeybindings()
	credList.SetFilteringEnabled(false)
	credList.SetShowHelp(false)
	credList.SetShowStatusBar(false)
	credList.Styles.TitleBar = credList.Styles.TitleBar.PaddingLeft(0)
	credList.Styles.Title = lipgloss.NewStyle().Padding(0, 2).Foreground(cinnabar).Bold(true)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	cfg := config.DefaultConfig()

	vaultPathIn := textinput.New()
	vaultPathIn.Placeholder = "Enter a path (or leave this blank to use the current directory)"
	vaultPathIn.EchoMode = textinput.EchoNormal

	crInput := textinput.New()
	crInput.Placeholder = "hex or plain text"
	crInput.EchoMode = textinput.EchoNormal

	addLabel := textinput.New()
	addLabel.Placeholder = "Label or otpauth:// URI"
	addLabel.EchoMode = textinput.EchoNormal

	addIssuer := textinput.New()
	addIssuer.Placeholder = "Issuer (optional)"
	addIssuer.EchoMode = textinput.EchoNormal

	addSecret := textinput.New()
	addSecret.Placeholder = "Secret (base32)"
	addSecret.EchoMode = textinput.EchoPassword
	addSecret.EchoCharacter = '·'

	addPeriod := textinput.New()
	addPeriod.Placeholder = "30"
	addPeriod.EchoMode = textinput.EchoNormal

	addTags := textinput.New()
	addTags.Placeholder = "Tags (comma-separated)"
	addTags.EchoMode = textinput.EchoNormal

	settingsIn1 := textinput.New()
	settingsIn1.EchoMode = textinput.EchoNormal

	settingsIn2 := textinput.New()
	settingsIn2.EchoMode = textinput.EchoPassword
	settingsIn2.EchoCharacter = '·'

	settingsIn3 := textinput.New()
	settingsIn3.EchoMode = textinput.EchoPassword
	settingsIn3.EchoCharacter = '·'

	m := model{
		vaultPath:        vaultPath,
		cfg:              cfg,
		idleTimeout:      cfg.IdleTimeout,
		now:              time.Now(),
		lastActivity:     time.Now(),
		passphraseInput:  newPassphraseInput("Passphrase"),
		confirmInput:     newPassphraseInput("Confirm passphrase"),
		vaultPathInput:   vaultPathIn,
		crChallengeInput: crInput,
		addLabelInput:    addLabel,
		addIssuerInput:   addIssuer,
		addSecretInput:   addSecret,
		addPeriodInput:   addPeriod,
		addTagsInput:     addTags,
		settingsInput1:   settingsIn1,
		settingsInput2:   settingsIn2,
		settingsInput3:   settingsIn3,
		credList:         credList,
		spinner:          sp,
		clipMgr:          clipboard.NewManager(),
	}

	if vaultPath == "" {
		m.state = stateWizardWelcome
		m.vaultPathInput.Focus()
	} else {
		m.state = stateUnlock
		m.passphraseInput.Focus()
	}

	return m
}

// Init satisfies the tea.Model interface. The TOTP ticker is started when
// entering stateMainView. Return nil here so there is no initial command
// before a vault is open.
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
		// Sidebar is 28 wide; leave room for border/padding.
		listHeight := msg.Height - 6
		if listHeight < 1 {
			listHeight = 1
		}
		m.credList.SetSize(26, listHeight)
		if msg.Width < 80 {
			if m.state != stateTerminalTooNarrow {
				m.prevState = m.state
			}
			m.state = stateTerminalTooNarrow
		} else if m.state == stateTerminalTooNarrow {
			m.state = m.prevState
		}
		return m, nil

	case sigTermMsg:
		return m.quit()

	case tea.KeyMsg:
		// Ctrl+C always quits, even during text input.
		if msg.Type == tea.KeyCtrlC {
			return m.quit()
		}
		// 'q' quits only from stateMainView and only when no text input is focused.
		if !m.isInputFocused() && len(msg.Runes) == 1 && msg.Runes[0] == 'q' {
			if m.state == stateMainView {
				return m.quit()
			}
		}

	case tickMsg:
		m.now = msg.t
		// Auto-dismiss the "hash copied" status message after 3 seconds so it
		// doesn't linger for the full clipboard auto-clear duration.
		if m.state == stateOverlayAudit && m.auditSubFlow == "history" &&
			!m.auditMsgTime.IsZero() && time.Since(m.auditMsgTime) >= 3*time.Second {
			m.auditMsg = ""
			m.auditMsgTime = time.Time{}
		}
		// Determine whether the effective state (accounting for narrow
		// terminal overlay) is one where idle auto-lock should apply.
		effectiveState := m.state
		if effectiveState == stateTerminalTooNarrow {
			effectiveState = m.prevState
		}
		idleLockable := effectiveState == stateMainView ||
			effectiveState == stateOverlayAdd ||
			effectiveState == stateOverlayRemove ||
			effectiveState == stateOverlaySettings ||
			effectiveState == stateOverlayAudit
		if idleLockable && time.Since(m.lastActivity) >= m.idleTimeout {
			if m.builder != nil {
				if logErr := m.builder.LogEvent("vault-lock", "", "", audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(os.Stderr, "Warning: Audit log failed: %v\n", logErr)
				}
				_ = m.builder.Close()
				m.builder = nil
			}
			if m.vaultMgr != nil {
				m.vaultMgr.Close()
				m.vaultMgr = nil
			}
			// Clear messages that may contain sensitive fallback data
			// (e.g., OTP codes shown when clipboard was unavailable).
			m.statusMsg = ""
			m.errMsg = ""
			// Reset overlay and challenge-response state so stale focus
			// does not suppress keybindings after re-unlock.
			m.resetAddOverlay()
			m.resetSettingsOverlay()
			m.resetAuditOverlay()
			m.crChallengeActive = false
			m.crChallengeInput.Reset()
			m.crChallengeInput.Blur()
			m.state = stateLockedIdle
			m.prevState = stateLockedIdle
			return m, nil
		}
		return m, tickCmd()

	case unlockResultMsg:
		return m.handleUnlockResult(msg)

	case auditHistoryMsg:
		m.auditLoading = false
		if msg.err != nil {
			m.auditMsg = fmt.Sprintf("Error: %v", msg.err)
		} else if len(msg.records) == 0 {
			m.auditMsg = "No audit events found."
			m.auditRecords = nil
		} else {
			m.auditRecords = msg.records
			m.auditMsg = fmt.Sprintf("%d events", len(msg.records))
			if msg.warning != "" {
				m.auditMsg += " (" + msg.warning + ")"
			}
		}
		return m, nil

	case auditVerifyMsg:
		m.auditLoading = false
		if msg.err != nil {
			m.auditMsg = fmt.Sprintf("Error: %v", msg.err)
		} else if msg.eventCount == 0 {
			m.auditMsg = "No audit events found. Nothing to verify."
		} else if msg.valid {
			m.auditMsg = fmt.Sprintf("Audit log integrity verified. %d events checked.", msg.eventCount)
		} else {
			var sb strings.Builder
			sb.WriteString("TAMPERING DETECTED\n")
			for _, f := range msg.faults {
				sb.WriteString(formatFault(f) + "\n")
			}
			m.auditMsg = strings.TrimRight(sb.String(), "\n")
		}
		return m, nil

	case auditStartMsg:
		m.auditLoading = false
		if msg.err != nil {
			m.auditMsg = "Setup failed: " + msg.err.Error()
		} else {
			m.auditMsg = "Ledger server started. Audit logging is now active."
			// Update in-memory config so the rest of the session sees audit enabled.
			m.cfg.Audit = msg.newCfg
			// Rebuild EventBuilder so auth events are logged in this session.
			// The vault passphrase is unavailable at this point, so use an
			// in-memory queue instead of the persistent on-disk queue.
			client, clientErr := audit.NewClientFromConfig(msg.newCfg)
			if clientErr == nil {
				if newBuilder, buildErr := audit.NewEventBuilderMemQueue(client); buildErr == nil {
					if m.builder != nil {
						_ = m.builder.Close()
					}
					m.builder = newBuilder
				}
			}
		}
		return m, nil
	}

	// Delegate to per-state handlers.
	switch m.state {
	case stateWizardWelcome,
		stateWizardPassphrase,
		stateWizardRecoveryKey,
		stateWizardAddCredential,
		stateWizardAuditOptIn:
		return m.updateWizard(msg)

	case stateUnlock, stateLockedIdle:
		return m.updateUnlock(msg)

	case stateMainView:
		return m.updateMainView(msg)

	case stateOverlayAdd, stateOverlayRemove, stateOverlaySettings, stateOverlayAudit:
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
		stateWizardAddCredential,
		stateWizardAuditOptIn:
		return m.viewWizard()

	case stateUnlock, stateLockedIdle:
		return m.viewUnlock()

	case stateMainView:
		return m.viewMainView()

	case stateOverlayAdd, stateOverlayRemove, stateOverlaySettings, stateOverlayAudit:
		return m.viewOverlay()
	}

	return fmt.Sprintf("[unknown state: %d]", m.state)
}

// quit cleanly closes all resources and returns tea.Quit.
func (m model) quit() (tea.Model, tea.Cmd) {
	if m.builder != nil {
		if logErr := m.builder.LogEvent("vault-lock", "", "", audit.Hostname(), true); logErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: Audit log failed: %v\n", logErr)
		}
		_ = m.builder.Close()
		m.builder = nil
	}
	if m.clipMgr != nil {
		m.clipMgr.Close()
	}
	if m.vaultMgr != nil {
		m.vaultMgr.Close()
		m.vaultMgr = nil
	}
	return m, tea.Quit
}

// isInputFocused returns true when a text input sub-model currently has focus,
// which suppresses the global 'q' quit binding.
func (m model) isInputFocused() bool {
	return m.passphraseInput.Focused() || m.confirmInput.Focused() ||
		m.vaultPathInput.Focused() ||
		m.crChallengeInput.Focused() ||
		m.addLabelInput.Focused() || m.addIssuerInput.Focused() || m.addSecretInput.Focused() ||
		m.addPeriodInput.Focused() || m.addTagsInput.Focused() ||
		m.settingsInput1.Focused() || m.settingsInput2.Focused() || m.settingsInput3.Focused()
}

// tickCmd returns a tea.Cmd that fires a tickMsg after one second.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg{t: t}
	})
}

// updateOverlay delegates overlay key events to the correct handler.
// Key events reset the idle timer so overlays don't trigger auto-lock
// while the user is actively typing.
func (m model) updateOverlay(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); ok {
		m.lastActivity = time.Now()
	}
	switch m.state {
	case stateOverlayAdd:
		return m.updateOverlayAdd(msg)
	case stateOverlayRemove:
		return m.updateOverlayRemove(msg)
	case stateOverlaySettings:
		return m.updateOverlaySettings(msg)
	case stateOverlayAudit:
		return m.updateOverlayAudit(msg)
	}
	return m, nil
}

// viewOverlay delegates overlay rendering to the correct view.
func (m model) viewOverlay() string {
	switch m.state {
	case stateOverlayAdd:
		return m.viewOverlayAdd()
	case stateOverlayRemove:
		return m.viewOverlayRemove()
	case stateOverlaySettings:
		return m.viewOverlaySettings()
	case stateOverlayAudit:
		return m.viewOverlayAudit()
	}
	return "[overlay not yet implemented]"
}
