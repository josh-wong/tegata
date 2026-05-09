# ScalarDL upgrade maintenance

This guide is for Tegata maintainers who are upgrading the bundled ScalarDL stack used by optional audit logging.

User-facing setup and usage belong in [site/docs/scalardl-setup.mdx](../site/docs/scalardl-setup.mdx).

## Upgrade workflow

When upgrading to a new ScalarDL version, update the bundled Docker Compose files and SHA256 hash for the HashStore SDK together.

1. Identify the target ScalarDL version from the [ScalarDL releases page](https://github.com/scalar-labs/scalardl/releases).
2. In all three Docker Compose files, update the ledger/schema-loader image tags and HashStore SDK URL/path:
   - `cmd/tegata/docker-bundle/docker-compose.yml`
   - `cmd/tegata-gui/docker-bundle/docker-compose.yml`
   - `deployments/docker-compose/docker-compose.yml`
3. Download the SDK zip and compute its SHA256 hash (replace `X.Y.Z`):

   ```bash
   curl -fsSL -O https://github.com/scalar-labs/scalardl/releases/download/vX.Y.Z/scalardl-hashstore-java-client-sdk-X.Y.Z.zip
   shasum -a 256 scalardl-hashstore-java-client-sdk-X.Y.Z.zip
   ```

4. Update the SHA256 hash in all three Docker Compose files (the verification line immediately after the `curl` download).
5. Update source comment URLs in:
   - `internal/audit/rpc/scalar.pb.go`
   - `internal/audit/rpc/scalar.proto`
   - `internal/audit/rpc/scalar_grpc.pb.go`

## Why this matters

The SHA256 hash is verified during container startup and fails fast on mismatch, protecting against supply-chain tampering and accidental corruption.

After a new Tegata release, end users do not need to manually update `~/.tegata/docker/docker-compose.yml`. Tegata syncs the embedded file to that path on vault unlock.
