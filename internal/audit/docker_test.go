package audit

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
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
	if err := extractComposeFiles(fsys, dir, ""); err != nil {
		t.Fatalf("extractComposeFiles: %v", err)
	}

	if _, err := os.Stat(dir + "/docker-compose.yml"); err != nil {
		t.Errorf("docker-compose.yml not extracted: %v", err)
	}
	if _, err := os.Stat(dir + "/certs/client.properties"); err != nil {
		t.Errorf("certs/client.properties not extracted: %v", err)
	}
}

// TestExtractComposeFiles_RewritesVolume verifies that extractComposeFiles
// rewrites the Docker volume name in docker-compose.yml when entityID is set.
func TestExtractComposeFiles_RewritesVolume(t *testing.T) {
	const composeContent = "volumes:\n  scalardl-data:\n    name: tegata-scalardl-data\n"
	fsys := fstest.MapFS{
		"docker-compose.yml":      &fstest.MapFile{Data: []byte(composeContent)},
		"certs/client.properties": &fstest.MapFile{Data: []byte("entity.id=test")},
	}

	dir := t.TempDir()
	if err := extractComposeFiles(fsys, dir, "tegata-abc12345"); err != nil {
		t.Fatalf("extractComposeFiles: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "docker-compose.yml"))
	if err != nil {
		t.Fatalf("reading docker-compose.yml: %v", err)
	}
	if !bytes.Contains(got, []byte("name: tegata-abc12345-scalardl-data")) {
		t.Errorf("per-vault volume name not found in extracted compose file: %s", got)
	}
	if bytes.Contains(got, []byte("name: tegata-scalardl-data\n")) {
		t.Errorf("original volume name still present in extracted compose file: %s", got)
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

// TestComposeDirForVault verifies the per-vault compose directory path format.
func TestComposeDirForVault(t *testing.T) {
	got := ComposeDirForVault("/home/user", "a3f2c810-1234-4567-89ab-cdef01234567")
	want := filepath.Join("/home/user", ".tegata", "docker", "tegata-a3f2c810")
	if got != want {
		t.Errorf("ComposeDirForVault = %q, want %q", got, want)
	}
}

// TestRewriteComposeVolume verifies the volume name substitution logic.
func TestRewriteComposeVolume(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		entityID  string
		want      string
		wantMatch bool
	}{
		{
			name:      "rewrites matching volume name",
			input:     "volumes:\n  scalardl-data:\n    name: tegata-scalardl-data\n",
			entityID:  "tegata-abc12345",
			want:      "volumes:\n  scalardl-data:\n    name: tegata-abc12345-scalardl-data\n",
			wantMatch: true,
		},
		{
			name:      "no match leaves data unchanged",
			input:     "volumes:\n  other:\n    name: something-else\n",
			entityID:  "tegata-abc12345",
			want:      "volumes:\n  other:\n    name: something-else\n",
			wantMatch: false,
		},
		{
			name:      "replaces all occurrences",
			input:     "name: tegata-scalardl-data\nname: tegata-scalardl-data\n",
			entityID:  "tegata-def67890",
			want:      "name: tegata-def67890-scalardl-data\nname: tegata-def67890-scalardl-data\n",
			wantMatch: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := rewriteComposeVolume([]byte(tc.input), tc.entityID)
			if string(got) != tc.want {
				t.Errorf("rewriteComposeVolume data = %q, want %q", got, tc.want)
			}
			if ok != tc.wantMatch {
				t.Errorf("rewriteComposeVolume ok = %v, want %v", ok, tc.wantMatch)
			}
		})
	}
}

// TestSyncDockerCompose_RewritesVolume verifies that syncDockerCompose rewrites
// the volume name when entityID is non-empty and leaves it unchanged when empty.
func TestSyncDockerCompose_RewritesVolume(t *testing.T) {
	const composeContent = "volumes:\n  scalardl-data:\n    name: tegata-scalardl-data\n"
	fsys := fstest.MapFS{
		"docker-compose.yml": &fstest.MapFile{Data: []byte(composeContent)},
	}

	t.Run("rewrites volume with entityID", func(t *testing.T) {
		dir := t.TempDir()
		composePath := filepath.Join(dir, "docker-compose.yml")
		if err := syncDockerCompose(fsys, composePath, "tegata-abc12345"); err != nil {
			t.Fatalf("syncDockerCompose: %v", err)
		}
		got, err := os.ReadFile(composePath)
		if err != nil {
			t.Fatalf("reading file: %v", err)
		}
		if !bytes.Contains(got, []byte("name: tegata-abc12345-scalardl-data")) {
			t.Errorf("rewritten volume name not found in output: %s", got)
		}
		if bytes.Contains(got, []byte("name: tegata-scalardl-data\n")) {
			t.Errorf("original volume name still present in output: %s", got)
		}
	})

	t.Run("leaves volume unchanged when entityID empty", func(t *testing.T) {
		dir := t.TempDir()
		composePath := filepath.Join(dir, "docker-compose.yml")
		if err := syncDockerCompose(fsys, composePath, ""); err != nil {
			t.Fatalf("syncDockerCompose: %v", err)
		}
		got, err := os.ReadFile(composePath)
		if err != nil {
			t.Fatalf("reading file: %v", err)
		}
		if !bytes.Contains(got, []byte("name: tegata-scalardl-data")) {
			t.Errorf("expected original volume name in output: %s", got)
		}
	})
}

// TestIsDockerProjectRunning_NoComposeFile verifies that isDockerProjectRunning
// returns false when the compose file does not exist (project was never set up).
func TestIsDockerProjectRunning_NoComposeFile(t *testing.T) {
	if isDockerProjectRunning("/nonexistent/docker-compose.yml", "some-project") {
		t.Error("isDockerProjectRunning: expected false for nonexistent compose file, got true")
	}
}

// TestIsDockerProjectRunning_EmptyArgs verifies that isDockerProjectRunning
// returns false when composePath or projectName is empty.
func TestIsDockerProjectRunning_EmptyArgs(t *testing.T) {
	if isDockerProjectRunning("", "project") {
		t.Error("isDockerProjectRunning: expected false for empty composePath")
	}
	if isDockerProjectRunning("/some/path", "") {
		t.Error("isDockerProjectRunning: expected false for empty projectName")
	}
}

// TestCheckLedgerAvailability_NoDockerPath verifies that CheckLedgerAvailability
// is a no-op when DockerComposePath is empty (externally managed ledger).
func TestCheckLedgerAvailability_NoDockerPath(t *testing.T) {
	cfg := config.AuditConfig{DockerComposePath: ""}
	if err := CheckLedgerAvailability(cfg); err != nil {
		t.Errorf("CheckLedgerAvailability with no compose path: unexpected error: %v", err)
	}
}

// TestCheckPorts_PortInUse verifies that checkPorts returns a descriptive error
// when a port is already bound.
func TestCheckPorts_PortInUse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	err = checkPorts([]int{port})
	if err == nil {
		t.Fatal("checkPorts: expected error when port is in use, got nil")
	}
	want := fmt.Sprintf("Port %d is already in use", port)
	if !strings.Contains(err.Error(), want) {
		t.Errorf("checkPorts error = %q, want to contain %q", err.Error(), want)
	}
	if !strings.Contains(err.Error(), "tegata ledger stop") {
		t.Errorf("checkPorts error = %q, want to mention 'tegata ledger stop'", err.Error())
	}
}

// TestCheckPorts_PortFree verifies that checkPorts returns nil when the port
// list is empty (no ports to check).
func TestCheckPorts_PortFree(t *testing.T) {
	if err := checkPorts(nil); err != nil {
		t.Errorf("checkPorts(nil): unexpected error: %v", err)
	}
}

// TestCheckPorts_FirstConflictReported verifies that checkPorts reports the
// first conflicting port, not a later one, so the error is deterministic.
func TestCheckPorts_FirstConflictReported(t *testing.T) {
	ln1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen ln1: %v", err)
	}
	defer ln1.Close()
	ln2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen ln2: %v", err)
	}
	defer ln2.Close()

	port1 := ln1.Addr().(*net.TCPAddr).Port
	port2 := ln2.Addr().(*net.TCPAddr).Port

	err = checkPorts([]int{port1, port2})
	if err == nil {
		t.Fatal("checkPorts: expected error, got nil")
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("Port %d", port1)) {
		t.Errorf("checkPorts error = %q, want to mention first port %d", err.Error(), port1)
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
