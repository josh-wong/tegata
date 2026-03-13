# Tegata – Product requirements document

**Author:** [josh-wong](https://github.com/josh-wong)
**Date:** March 8, 2026
**Last revised:** March 12, 2026
**Status:** Approved
**Companion document:** [Design document](./design-doc.md)

---

## 1. Overview

This document defines the product requirements, architecture, and release plan for Tegata, an open-source portable authenticator with optional tamper-evident audit logging.

### 1.1 What is Tegata?

Tegata is an open-source, portable software authenticator that stores encrypted authentication keys on standard USB drives or microSD cards. It supports TOTP, HOTP, challenge-response signing, and static password storage—providing a low-cost alternative to hardware security keys like YubiKey.

An optional tamper-evident audit layer powered by ScalarDL Ledger (Community Edition, Apache 2.0) records every authentication event in an immutable, hash-chained ledger, enabling users to verify that their authentication history hasn't been tampered with.

### 1.2 Project name origin

Tegata (手形) were handprint-stamped travel passes carried by samurai and merchants between checkpoint stations (関所, sekisho) in feudal Japan. They served as portable, verifiable proof of identity—a concept that maps directly to what this project does: carry your authentication keys on a USB stick, prove who you are, and leave a tamper-evident trail.

### 1.3 Tagline

> *"Your authentication history. Integrity checked."*

### 1.4 License

- **Tegata core:** Apache 2.0
- **ScalarDL Ledger integration:** Apache 2.0 (Community Edition only; ScalarDL Auditor requires an enterprise license and is explicitly out of scope)

## 2. Problem statement

This section identifies the problems Tegata addresses and explicitly defines what it does NOT solve.

### 2.1 The problem

Hardware security keys (YubiKey, SoloKey, etc.) provide excellent authentication security through dedicated secure elements, but they are expensive ($25–$70+ per unit), proprietary, and require purchasing separate backup keys. Many individual developers and power users want portable, multi-protocol authentication without the cost or vendor lock-in.

At the same time, authentication event logging is typically handled by server-side systems that administrators can silently modify or delete. There is no easy, open-source way for an individual to maintain a personal, tamper-evident record of their own authentication history.

### 2.2 What Tegata does NOT solve

Tegata is a **software authenticator with portable key storage**, not a hardware security key replacement. The critical distinction:

- **YubiKey/hardware keys:** Private keys live on a secure element. Cryptographic operations execute on-device. Keys never leave the hardware. This provides hardware-level isolation.
- **Tegata:** Private keys are stored encrypted on the USB drive/microSD card but are **decrypted in host memory** when used. This provides portability and auditability, but **not** hardware-level key isolation.

Users must understand this tradeoff. Tegata is appropriate for individuals who want portable, auditable authentication at low cost. It is not appropriate for high-security environments where hardware-level key isolation is required.

## 3. Target audience

Tegata is designed for individual developers and power users, with potential future expansion to small teams.

### 3.1 Primary: Individual developers and power users

- **Developers** who work across multiple machines (personal laptop, work desktop, servers)
- **Privacy-conscious individuals** who want control over their authentication keys
- **Open-source enthusiasts** who prefer auditable, non-proprietary tools
- **Users** who want a backup/secondary authenticator without buying additional hardware

### 3.2 Secondary (future): Small teams and startups

- **Teams** that want portable, accountable credential management without enterprise IAM costs
- **Organizations** in regulated environments (healthcare, finance) that need tamper-evident auth logs for compliance

### 3.3 Explicitly out of scope for v1

- **Enterprise deployments** requiring ScalarDL Auditor (Byzantine fault detection across two administrative domains)
- **Non-technical consumers** who need plug-and-play simplicity with zero setup

## 4. Goals and success metrics

This section defines the priority goals for Tegata and the metrics used to measure success post-launch.

### 4.1 Goals

| Priority | Goal                         | Description                                                                                                                                                  |
|----------|------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------|
| P0       | Functional authenticator     | Users can generate TOTP/HOTP codes, perform challenge-response signing, and retrieve static passwords from an encrypted vault on a USB drive or microSD card |
| P0       | Portable and zero-install    | The host application runs as a single portable binary from the USB drive itself—no installation required on the host machine                                 |
| P0       | Open source                  | The entire project is Apache 2.0 licensed and publicly auditable                                                                                             |
| P1       | Tamper-evident audit logging | Authentication events are optionally recorded to a ScalarDL Ledger instance with hash-chain integrity                                                        |
| P1       | Cross-platform               | Works on Windows, macOS, and Linux                                                                                                                           |
| P2       | User-friendly CLI and TUI    | A terminal UI that guides users through setup and daily use                                                                                                  |

### 4.2 Success metrics (6 months post-launch)

- 50+ GitHub stars
- 3+ unique contributors or forks
- 1+ third-party blog post or review
- Functional on all three major desktop operating systems
- Zero reported key leakage or vault compromise incidents

## 5. Architecture overview

Tegata uses a layered architecture that separates the core authenticator (offline-capable) from the optional audit logging layer (requires server infrastructure).

### 5.1 Layered design principle

Tegata is designed as two independent layers that can be used together or separately.

#### Layer 1 – Core Authenticator (standalone, offline-capable)

A portable binary + encrypted key vault on a USB drive or microSD card. This layer handles all authentication operations and works entirely offline. No server, no network, no JVM required.

#### Layer 2 – Audit Ledger (optional, requires server)

A ScalarDL Ledger integration that records authentication events to an immutable, hash-chained ledger. This layer requires a running ScalarDL Ledger instance (with a backing database) and network connectivity.

### 5.2 Component breakdown

The following subsections detail each major component of the Tegata system.

#### 5.2.1 Encrypted key vault (on USB/microSD)

- **Format:** AES-256-GCM encrypted JSON file
- **Protection:** User-provided PIN or passphrase, stretched via Argon2id
- **Contents:** TOTP/HOTP secrets, challenge-response private keys, static passwords, vault metadata (creation date, last modified, version)
- **Location:** Stored on the USB drive or microSD card alongside the portable binary

#### 5.2.2 CLI binary (portable)

- **Language:** Go, compiling to a single static binary per platform with CGO_ENABLED=0 — no runtime dependencies.
- **Functionality:**
  - Decrypts the key vault using the user's PIN/passphrase.
  - Generates TOTP/HOTP codes.
  - Performs challenge-response signing (HMAC-SHA1/SHA256).
  - Retrieves static passwords.
  - Optionally submits auth events to ScalarDL Ledger.
- **Distribution:** Pre-compiled static binaries for Windows (amd64), macOS (arm64, amd64), and Linux (amd64). Linux support is CI cross-compiled; manual testing only (no Linux CI runner). Binaries are stored on the USB drive itself.
- **Memory safety:** Keys are decrypted in memory, used, and immediately zeroed. The vault file on disk is never written in plaintext.

#### 5.2.3 Desktop GUI (host-installed)

- **Framework:** Wails desktop application (CGO required; system WebView used as frontend).
- **Feature parity:** Full feature parity with the CLI — all authentication operations, vault management, and optional audit logging available via a graphical interface.
- **Shared service layer:** The same internal Go packages handle vault, auth, and audit operations in both the CLI and GUI binaries.
- **Distribution:** Platform-specific installer; installed on the host machine, not carried on the USB drive.
- **Architecture details:** App struct design, frontend framework selection, and build flags are deferred to the design document.

#### 5.2.4 Event Builder

- **Purpose:** Constructs authentication event records from vault operations before sending to ScalarDL.
- **Inputs:** Operation type (TOTP/HOTP/CR/static), credential label, target service, success/failure status.
- **Outputs:** AuthEvent struct with hashed identifiers (labels and services are hashed before transmission).
- **Location:** Runs within the host application, invoked by authentication engines.

#### 5.2.5 ScalarDL Ledger integration (optional)

- **Version:** ScalarDL 3.12 Community Edition (Apache 2.0).
- **Deployment:** User-managed. Can run on a local machine, a Raspberry Pi, a VPS, or a cloud instance. Not bundled with the USB drive.
- **Backing database:** PostgreSQL (recommended for multi-device use), SQLite, or MySQL
- **Communication:** The host application communicates with ScalarDL Ledger via gRPC. Since the host app is Go/Rust, a lightweight gRPC client will be implemented (not the Java Client SDK).
- **What gets logged:** Timestamp, auth protocol used, target service identifier (hashed), success/failure, a hash of the challenge (never the key material itself).
- **Contracts:** Will primarily use the HashStore contracts (`object.Put`, `object.Validate`) in ScalarDL 3.12 to store and verify event hashes, avoiding the need to write and deploy custom Java contracts.
- **Validation:** Users can run `tegata verify` to traverse the hash chain and confirm no entries have been tampered with.

### 5.3 What Tegata does NOT include on the USB drive

- No JVM or Java runtime
- No database engine
- No ScalarDL Ledger server
- No network services or daemons
- No GUI binary (the desktop GUI is installed on the host machine, not carried on the USB drive)

The USB drive contains only: the portable binary (per-platform), the encrypted vault file, and a README.

## 6. Functional requirements

Tegata must support multiple authentication protocols, vault management operations, and optional audit logging capabilities.

### 6.1 Authentication protocols

Tegata must support the following authentication protocols for compatibility with common 2FA and authentication systems.

#### FR-1: TOTP (time-based one-time passwords)

- Implement RFC 6238.
- Support SHA-1, SHA-256, and SHA-512 hash algorithms.
- Support 6-digit and 8-digit codes.
- Support configurable time steps (default 30 seconds).
- Display time remaining for current code.

#### FR-2: HOTP (counter-based one-time passwords)

- Implement RFC 4226.
- Maintain counter state in the encrypted vault.
- Support resynchronization.

#### FR-3: Challenge-response signing

- Support HMAC-SHA1 and HMAC-SHA256 challenge-response.
- Accept challenges via CLI argument or stdin.
- Output signed response to stdout.
- Suitable for use as an SSH authentication helper or custom protocol integration.

#### FR-4: Static password storage

- Store and retrieve static passwords/secrets by label.
- Copy to clipboard with auto-clear after configurable timeout (default 45 seconds).
- Never display passwords in terminal output by default (require explicit `--show` flag).

### 6.2 Vault management

The vault must support initialization, credential management, and backup/restore operations.

#### FR-5: Vault initialization

- `tegata init` creates a new encrypted vault on the USB drive or microSD card.
- Prompts for PIN/passphrase with strength validation.
- Generates a recovery key that the user must store separately.
- Creates platform-specific portable binaries directory on the drive.

#### FR-6: Adding credentials

- `tegata add` supports manual entry of TOTP/HOTP secrets.
- `tegata add --scan` accepts otpauth:// URIs (for QR code content pasting).
- Each credential has: label, issuer, protocol type, secret, and optional metadata.

#### FR-7: Listing and organizing credentials

- `tegata list` shows all stored credentials (labels and issuers only—never secrets).
- Support for grouping/tagging credentials.

#### FR-8: Removing credentials

- `tegata remove <label>` deletes a credential from the vault.
- Requires PIN/passphrase re-entry for confirmation.

#### FR-9: Vault export/import

- `tegata export` exports vault contents in an encrypted format for backup.
- `tegata import` imports from a previously exported backup.
- Export format is Tegata-specific (not interoperable with other tools in v1).

### 6.3 Audit logging (optional – requires ScalarDL Ledger)

When ScalarDL integration is enabled, Tegata provides tamper-evident logging and verification capabilities.

#### FR-10: Event logging

- When ScalarDL integration is configured, every auth operation (code generation, challenge signing, password retrieval) is logged.
- **Log entries include** a timestamp, operation type, credential label hash, target service hash, success/failure, and host machine fingerprint hash.
- **Log entries never include** plaintext secrets, passwords, or unhashed identifiers.

#### FR-11: Audit verification

- `tegata verify` connects to the ScalarDL Ledger instance and validates the hash chain integrity of all logged events.
- Reports any detected tampering or missing entries.
- Uses the built-in `object.Validate` contract in ScalarDL.

#### FR-12: Audit history

- `tegata history` displays a human-readable history of authentication events from the ledger.
- Supports filtering by date range, credential label, and operation type.

#### FR-13: Offline queueing

- If the ScalarDL Ledger instance is unreachable, auth events are queued locally in an encrypted buffer on the USB drive.
- Queued events are automatically submitted when connectivity is restored.
- The queue itself is integrity-protected (local hash chain) to prevent tampering during the offline window.

## 7. Non-functional requirements

These requirements define security, performance, compatibility, and usability constraints that Tegata must meet.

### 7.1 Security

| ID    | Requirement                                                                               |
|-------|-------------------------------------------------------------------------------------------|
| NFR-1 | Key material is never written to disk in plaintext                                        |
| NFR-2 | Decrypted keys in memory are zeroed immediately after use                                 |
| NFR-3 | Vault encryption uses AES-256-GCM with Argon2id key derivation                            |
| NFR-4 | Failed PIN/passphrase attempts are rate-limited (exponential backoff)                     |
| NFR-5 | The vault auto-locks after a configurable idle timeout (default 5 minutes)                |
| NFR-6 | No telemetry, analytics, or network calls except to the user-configured ScalarDL instance |
| NFR-7 | All ScalarDL communication uses TLS                                                       |

### 7.2 Performance

| ID     | Requirement                                                                      |
|--------|----------------------------------------------------------------------------------|
| NFR-8  | TOTP code generation completes in <100ms after vault unlock                      |
| NFR-9  | Vault unlock (Argon2id derivation) completes in <3 seconds on commodity hardware |
| NFR-10 | Portable binary size is <20MB per platform                                       |

### 7.3 Compatibility

| ID     | Requirement                                                                    |
|--------|--------------------------------------------------------------------------------|
| NFR-11 | Host application runs on Windows 10+, macOS 12+, and Ubuntu 20.04+             |
| NFR-12 | USB drive must be formatted as FAT32 or exFAT for cross-platform compatibility |
| NFR-13 | No elevated permissions (admin/root) required for normal operation             |
| NFR-14 | ScalarDL integration is compatible with ScalarDL 3.12 Community Edition        |

### 7.4 Usability

| ID     | Requirement                                                                        |
|--------|------------------------------------------------------------------------------------|
| NFR-15 | First-time setup (init + add first credential) completes in under 5 minutes        |
| NFR-16 | Daily use flow (plug in → enter PIN → get code) completes in under 10 seconds      |
| NFR-17 | All CLI commands include `--help` with examples                                    |
| NFR-18 | Error messages are actionable (tell the user what to do, not just what went wrong) |

## 8. User flows

These flows illustrate how users interact with Tegata during setup, daily use, and audit verification.

### 8.1 First-time setup

```
1. User downloads Tegata release package (or clones repo and builds).
2. User copies platform binaries to USB drive or microSD card.
3. User runs: `tegata init`.
4. Tegata prompts for a PIN/passphrase.
5. Tegata generates an encrypted vault and a recovery key.
6. User stores recovery key in a safe location.
7. User runs: `tegata add --scan "otpauth://totp/GitHub:josh?secret=JBSWY3DPEHPK3PXP&issuer=GitHub"`
8. Credential is encrypted and stored in the vault.
9. Setup complete.
```

### 8.2 Daily use – generate a TOTP code

```
1. User plugs in USB drive.
2. User opens terminal, navigates to USB drive.
3. User runs: `tegata code GitHub`.
4. Tegata prompts for PIN.
5. Tegata decrypts vault, generates TOTP code, displays it with time remaining.
6. If ScalarDL is configured: auth event is logged to the ledger in the background.
7. Vault auto-locks after idle timeout
```

### 8.3 Audit verification

```
1. User runs: `tegata verify`
2. Tegata connects to the configured ScalarDL Ledger instance
3. Tegata traverses the hash chain of all logged events
4. Output: "✓ 1,247 events verified. Hash chain intact. No tampering detected."
   – or –
   Output: "✗ Integrity violation detected at event #843. The hash chain is broken
            between 2026-02-14T09:31:00Z and 2026-02-14T09:35:00Z. 
            Run 'tegata history --around 843' for details."
```

### 8.4 Plain-language mental model (for non-technical users)

For someone who doesn't know what TOTP or hash chains are:

> *Tegata is like a house key on a USB stick with a security camera that can't be erased.*
>
> You plug in your USB drive. You type your PIN. Tegata gives you a code to log in to your account. When you pull the drive out, nothing stays on the computer.
>
> Meanwhile, every time you use it, Tegata writes a line in a logbook that nobody can erase or change—not even you. If someone ever tries to mess with the logbook, Tegata will tell you.

## 9. ScalarDL integration details

This section explains how Tegata integrates with ScalarDL Ledger, including design rationale, limitations, and implementation details.

### 9.1 Why ScalarDL Ledger?

ScalarDL Ledger provides tamper-evident, append-only storage with hash-chain integrity—meaning if any entry is modified or deleted after the fact, the chain breaks and the tampering is detectable. This is exactly what an authentication audit log needs.

Without ScalarDL Auditor (enterprise-only), Tegata cannot perform Byzantine fault detection across two separately administered nodes. However, for a single-user or small-team tool, Ledger-only validation (hash-chain traversal and recomputation) provides meaningful integrity guarantees against:

- Accidental log corruption.
- After-the-fact tampering by a compromised server.
- Silent deletion of log entries.

### 9.2 What Ledger-only cannot guarantee

- If the Ledger server itself is fully compromised (both the database and the ScalarDL process), an attacker could theoretically reconstruct a valid hash chain with altered data. ScalarDL Auditor exists to solve this via an independent second node, but it requires an enterprise license.
- This limitation should be clearly documented so users can make informed decisions about their threat model.

### 9.3 Deployment model

ScalarDL Ledger is **not bundled** with Tegata. Users who want audit logging must deploy their own Ledger instance. Tegata will provide:

- A Docker Compose file for quick local setup (ScalarDL Ledger + PostgreSQL).
- Configuration documentation for connecting Tegata to the Ledger instance.
- A `tegata ledger setup` command that validates connectivity and registers the necessary HashStore contracts.

### 9.4 Contract strategy

Tegata will use **HashStore** (generic contracts) in ScalarDL 3.12 rather than custom contracts:

- `object.Put`. Store a hash of each authentication event.
- `object.Get`. Retrieve event records.
- `object.Validate`. Verify hash chain integrity.
- `collection.Create` / `collection.Add`. Group events by user, device, or time period.

This avoids requiring users to write, compile, and deploy Java contracts (which would need a JDK 8-compatible toolchain), keeping the setup lightweight.

### 9.5 gRPC client

Since the Tegata host app is written in Go (not Java), the ScalarDL Java Client SDK cannot be used directly. Tegata will implement a lightweight gRPC client based on the protobuf service definitions in ScalarDL. This client will support:

- Certificate-based authentication with the Ledger server.
- Contract execution (`Put`, `Get`, `Validate`).
- TLS encryption.

## 10. Release plan

Tegata will be developed across nine versions, progressing from planning through a full-featured system with audit capabilities and a desktop GUI.

### 10.1 v0.1 – Planning

1. Product requirements document
2. Design document
3. App design mockups
4. Repository setup with CI/CD pipelines

### 10.2 v0.2 – Core authenticator (MVP)

- Encrypted vault (AES-256-GCM + Argon2id)
- TOTP generation (RFC 6238)
- HOTP generation (RFC 4226)
- Static password storage and retrieval
- CLI interface with `init`, `add`, `list`, `code`, and `remove` commands
- Binary builds for Windows (amd64), macOS (arm64, amd64), and Linux (amd64)
- No ScalarDL integration

### 10.3 v0.3 – Challenge-response and enhancements

- Challenge-response signing (HMAC-SHA1/SHA256)
- Clipboard integration with auto-clear
- Vault export/import
- QR code scanning support (from terminal-pasted `otpauth://` URIs)

### 10.4 v0.4 – ScalarDL audit layer

- ScalarDL Ledger integration via gRPC
- `tegata ledger setup` command
- Event logging on every auth operation
- `tegata verify` for hash-chain validation
- `tegata history` for viewing auth event history
- Offline event queueing
- Docker Compose deployment template for ScalarDL Ledger

### 10.5 v0.5 – TUI

- Terminal user interface (bubbletea) for guided setup and daily use
- First-time setup flow, daily use flow, credential management

### 10.6 v0.6 – Desktop GUI

- Wails desktop application with full CLI feature parity
- CGO build with system WebView frontend
- Platform-specific installer (host-installed, not on USB drive)
- Shared service layer with CLI binary

### 10.7 v0.7 – Stable CLI API

- Stable CLI command set with backwards-compatibility guarantees documented

### 10.8 v0.8 – Security audit

- Independent security review of cryptographic implementation, vault format, and memory handling

### 10.9 v1.0 – Stable release

- Comprehensive documentation and user guides
- Contribution guidelines and community governance

## 11. Risks and mitigations

The following table identifies potential risks and their mitigation strategies.

| Risk                                                                                             | Severity | Mitigation                                                                                                                                                                  |
|--------------------------------------------------------------------------------------------------|----------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Keys decrypted in host memory can be extracted by malware or memory dumps                        | High     | Document this limitation clearly. Zero memory after use. Recommend full-disk encryption on host. Position Tegata as complementary to (not a replacement for) hardware keys. |
| Corporate/locked-down machines may block execution of portable binaries from USB drives          | Medium   | Provide instructions for running from a local copy. Consider a signed binary option for Windows.                                                                            |
| ScalarDL gRPC client implementation may diverge from Java SDK behavior                           | Medium   | Test against ScalarDL 3.12 integration tests. Document that ScalarDL integration is optional and can be disabled if necessary.                                              | 
| USB drive loss or theft exposes encrypted vault                                                  | Medium   | Vault is AES-256-GCM encrypted with Argon2id. Without the PIN/passphrase, the vault is computationally infeasible to crack. Document the importance of a strong passphrase. |
| ScalarDL Ledger-only validation is insufficient against a fully compromised server               | Medium   | Document this limitation explicitly. Recommend Auditor for high-security use cases (with the understanding that it requires an enterprise license).                         |
| Users may confuse Tegata with a hardware security key and overestimate its security properties   | Medium   | Prominent documentation and CLI warnings stating that Tegata is a software authenticator, not a hardware key.                                                               |
| FAT32/exFAT file system permissions do not support Unix-style file permission restrictions       | Low      | Vault security relies on encryption, not file permissions. Document that the encrypted vault file is safe to store on any file system.                                      |

## 12. Open questions

These questions require resolution before or during v1.0 development.

1. **Language choice: Go vs. Rust?** Go offers faster development and simpler cross-compilation. Rust offers better memory safety guarantees (important for crypto operations) and smaller binaries. Decision needed before v0.1.
2. **Should Tegata support FIDO2/WebAuthn in software?** This is technically possible (software-based FIDO2 authenticator) but controversial—the FIDO Alliance specifically designed FIDO2 to require hardware attestation. Including it could create false security expectations.
3. **Should the ScalarDL gRPC client be extracted as a standalone open-source Go/Rust library?** This could benefit the broader ScalarDL ecosystem but adds maintenance scope.
4. **Vault format: JSON vs. SQLite?** JSON is simpler and more portable. SQLite handles larger vaults more efficiently and supports atomic writes. Decision needed before v0.1.
5. **Should v1.0 include a GUI application in addition to CLI/TUI?** A GUI would improve accessibility for non-developer users but significantly increases development and maintenance scope.

## 13. References

- [ScalarDL GitHub Repository](https://github.com/scalar-labs/scalardl) (Apache 2.0 + Commercial dual license)
- [ScalarDL 3.12 Documentation](https://scalardl.scalar-labs.com/docs/latest/)
  - [Get Started with ScalarDL HashStore](https://scalardl.scalar-labs.com/docs/latest/getting-started-hashstore/)
  - [Write a ScalarDL Application with the HashStore Abstraction](https://scalardl.scalar-labs.com/docs/latest/how-to-write-applications-with-hashstore/)
- [RFC 6238 – TOTP](https://datatracker.ietf.org/doc/html/rfc6238)
- [RFC 4226 – HOTP](https://datatracker.ietf.org/doc/html/rfc4226)
- [Argon2 Reference](https://github.com/P-H-C/phc-winner-argon2)
- [YubiKey Developer Documentation](https://developers.yubico.com/) (for protocol compatibility reference)
