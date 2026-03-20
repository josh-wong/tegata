package main

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	pkgmodel "github.com/josh-wong/tegata/pkg/model"
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
	case "tab":
		msg = tea.KeyMsg{Type: tea.KeyTab}
	case "ctrl+c":
		msg = tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	updated, _ := m.Update(msg)
	return updated.(model)
}

// typeInto types each rune of s into m via individual KeyRunes messages.
func typeInto(m model, s string) model {
	for _, r := range s {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		updated, _ := m.Update(msg)
		m = updated.(model)
	}
	return m
}

// TestModel_NoVault_StartsWizard asserts that when no vault path is configured,
// the model's initial state is stateWizardWelcome.
func TestModel_NoVault_StartsWizard(t *testing.T) {
	m := initialModel("")
	if m.state != stateWizardWelcome {
		t.Errorf("expected stateWizardWelcome, got %v", m.state)
	}
}

// TestWizardStateMachine asserts the full wizard state transition:
// welcome → passphrase → recovery_key → add_credential → overlay_add.
func TestWizardStateMachine(t *testing.T) {
	m := initialModel("")
	if m.state != stateWizardWelcome {
		t.Fatalf("expected stateWizardWelcome, got %v", m.state)
	}
	m = sendKey(m, "enter") // advance from welcome → passphrase
	if m.state != stateWizardPassphrase {
		t.Errorf("expected stateWizardPassphrase, got %v", m.state)
	}
	// Passphrase step requires matching text in both fields.
	// Enter on the first field moves focus to the confirm field.
	m = typeInto(m, "correct-horse-battery")
	m = sendKey(m, "enter") // focus moves to confirm field; state stays at passphrase
	if m.state != stateWizardPassphrase {
		t.Errorf("expected stateWizardPassphrase after first Enter, got %v", m.state)
	}
	m = typeInto(m, "correct-horse-battery")
	m = sendKey(m, "enter") // matching passphrases → advance to recovery_key
	if m.state != stateWizardRecoveryKey {
		t.Errorf("expected stateWizardRecoveryKey, got %v", m.state)
	}
	// Simulate async vault creation completing (the tea.Cmd is discarded by
	// sendKey, so we deliver the result message directly).
	updated, _ := m.Update(createVaultResultMsg{recoveryKey: "test-recovery-key"})
	m = updated.(model)
	m = sendKey(m, "enter") // advance from recovery_key → add_credential
	if m.state != stateWizardAddCredential {
		t.Errorf("expected stateWizardAddCredential, got %v", m.state)
	}
	m = sendKey(m, "enter") // Enter opens the add-credential overlay
	if m.state != stateOverlayAdd {
		t.Errorf("expected stateOverlayAdd after Enter on add-credential screen, got %v", m.state)
	}
}

// TestWizardSkipCredential asserts that pressing Esc on stateWizardAddCredential
// transitions directly to stateMainView without adding a credential.
func TestWizardSkipCredential(t *testing.T) {
	m := initialModel("")
	// Navigate to stateWizardAddCredential via the full passphrase confirm flow.
	m = sendKey(m, "enter")                    // welcome → passphrase
	m = typeInto(m, "correct-horse-battery")   // type in passphrase field
	m = sendKey(m, "enter")                    // move focus to confirm field
	m = typeInto(m, "correct-horse-battery")   // type matching passphrase
	m = sendKey(m, "enter")                    // passphrase → recovery_key
	// Simulate async vault creation completing.
	updated, _ := m.Update(createVaultResultMsg{recoveryKey: "test-recovery-key"})
	m = updated.(model)
	m = sendKey(m, "enter")                    // recovery_key → add_credential
	if m.state != stateWizardAddCredential {
		t.Fatalf("expected stateWizardAddCredential, got %v", m.state)
	}
	m = sendKey(m, "esc")
	if m.state != stateMainView {
		t.Errorf("expected stateMainView after Esc, got %v", m.state)
	}
}

// TestWizardPassphraseTooShort asserts that a passphrase shorter than 8 characters
// is rejected with an error message.
func TestWizardPassphraseTooShort(t *testing.T) {
	m := initialModel("")
	m = sendKey(m, "enter")          // welcome → passphrase
	m = typeInto(m, "short")         // 5 chars — below minimum
	m = sendKey(m, "enter")          // focus to confirm
	m = typeInto(m, "short")         // matching but too short
	m = sendKey(m, "enter")          // should reject
	if m.state != stateWizardPassphrase {
		t.Errorf("expected stateWizardPassphrase after short passphrase, got %v", m.state)
	}
	if m.errMsg == "" {
		t.Error("expected errMsg to be set after short passphrase")
	}
}

