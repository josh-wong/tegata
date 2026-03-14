# Tegata — CLI/TUI mockups

This document is the visual specification for every CLI command output and TUI flow in Tegata. A developer reading this document can match the output of any command character-for-character during implementation, without consulting the design document. Sections 1 through 4 cover conventions, v0.2 CLI commands, v0.3+ CLI commands (placeholder), and error states. Sections 5 through 7 cover TUI wireframes (placeholders, to be populated in subsequent plans).

## Table of contents

- [1. Document conventions](#1-document-conventions)
  - [1.1 Symbol table](#11-symbol-table)
  - [1.2 Color annotations](#12-color-annotations)
  - [1.3 JSON envelope](#13-json-envelope)
  - [1.4 Exit codes](#14-exit-codes)
  - [1.5 NO\_COLOR behavior](#15-no_color-behavior)
- [2. CLI mockups — v0.2 commands](#2-cli-mockups--v02-commands)
  - [2.1 tegata init](#21-tegata-init)
  - [2.2 tegata add](#22-tegata-add)
  - [2.3 tegata list](#23-tegata-list)
  - [2.4 tegata code](#24-tegata-code)
  - [2.5 tegata remove](#25-tegata-remove)
- [3. CLI mockups — v0.3+ commands](#3-cli-mockups--v03-commands)
  - [3.1 tegata sign](#31-tegata-sign)
  - [3.2 tegata get](#32-tegata-get)
  - [3.3 tegata export and tegata import](#33-tegata-export-and-tegata-import)
  - [3.4 tegata resync](#34-tegata-resync)
  - [3.5 tegata history](#35-tegata-history)
  - [3.6 tegata verify](#36-tegata-verify)
  - [3.7 tegata ledger setup](#37-tegata-ledger-setup)
  - [3.8 tegata config show](#38-tegata-config-show)
  - [3.9 tegata version](#39-tegata-version)
- [4. CLI mockups — error states](#4-cli-mockups--error-states)
  - [4.1 Wrong passphrase (exit 2)](#41-wrong-passphrase-exit-2)
  - [4.2 Missing vault (exit 3)](#42-missing-vault-exit-3)
  - [4.3 Corrupted vault (exit 3)](#43-corrupted-vault-exit-3)
  - [4.4 ScalarDL unreachable (exit 4)](#44-scalardl-unreachable-exit-4)
  - [4.5 Integrity violation (exit 5)](#45-integrity-violation-exit-5)
- [5. TUI wireframes — first-time setup](#5-tui-wireframes--first-time-setup)
  - [5.1 Step 1 of 4 — Welcome](#51-step-1-of-4--welcome)
  - [5.2 Step 2 of 4 — Create passphrase](#52-step-2-of-4--create-passphrase)
  - [5.3 Step 3 of 4 — Recovery key](#53-step-3-of-4--recovery-key)
  - [5.4 Step 4 of 4 — Add first credential](#54-step-4-of-4--add-first-credential)
- [6. TUI wireframes — daily use](#6-tui-wireframes--daily-use)
  - [6.1 Unlock vault](#61-unlock-vault)
  - [6.2 Main view with code generation](#62-main-view-with-code-generation)
  - [6.3 Auto-lock](#63-auto-lock)
- [7. TUI wireframes — credential management](#7-tui-wireframes--credential-management)
  - [7.1 Credential list view](#71-credential-list-view)
  - [7.2 Add credential](#72-add-credential)
  - [7.3 Remove credential (confirmation)](#73-remove-credential-confirmation)

---

## 1. Document conventions

This section defines the rules that all mockups in this document follow. Conventions established here apply uniformly to every subsequent section.

### 1.1 Symbol table

The following Unicode symbols appear throughout this document. They are chosen because they are present in all modern terminal fonts and remain meaningful without color.

| Symbol | Meaning                        | Unicode code point |
|--------|--------------------------------|--------------------|
| `✓`    | Success or positive outcome    | U+2713             |
| `✗`    | Error or negative outcome      | U+2717             |
| `!`    | Warning (non-blocking)         | ASCII 0x21         |
| `→`    | Pointer, flow, or continuation | U+2192             |
| `•`    | Bullet point in lists          | U+2022             |

These symbols are **not** replaced with ASCII fallbacks when `NO_COLOR` is set. They convey semantic meaning independent of color — the `✓` and `✗` symbols make success and failure distinguishable without relying on green and red.

### 1.2 Color annotations

Human-readable output mockups use inline color annotations to indicate what color each line would be rendered in when color is enabled. The annotation format is `(colorname)` appended at the end of the line it applies to.

The color semantics are:

- `(green)` — success messages and positive confirmations
- `(red)` — errors and failure messages
- `(yellow)` — warnings and non-blocking notices
- `(cyan)` — labels, column headers, and structural elements

Data values (the credential label, the TOTP code, the vault path) are **never** colored. Color applies only to the structural text around data values. This ensures that data remains clearly readable at any color depth.

### 1.3 JSON envelope

Every command that supports `--json` produces a JSON object following this envelope. The rules are:

- All responses include `"status": "ok"` or `"status": "error"` at the top level.
- Success responses add their data fields at the top level (flat, not nested under a `"data"` key).
- Success responses that return a list use a named array field (for example, `"credentials": [...]`), not an anonymous array.
- Error responses always include `"code"` (integer, matching exit code) and `"message"` (string, human-readable).

**Success example:**

```json
{
  "status": "ok",
  "vault_path": "./vault.tegata"
}
```

**Error example:**

```json
{
  "status": "error",
  "code": 2,
  "category": "auth",
  "message": "Incorrect passphrase. 2 attempts remaining before rate-limiting."
}
```

### 1.4 Exit codes

The following exit codes are used by all Tegata commands. They are reproduced from design document section 5.5.

| Code | Meaning                                     |
|------|---------------------------------------------|
| 0    | Success                                     |
| 1    | General error (invalid input, missing file) |
| 2    | Authentication error (wrong passphrase)     |
| 3    | Vault error (corrupted, missing, locked)    |
| 4    | Network error (ScalarDL unreachable)        |
| 5    | Integrity error (audit chain broken)        |

Exit codes appear in mockups as `[exit N]` annotations placed below the code block, outside of it.

### 1.5 NO\_COLOR behavior

When the `NO_COLOR` environment variable is set to any value (including an empty string), Tegata suppresses all ANSI color codes. The following elements are **not** affected:

- Unicode symbols (`✓`, `✗`, `!`, `→`, `•`) remain unchanged.
- Progress bar characters (`█`, `░`) remain unchanged.
- Column alignment and spacing remain unchanged.
- All text content remains unchanged.

The output is functionally identical to the colored version except that no ANSI escape sequences are emitted. A `NO_COLOR` environment does not require separate mockups — remove the `(colorname)` annotations mentally and the text represents exactly what appears.

---

## 2. CLI mockups — v0.2 commands

The commands in this section constitute the v0.2 release scope: vault initialization, credential management (add, list, remove), and code generation. Each subsection shows the human-readable output, the `--json` output, and design notes.

### 2.1 `tegata init`

The init command creates a new vault and guides the user through passphrase creation and recovery key display.

**Human-readable output:**

```
$ tegata init
 _                    _
| |_ ___  __ _  __ _| |_ __ _     (cyan)
| __/ _ \/ _` |/ _` | __/ _` |    (cyan)
 \__\___/\__, |\__,_|\__\__,_|    (cyan)
         |___/                     (cyan)

Tegata — portable authenticator (v0.2.0)

Step 1 of 2: Create your vault passphrase
Passphrase: ········
Confirm:    ········

✓ Vault created at ./vault.tegata                             (green)

Step 2 of 2: Save your recovery key
──────────────────────────────────────────────────────
ABCD-EFGH-IJKL-MNOP-QRST-UVWX-YZAB-CDEF-GHIJ-KLMN-OPQR-STUV-WX
──────────────────────────────────────────────────────
! Store this key offline. It will not be shown again.         (yellow)

✓ Setup complete. Run 'tegata add --totp <label>' to add your first credential.  (green)
```

**JSON output (`--json`):**

```json
{
  "status": "ok",
  "vault_path": "./vault.tegata"
}
```

> **Design notes:** The ASCII art logo uses only standard Latin characters available in every terminal font. The four logo lines are colored cyan as structural decoration — they are the only decorative element in the CLI. The recovery key uses 4-character groups separated by hyphens (13 groups of 4 characters = 52 characters from the base32 alphabet, representing a 256-bit key with 4 bits of padding), matching the SSH key fingerprint convention users already recognize. The horizontal separator lines use the em-dash character (──) without color, so the key remains visually bracketed even under `NO_COLOR`. The recovery key is intentionally absent from the JSON output — never include key material in machine-parseable output where it could be logged.

### 2.2 `tegata add`

The add command stores a new credential in the vault. Secrets are always prompted interactively rather than passed as command-line arguments, preventing shell history from recording sensitive material.

**Variant (a) — Adding a TOTP credential:**

```
$ tegata add --totp GitHub
Passphrase: ········
TOTP secret: ········

✓ Credential 'GitHub' added (totp).  (green)
```

**Variant (b) — Adding a static password:**

```
$ tegata add --static DB-pass
Passphrase: ········
Password: ········

✓ Credential 'DB-pass' added (pw).  (green)
```

**JSON output (`--json`, TOTP example):**

```json
{
  "status": "ok",
  "label": "GitHub",
  "type": "totp"
}
```

> **Design notes:** Secrets are prompted interactively regardless of credential type — the `[secret]` positional argument in the command tree is only for `--scan` mode (which accepts an `otpauth://` URI piped from a QR code decoder). The `--scan` variant follows the same output format as variant (a) above. The credential type in the confirmation message uses the short badge form (`totp`, `hotp`, `cr`, `pw`) that also appears in `tegata list` output, establishing visual consistency.

### 2.3 `tegata list`

The list command displays all credentials stored in the vault. Three states are specified because the column layout and empty-state message must be defined before implementation.

**State (a) — Empty vault:**

```
$ tegata list
Passphrase: ········

No credentials. Run 'tegata add --totp <label>' to add your first credential.
```

**State (b) — Typical vault (4 credentials):**

```
$ tegata list
Passphrase: ········

Label               Type  Algorithm  Added       (cyan)
──────────────────  ────  ─────────  ──────────  (cyan)
GitHub              totp  SHA-1      2026-03-12
AWS-prod            hotp  SHA-1      2026-03-12
SSH-signing         cr    SHA-256    2026-03-13
WiFi-office         pw    —          2026-03-14

4 credentials
```

**State (c) — Long label edge case (truncated):**

```
$ tegata list
Passphrase: ········

Label               Type  Algorithm  Added       (cyan)
──────────────────  ────  ─────────  ──────────  (cyan)
AWS-production-acc… totp  SHA-1      2026-03-12
GitHub              totp  SHA-1      2026-03-13

2 credentials
```

**JSON output (`--json`):**

```json
{
  "status": "ok",
  "credentials": [
    {
      "label": "GitHub",
      "type": "totp",
      "algorithm": "SHA-1",
      "added": "2026-03-12"
    },
    {
      "label": "AWS-prod",
      "type": "hotp",
      "algorithm": "SHA-1",
      "added": "2026-03-12"
    },
    {
      "label": "SSH-signing",
      "type": "cr",
      "algorithm": "SHA-256",
      "added": "2026-03-13"
    },
    {
      "label": "WiFi-office",
      "type": "pw",
      "algorithm": null,
      "added": "2026-03-14"
    }
  ]
}
```

> **Design notes:** Column widths are fixed: Label is 18 characters (truncated with `…` at position 18 if longer), Type is 4 characters, Algorithm is 9 characters, Added is 10 characters (ISO date only, no time). The separator line uses the em-dash character (──) to fill each column width exactly. The empty-state output skips column headers entirely — headers with no rows create visual confusion. The Algorithm column shows `—` (em-dash) for static passwords since they have no algorithm. In JSON output, `"algorithm"` is `null` for static passwords.

### 2.4 `tegata code`

The code command generates an authentication code. TOTP codes live-update in place with a countdown; HOTP codes display the counter value without a countdown.

**TOTP variant (live-updating, Ctrl+C to exit):**

```
$ tegata code GitHub
Passphrase: ········

GitHub
482 913  [████████░░] 18s   (updates in place every second)

✓ Copied to clipboard (auto-clear in 45s)                    (green)

^C
```

**HOTP variant:**

```
$ tegata code AWS-prod
Passphrase: ········

AWS-prod
483 029  (counter: 43)

✓ Copied to clipboard (auto-clear in 45s)                    (green)
```

**JSON output (`--json`, TOTP):**

```json
{
  "status": "ok",
  "code": "482913",
  "ttl": 18,
  "label": "GitHub",
  "type": "totp"
}
```

**JSON output (`--json`, HOTP):**

```json
{
  "status": "ok",
  "code": "483029",
  "counter": 43,
  "label": "AWS-prod",
  "type": "hotp"
}
```

> **Design notes:** The TOTP code line overwrites itself in place using ANSI cursor-up and carriage return — no scrolling history accumulates. When the 30-second window expires, the new code replaces the old one on the same line. The progress bar uses 10 blocks: at t=0 all 10 are filled (█); each 3 seconds one block becomes empty (░). The clipboard copy contains digits only — no space between the two 3-digit groups (`"482913"`, not `"482 913"`). HOTP has no countdown because the code does not expire on a time window; it shows the counter value instead so the user can verify counter synchronization. JSON output includes `"ttl"` for TOTP and `"counter"` for HOTP — never both.

### 2.5 `tegata remove`

The remove command deletes a credential from the vault after explicit confirmation. The default answer is no, requiring the user to type `y` explicitly.

**Human-readable output:**

```
$ tegata remove GitHub
Passphrase: ········

Remove credential 'GitHub'? This cannot be undone. [y/N] y

✓ Credential 'GitHub' removed.  (green)
```

**JSON output (`--json`):**

```json
{
  "status": "ok",
  "label": "GitHub",
  "removed": true
}
```

> **Design notes:** The confirmation prompt uses `[y/N]` with uppercase `N` to signal that the default is no — pressing Enter without typing `y` aborts the operation. The `--force` flag skips the prompt entirely (useful for scripting). In `--json` mode, the confirmation prompt is suppressed; `--force` is required to proceed non-interactively, otherwise the command exits with code 1 and an error message.

---

## 3. CLI mockups — v0.3+ commands

The commands in this section constitute the v0.3+ release scope: challenge-response signing, static password retrieval, vault portability, HOTP counter resynchronization, audit history and verification, ledger configuration, general configuration display, and version information. Each subsection follows the same structure as section 2: human-readable output, `--json` output, and design notes.

### 3.1 `tegata sign`

The sign command performs HMAC-SHA256 challenge-response signing using a stored challenge-response credential. The challenge can be entered interactively or passed as a flag.

**Variant (a) — Interactive challenge entry:**

```
$ tegata sign SSH-key
Passphrase: ········
Challenge: my-ssh-server-nonce-20260314

✓ Signature: a3f8c2d9e1b7045f6a92c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718  (green)
✓ Copied to clipboard (auto-clear in 45s)                                    (green)
```

**Variant (b) — Non-interactive with `--challenge` flag:**

```
$ tegata sign SSH-key --challenge my-ssh-server-nonce-20260314
Passphrase: ········

✓ Signature: a3f8c2d9e1b7045f6a92c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718  (green)
✓ Copied to clipboard (auto-clear in 45s)                                    (green)
```

**JSON output (`--json`):**

```json
{
  "status": "ok",
  "label": "SSH-key",
  "type": "cr",
  "algorithm": "SHA-256",
  "signature": "a3f8c2d9e1b7045f6a92c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718"
}
```

> **Design notes:** The signature is encoded as a lowercase hex string — 64 characters for a 32-byte HMAC-SHA256 output. Hex is used rather than base64 because hex strings are easier to visually inspect and compare, and SSH tooling commonly uses hex for fingerprints and challenge-response values. The signature is copied to clipboard by default; use `--no-clipboard` to suppress the copy. The `--challenge` flag enables non-interactive use in scripts. The JSON output includes `"algorithm"` so callers can verify the signing algorithm without reading the credential metadata separately.

### 3.2 `tegata get`

The get command retrieves a static password from the vault and copies it to the clipboard. The password is never printed to the terminal.

**Human-readable output:**

```
$ tegata get WiFi-office
Passphrase: ········

✓ Copied to clipboard (auto-clear in 45s)  (green)
```

**JSON output (`--json`):**

```json
{
  "status": "ok",
  "label": "WiFi-office",
  "type": "pw",
  "copied": true
}
```

> **Design notes:** The password value is intentionally absent from both the human-readable output and the JSON output. Printing a static password to the terminal exposes it to shoulder surfing and stores it permanently in terminal scrollback history. This is the strongest security guarantee Tegata can provide for static credentials: once added, the value leaves the vault only through the clipboard. The JSON output confirms the copy succeeded via `"copied": true` without revealing the value. The clipboard is automatically cleared after 45 seconds (configurable via `tegata.toml`).

### 3.3 `tegata export` and `tegata import`

The export command writes an encrypted backup of the vault to a file. The import command reads that backup and merges its credentials into the current vault. Both commands use interactive prompts for passphrase entry; the export file uses a separate passphrase from the vault passphrase.

**Export — human-readable output:**

```
$ tegata export backup-2026-03-14.tegata
Vault passphrase: ········
Export passphrase (for backup file): ········
Confirm export passphrase:           ········

✓ Exported 4 credentials to backup-2026-03-14.tegata  (green)
```

**Export — JSON output (`--json`):**

```json
{
  "status": "ok",
  "path": "backup-2026-03-14.tegata",
  "count": 4
}
```

**Import — human-readable output (with merge conflict):**

```
$ tegata import backup-2026-03-14.tegata
Export passphrase: ········

Credential 'GitHub' already exists. [s]kip / [o]verwrite / [r]ename? s
Credential 'AWS-prod' already exists. [s]kip / [o]verwrite / [r]ename? o

✓ Imported 3 credentials (1 skipped, 1 overwritten)  (green)
```

**Import — JSON output (`--json`):**

```json
{
  "status": "ok",
  "imported": 2,
  "skipped": 1,
  "overwritten": 1
}
```

> **Design notes:** The export file uses a separate passphrase from the vault passphrase so that a backup stored in an untrusted location does not compromise the vault key. Both files are AES-256-GCM encrypted but derive their keys from different passphrases. During import, each conflicting credential label is resolved individually — there is no global skip-all or overwrite-all option in the interactive flow (use `--skip-conflicts` or `--overwrite-conflicts` flags for scripting). The rename option prompts for a new label immediately after the user types `r`. Import counts in both human-readable and JSON output distinguish between freshly imported, skipped, and overwritten credentials for auditability.

### 3.4 `tegata resync`

The resync command resynchronizes the HOTP counter for a credential when the local counter has drifted from the server's expected counter. It requires two consecutive correct codes to confirm synchronization.

**Variant (a) — Successful resync:**

```
$ tegata resync AWS-prod
Passphrase: ········

Enter code 1: 483029
Enter code 2: 729401

✓ HOTP counter resynchronized for 'AWS-prod' (counter: 47)  (green)
```

**Variant (b) — Failed resync:**

```
$ tegata resync AWS-prod
Passphrase: ········

Enter code 1: 123456
Enter code 2: 789012

✗ Codes do not match any counter in the search window (counters 43–143). Verify the codes and try again.  (red)
```

[exit 1]

**JSON output (`--json`, success):**

```json
{
  "status": "ok",
  "label": "AWS-prod",
  "type": "hotp",
  "new_counter": 47
}
```

> **Design notes:** Resync searches a window of 100 counters ahead of the current stored counter (for example, counters 43–143 if the stored counter is 43). Two consecutive codes are required — not one — because a single matching code could be a coincidental collision within the search window. Requiring two sequential codes eliminates that risk and confirms the server and client are genuinely synchronized at the same counter position. The confirmed new counter value appears in both the human-readable output and the JSON response so the user can verify the expected counter. The failure message shows the exact search window so the user can assess whether more drift has occurred than the window covers.

### 3.5 `tegata history`

The history command retrieves recent audit events from the ScalarDL Ledger and displays them in a table. The `--around N` variant shows context surrounding a specific event number, useful when `tegata verify` reports an integrity violation.

**Variant (a) — Default (recent events):**

```
$ tegata history
Passphrase: ········

#     Timestamp             Label (hashed)    Type  Status  (cyan)
────  ────────────────────  ────────────────  ────  ──────  (cyan)
843   2026-03-14 07:42:11   a3f8c2d9…        totp  ok
844   2026-03-14 08:15:03   7b19e4f2…        cr    ok
845   2026-03-14 09:30:47   a3f8c2d9…        totp  ok
846   2026-03-14 11:02:18   c8d5e3a1…        hotp  ok
847   2026-03-14 12:45:59   a3f8c2d9…        totp  ok

847 events total
```

**Variant (b) — Context around a specific event (`--around 843`):**

```
$ tegata history --around 843
Passphrase: ········

#     Timestamp             Label (hashed)    Type  Status  (cyan)
────  ────────────────────  ────────────────  ────  ──────  (cyan)
840   2026-03-14 06:11:22   7b19e4f2…        cr    ok
841   2026-03-14 06:44:09   a3f8c2d9…        totp  ok
842   2026-03-14 07:15:33   c8d5e3a1…        hotp  ok
>>> 843   2026-03-14 07:42:11   a3f8c2d9…   totp  ok
844   2026-03-14 08:15:03   7b19e4f2…        cr    ok
845   2026-03-14 09:30:47   a3f8c2d9…        totp  ok

Showing 3 events before and after event #843
```

**JSON output (`--json`):**

```json
{
  "status": "ok",
  "total": 847,
  "events": [
    {
      "index": 843,
      "timestamp": "2026-03-14T07:42:11Z",
      "label_hash": "a3f8c2d9e1b7045f",
      "type": "totp",
      "status": "ok"
    },
    {
      "index": 844,
      "timestamp": "2026-03-14T08:15:03Z",
      "label_hash": "7b19e4f2c8d5e3a1",
      "type": "cr",
      "status": "ok"
    }
  ]
}
```

> **Design notes:** Labels are stored as hashed values in the audit log — the full credential name never appears on the ScalarDL Ledger. This protects privacy even if the ledger is accessed by a third party; the hash identifies the credential for correlation without revealing the label text. The `--around N` flag is the primary investigation tool when `tegata verify` (section 3.6) reports an integrity violation at a specific event: the user runs `tegata history --around N` to see the surrounding context. The `>>>` marker on the target event makes it visually unambiguous even when the terminal has no color. The `history` command requires a reachable ScalarDL Ledger instance — it cannot operate from the offline queue.

### 3.6 `tegata verify`

The verify command checks the full audit chain integrity by calling the ScalarDL Ledger's validate contract. It traverses all stored events and confirms that the hash chain is unbroken from event 1 to the latest event.

**Variant (a) — Chain valid:**

```
$ tegata verify

✓ Audit chain verified. 847 events, all hashes valid.  (green)
```

**Variant (b) — Integrity violation detected:**

```
$ tegata verify

✗ Audit chain integrity violation at event #843. Expected hash does not match stored hash. Run 'tegata history --around 843' for details.  (red)
```

[exit 5]

**JSON output (`--json`, success):**

```json
{
  "status": "ok",
  "events_verified": 847,
  "valid": true
}
```

**JSON output (`--json`, failure):**

```json
{
  "status": "error",
  "code": 5,
  "category": "integrity",
  "message": "Audit chain integrity violation at event #843. Expected hash does not match stored hash.",
  "failed_event": 843
}
```

> **Design notes:** Verification checks the complete chain from event 1 to the latest — partial verification is not supported in v0.3 because a partial check provides incomplete assurance. The command does not require a passphrase because it reads only from the ScalarDL Ledger (not the vault). The failure output is identical to section 4.5 (integrity violation error state), ensuring consistency: whichever entry point leads to the integrity violation, the user sees the same message and the same recovery step. The `"failed_event"` field in the JSON error response is separate from the message string, making it easy for scripts to extract the event index without parsing text.

### 3.7 `tegata ledger setup`

The ledger setup command configures the ScalarDL Ledger connection interactively. It prompts for the server address, TLS certificate paths, and certificate holder credentials, then validates the connection by registering the certificate and running a test contract call.

**Human-readable output:**

```
$ tegata ledger setup

ScalarDL Ledger configuration

Host [localhost]: localhost
Port [50051]:
TLS CA certificate path: certs/ca.pem
TLS client certificate path: certs/client.pem
TLS client key path: certs/client-key.pem
Certificate holder ID: my-tegata-user
Certificate version [1]:

Testing connection to localhost:50051...
✓ Connected to ScalarDL Ledger    (green)
Registering certificate...
✓ Certificate registered           (green)

✓ Ledger setup complete. Audit logging is now enabled.  (green)
```

**JSON output (`--json`):**

```json
{
  "status": "ok",
  "host": "localhost",
  "port": 50051,
  "tls": true,
  "cert_holder_id": "my-tegata-user",
  "cert_version": 1
}
```

> **Design notes:** Bracket notation on prompts (for example, `Port [50051]:`) indicates the default value — pressing Enter without typing accepts the default. The setup command writes all confirmed values to `tegata.toml` on the USB drive and sets `enabled = true` in the `[audit]` section. The `tegata ledger setup` command can be re-run to update any setting; subsequent runs overwrite the existing `[audit]` block in `tegata.toml`. The certificate registration step calls the ScalarDL `RegisterCertificate` RPC with the `CertHolderId` and `CertVersion` fields. If registration fails (for example, the certificate is already registered with a different version), the error is shown as a `✗` line with the specific reason from the gRPC status message.

### 3.8 `tegata config show`

The config show command displays the current resolved configuration — vault path, config file path, audit settings, and timeout values. It does not require a passphrase because no vault data is accessed.

**Human-readable output:**

```
$ tegata config show

vault_path:        /Volumes/TEGATA/vault.tegata    (cyan)
config_path:       /Volumes/TEGATA/tegata.toml     (cyan)
idle_timeout:      300s                            (cyan)
clipboard_timeout: 45s                             (cyan)
audit_enabled:     true                            (cyan)
ledger_host:       localhost:50051                 (cyan)
cert_holder_id:    my-tegata-user                  (cyan)
```

**JSON output (`--json`):**

```json
{
  "status": "ok",
  "vault_path": "/Volumes/TEGATA/vault.tegata",
  "config_path": "/Volumes/TEGATA/tegata.toml",
  "idle_timeout": 300,
  "clipboard_timeout": 45,
  "audit_enabled": true,
  "ledger_host": "localhost:50051",
  "cert_holder_id": "my-tegata-user"
}
```

> **Design notes:** Labels (the left column) are colored cyan as structural elements; values (the right column) are uncolored, consistent with the convention established in section 1.2. Sensitive values such as TLS certificate file paths are shown because they are file paths, not secrets. Certificate key material is never displayed — only the path to the key file is shown, and only the holder ID (not the certificate contents) appears. When audit is disabled, `ledger_host` and `cert_holder_id` are omitted from both the human-readable output and the JSON response to avoid showing empty or default values that would mislead the user into thinking audit is configured.

### 3.9 `tegata version`

The version command displays build information including the version number, Go runtime version, build date, commit hash, and target platform. It displays the same ASCII art logo as `tegata init`.

**Human-readable output:**

```
$ tegata version
 _                    _
| |_ ___  __ _  __ _| |_ __ _     (cyan)
| __/ _ \/ _` |/ _` | __/ _` |    (cyan)
 \__\___/\__, |\__,_|\__\__,_|    (cyan)
         |___/                     (cyan)

tegata v0.3.0 (go1.23.0, built 2026-06-15, commit abc1234, linux/amd64)
```

**JSON output (`--json`):**

```json
{
  "status": "ok",
  "version": "0.3.0",
  "go_version": "go1.23.0",
  "commit": "abc1234",
  "platform": "linux/amd64",
  "build_date": "2026-06-15"
}
```

> **Design notes:** The ASCII art logo is identical to the one in `tegata init` (section 2.1) — a single shared constant in the codebase renders both. Build information is embedded at compile time using Go's `debug/buildinfo` package or `-ldflags` injection, providing debugging context when users report issues. The `"commit"` field is the short (7-character) Git commit hash. In development builds where commit information is unavailable, `"commit"` is `"dev"`. The version command does not require a passphrase and does not access the vault or config file.

---

## 4. CLI mockups — error states

All five documented failure modes are specified here. Each subsection includes the human-readable output to stderr, the `--json` equivalent, and design notes.

### 4.1 Wrong passphrase (exit 2)

This error occurs when the provided passphrase does not match the vault's stored key derivation.

**Human-readable output:**

```
$ tegata code GitHub
Passphrase: ········

✗ Incorrect passphrase. 2 attempts remaining before rate-limiting.  (red)
```

[exit 2]

**After rate-limiting triggers:**

```
✗ Too many attempts. Try again in 30 seconds.  (red)
```

[exit 2]

**JSON output (`--json`):**

```json
{
  "status": "error",
  "code": 2,
  "category": "auth",
  "message": "Incorrect passphrase. 2 attempts remaining before rate-limiting."
}
```

> **Design notes:** The remaining-attempts count counts down from the configured limit (default: 3 attempts before rate-limiting begins). The rate-limit message shows the wait duration in seconds. After the wait expires, the prompt reappears automatically. In `--json` mode the rate-limit state is conveyed in the `"message"` field using the same text as the human-readable output.

### 4.2 Missing vault (exit 3)

This error occurs when no vault file can be found via auto-detection and no `--vault` path was specified.

**Human-readable output:**

```
$ tegata code GitHub

✗ No vault found. Run 'tegata init' to create one, or use '--vault <path>' to specify a location.  (red)
```

[exit 3]

**JSON output (`--json`):**

```json
{
  "status": "error",
  "code": 3,
  "category": "vault",
  "message": "No vault found. Run 'tegata init' to create one, or use '--vault <path>' to specify a location."
}
```

> **Design notes:** No passphrase prompt appears before this error — vault existence is checked before prompting for credentials. The error message provides two recovery paths: create a new vault or point to an existing one. This matches design doc section 5.3 vault auto-detection behavior.

### 4.3 Corrupted vault (exit 3)

This error occurs when the vault file exists but its AES-256-GCM authentication tag fails verification, indicating corruption or tampering.

**Variant (a) — Backup exists:**

```
$ tegata code GitHub
Passphrase: ········

✗ Vault file is corrupted (integrity check failed). A backup exists at vault.tegata.bak — run 'tegata import vault.tegata.bak' to restore.  (red)
```

[exit 3]

**Variant (b) — No backup exists:**

```
$ tegata code GitHub
Passphrase: ········

✗ Vault file is corrupted (integrity check failed). No backup found. If you have an exported backup, run 'tegata import <file>'.  (red)
```

[exit 3]

**JSON output (`--json`, backup exists):**

```json
{
  "status": "error",
  "code": 3,
  "category": "vault",
  "message": "Vault file is corrupted (integrity check failed). A backup exists at vault.tegata.bak — run 'tegata import vault.tegata.bak' to restore."
}
```

> **Design notes:** The passphrase is prompted first because corruption is only detectable after attempting decryption (the GCM tag is verified during decrypt). The two variants differ only in their recovery suggestion. The backup path `vault.tegata.bak` is deterministic — Tegata writes a `.bak` file on each successful write before overwriting the main file.

### 4.4 ScalarDL unreachable (exit 4)

This is a **warning**, not a blocking error. Authentication operations succeed normally; only the audit event is affected. The exit code remains 0 when the authentication itself succeeds.

**Human-readable output (warning, non-blocking):**

```
$ tegata code GitHub
Passphrase: ········

GitHub
482 913  [████████░░] 18s   (updates in place every second)

✓ Copied to clipboard (auto-clear in 45s)                    (green)

! Cannot reach ScalarDL Ledger at localhost:50051. Event queued locally (3 events pending).  (yellow)
```

[exit 0]

**JSON output (`--json`):**

```json
{
  "status": "ok",
  "code": "482913",
  "ttl": 18,
  "label": "GitHub",
  "type": "totp",
  "audit_warning": {
    "message": "Cannot reach ScalarDL Ledger at localhost:50051. Event queued locally (3 events pending).",
    "queued_events": 3
  }
}
```

> **Design notes:** The warning appears after the authentication output, not before it, so the code is visible immediately. The exit code is 0 because authentication succeeded — only the audit submission was deferred. In `--json` mode, the audit warning is nested under `"audit_warning"` to keep the top-level fields unambiguous: `"status": "ok"` remains accurate, and scripts can check `audit_warning` separately. When ScalarDL is not configured, this warning is never shown.

### 4.5 Integrity violation (exit 5)

This error occurs when `tegata verify` detects that the audit chain hash does not match the stored hash at a specific event index.

**Human-readable output:**

```
$ tegata verify

✗ Audit chain integrity violation at event #843. Expected hash does not match stored hash. Run 'tegata history --around 843' for details.  (red)
```

[exit 5]

**JSON output (`--json`):**

```json
{
  "status": "error",
  "code": 5,
  "category": "integrity",
  "message": "Audit chain integrity violation at event #843. Expected hash does not match stored hash.",
  "event_index": 843
}
```

> **Design notes:** The event index in the JSON response (`"event_index"`) is separate from the message string, making it easy for scripts to act on the specific index. The human-readable message includes the exact `tegata history` command to run next, reducing the steps required to investigate. This is the only Tegata error that directly implies a potential security incident rather than a user error or configuration issue.

---

## 5. TUI wireframes — first-time setup

The first-time setup wizard runs when `tegata init` is invoked and the TUI flag is active (or the binary is launched without arguments in GUI mode). All four steps render in a full-width panel — the sidebar is not shown during the wizard because no credentials exist yet. Each step uses a 4-character step indicator in the panel title.

The following TUI conventions apply across all sections in this document.

**Terminal minimum:** 80 columns wide. Below 80 columns, the TUI does not render the panel layout; instead it shows:

```
Terminal too narrow (minimum 80 columns)
```

**Sidebar width:** Fixed 28 characters minimum. Labels truncate with `…` at 20 characters. The type badge is right-aligned within the sidebar column.

**Main panel:** Fills remaining terminal width after the sidebar and divider.

**Help bar:** Always rendered at the bottom of the outer frame, showing context-sensitive keyboard shortcuts for the current state.

**Selection highlight:** The currently selected item is highlighted using reverse video (lipgloss `Reverse` style) — a full-row background swap that remains visible without relying on color.

All wireframes in this document assume an 80-column terminal.

### 5.1 Step 1 of 4 — Welcome

The welcome step displays the Tegata logo and tagline and prompts the user to begin setup or quit.

**Frame 1: Welcome screen**

```
┌─ Tegata setup ───────────────────────────────────────────────────────────────┐
│                                                                              │
│                          _                    _                              │
│                         | |_ ___  __ _  __ _| |_ __ _                       │
│                         | __/ _ \/ _` |/ _` | __/ _` |                      │
│                          \__\___/\__, |\__,_|\__\__,_|                      │
│                                  |___/                                       │
│                                                                              │
│                   Tegata — portable authenticator (v0.2.0)                  │
│                   Your authentication history. Integrity checked.           │
│                                                                              │
│                         Press Enter to begin setup.                         │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│  [Enter] Continue  [q] Quit                                                  │
└──────────────────────────────────────────────────────────────────────────────┘
```

**State:** `wizard_welcome`

**Transition:**

| Action  | Target state        |
|---------|---------------------|
| `Enter` | `wizard_passphrase` |
| `q`     | Exit (no vault)     |

### 5.2 Step 2 of 4 — Create passphrase

The passphrase step collects and confirms the vault passphrase. Two frames are shown: the initial state with both fields empty, and the state after the first field is filled.

**Frame 1: Initial state**

```
┌─ Tegata setup — Create passphrase (Step 2 of 4) ────────────────────────────┐
│                                                                              │
│  Create a passphrase to protect your vault. Choose something memorable      │
│  but difficult to guess. The passphrase is never stored — it is used only   │
│  to derive the encryption key.                                               │
│                                                                              │
│  Passphrase:   <passphrase input>                                            │
│  Confirm:      <confirm input>                                               │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│  [Enter] Continue  [Esc] Back                                                │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Frame 2: After passphrase entered, confirm field active**

```
┌─ Tegata setup — Create passphrase (Step 2 of 4) ────────────────────────────┐
│                                                                              │
│  Create a passphrase to protect your vault. Choose something memorable      │
│  but difficult to guess. The passphrase is never stored — it is used only   │
│  to derive the encryption key.                                               │
│                                                                              │
│  Passphrase:   ················                                              │
│  Confirm:      <confirm input, cursor here>                                  │
│                                                                              │
│  Strength: ████████░░  Strong                                                │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│  [Enter] Continue  [Esc] Back                                                │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Frame 3: Mismatch error after Enter pressed**

```
┌─ Tegata setup — Create passphrase (Step 2 of 4) ────────────────────────────┐
│                                                                              │
│  Create a passphrase to protect your vault. Choose something memorable      │
│  but difficult to guess. The passphrase is never stored — it is used only   │
│  to derive the encryption key.                                               │
│                                                                              │
│  Passphrase:   ················                                              │
│  Confirm:      ········                                                      │
│                                                                              │
│  Strength: ████████░░  Strong                                                │
│                                                                              │
│  ✗ Passphrases do not match                                      (red)       │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│  [Enter] Continue  [Esc] Back                                                │
└──────────────────────────────────────────────────────────────────────────────┘
```

> **Design notes:** The strength indicator (frame 2) is optional visual feedback computed from passphrase length and character variety — it does not enforce any minimum. The `✗` mismatch message (frame 3) is red when color is enabled; it remains clearly labeled by the `✗` prefix under `NO_COLOR`. The Confirm field is cleared after a mismatch so the user re-enters only the confirmation.

**State:** `wizard_passphrase`

**Transition:**

| Action                       | Target state          |
|------------------------------|-----------------------|
| Both fields match + `Enter`  | `wizard_recovery_key` |
| Fields mismatch + `Enter`    | Stay (show error)     |
| `Esc`                        | `wizard_welcome`      |

### 5.3 Step 3 of 4 — Recovery key

The recovery key step displays the generated recovery key and requires the user to confirm they have saved it before proceeding.

**Frame 1: Recovery key displayed**

```
┌─ Tegata setup — Recovery key (Step 3 of 4) ─────────────────────────────────┐
│                                                                              │
│  Your recovery key allows you to access your vault if you forget your       │
│  passphrase. Store it somewhere safe — offline, not on this device.         │
│                                                                              │
│  ────────────────────────────────────────────────────────────────────────── │
│                                                                              │
│    ABCD-EFGH-IJKL-MNOP-QRST-UVWX-YZAB-CDEF-GHIJ-KLMN-OPQR-STUV-WX        │
│                                                                              │
│  ────────────────────────────────────────────────────────────────────────── │
│                                                                              │
│  ! Store this key somewhere safe. It will not be shown again.   (yellow)    │
│                                                                              │
│                                                                              │
│  [ ] I have saved my recovery key                                            │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│  [Space] Check box  [Enter] Continue  [Esc] Back                             │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Frame 2: Checkbox checked, ready to continue**

```
┌─ Tegata setup — Recovery key (Step 3 of 4) ─────────────────────────────────┐
│                                                                              │
│  Your recovery key allows you to access your vault if you forget your       │
│  passphrase. Store it somewhere safe — offline, not on this device.         │
│                                                                              │
│  ────────────────────────────────────────────────────────────────────────── │
│                                                                              │
│    ABCD-EFGH-IJKL-MNOP-QRST-UVWX-YZAB-CDEF-GHIJ-KLMN-OPQR-STUV-WX        │
│                                                                              │
│  ────────────────────────────────────────────────────────────────────────── │
│                                                                              │
│  ! Store this key somewhere safe. It will not be shown again.   (yellow)    │
│                                                                              │
│                                                                              │
│  [x] I have saved my recovery key                                            │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│  [Space] Check box  [Enter] Continue  [Esc] Back                             │
└──────────────────────────────────────────────────────────────────────────────┘
```

> **Design notes:** The recovery key uses 4-character groups separated by hyphens (13 groups, 52 characters total from the base32 alphabet). The horizontal separator lines visually bracket the key without color, so it is clearly set apart even under `NO_COLOR`. The `Enter` key is disabled while the checkbox is unchecked — the user must positively affirm they have saved the key before proceeding. This is a one-time display; navigating back to this step does not re-show the key.

**State:** `wizard_recovery_key`

**Transition:**

| Action                       | Target state              |
|------------------------------|---------------------------|
| `Space`                      | Toggle checkbox           |
| Checkbox checked + `Enter`   | `wizard_add_credential`   |
| Checkbox unchecked + `Enter` | Stay (no transition)      |
| `Esc`                        | `wizard_passphrase`       |

### 5.4 Step 4 of 4 — Add first credential

The final wizard step adds an initial credential. It can be skipped — the user goes directly to the main view without any credentials if they press Escape.

**Frame 1: Empty form, TOTP selected**

```
┌─ Tegata setup — Add first credential (Step 4 of 4) ─────────────────────────┐
│                                                                              │
│  Add your first credential to the vault. You can add more later with        │
│  the [a] key in the main view.                                               │
│                                                                              │
│  Type:    [TOTP]  HOTP   CR   Static                                         │
│                                                                              │
│  Label:   <label input>                                                      │
│  Secret:  <secret input, masked>                                             │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│  [Tab] Switch type  [Enter] Add  [Esc] Skip                                  │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Frame 2: Form filled with a credential**

```
┌─ Tegata setup — Add first credential (Step 4 of 4) ─────────────────────────┐
│                                                                              │
│  Add your first credential to the vault. You can add more later with        │
│  the [a] key in the main view.                                               │
│                                                                              │
│  Type:    [TOTP]  HOTP   CR   Static                                         │
│                                                                              │
│  Label:   GitHub                                                             │
│  Secret:  ································                                   │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│  [Tab] Switch type  [Enter] Add  [Esc] Skip                                  │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Frame 3: Success state**

```
┌─ Tegata setup — Add first credential (Step 4 of 4) ─────────────────────────┐
│                                                                              │
│  Add your first credential to the vault. You can add more later with        │
│  the [a] key in the main view.                                               │
│                                                                              │
│  Type:    [TOTP]  HOTP   CR   Static                                         │
│                                                                              │
│  Label:   GitHub                                                             │
│  Secret:  ································                                   │
│                                                                              │
│  ✓ Credential 'GitHub' added.                                    (green)     │
│                                                                              │
│  Press Enter to finish setup.                                                │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│  [Enter] Finish                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

> **Design notes:** The type selector uses bracket notation to show which type is currently active: `[TOTP]` is highlighted (reverse video), while the others are uncolored plain text. Pressing Tab cycles through `TOTP → HOTP → CR → Static → TOTP`. The selected type determines which field label appears as the third input — for `CR` (challenge-response) the field reads `Shared key:` instead of `Secret:`. Static password types show `Password:`. The form fields are standard masked inputs; the secret is never shown in plaintext.

**State:** `wizard_add_credential`

**Transition:**

| Action                       | Target state                   |
|------------------------------|--------------------------------|
| `Tab`                        | Cycle credential type          |
| `Enter` (form filled)        | `wizard_success` → `main_view` |
| `Enter` (after success msg)  | `main_view`                    |
| `Esc`                        | `main_view` (skip adding)      |

---

## 6. TUI wireframes — daily use

Daily use covers the three most common interactions: unlocking the vault, generating authentication codes, and returning to the unlock screen after idle timeout. The sidebar + main panel layout is introduced in section 6.2 and applies to all subsequent sections.

### 6.1 Unlock vault

The unlock screen appears at startup when a vault exists and has not yet been unlocked. It uses a full-width centered panel — no sidebar is displayed until the vault is open.

**Frame 1: Passphrase prompt**

```
┌─ Tegata ─────────────────────────────────────────────────────────────────────┐
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                    Vault: /Volumes/TEGATA/vault.tegata                       │
│                                                                              │
│                    Passphrase: <passphrase input>                            │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│  [Enter] Unlock  [q] Quit                                                    │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Frame 2: Incorrect passphrase error**

```
┌─ Tegata ─────────────────────────────────────────────────────────────────────┐
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                    Vault: /Volumes/TEGATA/vault.tegata                       │
│                                                                              │
│                    Passphrase: <passphrase input, cleared>                   │
│                                                                              │
│                    ✗ Incorrect passphrase. 2 attempts remaining. (red)       │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│  [Enter] Unlock  [q] Quit                                                    │
└──────────────────────────────────────────────────────────────────────────────┘
```

> **Design notes:** The vault path is shown before the passphrase prompt so the user can confirm they are unlocking the correct vault — relevant when multiple vaults exist across different drives. The passphrase field is cleared after an incorrect attempt; only the input clears, not the vault path label. Rate-limiting follows the same pattern as the CLI: after the configured limit, the prompt displays a wait duration and re-enables after the cooldown expires.

**State:** `unlock`

**Transition:**

| Action                 | Target state        |
|------------------------|---------------------|
| Correct passphrase + `Enter` | `main_view`   |
| Wrong passphrase + `Enter`   | Stay (show error, clear input) |
| `q`                    | Exit                |

### 6.2 Main view with code generation

The main view uses the sidebar + main panel layout. The left sidebar lists all credentials with their type badges. The right main panel shows the selected credential's output. The selected credential is highlighted in reverse video.

**Frame 1: TOTP credential selected, code copied**

```
┌─ Tegata ─────────────────────────────────────────────────────────────────────┐
│ ┌── Credentials (3) ──────────┐ ┌── GitHub ──────────────────────────────── │
│ │ > GitHub             totp   │ │                                            │
│ │   AWS-prod           hotp   │ │   482 913  [████████░░] 18s               │
│ │   WiFi-office        pw     │ │                                            │
│ │                             │ │   ✓ Copied to clipboard        (green)     │
│ │                             │ │     (auto-clear in 45s)                    │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ └─────────────────────────────┘ └──────────────────────────────────────────── │
│  [j/k] Navigate  [Enter] Copy  [a] Add  [r] Remove  [q] Quit                │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Frame 2: HOTP credential selected, code copied**

```
┌─ Tegata ─────────────────────────────────────────────────────────────────────┐
│ ┌── Credentials (3) ──────────┐ ┌── AWS-prod ─────────────────────────────── │
│ │   GitHub             totp   │ │                                            │
│ │ > AWS-prod           hotp   │ │   483 029  (counter: 43)                  │
│ │   WiFi-office        pw     │ │                                            │
│ │                             │ │   ✓ Copied to clipboard        (green)     │
│ │                             │ │     (auto-clear in 45s)                    │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ └─────────────────────────────┘ └──────────────────────────────────────────── │
│  [j/k] Navigate  [Enter] Copy  [a] Add  [r] Remove  [q] Quit                │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Frame 3: Static password selected, clipboard confirmed**

```
┌─ Tegata ─────────────────────────────────────────────────────────────────────┐
│ ┌── Credentials (3) ──────────┐ ┌── WiFi-office ──────────────────────────── │
│ │   GitHub             totp   │ │                                            │
│ │   AWS-prod           hotp   │ │   ✓ Copied to clipboard        (green)     │
│ │ > WiFi-office        pw     │ │     (auto-clear in 45s)                    │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ └─────────────────────────────┘ └──────────────────────────────────────────── │
│  [j/k] Navigate  [Enter] Copy  [a] Add  [r] Remove  [q] Quit                │
└──────────────────────────────────────────────────────────────────────────────┘
```

> **Design notes:** The sidebar is fixed at 30 characters wide (28 content + 2 border characters), and labels truncate with `…` at 20 characters. The type badge is right-aligned at the far right of each sidebar row. The `>` marker on the selected row is supplemented by reverse video highlighting — the `>` ensures the selection is unambiguous under `NO_COLOR`. The TOTP countdown bar updates in place every second without redrawing the full frame (bubbletea's tick-based model). HOTP shows the counter value instead of a countdown bar because HOTP has no time window. Static password output matches the CLI `tegata get` behavior — no password is ever rendered in the panel.

**State:** `main_view` with sub-states `main_totp_active`, `main_hotp_active`, `main_static_active`

**Transition:**

| Action   | Target state / effect                  |
|----------|----------------------------------------|
| `j`      | Select next credential in sidebar      |
| `k`      | Select previous credential in sidebar  |
| `Enter`  | Copy code / password to clipboard      |
| `a`      | `overlay_add_credential`               |
| `r`      | `overlay_remove_confirm`               |
| `q`      | Exit TUI                               |

### 6.3 Auto-lock

After 5 minutes of idle time (no keystrokes), the vault locks automatically. The main view is replaced by the unlock screen with an additional idle-timeout notice above the passphrase prompt.

**Frame 1: Auto-locked state**

```
┌─ Tegata ─────────────────────────────────────────────────────────────────────┐
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                    ! Vault locked (idle timeout)              (yellow)       │
│                                                                              │
│                    Vault: /Volumes/TEGATA/vault.tegata                       │
│                                                                              │
│                    Passphrase: <passphrase input>                            │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│  [Enter] Unlock  [q] Quit                                                    │
└──────────────────────────────────────────────────────────────────────────────┘
```

> **Design notes:** The auto-lock screen is identical to the initial unlock screen (section 6.1) with one addition: the yellow `! Vault locked (idle timeout)` notice above the vault path. This notice is yellow to signal a non-error condition — the lock was intentional and protective, not a failure. After unlocking, the main view returns to the same selection position the user was at before the timeout. The clipboard is also cleared on auto-lock if the 45-second auto-clear has not already fired.

**State:** `locked_idle`

**Transition:**

| Action                       | Target state                   |
|------------------------------|--------------------------------|
| Correct passphrase + `Enter` | `main_view` (prior position)   |
| Wrong passphrase + `Enter`   | Stay (show error, clear input) |
| `q`                          | Exit                           |

---

## 7. TUI wireframes — credential management

Credential management covers the three flows a user performs after the initial setup: viewing the full credential list, adding a new credential, and removing an existing credential. Add and remove operations use overlay panels (modals) that appear over the main view rather than replacing it.

### 7.1 Credential list view

The credential list is the sidebar component of the main view. This section documents it independently for the credential management context, specifically covering the scrolling behavior when the list exceeds the visible sidebar area.

**Frame 1: Sidebar with many credentials (scrolling active)**

```
┌─ Tegata ─────────────────────────────────────────────────────────────────────┐
│ ┌── Credentials (8) ──────────┐ ┌── GitHub ──────────────────────────────── │
│ │ > GitHub             totp   │ │                                            │
│ │   AWS-prod           hotp   │ │   482 913  [████████░░] 18s               │
│ │   SSH-signing        cr     │ │                                            │
│ │   WiFi-office        pw     │ │   ✓ Copied to clipboard        (green)     │
│ │   Work-VPN           totp   │ │     (auto-clear in 45s)                    │
│ │   Bitwarden          totp   │ │                                            │
│ │   ▼ 2 more                  │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ └─────────────────────────────┘ └──────────────────────────────────────────── │
│  [j/k] Navigate  [Enter] Copy  [a] Add  [r] Remove  [q] Quit                │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Frame 2: Scrolled to bottom, last credentials visible**

```
┌─ Tegata ─────────────────────────────────────────────────────────────────────┐
│ ┌── Credentials (8) ──────────┐ ┌── DB-backup ────────────────────────────── │
│ │   Bitwarden          totp   │ │                                            │
│ │   ▲ 6 more above            │ │   ✓ Copied to clipboard        (green)     │
│ │ > DB-backup          pw     │ │     (auto-clear in 45s)                    │
│ │   Corp-LDAP          cr     │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ └─────────────────────────────┘ └──────────────────────────────────────────── │
│  [j/k] Navigate  [Enter] Copy  [a] Add  [r] Remove  [q] Quit                │
└──────────────────────────────────────────────────────────────────────────────┘
```

> **Design notes:** The scroll indicator (`▼ 2 more` / `▲ 6 more above`) appears in the last visible row of the sidebar list area when additional items exist below or above the viewport. The indicator uses plain characters so it is readable without color. The sidebar scrolls independently of the main panel — the main panel always shows the currently selected credential regardless of scroll position. Selection wraps around: pressing `k` on the first credential moves to the last, and pressing `j` on the last credential moves to the first.

**State:** `main_view` (credential list scrolled)

**Transition:**

| Action  | Target state / effect                          |
|---------|------------------------------------------------|
| `j`     | Move selection down (scroll sidebar if needed) |
| `k`     | Move selection up (scroll sidebar if needed)   |
| `Enter` | Copy code / password for selected credential   |

### 7.2 Add credential

Adding a credential opens an overlay panel centered over the main view. The overlay uses the same form as section 5.4 (wizard step 4) so the experience is consistent whether adding a credential during setup or after.

**Frame 1: Add overlay, form empty**

```
┌─ Tegata ─────────────────────────────────────────────────────────────────────┐
│ ┌── Credentials (3) ──────────┐ ┌── GitHub ──────────────────────────────── │
│ │ ┌─ Add credential ────────────────────────────────┐                       │
│ │ │                                                  │                       │
│ │ │  Type:    [TOTP]  HOTP   CR   Static             │                       │
│ │ │                                                  │                       │
│ │ │  Label:   <label input>                          │                       │
│ │ │  Secret:  <secret input, masked>                 │                       │
│ │ │                                                  │                       │
│ │ │                                                  │                       │
│ │ │                                                  │                       │
│ │ │                                                  │                       │
│ │ │  [Tab] Switch type  [Enter] Add  [Esc] Cancel    │                       │
│ │ └──────────────────────────────────────────────────┘                       │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ └─────────────────────────────┘ └──────────────────────────────────────────── │
│  [j/k] Navigate  [Enter] Copy  [a] Add  [r] Remove  [q] Quit                │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Frame 2: Credential added successfully**

```
┌─ Tegata ─────────────────────────────────────────────────────────────────────┐
│ ┌── Credentials (4) ──────────┐ ┌── NewService ───────────────────────────── │
│ │   GitHub             totp   │ │                                            │
│ │   AWS-prod           hotp   │ │   ✓ Credential 'NewService' added.(green)  │
│ │   WiFi-office        pw     │ │                                            │
│ │ > NewService         totp   │ │   Press Enter to generate a code.         │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ └─────────────────────────────┘ └──────────────────────────────────────────── │
│  [j/k] Navigate  [Enter] Copy  [a] Add  [r] Remove  [q] Quit                │
└──────────────────────────────────────────────────────────────────────────────┘
```

> **Design notes:** The overlay panel is drawn using the same box-drawing characters as the outer frame — it visually floats over the main view. The background main view content remains partially visible around the overlay edges, which helps the user maintain spatial orientation. After a credential is successfully added, the overlay closes and the new credential is immediately selected in the sidebar and shown in the main panel. The success message in the main panel is temporary; pressing Enter or moving focus clears it and shows the normal code generation view.

**State:** `overlay_add_credential`

**Transition:**

| Action                | Target state                              |
|-----------------------|-------------------------------------------|
| `Tab`                 | Cycle credential type in overlay          |
| `Enter` (form filled) | Close overlay → `main_view` (new selected)|
| `Esc`                 | Close overlay → `main_view` (unchanged)   |

### 7.3 Remove credential (confirmation)

Removing a credential requires explicit confirmation via a small confirmation dialog overlaying the main view. The default answer is no — the user must type `y` to confirm.

**Frame 1: Confirmation dialog**

```
┌─ Tegata ─────────────────────────────────────────────────────────────────────┐
│ ┌── Credentials (3) ──────────┐ ┌── GitHub ──────────────────────────────── │
│ │ > GitHub             totp   │ │                                            │
│ │   AWS-prod           hotp   │ │                                            │
│ │   WiFi-office        pw     │ │  ┌─ Remove credential? ──────────────┐    │
│ │                             │ │  │                                    │    │
│ │                             │ │  │  Remove credential 'GitHub'?       │    │
│ │                             │ │  │  This cannot be undone.            │    │
│ │                             │ │  │                                    │    │
│ │                             │ │  │  [y] Yes, remove  [n] Cancel       │    │
│ │                             │ │  │                                    │    │
│ │                             │ │  └────────────────────────────────────┘    │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ └─────────────────────────────┘ └──────────────────────────────────────────── │
│  [j/k] Navigate  [Enter] Copy  [a] Add  [r] Remove  [q] Quit                │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Frame 2: Post-removal state**

```
┌─ Tegata ─────────────────────────────────────────────────────────────────────┐
│ ┌── Credentials (2) ──────────┐ ┌── AWS-prod ─────────────────────────────── │
│ │ > AWS-prod           hotp   │ │                                            │
│ │   WiFi-office        pw     │ │   ✓ Credential 'GitHub' removed.(green)    │
│ │                             │ │                                            │
│ │                             │ │   Press Enter to generate a code.         │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ │                             │ │                                            │
│ └─────────────────────────────┘ └──────────────────────────────────────────── │
│  [j/k] Navigate  [Enter] Copy  [a] Add  [r] Remove  [q] Quit                │
└──────────────────────────────────────────────────────────────────────────────┘
```

> **Design notes:** The confirmation dialog is a small overlay within the main panel area, not a full-screen modal. The credential name appears in the dialog to confirm which credential will be deleted — this prevents accidental deletion when the user pressed `r` without looking at the sidebar selection. The default action is cancel (`n`), matching the CLI `tegata remove` behavior where `[y/N]` signals the default is no. After removal, the sidebar updates immediately (the count drops by one) and the selection moves to the next available credential. If the last credential is removed, the main panel shows the empty-vault prompt.

**State:** `overlay_remove_confirm`

**Transition:**

| Action  | Target state                                     |
|---------|--------------------------------------------------|
| `y`     | Remove credential → `main_view` (next selected)  |
| `n`     | Close dialog → `main_view` (unchanged)           |
| `Esc`   | Close dialog → `main_view` (unchanged)           |
