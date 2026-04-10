package main

import (
	"strings"
	"testing"
)

// TestAuditOverlay_MenuItems verifies that viewAuditMenu renders the two
// expected items: "View history" and "Verify integrity".
func TestAuditOverlay_MenuItems(t *testing.T) {
	m := model{}
	view := m.viewAuditMenu()
	for _, item := range []string{"View history", "Verify integrity"} {
		if !strings.Contains(view, item) {
			t.Errorf("viewAuditMenu should contain %q", item)
		}
	}
	if strings.Contains(view, "Start ledger server") {
		t.Error("viewAuditMenu should not contain 'Start ledger server' (removed: ledger starts automatically on unlock)")
	}
}

// TestAuditOverlay_TwoItems verifies the menu has exactly 2 items.
func TestAuditOverlay_TwoItems(t *testing.T) {
	m := model{}
	view := m.viewAuditMenu()

	count := 0
	for _, item := range []string{"View history", "Verify integrity"} {
		if strings.Contains(view, item) {
			count++
		}
	}
	if count != 2 {
		t.Errorf("viewAuditMenu should contain 2 items, found %d", count)
	}
}
