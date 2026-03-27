package audit

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// detectDocker checks that Docker and Docker Compose v2 are installed and
// accessible in the system PATH. Returns nil if both are found.
func detectDocker() error {
	_, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("Docker is not installed or not in PATH. Install Docker from https://docs.docker.com/get-docker/")
	}

	cmd := exec.Command("docker", "compose", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Docker Compose v2 plugin is not available. Upgrade Docker to a version that includes Compose v2 (Docker Desktop 3.4+ or Docker Engine 20.10+ with compose plugin)")
	}

	return nil
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

	// Take the first segment before any dash, or the first 8 chars.
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
