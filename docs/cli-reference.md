# CLI command reference

This document describes every Tegata CLI command with its flags, defaults, and usage examples.

## Vault resolution

All commands that access the vault use the following resolution order to locate the vault file.

1. `--vault` flag (directory or file path)
2. `TEGATA_VAULT` environment variable (directory or file path)
3. `./vault.tegata` in the current working directory

If a directory is given, Tegata appends `vault.tegata` to the path automatically.

## Global flags

These flags are available on all commands.

| Flag        | Type   | Default | Description                          |
|-------------|--------|---------|--------------------------------------|
| `--vault`   | string | (none)  | Path to vault file or directory      |
| `--verbose` | bool   | false   | Enable debug logging (stderr output) |

## Commands

Commands are listed in alphabetical order.

### tegata add

Add a credential to the vault. Supports TOTP, HOTP, static password, and challenge-response credential types.

**Usage:** `tegata add <label> [flags]`

**Flags:**

| Flag          | Type   | Default | Description                                                    |
|---------------|--------|---------|----------------------------------------------------------------|
| `--scan`      | bool   | false   | Paste an otpauth:// URI instead of entering fields manually    |
| `--type`      | string | totp    | Credential type (totp, hotp, static, challenge-response)       |
| `--issuer`    | string | (none)  | Credential issuer name                                         |
| `--algorithm` | string | SHA1    | HMAC algorithm (SHA1, SHA256, SHA512)                          |
| `--digits`    | int    | 6       | Number of digits in generated code (1-10)                      |
| `--period`    | int    | 30      | TOTP period in seconds                                         |
| `--tag`       | string | (none)  | Tag to apply (repeatable, e.g. `--tag work --tag totp`)        |

**Examples:**

```bash
# Add a TOTP credential with manual secret entry
tegata add GitHub --type totp --issuer GitHub

# Add from an otpauth:// URI
tegata add GitHub --scan

# Add with tags
tegata add GitHub --type totp --issuer GitHub --tag work --tag totp
```

The secret is always prompted interactively (hidden input). When using `--scan`, Tegata parses the type, issuer, algorithm, digits, and period from the URI automatically.

### tegata bench

Benchmark Argon2id key derivation performance on the current machine. Runs 3 iterations with the default parameters (time=3, memory=64 MiB, parallelism=4) and reports the average unlock time.

**Usage:** `tegata bench`

**Example:**

```bash
tegata bench
```

Output includes per-run timings and whether the result is within the 3-second target.

### tegata change-passphrase

Rotate the vault passphrase without re-encrypting the credential payload. Only the passphrase-wrapped data encryption key and header salt are replaced.

**Usage:** `tegata change-passphrase`

**Example:**

```bash
tegata change-passphrase
```

Tegata prompts for the current passphrase, then prompts for and confirms the new passphrase.

### tegata code

Generate a TOTP or HOTP code for a credential. The code is displayed in the terminal and optionally copied to the clipboard.

**Usage:** `tegata code <label> [flags]`

**Flags:**

| Flag     | Type | Default | Description                        |
|----------|------|---------|------------------------------------|
| `--clip` | bool | true    | Copy code to clipboard             |
| `--show` | bool | true    | Display code in terminal           |

**Examples:**

```bash
# Generate a TOTP code (displays and copies to clipboard)
tegata code GitHub

# Generate without clipboard copy
tegata code GitHub --clip=false
```

For TOTP credentials, the output includes the remaining seconds until expiry. For HOTP credentials, the counter is incremented and saved before displaying the code.

### tegata config show

Display the effective configuration, including values from `tegata.toml` or defaults.

**Usage:** `tegata config show`

**Example:**

```bash
tegata config show
```

### tegata export

Export all credentials to an encrypted `.tegata-backup` file. The backup is protected by a separate export passphrase that you choose.

**Usage:** `tegata export [flags]`

**Flags:**

| Flag    | Type   | Default                                         | Description                    |
|---------|--------|-------------------------------------------------|--------------------------------|
| `--out` | string | `vault.tegata-backup` in the vault directory     | Output path for the backup file |

**Example:**

```bash
tegata export --out ~/backups/vault.tegata-backup
```

Tegata prompts for the vault passphrase, then prompts for and confirms a new export passphrase (minimum 8 characters). The export passphrase is independent of the vault passphrase.

### tegata get

Retrieve a static password credential. The password is copied to the clipboard by default and optionally displayed in the terminal.

**Usage:** `tegata get <label> [flags]`

**Flags:**

| Flag     | Type | Default | Description                   |
|----------|------|---------|-------------------------------|
| `--show` | bool | false   | Display password in terminal  |

**Examples:**

```bash
# Retrieve and copy to clipboard (default)
tegata get backup-key

# Also display in terminal
tegata get backup-key --show
```

### tegata history

View authentication event history from the ScalarDL Ledger. Events are retrieved from the entity's audit collection and displayed with metadata columns. Requires audit to be enabled in `tegata.toml`.

**Usage:** `tegata history [flags]`

**Flags:**

| Flag     | Type   | Default | Description                         |
|----------|--------|---------|-------------------------------------|
| `--from` | string | (none)  | Start date filter (YYYY-MM-DD)      |
| `--to`   | string | (none)  | End date filter (YYYY-MM-DD)        |
| `--json` | bool   | false   | Output as JSON array                |

The default table output shows four columns: Operation (totp, hotp, cr, static), Label (first 12 characters of the label hash), Timestamp (UTC), and Hash (truncated event hash). With `--json`, each record includes `object_id`, `operation`, `label_hash`, `timestamp` (Unix seconds), and `hash_value`.

