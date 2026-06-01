package main

import (
	"testing"
	"time"
)

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		current string
		latest  string
		want    bool
	}{
		{"1.0.0", "1.0.1", true},
		{"1.0.0", "1.1.0", true},
		{"1.0.0", "2.0.0", true},
		{"1.0.0", "1.0.0", false},
		{"1.1.0", "1.0.9", false},
		{"2.0.0", "1.9.9", false},
		{"dev", "1.0.0", false},
		{"", "1.0.0", false},
		{"1.0.0", "v1.0.1", true},
		{"v1.0.0", "v1.0.1", true},
		{"1.0.0", "", false},
		{"1.2.3", "1.2.3", false},
		{"1.0.0", "v1.0.1-beta", true}, // pre-release suffix stripped during comparison
	}
	for _, tt := range tests {
		got := isNewerVersion(tt.current, tt.latest)
		if got != tt.want {
			t.Errorf("isNewerVersion(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
		}
	}
}

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"2.0.0", "1.9.9", 1},
		{"1.9.9", "2.0.0", -1},
		{"1.10.0", "1.9.0", 1},
		{"1.0.0-rc1", "1.0.0", 0},  // pre-release suffix stripped
		{"1.0.1+build", "1.0.1", 0}, // build metadata stripped
	}
	for _, tt := range tests {
		got := compareSemver(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("compareSemver(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestIsDismissed(t *testing.T) {
	const tag = "v1.2.0"

	t.Run("not dismissed when no prefs", func(t *testing.T) {
		if isDismissed(updatePrefs{}, tag) {
			t.Error("expected not dismissed")
		}
	})

	t.Run("not dismissed when different version dismissed", func(t *testing.T) {
		prefs := updatePrefs{DismissedVersion: "v1.1.0"}
		if isDismissed(prefs, tag) {
			t.Error("expected not dismissed")
		}
	})

	t.Run("dismissed when same version, no remind date (next-release)", func(t *testing.T) {
		prefs := updatePrefs{DismissedVersion: tag}
		if !isDismissed(prefs, tag) {
			t.Error("expected dismissed")
		}
	})

	t.Run("dismissed when stored without v prefix but tag has v prefix", func(t *testing.T) {
		prefs := updatePrefs{DismissedVersion: "1.2.0"} // stored from UI (no v)
		if !isDismissed(prefs, tag) {                   // tag is "v1.2.0" from GitHub
			t.Error("expected dismissed: v-prefix mismatch should be normalized")
		}
	})

	t.Run("dismissed when same version and remind date is in future", func(t *testing.T) {
		prefs := updatePrefs{
			DismissedVersion: tag,
			RemindAfter:      time.Now().Add(24 * time.Hour),
		}
		if !isDismissed(prefs, tag) {
			t.Error("expected dismissed")
		}
	})

	t.Run("not dismissed when same version but remind date has passed", func(t *testing.T) {
		prefs := updatePrefs{
			DismissedVersion: tag,
			RemindAfter:      time.Now().Add(-24 * time.Hour),
		}
		if isDismissed(prefs, tag) {
			t.Error("expected not dismissed: remind date has passed")
		}
	})
}
