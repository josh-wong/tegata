# Tegata – Design document

- **Author:** [josh-wong](https://github.com/josh-wong)
- **Date:** March 12, 2026
- **Last revised:** March 17, 2026
- **Status:** Draft (Phase 2 complete)
- **Companion document:** [Product requirements document](./v1-product-requirements-doc.md)

---

## 1. Scope and context

This document describes the technical architecture, component design, and implementation strategy for Tegata, an open-source portable authenticator with optional tamper-evident audit logging. It serves as the companion to the [product requirements document](./v1-product-requirements-doc.md) and provides the engineering detail needed to implement the system.

### 1.1 Purpose

The design document translates the PRD's functional and non-functional requirements into concrete technical decisions: which language, which libraries, how data is structured on disk, how components communicate, and how the system is built, tested, and deployed. Developers should read the PRD for *what* and *why*; this document covers *how*.

### 1.2 Terminology

The following terms are used throughout this document.

| Term              | Definition                                                                                           |
|-------------------|------------------------------------------------------------------------------------------------------|
| **Vault**         | The AES-256-GCM encrypted file on the USB drive that stores all credentials and metadata             |
| **Credential**    | A single authentication entry (TOTP secret, HOTP secret, challenge-response key, or static password) |
| **Label**         | A user-assigned name for a credential (for example, `GitHub` or `AWS-prod`)                          |
| **Passphrase**    | The user-provided secret used with Argon2id to derive the vault encryption key                       |
| **Recovery key**  | A randomly generated key stored offline that can decrypt the vault if the passphrase is lost          |
| **DEK**           | Data encryption key – the AES-256 key derived from the passphrase via Argon2id                       |
| **Event**         | A single authentication operation record submitted to ScalarDL Ledger                                |
| **HashStore**     | ScalarDL's built-in generic contract abstraction for storing and validating hash-chained records      |
| **Offline queue** | An encrypted local buffer that stores audit events when the ScalarDL Ledger instance is unreachable   |

### 1.3 Key decisions

The following table summarizes the major architectural decisions resolved during v0.1 planning.

| Decision           | Choice                     | Rationale                                                                                                                                                                                                                        |
|--------------------|----------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **Language**        | Go                         | Faster development velocity, simpler cross-compilation (`GOOS`/`GOARCH` flags), mature ecosystem for CLI tools and gRPC. Rust's stronger memory safety is valuable but adds development time that delays the MVP.                |
| **Vault format**    | JSON (whole-blob AES-256-GCM) | Simpler than SQLite for the expected vault size (tens to low hundreds of credentials). The entire JSON blob is encrypted as a single unit, avoiding the complexity of per-field or per-row encryption. Atomic writes via temp-file rename. |
| **Vault encryption** | AES-256-GCM               | NIST-approved authenticated encryption. Provides both confidentiality and integrity in a single operation. Well-supported by Go's `crypto/aes` and `crypto/cipher` standard library.                                             |
| **Key derivation**  | Argon2id                   | Winner of the Password Hashing Competition. Resists both GPU and ASIC attacks via memory-hard computation. The `id` variant combines Argon2i (side-channel resistance) and Argon2d (GPU resistance).                             |
| **Memory safety**   | memguard                   | Go library that allocates sensitive data in guarded memory pages (mlock, mprotect, canary checks). Provides `Enclave` and `LockedBuffer` types for key material lifecycle management.                                            |
| **gRPC client**     | grpc-go                    | Official Go gRPC implementation. Tegata implements a lightweight client against ScalarDL's protobuf service definitions rather than wrapping the Java Client SDK.                                                                |

## 2. Architecture overview

Tegata follows a layered architecture that separates the core authenticator (offline-capable, standalone) from the optional audit layer (requires a ScalarDL Ledger server). This section maps the PRD's architectural vision (PRD section 5) to concrete Go packages and data flows.

### 2.1 Package layout

The codebase is organized by responsibility. Each top-level package corresponds to a distinct component in the architecture.

```
tegata/
├── cmd/
│   ├── tegata/          # CLI entrypoint (cobra commands)
│   │   └── main.go
│   └── tegata-gui/      # GUI entrypoint (Wails app)
│       ├── main.go
│       ├── app.go       # App struct (thin adapter over internal/)
│       └── frontend/    # React + TypeScript (Wails managed)
│           ├── src/
│           ├── wailsjs/  # Auto-generated Go bindings
│           └── package.json
├── internal/
│   ├── vault/           # Vault Manager – encrypt, decrypt, read, write
│   ├── auth/            # Authentication engines – TOTP, HOTP, CR, static
│   ├── clipboard/       # Clipboard Manager – copy, auto-clear
│   ├── config/          # Config Manager – tegata.toml parsing, defaults
│   ├── audit/           # Event Builder + gRPC client + offline queue
│   └── crypto/          # Shared crypto primitives – Argon2id, AES-GCM, memguard wrappers
│       └── guard/       # memguard wrapper – SecretBuffer, KeyEnclave
├── pkg/
│   └── model/           # Shared types – Credential, AuthEvent, VaultHeader
├── scripts/             # Build, release, and CI helper scripts
├── deployments/
│   └── docker-compose/  # ScalarDL Ledger + PostgreSQL local setup
├── docs/                # PRD, design doc, and other documentation
├── go.mod
└── go.sum
```

Packages under `internal/` are not importable by external code. The `pkg/model/` package contains shared data types used across internal packages.

### 2.2 Component diagram

The following diagram shows the runtime relationships between components.

```mermaid
graph TB
    subgraph CLI["cmd/tegata"]
        Cobra["Cobra CLI Router"]
    end

    subgraph Core["internal/ (core layer)"]
        VM["vault.Manager"]
        TOTP["auth.TOTP"]
        HOTP["auth.HOTP"]
        CR["auth.ChallengeResponse"]
        Static["auth.StaticPassword"]
        Clip["clipboard.Manager"]
        Cfg["config.Manager"]
        Crypto["crypto (Argon2id, AES-GCM, memguard)"]
    end

    subgraph AuditLayer["internal/audit (optional layer)"]
        EB["EventBuilder"]
        GRPC["GRPCClient"]
        OQ["OfflineQueue"]
    end

    subgraph Disk["USB Drive / microSD"]
        VaultFile["vault.tegata (encrypted)"]
        QueueFile["queue.tegata (encrypted)"]
        ConfigFile["tegata.toml"]
    end

    subgraph Remote["ScalarDL Ledger (optional)"]
        Ledger["ScalarDL 3.12 + PostgreSQL"]
    end

    Cobra --> VM
    Cobra --> TOTP
    Cobra --> HOTP
    Cobra --> CR
    Cobra --> Static
    Cobra --> Clip
    Cobra --> Cfg
    Cobra --> EB

    VM --> Crypto
    VM <--> VaultFile
    TOTP --> VM
    HOTP --> VM
    CR --> VM
    Static --> VM

    TOTP --> EB
    HOTP --> EB
    CR --> EB
    Static --> EB

    EB --> GRPC
    EB --> OQ
    GRPC --> Ledger
    OQ <--> QueueFile
    OQ -.->|"flush when online"| GRPC

    Cfg <--> ConfigFile

    style CLI fill:#e1f5ff
    style Core fill:#fff4e1
    style AuditLayer fill:#f0e1ff
    style Disk fill:#e1ffe1
    style Remote fill:#ffe1e1
```

### 2.3 Data flow: TOTP code generation

This sequence shows the complete flow when a user runs `tegata code GitHub`, including the optional audit path.

```mermaid
sequenceDiagram
    participant User
    participant CLI
    participant VaultManager
    participant TOTP as auth.TOTP
    participant EventBuilder
    participant GRPCClient

    User->>CLI: tegata code GitHub
    CLI->>VaultManager: Unlock(passphrase)
    VaultManager->>VaultManager: Argon2id(passphrase, salt) → DEK
    VaultManager->>VaultManager: AES-GCM decrypt(DEK, nonce, blob)
    VaultManager-->>CLI: TOTP secret
    CLI->>TOTP: Generate(secret)
    TOTP-->>CLI: code + TTL
    CLI-->>User: display code
    CLI->>EventBuilder: LogEvent(totp, GitHub)
    EventBuilder->>GRPCClient: ExecuteContract(object.Put, event_hash)
    GRPCClient-->>EventBuilder: OK
    VaultManager->>VaultManager: zero DEK (guard.Destroy)
```

If the gRPC call fails, `EventBuilder` routes the event to `OfflineQueue` instead. The queue flushes automatically on the next successful connection.

### 2.4 Data flow: audit verification

When a user runs `tegata verify`, the following flow executes.

```mermaid
sequenceDiagram
    participant User
    participant CLI
    participant GRPCClient
    participant Ledger as ScalarDL Ledger

    User->>CLI: tegata verify
    CLI->>GRPCClient: ValidateChain()
    GRPCClient->>Ledger: ExecuteContract(object.Validate, asset_range)
    Ledger-->>GRPCClient: chain status
    GRPCClient-->>CLI: validation result
    CLI-->>User: "N events verified, chain intact"
```

### 2.5 Dependency list

The following Go modules are expected dependencies for the initial implementation.

| Module                                    | Purpose                                      | License    |
|-------------------------------------------|----------------------------------------------|------------|
| `github.com/spf13/cobra`                  | CLI command routing and flag parsing         | Apache 2.0 |
| `github.com/awnumar/memguard`              | Guarded memory for sensitive data (accessed only via `internal/crypto/guard`) | Apache 2.0 |
| `golang.org/x/crypto/argon2`              | Argon2id key derivation                      | BSD-3      |
| `google.golang.org/grpc`                  | gRPC client for ScalarDL communication       | Apache 2.0 |
| `google.golang.org/protobuf`              | Protobuf serialization for gRPC messages     | BSD-3      |
| `github.com/BurntSushi/toml`             | TOML configuration file parsing              | MIT        |
| `github.com/atotto/clipboard`             | Cross-platform clipboard access              | BSD-3      |
| `crypto/aes`, `crypto/cipher` (stdlib)    | AES-256-GCM encryption/decryption           | Go license |
| `crypto/hmac`, `crypto/sha1/sha256/sha512` (stdlib) | HMAC and hash operations for TOTP/HOTP/CR | Go license |

## 3. Vault format and storage

The vault is the central data structure in Tegata. It stores all credentials encrypted on the USB drive or microSD card. This section covers the on-disk format, the inner JSON schema, encryption parameters, and the write strategy.

### 3.1 File structure

The vault file (`vault.tegata`) consists of a plaintext header followed by an encrypted blob.

```
┌─────────────────────────────────────────┐
│  Plaintext header (fixed-size, 128 B)   │
│  ┌─────────────────────────────────────┐│
│  │ Magic bytes: "TEGATA\x00\x01"  (8B)││
│  │ Version: uint16              (2B)   ││
│  │ Argon2id time cost: uint32   (4B)   ││
│  │ Argon2id memory cost: uint32 (4B)   ││
│  │ Argon2id parallelism: uint8  (1B)   ││
│  │ Salt: [32]byte               (32B)  ││
│  │ Recovery key salt: [32]byte  (32B)  ││
│  │ Write counter: uint64        (8B)   ││
│  │ Nonce: [12]byte              (12B)  ││
│  │ Reserved: [25]byte           (25B)  ││
│  └─────────────────────────────────────┘│
├─────────────────────────────────────────┤
│  Encrypted blob (variable size)         │
│  AES-256-GCM(DEK, nonce, JSON payload) │
│  Includes 16-byte GCM auth tag         │
└─────────────────────────────────────────┘
```

The plaintext header stores only the parameters needed to derive the decryption key. It reveals no information about the vault contents (number of credentials, labels, or types). The write counter is a monotonic uint64 incremented on each vault write; the 12-byte nonce is derived deterministically from it as `counter_be8 || zeros4`. Storing both the counter and derived nonce in the header allows direct nonce validation during decryption without recomputation. The 25 reserved bytes allow future header extensions without breaking compatibility.

### 3.2 Inner JSON schema

After decryption, the blob contains a JSON document with the following structure.

```json
{
  "version": 1,
  "created_at": "2026-03-12T10:00:00Z",
  "modified_at": "2026-03-12T14:30:00Z",
  "credentials": [
    {
      "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "label": "GitHub",
      "issuer": "GitHub",
      "type": "totp",
      "algorithm": "SHA1",
      "digits": 6,
      "period": 30,
      "secret": "base32-encoded-secret",
      "tags": ["dev", "work"],
      "created_at": "2026-03-12T10:05:00Z",
      "modified_at": "2026-03-12T10:05:00Z"
    },
    {
      "id": "b2c3d4e5-f6a7-8901-bcde-f12345678901",
      "label": "AWS-prod",
      "issuer": "Amazon",
      "type": "hotp",
      "algorithm": "SHA1",
      "digits": 6,
      "counter": 42,
      "secret": "base32-encoded-secret",
      "tags": ["cloud"],
      "created_at": "2026-03-12T10:10:00Z",
      "modified_at": "2026-03-12T11:00:00Z"
    },
    {
      "id": "c3d4e5f6-a7b8-9012-cdef-123456789012",
      "label": "SSH-signing",
      "type": "challenge-response",
      "algorithm": "SHA256",
      "secret": "hex-encoded-key",
      "tags": [],
      "created_at": "2026-03-12T10:15:00Z",
      "modified_at": "2026-03-12T10:15:00Z"
    },
    {
      "id": "d4e5f6a7-b8c9-0123-defa-234567890123",
      "label": "WiFi-office",
      "type": "static",
      "secret": "plaintext-password-value",
      "tags": ["network"],
      "created_at": "2026-03-12T10:20:00Z",
      "modified_at": "2026-03-12T10:20:00Z"
    }
  ],
  "recovery_key_hash": "hex-encoded-sha256-of-recovery-key"
}
```

Fields vary by credential type. TOTP credentials include `algorithm`, `digits`, and `period`. HOTP credentials include `algorithm`, `digits`, and `counter`. Challenge-response credentials include `algorithm`. Static passwords have only `secret`. All types share `id`, `label`, `type`, `secret`, `tags`, `created_at`, and `modified_at`.

The `type` field uses the full string in the vault JSON schema (`"totp"`, `"hotp"`, `"challenge-response"`, `"static"`). CLI human-readable output and JSON output use the abbreviated form `"cr"` for display. The mapping is: `"challenge-response"` (storage) ↔ `"cr"` (CLI output).

### 3.3 Argon2id parameters

The following parameters are used for key derivation from the user's passphrase.

| Parameter         | Default  | OWASP minimum | Rationale                                                                                                             |
|-------------------|----------|---------------|-----------------------------------------------------------------------------------------------------------------------|
| Time cost         | 3        | 2             | Three iterations balance security and NFR-9 (<3s) on commodity hardware                                              |
| Memory cost       | 64 MiB   | 19 MiB        | Well above minimum; 64 MiB feasible on all target platforms                                                          |
| Parallelism       | 4        | 1             | Matches common core counts; lower values appropriate for single-core hosts                                            |
| Salt length       | 32 bytes | —             | Exceeds recommended 16-byte minimum                                                                                   |
| Output key length | 32 bytes | —             | Produces 256-bit key for AES-256                                                                                      |

These parameters are stored in the vault header so that future versions can adjust them without breaking existing vaults. The `tegata bench` command benchmarks Argon2id on the current machine and recommends parameter adjustments if derivation exceeds 3 seconds. When reducing parameters, lower memory cost first (since memory cost is more resistant to GPU attacks than time cost). Never allow time cost below 2 or memory cost below 19 MiB (OWASP minimum) without an explicit override flag.

### 3.4 Encrypt/decrypt flow

**Encryption (on vault write):**

1. Serialize credentials to JSON.
2. Read the salt from the vault header (or generate a new one for `init`).
3. Derive the DEK from the passphrase by using Argon2id with the stored parameters and salt.
4. Increment the write counter in the header.
5. Derive the 12-byte nonce from the write counter: `nonce = counter_be8 || zeros4`.
6. Encrypt the JSON blob by using AES-256-GCM with the DEK and derived nonce.

> [!NOTE]
> 
> **Implementation note:** The `crypto.Seal()` and `crypto.Open()` functions encapsulate nonce derivation and the AES-256-GCM operation. Callers pass the key, write counter, plaintext/ciphertext, and the vault header as AAD. Nonce derivation from the counter is handled internally—callers never construct or pass nonces directly.

7. Write the header (with updated write counter and derived nonce) and encrypted blob to a temporary file.
8. Rename the temporary file over the vault file (atomic on all target file systems).
9. Zero the DEK and plaintext JSON from memory by using memguard.

**Decryption (on vault unlock):**

1. Read the plaintext header to extract salt, Argon2id parameters, write counter, and nonce.
2. Verify the magic bytes and version field—reject with a corruption error (exit 3) if invalid.
3. Verify the nonce matches `deriveNonce(write_counter)`—reject with a corruption error (exit 3) if mismatched.
4. Derive the DEK from the passphrase by using the extracted parameters.
5. Decrypt the blob by using AES-256-GCM with the DEK and nonce.
6. If decryption fails (GCM tag mismatch) and the header passed validation in steps 2–3, the passphrase is incorrect—return an authentication error (exit 2). If the header was borderline (valid magic but unexpected field values), return a corruption error (exit 3) instead.
7. Deserialize the JSON into in-memory credential structs (within memguard `LockedBuffer`).
8. Zero the DEK. The plaintext credentials remain in guarded memory until the vault locks.

The nonce derivation function ensures the counter and nonce are always consistent.

```go
func deriveNonce(counter uint64) [12]byte {
    var nonce [12]byte
    binary.BigEndian.PutUint64(nonce[:8], counter)
    // nonce[8:12] are already zero
    return nonce
}
```

The write counter starts at 1 on `tegata init` (counter 0 is reserved as an invalid state). The counter is stored as both the uint64 value and the derived 12-byte nonce in the header. Storing both allows direct nonce validation during decryption without recomputing. If the counter reaches 2^63, the vault format mandates key rotation via `tegata init` with a new salt—this bound is astronomically unreachable in practice but specified for format completeness.

### 3.5 Write strategy: temp-file rename with backup

Vault writes follow this sequence to prevent data loss.

1. Write the new vault contents to `vault.tegata.tmp` in the same directory.
2. If `vault.tegata.bak` exists, delete it.
3. Rename `vault.tegata` to `vault.tegata.bak`.
4. Rename `vault.tegata.tmp` to `vault.tegata`.

If the process crashes between steps 3 and 4, the backup file preserves the previous state. On startup, `vault.Manager` checks for orphaned `.tmp` files and warns the user. FAT32 and exFAT support atomic rename within the same directory, making this strategy safe on USB drives.

### 3.6 Recovery key

During `tegata init`, Tegata generates a 256-bit random recovery key encoded as a base32 string (52 characters, grouped for readability). The recovery key is used via a key-wrapping mechanism: during `tegata init`, the DEK is encrypted twice—once with the passphrase-derived key (stored as the main encrypted blob) and once with the recovery key (stored as a separate encrypted DEK blob appended after the main encrypted blob). On recovery, Tegata decrypts the wrapped DEK by using the recovery key, then uses that DEK to decrypt the vault blob. A SHA-256 hash of the recovery key is stored inside the encrypted vault to validate it without storing the key itself on disk.

Users must store the recovery key offline (printed, in a password manager, etc.). Tegata displays it once during init and never again.

**Recovery key verification (`tegata verify-recovery`):** Users can confirm a stored recovery key string is still intact without performing a full emergency unlock. The flow is:

1. Unlock the vault with the regular passphrase.
2. Prompt the user to paste or type their recovery key string.
3. Decode the base32 string, compute `SHA-256(recoveryRaw)`, and compare the result against the `recovery_key_hash` stored in the decrypted vault payload.
4. Report whether the key matches.

The hash comparison approach is preferred over attempting a live `UnlockWithRecoveryKey` call because it avoids a full Argon2id derivation and GCM decryption cycle. It also avoids incrementing the rate-limit counter on a verification that is expected to succeed. Users are encouraged to run `tegata verify-recovery` periodically—for example, when rotating their password manager or testing a printed backup.

### 3.7 Passphrase rotation

`tegata change-passphrase` rewraps the DEK under a new passphrase without re-encrypting the payload. The existing recovery-wrapped DEK is preserved unchanged, so the recovery key continues to work after rotation.

**Flow:**

1. Unlock the vault with the current passphrase (full `Unlock` path, including rate-limit check).
2. Prompt for the new passphrase and confirmation; enforce the same 8-character minimum and display the strength meter.
3. Generate a new 32-byte Argon2id salt.
4. Derive a new passphrase key: `newKey = DeriveKey(newPassphrase, newSalt, params)`.
5. Rewrap the DEK: `newPassphraseWrappedDEK = Seal(newKey, counter=1, dekRaw, nil)`.
6. Update the header salt field with `newSalt`.
7. Write the vault atomically (temp-file rename) with the updated header, the existing encrypted payload and write counter unchanged, the new passphrase-wrapped DEK, and the unchanged recovery-wrapped DEK.

Because the DEK and payload are unchanged, the write counter does not need to be incremented. The only fields that change on disk are the header salt and the passphrase-wrapped DEK blob. The recovery key salt in the header and the recovery-wrapped DEK are left as-is.

## 4. Authentication engines

Each authentication protocol is implemented as a separate engine in the `internal/auth/` package. All engines share a common interface for credential access via `vault.Manager` and event emission via `audit.EventBuilder`.

### 4.1 TOTP (RFC 6238)

The TOTP engine generates time-based one-time passwords as specified in RFC 6238.

**Implementation details:**

- Compute `T = floor((current_unix_time - T0) / period)` where `T0 = 0` and `period` defaults to 30 seconds.
- Compute `HMAC(algorithm, secret, T)` where `algorithm` is SHA-1, SHA-256, or SHA-512.
- Apply dynamic truncation per RFC 4226 section 5.4 to produce a 6-digit or 8-digit code.
- Return the code and the number of seconds remaining in the current period.

**Supported parameters:**

| Parameter   | Values              | Default |
|-------------|---------------------|---------|
| Algorithm   | SHA-1, SHA-256, SHA-512 | SHA-1   |
| Digits      | 6, 8                | 6       |
| Period      | 15–120 seconds      | 30      |

**Validation:** Unit tests use the test vectors from RFC 6238 appendix B to verify correctness across all three hash algorithms.

### 4.2 HOTP (RFC 4226)

The HOTP engine generates counter-based one-time passwords as specified in RFC 4226.

**Implementation details:**

- Compute `HMAC-SHA1(secret, counter)` where `counter` is an 8-byte big-endian integer.
- Apply dynamic truncation to produce a 6-digit or 8-digit code.
- Persist the counter to disk before returning the code (see counter persistence order below).

**Counter persistence order:** The counter update and code generation follow this exact sequence to prevent counter/code inconsistency on crash:

1. Increment the counter in-memory.
2. Write the vault with the new counter to a temporary file.
3. Atomic rename the temporary file over the vault file.
4. Generate and return the code to the caller.

If step 2 or 3 fails, the code is never returned—the user retries with the same counter value. This ordering ensures that a displayed code always corresponds to a persisted counter. See section 9 (Security model), cryptographic pitfall #4 for the rationale behind this ordering, and section 3.5 for the atomic write strategy.

The counter is stored in the credential's `counter` field inside the encrypted vault. Because the entire vault is re-encrypted on every counter update, the write-temp-rename strategy (section 3.5) ensures atomicity.

**Resynchronization:** If a user's counter drifts from the server, `tegata resync <label>` prompts the user to enter two consecutive codes currently displayed by the server. Tegata searches the look-ahead window (default 100) for a counter position where both codes match consecutively, then updates the stored counter accordingly. Requiring two consecutive codes eliminates false matches from coincidental collisions within the search window.

### 4.3 Challenge-response (HMAC)

The challenge-response engine signs arbitrary challenges by using HMAC.

**Implementation details:**

- Accept a challenge via the `--challenge` flag (`tegata sign <label> --challenge <value>`), interactive prompt, or stdin.
- Compute `HMAC(algorithm, secret, challenge)` where `algorithm` is SHA-1 or SHA-256.
- Output the hex-encoded signature to stdout.

**Use cases:** SSH authentication helpers, custom protocol integrations, and any system that accepts HMAC-based challenge-response authentication.

### 4.4 Static passwords and clipboard

The static password engine retrieves stored passwords and manages clipboard interaction.

**Retrieval flow:**

1. User runs `tegata get <label>`.
2. The password is copied to the system clipboard.
3. A background goroutine clears the clipboard after the configured timeout (default 45 seconds).
4. The password is never printed to stdout unless the user passes the `--show` flag.

**Clipboard manager:** The `internal/clipboard/` package wraps `github.com/atotto/clipboard` and adds auto-clear functionality. It spawns a goroutine that sleeps for the timeout duration, then overwrites the clipboard with an empty string. If the user copies something else before the timeout expires, the auto-clear cancels (to avoid erasing unrelated content).

## 5. CLI design

The CLI is built with Cobra and follows conventional Unix command-line patterns. All commands support `--help` with usage examples. Each CLI invocation is stateless: it unlocks the vault, performs the requested operation, and immediately zeros all key material. There is no persistent "unlocked session" in CLI mode—only the TUI (v0.5) and GUI (v0.6) maintain session state with idle timeout-based auto-lock.

### 5.1 Command tree

```
tegata
├── init                    # Create a new vault
├── add [--totp|--hotp|--cr|--static] <label> [secret]
│                           # Add a credential
├── list                    # List all credentials
├── code <label>            # Generate TOTP/HOTP code
├── sign <label> [--challenge <value>]
│                           # Challenge-response signing
├── get <label>             # Retrieve static password (clipboard)
├── remove <label>          # Remove a credential
├── export <file>           # Export vault backup
├── import <file>           # Import vault backup
├── resync <label>          # Resynchronize HOTP counter
├── bench                   # Benchmark Argon2id on this machine
├── history                 # View audit event history
├── verify                  # Verify audit chain integrity
├── ledger
│   └── setup               # Configure and validate ScalarDL connection
├── config
│   └── show                # Display current configuration
└── version                 # Print version and build info
```

### 5.2 Global flags

The following flags are available on all commands.

| Flag                 | Short | Description                                   | Default        |
|----------------------|-------|-----------------------------------------------|----------------|
| `--vault <path>`     | `-v`  | Path to the vault file                        | Auto-detect    |
| `--config <path>`    | `-c`  | Path to the config file                       | Auto-detect    |
| `--no-audit`         |       | Suppress audit logging for this command       | `false`        |
| `--json`             |       | Output in JSON format (for scripting)         | `false`        |
| `--quiet`            | `-q`  | Suppress non-essential output                 | `false`        |

### 5.3 Vault auto-detection

When `--vault` is not specified, Tegata searches for a vault file in the following order:

1. The current working directory (`./vault.tegata`).
2. The directory containing the `tegata` binary (supports running directly from USB).
3. The path specified in `tegata.toml` (if a config file is found).

If no vault is found, Tegata prints a message suggesting `tegata init`.

### 5.4 Output conventions

**Human-readable output (default):** Designed for terminal use with color support (respects `NO_COLOR` environment variable). TOTP codes include a countdown indicator. Lists use aligned columns.

**JSON output (`--json` flag):** Machine-parseable output for scripting and integration. Every command produces a JSON object with at minimum `{ "status": "ok"|"error" }`.

### 5.5 Exit codes

All commands return one of the following exit codes.

| Code | Meaning                                    |
|------|--------------------------------------------|
| 0    | Success                                    |
| 1    | General error (invalid input, missing file)|
| 2    | Authentication error (wrong passphrase)    |
| 3    | Vault error (corrupted, missing, locked)   |
| 4    | Network error (ScalarDL unreachable)       |
| 5    | Integrity error (audit chain broken)       |

### 5.6 Usability targets

The CLI workflow is designed to meet two usability benchmarks from the PRD.

- **First-time setup under 5 minutes (NFR-15):** The `tegata init` command handles vault creation in a single interactive flow (passphrase entry, recovery key display). Adding a first credential via `tegata add --scan` with an `otpauth://` URI is a single command. The entire init-add-verify cycle requires three commands.
- **Daily use under 10 seconds (NFR-16):** The daily flow is `tegata code <label>`, passphrase entry, and code display. Vault auto-detection (section 5.3) eliminates the need to specify paths. Argon2id derivation targets under 3 seconds (section 3.3), and TOTP generation completes in under 100 ms after unlock.

### 5.7 Bench command

**`tegata bench`** benchmarks Argon2id key derivation on the current machine by using the default parameters (t=3, m=64MiB, p=4). It reports the derivation time and, if it exceeds 3 seconds, recommends adjusted parameters. The recommendation prioritizes reducing memory cost before time cost, since memory cost provides stronger GPU attack resistance.

Example output when within target:

```
Benchmarking Argon2id (t=3, m=64MiB, p=4)...
Derivation time: 2.1s ✓ (within 3s target)
```

Example output when over target:

```
Benchmarking Argon2id (t=3, m=64MiB, p=4)...
Derivation time: 4.8s ✗ (exceeds 3s target)
Recommended: t=3, m=32MiB, p=4 (estimated 2.4s)

Run 'tegata init --time=3 --memory=32 --parallel=4' to use adjusted parameters.
```

The bench command does not modify the vault. It is informational only.

## 6. Configuration

Tegata uses a TOML configuration file (`tegata.toml`) stored on the USB drive alongside the vault.

### 6.1 Configuration file

The configuration file uses TOML format with the following sections.

```toml
# tegata.toml – Tegata configuration

[vault]
# Idle timeout before auto-lock (seconds)
idle_timeout = 300

# Clipboard auto-clear timeout (seconds)
clipboard_timeout = 45

[audit]
# Enable ScalarDL audit logging
enabled = false

# ScalarDL Ledger server address
server = "localhost:50051"

# TLS certificate paths (relative to config file directory)
cert = "certs/client.pem"
key = "certs/client-key.pem"
ca = "certs/ca.pem"

# Offline queue settings
queue_max_events = 10000
```

### 6.2 Precedence rules

Configuration values are resolved in the following order (highest priority first):

1. **CLI flags.** Override everything for the current command.
2. **Environment variables.** Prefixed with `TEGATA_` (for example, `TEGATA_VAULT_IDLE_TIMEOUT=600`).
3. **Config file.** `tegata.toml` located via auto-detection (same search order as vault).
4. **Built-in defaults.** The values shown in the example above.

### 6.3 ScalarDL connection settings

The `[audit]` section configures the optional ScalarDL Ledger connection. When `enabled = false` (the default), Tegata operates as a standalone authenticator with no network activity.

When enabled, the gRPC client requires:

- A reachable server address (`server`).
- Valid TLS certificates (`cert`, `key`, `ca`).
- The `tegata ledger setup` command validates connectivity, negotiates TLS, and confirms that the required HashStore contracts are available on the Ledger instance.

## 7. Wails GUI architecture

Tegata provides an optional desktop GUI application built with Wails v2, planned for v0.6. The GUI binary is a separate executable installed on the host machine (not carried on the USB drive like the CLI binary). It shares the same internal service packages as the CLI, ensuring feature parity without code duplication. This section specifies the architecture in enough detail that v0.6 implementation can begin without further design work.

The `frontend/` directory within `cmd/tegata-gui/` is managed by Wails and uses npm for dependency management. It is only relevant to the GUI binary—the CLI build ignores it entirely.

### 7.1 App struct as thin adapter

The Wails v2 application entry point is an `App` struct registered with `wails.Run()`. Exported methods on the App struct are automatically bound to the JavaScript frontend. The App struct is a thin adapter—it holds references to the same internal service instances (`vault.Manager`, `auth.Registry`, `audit.EventBuilder`) used by the CLI's Cobra command handlers.

```go
// cmd/tegata-gui/app.go
type App struct {
    ctx   context.Context
    vault *vault.Manager      // same internal package as CLI
    auth  *auth.Registry      // same internal package as CLI
    audit *audit.EventBuilder // same internal package as CLI
}

func NewApp() *App {
    return &App{
        vault: vault.NewManager(),
        auth:  auth.NewRegistry(),
        audit: audit.NewEventBuilder(),
    }
}

func (a *App) startup(ctx context.Context) {
    a.ctx = ctx
}

// VaultUnlock is bound to JavaScript as window.go.main.App.VaultUnlock()
func (a *App) VaultUnlock(passphrase string) error {
    return a.vault.Unlock([]byte(passphrase))
}

// GetTOTPCode is bound to JavaScript as window.go.main.App.GetTOTPCode()
func (a *App) GetTOTPCode(label string) (TOTPResult, error) {
    return a.auth.GenerateTOTP(label)
}
```

The CLI's Cobra commands call the same `vault.Manager.Unlock()` and `auth.Registry.GenerateTOTP()` methods. The GUI adapter adds no business logic—it translates frontend calls to internal package calls.

### 7.2 Frontend framework

The GUI frontend uses React 18 with TypeScript 5, initialized from the official Wails react-ts template. React was chosen because the developer has existing React experience and the bundle size difference between React and lighter alternatives (Svelte, Preact) is negligible for an embedded desktop application where the assets are bundled into the binary.

Wails v2 automatically generates TypeScript bindings in `frontend/wailsjs/go/` from exported Go types. Developers do not manually maintain Go/TypeScript type parity—the build step regenerates bindings on each `wails build` or `wails dev` invocation.

The specific component library is deferred to v0.6 planning. UI frameworks evolve rapidly; locking in a component library now risks choosing one that is outdated by the time implementation begins. The architecture is framework-agnostic—any React component library can be added at v0.6.

### 7.3 CGO build requirements

The GUI binary requires CGO (unlike the CLI binary which uses `CGO_ENABLED=0`). Wails v2 depends on the system WebView for rendering.

| Platform      | WebView dependency                         | Installation                                     |
|---------------|--------------------------------------------|--------------------------------------------------|
| Windows 10+   | WebView2 (Microsoft Edge Chromium runtime) | Bundled with Windows 10 1903+ by default         |
| macOS 12+     | WKWebView                                  | System framework—no installation needed        |
| Linux (amd64) | WebKitGTK                                  | `apt install libgtk-3-dev libwebkit2gtk-4.0-dev` |

Because CGO is required, the GUI binary cannot be cross-compiled. Each platform binary must be built natively on that platform. This is the primary operational distinction from the CLI (which cross-compiles freely with `GOOS`/`GOARCH`).

Build commands for the GUI binary:

```bash
# Install Wails CLI (development dependency)
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Development mode (hot reload)
wails dev

# Production build
wails build -clean -o tegata-gui

# Windows with NSIS installer
wails build -clean -nsis -o tegata-gui.exe
```

### 7.4 Installer strategy

The GUI binary is installed on the host machine (unlike the CLI, which runs from the USB drive). Each platform uses its native installer format.

| Platform | Installer format | Tool                | Notes                                                                          |
|----------|------------------|---------------------|--------------------------------------------------------------------------------|
| Windows  | MSI (via NSIS)   | `wails build -nsis` | Built-in Wails support; requires NSIS installed                                |
| macOS    | DMG              | `create-dmg`        | Wails v2 does not natively produce DMG files                                   |
| Linux    | .deb / .rpm      | `nfpm`              | AppImage has known WebKitNetworkProcess issues with Wails; prefer native packages |

The installer bundles only the GUI binary and frontend assets. It does not bundle the vault, config, or CLI binary—those remain on the USB drive. The GUI discovers the vault file by using the same auto-detection logic as the CLI (section 5.3).

## 8. ScalarDL integration

This section details how Tegata communicates with ScalarDL Ledger for tamper-evident audit logging. All functionality in this section is optional—Tegata operates as a fully functional authenticator without it.

### 8.1 gRPC client

Tegata implements a lightweight gRPC client by using `grpc-go` against ScalarDL's protobuf service definitions. The client communicates with the `Ledger` gRPC service by using a single RPC method—`ExecuteContract`—and varies behavior by passing different contract identifiers.

| Operation  | Contract identifier | Purpose                                       |
|------------|---------------------|-----------------------------------------------|
| Put        | `object.Put`        | Store an authentication event record          |
| Get        | `object.Get`        | Retrieve event records for history display    |
| Validate   | `object.Validate`   | Verify hash-chain integrity across all events |

All three operations are invoked via the `ExecuteContract` RPC method on ScalarDL's `Ledger` gRPC service. The contract identifier is passed as the `ContractId` field in `ContractExecutionRequest`, along with a JSON-formatted `ContractArgument` and certificate credentials. The ScalarDL proto file defines the `Ledger` service with `ExecuteContract(ContractExecutionRequest) returns (ContractExecutionResponse)` as the primary entry point.

The client handles TLS negotiation, certificate-based authentication, and automatic retry with exponential backoff for transient failures.

#### gRPC stub generation

Generated stubs are committed to the repository so that `go build` succeeds without requiring protoc. Stubs are regenerated only when the ScalarDL proto version changes.

```bash
# Install protoc plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Download ScalarDL proto file
curl -O https://raw.githubusercontent.com/scalar-labs/scalardl/master/rpc/src/main/proto/scalar.proto

# Generate Go stubs into internal/audit/rpc/
protoc --go_out=internal/audit/rpc --go-grpc_out=internal/audit/rpc scalar.proto
```

The first client call pattern follows this structure:

```go
conn, err := grpc.NewClient(
    serverAddr,
    grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
)
client := rpc.NewLedgerClient(conn)

req := &rpc.ContractExecutionRequest{
    ContractId:       "object.Put",
    ContractArgument: `{"object_id": "event-001", "hash_value": "abc123..."}`,
    CertHolderId:     certHolderID,
    CertVersion:      1,
}
resp, err := client.ExecuteContract(ctx, req)
```

Note that `grpc.NewClient` is used instead of the deprecated `grpc.Dial`.

### 8.2 AuthEvent struct

Each authentication operation produces an `AuthEvent` that is serialized and submitted to ScalarDL.

```go
type AuthEvent struct {
    EventID       string    // UUID v4
    Timestamp     time.Time // UTC
    OperationType string    // "totp", "hotp", "challenge-response", "static"
    LabelHash     string    // SHA-256(credential_label)
    ServiceHash   string    // SHA-256(issuer or service name)
    HostHash      string    // SHA-256(machine fingerprint)
    Success       bool      // Whether the operation succeeded
}
```

Labels, service names, and host identifiers are always hashed before transmission. This protects privacy even if the audit log is compromised—an attacker cannot determine which services the user authenticates with.

### 8.3 HashStore contract usage

Tegata uses ScalarDL's HashStore abstraction rather than custom Java contracts. This avoids requiring a JDK toolchain for contract compilation and deployment.

**Event storage flow:**

1. Serialize the `AuthEvent` to JSON.
2. Compute `SHA-256(serialized_event)`.
3. Call `ExecuteContract` with contract identifier `object.Put`, passing the event hash as the value and a sequential asset ID in the JSON argument.
4. ScalarDL appends the entry to the hash chain and returns confirmation.

**Verification flow:**

1. Call `ExecuteContract` with contract identifier `object.Validate` and the asset ID range to verify.
2. ScalarDL traverses the hash chain, recomputes hashes, and returns the validation result.
3. Tegata reports the result to the user.

### 8.4 Offline queue

When the ScalarDL Ledger instance is unreachable, events are stored in an encrypted local queue (`queue.tegata`) on the USB drive.

**Queue format:** The queue file uses AES-256-GCM encryption with random 96-bit nonces (not the vault's write-counter approach). Random nonces are safe for the queue because queue writes are bounded by authentication frequency—at a realistic rate of 100 operations per day, it would take over 100 years to approach the birthday-bound collision risk of approximately 2^32 encryptions. Using random nonces avoids the complexity of shared counter state between the vault and queue files. Each queue entry stores its own nonce alongside the ciphertext.

The queue uses a key derived from the same passphrase but with a distinct salt (stored in the queue file header), ensuring cryptographic separation from the vault. Events are stored as a JSON array of `AuthEvent` objects. A local hash chain (each event includes a hash of the previous event) protects integrity during the offline window.

**Flush behavior:**

- On every `tegata` command that produces an audit event, the client first attempts to flush any queued events before submitting the new event.
- If the flush succeeds, the queue file is cleared.
- If the flush fails, the new event is appended to the queue.
- A configurable maximum queue size (`queue_max_events`, default 10,000) prevents unbounded growth. When the limit is reached, the oldest events are dropped with a warning.

### 8.5 `tegata verify` and `tegata history`

**`tegata verify`:** Calls `ExecuteContract` with the `object.Validate` contract on the ScalarDL Ledger instance. Reports the total number of events verified and whether the hash chain is intact. If tampering is detected, reports the range of affected events.

**`tegata history`:** Calls `ExecuteContract` with the `object.Get` contract to retrieve event records. Supports filtering by date range (`--from`, `--to`), credential label (`--label`), and operation type (`--type`). Displays results in a human-readable table or JSON (`--json`).

**`tegata ledger setup`:** Performs `RegisterCertificate` to register the user's TLS certificate with the ScalarDL Ledger, followed by a test `ExecuteContract` call that uses `object.Put` with a sentinel value to confirm connectivity. The exact `RegisterCertificate` proto message fields (`CertHolderId`, `CertVersion`) require integration-test validation against ScalarDL 3.12 before implementation.

## 9. Security model

This section describes the threat model, cryptographic rationale, and operational security measures. Users should read this section to understand both what Tegata protects against and what it does not.

### 9.1 Threat model

Tegata is designed to protect against the following threats.

**In scope (Tegata provides meaningful protection):**

| Threat                               | Mitigation                                                                                            |
|--------------------------------------|-------------------------------------------------------------------------------------------------------|
| USB drive loss or theft              | Vault is AES-256-GCM encrypted with Argon2id. Without the passphrase, brute-force is computationally infeasible. |
| Passive filesystem snooping          | Vault contents are never written to disk in plaintext. Temp files are encrypted before write.         |
| Audit log tampering (casual)         | ScalarDL hash-chain integrity detects modifications, deletions, and insertions.                       |
| Credential eavesdropping in transit  | All ScalarDL communication uses TLS. Audit events contain only hashed identifiers.                    |
| Brute-force passphrase attacks       | Argon2id with 64 MiB memory cost and rate-limiting with exponential backoff.                          |

**Out of scope (Tegata does NOT protect against):**

| Threat                                     | Limitation                                                                                         |
|--------------------------------------------|----------------------------------------------------------------------------------------------------|
| Memory extraction on compromised host      | Keys are decrypted in host memory during use. Malware with memory access can extract them.         |
| Keylogger capturing passphrase             | The passphrase is entered via stdin on the host. Tegata cannot protect against keyloggers.          |
| Physical access to unlocked session        | An unlocked vault is accessible until idle timeout. Physical access to the terminal is a compromise.|
| Fully compromised ScalarDL Ledger server   | An attacker controlling both the database and the ScalarDL process could reconstruct a valid chain. |
| Cold boot attacks                          | memguard mitigates this with mlock but cannot guarantee protection on all hardware.                 |

### 9.2 Cryptographic rationale

**AES-256-GCM** was chosen over AES-256-CBC or XChaCha20-Poly1305 because GCM provides authenticated encryption (confidentiality + integrity) in a single operation, is NIST-approved, has hardware acceleration on modern CPUs (AES-NI), and is well-supported by Go's standard library with constant-time implementations.

**Argon2id** was chosen over bcrypt, scrypt, or PBKDF2 because it is the winner of the Password Hashing Competition (2015), the `id` variant combines side-channel resistance (Argon2i) with GPU resistance (Argon2d), it is memory-hard (configurable to 64 MiB), and it is recommended by OWASP for password hashing.

**HMAC-SHA1 for TOTP/HOTP** is specified by RFC 6238 and RFC 4226. While SHA-1 is deprecated for collision resistance, it remains secure for HMAC construction (HMAC-SHA1 security depends on PRF properties, not collision resistance). SHA-256 and SHA-512 are also supported for TOTP.

### 9.3 Cryptographic pitfall mitigations

The following table documents the five most critical cryptographic pitfalls identified during research and the specific mitigations that Tegata's design enforces. Each pitfall has caused real-world vulnerabilities in similar software.

| Pitfall                              | Risk                                                                                                                                                                              | Mitigation                                                                                                                                                                                                                                                          |
|--------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Nonce reuse in AES-256-GCM           | Two encryptions with the same key and nonce break GCM security—the authentication key can be recovered and all prior messages can be forged                                     | Write-counter nonce: monotonic uint64 counter in vault header, incremented before each write, nonce derived deterministically as `counter_be8 \|\| zeros4`. Counter starts at 1; reads never increment; a crashed write leaves an incremented counter on the next successful write. See section 3.4. |
| Key material copies escaping guarded memory | Go's garbage collector does not zero freed memory. Key bytes copied into plain `[]byte` slices survive in memory after vault locks.                                        | All key material access goes through `internal/crypto/guard`. `KeyEnclave.Open(fn func([]byte) error)` ensures key bytes exist only inside the callback; the buffer is destroyed on return. No function should accept `key []byte` parameters—use `*guard.KeyEnclave` instead.               |
| Weak Argon2id parameters             | Using insufficient memory cost (e.g., t=1, m=4MiB, p=1) defeats GPU-resistance. Copy-pasted defaults from documentation examples are often dangerously low.                      | Defaults locked in vault header at creation: t=3, m=64MiB, p=4 (above OWASP minimums). `tegata bench` warns if derivation exceeds 3 seconds. Never allow t < 2 or m < 19 MiB without explicit override. See section 3.3.                                                                       |
| HOTP counter race condition          | If the counter is incremented in memory but the vault write fails, the in-memory and on-disk counters diverge, causing authentication failures.                                    | Correct order: (1) increment counter in-memory, (2) write vault with new counter to temp file, (3) atomic rename over vault, (4) generate and return code. If step 2 or 3 fails, the code is never displayed—user retries with same counter. See section 3.5 for atomic write strategy.        |
| memguard pre-v1 API instability      | memguard explicitly warns its API may change. Direct usage throughout the codebase means a breaking change requires updates across all packages.                                   | All memguard usage MUST go through `internal/crypto/guard`. This is an architectural rule, not a recommendation. Only `internal/crypto/guard` may import `github.com/awnumar/memguard`.                                                                                                          |

### 9.4 memguard key lifecycle

All memguard operations are accessed through the `internal/crypto/guard` wrapper package, which provides `SecretBuffer` (for sensitive byte slices like passphrases) and `KeyEnclave` (for the DEK, encrypted in RAM). This wrapper isolates memguard's pre-v1 API behind stable Tegata-specific types. See the pitfall table in section 9.3 for the architectural rationale.

The following lifecycle applies to all key material in Tegata.

```
Passphrase entry (stdin)
        │
        ▼
┌─────────────────────────┐
│  guard.NewSecretBuffer   │  Passphrase stored in mlock'd, guard-paged memory
└────────┬────────────────┘
         │
         ▼
┌─────────────────────────┐
│  Argon2id derivation     │  DEK derived, passphrase buffer destroyed
└────────┬────────────────┘
         │
         ▼
┌─────────────────────────┐
│  guard.NewKeyEnclave     │  DEK sealed in encrypted enclave (encrypted in RAM)
└────────┬────────────────┘
         │
         ▼  (on vault access)
┌─────────────────────────┐
│  KeyEnclave.Open(fn)     │  DEK temporarily unsealed inside callback, resealed on return
└────────┬────────────────┘
         │
         ▼
┌─────────────────────────┐
│  KeyEnclave.Destroy()    │  DEK memory zeroed and deallocated on vault lock
└─────────────────────────┘
```

**Rules:**

- Plaintext passphrases never exist outside of a `guard.SecretBuffer`.
- The DEK is stored in a `guard.KeyEnclave` (encrypted in RAM) between operations.
- After each vault operation, the unsealed DEK buffer is resealed immediately.
- On vault lock (idle timeout or explicit lock), the `KeyEnclave` is destroyed and all memory is zeroed.

### 9.5 Guard wrapper package

The `internal/crypto/guard` package provides stable Tegata-specific types over memguard's pre-v1 API. It is the only package in the codebase permitted to import `github.com/awnumar/memguard`.

```go
// internal/crypto/guard/guard.go

// SecretBuffer wraps memguard.LockedBuffer for sensitive byte slices
// (passphrases, plaintext credential bytes).
type SecretBuffer struct {
    lb *memguard.LockedBuffer
}

func NewSecretBuffer(data []byte) (*SecretBuffer, error)
func (s *SecretBuffer) Bytes() []byte   // read-only view; invalidated on Destroy
func (s *SecretBuffer) Destroy()

// KeyEnclave wraps memguard.Enclave for the DEK (encrypted in RAM).
type KeyEnclave struct {
    enc *memguard.Enclave
}

func NewKeyEnclave(key []byte) (*KeyEnclave, error)
func (e *KeyEnclave) Open(fn func([]byte) error) error  // unseals, calls fn, reseals
func (e *KeyEnclave) Destroy()
```

The `Open(fn)` pattern is the only way to access DEK bytes. This ensures the key is never stored in a local variable—the callback receives a slice backed by the guarded buffer, and the buffer is destroyed when the callback returns.

### 9.6 Passphrase rate-limiting

Failed passphrase attempts trigger exponential backoff.

| Attempt | Delay     |
|---------|-----------|
| 1–3     | No delay  |
| 4       | 1 second  |
| 5       | 2 seconds |
| 6       | 4 seconds |
| 7       | 8 seconds |
| 8+      | 16 seconds (cap) |

The delay is enforced in-process (not stored on disk) to prevent bypass by restarting the binary. After 20 consecutive failures, Tegata prints a warning suggesting the user may be using the wrong vault or passphrase.

### 9.7 Software authenticator limitations

Tegata prominently documents that it is a software authenticator, not a hardware security key. The following limitations are displayed during `tegata init` and in `tegata --help`.

- Keys are decrypted in host memory and are vulnerable to memory extraction.
- Tegata does not implement FIDO2/WebAuthn (which requires hardware attestation).
- Tegata does not provide tamper-resistant hardware isolation.
- Users requiring hardware-level security should use a dedicated hardware security key.

## 10. Error handling

Tegata uses structured error categories and actionable messages to help users resolve problems.

### 10.1 Error categories

Every error returned by Tegata falls into one of the following categories, each mapped to a distinct exit code.

| Category       | Exit code | Description                                     | Example                                      |
|----------------|-----------|------------------------------------------------|----------------------------------------------|
| `input`        | 1         | Invalid user input or missing arguments        | `"Label 'GitHub' not found. Run 'tegata list' to see available credentials."` |
| `auth`         | 2         | Authentication failure                         | `"Incorrect passphrase. 2 attempts remaining before rate-limiting."` |
| `vault`        | 3         | Vault file issues                              | `"Vault file is corrupted. A backup exists at vault.tegata.bak—run 'tegata import vault.tegata.bak' to restore."` |
| `network`      | 4         | ScalarDL connectivity issues                   | `"Cannot reach ScalarDL Ledger at localhost:50051. Event queued locally (3 events pending)."` |
| `integrity`    | 5         | Audit chain integrity violation                | `"Hash chain broken at event #843. Run 'tegata history --around 843' for details."` |

### 10.2 Actionable message format

Every error message follows this structure:

1. **What happened.** A clear description of the problem.
2. **What to do.** A concrete next step the user can take.

Error messages never display raw stack traces, internal error codes, or technical jargon without explanation.

### 10.3 Graceful degradation

ScalarDL failures never block authentication operations. If the audit layer encounters an error, Tegata:

1. Completes the authentication operation normally.
2. Queues the audit event in the offline queue.
3. Prints a non-blocking warning: `"Warning: Audit event queued locally (ScalarDL unreachable)."`.

This ensures that the P0 requirement (functional authenticator) is never compromised by the P1 requirement (audit logging).

## 11. Testing strategy

Testing covers correctness, security, and cross-platform compatibility.

### 11.1 Unit tests

Unit tests verify the core logic of each component.

**Authentication engines:**

- TOTP tests use the RFC 6238 appendix B test vectors (SHA-1, SHA-256, SHA-512 at multiple time values).
- HOTP tests use the RFC 4226 appendix D test vectors (counter values 0–9).
- Challenge-response tests use known HMAC vectors from RFC 2104 and RFC 4231.

**Vault:**

- Round-trip tests: encrypt then decrypt, verify data integrity.
- Wrong-passphrase tests: confirm GCM tag verification fails.
- Corrupted-file tests: truncated files, modified headers, and modified ciphertext.

**Configuration:**

- Precedence tests: verify that CLI flags override env vars override config file override defaults.
- Malformed config tests: missing fields, invalid values, unknown keys.

### 11.2 Integration tests

**ScalarDL integration:**

- A Docker Compose environment (`deployments/docker-compose/`) provides a local ScalarDL Ledger instance with PostgreSQL for integration testing.
- Tests verify the full event lifecycle: Put, Get, Validate.
- Tests verify offline queue flush behavior: disconnect, queue events, reconnect, verify flush.
- Tests verify `tegata verify` detects tampered records.

**End-to-end CLI tests:**

- Script-driven tests exercise the full CLI workflow: `init` -> `add` -> `code` -> `verify`.
- Tests run against a real vault file on a temporary directory simulating a USB drive.

### 11.3 Platform testing

The platform testing matrix differs between the CLI binary (cross-compiled, no CGO) and the GUI binary (native CGO builds required).

| Platform            | CLI CI             | GUI CI          | Manual testing     |
|---------------------|--------------------|-----------------|--------------------|
| Windows 10+ (amd64) | GitHub Actions     | Platform runner | Developer machine  |
| macOS 12+ (arm64)   | GitHub Actions     | Platform runner | Developer machine  |
| macOS 12+ (amd64)   | GitHub Actions     | Platform runner | —                  |
| Linux (amd64)       | GitHub Actions     | Platform runner | Docker/WSL         |

CLI cross-compilation is verified in CI by using `GOOS`/`GOARCH` flags with `CGO_ENABLED=0`.

### 11.4 GUI and CGO testing

GUI testing uses Wails' built-in test utilities for Go binding verification and Playwright (or similar browser automation) for end-to-end frontend tests. Because the GUI binary requires CGO, GUI-specific tests cannot run on `CGO_ENABLED=0` CI runners. GUI tests run on platform-specific CI runners or during manual testing. The shared service layer (`internal/`) is fully tested by the CLI test suite—GUI-specific tests focus on the binding layer and frontend interactions.

### 11.5 Security and fuzz testing

- **Fuzz testing:** Go's built-in fuzzing (`go test -fuzz`) targets the vault decrypt path, TOTP generation, and otpauth:// URI parsing.
- **Static analysis:** `go vet`, `staticcheck`, and `gosec` run in CI on every pull request.
- **Dependency auditing:** `govulncheck` runs in CI to detect known vulnerabilities in dependencies.

## 12. Development and deployment

This section covers the developer setup, build process, cross-compilation strategy, and USB drive layout.

### 12.1 Development prerequisites

The following tools are required for development.

| Tool           | Version | Purpose                                        |
|----------------|---------|------------------------------------------------|
| Go             | 1.25+   | Compiler and toolchain                         |
| Docker         | 24+     | ScalarDL integration testing                   |
| Docker Compose | 2.20+   | Local ScalarDL Ledger environment              |
| Git            | 2.40+   | Version control                                |
| golangci-lint  | 1.60+   | Linting and static analysis                    |
| Wails CLI      | 2.9+    | GUI binary build tool (GUI development only)   |
| Node.js        | 18+     | Frontend build toolchain (GUI development only)|
| npm            | 9+      | Frontend dependency management (GUI development only)|

The Wails CLI, Node.js, and npm are only required for GUI development. CLI-only development needs none of these.

### 12.2 Build commands

```bash
# Development build (current platform)
go build -o tegata ./cmd/tegata

# Run tests
go test ./...

# Run tests with race detector
go test -race ./...

# Run fuzz tests (vault decrypt path, 30 seconds)
go test -fuzz=FuzzVaultDecrypt -fuzztime=30s ./internal/vault/

# Lint
golangci-lint run ./...

# Security scan
gosec ./...
govulncheck ./...

# GUI development mode (hot reload, requires Wails CLI)
cd cmd/tegata-gui && wails dev

# GUI production build
cd cmd/tegata-gui && wails build -clean -o tegata-gui

# GUI Windows installer (requires NSIS)
cd cmd/tegata-gui && wails build -clean -nsis -o tegata-gui.exe
```

### 12.3 Cross-compilation and build matrix

Tegata produces two distinct binaries with different CGO requirements and cross-compilation strategies.

The following build matrix summarizes the key differences.

| Binary             | CGO                   | Cross-compile | Build tool    | Platforms                                       |
|--------------------|-----------------------|---------------|---------------|-------------------------------------------------|
| CLI (`tegata`)     | Disabled (CGO_ENABLED=0) | Yes        | `go build`    | Windows amd64, macOS arm64/amd64, Linux amd64   |
| GUI (`tegata-gui`) | Required              | No, native only | `wails build` | Windows amd64, macOS arm64/amd64, Linux amd64   |

The CLI binary cross-compiles freely because all dependencies are pure Go. The GUI binary requires CGO for Wails/WebView integration and must be built natively on each target platform. In CI, the CLI is cross-compiled from a single Linux runner; the GUI requires per-platform runners (or is built in platform-specific release workflows).

CLI cross-compilation commands:

```bash
# Windows (amd64)
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o tegata.exe ./cmd/tegata

# macOS (arm64 – Apple Silicon)
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o tegata-darwin-arm64 ./cmd/tegata

# macOS (amd64 – Intel)
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o tegata-darwin-amd64 ./cmd/tegata

# Linux (amd64)
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o tegata-linux-amd64 ./cmd/tegata
```

GUI binaries are not cross-compiled. See the Wails GUI architecture section for platform-specific build commands.

**CGO_ENABLED=0 constraint:** All Go dependencies must be pure Go. This rules out libraries that require CGo bindings (such as `go-sqlite3`). The decision to use JSON rather than SQLite for the vault format eliminates the primary CGo dependency risk.

**No elevated permissions (NFR-13):** Static binaries with no CGo dependencies, no installation step, and no system library requirements mean Tegata runs entirely in user space. It reads and writes only to the USB drive (vault, config, queue files) and the system clipboard. No admin/root privileges are required on any supported platform.

#### 12.3.1 Windows SmartScreen consideration

Windows SmartScreen will block execution of unsigned binaries downloaded from the internet. For v0.2, the CLI binary distributed via GitHub Releases will trigger SmartScreen warnings. Users can bypass this via PowerShell (`Unblock-File -Path tegata.exe`) or by right-clicking the binary and selecting "Unblock" in Properties. Code signing (via Authenticode) is planned for a future release when a code signing certificate is obtained.

### 12.4 USB drive layout

After setup, the USB drive or microSD card contains the following files. The GUI binary is installed on the host machine via platform-specific installers (see the Wails GUI architecture section). It is not included on the USB drive.

```
USB_DRIVE/
├── tegata.exe              # Windows binary
├── tegata-darwin-arm64     # macOS (Apple Silicon) binary
├── tegata-darwin-amd64     # macOS (Intel) binary
├── tegata-linux-amd64      # Linux binary
├── vault.tegata            # Encrypted vault file
├── tegata.toml             # Configuration (optional)
├── queue.tegata            # Offline audit queue (created when needed)
├── certs/                  # TLS certificates for ScalarDL (optional)
│   ├── client.pem
│   ├── client-key.pem
│   └── ca.pem
└── README.txt              # Quick-start instructions
```

All binaries are included so users can plug the drive into any supported platform and run the appropriate binary.

### 12.5 ScalarDL Docker Compose (development)

A `deployments/docker-compose/docker-compose.yml` provides a local ScalarDL Ledger instance for development and testing.

```yaml
services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: scalardl
      POSTGRES_USER: scalardl
      POSTGRES_PASSWORD: scalardl
    ports:
      - "5432:5432"
    volumes:
      - postgres-data:/var/lib/postgresql/data

  scalardl-ledger:
    image: ghcr.io/scalar-labs/scalardl-ledger:3.12
    depends_on:
      - postgres
    ports:
      - "50051:50051"
      - "50052:50052"
    environment:
      SCALAR_DL_LEDGER_DATABASE_TYPE: postgres
      SCALAR_DL_LEDGER_DATABASE_HOST: postgres
      SCALAR_DL_LEDGER_DATABASE_PORT: 5432
      SCALAR_DL_LEDGER_DATABASE_NAME: scalardl
      SCALAR_DL_LEDGER_DATABASE_USER: scalardl
      SCALAR_DL_LEDGER_DATABASE_PASSWORD: scalardl

volumes:
  postgres-data:
```

Run `docker compose up -d` from the `deployments/docker-compose/` directory to start the environment. Integration tests automatically detect this local instance.

### 12.6 CI/CD

GitHub Actions runs the following pipeline on every pull request.

| Step                  | Description                                             |
|-----------------------|---------------------------------------------------------|
| `go vet`              | Standard Go static analysis                             |
| `golangci-lint`       | Extended linting (unused code, error handling, style)   |
| `go test -race`       | Unit and integration tests with race detector           |
| `gosec`               | Security-focused static analysis                        |
| `govulncheck`         | Known vulnerability detection in dependencies           |
| Cross-compile (4 targets) | Verify all platform builds succeed                 |
| Binary size check     | Warn if any binary exceeds 20 MB (NFR-10)               |

Release builds use `goreleaser` to produce tagged, checksummed binaries for each platform. Releases are published as GitHub Releases with SHA-256 checksums.

## 13. Future considerations

The following features are explicitly deferred from v1.0 but may be considered in future versions.

**Terminal user interface (TUI):** A guided interactive interface that uses a library such as `bubbletea` for users who prefer visual navigation over CLI commands. Planned for v0.5 per the release plan.

**FIDO2/WebAuthn:** FIDO2/WebAuthn is excluded from all Tegata versions. FIDO2 is designed for hardware attestation, and a software implementation would not provide the security guarantees that relying parties expect. See PRD section 12 Q4 for the full rationale.

**Standalone gRPC library:** The ScalarDL gRPC client implemented for Tegata could be extracted as a standalone open-source Go library, benefiting the broader ScalarDL ecosystem. This adds maintenance scope and is deferred until the client implementation stabilizes.

**Multi-user support:** The current design assumes a single user per vault. Supporting multiple users (for example, a shared team vault) would require access control, per-user encryption keys, and conflict resolution. This is a significant architectural change deferred to a future major version.

**GUI application:** The GUI architecture is fully specified in section 7. Implementation begins at v0.6 with full CLI feature parity. Component library selection is deferred to v0.6 planning.

## 14. References

Standards and specifications referenced in this document.

- [RFC 6238 – TOTP: Time-Based One-Time Password Algorithm](https://datatracker.ietf.org/doc/html/rfc6238)
- [RFC 4226 – HOTP: An HMAC-Based One-Time Password Algorithm](https://datatracker.ietf.org/doc/html/rfc4226)
- [RFC 2104 – HMAC: Keyed-Hashing for Message Authentication](https://datatracker.ietf.org/doc/html/rfc2104)
- [RFC 4231 – Identifiers and Test Vectors for HMAC-SHA-224, HMAC-SHA-256, HMAC-SHA-384, and HMAC-SHA-512](https://datatracker.ietf.org/doc/html/rfc4231)
- [NIST SP 800-38D – Recommendation for Block Cipher Modes of Operation: Galois/Counter Mode (GCM)](https://csrc.nist.gov/pubs/sp/800/38/d/final)
- [Argon2 Reference Implementation](https://github.com/P-H-C/phc-winner-argon2)
- [OWASP Password Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)

Go libraries referenced in this document.

- [spf13/cobra – CLI framework](https://github.com/spf13/cobra)
- [awnumar/memguard – Guarded memory for Go](https://github.com/awnumar/memguard)
- [grpc-go – gRPC for Go](https://github.com/grpc/grpc-go)
- [BurntSushi/toml – TOML parser for Go](https://github.com/BurntSushi/toml)
- [atotto/clipboard – Cross-platform clipboard for Go](https://github.com/atotto/clipboard)
- [golang.org/x/crypto – Extended Go crypto library (Argon2)](https://pkg.go.dev/golang.org/x/crypto)
- [wailsapp/wails – Desktop GUI framework](https://github.com/wailsapp/wails)

ScalarDL documentation.

- [ScalarDL Documentation](https://scalardl.scalar-labs.com/docs/latest/)
- [Get Started with ScalarDL HashStore](https://scalardl.scalar-labs.com/docs/latest/getting-started-hashstore/)
- [Write a ScalarDL Application with the HashStore Abstraction](https://scalardl.scalar-labs.com/docs/latest/how-to-write-applications-with-hashstore/)
- [ScalarDL GitHub Repository](https://github.com/scalar-labs/scalardl)
- [ScalarDL Protobuf Definitions](https://github.com/scalar-labs/scalardl/tree/master/rpc/src/main/proto)
