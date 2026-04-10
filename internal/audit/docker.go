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

// SetupTestObjectID is a fixed well-known key used during setup and by
// `tegata ledger setup` to verify that the generic contracts are registered.
// Using a constant avoids accumulating unique orphan objects on every run.
const SetupTestObjectID = "tegata-setup-probe"

// daemonPollRetries and daemonPollInterval control how long detectDocker
// waits for the Docker daemon to become ready after attempting an auto-start.
const (
	daemonPollRetries  = 30
	daemonPollInterval = 2 * time.Second
)

// contractRetries and contractRetryInterval control how long waitForContracts
// polls for the generic ScalarDL contracts to become reachable. The
// scalardl-contract-registration container runs `apk add curl unzip`,
// downloads the ~50 MB HashStore SDK from GitHub, starts the JVM, and calls
// `scalardl-hashstore bootstrap` — which can exceed 2 minutes on first run.
// 30 retries x 10s = 5 minutes total.
const (
	contractRetries       = 30
	contractRetryInterval = 10 * time.Second
)

// detectDocker checks that Docker is installed, the daemon is running (starting
// it automatically if needed), and Compose v2 is available.
func detectDocker() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker binary not found in PATH. Install Docker Desktop from https://docs.docker.com/get-docker/")
	}

	if err := ensureDockerDaemon(); err != nil {
		return err
	}

	if err := exec.Command("docker", "compose", "version").Run(); err != nil {
		return fmt.Errorf("docker compose v2 plugin not available. Upgrade to Docker Desktop 3.4+ or Docker Engine 20.10+ with the compose plugin")
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
	_ = startDockerDaemon() // best-effort; ignore launch error and poll instead

	for i := 0; i < daemonPollRetries; i++ {
		time.Sleep(daemonPollInterval)
		if exec.Command("docker", "info").Run() == nil {
			return nil
		}
	}

	waitSecs := daemonPollRetries * int(daemonPollInterval/time.Second)
	return fmt.Errorf("docker daemon did not start within %d seconds. Please start Docker Desktop and retry", waitSecs)
}

