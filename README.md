# Tegata

<p align="center"><strong>Your 2FA codes and credentials, encrypted and portable</strong></p>

<p align="center">
	<a href="https://github.com/josh-wong/tegata/releases"><img src="https://img.shields.io/github/v/release/josh-wong/tegata" alt="Latest Release"></a>
	<img src="https://github.com/josh-wong/tegata/actions/workflows/ci.yml/badge.svg" alt="CI">
	<a href="https://github.com/josh-wong/tegata/actions/workflows/release.yml"><img src="https://github.com/josh-wong/tegata/actions/workflows/release.yml/badge.svg" alt="Release Workflow"></a>
	<br />
	<a href="https://github.com/josh-wong/tegata/actions/workflows/deploy.yml"><img src="https://github.com/josh-wong/tegata/actions/workflows/deploy.yml/badge.svg" alt="Docs Deploy"></a>
	<img src="https://img.shields.io/badge/Go-1.25%2B-00ADD8.svg" alt="Go 1.25+">
	<a href="https://goreportcard.com/report/github.com/josh-wong/tegata"><img src="https://goreportcard.com/badge/github.com/josh-wong/tegata" alt="Go Report Card"></a>
	<img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT">
</p>

Tegata is an open-source portable authenticator that stores your two-factor authentication (2FA) codes and other credentials in an encrypted vault on standard USB drives or microSD cards. It combines TOTP/HOTP code generation, challenge-response signing, and static password storage with AES-256-GCM encryption and Argon2id key derivation. Optional tamper-evident audit logging via [ScalarDL Ledger](https://github.com/scalar-labs/scalardl) provides verifiable authentication history.

> [!NOTE]
>
> "Tegata" (手形) is a Japanese term for a written certificate, historically required to pass through checkpoints. The name reflects the project's goal of providing a secure, portable solution for your digital credentials.

## Feature highlights

Tegata provides a low-cost alternative to hardware security keys with portable encrypted credential storage. It supports multiple credential types and includes these capabilities:

**Credential types**

- **TOTP and HOTP code generation**
- **Static password storage**
- **Challenge-response signing**

**Vault and encryption**

- **AES-256-GCM encrypted vault** on USB/microSD
- **Argon2id key derivation** (time=3, memory=64 MiB, parallelism=4)
- **memguard protected memory** for key material
- **Recovery key** for vault access if passphrase is lost

**Interfaces and organization**

- **Terminal UI** with setup wizard, visual countdown, and keyboard navigation
- **Desktop GUI** via Wails with React frontend
- **Tag-based credential organization** with filtered listing

**Portability and data management**

- **Cross-platform:** Windows 10+, macOS 12+, Linux (single static binary)
- **Export and import** encrypted backup files

**Audit logging**

- **Optional ScalarDL Ledger integration** for tamper-evident audit logging with hash-chain verification
- **Offline event queue** stores audit events when the ledger is unreachable

OTP algorithm standards in Tegata follow the RFCs and common provisioning behavior:

- **HOTP** uses **SHA-1 only** (RFC 4226).
- **TOTP** supports **SHA-1, SHA-256, and SHA-512** (RFC 6238).
- For manual TOTP entry, default to **SHA-1** unless your provider's `otpauth://` URI explicitly specifies a different algorithm.

## Security model

Tegata is a software authenticator with portable key storage, not a hardware security key replacement. Keys are encrypted at rest with AES-256-GCM and decrypted in host memory during use. Sensitive memory is zeroed immediately after use via memguard. Rate limiting with exponential backoff protects against brute-force passphrase attempts.

The optional ScalarDL Ledger integration provides tamper-evident audit logging, but does not protect against vault compromise if the ledger is breached. You should follow best practices for passphrase strength and vault backup.

## Installation

Tegata can be installed using pre-built binaries or built from source. Install the interface that fits your workflow, or install both.

**CLI and TUI** (runs from USB drive — no host installation required):

- **Windows:** [tegata-windows-amd64.exe](https://github.com/josh-wong/tegata/releases/latest/download/tegata-windows-amd64.exe)
- **macOS (Apple Silicon):** [tegata-darwin-arm64](https://github.com/josh-wong/tegata/releases/latest/download/tegata-darwin-arm64)
- **macOS (Intel):** [tegata-darwin-amd64](https://github.com/josh-wong/tegata/releases/latest/download/tegata-darwin-amd64)
- **Linux:** [tegata-linux-amd64](https://github.com/josh-wong/tegata/releases/latest/download/tegata-linux-amd64)

> [!IMPORTANT]
> 
> On macOS and Linux, mark the binary as executable with `chmod +x`.

**Desktop GUI** (installed on the host machine):

- **Windows:** [tegata-gui-windows-amd64-setup.exe](https://github.com/josh-wong/tegata/releases/latest/download/tegata-gui-windows-amd64-setup.exe)
- **macOS:** [tegata-gui-darwin-universal.dmg](https://github.com/josh-wong/tegata/releases/latest/download/tegata-gui-darwin-universal.dmg)
- **Linux (.deb):** [tegata-gui-linux-amd64.deb](https://github.com/josh-wong/tegata/releases/latest/download/tegata-gui-linux-amd64.deb)
- **Linux (.rpm):** [tegata-gui-linux-amd64.rpm](https://github.com/josh-wong/tegata/releases/latest/download/tegata-gui-linux-amd64.rpm)

All links above always resolve to the latest release. For a full list of release assets, see the [Releases](https://github.com/josh-wong/tegata/releases) page.

> [!NOTE]
> 
> If you are a maintainer creating release artifacts, see [Build and release artifacts](admin/build-and-release.md) for the full cross-platform build, packaging, signing, and publishing runbook.

For comprehensive guides and references, see the [Tegata documentation](https://tegata.080f53.com).

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, coding standards, commit conventions, and the pull request process.

## License

MIT. See [LICENSE](LICENSE) for the full license text.
