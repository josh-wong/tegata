# ScalarDL Ledger setup

This guide explains how to configure Tegata's optional tamper-evident audit layer backed by ScalarDL 3.12 Ledger. The audit layer records every authentication event in a hash-chained ledger, enabling post-hoc integrity verification with `tegata verify`.

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
enabled     = true
server      = "localhost:50051"
cert_path   = "client-cert.pem"
key_path    = "client-key.pem"
ca_cert_path = ""          # Leave empty to use system CA bundle.
entity_id   = "tegata-vault-alice"
key_version = 1
queue_max_events = 10000
```

`entity_id` is a unique identifier for this vault instance. Use a descriptive name such as `tegata-vault-alice` or `tegata-usb-work`. `key_version` starts at `1` and is incremented if you rotate your certificate.

## Running tegata ledger setup

After writing the configuration, register the certificate and verify connectivity in one step.

```bash
tegata ledger setup --vault /media/usb
```

This command reads `tegata.toml`, connects to the ScalarDL LedgerPrivileged service on the configured server, calls `RegisterCert` to associate your certificate with `entity_id` and `key_version`, then calls `Ping` on the Ledger service to confirm end-to-end connectivity.

A successful run prints output similar to:

```
Connecting to ScalarDL Ledger at localhost:50051...
Registering certificate for entity "tegata-vault-alice" (key version 1)...
Certificate registered successfully.
Verifying Ledger connectivity...
ScalarDL Ledger is reachable. Audit setup complete.
```

If the server is unreachable, the command exits with code 8 (`ErrNetworkFailed`).

## Known limitations

The ECDSA signature byte serialization in `ECDSASigner.Sign` (see `internal/audit/signer.go`) is an approximation of what ScalarDL's Java `ClientService.RequestBuilder` produces. The exact byte concatenation order is not documented in the ScalarDL Go client documentation.

If `tegata ledger setup` succeeds but subsequent contract executions return `UNAUTHENTICATED` gRPC errors, the signature byte layout is incorrect. To diagnose:

```bash
go test -tags integration ./internal/audit/... -run TestIntegration_SignatureByteLayout -v \
  SCALARDL_ADDR=localhost:50051 \
  SCALARDL_ENTITY_ID=tegata-vault-alice \
  SCALARDL_CERT_PATH=client-cert.pem \
  SCALARDL_KEY_PATH=client-key.pem
```

The test output will report the exact error and the expected byte layout. Compare against the Java source to determine the correct order and update `ECDSASigner.Sign` accordingly.

## Verifying audit log integrity

After recording events, verify the integrity of the ledger with:

```bash
tegata verify --vault /media/usb
```

This command checks the local hash chain in the offline queue and (if the ledger is reachable) cross-validates event hashes against the ScalarDL Ledger, returning a non-zero exit code if any integrity violation is detected.
