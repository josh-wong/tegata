# ScalarDL audit setup and testing

This guide explains how to set up and test Tegata's optional tamper-evident audit layer backed by ScalarDL Ledger 3.12. The audit layer records every authentication event in a hash-chained ledger, enabling post-hoc integrity verification with `tegata verify`.

Audit logging is disabled by default. All authentication operations remain fully functional without it.

## Prerequisites

Setting up the audit layer requires the following tools.

- Docker Desktop (Windows, macOS) or Docker Engine 24+ (Linux) with Compose v2
- Go 1.25+ to build Tegata from source
- A Tegata vault initialized with `tegata init`

No certificates or key pairs are needed. Tegata uses ScalarDL's HMAC authentication mode, which requires only a shared secret key configured in `tegata.toml`.

## Step 1: Start ScalarDL with Docker Compose

From the repository root, navigate to the Docker Compose directory and start the stack.

```bash
cd deployments/docker-compose
docker compose up -d
```

This starts four services.

- **postgres** — backing database for ScalarDL
- **scalardl-schema-loader** — creates the ledger database schema on first run
- **scalardl-coordinator-schema** — creates the coordinator table needed by the transaction manager
- **scalardl-ledger** — the ScalarDL Ledger gRPC server (ports 50051 and 50052)

A fifth service, **scalardl-contract-registration**, downloads the HashStore SDK and registers the generic contracts (`object.Put`, `object.Get`, `object.Validate`, `collection.Create`, `collection.Add`, `collection.Get`). Wait about 30 seconds after starting for this service to finish.

Verify the registration succeeded.

```bash
docker compose logs scalardl-contract-registration
```

You should see `"status_code" : "OK"` in the output. If you see `INVALID_SIGNATURE`, check that `certs/client.properties` has the secret key on a single line (no line breaks in the value).

## Step 2: Configure the client properties

Copy the example client properties file. This file is used by the Docker bootstrap container to register the HMAC secret and contracts.

```bash
cp client.properties.example certs/client.properties
```

The default entity ID is `tegata-client`. If you want the bootstrap to register under the same entity as your vault, edit `certs/client.properties` and change the entity ID to match your vault's `tegata.toml` (see step 3). The secret key must also match.

The default properties file contents are as follows.

```properties
scalar.dl.client.server.host=scalardl-ledger
scalar.dl.client.server.port=50051
scalar.dl.client.server.privileged_port=50052
scalar.dl.client.authentication.method=hmac
scalar.dl.client.entity.id=tegata-client
scalar.dl.client.entity.identity.hmac.secret_key=tegata-test-secret-key-1234567890abcdef
scalar.dl.client.entity.identity.hmac.secret_key_version=1
```

The `entity.id` in the bootstrap must match the `entity_id` in your vault's `tegata.toml`. Contracts are registered per entity, so the entity that registers contracts must be the same entity that executes them.

## Step 3: Configure tegata.toml

Add the `[audit]` section to the `tegata.toml` file in your vault directory. The `secret_key` must match the value in `certs/client.properties`.

```toml
[audit]
enabled           = true
server            = "localhost:50051"
privileged_server = "localhost:50052"
entity_id         = "tegata-client"
key_version       = 1
secret_key        = "tegata-test-secret-key-1234567890abcdef"
insecure          = true
```

The `insecure = true` flag disables TLS, which is appropriate for local Docker testing. Do not use this in production.

`entity_id` is a unique identifier for this vault instance. Use a descriptive name such as `tegata-vault-alice` or `tegata-usb-work`. `key_version` starts at `1` and is incremented if you rotate your secret key.

For production deployments, use a strong random secret key (at least 32 characters of hex). The `secret_key` value in `tegata.toml` and `client.properties` must match exactly.

## Step 4: Run ledger setup

Build the Tegata binary and register the HMAC secret with the ledger.

```bash
cd ../../
go build -o tegata ./cmd/tegata/
./tegata ledger setup --vault /path/to/your/vault
```

On Windows (PowerShell), build with `go build -o tegata.exe ./cmd/tegata/` and run with `.\tegata.exe ledger setup --vault $VaultPath`.

A successful run prints the following.

```
Connecting to ScalarDL Ledger at localhost:50051 (privileged: localhost:50052)...
WARNING: Insecure mode enabled — TLS disabled. Do not use in production.
Registering secret for entity "tegata-client" (key version 1)...
Secret registered successfully.
Verifying ledger connectivity...
Verifying generic contracts are registered...
Generic contracts verified. Audit setup complete.
```

## Step 5: Test audit in the CLI

