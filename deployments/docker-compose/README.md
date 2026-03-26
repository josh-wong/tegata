# ScalarDL local development setup

This Docker Compose template starts an instance of ScalarDL Ledger 3.12 and a PostgreSQL 15 database for local development and integration testing of the Tegata audit layer.

## Prerequisites

Running this setup requires Docker Engine 24 or later with the Compose v2 plugin (`docker compose`, not the older `docker-compose` command).

### macOS with Apple Silicon

The ScalarDL and PostgreSQL Docker images are built for x86-64 (AMD64) only. The `docker-compose.yml` includes `platform: linux/amd64` directives so Docker Desktop runs these images under Rosetta emulation automatically. No extra flags are needed.

## Starting the services

The `--bootstrap` flag on the `scalardl-ledger` service automatically registers the built-in HashStore contracts (`object.Put`, `object.Get`, `object.Validate`, `collection.Create`, `collection.Add`, `collection.Get`) the first time the container starts. These are the contracts that Tegata uses to record and retrieve audit events.

```bash
docker compose up -d
```

## Checking health

Both services should reach a healthy state within 30 seconds.

```bash
docker compose ps
```

## Registering a client certificate

The ScalarDL `LedgerPrivileged` service listens on port 50052. After generating a certificate (see `docs/scalardl-setup.md`), run:

```bash
tegata ledger setup --vault /path/to/vault
```

This calls `RegisterCert` on the privileged port, then `Ping` on port 50051 to confirm end-to-end connectivity.

## Stopping the services

```bash
docker compose down
```

To also remove persistent data volumes:

```bash
docker compose down -v
```
