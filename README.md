# Tegata (手形)

**Your credentials, encrypted and portable.**

![CI](https://github.com/josh-wong/tegata/actions/workflows/ci.yml/badge.svg)
![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)
![Go 1.25+](https://img.shields.io/badge/Go-1.25%2B-00ADD8.svg)

Tegata is an open-source portable authenticator that stores encrypted credentials on standard USB drives or microSD cards. It combines TOTP/HOTP code generation, challenge-response signing, and static password storage with AES-256-GCM encryption and Argon2id key derivation. Optional tamper-evident audit logging via ScalarDL Ledger provides verifiable authentication history.

## Feature highlights

Tegata provides a low-cost alternative to hardware security keys with portable encrypted credential storage.

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

## Installation

Tegata can be installed using pre-built binaries or built from source.

### Pre-built binaries

Download the binary for your platform from the [Releases](https://github.com/josh-wong/tegata/releases) page. On macOS and Linux, mark the binary as executable with `chmod +x`.

### Build from source

```bash
git clone https://github.com/josh-wong/tegata.git
cd tegata
make build
```

The binary is placed in `bin/tegata`. Copy it to your USB drive alongside the vault.

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

Set `TEGATA_VAULT=/mnt/usb` to avoid repeating the `--vault` flag. For the full walkthrough, see the [getting started guide](docs/getting-started.md).

## Screenshots

<!-- Screenshots will be added before the v1.0 release. -->

## Security model

Tegata is a software authenticator with portable key storage, not a hardware security key replacement. Keys are encrypted at rest with AES-256-GCM and decrypted in host memory during use. Sensitive memory is zeroed immediately after use via memguard. Rate limiting with exponential backoff protects against brute-force passphrase attempts.

For a detailed review of the cryptographic implementation, memory handling, vault format, and input validation, see the [security audit](docs/SECURITY-AUDIT.md).

## Documentation

| Document                                               | Description                                       |
|--------------------------------------------------------|---------------------------------------------------|
| [Getting started](docs/getting-started.md)             | Installation, quickstart, and daily use            |
| [CLI command reference](docs/cli-reference.md)         | Complete documentation of all CLI commands          |
| [ScalarDL setup](docs/scalardl-setup.md)               | Configure optional tamper-evident audit logging    |
| [Security audit](docs/SECURITY-AUDIT.md)               | Self-audit of cryptographic and security practices |
| [Design document](docs/v1-design-doc.md)               | Technical architecture and component specifications |
| [Product requirements](docs/v1-product-requirements-doc.md) | Requirements, use cases, and release plan     |
| [Contributing](CONTRIBUTING.md)                        | Development setup, coding standards, and PR process |

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, coding standards, commit conventions, and the pull request process.

## License

MIT. See [LICENSE](LICENSE) for the full license text.
