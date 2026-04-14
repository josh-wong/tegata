# ScalarDL audit setup and testing

This guide explains how to set up and test Tegata's optional tamper-evident audit layer backed by ScalarDL Ledger 3.13. The audit layer records every authentication event in a hash-chained ledger, enabling post-hoc integrity verification with `tegata verify`.

Audit logging is disabled by default. All authentication operations remain fully functional without it.

## Prerequisites

Setting up the audit layer requires the following tools.

- Docker Desktop (Windows, macOS) or Docker Engine 24+ (Linux) with Compose v2
- Go 1.25+ to build Tegata from source
- A Tegata vault initialized with `tegata init`

No certificates or key pairs are needed. Tegata uses ScalarDL's HMAC authentication mode, which requires only a shared secret key that is generated automatically during setup.

## One-click setup

Run `tegata ledger start` once after initializing your vault. Tegata handles everything automatically.

```bash
tegata ledger start --vault /path/to/vault
```

Tegata prompts for your vault passphrase, then runs the full setup sequence.

1. Checks that Docker is installed and starts the Docker daemon if it is not running
2. Extracts the bundled `docker-compose.yml` to `~/.tegata/docker/`
3. Generates a unique entity ID and secret key from your vault
4. Starts the Docker stack (`docker compose up -d`)
5. Waits for the ledger to become ready (up to 30 seconds)
6. Waits for the HashStore contracts to be registered (up to 5 minutes on first run — the JVM and ~50 MB SDK download are the slow parts)
7. Registers your audit credentials with the ledger
8. Writes the `[audit]` section to `tegata.toml`

When setup completes, you should see:

```
Ledger server started. Audit logging is now active.
```

Audit logging is now active. From this point forward, every vault unlock automatically starts the Docker stack — including starting the Docker daemon if it is not running — in the background, with no action needed from you.

## Upgrading ScalarDL

When upgrading to a new ScalarDL version, the bundled Docker Compose files and SHA256 hash for the HashStore SDK must be updated together. The process is straightforward.

