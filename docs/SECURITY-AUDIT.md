# Security audit

This document is a structured self-audit of Tegata's security implementation. It covers four areas: cryptographic implementation, memory handling, vault format and integrity, and input validation. Each finding is classified as PASS (meets requirements), NOTE (informational observation), or FINDING (issue requiring attention).

## Methodology

This audit was conducted as a self-review of the Tegata v1.0 codebase. It is not an independent third-party audit. The review covers all security-sensitive code paths by examining the Go source files directly, verifying that implementation matches the design document specifications, and documenting known limitations.

**Review date:** March 2026
**Codebase version:** v1.0
**Scope:** Core authenticator (CLI, TUI, GUI adapter), vault encryption, key derivation, audit layer

## Area 1: Cryptographic implementation

This section reviews the cryptographic primitives, key derivation, and authentication protocols.

**[PASS] Key derivation: Argon2id with recommended parameters.**
Tegata uses Argon2id via `golang.org/x/crypto/argon2` with parameters time=3, memory=64 MiB, parallelism=4, keyLen=32 (`internal/crypto/kdf.go`, `DefaultParams`). These parameters meet the OWASP recommendation of Argon2id with a minimum 19 MiB memory and 2 iterations. The `DeriveKey` function returns a `*guard.SecretBuffer`, ensuring derived keys are stored in guarded memory.

**[PASS] Encryption: AES-256-GCM with counter-derived nonces.**
Vault encryption uses AES-256-GCM (`internal/crypto/aead.go`) via the Go standard library `crypto/aes` and `crypto/cipher` packages. Nonces are derived deterministically as `counter_be8 || zeros4` (12 bytes total) from a monotonic write counter, matching design document section 3.4. This avoids random nonce collision risk while remaining safe because each counter value is used at most once with a given key. No custom cryptographic primitives are implemented.

**[PASS] TOTP/HOTP: RFC 6238 and RFC 4226 compliance.**
TOTP (`internal/auth/totp.go`) and HOTP (`internal/auth/hotp.go`) implementations follow RFC 6238 and RFC 4226 respectively. The shared `computeHOTP` function in `internal/auth/otp.go` implements dynamic truncation per RFC 4226 section 5.4 using big-endian counter encoding. SHA-1 is the default HMAC function per the RFC specifications; SHA-256 and SHA-512 are supported via `hashFuncFromAlgorithm`. SHA-1 usage in the HMAC context is not a weakness because HMAC-SHA1 security depends on the pseudorandom function properties of SHA-1, not its collision resistance.

**[PASS] Challenge-response: HMAC-SHA256/SHA1 signing.**
Challenge-response signing (`internal/auth/cr.go`) uses `crypto/hmac` with either `sha256.New` or `sha1.New` depending on the credential's algorithm field. Neither the challenge nor the response is logged at any slog level. The challenge is treated as raw bytes with no hex decoding, leaving encoding responsibility to the caller.

**[PASS] No custom cryptographic primitives.**
All cryptographic operations use the Go standard library (`crypto/aes`, `crypto/cipher`, `crypto/hmac`, `crypto/sha256`), `golang.org/x/crypto/argon2`, and `github.com/awnumar/memguard`. No custom ciphers, PRNGs, or hash functions are implemented.

**[NOTE] Counter reuse safety in key wrapping.**
Both vault creation and passphrase change use counter=1 for wrapping the DEK (`internal/vault/manager.go`). This is safe because each wrapping operation uses a freshly generated random salt, producing a unique derived key. The (key, nonce) pair is therefore unique despite the fixed counter value.

## Area 2: Memory handling

This section reviews how sensitive data (passphrases, keys, plaintext credentials) is managed in memory.

**[PASS] memguard isolation: single import point.**
The `memguard` package is imported only in `internal/crypto/guard/guard.go`. A grep of the codebase confirms no other package imports `github.com/awnumar/memguard` directly. All other packages use the `guard.SecretBuffer` and `guard.KeyEnclave` wrapper types.

**[PASS] DEK storage in encrypted enclave.**
After vault unlock, the data encryption key is sealed into a `guard.KeyEnclave` (`internal/vault/manager.go`, `guard.Seal`), which encrypts the key at rest in memory without consuming mlock quota. The enclave is only opened to a `SecretBuffer` during active encryption/decryption operations and immediately destroyed afterward.

