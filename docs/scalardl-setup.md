# ScalarDL Ledger setup

This guide explains how to configure Tegata's optional tamper-evident audit layer backed by ScalarDL Ledger3.12. The audit layer records every authentication event in a hash-chained ledger, enabling post-hoc integrity verification with `tegata verify`.

Audit logging is disabled by default. All authentication operations remain fully functional without it.

## Prerequisites

Setting up the audit layer requires the following tools.

- Docker Engine 24+ with Compose v2 (for the local dev instance)
- `openssl` for generating a P-256 key pair and self-signed certificate
- A Tegata vault initialized with `tegata init`

## Generating a self-signed client certificate

ScalarDL uses digital-signature authentication. Each Tegata vault needs an ECDSA P-256 key pair registered with the ledger. The following `openssl` commands generate a self-signed certificate suitable for development and self-hosted deployments.

```bash
# Generate a P-256 private key (PKCS#8 format).
openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -out client-key.pem

# Generate a self-signed certificate valid for 3 years.
openssl req -new -x509 -key client-key.pem -out client-cert.pem \
  -days 1095 \
  -subj "/CN=tegata-vault/O=tegata"

# Verify the key and certificate match.
openssl pkey -in client-key.pem -pubout | openssl sha256
openssl x509 -in client-cert.pem -pubkey -noout | openssl sha256
```

Store `client-key.pem` and `client-cert.pem` alongside your vault file (on the USB drive). Both files must be present at the paths configured in `tegata.toml`.

## Configuring tegata.toml

Add the `[audit]` section to the `tegata.toml` file in your vault directory. All paths can be relative to the vault directory or absolute.

```toml
[audit]
enabled           = true
server            = "127.0.0.1:50051"
privileged_server = "127.0.0.1:50052"
cert_path         = "client-cert.pem"
key_path          = "client-key.pem"
ca_cert_path      = ""          # Leave empty to use system CA bundle.
entity_id         = "tegata-vault-alice"
key_version       = 1
queue_max_events  = 10000
```

> **Note:** Use `127.0.0.1` instead of `localhost` on WSL. WSL resolves `localhost` to the IPv6 address `::1`, but the ScalarDL containers only listen on IPv4.

For a development setup without TLS (not for production), add:

```toml
insecure = true
```

`entity_id` is a unique identifier for this vault instance. Use a descriptive name such as `tegata-vault-alice` or `tegata-usb-work`. `key_version` starts at `1` and is incremented if you rotate your certificate.

## Registering generic contracts

Generic contracts (`object.Put`, `object.Get`, `object.Validate`) must be registered on the ScalarDL Ledger before any audit operations. The Docker Compose setup includes a registration init container that handles this automatically.

If you need to run registration manually (or re-run it after a fresh Ledger deploy), use the following command.

```bash
docker compose run --rm scalardl-contract-registration
```

This step is idempotent and safe to run multiple times. If you skip it, `tegata ledger setup` will report "Generic contracts are NOT registered" and exit with an error.

## Running tegata ledger setup

After writing the configuration, register the certificate and verify connectivity in one step.

```bash
tegata ledger setup --vault /media/usb
```

This command reads `tegata.toml`, connects to the ScalarDL `LedgerPrivileged` service on the configured server, calls `RegisterCert` to associate your certificate with `entity_id` and `key_version`, calls `Ping` on the Ledger service to confirm connectivity, and then verifies that the generic contracts are registered by performing a test `object.Put` call.

A successful run prints output similar to:

```
Connecting to ScalarDL Ledger at 127.0.0.1:50051 (privileged: 127.0.0.1:50052)...
Registering certificate for entity "tegata-vault-alice" (key version 1)...
Certificate registered successfully.
Verifying ledger connectivity...
Verifying generic contracts are registered...
Generic contracts verified. Audit setup complete.
```

If the server is unreachable, the command exits with code 8 (`ErrNetworkFailed`).

## Verifying the signature byte layout

The ECDSA signature in `ECDSASigner.Sign` serializes the request fields using the same layout as `ContractExecutionRequest.serialize()` in the ScalarDL Java SDK:

```
contractId (UTF-8) || contractArgument (UTF-8) || entityId (UTF-8) || keyVersion (4-byte big-endian int)
```

If contract executions return `UNAUTHENTICATED` gRPC errors after a successful `ledger setup`, you can confirm the layout is correct by running the signature integration test against a live instance:

```bash
go test -tags integration ./internal/audit/... -run TestIntegration_SignatureByteLayout -v \
  SCALARDL_ADDR=localhost:50051 \
  SCALARDL_ENTITY_ID=tegata-vault-alice \
  SCALARDL_CERT_PATH=client-cert.pem \
  SCALARDL_KEY_PATH=client-key.pem
```

## Verifying audit log integrity

After recording events, verify the integrity of the ledger with:

```bash
tegata verify --vault /media/usb
```

This command calls the ScalarDL Ledger to verify the integrity of all recorded audit events. It requires an active ledger connection and exits with code 9 if an integrity violation is detected, or code 8 if the ledger is unreachable.