Generate a TOTP code to trigger an audit event.

```bash
./tegata code my-totp-credential --vault /path/to/your/vault
```

Check audit history to confirm the event was recorded.

```bash
./tegata history --vault /path/to/your/vault
```

Verify hash-chain integrity.

```bash
./tegata verify --vault /path/to/your/vault
```

This command exits with code 9 if an integrity violation is detected, or code 8 if the ledger is unreachable.

Expected output when the audit log is intact:

```
Audit log integrity verified. 3 events checked.
```

## Step 6: Test audit in the TUI

Launch the TUI and unlock your vault.

```bash
./tegata tui --vault /path/to/your/vault
```

Press `v` to open the audit overlay. Use the arrow keys or `j`/`k` to select an option and press Enter.

- **View history** fetches and displays all audit events from the ledger.
- **Verify integrity** validates the hash chain and reports whether the log is intact or tampered.

Press Esc to return to the menu or close the overlay.

## Step 7: Test audit in the GUI

Build and run the desktop GUI.

```bash
cd cmd/tegata-gui
wails dev
```

Unlock your vault, then click the shield icon in the header bar to open the audit panel. The panel provides two buttons.

- **View history** fetches and displays a table of audit events.
- **Verify integrity** runs the hash-chain check. A green banner indicates a valid log. A red banner with "TAMPER DETECTED" appears if the log has been modified.

## Step 8: Demonstrate tamper detection

This step intentionally corrupts an audit record in the database to prove that ScalarDL detects tampering. First, verify that the audit log is currently intact.

```bash
./tegata verify --vault /path/to/your/vault
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

Run verify again to see the tamper detection.

```bash
./tegata verify --vault /path/to/your/vault
```

Expected output when tampering is detected:

```
Integrity violation detected. <error details>
```

The command exits with code 9 to indicate an integrity violation. This demonstrates that even with direct database access, modifications to audit records are detectable because ScalarDL maintains an independent hash chain that cannot be reconstructed without the original data.

After testing, wipe and restart the Docker stack to restore a clean state.

```bash
cd deployments/docker-compose
docker compose down -v
docker compose up -d
```

## Where tamper detection is surfaced

Tamper detection is available in all three interfaces.

- **CLI:** `tegata verify` validates each event individually via the per-entity collection and reports all faults. `tegata history` displays events with operation, label hash, timestamp, and hash columns.
- **TUI:** Press `v` to open the audit overlay, which provides "View history" and "Verify integrity" options matching the CLI output.
- **GUI:** Click the shield icon in the header bar to open the audit panel with "View history" and "Verify integrity" buttons.

## Troubleshooting

### Generic contracts are NOT registered

Run the registration container manually.

```bash
cd deployments/docker-compose
docker compose run --rm scalardl-contract-registration
```

If this fails with `INVALID_SIGNATURE`, verify that the secret key in `certs/client.properties` is on a single line with no line breaks in the value.

### The request signature can't be validated

This error (`DL-COMMON-400003`) means the HMAC secret key used for signing does not match the secret registered with the ledger. Confirm the `secret_key` in `tegata.toml` matches the value in `certs/client.properties` exactly. If you changed the secret after the initial registration, wipe the database and restart.

```bash
docker compose down -v
docker compose up -d
```

### coordinator.state table does not exist

This error (`DB-CORE-10016`) means the coordinator schema was not created. The `scalardl-coordinator-schema` init container handles this automatically. If you started the stack before this service was added, wipe and restart.

```bash
docker compose down -v
docker compose up -d
```

### Docker image errors on macOS with Apple Silicon

If you see errors like `no matching manifest for linux/arm64/v8` or `exec format error` when starting Docker Compose on macOS with Apple Silicon (M1/M2/M3/M4), the ScalarDL and PostgreSQL images are x86-64 only. The `docker-compose.yml` includes `platform: linux/amd64` directives to handle this automatically. If you are using a custom compose file without these directives, add `platform: linux/amd64` to each service or set the environment variable before starting.

```bash
DOCKER_DEFAULT_PLATFORM=linux/amd64 docker compose up -d
```

The images run under Rosetta emulation, which is fully supported by Docker Desktop for Mac.

### Connection refused or timeout

Confirm the ScalarDL containers are running with `docker compose ps`. On WSL, use `127.0.0.1` instead of `localhost` in `tegata.toml` because WSL resolves `localhost` to IPv6 `::1`, but the ScalarDL containers only listen on IPv4.

### Audit not enabled

Confirm `tegata.toml` has `enabled = true` in the `[audit]` section and that the file is in the same directory as `vault.tegata`.
