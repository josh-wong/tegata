# Getting started with Tegata

Tegata is a portable encrypted authenticator that runs from USB drives or microSD cards. This guide walks you through installation, vault creation, and generating your first authentication code.

## Prerequisites

You need the following before getting started.

- **Go 1.25+** if building from source (not needed for pre-built binaries)
- **USB drive or microSD card** formatted as FAT32 or exFAT for portable use
- **A TOTP-compatible service** (such as GitHub, Google, or any service that provides an otpauth:// URI or base32 secret)

## Installation

Tegata can be installed using pre-built binaries or built from source.

### Pre-built binaries

Download the binary for your platform from the [GitHub Releases](https://github.com/josh-wong/tegata/releases) page.

- `tegata-windows-amd64.exe` for Windows (10+)
- `tegata-darwin-arm64` for macOS (Apple Silicon)
- `tegata-darwin-amd64` for macOS (Intel)
- `tegata-linux-amd64` for Linux

On macOS and Linux, mark the binary as executable after downloading:

```bash
chmod +x tegata-darwin-arm64
```

On Windows, PowerShell does not search the current directory for executables. If you are running Tegata from the same directory as the binary, prefix the command with `.\`:

```powershell
.\tegata.exe ui
```

Alternatively, add the directory containing the binary to your `PATH` so you can run `tegata` from anywhere.

Copy the binary to your USB drive so it travels with your vault.

### Desktop GUI

A graphical interface is also available for each platform.

- `tegata-gui-windows-amd64-setup.exe` for Windows (NSIS installer)
- `tegata-gui-darwin-universal.dmg` for macOS (drag to Applications)
- `tegata-gui-linux-amd64.deb` for Debian/Ubuntu
- `tegata-gui-linux-amd64.rpm` for Fedora/RHEL

Download the installer for your platform from the [GitHub Releases](https://github.com/josh-wong/tegata/releases) page. The GUI provides the same vault management and credential operations as the CLI with a visual interface, including a setup wizard, live TOTP countdown, and settings panel.

### Build from source

Clone the repository and build with Make:

```bash
git clone https://github.com/josh-wong/tegata.git
cd tegata
make build
```

The binary is placed in `bin/tegata`. Copy it to your USB drive alongside the vault.

## Quickstart

Follow these numbered steps to create a vault, add a credential, and generate your first TOTP code.

1. **Initialize a vault** on your USB drive. Tegata prompts you to create a passphrase (minimum 8 characters) and displays a recovery key.

   ```bash
   tegata init /mnt/usb
   ```

   Output:

   ```
   Vault created: /mnt/usb/vault.tegata

   Recovery key (store this somewhere safe -- you cannot see it again):

       ABCD-EFGH-IJKL-MNOP-QRST-UVWX-YZ23-4567

   If you forget your passphrase, this key is the only way to recover your vault.
   ```

   Store the recovery key in a separate, secure location (such as a password manager or printed paper stored safely). You cannot retrieve it later.

2. **Add a credential** to the vault. You can provide the secret manually or paste an otpauth:// URI.

   Using manual entry:

   ```bash
   tegata add GitHub --type totp --issuer GitHub --vault /mnt/usb
   ```

   Tegata prompts for the base32 secret interactively (input is hidden).

   Using an otpauth:// URI:

   ```bash
   tegata add GitHub --scan --vault /mnt/usb
   ```

   Tegata prompts you to paste the `otpauth://` URI. The type, issuer, algorithm, digits, and period are all parsed automatically from the URI.

3. **Generate a code** for the credential you just added.

   ```bash
   tegata code GitHub --vault /mnt/usb
   ```

   Output:

   ```
   482901
   Expires in 18s
   Copied to clipboard (auto-clear in 45s)
   ```

   The code is copied to your clipboard automatically and cleared after 45 seconds.

4. **List all credentials** in the vault.

   ```bash
   tegata list --vault /mnt/usb
   ```

   Credentials are grouped by tag. Untagged credentials appear under `[untagged]`.

## Daily use

The typical workflow is straightforward: plug in your USB drive, run `tegata code <label>`, and paste the code where needed. The clipboard auto-clears after 45 seconds.

To avoid typing `--vault /mnt/usb` on every command, set the `TEGATA_VAULT` environment variable:

```bash
export TEGATA_VAULT=/mnt/usb
```

With this variable set, all commands find the vault automatically:

```bash
tegata code GitHub
tegata list
```

The vault resolution order is:

1. `--vault` flag (highest priority)
2. `TEGATA_VAULT` environment variable
3. `./vault.tegata` in the current directory

## Terminal UI

Tegata includes an interactive terminal interface for users who prefer a guided experience. Launch it with:

```bash
tegata ui
```

If no vault is found, the TUI launches a setup wizard that guides you through vault creation. If a vault exists, it prompts you to unlock it.

The TUI provides visual countdown timers for TOTP codes, keyboard navigation for credential selection, and access to settings like export and import.

## Optional audit logging

Tegata supports optional tamper-evident logging of authentication events via ScalarDL Ledger. When enabled, every code generation, password retrieval, and challenge-response signing is recorded in an immutable audit log.

This feature requires a running ScalarDL Ledger instance. For setup instructions, see [ScalarDL setup guide](scalardl-setup.md).

## Configuration

Tegata stores its configuration in `tegata.toml` alongside the vault file. The `tegata init` command creates a default configuration file with commented-out settings. Key configuration options include clipboard auto-clear timeout (default 45 seconds) and vault idle timeout (default 5 minutes).

View the current effective configuration with:

```bash
tegata config show
```

## Next steps

For complete documentation of every CLI command, flags, and examples, see the [CLI command reference](cli-reference.md).
