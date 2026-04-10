// Package clipboard provides cross-platform clipboard access with automatic
// clearing after a timeout. It wraps github.com/atotto/clipboard with
// auto-clear logic and cancellation support. On WSL2, clip.exe and
// powershell.exe are copied to a temp directory (to gain execute bits that
// DrvFs mounts strip) and used to reach the Windows clipboard.
package clipboard

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

func (systemClipboard) WriteAll(text string) error { return clip.WriteAll(text) }
func (systemClipboard) ReadAll() (string, error)   { return clip.ReadAll() }

// wslClipboard implements ClipboardAccess for WSL2 using Windows executables
// copied to a temp directory. DrvFs mounts often strip execute bits from
// Windows system binaries, so we copy them to the Linux filesystem where
// chmod works, then run them via binfmt_misc (WSL interop).
type wslClipboard struct {
	clipPath string // path to executable copy of clip.exe
	psPath   string // path to executable copy of powershell.exe
}

// newWSLClipboard creates a wslClipboard by copying clip.exe and powershell.exe
// to a temp directory with execute permissions. Returns an error if the Windows
// binaries cannot be found or copied.
func newWSLClipboard() (*wslClipboard, error) {
	tmpDir, err := os.MkdirTemp("", "tegata-clip-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	clipSrc := findWindowsBinary("clip.exe", "/mnt/c/Windows/System32/clip.exe")
	if clipSrc == "" {
		_ = os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("clip.exe not found")
	}

	psSrc := findWindowsBinary("powershell.exe",
		"/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe")

	clipDst := filepath.Join(tmpDir, "clip.exe")
	if err := copyExecutable(clipSrc, clipDst); err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("copy clip.exe: %w", err)
	}

	w := &wslClipboard{clipPath: clipDst}

	if psSrc != "" {
		psDst := filepath.Join(tmpDir, "powershell.exe")
		if err := copyExecutable(psSrc, psDst); err == nil {
			w.psPath = psDst
		}
	}

	return w, nil
}

// findWindowsBinary returns the full path to a Windows binary, checking PATH
// first and falling back to the provided absolute path.
func findWindowsBinary(name, fallback string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	if _, err := os.Stat(fallback); err == nil {
		return fallback
	}
	return ""
}

// copyExecutable copies src to dst and sets the execute bit.
func copyExecutable(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := in.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	_, err = io.Copy(out, in)
	return err
}

func (w *wslClipboard) WriteAll(text string) error {
	cmd := exec.Command(w.clipPath)
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func (w *wslClipboard) ReadAll() (string, error) {
	if w.psPath == "" {
		return "", fmt.Errorf("powershell.exe not available")
	}
	out, err := exec.Command(w.psPath, "-command", "Get-Clipboard").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}

// isWSL reports whether the process is running inside WSL.
func isWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" || os.Getenv("WSL_INTEROP") != "" {
		return true
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}

// Manager handles clipboard operations with automatic clearing.
type Manager struct {
	cb          ClipboardAccess
	cancelClear context.CancelFunc
	mu          sync.Mutex
	tmpDir      string // set when WSL temp copies are used; cleaned up on Close
}

// NewManager creates a new clipboard manager. On WSL2 it copies Windows
// clipboard binaries to a temp directory (to work around DrvFs execute-bit
// restrictions) and uses them directly. Elsewhere it uses the system clipboard
// via atotto/clipboard.
func NewManager() *Manager {
	if runtime.GOOS == "linux" && isWSL() {
		if wsl, err := newWSLClipboard(); err == nil {
			return &Manager{
				cb:     wsl,
				tmpDir: filepath.Dir(wsl.clipPath),
			}
		}
		// Fall through to system clipboard if WSL setup fails.
	}
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
			// Canceled by a new copy or Close.
		}
	}()

	return nil
}

// Close cancels any pending auto-clear goroutine and removes temp files.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancelClear != nil {
		m.cancelClear()
		m.cancelClear = nil
	}
	if m.tmpDir != "" {
		_ = os.RemoveAll(m.tmpDir)
		m.tmpDir = ""
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
