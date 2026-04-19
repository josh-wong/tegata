//go:build integration

// Run Docker integration tests:
//
//	go test -tags integration ./internal/audit/... -v -timeout 300s
//
// These tests require Docker Engine and Docker Compose v2. They make real
// docker compose up/down calls and take several minutes on first run
// (contract registration downloads ~50 MB HashStore SDK).
package audit_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"testing/fstest"
	"time"

	"github.com/josh-wong/tegata/internal/audit"
)

// requireDocker skips the test if Docker Engine and Compose v2 are not available.
// Uses t.Helper() to exclude this function from test failure stack traces.
func requireDocker(t *testing.T) {
	t.Helper()
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("integration test skipped: Docker daemon not available")
	}
	if err := exec.Command("docker", "compose", "version").Run(); err != nil {
		t.Skip("integration test skipped: Docker Compose v2 not available")
	}
}

// composeSourceDir returns the absolute path to deployments/docker-compose/
// by walking up from the current test file location. This avoids hardcoded
// paths that break in CI or when the repository is in non-standard locations.
func composeSourceDir() (string, error) {
	// Get the path of this file (docker_integration_test.go).
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("unable to determine current file path")
	}

	// Walk up: docker_integration_test.go -> internal/audit -> internal -> root
	repoRoot := filepath.Join(filepath.Dir(currentFile), "..", "..")

	// Compose files are at deployments/docker-compose/
	composeDir := filepath.Join(repoRoot, "deployments", "docker-compose")

	// Verify the compose file exists.
	composePath := filepath.Join(composeDir, "docker-compose.yml")
	if _, err := os.Stat(composePath); err != nil {
		return "", fmt.Errorf("docker-compose.yml not found at %s: %w", composePath, err)
	}

	return composeDir, nil
}

// TestIntegration_SetupStack_HappyPath spins up a complete Docker Compose stack,
// registers credentials, verifies the ledger is reachable, and stores state for
// subsequent tests to reuse (to avoid re-running the expensive ~2-5 minute setup).
//
// This test MUST run first in the file so sharedComposeDir and sharedCfg are
// populated before MaybeAutoStart tests execute.
func TestIntegration_SetupStack_HappyPath(t *testing.T) {
	requireDocker(t)

	// Obtain the source directory containing docker-compose.yml.
	srcDir, err := composeSourceDir()
	if err != nil {
		t.Fatalf("determining compose source directory: %v", err)
	}

	// Create a temporary directory for the compose working directory.
	composeWorkDir := t.TempDir()

	// Use os.DirFS to read the actual compose files from the source repository.
	fsys := os.DirFS(srcDir)

	// Run the full SetupStack sequence.
	cfg, err := audit.SetupStack(fsys, composeWorkDir, "test-vault-id-e2e-9999", nil, nil)
	if err != nil {
		t.Fatalf("SetupStack: %v", err)
	}

	// Register cleanup: tear down the stack (removes containers and volume).
	composePath := filepath.Join(composeWorkDir, "docker-compose.yml")
	t.Cleanup(func() {
		_ = audit.TeardownStack(composePath)
	})

	// Verify returned AuditConfig has expected values.
	if cfg.Server != "127.0.0.1:50051" {
		t.Errorf("cfg.Server = %q, want 127.0.0.1:50051", cfg.Server)
	}
	if cfg.PrivilegedServer != "127.0.0.1:50052" {
		t.Errorf("cfg.PrivilegedServer = %q, want 127.0.0.1:50052", cfg.PrivilegedServer)
	}
	if cfg.DockerComposePath != composePath {
		t.Errorf("cfg.DockerComposePath = %q, want %q", cfg.DockerComposePath, composePath)
	}
	if cfg.EntityID == "" || len(cfg.EntityID) < 7 {
		t.Errorf("cfg.EntityID = %q, want non-empty starting with tegata-", cfg.EntityID)
	}
	if !cfg.Enabled {
		t.Error("cfg.Enabled = false, want true")
	}
	if cfg.Insecure != true {
		t.Error("cfg.Insecure = false, want true")
	}

	// Verify the secret key is 64 characters (hex-encoded 32 bytes).
	if len(cfg.SecretKey) != 64 {
		t.Errorf("cfg.SecretKey length = %d, want 64", len(cfg.SecretKey))
	}

	// Verify the ledger is reachable by creating a client and calling Ping.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := audit.NewClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("creating ledger client: %v", err)
	}
	defer func() { _ = client.Close() }()

	if err := client.Ping(ctx); err != nil {
		t.Fatalf("ledger Ping: %v", err)
	}

	t.Log("SetupStack happy path succeeded: Docker stack is running and ledger is reachable")
}

