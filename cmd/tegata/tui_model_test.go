package main

import (
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Compile-time anchor: ensure lipgloss and bubbles/textinput are imported.
// These are used by the TUI model (Plan 02) and will be exercised by the tests
// once the model type exists. The var block prevents "imported and not used" errors.
var (
	_ = lipgloss.NewStyle()
	_ = textinput.New
)

// sendKey wraps model.Update with a tea.KeyMsg for readable test assertions.
// Handles common special-key names as well as arbitrary rune sequences.
func sendKey(m model, key string) model {
	var msg tea.KeyMsg
	switch key {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+c":
		msg = tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	updated, _ := m.Update(msg)
	return updated.(model)
}

// TestModel_NoVault_StartsWizard asserts that when no vault path is configured,
// the model's initial state is stateWizardWelcome.
func TestModel_NoVault_StartsWizard(t *testing.T) {
	t.Skip("not yet implemented")
	m := initialModel("")
	if m.state != stateWizardWelcome {
		t.Errorf("expected stateWizardWelcome, got %v", m.state)
	}
}

// TestWizardStateMachine asserts the full wizard state transition:
// welcome → passphrase → recovery_key → add_credential → main_view.
func TestWizardStateMachine(t *testing.T) {
	t.Skip("not yet implemented")
	m := initialModel("")
	if m.state != stateWizardWelcome {
		t.Fatalf("expected stateWizardWelcome, got %v", m.state)
	}
	m = sendKey(m, "enter") // advance from welcome → passphrase
	if m.state != stateWizardPassphrase {
		t.Errorf("expected stateWizardPassphrase, got %v", m.state)
	}
	m = sendKey(m, "enter") // advance from passphrase → recovery_key
	if m.state != stateWizardRecoveryKey {
		t.Errorf("expected stateWizardRecoveryKey, got %v", m.state)
	}
	m = sendKey(m, "enter") // advance from recovery_key → add_credential
	if m.state != stateWizardAddCredential {
		t.Errorf("expected stateWizardAddCredential, got %v", m.state)
	}
	m = sendKey(m, "enter") // advance from add_credential → main_view
	if m.state != stateMainView {
		t.Errorf("expected stateMainView, got %v", m.state)
	}
}

// TestWizardSkipCredential asserts that pressing Esc on stateWizardAddCredential
// transitions directly to stateMainView without adding a credential.
func TestWizardSkipCredential(t *testing.T) {
	t.Skip("not yet implemented")
	m := initialModel("")
	// Navigate to stateWizardAddCredential.
	m = sendKey(m, "enter")
	m = sendKey(m, "enter")
	m = sendKey(m, "enter")
	if m.state != stateWizardAddCredential {
		t.Fatalf("expected stateWizardAddCredential, got %v", m.state)
	}
	m = sendKey(m, "esc")
	if m.state != stateMainView {
		t.Errorf("expected stateMainView after Esc, got %v", m.state)
	}
}

// TestModel_VaultFound_GoesToUnlock asserts that when a vault path is present,
// the model's initial state is stateUnlock.
func TestModel_VaultFound_GoesToUnlock(t *testing.T) {
	t.Skip("not yet implemented")
	m := initialModel("/path/to/vault.tegata")
	if m.state != stateUnlock {
		t.Errorf("expected stateUnlock, got %v", m.state)
	}
}

// TestTOTPTicker asserts that a tickMsg updates m.now and returns a tickCmd
// so that the ticker keeps firing every second.
func TestTOTPTicker(t *testing.T) {
	t.Skip("not yet implemented")
	m := initialModel("")
	// Simulate the model being in main view with a vault open.
	m.state = stateMainView
	before := m.now
	tick := tickMsg{t: time.Now().Add(time.Second)}
	updated, cmd := m.Update(tick)
	next := updated.(model)
	if !next.now.After(before) {
		t.Error("expected m.now to advance after tickMsg")
	}
	if cmd == nil {
		t.Error("expected a non-nil tickCmd after tickMsg")
	}
}

// TestIdleAutoLock asserts that a tickMsg fires auto-lock when idle time
// exceeds the configured idleTimeout, transitioning to stateLockedIdle.
func TestIdleAutoLock(t *testing.T) {
	t.Skip("not yet implemented")
	m := initialModel("")
	m.state = stateMainView
	m.lastActivity = time.Now().Add(-m.idleTimeout - time.Second)
	tick := tickMsg{t: time.Now()}
	updated, _ := m.Update(tick)
	next := updated.(model)
	if next.state != stateLockedIdle {
		t.Errorf("expected stateLockedIdle after idle timeout, got %v", next.state)
	}
}

// TestMainViewNavigation asserts that j/k move the credential list selection
// and that pressing Enter on a TOTP credential triggers a copyCmd.
func TestMainViewNavigation(t *testing.T) {
	t.Skip("not yet implemented")
	m := initialModel("")
	m.state = stateMainView
	initial := m.cursor
	m = sendKey(m, "j")
	if m.cursor != initial+1 {
		t.Errorf("expected cursor to move down after j, got %d", m.cursor)
	}
	m = sendKey(m, "k")
	if m.cursor != initial {
		t.Errorf("expected cursor to move back up after k, got %d", m.cursor)
	}
}

// TestOverlayAdd asserts that pressing 'a' from stateMainView transitions
// to stateOverlayAdd, and that pressing Esc closes the overlay without changes.
func TestOverlayAdd(t *testing.T) {
	t.Skip("not yet implemented")
	m := initialModel("")
	m.state = stateMainView
	m = sendKey(m, "a")
	if m.state != stateOverlayAdd {
		t.Errorf("expected stateOverlayAdd after 'a', got %v", m.state)
	}
	m = sendKey(m, "esc")
	if m.state != stateMainView {
		t.Errorf("expected stateMainView after Esc, got %v", m.state)
	}
}

// TestOverlayRemove asserts that pressing 'r' from stateMainView transitions
// to stateOverlayRemove; 'y' removes the selected credential and 'n' cancels.
func TestOverlayRemove(t *testing.T) {
	t.Skip("not yet implemented")
	m := initialModel("")
	m.state = stateMainView
	m = sendKey(m, "r")
	if m.state != stateOverlayRemove {
		t.Errorf("expected stateOverlayRemove after 'r', got %v", m.state)
	}
	// 'n' should cancel and return to stateMainView.
	cancel := sendKey(m, "n")
	if cancel.state != stateMainView {
		t.Errorf("expected stateMainView after 'n', got %v", cancel.state)
	}
	// 'y' should confirm removal and return to stateMainView.
	confirmed := sendKey(m, "y")
	if confirmed.state != stateMainView {
		t.Errorf("expected stateMainView after 'y', got %v", confirmed.state)
	}
}

// TestOverlaySettings asserts that pressing 's' from stateMainView transitions
// to stateOverlaySettings and that View() contains all four menu items.
func TestOverlaySettings(t *testing.T) {
	t.Skip("not yet implemented")
	m := initialModel("")
	m.state = stateMainView
	m = sendKey(m, "s")
	if m.state != stateOverlaySettings {
		t.Errorf("expected stateOverlaySettings after 's', got %v", m.state)
	}
	view := m.View()
	for _, item := range []string{"Tag management", "Change passphrase", "Export", "Config settings"} {
		if !containsString(view, item) {
			t.Errorf("expected settings overlay to contain %q", item)
		}
	}
}

// containsString is a helper used by TestOverlaySettings to check view output.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
