package drives

import "testing"

func TestIsRemovablePath_NoPanic(t *testing.T) {
	// IsRemovablePath must not panic for any input on any platform.
	paths := []string{
		".",
		"/tmp",
		"/nonexistent/path/12345",
		"",
		"relative/path",
	}
	for _, p := range paths {
		_ = IsRemovablePath(p)
	}
}

func TestIsRemovablePath_CurrentDir(t *testing.T) {
	// Running from the working directory (almost always a system drive in CI)
	// should return a bool without panicking.
	_ = IsRemovablePath(".")
}
