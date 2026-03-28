package audit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/josh-wong/tegata/internal/config"
)

// autoStartRetries and autoStartInterval control how long MaybeAutoStart
// waits for the ledger to become reachable after docker compose up -d.
// 15 retries x 2s = 30s total, matching the scalardl-contract-registration
// container's own wait loop (per RESEARCH.md).
const (
	autoStartRetries  = 15
	autoStartInterval = 2 * time.Second
)

// setupTestObjectID is a fixed well-known key used during setup to verify that
// the generic contracts are registered. Using a constant avoids accumulating
// unique orphan objects on every run.
const setupTestObjectID = "tegata-setup-probe"

// daemonPollRetries and daemonPollInterval control how long detectDocker
// waits for the Docker daemon to become ready after attempting an auto-start.
const (
	daemonPollRetries  = 30
	daemonPollInterval = 2 * time.Second
)

// detectDocker checks that Docker is installed, the daemon is running (starting
// it automatically if needed), and Compose v2 is available.
func detectDocker() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("Docker is not installed or not in PATH. Install Docker Desktop from https://docs.docker.com/get-docker/")
	}

	if err := ensureDockerDaemon(); err != nil {
		return err
	}

	if err := exec.Command("docker", "compose", "version").Run(); err != nil {
		return fmt.Errorf("Docker Compose v2 plugin is not available. Upgrade Docker to a version that includes Compose v2 (Docker Desktop 3.4+ or Docker Engine 20.10+ with compose plugin)")
	}

	return nil
}

// ensureDockerDaemon verifies the Docker daemon is reachable. If not, it
// attempts a platform-specific auto-start and polls until ready or timeout.
func ensureDockerDaemon() error {
	if exec.Command("docker", "info").Run() == nil {
		return nil
	}

	// Daemon not running — attempt auto-start.
	_, _ = fmt.Fprintln(os.Stderr, "tegata: Docker daemon is not running; attempting to start it...")
	_ = startDockerDaemon() // best-effort; ignore launch error and poll instead

	for i := 0; i < daemonPollRetries; i++ {
		time.Sleep(daemonPollInterval)
		if exec.Command("docker", "info").Run() == nil {
			return nil
		}
	}

	waitSecs := daemonPollRetries * int(daemonPollInterval/time.Second)
	return fmt.Errorf("Docker daemon did not start within %d seconds. Please start Docker Desktop and retry", waitSecs)
}

// startDockerDaemon attempts to launch the Docker daemon using
// platform-specific methods. Returns an error only if the launch command
// itself fails to start; daemon readiness is polled separately by the caller.
func startDockerDaemon() error {
	switch runtime.GOOS {
	case "windows":
		progFiles := os.Getenv("ProgramFiles")
		if progFiles == "" {
			progFiles = `C:\Program Files`
		}
		desktopExe := filepath.Join(progFiles, "Docker", "Docker", "Docker Desktop.exe")
		return exec.Command("cmd", "/c", "start", "", desktopExe).Start()
	case "darwin":
		return exec.Command("open", "-a", "Docker").Start()
	default: // linux
		return exec.Command("systemctl", "start", "docker").Start()
	}
}

// entityIDFromVaultID derives a ScalarDL entity ID from a vault UUID. The
// format is "tegata-<first8chars>". If vaultID is empty, a random 8-char hex
// suffix is generated instead.
func entityIDFromVaultID(vaultID string) string {
	if vaultID == "" {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		return "tegata-" + hex.EncodeToString(b)
	}

	// Strip hyphens and take the first 8 characters.
	id := strings.ReplaceAll(vaultID, "-", "")
	if len(id) > 8 {
		id = id[:8]
	}

	return "tegata-" + id
}

// generateSecretKey generates a cryptographically random 32-byte secret key
// and returns it as a 64-character lowercase hex string.
func generateSecretKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating secret key: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// extractComposeFiles walks the provided fs.FS and writes each file to
// targetDir, preserving the directory structure.
func extractComposeFiles(fsys fs.FS, targetDir string) error {
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}

		target := filepath.Join(targetDir, filepath.FromSlash(path))

		if d.IsDir() {
			return os.MkdirAll(target, 0700)
		}

		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("reading embedded file %s: %w", path, err)
		}

		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}

		return os.WriteFile(target, data, 0600)
	})
}

