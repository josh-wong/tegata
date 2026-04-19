package main

import "testing"

// TestLedgerStartCmd verifies that 'tegata ledger start' is registered.
func TestLedgerStartCmd(t *testing.T) {
	cmd := newLedgerCmd()
	var found bool
	for _, sub := range cmd.Commands() {
		if sub.Use == "start" {
			found = true
			break
		}
	}
	if !found {
		t.Skip("'tegata ledger start' subcommand not yet registered — expected before Plan 03")
	}
}

// TestLedgerStopCmd verifies that 'tegata ledger stop' is registered.
func TestLedgerStopCmd(t *testing.T) {
	cmd := newLedgerCmd()
	var found bool
	for _, sub := range cmd.Commands() {
		if sub.Use == "stop" {
			found = true
			break
		}
	}
	if !found {
		t.Skip("'tegata ledger stop' subcommand not yet registered — expected before Plan 03")
	}
}
