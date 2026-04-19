package clipboard

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// mockClipboard is a thread-safe mock for testing clipboard operations without
// a display server.
type mockClipboard struct {
	mu      sync.Mutex
	content string
	writes  []string
}

func newMockClipboard() *mockClipboard {
	return &mockClipboard{}
}

func (m *mockClipboard) WriteAll(text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.content = text
	m.writes = append(m.writes, text)
	return nil
}

func (m *mockClipboard) ReadAll() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.content, nil
}

func (m *mockClipboard) getContent() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.content
}

func (m *mockClipboard) setContent(s string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.content = s
}

func TestCopyWritesToClipboard(t *testing.T) {
	mock := newMockClipboard()
	mgr := NewManagerWith(mock)
	defer mgr.Close()

	if err := mgr.CopyWithAutoClear("secret", 1*time.Second); err != nil {
		t.Fatalf("CopyWithAutoClear returned error: %v", err)
	}

	got := mock.getContent()
	if got != "secret" {
		t.Errorf("clipboard content = %q, want %q", got, "secret")
	}
}

func TestAutoClearAfterTimeout(t *testing.T) {
	mock := newMockClipboard()
	mgr := NewManagerWith(mock)
	defer mgr.Close()

	if err := mgr.CopyWithAutoClear("secret", 100*time.Millisecond); err != nil {
		t.Fatal(err)
	}

	// Wait for auto-clear to fire
	time.Sleep(250 * time.Millisecond)

	got := mock.getContent()
	if got != "" {
		t.Errorf("clipboard should be cleared after timeout, got %q", got)
	}
}

func TestAutoClearPreservesChangedContent(t *testing.T) {
	mock := newMockClipboard()
	mgr := NewManagerWith(mock)
	defer mgr.Close()

	if err := mgr.CopyWithAutoClear("secret", 100*time.Millisecond); err != nil {
		t.Fatal(err)
	}

	// Simulate user changing clipboard content
	mock.setContent("other")

	// Wait for auto-clear to fire
	time.Sleep(250 * time.Millisecond)

	got := mock.getContent()
	if got != "other" {
		t.Errorf("clipboard should preserve user content %q, got %q", "other", got)
	}
}

func TestNewCopyCancelsPrevious(t *testing.T) {
	mock := newMockClipboard()
	mgr := NewManagerWith(mock)
	defer mgr.Close()

	if err := mgr.CopyWithAutoClear("secret1", 100*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := mgr.CopyWithAutoClear("secret2", 200*time.Millisecond); err != nil {
		t.Fatal(err)
	}

	// After first timeout but before second
	time.Sleep(150 * time.Millisecond)

	got := mock.getContent()
	if got != "secret2" {
		t.Errorf("first auto-clear should have been canceled, clipboard = %q, want %q", got, "secret2")
	}

	// After second timeout
	time.Sleep(150 * time.Millisecond)

	got = mock.getContent()
	if got != "" {
		t.Errorf("clipboard should be cleared after second timeout, got %q", got)
	}
}

func TestCloseStopsAutoClear(t *testing.T) {
	mock := newMockClipboard()
	mgr := NewManagerWith(mock)

	if err := mgr.CopyWithAutoClear("secret", 100*time.Millisecond); err != nil {
		t.Fatal(err)
	}

	mgr.Close()

	// Wait for what would have been the auto-clear
	time.Sleep(200 * time.Millisecond)

	got := mock.getContent()
	if got != "secret" {
		t.Errorf("clipboard should not be cleared after Close, got %q", got)
	}
}

// errClipboard is a mock ClipboardAccess that always returns an error on write.
type errClipboard struct{ err error }

func (e *errClipboard) WriteAll(_ string) error  { return e.err }
func (e *errClipboard) ReadAll() (string, error) { return "", e.err }

func TestCopyWithAutoClearReturnsClipboardError(t *testing.T) {
	underlying := errors.New("display not found")
	mgr := NewManagerWith(&errClipboard{err: underlying})
	defer mgr.Close()

	err := mgr.CopyWithAutoClear("secret", time.Second)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var ce *ClipboardError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ClipboardError, got %T: %v", err, err)
	}
	if ce.Unwrap() == nil {
		t.Error("ClipboardError should wrap the underlying error")
	}
}

func TestNewWaylandClipboardFailsWithoutWlCopy(t *testing.T) {
	t.Setenv("PATH", "")

	_, err := newWaylandClipboard()
	if err == nil {
		t.Error("expected error when wl-copy is absent, got nil")
	}
}

func TestNewWaylandClipboardFailsWithoutWlPaste(t *testing.T) {
	// Create a temp dir that contains wl-copy but not wl-paste so that only
	// the wl-paste look-up fails.
	tmpDir := t.TempDir()
	fakeCopy := filepath.Join(tmpDir, "wl-copy")
	if err := os.WriteFile(fakeCopy, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tmpDir)

	_, err := newWaylandClipboard()
	if err == nil {
		t.Error("expected error when wl-paste is absent, got nil")
	}
}

func TestIsWaylandDetection(t *testing.T) {
	tests := []struct {
		name           string
		waylandDisplay string
		xdgSessionType string
		want           bool
	}{
		{"no wayland env", "", "", false},
		{"WAYLAND_DISPLAY set", "wayland-0", "", true},
		{"XDG_SESSION_TYPE wayland", "", "wayland", true},
		{"XDG_SESSION_TYPE WAYLAND uppercase", "", "WAYLAND", true},
		{"XDG_SESSION_TYPE x11", "", "x11", false},
		{"both set", "wayland-0", "wayland", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("WAYLAND_DISPLAY", tc.waylandDisplay)
			t.Setenv("XDG_SESSION_TYPE", tc.xdgSessionType)

			got := isWayland()
			if got != tc.want {
				t.Errorf("isWayland() = %v, want %v", got, tc.want)
			}
		})
	}
}
