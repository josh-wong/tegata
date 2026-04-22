# Roadmap: Tegata

## Overview

Tegata delivers a portable encrypted authenticator in six phases, building bottom-up from cryptographic primitives to a full desktop GUI. Phase 1 establishes the security foundation that every subsequent component depends on. Phase 2 delivers a complete standalone authenticator (vault, TOTP, HOTP, static passwords, CLI) that users can carry on a USB drive. Phase 3 adds challenge-response signing, vault export/import, and credential organization. Phase 4 layers on the ScalarDL tamper-evident audit system. Phase 5 wraps the CLI in a terminal UI for guided workflows. Phase 6 delivers the desktop GUI, documentation, and security review to reach v1.0 stable.

## Phases

**Phase Numbering:**

- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Foundation** - Crypto primitives, memguard wrapper, shared types, and project scaffolding
- [ ] **Phase 2: Core authenticator** - Vault, auth engines, clipboard, config, CLI with daily-use commands
- [x] **Phase 3: Enhancements and export** - Challenge-response signing, vault export/import, credential organization (completed 2026-03-17)
- [x] **Phase 4: Audit layer** - ScalarDL integration, offline queue, verify and history commands (completed 2026-03-18)
- [ ] **Phase 4.1: Deploy HashStore contracts** - INSERTED: Fix contract IDs, add registration step, wire audit into TUI and GUI, E2E integration test (GitHub #6)
- [x] **Phase 4.2: Audit verification in TUI and GUI** - INSERTED: Tamper detection and audit history views in TUI and GUI (completed 2026-03-25)
- [x] **Phase 4.3: Collection-based audit storage** - INSERTED: Rearchitect to use ScalarDL collections for full event history with metadata (completed 2026-03-26)
- [x] **Phase 5: Terminal UI** - Bubbletea TUI with guided setup, visual countdown, and credential management (completed 2026-03-20)
- [ ] **Phase 6: GUI, docs, and release** - Wails desktop GUI, documentation, security review, release automation
- [ ] **Phase 7: One-click Docker audit setup with auto-start on vault unlock** - Single-command audit setup across CLI, TUI, and GUI; Docker stack auto-starts on unlock
- [x] **Phase 8: Audit opt-in during vault creation and auto_start config toggle** - Prompt during vault creation, `audit.auto_start` field, and settings toggle across CLI/TUI/GUI (GitHub #15) (completed 2026-04-02)
- [x] **Phase 9: E2E Docker audit testing** - Integration tests for SetupStack/MaybeAutoStart with CI job (completed 2026-04-04)
- [x] **Phase 10: Audit client TLS and refactoring** - TLS transport, struct-based client config, shared event builder factory, FAT32 crash test (v0.9) (completed 2026-04-15)
- [x] **Phase 11: macOS binary signing** - Apple Developer ID codesigning and notarization in GitHub Actions (v0.9) (completed 2026-04-12)
- [x] **Phase 12: ScalarDL 3.13 upgrade** - Upgrade Ledger and schema loader Docker images from 3.12 to 3.13 (v0.9) (completed 2026-04-14)
- [x] **Phase 12.1: Release workflow end-to-end test** - INSERTED: Test GitHub Actions release workflow with pre-release tag, verify all artifacts (GitHub #20) (v0.9.5) (completed 2026-04-18)
- [ ] **Phase 12.2: Screenshots and documentation polish** - INSERTED: Add GUI and TUI screenshots to README, visual polish (GitHub #21) (v1.0)
- [ ] **Phase 12.3: UI/UX improvements - audit and critical fixes** - INSERTED: Audit history sorting, TUI Enter key fix, vault path storage warning (GitHub #24 selected items) (v0.9.5)
- [ ] **Phase 12.4: Pre-release validation** - INSERTED: Cross-platform testing and release readiness sign-off before v0.9.5 tag (Wayland and idle timer fixes already merged) (v0.9.5)
- [ ] **Phase 12.5: Fix audit validation** - INSERTED: Store hash values in vault at submission time for independent validation (GitHub #67) (v0.9.5)
- [ ] **Phase 12.6: Expand audit logging** - INSERTED: Capture vault lifecycle and credential management events in audit log (GitHub #59) (v0.9.5)
- [ ] **Phase 12.7: Audit history display improvements** - INSERTED: Sortable/filterable audit history, pagination, click-to-reveal hashes (GitHub #24) (v0.9.5)
- [ ] **Phase 12.8: Docker lifecycle and HMAC key hardening** - INSERTED: Stop Docker containers on app close, move HMAC secret from plaintext files into encrypted vault (GitHub #52, #53) (v0.9.6)
- [ ] **Phase 12.9: Audit server UX and PostgreSQL volume protection** - INSERTED: Surface running audit server status to users; PostgreSQL exposure window closed by #52 fix (GitHub #54, #55) (v0.9.6)
- [ ] **Phase 13: Traceability cleanup** - Update 20 stale Pending entries in REQUIREMENTS.md to Complete (v1.0)
- [ ] **Phase 14: v1.0 Documentation and release** - Comprehensive user documentation, contribution guides, final release (v1.0)

## Phase Details

### Phase 1: Foundation

**Goal**: Cryptographic primitives and shared data types are implemented, tested, and enforced so that every subsequent component builds on a secure, correct foundation.
**Depends on**: Nothing (first phase)
**Requirements**: SECR-01, SECR-02, PLAT-04
**Success Criteria** (what must be TRUE):
  1. Argon2id key derivation produces correct output for known test vectors
  2. AES-256-GCM encrypt/decrypt round-trips correctly with authenticated data
  3. All key material is allocated via memguard outside the Go GC heap, and zeroed after use
  4. No internal package other than `internal/crypto/guard` imports memguard directly
  5. Project builds as a single static binary (CGO_ENABLED=0) under 20MB
**Plans**: 2 plans

Plans:

- [ ] 01-01-PLAN.md -- Project scaffolding and memguard guard wrapper with TDD
- [ ] 01-02-PLAN.md -- Crypto primitives, model types, error conventions, CLI stub, and binary size verification

### Phase 2: Core authenticator

**Goal**: Users can initialize an encrypted vault on a USB drive, add credentials, and generate TOTP/HOTP codes and retrieve static passwords from any machine without installing software.
**Depends on**: Phase 1
**Requirements**: AUTH-01, AUTH-02, AUTH-03, AUTH-04, AUTH-05, AUTH-08, AUTH-09, AUTH-10, VALT-01, VALT-02, VALT-03, VALT-04, VALT-05, VALT-06, VALT-07, VALT-08, VALT-09, SECR-03, SECR-04, SECR-05, CLIP-01, CLIP-02, CLIP-03, CLIP-04, PLAT-01, PLAT-02, PLAT-03, PLAT-05, PLAT-06, PLAT-07, PLAT-08
**Success Criteria** (what must be TRUE):
  1. User can run `tegata init` on a USB drive to create an encrypted vault with PIN/passphrase and receive a recovery key
  2. User can add TOTP/HOTP credentials manually or by pasting an otpauth:// URI, and see them listed via `tegata list`
  3. User can run `tegata code <label>` to generate a valid TOTP or HOTP code that works with the target service
  4. User can store a static password and retrieve it to the clipboard, which auto-clears after the configured timeout
  5. Vault auto-locks after idle timeout, PIN attempts are rate-limited, and the binary runs from USB with zero installation on Windows, macOS, and Linux
**Plans**: 6 plans

Plans:

- [ ] 02-01-PLAN.md -- Vault manager: header serialization, rate limiting, create/open/unlock/save, recovery key, credential CRUD
- [ ] 02-02-PLAN.md -- Auth engines: TOTP (RFC 6238), HOTP (RFC 4226), static password, otpauth:// URI parsing
- [ ] 02-03-PLAN.md -- Config manager (TOML parsing with defaults) and clipboard manager (auto-clear with cancellation)
- [ ] 02-04-PLAN.md -- CLI commands: init, add, list, remove with vault discovery and passphrase prompting helpers
- [ ] 02-05-PLAN.md -- CLI commands: code, get, resync, bench, config show with cross-platform build verification
- [ ] 02-06-PLAN.md -- Integration tests and human verification of full CLI workflow

### Phase 3: Enhancements and export

**Goal**: Users can perform challenge-response signing, back up and restore their vault, and organize credentials as their collection grows.
**Depends on**: Phase 2
**Requirements**: AUTH-06, AUTH-07, VALT-10, VALT-11, VALT-12, VALT-13, VALT-14, CLIP-05
**Success Criteria** (what must be TRUE):
  1. User can run `tegata sign <label>` with a challenge (via argument or stdin) and receive an HMAC-SHA1 or HMAC-SHA256 signed response on stdout
  2. User can export vault to an encrypted backup file and import it on another machine to restore all credentials
  3. User can organize credentials with groups and tags, and filter `tegata list` output accordingly
  4. CLI command set is documented with backwards-compatibility guarantees
**Plans**: 3 plans

Plans:

- [ ] 03-01-PLAN.md -- Challenge-response engine (TDD) and tegata sign command with --challenge and --clip flags
- [ ] 03-02-PLAN.md -- Encrypted vault export/import (TDD) with tegata export and tegata import commands
- [ ] 03-03-PLAN.md -- Tag organization, vault maintenance commands (change-passphrase, verify-recovery), and STABILITY.md

### Phase 4: Audit layer

**Goal**: Users can connect to a ScalarDL Ledger instance and get tamper-evident logging of every authentication operation, with offline resilience and verifiable integrity.
**Depends on**: Phase 2
**Requirements**: AUDT-01, AUDT-02, AUDT-03, AUDT-04, AUDT-05, AUDT-06, AUDT-07, AUDT-08, AUDT-09, AUDT-10, SECR-06, DOCS-03, DOCS-04
**Success Criteria** (what must be TRUE):
  1. User can run `tegata ledger setup` to configure a ScalarDL connection, and subsequent auth operations are logged to the ledger
  2. User can run `tegata verify` to confirm the hash-chain integrity of their audit log
  3. User can run `tegata history` to view a human-readable list of auth events, filtered by date, label, or operation type
  4. Auth events queue locally when ScalarDL is unreachable and automatically submit when connectivity is restored
  5. Audit log entries never contain plaintext secrets or unhashed credential identifiers
**Plans**: 3 plans

Plans:

- [ ] 04-01-PLAN.md — AuthEvent struct, OfflineQueue with AES-256-GCM and hash chain, AuditConfig extension (TDD)
- [ ] 04-02-PLAN.md — gRPC client, proto stubs, ECDSASigner, tegata ledger setup, Docker Compose, scalardl-setup.md
- [ ] 04-03-PLAN.md — EventBuilder wired into auth commands, tegata history and tegata verify commands

### Phase 4.1: Deploy HashStore contracts

**Goal**: ScalarDL audit integration works end-to-end against a live HashStore instance — correct contract IDs, registration step, audit events from all interfaces (CLI, TUI, GUI), and offline queue flushing verified with an integration test.
**Depends on**: Phase 4
**Requirements**: AUDT-01, AUDT-02, AUDT-05, AUDT-06, AUDT-09
**Success Criteria** (what must be TRUE):
  1. `tegata ledger setup` connects to a Docker ScalarDL instance and registers contracts successfully
  2. `tegata history` returns recorded authentication events from a live ScalarDL Ledger
  3. `tegata verify` validates hash-chain integrity and reports success or tampering
  4. TUI credential actions (code, get, sign) emit audit events when audit is configured
  5. Desktop GUI credential actions emit audit events when audit is configured
  6. Offline event queue flushes correctly when ScalarDL connectivity is restored
  7. Integration test passes against a live ScalarDL Docker instance
**Plans**: 3 plans

Plans:

- [x] 04.1-01-PLAN.md — Fix object.Validate schema, add contract registration to Docker Compose, enhance ledger setup
- [x] 04.1-02-PLAN.md — Wire EventBuilder into TUI and GUI credential action handlers
- [x] 04.1-03-PLAN.md — E2E integration tests for Put+Get+Validate and offline queue flush

### Phase 5: Terminal UI

**Goal**: Users who prefer a guided interface can use Tegata through an interactive terminal UI with visual feedback, without losing any CLI functionality.
**Depends on**: Phase 3
**Requirements**: TUIX-01, TUIX-02, TUIX-03
**Success Criteria** (what must be TRUE):
  1. User can launch the TUI and complete first-time vault setup through a guided wizard flow
  2. User can view all credentials with a live TOTP countdown timer and copy codes interactively
  3. User can add, edit, and remove credentials through the TUI without dropping to CLI commands
**Plans**: 5 plans

Plans:

- [ ] 05-01-PLAN.md — Install charmbracelet dependencies and create TUI test scaffold (Wave 0)
- [ ] 05-02-PLAN.md — Root model, styles, cobra entry point, and first-time setup wizard (TUIX-01)
- [ ] 05-03-PLAN.md — Unlock screen, daily use main view, TOTP ticker, idle auto-lock (TUIX-02)
- [ ] 05-04-PLAN.md — Add/remove overlays and settings overlay with 4 sub-flows (TUIX-03)
- [ ] 05-05-PLAN.md — Human verification checkpoint: end-to-end TUI in real terminal

### Phase 4.2: Audit verification in TUI and GUI

**Goal**: Users can check audit log integrity and view audit history directly from the TUI and GUI without dropping to the CLI.
**Depends on**: Phase 4.1, Phase 5, Phase 6
**Requirements**: AUDT-05, AUDT-06
**Success Criteria** (what must be TRUE):
  1. TUI displays an "Audit" menu item that shows audit history and a verify action
  2. GUI displays an audit status indicator and a verify button in the settings or audit panel
  3. Both TUI and GUI show a clear tamper-detected warning when `tegata verify` reports an integrity violation
  4. Audit history is viewable in both TUI and GUI with the same data as `tegata history`
**Plans**: 2 plans

Plans:

- [x] 04.2-01-PLAN.md — TUI audit overlay with async history viewing and integrity verification
- [x] 04.2-02-PLAN.md — GUI audit panel with Go backend methods, React component, and tamper warning

### Phase 4.3: Rearchitect audit storage with ScalarDL collections

**Goal**: Audit history shows all events with readable details (operation type, label, timestamp), not just the latest hash. Each event is individually verifiable.
**Depends on**: Phase 4.2
**Requirements**: AUDT-05, AUDT-06
**Success Criteria** (what must be TRUE):
  1. Each audit event is stored as its own ScalarDL object with metadata (operation, label hash, timestamp)
  2. A per-entity collection tracks all event object IDs
  3. `tegata history` lists all events with operation type, label hash, and timestamp
  4. TUI and GUI history views show the same detailed event list
  5. `tegata verify` validates each event individually via the collection
  6. Multiple code generations produce multiple distinct history entries
**Plans**: 2 plans

Plans:

- [x] 04.3-01-PLAN.md — Core client collection methods, CLI history and verify rewrite
- [x] 04.3-02-PLAN.md — TUI audit overlay and GUI audit panel collection-based updates

**Architecture:**
- Store: `object.Put(eventID, hash, metadata:{operation, label_hash, timestamp})` + `collection.Add(collectionID, [eventID])`
- List: `collection.Get(collectionID)` → all event IDs → `object.Get(each)` for hash + metadata
- Verify: iterate collection members → `object.Validate(each)`
- Contract IDs: `object.v1_0_0.Put`, `collection.v1_0_0.Create`, `collection.v1_0_0.Add`, `collection.v1_0_0.Get`
- WIP stashed on `feature/deploy-hashstore-contracts` branch (`git stash list`)

### Phase 6: GUI, docs, and release

**Goal**: Tegata reaches v1.0 stable with a desktop GUI for non-CLI users, comprehensive documentation, and a completed security review.
**Depends on**: Phase 4, Phase 5
**Requirements**: GUIX-01, GUIX-02, GUIX-03, GUIX-04, SECR-07, DOCS-01, DOCS-02
**Success Criteria** (what must be TRUE):
  1. User can install the desktop GUI on Windows, macOS, or Linux and perform all operations available in the CLI
  2. GUI detects and unlocks a vault on a connected USB drive without requiring manual path entry
  3. Comprehensive user documentation, contribution guidelines, and community governance documents are published
  4. An independent security review of cryptographic implementation, vault format, and memory handling is completed and findings addressed
**Plans**: 7 plans

Plans:

- [x] 06-01-PLAN.md — Wails project scaffolding and Go adapter with unit tests
- [x] 06-02-PLAN.md — Frontend shell: shadcn/ui, Tailwind, cinnabar theming, and layout components
- [x] 06-03-PLAN.md — User documentation, CLI reference, security self-audit, CONTRIBUTING.md, and README
- [x] 06-04-PLAN.md — GUI hooks, shared components, unlock view, and setup wizard
- [x] 06-05-PLAN.md — GUI credential detail, settings panel, and App.tsx wiring
- [x] 06-06-PLAN.md — Release automation: GitHub Actions workflow, installer configs, Makefile targets
- [ ] 06-07-PLAN.md — Human verification checkpoint for all Phase 6 deliverables

### Phase 7: One-click Docker audit setup with auto-start on vault unlock

**Goal**: The ScalarDL audit layer is practically usable for all users — CLI, TUI, and GUI — via a single setup action that auto-starts the Docker stack on vault unlock.
**Depends on**: Phase 6
**Requirements**: TBD
**Success Criteria** (what must be TRUE):
  1. User can run `tegata ledger start` to complete the full Docker audit setup sequence in one command
  2. TUI audit overlay offers "Start audit server" as a third menu item alongside View history and Verify integrity
  3. GUI AuditPanel shows a "Start audit server" button that runs the setup sequence inline
  4. After setup, the Docker stack starts automatically in the background when the user unlocks the vault and the ledger is unreachable
  5. `tegata ledger stop` stops containers while preserving audit data
  6. Users who never run setup never encounter any Docker-related behavior
**Plans**: 6 plans

Plans:

- [x] 07-01-PLAN.md — Core infrastructure: audit/docker.go, config/write_audit.go, VaultPayload.VaultID, compose named volume, embed bundles
- [x] 07-02-PLAN.md — Test scaffold (Wave 0): docker_test.go, write_audit_test.go, CLI and TUI test stubs
- [x] 07-03-PLAN.md — CLI wiring: tegata ledger start/stop subcommands and auto-start hook in TUI and GUI unlock
- [ ] 07-04-PLAN.md — TUI audit overlay: Start audit server menu item and viewAuditStart sub-flow
- [ ] 07-05-PLAN.md — GUI wiring: AuditPanel Start/Stop buttons, Wails bindings
- [ ] 07-06-PLAN.md — Human verification checkpoint for all Phase 7 deliverables

### Phase 8: Audit opt-in during vault creation and auto_start config toggle

**Goal**: Users are prompted to opt in to audit logging during vault creation, and can toggle auto-start from settings — closing the remaining gaps from GitHub issue #15.
**Depends on**: Phase 7
**Requirements**: TBD
**Success Criteria** (what must be TRUE):
  1. `tegata init`, TUI creation wizard, and GUI creation wizard all prompt users to optionally enable audit logging (requires Docker)
  2. `audit.auto_start` boolean field exists in `tegata.toml` and controls whether Docker auto-start fires on unlock
  3. `tegata config` (or equivalent), TUI settings overlay, and GUI settings panel all expose a toggle for `auto_start`
  4. Users who decline the prompt during vault creation never see any Docker-related behavior
**Plans**: 4 plans

Plans:

- [x] 08-01-PLAN.md — Config layer: AutoStart field, default logic, WriteAuditSection extension, MaybeAutoStart gate
- [x] 08-02-PLAN.md — Vault creation opt-in: CLI init prompt, TUI wizard step 5/5, GUI wizard checkbox
- [x] 08-03-PLAN.md — Settings toggle: tegata config set subcommand, TUI keybind, GUI settings checkbox
- [x] 08-04-PLAN.md — Gap closure: Fix enabled=false in opt-in flows, conditional audit menu visibility, Enabled-based gating

### Phase 9: End-to-end Docker audit setup verification and auto-start testing

**Goal:** Docker audit setup (`SetupStack`) and auto-start (`MaybeAutoStart`) are validated end-to-end against a real Docker environment via integration tests, with a CI job to run them automatically on ubuntu-latest.
**Requirements**: D-01, D-02, D-03, D-04, D-05, D-06
**Depends on:** Phase 8
**Success Criteria** (what must be TRUE):
  1. Integration tests verify SetupStack happy path, MaybeAutoStart restart flow, and error paths against real Docker
  2. Tests use `//go:build integration` tag and run with `go test -tags integration ./internal/audit/... -v -timeout 300s`
  3. CI has a separate `integration-tests` job on `ubuntu-latest` that runs Docker integration tests
  4. Any bugs surfaced by the tests are fixed inline
**Plans:** 1 plan

Plans:

- [x] 09-01-PLAN.md -- Docker integration tests for SetupStack/MaybeAutoStart/error paths and CI job addition

### Phase 10: Audit client TLS and refactoring

**Goal**: The audit client supports TLS transport alongside HMAC authentication, `NewClientFromConfig` accepts a struct instead of 6 positional parameters, and event builder construction is deduplicated into a shared factory — closing GitHub issues #22 and #30 (v0.9).
**Depends on**: Phase 9
**Milestone**: v0.9 -- Security audit
**Requirements**: AUDT-11, AUDT-12, AUDT-13, SECR-08
**Success Criteria** (what must be TRUE):
  1. `NewClientFromConfig` accepts `config.AuditConfig` and supports `insecure = false` with HMAC auth over TLS
  2. All 14 call sites of `NewClientFromConfig` are updated to the struct form
  3. A shared `audit.NewEventBuilderFromConfig` factory replaces duplicated key derivation logic in CLI and GUI
  4. All existing unit and integration tests pass
  5. FAT32 atomic write integration test verifies `vault.atomicWrite` recovery from simulated mid-rename failure
**Plans**: 2 plans

Plans:

- [x] 10-01-PLAN.md -- TLS support in audit client, `NewClientFromConfig` struct refactor, shared `NewEventBuilderFromConfig` factory
- [x] 10-02-PLAN.md -- FAT32 atomic write integration test (quick task)

### Phase 11: macOS binary signing

**Goal**: macOS release binaries are codesigned and notarized so users can run Tegata without Gatekeeper warnings or manual security overrides.
**Depends on**: Phase 10
**Milestone**: v0.9 -- Security audit
**Requirements**: SECR-09
**Success Criteria** (what must be TRUE):
  1. GitHub Actions release workflow codesigns macOS amd64 and arm64 binaries with an Apple Developer ID certificate
  2. Signed binaries are submitted to Apple notarization service and stapled
  3. A freshly downloaded macOS binary passes Gatekeeper (`spctl --assess`) without user intervention
**Plans**: 2 plans

Plans:

- [x] 11-01-PLAN.md -- Apple Developer ID codesigning and notarization in GitHub Actions release workflow

### Phase 12: ScalarDL 3.13 upgrade

**Goal**: ScalarDL Docker images are upgraded from 3.12 to 3.13, picking up 6 CVE patches and the NPE fix for the gRPC client, with all integration tests still passing.
**Depends on**: Phase 10
**Milestone**: v0.9 -- Security audit
**Requirements**: AUDT-14
**Success Criteria** (what must be TRUE):
  1. Docker Compose files reference ScalarDL Ledger 3.13.0 and schema loader 3.13.0 images
  2. Proto stubs are verified compatible with 3.13 (no breaking changes in wire format)
  3. All existing integration tests (`go test -tags integration ./internal/audit/...`) pass against the 3.13 Docker stack
  4. `tegata ledger start` and `tegata ledger setup` complete successfully with the upgraded images
**Plans**: 2 plans

Plans:

- [x] 12-01-PLAN.md -- Bump ScalarDL Docker image tags to 3.13.0, verify proto compatibility, run integration tests

### Phase 12.1: Release workflow end-to-end test

**Goal**: The GitHub Actions release workflow produces correct artifacts for all platforms and can be reliably used to create releases. Pre-release testing validates the build pipeline before the final v1.0 release.
**Depends on**: Phase 6 (release automation completed)
**Milestone**: v0.9.5 -- Polish and validation
**Requirements**: Issue #20
**Success Criteria** (what must be TRUE):
  1. Tag a pre-release (e.g., `v0.9.5-rc.1`) to trigger the workflow
  2. All 5 release jobs complete successfully: cli-binaries, gui-windows, gui-macos, gui-linux, release
  3. CLI binaries are produced for Windows (amd64), macOS (arm64, amd64), and Linux (amd64), each < 20MB
  4. GUI installers are produced: Windows NSIS setup exe, macOS universal DMG, Linux deb/rpm
  5. Artifact names, sizes, and checksums are reasonable
  6. GitHub Release is created with all artifacts attached
  7. Test download and launch CLI binary and GUI installer on at least one platform
  8. Fix any workflow failures and re-test before tagging v1.0.0
**Plans**: 2 plans

Plans:

- [x] 12.1-01-PLAN.md -- Pre-release tag workflow testing, artifact verification, and platform validation

### Phase 12.2: Screenshots and documentation polish

**Goal**: README displays actual application screenshots instead of placeholders, giving users a visual preview of Tegata's interface before installation.
**Depends on**: Phase 6 (GUI complete)
**Milestone**: v1.0 -- Stable release
**Requirements**: Issue #21
**Success Criteria** (what must be TRUE):
  1. Screenshots capture: GUI unlock screen, credential list with TOTP countdown, add credential flow, audit panel, settings panel
  2. At least one TUI screenshot showing the main credential view
  3. Both light and dark themes are represented
  4. Images stored in docs/images/ with proper compression (PNG or WebP)
  5. README uses relative paths and renders correctly on GitHub
  6. Screenshots convey the application's look and feel clearly
**Plans**: 2 plans

Plans:

- [ ] 12.2-01-PLAN.md -- Capture GUI and TUI screenshots, optimize images, update README

### Phase 12.3: UI/UX improvements - audit and critical fixes

**Goal**: High-priority UI/UX issues are fixed to improve usability before release: audit history displays in chronological order (newest first), TUI vault path input responds to Enter key, and users are warned about local vault storage security tradeoffs.
**Depends on**: Phase 6 (GUI complete), Phase 5 (TUI complete)
**Milestone**: v0.9.5 -- Polish and validation
**Requirements**: GitHub issues #38, #39, #40 (maps to TUIX-04, TUIX-05, SECR-10)
**Success Criteria** (what must be TRUE):
  1. Audit history displays sorted by timestamp (newest first) across CLI, TUI, and GUI (issue #38)
  2. TUI vault path input correctly responds to Enter key to advance to passphrase screen (issue #39)
  3. Vault creation wizard warns users about local/system drive security tradeoffs vs. removable drives (issue #40)
  4. All UI changes are tested and verified across all three interfaces
  5. Three separate PRs delivered (one per improvement)
**Plans**: 3 plans (one per PR)

Plans:

- [x] 12.3-01-PLAN.md -- Audit history sorting (newest first) in CLI, TUI, GUI
- [x] 12.3-02-PLAN.md -- TUI vault path input Enter key fix
- [x] 12.3-03-PLAN.md -- Vault storage security warning (local vs. removable drives)

### Phase 12.4: Pre-release validation

**Goal**: Final validation before v0.9.5 release: cross-platform testing across platforms, and readiness verification. (Wayland clipboard fallback and idle timer fixes were merged early as PRs #48 and #49.)
**Depends on**: Phases 12.1, 12.2, 12.3
**Milestone**: v0.9.5 -- Polish and validation
**Requirements**: Cross-platform validation sign-off
**Success Criteria** (what must be TRUE):
  1. ~~Wayland clipboard fallback~~ — completed via PR #48
  2. Cross-platform testing on Windows 10+, macOS 12+, Linux (with Wayland and X11 variants)
  3. All integration tests pass (Docker, FAT32, audit, etc.)
  4. Release artifacts are tested on actual hardware/VMs
  5. Release notes are written summarizing v0.9.5 improvements
  6. Go-ahead decision made for v1.0 release
**Plans**: 2 plans

Plans:

- [ ] 12.4-01-PLAN.md -- Wayland clipboard fallback, cross-platform validation, release readiness

### Phase 12.5: Fix audit validation — store hash values independently

**Goal**: Audit integrity verification correctly detects real tampering by storing expected hash values in the vault at submission time, not fetching them from ScalarDL.
**Depends on**: Phase 4.3 (collection-based audit storage)
**Milestone**: v0.9.5 -- Polish and validation
**Requirements**: GitHub issue #67
**Success Criteria** (what must be TRUE):
  1. At submission time, the expected hash value for each audit event is written to the vault, keyed by event ID
  2. At validation time, expected hash values are read from the vault and passed to `object.v1_0_0.Validate` instead of fetching them from ScalarDL
  3. Manually tampering with a record's `hash_value` in the ScalarDL PostgreSQL database causes the GUI to show the red "Tamper detected" banner
  4. Credentials whose records have not been tampered with continue to show "Integrity verified"
  5. Vault-side audit record store is zeroed from memory after use
**Plans**: 2 plans

Plans:

- [x] 12.5-01-PLAN.md -- Add AuditHashes to vault model, update Validate/Submit signatures, update VerifyAll/VerifyByLabelHash
- [ ] 12.5-02-PLAN.md -- Wire CLI, GUI, TUI callers with vault hashes, EventBuilder callbacks, memory zeroing

### Phase 12.6: Expand audit logging to vault lifecycle and credential management events

**Goal**: The audit log captures vault unlock/lock events and credential create/edit/delete events in addition to authentication operations, providing a complete tamper-evident audit trail.
**Depends on**: Phase 4.3 (collection-based audit storage), Phase 12.5
**Milestone**: v0.9.5 -- Polish and validation
**Requirements**: GitHub issue #59
**Success Criteria** (what must be TRUE):
  1. Vault unlock and lock (including auto-lock) events are logged with timestamp and host machine fingerprint hash
  2. Credential created, edited, and deleted events are logged with timestamp, label hash, issuer hash, and host fingerprint hash
  3. All new events follow the existing privacy model (labels, service names, host identifiers are hashed)
  4. All new events are submitted to ScalarDL when audit is enabled, and queued locally when unreachable
  5. `tegata history` displays and can filter the new event types
**Plans**: 2 plans

Plans:

- [ ] 12.6-01-PLAN.md -- Extend AuthEvent struct and EventBuilder for lifecycle events; update history command

### Phase 12.7: Audit history display improvements

**Goal**: Audit history is sortable by column, filterable by operation type and date range, paginated for long lists, and hash values are click-to-reveal with auto-copy.
**Depends on**: Phase 4.3 (collection-based audit storage)
**Milestone**: v0.9.5 -- Polish and validation
**Requirements**: GitHub issue #24
**Success Criteria** (what must be TRUE):
  1. GUI audit panel supports column sorting (click header toggles asc/desc) and operation type dropdown filter and date range picker
  2. CLI supports `--sort`, `--order`, `--type`, and `--limit` flags on `tegata history`
  3. Long event lists are paginated or virtualized in GUI; scrollable in TUI; limited with `--limit` in CLI
  4. GUI: clicking a truncated hash reveals the full value and copies it to clipboard; TUI: Enter/`c` copies full hash with status message
**Plans**: 2 plans

Plans:

- [ ] 12.7-01-PLAN.md -- Column sorting, operation filter, date range, pagination, hash click-to-reveal across CLI/TUI/GUI

### Phase 12.8: Docker lifecycle and HMAC key hardening

**Goal**: Docker containers stop when the app closes (eliminating unattended audit server exposure), and the HMAC secret key is moved from plaintext disk files into the encrypted vault.
**Depends on**: Phase 7 (Docker audit setup)
**Milestone**: v0.9.6 -- Security hardening
**Requirements**: GitHub issues #52, #53
**Success Criteria** (what must be TRUE):
  1. Closing Tegata GUI stops all ScalarDL and PostgreSQL Docker containers
  2. HMAC secret key is no longer stored in `tegata.toml` or any plaintext file on disk
  3. HMAC secret key is stored in the encrypted vault and loaded into memory only when needed
  4. Existing audit functionality (submit, validate, history) continues to work correctly
  5. Issue #55 (PostgreSQL volume exposure) is resolved as a consequence of Docker stop-on-close
**Plans**: 2 plans

Plans:

- [ ] 12.8-01-PLAN.md -- Stop Docker containers on app close (GUI shutdown hook, OS signal handler)
- [ ] 12.8-02-PLAN.md -- Move HMAC secret key from plaintext tegata.toml into encrypted vault

### Phase 12.9: Audit server UX and PostgreSQL volume protection

**Goal**: Users are informed when the audit server is running after app close, and the PostgreSQL volume exposure window is eliminated (via Phase 12.8's Docker stop-on-close fix).
**Depends on**: Phase 12.8
**Milestone**: v0.9.6 -- Security hardening
**Requirements**: GitHub issues #54, #55
**Success Criteria** (what must be TRUE):
  1. When closing Tegata with the audit server enabled, the user sees a notification or status indicator that Docker containers are stopping
  2. If Docker stop fails, the user is warned that containers may still be running
  3. Issue #55 (PostgreSQL named volume with unencrypted audit data accessible while containers run) is closed as a consequence of Phase 12.8
**Plans**: 2 plans

Plans:

- [ ] 12.9-01-PLAN.md -- UX indicator for audit server shutdown state on app close

### Phase 13: Traceability cleanup

**Goal**: The REQUIREMENTS.md traceability table accurately reflects project status, with all completed requirements marked as such.
**Depends on**: Phase 10
**Milestone**: v1.0 -- Stable release
**Requirements**: DOCS-05
**Success Criteria** (what must be TRUE):
  1. All 20 stale "Pending" entries in the traceability table that correspond to implemented features are updated to "Complete"
  2. No requirement marked "Complete" lacks a corresponding implemented feature in the codebase
**Plans**: 2 plans

Plans:

- [ ] 13-01-PLAN.md -- Audit traceability table against codebase and update 20 stale Pending entries to Complete

## Progress

**Execution Order:**

Phases execute in numeric order: 1 → 2 → 3 → 4 → 4.1 → 5 → 6 → 4.2 → 4.3 → 7 → 8 → 9 → 10 → 11 → 12 → [v0.9 complete] → 12.1 → 12.2 → 12.3 → 12.4 → 12.5 → 12.6 → 12.7 → [v0.9.5 complete] → 12.8 → 12.9 → [v0.9.6 complete] → 13 → 14 → [v1.0 stable]

Note: Phase 3 and Phase 4 depend on Phase 2 but not on each other. Phases 11, 12 depend on Phase 10. Phases 12.1-12.7 are v0.9.5 insertions; phases 12.8-12.9 are v0.9.6 security hardening insertions. Phases 13-14 target v1.0 stable release.

| Phase                              | Plans Complete | Status      | Completed  |
|------------------------------------|----------------|-------------|------------|
| 1. Foundation                      | 2/2            | Complete    | 2026-03-16 |
| 2. Core authenticator              | 6/6            | Complete    | 2026-03-17 |
| 3. Enhancements and export         | 3/3            | Complete    | 2026-03-17 |
| 4. Audit layer                     | 3/3            | Complete    | 2026-03-18 |
| 4.1 Deploy HashStore               | 3/3            | Complete    | 2026-03-24 |
| 4.2 Audit in TUI/GUI              | 2/2            | Complete    | 2026-03-25 |
| 4.3 Collection storage             | 2/2            | Complete    | 2026-03-26 |
| 5. Terminal UI                     | 5/5            | Complete    | 2026-03-20 |
| 6. GUI, docs, and release          | 7/7            | Complete    | 2026-03-29 |
| 7. One-click Docker audit setup    | 6/6            | Complete    | 2026-03-29 |
| 8. Audit opt-in and auto_start     | 4/4            | Complete    | 2026-04-02 |
| 9. E2E Docker audit testing        | 1/1            | Complete    | 2026-04-04 |
| 10. Audit client TLS and refactor  | 2/2            | Complete    | 2026-04-15 |
| 11. macOS binary signing           | 1/1            | Complete    | 2026-04-12 |
| 12. ScalarDL 3.13 upgrade          | 1/1            | Complete    | 2026-04-14 |
| **v0.9 Security audit milestone complete** | — | — | — |
| 12.1 Release workflow testing      | 1/1            | Complete    | 2026-04-18 |
| 12.3 UI/UX audit and fixes         | 3/3            | Complete    | 2026-04-19 |
| 12.4 Pre-release validation        | 0/1            | Not started | -          |
| 12.5 Fix audit validation          | 1/2 | In Progress|  |
| 12.6 Audit lifecycle events        | 0/1            | Not started | -          |
| 12.7 Audit history improvements    | 0/1            | Not started | -          |
| **v0.9.5 Polish and validation milestone target** | — | — | — |
| 12.8 Docker lifecycle + HMAC key   | 0/2            | Not started | -          |
| 12.9 Audit server UX + Postgres    | 0/1            | Not started | -          |
| **v0.9.6 Security hardening milestone target** | — | — | — |
| 12.2 Screenshots and polish        | 0/1            | Not started | -          |
| 13. Traceability cleanup           | 0/1            | Not started | -          |
| 14. v1.0 Documentation and release | 0/1            | Not started | -          |
| **v1.0 Stable release milestone target** | — | — | — |
