# Tegata (手形)

**Your 2FA codes and credentials, encrypted and portable.**

![CI](https://github.com/josh-wong/tegata/actions/workflows/ci.yml/badge.svg)
![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)
![Go 1.25+](https://img.shields.io/badge/Go-1.25%2B-00ADD8.svg)

Tegata is an open-source portable authenticator that stores your two-factor authentication (2FA) codes and other credentials in an encrypted vault on standard USB drives or microSD cards. It combines TOTP/HOTP code generation, challenge-response signing, and static password storage with AES-256-GCM encryption and Argon2id key derivation. Optional tamper-evident audit logging via ScalarDL Ledger provides verifiable authentication history.

## Feature highlights

Tegata provides a low-cost alternative to hardware security keys with portable encrypted credential storage. While optimized for two-factor authentication (2FA) codes, it also supports other credential types:

- **TOTP and HOTP code generation** per RFC 6238 and RFC 4226
- **Static password storage** with clipboard auto-clear
- **Challenge-response signing** with HMAC-SHA256/SHA1
- **AES-256-GCM encrypted vault** on USB/microSD
- **Argon2id key derivation** (time=3, memory=64 MiB, parallelism=4)
- **memguard protected memory** for key material
- **Cross-platform:** Windows 10+, macOS 12+, Linux (single static binary)
- **Terminal UI** with setup wizard, visual countdown, and keyboard navigation
- **Desktop GUI** via Wails with React frontend
- **Recovery key** for vault access if passphrase is lost
- **Tag-based credential organization** with filtered listing
- **Export and import** encrypted backup files
- **Optional ScalarDL Ledger integration** for tamper-evident audit logging with hash-chain verification
- **Offline event queue** stores audit events when the ledger is unreachable

OTP algorithm standards in Tegata follow the RFCs and common provisioning behavior:

- **HOTP** uses **SHA-1 only** (RFC 4226).
- **TOTP** supports **SHA-1, SHA-256, and SHA-512** (RFC 6238).
- For manual TOTP entry, default to **SHA-1** unless your provider's `otpauth://` URI explicitly specifies a different algorithm.

## Installation

Tegata can be installed using pre-built binaries or built from source.

### Pre-built binaries

Download the binary for your platform from the [Releases](https://github.com/josh-wong/tegata/releases) page. On macOS and Linux, mark the binary as executable with `chmod +x`.

For GUI downloads, use the platform-specific release artifacts on the same page:

- Windows: `tegata-gui-windows-amd64-setup.exe`
- macOS: `tegata-gui-darwin-universal.dmg`
- Linux: `tegata-gui-linux-amd64.deb` or `tegata-gui-linux-amd64.rpm`

If you are a maintainer creating release artifacts, see [Build and release artifacts](admin/build-and-release.md) for the full cross-platform build, packaging, signing, and publishing runbook.

### Build from source

For source builds, the minimum prerequisites are:

- Go 1.25 or later for CLI/TUI builds
- Node.js 20 or later and Wails v2 for GUI builds

For full platform-specific prerequisites and step-by-step instructions, see [Build the CLI and TUI from source](https://josh-wong.github.io/tegata/docs/quickstart/#build-the-cli-and-tui-from-source) or [Build the desktop GUI from source](https://josh-wong.github.io/tegata/docs/quickstart/#build-the-desktop-gui-from-source).

To build from source, clone the repository and run `make build`:

```bash
git clone https://github.com/josh-wong/tegata.git
cd tegata
make build
```

The binary is placed in `bin/tegata`. Copy it to your USB drive alongside the vault.

> [!NOTE]
> 
> On Windows, run `.\bin\tegata.exe` from the repository root, or copy `bin\tegata.exe` to a directory on your user `PATH` (for example, `%USERPROFILE%\bin`) and add that directory to `PATH`.
>
> On macOS and Linux, install the binary to a directory on your `PATH` to run `tegata` directly by running the following command:
>
> ```bash
> sudo make install
> ```
>
> By default this installs to `/usr/local/bin/tegata`. You can use a user-local path instead with `make install PREFIX=$HOME/.local`.

## Quickstart

Three steps to start generating authentication codes.

1. **Initialize a vault** on your USB drive:

   ```bash
   tegata init /mnt/usb
   ```

   Store the displayed recovery key in a separate secure location.

2. **Add a credential** (secret is prompted interactively):

   ```bash
   tegata add GitHub --type totp --issuer GitHub --vault /mnt/usb
   ```

3. **Generate a code:**

   ```bash
   tegata code GitHub --vault /mnt/usb
   ```

Set `TEGATA_VAULT=/mnt/usb` to avoid repeating the `--vault` flag. For the full walkthrough, see the [getting started guide](docs/quick-start.md).

## Screenshots

<!-- Screenshots will be added before the v1.0 release. -->

## Security model

Tegata is a software authenticator with portable key storage, not a hardware security key replacement. Keys are encrypted at rest with AES-256-GCM and decrypted in host memory during use. Sensitive memory is zeroed immediately after use via memguard. Rate limiting with exponential backoff protects against brute-force passphrase attempts.

For end-user hardening guidance, see [security best practices](site/docs/security-best-practices.mdx).

## Documentation

| Document                                               | Description                                       |
|--------------------------------------------------------|---------------------------------------------------|
| [Quickstart](docs/quickstart.md)                      | Installation, quickstart, and daily use            |
| [CLI command reference](docs/cli-reference.md)         | Complete documentation of all CLI commands          |
| [Enable audit logging](site/docs/scalardl-setup.mdx)   | Configure optional tamper-evident audit logging    |
| [Security best practices](site/docs/security-best-practices.mdx) | User-facing security and operational guidance |
| [Design document](docs/v1-design-doc.md)               | Technical architecture and component specifications |
| [Product requirements](docs/v1-product-requirements-doc.md) | Requirements, use cases, and release plan     |
| [Contributing](CONTRIBUTING.md)                        | Development setup, coding standards, and PR process |

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, coding standards, commit conventions, and the pull request process.

## License

MIT. See [LICENSE](LICENSE) for the full license text.
