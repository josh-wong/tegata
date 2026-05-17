# Build and release artifacts

This guide is for maintainers who publish downloadable release files for Tegata.

It documents the exact artifact names shipped to GitHub Releases, how the automated workflow builds them, and how to run the process manually when needed.

## Scope

This runbook covers release artifacts for all supported platforms:

- CLI binaries for Windows, macOS, and Linux
- GUI installer for Windows
- GUI DMG for macOS
- GUI `.deb` and `.rpm` packages for Linux
- Release checksums (`SHA256SUMS.txt`)

## Release artifacts

The release workflow publishes these files to the [Tegata Releases](https://github.com/josh-wong/tegata/releases) page:

| Artifact | Platform | Purpose |
|----------|----------|---------|
| `tegata-windows-amd64.exe` | Windows | CLI binary |
| `tegata-darwin-arm64` | macOS Apple Silicon | CLI binary |
| `tegata-darwin-amd64` | macOS Intel | CLI binary |
| `tegata-linux-amd64` | Linux | CLI binary |
| `tegata-gui-windows-amd64-setup.exe` | Windows | GUI installer (NSIS) |
| `tegata-gui-darwin-universal.dmg` | macOS | GUI disk image |
| `tegata-gui-linux-amd64.deb` | Linux (Debian/Ubuntu family) | GUI package |
| `tegata-gui-linux-amd64.rpm` | Linux (RHEL/Fedora family) | GUI package |
| `SHA256SUMS.txt` | All | Checksums for all uploaded artifacts |

## Automated release process (recommended)

The canonical release path is the GitHub Actions workflow in `.github/workflows/release.yml`.

### Trigger

The workflow runs on tag pushes matching `v*`.

```bash
git tag v1.0.0
git push origin v1.0.0
```

### What the workflow does

1. Builds CLI binaries for Windows, macOS (arm64 and amd64), and Linux.
2. Builds GUI artifacts per platform:
   - Windows NSIS installer
   - macOS universal app bundle
   - Linux `.deb` and `.rpm` via `nfpm`
3. Signs and notarizes macOS CLI binaries and GUI app, then packages and notarizes the DMG.
4. Collects artifacts, generates `SHA256SUMS.txt`, and publishes to GitHub Releases.

### Required secrets for macOS signing and notarization

Set these repository secrets before tagging releases:

- `APPLE_DEVELOPER_CERTIFICATE_P12_BASE64`
- `APPLE_DEVELOPER_CERTIFICATE_PASSWORD`
- `APPLE_ID`
- `APPLE_TEAM_ID`
- `APPLE_APP_SPECIFIC_PASSWORD`

For setup steps, see [Set up macOS code signing](admin/macos-code-signing.md).

## Manual build commands

Use this section for local verification or emergency fallback. The automated workflow is still the source of truth for official artifacts.

## Build CLI release binaries

From the repository root:

```bash
VERSION=v1.0.0
mkdir -p dist

CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build \
  -ldflags="-s -w -X main.version=$VERSION" \
  -o dist/tegata-windows-amd64.exe ./cmd/tegata/

CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build \
  -ldflags="-s -w -X main.version=$VERSION" \
  -o dist/tegata-darwin-arm64 ./cmd/tegata/

CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build \
  -ldflags="-s -w -X main.version=$VERSION" \
  -o dist/tegata-darwin-amd64 ./cmd/tegata/

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w -X main.version=$VERSION" \
  -o dist/tegata-linux-amd64 ./cmd/tegata/
```

## Build GUI release artifacts

Install Wails first by running the following command:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
```

### Windows GUI installer

Run on Windows by running the following commands from the repository root:

```powershell
cd cmd\tegata-gui
wails build --platform windows/amd64 -webview2 download -nsis -o tegata-gui
```

Copy the generated installer to the canonical release filename:

`tegata-gui-windows-amd64-setup.exe`

### macOS GUI DMG

Run on macOS by running the following commands from the repository root:

```bash
cd cmd/tegata-gui
wails build --platform darwin/universal -o tegata-gui
mv build/bin/tegata-gui.app build/bin/Tegata.app
```

Sign and notarize before distribution. Follow [Set up macOS code signing](admin/macos-code-signing.md) for:

- Certificate setup
- App signing
- Notarization
- DMG creation and stapling

Expected release filename:

`tegata-gui-darwin-universal.dmg`

### Linux GUI packages

Run on Linux (Ubuntu runner recommended) by running the following commands from the repository root:

```bash
sudo apt-get update
sudo apt-get install -y \
  build-essential pkg-config \
  libgtk-3-dev \
  libwebkit2gtk-4.1-dev \
  libayatana-appindicator3-dev \
  librsvg2-dev

cd cmd/tegata-gui
wails build --platform linux/amd64 -o tegata-gui

go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest
VERSION=1.0.0
sed -i "s/\${VERSION}/$VERSION/g" nfpm.yaml
nfpm package -p deb -f nfpm.yaml
nfpm package -p rpm -f nfpm.yaml
```

Rename outputs to canonical release filenames:

- `tegata-gui-linux-amd64.deb`
- `tegata-gui-linux-amd64.rpm`

## Create checksums

From the directory that contains the final artifacts, run the following command to generate `SHA256SUMS.txt`:

```bash
sha256sum * > SHA256SUMS.txt
```

Upload all artifacts and `SHA256SUMS.txt` to the [Tegata Releases](https://github.com/josh-wong/tegata/releases) page.

## Verification checklist

Before publishing, verify:

- Artifact names exactly match the release artifact list in this document.
- macOS binaries and DMG are signed and notarized.
- Linux packages install cleanly (`dpkg -i`, `rpm -i`).
- Windows installer launches and installs successfully.
- `SHA256SUMS.txt` includes all uploaded files.

## Related documents

- [Set up macOS code signing](admin/macos-code-signing.md)
- [.github/workflows/release.yml](.github/workflows/release.yml)
- [cmd/tegata-gui/nfpm.yaml](cmd/tegata-gui/nfpm.yaml)