**[PASS] Passphrase zeroing in CLI commands.**
CLI command handlers in `cmd/tegata/helpers.go` and individual command files consistently use `defer zeroBytes(passphrase)` immediately after obtaining the passphrase from `promptPassphrase`. The `promptNewPassphrase` function zeros confirmation copies and returns errors after zeroing on mismatch.

**[PASS] Passphrase zeroing in GUI adapter.**
The GUI adapter (`cmd/tegata-gui/app.go`) converts string passphrases from JavaScript to byte slices and uses `defer zeroBytes(passBytes)` consistently across all vault operations (unlock, create, export, import, change passphrase, verify recovery).

**[FINDING - Low] JavaScript string immutability in GUI.**
The Wails GUI receives passphrases as JavaScript strings from the React frontend. JavaScript strings are immutable and garbage-collected non-deterministically, meaning the passphrase may persist in the V8/SpiderMonkey heap after the Go adapter zeroes its byte slice copy. The React frontend clears state immediately, but there is no mechanism to force zeroing of the original JS string in the runtime heap. This is an inherent limitation of the WebView architecture.

**Status:** Accepted. The Go side zeroes its copy immediately. Mitigating the JS heap limitation would require a fundamentally different architecture (such as a native GUI without WebView). This is documented as a known limitation.

**[PASS] Plaintext JSON zeroing.**
After marshaling vault payloads to JSON for encryption, the plaintext JSON byte slice is zeroed immediately via `zeroBytes(payloadJSON)` in both `vault.Create` and `vault.Save` (`internal/vault/manager.go`). The same pattern is applied in `ExportCredentials`.

**[PASS] Input data zeroing in guard.NewSecretBuffer.**
`guard.NewSecretBuffer` explicitly zeroes the input byte slice after copying it into guarded memory (`internal/crypto/guard/guard.go`, lines 33-35), ensuring the caller's original allocation does not retain key material.

## Area 3: Vault format and integrity

This section reviews the vault file format, header serialization, rate limiting, key architecture, and crash safety.

**[PASS] Header serialization: explicit byte offsets.**
`internal/vault/header.go` uses explicit byte offsets for marshaling and unmarshaling the 128-byte vault header. This avoids padding issues that would arise from `binary.Read`/`binary.Write` with struct fields. An offset assertion (`if off != headerSize`) guards against serialization drift.

**[PASS] Magic bytes and version validation.**
The vault file begins with an 8-byte magic signature (`TEGATA\0\0`) and a 2-byte version field. `Unmarshal` rejects files with incorrect magic bytes or unsupported versions with `ErrVaultCorrupt`.

**[PASS] Argon2id parameter validation on open.**
When opening a vault file, `vault.Open` validates the Argon2id parameters from the header against reasonable bounds (time: 1-100, memory: 8 KiB to 4 GiB, parallelism: >= 1). This prevents denial-of-service from crafted vault files that set unreasonable memory or iteration counts.

**[PASS] Rate limiting with exponential backoff.**
Failed passphrase attempts are tracked in the vault header (`FailedAttempts`, `LastAttemptTime` fields) and persist across sessions because they are stored in the header's reserved bytes. The rate limit state is updated on both passphrase and recovery key unlock attempts. The `RecordFailure` and `CheckRateLimit` functions implement exponential backoff.

**[PASS] DEK architecture: dual key wrapping.**
The vault uses a random 32-byte data encryption key (DEK) that is independently wrapped by both the passphrase-derived key and the recovery-derived key. Passphrase rotation re-wraps only the DEK without re-encrypting the credential payload, making the operation fast and crash-safe.

**[PASS] Atomic writes via temp-file-rename.**
`vault.atomicWrite` writes to a `.tmp` file, renames the existing vault to `.bak`, then renames `.tmp` to the final path. If the rename fails, the backup is restored. The backup file contents are overwritten with zeros before deletion to prevent encrypted bytes from persisting in unallocated disk blocks.

**[NOTE] FAT32 atomic write limitation on Windows.**
The temp-file-rename pattern is as close to atomic as possible on FAT32, but FAT32 does not provide true atomic rename guarantees. A power failure during the rename operation could theoretically leave the vault in an inconsistent state. The `.bak` file provides recovery in most scenarios. This edge case requires integration testing with a real FAT32 disk image.

