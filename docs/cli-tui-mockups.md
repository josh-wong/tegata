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
- [4. CLI mockups — error states](#4-cli-mockups--error-states)
  - [4.1 Wrong passphrase (exit 2)](#41-wrong-passphrase-exit-2)
  - [4.2 Missing vault (exit 3)](#42-missing-vault-exit-3)
  - [4.3 Corrupted vault (exit 3)](#43-corrupted-vault-exit-3)
  - [4.4 ScalarDL unreachable (exit 4)](#44-scalardl-unreachable-exit-4)
  - [4.5 Integrity violation (exit 5)](#45-integrity-violation-exit-5)
- [5. TUI wireframes — first-time setup](#5-tui-wireframes--first-time-setup)
- [6. TUI wireframes — daily use](#6-tui-wireframes--daily-use)
- [7. TUI wireframes — credential management](#7-tui-wireframes--credential-management)

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

Content for this section will be added in a subsequent plan. Subsection headings are listed here to establish the document structure.

### 3.1 `tegata sign`

Content will be added in a subsequent plan.

### 3.2 `tegata get`

Content will be added in a subsequent plan.

### 3.3 `tegata export` and `tegata import`

Content will be added in a subsequent plan.

### 3.4 `tegata resync`

Content will be added in a subsequent plan.

### 3.5 `tegata history`

Content will be added in a subsequent plan.

### 3.6 `tegata verify`

Content will be added in a subsequent plan.

### 3.7 `tegata ledger setup`

Content will be added in a subsequent plan.

### 3.8 `tegata config show`

Content will be added in a subsequent plan.

### 3.9 `tegata version`

Content will be added in a subsequent plan.

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

Content will be added in a subsequent plan.

---

## 6. TUI wireframes — daily use

Content will be added in a subsequent plan.

---

## 7. TUI wireframes — credential management

Content will be added in a subsequent plan.
