package audit

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net"
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
// `tegata ledger setup` to verify that the predefined HashStore contracts are registered.
// Using a constant avoids accumulating unique orphan objects on every run.
const SetupTestObjectID = "tegata-setup-probe"

// daemonPollRetries and daemonPollInterval control how long detectDocker
// waits for the Docker daemon to become ready after attempting an auto-start.
const (
	daemonPollRetries  = 30
	daemonPollInterval = 2 * time.Second
)

// knownDockerPaths lists well-known Docker binary locations that may not be
// in the PATH of a GUI app launched from Finder or Spotlight on macOS.
var knownDockerPaths = []string{
	"/usr/local/bin/docker",
	"/usr/bin/docker",
	"/Applications/Docker.app/Contents/Resources/bin/docker",
	"/opt/homebrew/bin/docker",
}

// auditPorts lists the TCP ports required by the Docker audit stack.
// Checked before starting to detect conflicts with another vault's running stack.
var auditPorts = []int{5432, 50051, 50052}

// checkPortsAvailable returns an error if any of the required audit ports are
// already bound. A port conflict means another vault's audit stack is already
// running; the error message tells the user how to resolve it.
func checkPortsAvailable() error {
	return checkPorts(auditPorts)
}

// checkPorts dials each port in the list and returns an error on the first one
// that is already bound. Separated from checkPortsAvailable to allow tests to
// inject arbitrary ports without touching system-reserved port numbers.
func checkPorts(ports []int) error {
	for _, port := range ports {
		// Use 127.0.0.1 explicitly — on macOS, "localhost" may resolve to ::1
		// (IPv6) while Docker Desktop binds ports to 127.0.0.1 (IPv4) only,
		// causing a false "port free" result on a live container.
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			_ = conn.Close()
			msg := fmt.Sprintf("Port %d is already in use. Stop the current vault's audit stack with \"tegata ledger stop\" before starting another.", port) //nolint:staticcheck // user-facing message requires sentence punctuation
			if runtime.GOOS == "windows" {
				msg += "\nIf no vault is running, PostgreSQL inside WSL2 may be using this port. To stop it, run:\n  wsl -- sudo service postgresql stop"
			}
			return fmt.Errorf("%s", msg) //nolint:staticcheck,govet // user-facing message
		}
	}
	return nil
}