// TestIntegration_MaybeAutoStart sets up its own Docker stack, stops it, then
// calls MaybeAutoStart and polls until the ledger becomes reachable again.
// Self-contained: does not rely on TestIntegration_SetupStack_HappyPath or
// any package-level state.
func TestIntegration_MaybeAutoStart(t *testing.T) {
	requireDocker(t)

	srcDir, err := composeSourceDir()
	if err != nil {
		t.Fatalf("determining compose source directory: %v", err)
	}
	composeWorkDir := t.TempDir()
	fsys := os.DirFS(srcDir)

	cfg, err := audit.SetupStack(fsys, composeWorkDir, "test-vault-id-e2e-autostart", nil, nil)
	if err != nil {
		t.Fatalf("SetupStack: %v", err)
	}

	composePath := filepath.Join(composeWorkDir, "docker-compose.yml")
	t.Cleanup(func() { _ = audit.TeardownStack(composePath) })

	// Stop without removing the volume to simulate "stack exists but is stopped".
	if err := audit.StopStack(composePath); err != nil {
		t.Fatalf("stopping Docker stack: %v", err)
	}
	t.Log("Docker stack stopped")

	cfg.DockerComposePath = composePath
	cfg.AutoStart = true

	// Call MaybeAutoStart (non-blocking; runs in goroutine).
	// Pass nil for fsys — the compose file already exists on disk from SetupStack.
	audit.MaybeAutoStart(cfg, nil)
	t.Log("MaybeAutoStart called (non-blocking)")

	// Poll for ledger readiness (up to 60 seconds).
	pollCtx, pollCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer pollCancel()

	for {
		select {
		case <-pollCtx.Done():
			t.Fatal("ledger did not become reachable within 60 seconds after MaybeAutoStart")
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		client, clientErr := audit.NewClientFromConfig(cfg)
		if clientErr == nil {
			pingErr := client.Ping(ctx)
			_ = client.Close()
			cancel()
			if pingErr == nil {
				t.Log("MaybeAutoStart succeeded: ledger became reachable within 60 seconds")
				return
			}
		} else {
			cancel()
		}

		time.Sleep(2 * time.Second)
	}
}

// TestIntegration_SetupStack_DockerAbsent verifies that SetupStack returns
// a descriptive error when Docker is absent from PATH.
// This test skips if Docker is found at a known fallback location (e.g.
// /usr/local/bin/docker on macOS) because dockerBin() checks those paths
// beyond PATH for GUI-app compatibility, making absence unsimulatable.
func TestIntegration_SetupStack_DockerAbsent(t *testing.T) {
	if audit.DockerBinPath() != "" {
		t.Skip("docker found at a known location; cannot simulate absence — skipping")
	}
	// Temporarily remove Docker from PATH.
	t.Setenv("PATH", "")

	// Use a minimal stub FS — SetupStack should fail before extracting files.
	fsys := fstest.MapFS{
		"docker-compose.yml": &fstest.MapFile{Data: []byte("name: tegata-ledger\nservices: {}\n")},
	}

	// SetupStack should fail with a "docker binary not found" error.
	_, err := audit.SetupStack(fsys, t.TempDir(), "test-vault-id", nil, nil)
	if err == nil {
		t.Fatal("SetupStack: expected error when docker not in PATH, got nil")
	}

	if err.Error() != "docker binary not found in PATH. Install Docker Desktop from https://docs.docker.com/get-docker/" {
		t.Errorf("SetupStack error = %q, want 'docker binary not found...'", err.Error())
	}

	t.Log("SetupStack_DockerAbsent verified: returned expected error")
}

// TestIntegration_StopStack_NonExistentCompose verifies that StopStack
// returns an error when given a non-existent compose file path.
func TestIntegration_StopStack_NonExistentCompose(t *testing.T) {
	requireDocker(t)

	// Attempt to stop a compose stack at a non-existent path.
	err := audit.StopStack("/nonexistent/path/docker-compose.yml")
	if err == nil {
		t.Fatal("StopStack: expected error for non-existent path, got nil")
	}

	// Docker compose should fail with a file-not-found or similar error.
	t.Logf("StopStack_NonExistentCompose verified: returned expected error: %v", err)
}