// writeClientProperties writes the client.properties file used by the
// ScalarDL HashStore SDK for HMAC authentication. Uses 127.0.0.1 (not
// localhost) to avoid IPv6 resolution issues on WSL.
func writeClientProperties(composeDir, entityID, secretKey string) error {
	certsDir := filepath.Join(composeDir, "certs")
	if err := os.MkdirAll(certsDir, 0700); err != nil {
		return fmt.Errorf("creating certs directory: %w", err)
	}

	content := fmt.Sprintf(`scalar.dl.client.server.host=127.0.0.1
scalar.dl.client.server.port=50051
scalar.dl.client.server.privileged_port=50052
scalar.dl.client.authentication.method=hmac
scalar.dl.client.entity.id=%s
scalar.dl.client.entity.identity.hmac.secret_key=%s
scalar.dl.client.entity.identity.hmac.secret_key_version=1
`, entityID, secretKey)

	path := filepath.Join(certsDir, "client.properties")
	return os.WriteFile(path, []byte(content), 0600)
}

// runDockerCompose executes a docker compose command with the given compose
// file path and arguments. Returns an error with stdout+stderr on failure.
func runDockerCompose(composePath string, args ...string) error {
	cmdArgs := append([]string{"compose", "-f", composePath}, args...)
	cmd := exec.Command("docker", cmdArgs...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose %v: %w\n%s", args, err, output)
	}
	return nil
}

// progress is a helper to call a progress callback if it is non-nil.
func progress(fn func(string), msg string) {
	if fn != nil {
		fn(msg)
	}
}

