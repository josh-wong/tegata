package main

import "testing"

// TestLedgerStartCmd verifies that 'tegata ledger start' is registered.
func TestLedgerStartCmd(t *testing.T) {
	cmd := newLedgerCmd()
	for _, sub := range cmd.Commands() {
		if sub.Use == "start" {
			return
		}
	}
	t.Fatal("'tegata ledger start' subcommand not registered")
}

// TestLedgerStopCmd verifies that 'tegata ledger stop' is registered.
func TestLedgerStopCmd(t *testing.T) {
	cmd := newLedgerCmd()
	for _, sub := range cmd.Commands() {
		if sub.Use == "stop" {
			return
		}
	}
	t.Fatal("'tegata ledger stop' subcommand not registered")
}
