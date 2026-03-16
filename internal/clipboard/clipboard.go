// Package clipboard provides cross-platform clipboard access with automatic
// clearing after a timeout. It wraps github.com/atotto/clipboard with
// auto-clear logic and cancellation support.
package clipboard

import (
	"context"
	"runtime"
	"sync"
	"time"

	clip "github.com/atotto/clipboard"
)

// ClipboardAccess abstracts platform clipboard operations for testability.
type ClipboardAccess interface {
	WriteAll(text string) error
	ReadAll() (string, error)
}

// systemClipboard implements ClipboardAccess using the real system clipboard.
type systemClipboard struct{}

func (systemClipboard) WriteAll(text string) error    { return clip.WriteAll(text) }
func (systemClipboard) ReadAll() (string, error)      { return clip.ReadAll() }

// Manager handles clipboard operations with automatic clearing.
type Manager struct {
	cb          ClipboardAccess
	cancelClear context.CancelFunc
	mu          sync.Mutex
}

// NewManager creates a new clipboard manager using the system clipboard.
func NewManager() *Manager {
	return &Manager{cb: systemClipboard{}}
}

// NewManagerWith creates a Manager using the provided clipboard implementation.
// This is primarily for testing without a display server.
func NewManagerWith(cb ClipboardAccess) *Manager {
	return &Manager{cb: cb}
}

// CopyWithAutoClear writes text to the clipboard and schedules automatic
// clearing after the specified timeout. If the clipboard content is changed by
// another application before the timeout, the clear is skipped.
// A new call to CopyWithAutoClear cancels any pending auto-clear.
func (m *Manager) CopyWithAutoClear(text string, timeout time.Duration) error {
	m.mu.Lock()

	// Cancel any previously scheduled auto-clear.
	if m.cancelClear != nil {
		m.cancelClear()
	}

	if err := m.cb.WriteAll(text); err != nil {
		m.mu.Unlock()
		return clipboardError(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelClear = cancel
	m.mu.Unlock()

	go func() {
		select {
		case <-time.After(timeout):
			m.mu.Lock()
			defer m.mu.Unlock()
			// Only clear if content is still what we wrote.
			current, err := m.cb.ReadAll()
			if err == nil && current == text {
				_ = m.cb.WriteAll("")
			}
		case <-ctx.Done():
			// Cancelled by a new copy or Close.
		}
	}()

	return nil
}

// Close cancels any pending auto-clear goroutine.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancelClear != nil {
		m.cancelClear()
		m.cancelClear = nil
	}
}

// clipboardError wraps a clipboard error with an actionable message.
func clipboardError(err error) error {
	switch runtime.GOOS {
	case "linux":
		return &ClipboardError{
			Err:     err,
			Message: "Clipboard not available. Install xclip, xsel, or wl-clipboard.",
		}
	default:
		return &ClipboardError{
			Err:     err,
			Message: "Clipboard not available.",
		}
	}
}

// ClipboardError wraps a clipboard operation error with an actionable message.
type ClipboardError struct {
	Err     error
	Message string
}

func (e *ClipboardError) Error() string {
	return e.Message
}

func (e *ClipboardError) Unwrap() error {
	return e.Err
}