// SetupStack runs the full 8-step Docker audit setup sequence (per D-03):
//  1. detectDocker
//  2. extractComposeFiles(fsys, composeDir)
//  3. entityID from vaultID, generate secretKey
//  4. write client.properties to composeDir/certs/
//  5. docker compose -f composeDir/docker-compose.yml up -d
//  6. wait for ledger (up to 30s)
//  7. RegisterSecret + Ping + verifyContracts (reuses audit.NewClientFromConfig)
//  8. returns populated AuditConfig -- caller writes tegata.toml
//
// progressFn receives one-line status strings as each step completes; it may be
// nil (no progress reporting). fsys must contain docker-compose.yml and
// certs/client.properties at its root.
//
// Per D-12: if Docker is absent, returns a descriptive error and does NOT
// modify any config. tegata.toml is only written by the caller on success.
func SetupStack(fsys fs.FS, composeDir, vaultID string, progressFn func(string)) (config.AuditConfig, error) {
	// Step 1: Check Docker installation.
	progress(progressFn, "Checking Docker installation...")
	if err := detectDocker(); err != nil {
		return config.AuditConfig{}, err
	}

	// Step 2: Extract compose files.
	progress(progressFn, "Extracting compose files to "+composeDir+"...")
	if err := os.MkdirAll(composeDir, 0700); err != nil {
		return config.AuditConfig{}, fmt.Errorf("creating compose directory: %w", err)
	}
	if err := extractComposeFiles(fsys, composeDir); err != nil {
		return config.AuditConfig{}, fmt.Errorf("extracting compose files: %w", err)
	}

	// Step 3: Generate entity ID and secret key.
	entityID := entityIDFromVaultID(vaultID)
	secretKey, err := generateSecretKey()
	if err != nil {
		return config.AuditConfig{}, err
	}

	// Step 4: Write client.properties.
	progress(progressFn, "Generating audit credentials...")
	if err := writeClientProperties(composeDir, entityID, secretKey); err != nil {
		return config.AuditConfig{}, fmt.Errorf("writing client properties: %w", err)
	}

	// Step 5: Start Docker stack.
	composePath := filepath.Join(composeDir, "docker-compose.yml")
	progress(progressFn, "Starting Docker stack...")
	if err := StartStack(composePath); err != nil {
		return config.AuditConfig{}, fmt.Errorf("starting Docker stack: %w", err)
	}

	// Step 6: Wait for ledger to become ready.
	progress(progressFn, "Waiting for ledger to become ready (up to 30s)...")
	cfg := config.AuditConfig{
		Enabled:           true,
		Server:            "127.0.0.1:50051",
		PrivilegedServer:  "127.0.0.1:50052",
		EntityID:          entityID,
		SecretKey:         secretKey,
		KeyVersion:        1,
		Insecure:          true,
		DockerComposePath: composePath,
	}

	if err := waitForLedger(cfg); err != nil {
		return config.AuditConfig{}, fmt.Errorf("waiting for ledger: %w", err)
	}

	// Step 7: Register secret and verify contracts.
	progress(progressFn, "Registering audit credentials...")
	client, err := NewClientFromConfig(cfg.Server, cfg.PrivilegedServer, cfg.EntityID, cfg.KeyVersion, cfg.SecretKey, cfg.Insecure)
	if err != nil {
		return config.AuditConfig{}, fmt.Errorf("connecting to ledger: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.RegisterSecret(ctx, entityID, cfg.KeyVersion, secretKey); err != nil {
		return config.AuditConfig{}, fmt.Errorf("registering secret: %w", err)
	}

	if err := client.Ping(ctx); err != nil {
		return config.AuditConfig{}, fmt.Errorf("ping after registration: %w", err)
	}

	if err := client.Put(ctx, setupTestObjectID, "0000000000000000000000000000000000000000000000000000000000000000"); err != nil {
		return config.AuditConfig{}, fmt.Errorf("contract verification: %w", err)
	}

	// Step 8: Return populated config.
	return cfg, nil
}

// waitForLedger retries connecting to the ledger up to autoStartRetries times
// with autoStartInterval between attempts. Returns nil on first successful
// ping, or an error if all retries are exhausted.
func waitForLedger(cfg config.AuditConfig) error {
	var lastErr error
	for i := 0; i < autoStartRetries; i++ {
		client, err := NewClientFromConfig(cfg.Server, cfg.PrivilegedServer, cfg.EntityID, cfg.KeyVersion, cfg.SecretKey, cfg.Insecure)
		if err != nil {
			lastErr = err
			time.Sleep(autoStartInterval)
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = client.Ping(ctx)
		cancel()
		_ = client.Close()
		if err == nil {
			return nil
		}
		lastErr = err
		time.Sleep(autoStartInterval)
	}
	return fmt.Errorf("ledger did not become ready after %d attempts: %w", autoStartRetries, lastErr)
}

// StartStack runs `docker compose -f composePath up -d` synchronously.
// Returns an error with Docker stdout+stderr on non-zero exit.
func StartStack(composePath string) error {
	return runDockerCompose(composePath, "up", "-d")
}

// StopStack runs `docker compose -f composePath stop` (preserves named volume)
// or `docker compose -f composePath down -v` when wipe is true.
func StopStack(composePath string, wipe bool) error {
	if wipe {
		return runDockerCompose(composePath, "down", "-v")
	}
	return runDockerCompose(composePath, "stop")
}

// MaybeAutoStart fires in a background goroutine when cfg.DockerComposePath
// is non-empty. It runs docker compose up -d then retries Ping up to
// autoStartRetries times. Non-blocking -- never panics, logs to stderr on
// failure. Per D-10 and D-13.
func MaybeAutoStart(cfg config.AuditConfig) {
	if cfg.DockerComposePath == "" {
		return
	}
	go func() {
		if err := StartStack(cfg.DockerComposePath); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "tegata: audit auto-start failed: %v\n", err)
			return
		}
		// Retry ping up to autoStartRetries times.
		for i := 0; i < autoStartRetries; i++ {
			client, err := NewClientFromConfig(cfg.Server, cfg.PrivilegedServer, cfg.EntityID, cfg.KeyVersion, cfg.SecretKey, cfg.Insecure)
			if err != nil {
				time.Sleep(autoStartInterval)
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			pingErr := client.Ping(ctx)
			cancel()
			_ = client.Close()
			if pingErr == nil {
				return
			}
			time.Sleep(autoStartInterval)
		}
		_, _ = fmt.Fprintf(os.Stderr, "tegata: audit ledger did not become ready after auto-start\n")
	}()
}
