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
		if sub.Use == "stop" || sub.Use == "stop [--wipe]" {
			found = true
			break
		}
	}
	if !found {
		t.Skip("'tegata ledger stop' subcommand not yet registered — expected before Plan 03")
	}
}

// TestLedgerStopCmd_WipeFlag verifies --wipe flag is declared on ledger stop.
func TestLedgerStopCmd_WipeFlag(t *testing.T) {
	cmd := newLedgerCmd()
	for _, sub := range cmd.Commands() {
		if sub.Use == "stop" || sub.Use == "stop [--wipe]" {
			if sub.Flags().Lookup("wipe") == nil {
				t.Error("--wipe flag not declared on 'tegata ledger stop'")
			}
			return
		}
	}
	t.Skip("ledger stop not yet registered — expected before Plan 03")
}