**Examples:**

```bash
# View all history
tegata history

# Filter by date range
tegata history --from 2026-01-01 --to 2026-03-31

# JSON output for scripting
tegata history --json
```

### tegata import

Import credentials from an encrypted `.tegata-backup` file into the current vault. Credentials whose label already exists in the vault are skipped.

**Usage:** `tegata import <backup-file>`

**Example:**

```bash
tegata import ~/backups/vault.tegata-backup
```

Tegata prompts for the vault passphrase and then the backup passphrase (the one set during export). For scripted restore flows, the backup passphrase can be set via the `TEGATA_BACKUP_PASSPHRASE` environment variable.

### tegata init

Create a new encrypted vault. If a path argument is provided, it is used as the vault directory. Otherwise the current directory is used.

**Usage:** `tegata init [path]`

**Examples:**

```bash
# Initialize vault on a USB drive
tegata init /mnt/usb

# Initialize in current directory
tegata init
```

Tegata prompts for and confirms a passphrase (minimum 8 characters), creates the vault file, writes a default `tegata.toml` configuration, and displays a recovery key. Store the recovery key in a separate secure location.

### tegata ledger setup

Register the client TLS certificate with ScalarDL and verify connectivity. This command must be run once before audit logging is active.

**Usage:** `tegata ledger setup`

**Example:**

```bash
tegata ledger setup
```

The command reads the `[audit]` section from `tegata.toml` and uses the configured certificate paths and server address. See [ScalarDL setup guide](scalardl-setup.md) for configuration steps.

### tegata list

List all credentials in the vault. By default, credentials are grouped by tag. Untagged credentials appear under `[untagged]`.

**Usage:** `tegata list [flags]`

**Flags:**

| Flag    | Type   | Default | Description                                        |
|---------|--------|---------|----------------------------------------------------|
| `--tag` | string | (none)  | Filter credentials by tag (case-sensitive exact match) |

**Examples:**

```bash
# List all credentials (grouped by tag)
tegata list

# Filter by tag
tegata list --tag work
```

### tegata remove

Remove a credential from the vault. Tegata prompts for confirmation before removing.

**Usage:** `tegata remove <label>`

**Example:**

```bash
tegata remove old-service
```

### tegata resync

Resynchronize an HOTP counter by providing two consecutive codes from the server or authenticator app. Tegata scans a look-ahead window of 100 counters to find the matching position.

**Usage:** `tegata resync <label>`

**Example:**

```bash
tegata resync my-hotp-service
```

Tegata prompts for two consecutive codes interactively.

### tegata sign

Sign a challenge string with a stored HMAC secret (challenge-response credential). The algorithm is determined by the credential's algorithm field: SHA256 produces HMAC-SHA256, anything else defaults to HMAC-SHA1.

**Usage:** `tegata sign <label> [flags]`

**Flags:**

| Flag          | Type   | Default | Description                                                |
|---------------|--------|---------|------------------------------------------------------------|
| `--challenge` | string | (none)  | Challenge string to sign (required)                        |
| `--clip`      | bool   | false   | Copy signed response to clipboard instead of printing      |

**Examples:**

```bash
# Sign and print to stdout
tegata sign github --challenge abc123

# Sign and copy to clipboard
tegata sign github --challenge abc123 --clip
```

### tegata tag

Add or remove tags on an existing credential.

**Usage:** `tegata tag <label> [flags]`

**Flags:**

| Flag       | Type   | Default | Description                    |
|------------|--------|---------|--------------------------------|
| `--add`    | string | (none)  | Tag to add (repeatable)        |
| `--remove` | string | (none)  | Tag to remove (repeatable)     |

**Examples:**

```bash
# Add tags
tegata tag github --add work --add totp

# Remove a tag
tegata tag github --remove personal
```

At least one of `--add` or `--remove` must be provided.

### tegata ui

Launch the interactive terminal user interface. If no vault is found, the TUI starts a setup wizard for vault creation.

**Usage:** `tegata ui`

**Example:**

```bash
tegata ui
```

### tegata verify

Verify the integrity of the audit log stored in ScalarDL Ledger. Retrieves all event IDs from the entity's collection and validates each event individually. Reports the total number of events checked and lists per-event faults if any are detected.

**Usage:** `tegata verify`

**Example:**

```bash
tegata verify
```

Exit codes: 0 on success, 9 on integrity violation, 8 on network failure. Requires audit to be enabled in `tegata.toml`.

### tegata verify-recovery

Verify a recovery key against the vault by checking its SHA-256 hash against the value stored at vault creation time.

**Usage:** `tegata verify-recovery`

**Example:**

```bash
tegata verify-recovery
```

Tegata prompts for the vault passphrase and then the recovery key string.

### tegata version

Print version information.

**Usage:** `tegata version`

**Example:**

```bash
tegata version
```

## Environment variables

Tegata recognizes the following environment variables.

| Variable                   | Description                                                                      |
|----------------------------|----------------------------------------------------------------------------------|
| `TEGATA_VAULT`             | Default vault path (used when `--vault` flag is not provided)                    |
| `TEGATA_PASSPHRASE`        | Vault passphrase for non-interactive use (warning printed to stderr)             |
| `TEGATA_BACKUP_PASSPHRASE` | Backup passphrase for scripted import (independent of vault passphrase)          |

The `TEGATA_PASSPHRASE` variable is primarily for CI/scripting. A warning is printed to stderr when it is used. The `tegata export` command never reads from this variable -- an interactive prompt is always required for setting a new backup passphrase.