// TestWizardPassphraseMismatch asserts that mismatched passphrases show an error
// and keep the model in stateWizardPassphrase with focus back on the first field.
func TestWizardPassphraseMismatch(t *testing.T) {
	m := initialModel("")
	m = sendKey(m, "enter")                  // welcome → passphrase
	m = typeInto(m, "correct-horse-battery") // type passphrase
	m = sendKey(m, "enter")                  // focus to confirm
	m = typeInto(m, "wrong-passphrase")      // mismatched confirm
	m = sendKey(m, "enter")                  // should reject
	if m.state != stateWizardPassphrase {
		t.Errorf("expected stateWizardPassphrase after mismatch, got %v", m.state)
	}
	if m.errMsg == "" {
		t.Error("expected errMsg to be set after passphrase mismatch")
	}
	if !m.passphraseInput.Focused() {
		t.Error("expected focus to return to passphraseInput after mismatch")
	}
}

// TestWizardRecoveryKeyBlocksDuringCreation asserts that Enter on the recovery
// key screen is ignored while vault creation is still in progress.
func TestWizardRecoveryKeyBlocksDuringCreation(t *testing.T) {
	m := initialModel("")
	m = sendKey(m, "enter")                  // welcome → passphrase
	m = typeInto(m, "correct-horse-battery") // type passphrase
	m = sendKey(m, "enter")                  // focus to confirm
	m = typeInto(m, "correct-horse-battery") // matching confirm
	m = sendKey(m, "enter")                  // passphrase → recovery_key (creating=true)
	if m.state != stateWizardRecoveryKey {
		t.Fatalf("expected stateWizardRecoveryKey, got %v", m.state)
	}
	if !m.creating {
		t.Fatal("expected creating=true while vault creation is in progress")
	}
	// Enter should be blocked while creating is true.
	m = sendKey(m, "enter")
	if m.state != stateWizardRecoveryKey {
		t.Errorf("expected stateWizardRecoveryKey (blocked), got %v", m.state)
	}
}

// TestModel_VaultFound_GoesToUnlock asserts that when a vault path is present,
// the model's initial state is stateUnlock.
func TestModel_VaultFound_GoesToUnlock(t *testing.T) {
	m := initialModel("/path/to/vault.tegata")
	if m.state != stateUnlock {
		t.Errorf("expected stateUnlock, got %v", m.state)
	}
}

// TestTOTPTicker asserts that a tickMsg updates m.now and returns a tickCmd
// so that the ticker keeps firing every second.
func TestTOTPTicker(t *testing.T) {
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
// and that the cursor stays within bounds.
func TestMainViewNavigation(t *testing.T) {
	m := initialModel("")
	m.state = stateMainView
	// Populate with dummy items so cursor movement is possible.
	m.credList.SetItems([]list.Item{
		credItem{cred: pkgmodel.Credential{Label: "A"}},
		credItem{cred: pkgmodel.Credential{Label: "B"}},
		credItem{cred: pkgmodel.Credential{Label: "C"}},
	})
	initial := m.cursor
	m = sendKey(m, "j")
	if m.cursor != initial+1 {
		t.Errorf("expected cursor to move down after j, got %d", m.cursor)
	}
	m = sendKey(m, "k")
	if m.cursor != initial {
		t.Errorf("expected cursor to move back up after k, got %d", m.cursor)
	}
	// Cursor should not go past the last item.
	m.cursor = 2
	m = sendKey(m, "j")
	if m.cursor != 2 {
		t.Errorf("expected cursor to stay at 2 (last item), got %d", m.cursor)
	}
}

// TestOverlayAdd asserts that pressing 'a' from stateMainView transitions
// to stateOverlayAdd, and that pressing Esc closes the overlay without changes.
func TestOverlayAdd(t *testing.T) {
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
	// Enter should NOT confirm removal (prevent accidental deletion).
	enterAttempt := sendKey(m, "enter")
	if enterAttempt.state != stateOverlayRemove {
		t.Errorf("expected stateOverlayRemove after Enter (should not confirm), got %v", enterAttempt.state)
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
	m := initialModel("")
	m.state = stateMainView
	m = sendKey(m, "s")
	if m.state != stateOverlaySettings {
		t.Errorf("expected stateOverlaySettings after 's', got %v", m.state)
	}
	view := m.View()
	for _, item := range []string{"Tag management", "Change passphrase", "Export", "Config settings"} {
		if !strings.Contains(view, item) {
			t.Errorf("expected settings overlay to contain %q", item)
		}
	}
}

// TestQuitFromMainViewAfterUnlock asserts that pressing 'q' from stateMainView
// produces a tea.Quit command. This verifies that the passphrase input is properly
// blurred after unlock so it does not suppress the global quit binding.
func TestQuitFromMainViewAfterUnlock(t *testing.T) {
	m := initialModel("/path/to/vault.tegata")
	// Simulate a successful unlock: passphraseInput was focused at startup,
	// Reset() clears the value, and handleUnlockResult transitions to main view.
	m.passphraseInput.Reset()
	updated, _ := m.Update(unlockResultMsg{mgr: nil})
	m = updated.(model)
	if m.state != stateMainView {
		t.Fatalf("expected stateMainView after unlock, got %v", m.state)
	}
	if m.passphraseInput.Focused() {
		t.Fatal("expected passphraseInput to be blurred after unlock")
	}
	// 'q' should now trigger quit.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("expected tea.Quit command from 'q' in stateMainView, got nil")
	}
}
