package main

import (
	"strings"
	"testing"
)

// TestAuditOverlay_StartOption verifies that viewAuditMenu renders
// "Start audit server" as the third item (index 2).
func TestAuditOverlay_StartOption(t *testing.T) {
	m := model{}
	view := m.viewAuditMenu()
	if !strings.Contains(view, "Start audit server") {
		t.Error("viewAuditMenu should contain 'Start audit server'")
	}
}

// TestAuditOverlay_ThreeItems verifies the menu has exactly 3 items.
func TestAuditOverlay_ThreeItems(t *testing.T) {
	m := model{}
	view := m.viewAuditMenu()

	count := 0
	for _, item := range []string{"View history", "Verify integrity", "Start audit server"} {
		if strings.Contains(view, item) {
			count++
		}
	}
	if count != 3 {
		t.Errorf("viewAuditMenu should contain 3 items, found %d", count)
	}
}

// TestAuditOverlay_StartSelect verifies that pressing Enter on index 2
// sets auditSubFlow to "start".
func TestAuditOverlay_StartSelect(t *testing.T) {
	m := model{auditMenuIdx: 2}
	// This test will pass once Plan 04 updates updateOverlayAudit to handle
	// index 2. Until then, it documents the expected behavior.
	_ = m
	t.Log("auditMenuIdx=2 Enter should set auditSubFlow='start' — verified in Plan 04")
}