## Area 4: Input validation and error handling

This section reviews how user input is validated, error messages are constructed, and audit privacy is maintained.

**[PASS] Vault path resolution.**
`resolveVaultPath` in `cmd/tegata/helpers.go` uses a strict resolution order (flag, env var, current directory) and returns a clear error message with remediation advice when no vault is found. The `resolvePathArg` function handles both directory and file path inputs correctly.

**[PASS] Passphrase strength enforcement.**
`promptNewPassphrase` enforces a minimum length of 8 characters and displays a strength meter (Weak/Fair/Strong based on length). Confirmation is required for interactive use. The `TEGATA_PASSPHRASE` environment variable is accepted for scripting but triggers a warning on stderr.

**[PASS] Base32 secret validation.**
The `decodeBase32Secret` helper in `cmd/tegata/helpers.go` tolerates common formatting variations (spaces, hyphens, lowercase, missing padding) and corrects common digit lookalikes (0 to O, 1 to L, 8 to B). Invalid base32 input produces a clear error message.

**[PASS] Credential type validation.**
The `add` command validates credential types against the known set (totp, hotp, static, challenge-response) and rejects unknown types with `ErrInvalidInput`.

**[PASS] GUI adapter input validation.**
The Wails GUI adapter in `cmd/tegata-gui/app.go` validates all parameters before delegating to internal packages. Passphrase bytes are converted from strings and zeroed after use.

**[PASS] Error message pattern.**
Error messages follow the pattern "Error: what happened. What to do." and never leak internal state, key material, or stack traces. The `errors.UserMessage` helper provides structured user-facing error messages with remediation advice.

**[PASS] Audit privacy: hashed labels and service names.**
Audit event records use hashed labels and service names before submission to ScalarDL Ledger, ensuring that credential names are never stored in plaintext in the audit log. The hashing happens in the `EventBuilder` before any network transmission.

**[PASS] Offline queue encryption.**
The offline event queue (`internal/audit/queue.go`) uses AES-256-GCM with random 12-byte nonces for each entry. The queue key is derived from the vault passphrase via Argon2id using a distinct salt stored in the queue file header. Re-encryption with fresh nonces occurs on every `Save` call, ensuring that two saves of the same queue produce different ciphertext.

**[PASS] Queue hash-chain integrity.**
Each queue entry stores a `PrevHash` (hex SHA-256 of the previous entry's plaintext JSON), forming a local hash chain. `VerifyChain` validates the chain integrity and returns `ErrIntegrityViolation` on any mismatch.

## Summary of findings

The following table summarizes all findings from this audit.

| ID  | Area          | Severity | Status   | Description                                               |
|-----|---------------|----------|----------|-----------------------------------------------------------|
| F-1 | Memory        | Low      | Accepted | JS string immutability in GUI WebView prevents zeroing    |
| N-1 | Crypto        | Info     | N/A      | Counter=1 reuse in key wrapping is safe due to fresh salts |
| N-2 | Vault         | Info     | N/A      | FAT32 atomic write not guaranteed on power failure         |

## Known limitations

The following limitations are accepted risks for v1.0.

**JavaScript memory zeroing.** The GUI uses a Wails WebView that receives passphrases as JavaScript strings. JavaScript strings are immutable and garbage-collected non-deterministically. The Go adapter zeroes its byte slice copy immediately, but the original JS string may persist in the WebView heap. Users requiring hardware-level memory protection should use hardware security keys instead.

**Unsigned macOS builds.** Pre-built macOS binaries are not currently signed with an Apple Developer certificate. Users may need to bypass Gatekeeper warnings on first launch. Code signing will be added in a future release.

**FAT32 atomic write edge cases.** The temp-file-rename pattern provides crash safety on most filesystems, but FAT32 on Windows does not guarantee atomic rename. A power failure during the rename operation could leave the vault in an inconsistent state. The `.bak` file provides recovery in most scenarios.

**No hardware key isolation.** Tegata decrypts keys in host memory during use. It does not provide the tamper-resistant key isolation of hardware security keys like YubiKey. Tegata is designed for portability and auditability, not for environments requiring hardware-level protection.
