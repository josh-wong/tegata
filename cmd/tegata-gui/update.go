package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// updatePrefs stores the user's update notification dismissal preferences.
// It is persisted to ~/.tegata/update-prefs.json and is independent of any
// individual vault.
type updatePrefs struct {
	// DismissedVersion is the release tag the user last dismissed (e.g. "v1.2.0").
	DismissedVersion string `json:"dismissed_version,omitempty"`
	// RemindAfter is a date-based reminder: re-show the notification after this
	// time even if the same version is still the latest. Zero means "never remind
	// for this version" (i.e. the user chose "not until next release").
	RemindAfter time.Time `json:"remind_after,omitempty"`
	// LastChecked is the timestamp of the most recent background update check.
	// Background checks are skipped if this is less than one hour ago.
	LastChecked time.Time `json:"last_checked,omitempty"`
}

func updatePrefsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".tegata", "update-prefs.json"), nil
}

func loadUpdatePrefs() updatePrefs {
	path, err := updatePrefsPath()
	if err != nil {
		return updatePrefs{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return updatePrefs{}
	}
	var prefs updatePrefs
	if err := json.Unmarshal(data, &prefs); err != nil {
		return updatePrefs{}
	}
	return prefs
}

func saveUpdatePrefs(prefs updatePrefs) error {
	path, err := updatePrefsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// dismissOption values for DismissUpdate.
const (
	DismissOptionTomorrow    = "tomorrow"
	DismissOptionOneMonth    = "one_month"
	DismissOptionNextRelease = "next_release"
)

// githubRelease is a minimal struct for the GitHub Releases API response.
type githubRelease struct {
	TagName    string `json:"tag_name"`
	HTMLURL    string `json:"html_url"`
	Body       string `json:"body"`
	Prerelease bool   `json:"prerelease"`
}

const githubReleasesURL = "https://api.github.com/repos/josh-wong/tegata/releases/latest"

// fetchLatestRelease queries the GitHub Releases API for the latest release.
func fetchLatestRelease() (*githubRelease, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, githubReleasesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "tegata-gui/"+version)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	// /releases/latest should never return a pre-release, but guard anyway.
	if release.Prerelease {
		return nil, fmt.Errorf("latest release %q is a pre-release", release.TagName)
	}
	return &release, nil
}

// isDismissed reports whether the notification for the given release tag should
// be suppressed based on stored preferences.
func isDismissed(prefs updatePrefs, latestTag string) bool {
	// Normalize both sides so "v1.2.0" and "1.2.0" compare equal regardless of
	// whether the stored value came from the raw GitHub tag or the UI version string.
	if strings.TrimPrefix(prefs.DismissedVersion, "v") != strings.TrimPrefix(latestTag, "v") {
		return false
	}
	// Same version was dismissed. If RemindAfter is set, only suppress until that time.
	if !prefs.RemindAfter.IsZero() {
		return time.Now().Before(prefs.RemindAfter)
	}
	// No RemindAfter means "skip this version entirely" (next-release dismissal).
	return true
}

// isNewerVersion reports whether the latest tag represents a newer release than
// the running build. Returns false when the current version is "dev" (unbuilt).
func isNewerVersion(current, latest string) bool {
	current = strings.TrimPrefix(strings.TrimSpace(current), "v")
	latest = strings.TrimPrefix(strings.TrimSpace(latest), "v")

	if current == "dev" || current == "" || latest == "" {
		return false
	}
	return compareSemver(latest, current) > 0
}

// compareSemver compares two semver strings without a "v" prefix.
// Returns 1 if a > b, -1 if a < b, 0 if equal.
func compareSemver(a, b string) int {
	ap := parseSemver(a)
	bp := parseSemver(b)
	for i := range ap {
		if ap[i] > bp[i] {
			return 1
		}
		if ap[i] < bp[i] {
			return -1
		}
	}
	return 0
}

func parseSemver(v string) [3]int {
	// Strip pre-release and build metadata suffixes.
	v = strings.SplitN(v, "-", 2)[0]
	v = strings.SplitN(v, "+", 2)[0]
	parts := strings.SplitN(v, ".", 3)
	var out [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		_, _ = fmt.Sscanf(parts[i], "%d", &out[i])
	}
	return out
}