// startDockerDaemon attempts to launch the Docker daemon using
// platform-specific methods. Returns an error only if no launch path
// succeeded; daemon readiness is polled separately by the caller.
func startDockerDaemon() error {
	switch runtime.GOOS {
	case "windows":
		// Docker Desktop can be installed system-wide (%ProgramFiles%) or
		// per-user (%LocalAppData%\Programs) on machines without admin rights.
		// Try each known location in order and launch the first one found.
		type candidate struct{ env, rel string }
		for _, c := range []candidate{
			{"ProgramFiles", `Docker\Docker\Docker Desktop.exe`},
			{"LocalAppData", `Programs\Docker\Docker\Docker Desktop.exe`},
			{"ProgramFiles(x86)", `Docker\Docker\Docker Desktop.exe`},
		} {
			dir := os.Getenv(c.env)
			if dir == "" {
				continue
			}
			exe := filepath.Join(dir, c.rel)
			if _, err := os.Stat(exe); err == nil {
				return exec.Command(exe).Start()
			}
		}
		return fmt.Errorf("no Docker Desktop installation found in any known location; please start it manually")
	case "darwin":
		return exec.Command("open", "-a", "Docker").Start()
	default: // linux
		// Try systemd first, then sysvinit/OpenRC for distros or WSL2
		// configurations that do not use systemd.
		if exec.Command("systemctl", "start", "docker").Run() == nil {
			return nil
		}
		return exec.Command("service", "docker", "start").Start()
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

// writeClientProperties writes two properties files to composeDir/certs/:
//
//   - client.properties: used by the Go client running on the host. Uses
//     127.0.0.1 (not localhost) to avoid IPv6 resolution issues on WSL.
//
//   - bootstrap.properties: used by scalardl-hashstore bootstrap running
//     inside the Docker Compose network, where services reach each other by
//     container hostname, not by the host-mapped 127.0.0.1 address.
func writeClientProperties(composeDir, entityID, secretKey string) error {
	certsDir := filepath.Join(composeDir, "certs")
	if err := os.MkdirAll(certsDir, 0700); err != nil {
		return fmt.Errorf("creating certs directory: %w", err)
	}

	props := func(host string) string {
		return fmt.Sprintf(`scalar.dl.client.server.host=%s
scalar.dl.client.server.port=50051
scalar.dl.client.server.privileged_port=50052
scalar.dl.client.authentication.method=hmac
scalar.dl.client.entity.id=%s
scalar.dl.client.entity.identity.hmac.secret_key=%s
scalar.dl.client.entity.identity.hmac.secret_key_version=1
`, host, entityID, secretKey)
	}

	if err := os.WriteFile(filepath.Join(certsDir, "client.properties"), []byte(props("127.0.0.1")), 0600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(certsDir, "bootstrap.properties"), []byte(props("scalardl-ledger")), 0600)
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
// SetupStack runs the full Docker audit setup sequence. It starts Docker, extracts
// compose files, generates entity credentials, starts the stack, waits for the
// ledger, registers the entity secret, and then waits for the generic contracts
// to be reachable (up to 5 minutes). After the entity is registered and the
// ledger is reachable, onRegistered is called with the populated AuditConfig —
// the caller should write tegata.toml at that point so config is persisted even
// if contract registration is still in progress. If onRegistered returns an
// error, SetupStack returns immediately with that error.
func SetupStack(fsys fs.FS, composeDir, vaultID string, progressFn func(string), onRegistered func(config.AuditConfig) error) (config.AuditConfig, error) {
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

	// Step 5: Start Docker stack and ensure contract registration runs with
	// the current entity's credentials. Force-recreate the registration
	// container so bootstrap runs even when the stack is already up (e.g.
	// when setting up audit for a second vault — each entity needs its own
	// contracts registered via scalardl-hashstore bootstrap).
	composePath := filepath.Join(composeDir, "docker-compose.yml")
	progress(progressFn, "Starting Docker stack...")
	if err := StartStack(composePath); err != nil {
		return config.AuditConfig{}, fmt.Errorf("starting Docker stack: %w", err)
	}
	progress(progressFn, "Registering contracts for entity...")
	if err := runDockerCompose(composePath, "up", "-d", "--force-recreate", "scalardl-contract-registration"); err != nil {
		return config.AuditConfig{}, fmt.Errorf("restarting contract registration: %w", err)
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

	// Step 7: Register entity secret and verify the ledger is reachable.
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

	// Notify the caller that credentials are registered and the config is ready.
	// The caller should persist tegata.toml here so audit is configured even if
	// contract registration below is still in progress.
	if onRegistered != nil {
		if err := onRegistered(cfg); err != nil {
			return config.AuditConfig{}, err
		}
	}

	// Step 8: Wait for generic contracts to become reachable (up to 5 minutes).
	// The scalardl-contract-registration container downloads ~50 MB and runs the
	// bootstrap tool; this is the slow part on first run.
	progress(progressFn, "Waiting for generic contracts to become ready (up to 5 minutes on first run)...")
	if err := waitForContracts(client, progressFn); err != nil {
		return config.AuditConfig{}, fmt.Errorf("waiting for contracts: %w", err)
	}

	return cfg, nil
}

// waitForContracts polls client.Put until the generic ScalarDL contracts are
// reachable, reporting elapsed time via progressFn on each failed attempt.
// Must be called after RegisterSecret so the entity is authenticated.
// Each attempt uses a fresh 5-second context; total wait is up to 5 minutes.
func waitForContracts(c *LedgerClient, progressFn func(string)) error {
	var lastErr error
	for i := 0; i < contractRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := c.Put(ctx, SetupTestObjectID, "0000000000000000000000000000000000000000000000000000000000000000")
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		if i < contractRetries-1 {
			elapsed := time.Duration(i+1) * contractRetryInterval
			progress(progressFn, fmt.Sprintf("  still waiting... (%ds elapsed) — %v", int(elapsed.Seconds()), err))
			time.Sleep(contractRetryInterval)
		}
	}
	total := time.Duration(contractRetries) * contractRetryInterval
	return fmt.Errorf("contracts not ready after %v: %w", total, lastErr)
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

// WipeHistory permanently deletes all audit records by truncating the
// ScalarDL ledger tables in the PostgreSQL container. The Docker stack
// continues running — entity registration, contracts, and config are
// all preserved, so logging resumes immediately after the wipe.
//
// The asset table schema is discovered at runtime via information_schema,
// because the ScalarDL schema loader may create the namespace under a name
// other than 'scalardl' depending on its configuration. After truncating,
// the ScalarDL ledger container is restarted to clear any in-memory state.
func WipeHistory(composePath string) error {
	// Step 1: Discover the schema that owns the 'asset' table.
	findSQL := `SELECT table_schema FROM information_schema.tables ` +
		`WHERE table_name = 'asset' ` +
		`AND table_schema NOT IN ('information_schema', 'pg_catalog') ` +
		`ORDER BY table_schema LIMIT 1`
	findCmd := exec.Command("docker", "compose", "-f", composePath,
		"exec", "-T", "postgres",
		"psql", "-U", "scalardl", "-d", "scalardl", "-tA", "-c", findSQL,
	)
	out, err := findCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("discovering ledger asset schema: %w\n%s", err, out)
	}
	assetSchema := strings.TrimSpace(string(out))
	if assetSchema == "" {
		// No asset table found — nothing to wipe.
		return nil
	}

	// Step 2: Truncate asset tables and, if present, coordinator.state.
	// ScalarDL stores tamper-evident hashes in asset_metadata alongside the
	// asset data table. Both must be truncated together — leaving asset_metadata
	// intact while clearing asset causes DL-LEDGER-305001 (inconsistent asset
	// and metadata) on the first contract execution after a wipe.
	sql := fmt.Sprintf(`TRUNCATE %[1]s.asset;
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables
             WHERE table_schema = '%[1]s' AND table_name = 'asset_metadata') THEN
    EXECUTE 'TRUNCATE %[1]s.asset_metadata';
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.tables
             WHERE table_schema = 'coordinator' AND table_name = 'state') THEN
    TRUNCATE coordinator.state;
  END IF;
END $$;`, assetSchema)
	wipeCmd := exec.Command("docker", "compose", "-f", composePath,
		"exec", "-T", "postgres",
		"psql", "-U", "scalardl", "-d", "scalardl", "-c", sql,
	)
	if output, err := wipeCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("clearing ledger history: %w\n%s", err, output)
	}

	// Step 3: Restart the ScalarDL ledger to flush any in-memory state so
	// subsequent GetHistory and Verify calls see the empty database.
	return runDockerCompose(composePath, "restart", "scalardl-ledger")
}

// StopStack runs `docker compose -f composePath stop` (preserves named volume)
// or `docker compose -f composePath down -v` when wipe is true.
func StopStack(composePath string, wipe bool) error {
	if wipe {
		return runDockerCompose(composePath, "down", "-v")
	}
	return runDockerCompose(composePath, "stop")
}

// EnsureStack starts the Docker audit stack synchronously, suitable for
// short-lived CLI processes where a goroutine would be killed on exit.
//
// It first checks whether the ledger is already reachable (2-second probe).
// If it is, it returns immediately with no overhead. If not, it:
//  1. Ensures the Docker daemon is running (starting it if needed, up to 60s)
//  2. Runs docker compose up -d
//  3. Waits for the ledger port to accept connections (up to 30s)
//
// progressFn receives one-line status strings at each step; it may be nil.
// Returns nil when the ledger is ready or when auto-start is not configured.
// Non-zero errors are non-fatal for callers — audit is optional.
func EnsureStack(cfg config.AuditConfig, progressFn func(string)) error {
	if cfg.DockerComposePath == "" || !cfg.AutoStart {
		return nil
	}

	// Quick probe: ledger already reachable — nothing to do.
	if client, err := NewClientFromConfig(cfg.Server, cfg.PrivilegedServer, cfg.EntityID, cfg.KeyVersion, cfg.SecretKey, cfg.Insecure); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		pingErr := client.Ping(ctx)
		cancel()
		_ = client.Close()
		if pingErr == nil {
			return nil
		}
	}

	_, _ = fmt.Fprintln(os.Stderr, "tegata: audit ledger is not running; starting it...")
	progress(progressFn, "Starting audit server...")
	if err := ensureDockerDaemon(); err != nil {
		return fmt.Errorf("docker daemon not ready: %w", err)
	}
	if err := StartStack(cfg.DockerComposePath); err != nil {
		return fmt.Errorf("starting audit stack: %w", err)
	}
	_, _ = fmt.Fprintln(os.Stderr, "tegata: waiting for ledger to become ready...")
	progress(progressFn, "Waiting for audit server to become ready...")
	if err := waitForLedger(cfg); err != nil {
		return fmt.Errorf("ledger did not become ready: %w", err)
	}
	progress(progressFn, "Audit server ready.")
	return nil
}

// MaybeAutoStart fires in a background goroutine when cfg.DockerComposePath
// is non-empty. It runs docker compose up -d then retries Ping up to
// autoStartRetries times. Non-blocking -- never panics, logs to stderr on
// failure. Suitable for long-lived processes (TUI, GUI) where the goroutine
// can complete after the unlock call returns. For short-lived CLI processes
// use EnsureStack instead. Per D-10 and D-13.
func MaybeAutoStart(cfg config.AuditConfig) {
	if cfg.DockerComposePath == "" || !cfg.AutoStart {
		return
	}
	go func() {
		if err := ensureDockerDaemon(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "tegata: audit auto-start: Docker daemon not ready: %v\n", err)
			return
		}
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
