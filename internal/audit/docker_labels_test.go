package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/pkg/model"
)

// bundleCompose reads the real embedded compose bundle so the tests catch a
// bundle that loses its label placeholders.
func bundleCompose(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "cmd", "tegata", "docker-bundle", "docker-compose.yml"))
	if err != nil {
		t.Fatalf("reading bundle: %v", err)
	}
	return data
}

func TestComposeBundles_HaveLabelPlaceholders(t *testing.T) {
	for _, p := range []string{
		"cmd/tegata/docker-bundle/docker-compose.yml",
		"cmd/tegata-gui/docker-bundle/docker-compose.yml",
		"deployments/docker-compose/docker-compose.yml",
	} {
		data, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(p)))
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		s := string(data)
		for _, key := range []string{"io.tegata.entity-id", "io.tegata.owner-hash", "io.tegata.owner-hash-scheme"} {
			if !strings.Contains(s, "  "+key+`: ""`) {
				t.Errorf("%s: missing placeholder for %s", p, key)
			}
		}
		if got := strings.Count(s, "labels: *tegata-labels"); got != 5 {
			t.Errorf("%s: %d services reference tegata-labels, want 5", p, got)
		}
	}
}

func TestRewriteComposeLabels(t *testing.T) {
	out := string(rewriteComposeLabels(bundleCompose(t), composeLabels{
		EntityID:        "tegata-abc12345",
		OwnerHash:       strings.Repeat("ab", 32),
		OwnerHashScheme: OwnerHashSchemeHMACSHA256,
	}))
	for _, want := range []string{
		`  io.tegata.entity-id: "tegata-abc12345"`,
		`  io.tegata.owner-hash: "` + strings.Repeat("ab", 32) + `"`,
		`  io.tegata.owner-hash-scheme: "hmac-sha256"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s", want)
		}
	}
}

func TestRewriteComposeLabels_RejectsUnsafeValues(t *testing.T) {
	in := bundleCompose(t)
	for _, bad := range []string{`x"` + "\n  evil: true", "has space", "a:b", strings.Repeat("a", 129)} {
		out := rewriteComposeLabels(in, composeLabels{OwnerHash: bad})
		if string(out) != string(in) {
			t.Errorf("unsafe value %q was written into the compose file", bad)
		}
	}
}

func TestRewriteComposeLabels_EmptyLeavesPlaceholders(t *testing.T) {
	in := bundleCompose(t)
	if out := rewriteComposeLabels(in, composeLabels{}); string(out) != string(in) {
		t.Error("empty labels should leave the compose file unchanged")
	}
}

func TestSyncDockerCompose_WritesOwnerLabels(t *testing.T) {
	fsys := fstest.MapFS{"docker-compose.yml": &fstest.MapFile{Data: bundleCompose(t)}}
	path := filepath.Join(t.TempDir(), "docker-compose.yml")
	owner := &model.OwnerIdentity{EmployeeIDHash: "1111", EmailHash: "2222", HashScheme: OwnerHashSchemeSHA256}
	auditCfg := config.AuditConfig{EntityID: "tegata-abc12345"}
	ApplyOwner(&auditCfg, owner)
	cfg := ownerLabels(auditCfg)

	// The project name differs from the entity ID, as in older configs that
	// set docker_project_name separately. The label must hold the entity ID.
	if err := syncDockerCompose(fsys, path, "legacy-project", "", cfg); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	s := string(data)
	if !strings.Contains(s, `io.tegata.owner-hash: "1111"`) {
		t.Error("primary owner hash (employee ID) not written")
	}
	if !strings.Contains(s, `io.tegata.entity-id: "tegata-abc12345"`) {
		t.Error("entity ID label not written")
	}
	if strings.Contains(s, "2222") {
		t.Error("only the primary owner hash belongs in container labels")
	}
}

func TestExtractComposeFiles_WritesOwnerLabels(t *testing.T) {
	fsys := fstest.MapFS{"docker-compose.yml": &fstest.MapFile{Data: bundleCompose(t)}}
	dir := t.TempDir()
	if err := extractComposeFiles(fsys, dir, "tegata-abc12345", "", composeLabels{OwnerHash: "3333", OwnerHashScheme: "sha256"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "docker-compose.yml"))
	if !strings.Contains(string(data), `io.tegata.owner-hash: "3333"`) {
		t.Error("owner hash label not written on extract")
	}
}