// isDockerProjectRunning returns true when at least one container for the
// given Docker Compose project is currently running. It queries Docker
// directly using the com.docker.compose.project label rather than parsing
// compose output, so it works across Docker Compose v2 versions. Returns
// false when composePath doesn't exist, projectName is empty, or Docker is
// not available — all safe defaults that let callers fall through to startup.
func isDockerProjectRunning(composePath, projectName string) bool {
	if composePath == "" || projectName == "" {
		return false
	}
	if _, err := os.Stat(composePath); err != nil {
		return false
	}
	label := "com.docker.compose.project=" + projectName
	out, err := dockerCmd("ps", "--filter", "label="+label, "--quiet").Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

// CheckLedgerAvailability verifies that the audit ledger is accessible for
// the given config before a query is made. When DockerComposePath is set,
// it checks whether this vault's Docker project is actually running (not a
// different vault's stack that happens to occupy the same ports). Returns a
// descriptive error if the stack is absent or a port conflict is detected.
// When DockerComposePath is empty the ledger is assumed to be externally
// managed and no Docker check is performed.
func CheckLedgerAvailability(cfg config.AuditConfig) error {
	if cfg.DockerComposePath == "" {
		return nil
	}
	if isDockerProjectRunning(cfg.DockerComposePath, cfg.DockerProjectName) {
		return nil
	}
	if portErr := checkPortsAvailable(); portErr != nil {
		return portErr
	}
	return fmt.Errorf("Audit stack is not running. Start it with \"tegata ledger start\".") //nolint:staticcheck // user-facing message requires sentence punctuation
}

// DockerBinPath returns the absolute path to the docker binary. It first
// checks PATH (via LookPath), then falls back to known macOS and Linux
// locations. Returns an empty string if docker cannot be found. Exported
// for use in integration tests that need to detect whether Docker is present
// at a known location before simulating absence.
func DockerBinPath() string {
	return dockerBin()
}

// dockerBin returns the absolute path to the docker binary. It first checks
// PATH (via LookPath), then falls back to known macOS and Linux locations.
// Returns an empty string if docker cannot be found.
func dockerBin() string {
	if p, err := exec.LookPath("docker"); err == nil {
		return p
	}
	for _, p := range knownDockerPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// dockerCmd returns an exec.Cmd for the docker binary with the given args.
// Uses dockerBin() so the binary is found even when PATH is restricted (e.g.
// GUI apps launched from Finder that don't inherit the shell PATH).
func dockerCmd(args ...string) *exec.Cmd {
	bin := dockerBin()
	if bin == "" {
		bin = "docker" // fallback: will fail with a clear "not found" error
	}
	return exec.Command(bin, args...)
}

// detectDocker checks that Docker is installed, the daemon is running (starting
// it automatically if needed), and Compose v2 is available.
func detectDocker() error {
	if dockerBin() == "" {
		return fmt.Errorf("docker binary not found in PATH. Install Docker Desktop from https://docs.docker.com/get-docker/")
	}

	if err := ensureDockerDaemon(); err != nil {
		return err
	}

	if err := dockerCmd("compose", "version").Run(); err != nil {
		return fmt.Errorf("docker compose v2 plugin not available. Upgrade to Docker Desktop 3.4+ or Docker Engine 20.10+ with the compose plugin")
	}

	return nil
}

// ensureDockerDaemon verifies the Docker daemon is reachable. If not, it
// attempts a platform-specific auto-start and polls until ready or timeout.
func ensureDockerDaemon() error {
	if dockerCmd("info").Run() == nil {
		return nil
	}

	// Daemon not running — attempt auto-start.
	_ = startDockerDaemon() // best-effort; ignore launch error and poll instead

	for i := 0; i < daemonPollRetries; i++ {
		time.Sleep(daemonPollInterval)
		if dockerCmd("info").Run() == nil {
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

// syncDockerCompose reads docker-compose.yml from fsys and writes it to
// composePath on disk, keeping the live file in sync with the embedded bundle.
// Called by EnsureStack and MaybeAutoStart so that binary upgrades
// automatically update the running compose configuration without requiring
// the user to re-run `tegata ledger start`. When entityID is non-empty the
// global volume name is rewritten to a per-vault name before writing.
func syncDockerCompose(fsys fs.FS, composePath, entityID string) error {
	data, err := fs.ReadFile(fsys, "docker-compose.yml")
	if err != nil {
		return fmt.Errorf("reading embedded docker-compose.yml: %w", err)
	}
	if entityID != "" {
		rewritten, ok := rewriteComposeVolume(data, entityID)
		if !ok {
			_, _ = fmt.Fprintf(os.Stderr, "tegata: warning: volume name %q not found in docker-compose.yml; per-vault isolation may not be applied\n", "tegata-scalardl-data")
		}
		data = rewritten
	}
	return os.WriteFile(composePath, data, 0600)
}

// extractComposeFiles walks the provided fs.FS and writes each file to
// targetDir, preserving the directory structure. When entityID is non-empty,
// docker-compose.yml is rewritten with a per-vault volume name so each vault's
// PostgreSQL data is stored in an isolated Docker named volume.
func extractComposeFiles(fsys fs.FS, targetDir, entityID string) error {
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

		if entityID != "" && path == "docker-compose.yml" {
			rewritten, ok := rewriteComposeVolume(data, entityID)
			if !ok {
				_, _ = fmt.Fprintf(os.Stderr, "tegata: warning: volume name %q not found in docker-compose.yml; per-vault isolation may not be applied\n", "tegata-scalardl-data")
			}
			data = rewritten
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

// ComposeDirForVault returns the per-vault Docker Compose directory path.
// Directory is named after the vault's entity ID so the correspondence is
// explicit: ~/.tegata/docker/tegata-<slug>/
func ComposeDirForVault(homeDir, vaultID string) string {
	return filepath.Join(homeDir, ".tegata", "docker", entityIDFromVaultID(vaultID))
}

// rewriteComposeVolume replaces the global volume name in docker-compose.yml
// content with a per-vault name. Returns the rewritten data and true if the
// substitution was made, or the original data and false if the target string
// was not found (compose file format may have drifted from expectations).
// entityID must be non-empty.
func rewriteComposeVolume(data []byte, entityID string) ([]byte, bool) {
	target := []byte("name: tegata-scalardl-data")
	if !bytes.Contains(data, target) {
		return data, false
	}
	return bytes.ReplaceAll(data, target, []byte("name: "+entityID+"-scalardl-data")), true
}

// runDockerCompose executes a docker compose command with the given compose
// file path, optional project name, and arguments. Returns an error with
// stdout+stderr on failure. When projectName is non-empty, --project-name is
// passed to override the name: directive in the compose file.
func runDockerCompose(composePath, projectName string, args ...string) error {
	cmdArgs := []string{"compose", "-f", composePath}
	if projectName != "" {
		cmdArgs = append(cmdArgs, "--project-name", projectName)
	}
	cmdArgs = append(cmdArgs, args...)
	cmd := dockerCmd(cmdArgs...)

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
//  5. start infrastructure services (postgres, schema-loader, coordinator, ledger)
//  6. wait for ledger (up to 30s)
//  7. RegisterSecret + Ping
//  8. start scalardl-contract-registration, wait for it to exit via `docker compose wait` (up to 10 minutes)
//
// progressFn receives one-line status strings as each step completes; it may be
// nil (no progress reporting). fsys must contain docker-compose.yml and
// certs/client.properties at its root.
//
// Per D-12: if Docker is absent, returns a descriptive error and does NOT
// modify any config. tegata.toml is only written by the caller on success.
// SetupStack runs the full Docker audit setup sequence. It starts Docker, extracts
// compose files, generates entity credentials, starts the stack, waits for the
// ledger, registers the entity secret, and then waits for the predefined HashStore contracts
// to be reachable (up to 5 minutes). After the entity is registered and the
// ledger is reachable, onRegistered is called with the populated AuditConfig —
// the caller should write tegata.toml at that point so config is persisted even
// if contract registration is still in progress. If onRegistered returns an
// error, SetupStack returns immediately with that error.
func SetupStack(fsys fs.FS, composeDir, vaultID string, progressFn func(string), onRegistered func(config.AuditConfig) error) (config.AuditConfig, error) {
	if fsys == nil {
		return config.AuditConfig{}, fmt.Errorf("compose bundle filesystem is nil")
	}

	// Step 1: Check Docker installation.
	progress(progressFn, "Checking Docker installation...")
	if err := detectDocker(); err != nil {
		return config.AuditConfig{}, err
	}

	// Step 2: Derive entity ID and extract compose files with per-vault volume name.
	entityID := entityIDFromVaultID(vaultID)
	progress(progressFn, "Extracting compose files to "+composeDir+"...")
	if err := os.MkdirAll(composeDir, 0700); err != nil {
		return config.AuditConfig{}, fmt.Errorf("creating compose directory: %w", err)
	}
	if err := extractComposeFiles(fsys, composeDir, entityID); err != nil {
		return config.AuditConfig{}, fmt.Errorf("extracting compose files: %w", err)
	}

	// Step 3: Generate secret key.
	secretKey, err := generateSecretKey()
	if err != nil {
		return config.AuditConfig{}, err
	}

	// Step 4: Write client.properties.
	progress(progressFn, "Generating audit credentials...")
	if err := writeClientProperties(composeDir, entityID, secretKey); err != nil {
		return config.AuditConfig{}, fmt.Errorf("writing client properties: %w", err)
	}

	// Step 5: Start infrastructure services only — do NOT start
	// scalardl-contract-registration yet. The bootstrap tool inside that
	// container downloads ~50 MB, then calls scalardl-hashstore bootstrap
	// which needs (a) the ledger to be fully ready and (b) the entity secret
	// to be registered. On Windows, the full startup chain (postgres
	// healthcheck → schema-loader → coordinator schema → ledger JVM init)
	// can exceed the container's internal 60-second nc wait loop, causing
	// bootstrap to fail and exhaust its on-failure:3 restart budget before
	// contracts are ever registered. By starting contract-registration only
	// after the Go code confirms the ledger is ready and the entity is
	// registered, bootstrap succeeds on the first attempt.
	composePath := filepath.Join(composeDir, "docker-compose.yml")
	progress(progressFn, "Starting Docker stack...")
	if err := checkPortsAvailable(); err != nil {
		return config.AuditConfig{}, err
	}
	if err := runDockerCompose(composePath, entityID, "up", "-d",
		"postgres", "scalardl-schema-loader", "scalardl-coordinator-schema", "scalardl-ledger"); err != nil {
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
		DockerProjectName: entityID,
	}

	if err := waitForLedger(cfg); err != nil {
		return config.AuditConfig{}, fmt.Errorf("waiting for ledger: %w", err)
	}

	// Step 7: Register entity secret and verify the ledger is reachable.
	progress(progressFn, "Registering audit credentials...")
	client, err := NewClientFromConfig(cfg)
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

	// Step 8a: Now that the ledger is fully ready and the entity secret is
	// registered, start the contract-registration container. Bootstrap will
	// find the ledger immediately reachable and the entity already registered,
	// so it succeeds on the first attempt without exhausting its retry budget.
	progress(progressFn, "Registering contracts for entity...")
	if err := runDockerCompose(composePath, entityID, "up", "-d", "scalardl-contract-registration"); err != nil {
		return config.AuditConfig{}, fmt.Errorf("starting contract registration: %w", err)
	}

	// Notify the caller that credentials are registered and the config is ready.
	// The caller should persist tegata.toml here so audit is configured even if
	// contract registration below is still in progress.
	if onRegistered != nil {
		if err := onRegistered(cfg); err != nil {
			return config.AuditConfig{}, err
		}
	}

	// Step 8b: Wait for the contract-registration container to finish.
	// `docker compose wait` blocks until the service exits and reflects its exit
	// code, so we know exactly when bootstrap completed and whether it succeeded.
	// On Windows, package installation + 50 MB SDK download from GitHub + JVM
	// startup can exceed 5 minutes, which caused the previous polling approach
	// (30 × 10s = 5 min) to time out while bootstrap was still running.
	progress(progressFn, "Waiting for contract registration to complete (up to 10 minutes on first run)...")
	if err := waitForBootstrap(composePath, entityID, 10*time.Minute); err != nil {
		return config.AuditConfig{}, fmt.Errorf("contract registration: %w", err)
	}

	// Verify contracts are reachable with a single Put call.
	ctxVerify, cancelVerify := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelVerify()
	if err := client.Put(ctxVerify, SetupTestObjectID, "0000000000000000000000000000000000000000000000000000000000000000"); err != nil {
		return config.AuditConfig{}, fmt.Errorf("contract verification: %w", err)
	}

	return cfg, nil
}

// waitForBootstrap waits for the scalardl-contract-registration service to exit
// successfully using `docker compose wait` (requires Docker Compose v2.4+,
// included in Docker Desktop 4.9+ released April 2022). This is more reliable
// than polling client.Put because it directly observes whether the bootstrap
// script completed rather than inferring readiness from RPC call results.
// Returns an error if the container exits non-zero or the timeout is reached.
func waitForBootstrap(composePath, projectName string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmdArgs := []string{"compose", "-f", composePath}
	if projectName != "" {
		cmdArgs = append(cmdArgs, "--project-name", projectName)
	}
	cmdArgs = append(cmdArgs, "wait", "scalardl-contract-registration")

	bin := dockerBin()
	if bin == "" {
		bin = "docker"
	}
	cmd := exec.CommandContext(ctx, bin, cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("timed out after %v; to diagnose, run: docker logs %s-scalardl-contract-registration-1", timeout, projectName)
		}
		return fmt.Errorf("bootstrap container exited with error: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// waitForLedger retries connecting to the ledger up to autoStartRetries times
// with autoStartInterval between attempts. Returns nil on first successful
// ping, or an error if all retries are exhausted.
func waitForLedger(cfg config.AuditConfig) error {
	var lastErr error
	for i := 0; i < autoStartRetries; i++ {
		client, err := NewClientFromConfig(cfg)
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
// projectName, when non-empty, is passed as --project-name to scope containers
// and volumes to this vault's stack. Returns a descriptive error if any
// required port is already occupied, or if Docker fails on non-zero exit.
func StartStack(composePath, projectName string) error {
	if err := checkPortsAvailable(); err != nil {
		return err
	}
	return runDockerCompose(composePath, projectName, "up", "-d")
}

// StopStack runs `docker compose -f composePath stop`, preserving the named
// volume so audit history is retained. projectName scopes the command to the
// correct stack when non-empty.
func StopStack(composePath, projectName string) error {
	return runDockerCompose(composePath, projectName, "stop")
}

// TeardownStack runs `docker compose -f composePath down -v`, removing
// containers and the named volume. Use this only in integration tests for
// post-test cleanup — it permanently deletes all audit history.
func TeardownStack(composePath, projectName string) error {
	return runDockerCompose(composePath, projectName, "down", "-v")
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
// fsys should be the embedded docker bundle FS (after fs.Sub to remove the
// bundle prefix). When non-nil, docker-compose.yml is synced from the bundle
// before starting so binary upgrades take effect without re-running setup.
func EnsureStack(cfg config.AuditConfig, fsys fs.FS, progressFn func(string)) error {
	if cfg.DockerComposePath == "" || !cfg.AutoStart {
		return nil
	}

	// Sync docker-compose.yml from the embedded bundle so binary upgrades
	// (e.g. ScalarDL version bumps) take effect automatically.
	if fsys != nil {
		if err := syncDockerCompose(fsys, cfg.DockerComposePath, cfg.DockerProjectName); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "tegata: warning: could not sync docker-compose.yml: %v\n", err)
		}
	}

	// Quick probe: only skip startup when THIS vault's Docker project is
	// confirmed running. Without the project check, the ping could succeed
	// against a different vault's stack on the same ports, causing subsequent
	// queries to silently target the wrong entity and return no events.
	if isDockerProjectRunning(cfg.DockerComposePath, cfg.DockerProjectName) {
		if client, err := NewClientFromConfig(cfg); err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			pingErr := client.Ping(ctx)
			cancel()
			_ = client.Close()
			if pingErr == nil {
				return nil
			}
		}
	}

	_, _ = fmt.Fprintln(os.Stderr, "tegata: audit ledger is not running; starting it...")
	progress(progressFn, "Starting audit server...")
	if err := ensureDockerDaemon(); err != nil {
		return fmt.Errorf("docker daemon not ready: %w", err)
	}
	if err := StartStack(cfg.DockerComposePath, cfg.DockerProjectName); err != nil {
		return fmt.Errorf("starting audit stack: %w", err)
	}
	_, _ = fmt.Fprintln(os.Stderr, "tegata: waiting for ledger to become ready...")
	progress(progressFn, "Waiting for audit server to become ready...")
	if err := waitForLedger(cfg); err != nil {
		return fmt.Errorf("ledger did not become ready: %w", err)
	}
	progress(progressFn, "Audit server ready.")
	_, _ = fmt.Fprintln(os.Stderr, "tegata: ledger stack started. Run 'tegata ledger stop' to shut it down when finished.")
	return nil
}

// RunAutoStart performs the Docker audit stack auto-start synchronously.
// It syncs docker-compose.yml from fsys (when non-nil), starts the stack,
// and waits for the ledger to become reachable. Returns an error on failure
// so callers can route it through their own error channel rather than writing
// to stderr. A nil return means the ledger is ready.
// When cfg.DockerComposePath is empty or cfg.AutoStart is false, it is a no-op.
func RunAutoStart(cfg config.AuditConfig, fsys fs.FS) error {
	if cfg.DockerComposePath == "" || !cfg.AutoStart {
		return nil
	}
	if fsys != nil {
		if err := syncDockerCompose(fsys, cfg.DockerComposePath, cfg.DockerProjectName); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "tegata: warning: could not sync docker-compose.yml: %v\n", err)
		}
	}
	if err := ensureDockerDaemon(); err != nil {
		return fmt.Errorf("docker daemon not ready: %w", err)
	}
	if err := StartStack(cfg.DockerComposePath, cfg.DockerProjectName); err != nil {
		return err
	}
	for i := 0; i < autoStartRetries; i++ {
		client, err := NewClientFromConfig(cfg)
		if err != nil {
			time.Sleep(autoStartInterval)
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		pingErr := client.Ping(ctx)
		cancel()
		_ = client.Close()
		if pingErr == nil {
			return nil
		}
		time.Sleep(autoStartInterval)
	}
	return fmt.Errorf("ledger did not become ready after %d attempts", autoStartRetries)
}

// MaybeAutoStart fires in a background goroutine when cfg.DockerComposePath
// is non-empty. Non-blocking — never panics, logs to stderr on failure.
// Suitable for non-TUI contexts (GUI, background CLI) where stderr output
// is acceptable. For TUI processes use RunAutoStart via a tea.Cmd instead
// so errors route through the message loop rather than corrupting the renderer.
// Per D-10 and D-13.
func MaybeAutoStart(cfg config.AuditConfig, fsys fs.FS) {
	if cfg.DockerComposePath == "" || !cfg.AutoStart {
		return
	}
	go func() {
		if err := RunAutoStart(cfg, fsys); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "tegata: audit auto-start failed: %v\n", err)
		}
	}()
}
