package audit

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/josh-wong/tegata/internal/config"
)

// TestEntityIDFromVaultID verifies the entity ID derivation format.
func TestEntityIDFromVaultID(t *testing.T) {
	got := entityIDFromVaultID("a3f2c810-1234-4567-89ab-cdef01234567")
	want := "tegata-a3f2c810"
	if got != want {
		t.Errorf("entityIDFromVaultID = %q, want %q", got, want)
	}
}

// TestEntityIDFromVaultID_Empty verifies fallback when vaultID is empty.
func TestEntityIDFromVaultID_Empty(t *testing.T) {
	got := entityIDFromVaultID("")
	if len(got) != len("tegata-")+8 {
		t.Errorf("entityIDFromVaultID('') = %q, want 'tegata-' + 8 hex chars", got)
	}
	matched, _ := regexp.MatchString(`^tegata-[0-9a-f]{8}$`, got)
	if !matched {
		t.Errorf("entityIDFromVaultID('') = %q, does not match tegata-[0-9a-f]{8}", got)
	}
}

// TestGenerateSecretKey verifies the format and randomness of generated keys.
func TestGenerateSecretKey(t *testing.T) {
	key1, err := generateSecretKey()
	if err != nil {
		t.Fatalf("generateSecretKey: %v", err)
	}
	if len(key1) != 64 {
		t.Errorf("key length = %d, want 64", len(key1))
	}
	matched, _ := regexp.MatchString(`^[0-9a-f]{64}$`, key1)
	if !matched {
		t.Errorf("key %q does not match [0-9a-f]{64}", key1)
	}

	key2, _ := generateSecretKey()
	if key1 == key2 {
		t.Error("two generateSecretKey calls produced identical keys")
	}
}

// TestExtractComposeFiles verifies that compose files are extracted to disk.
func TestExtractComposeFiles(t *testing.T) {
	fsys := fstest.MapFS{
		"docker-compose.yml":        &fstest.MapFile{Data: []byte("version: '3'")},
		"certs/client.properties":   &fstest.MapFile{Data: []byte("entity.id=test")},
	}

	dir := t.TempDir()
	if err := extractComposeFiles(fsys, dir); err != nil {
		t.Fatalf("extractComposeFiles: %v", err)
	}

	if _, err := os.Stat(dir + "/docker-compose.yml"); err != nil {
		t.Errorf("docker-compose.yml not extracted: %v", err)
	}
	if _, err := os.Stat(dir + "/certs/client.properties"); err != nil {
		t.Errorf("certs/client.properties not extracted: %v", err)
	}
}

// TestDetectDocker_NotFound verifies the error message when Docker is absent.
// This test temporarily modifies PATH to simulate Docker being absent.
// It skips when Docker is found at a known fallback location (e.g. during
// local development on macOS where /usr/local/bin/docker exists) because
// dockerBin() now checks known locations beyond PATH for GUI-app compatibility.
func TestDetectDocker_NotFound(t *testing.T) {
	if dockerBin() != "" {
		t.Skip("docker found at a known location; skipping not-found simulation")
	}
	orig := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", orig) })
	_ = os.Setenv("PATH", "")

	err := detectDocker()
	if err == nil {
		t.Fatal("detectDocker: expected error when docker not in PATH")
	}
	if !strings.Contains(err.Error(), "docker binary not found") {
		t.Errorf("detectDocker error = %q, want to contain 'docker binary not found'", err.Error())
	}
}

// TestMaybeAutoStart_NoPath verifies that auto-start is a no-op when
// DockerComposePath is empty (per D-11: users who never ran setup see nothing).
func TestMaybeAutoStart_NoPath(t *testing.T) {
	// MaybeAutoStart should return immediately without spawning a goroutine
	// that panics when DockerComposePath is empty. This test confirms the
	// function returns and the process does not crash.
	cfg := config.AuditConfig{DockerComposePath: ""}
	MaybeAutoStart(cfg, nil)
	// If we reach here without panic or hang, the no-op path works.
}