1. Identify the new ScalarDL version (from [Scalar Inc. releases](https://github.com/scalar-labs/scalardl/releases))
2. In all three Docker Compose files (`cmd/tegata/docker-bundle/docker-compose.yml`, `cmd/tegata-gui/docker-bundle/docker-compose.yml`, `deployments/docker-compose/docker-compose.yml`), update:
   - The `scalardl-ledger` image tag
   - The `scalardl-schema-loader` image tag
   - The HashStore SDK download URL and unzip path
3. Download the new SDK zip and compute its SHA256 hash:
   ```bash
   curl -fsSL -O https://github.com/scalar-labs/scalardl/releases/download/vX.Y.Z/scalardl-hashstore-java-client-sdk-X.Y.Z.zip
   shasum -a 256 scalardl-hashstore-java-client-sdk-X.Y.Z.zip
   ```
4. Update the SHA256 hash in all three Docker Compose files (in the verification line immediately after the curl download)
5. Update source comment URLs in `internal/audit/rpc/scalar.proto`, `scalar.pb.go`, and `scalar_grpc.pb.go` to reference the new version

The SHA256 hash is verified during Docker container startup and will fail fast if the downloaded artifact does not match, protecting against supply chain tampering or accidental corruption.

## Auto-start behavior

After setup, every vault unlock fires `MaybeAutoStart` in the background.

- If Docker Desktop is closed, Tegata starts it and waits for it to become ready (up to 60 seconds)
- Once the Docker daemon is running, Tegata starts the ScalarDL containers (`docker compose up -d`)
- The vault unlocks immediately — auto-start runs asynchronously and never blocks the unlock

If auto-start fails for any reason, Tegata logs the error to stderr and queues audit events locally. Queued events are submitted when the ledger becomes reachable.

To disable auto-start without removing the audit configuration, run:

```bash
tegata config set audit.auto_start false
```

To re-enable it:

```bash
tegata config set audit.auto_start true
```

## Testing audit in the CLI

With the vault unlocked and the ledger running, generate a TOTP code to trigger an audit event.

```bash
tegata code my-totp-credential --vault /path/to/vault
```

Check that the event was recorded.

```bash
tegata history --vault /path/to/vault
```

Verify hash-chain integrity.

```bash
tegata verify --vault /path/to/vault
```

Expected output when the audit log is intact:

```
Audit log integrity verified. 3 events checked.
```

This command exits with code 9 if an integrity violation is detected, or code 8 if the ledger is unreachable.

## Testing audit in the TUI

Launch the TUI and unlock your vault.

```bash
tegata ui --vault /path/to/vault
```

Press `v` to open the audit overlay. Use the arrow keys or `j`/`k` to select an option and press Enter.

- **View history** fetches and displays all audit events from the ledger.
- **Verify integrity** validates the hash chain and reports whether the log is intact or tampered.

Press Esc to return to the menu or close the overlay.

## Testing audit in the GUI

Build and run the desktop GUI.

```bash
cd cmd/tegata-gui
wails dev
```

Unlock your vault. The Docker audit stack starts in the background automatically. Click the shield icon in the header bar to open the audit panel. The panel provides two buttons.

- **View history** fetches and displays a table of audit events.
- **Verify integrity** runs the hash-chain check. A green banner indicates a valid log. A red banner with "TAMPER DETECTED" appears if the log has been modified.

## Demonstrating tamper detection

This step intentionally corrupts an audit record to prove that ScalarDL detects tampering. First, verify that the audit log is currently intact.

```bash
tegata verify --vault /path/to/vault
```

Now tamper with a record by modifying a hash value directly in the database.

```bash
docker exec docker-compose-postgres-1 psql -U scalardl -d scalardl -c "
  UPDATE scalar.asset
  SET hash = decode('0000000000000000000000000000000000000000000000000000000000000000', 'hex')
  WHERE id LIKE 'tegata-%'
  LIMIT 1;
"
```

On Windows (PowerShell), run this as a single line.

```powershell
docker exec docker-compose-postgres-1 psql -U scalardl -d scalardl -c "UPDATE scalar.asset SET hash = decode('0000000000000000000000000000000000000000000000000000000000000000', 'hex') WHERE id LIKE 'tegata-%%' LIMIT 1;"
```

Run verify again to see tamper detection in action.

```bash
tegata verify --vault /path/to/vault
```

Expected output:

```
Integrity violation detected. <error details>
```

The command exits with code 9. Even with direct database access, modifications are detectable because ScalarDL maintains an independent hash chain that cannot be reconstructed without the original data.

After testing, wipe the audit history to restore a clean state.

```bash
tegata ledger stop --wipe --vault /path/to/vault
```

## Where tamper detection is surfaced

Tamper detection is available in all three interfaces.

- **CLI:** `tegata verify` validates each event individually and reports all faults. `tegata history` displays events with operation, label hash, timestamp, and hash columns.
- **TUI:** Press `v` to open the audit overlay, which provides "View history" and "Verify integrity" options.
- **GUI:** Click the shield icon in the header bar to open the audit panel with "View history" and "Verify integrity" buttons.

## Manual setup (advanced)

If you are running your own ScalarDL instance rather than the bundled Docker stack, skip `tegata ledger start` and configure `tegata.toml` manually.

Add the `[audit]` section to `tegata.toml` in your vault directory.

```toml
[audit]
enabled           = true
server            = "127.0.0.1:50051"
privileged_server = "127.0.0.1:50052"
entity_id         = "tegata-client"
key_version       = 1
secret_key        = "your-32-byte-hex-secret-key-here"
insecure          = true
```

The `insecure = true` flag disables TLS, which is appropriate for local testing. Do not use this in production.

> **Known limitation:** TLS transport with HMAC authentication is not yet supported (tracked in issue #22). For production deployments, protect the connection at the network level using a VPN or SSH tunnel until TLS support is available.

Then run `tegata ledger setup` to register your credentials.

```bash
tegata ledger setup --vault /path/to/vault
```

## Troubleshooting

### Docker daemon does not start

On Windows and macOS, Tegata attempts to launch Docker Desktop automatically. If it is not installed in a standard location, start Docker Desktop manually before running `tegata ledger start` or unlocking the vault.

On Linux, Tegata tries `systemctl start docker` then `service docker start`. If neither works, start the Docker daemon manually with `sudo systemctl start docker`.

### Generic contracts are not registered

Run the registration container manually.

```bash
cd ~/.tegata/docker
docker compose run --rm scalardl-contract-registration
```

If this fails with `INVALID_SIGNATURE`, verify that the secret key in `certs/client.properties` is on a single line with no line breaks in the value.

### The request signature cannot be validated

This error (`DL-COMMON-400003`) means the HMAC secret key used for signing does not match the secret registered with the ledger. Run `tegata config show` to check the configured `secret_key`. If you changed the secret after the initial registration, tear down the stack completely and re-run setup.

```bash
docker compose -f ~/.tegata/docker/docker-compose.yml down -v
tegata ledger start --vault /path/to/vault
```

### coordinator.state table does not exist

This error (`DB-CORE-10016`) means the coordinator schema was not created. Tear down the stack completely and re-run setup.

```bash
docker compose -f ~/.tegata/docker/docker-compose.yml down -v
tegata ledger start --vault /path/to/vault
```

### Docker image errors on macOS with Apple Silicon

If you see errors like `no matching manifest for linux/arm64/v8` or `exec format error`, the ScalarDL and PostgreSQL images are x86-64 only. The bundled `docker-compose.yml` includes `platform: linux/amd64` directives to handle this automatically via Rosetta emulation, which is fully supported by Docker Desktop for Mac.

### Connection refused or timeout

Confirm the ScalarDL containers are running with:

```bash
cd ~/.tegata/docker && docker compose ps
```

On WSL, use `127.0.0.1` instead of `localhost` in `tegata.toml`. WSL resolves `localhost` to IPv6 `::1`, but the ScalarDL containers only listen on IPv4.

### Audit not enabled

Confirm `tegata.toml` has `enabled = true` in the `[audit]` section and that the file is in the same directory as `vault.tegata`. Run `tegata config show` to see the effective configuration.
