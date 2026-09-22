# AI assistant configuration

This file contains configuration and guidelines for AI assistants when working on this repository. It follows the [AGENTS.md](https://agents.md) convention and is read directly by GitHub Copilot; Claude Code reads it via the `@AGENTS.md` import in `CLAUDE.md`.

## Important notes

**GIT SAFETY:** NEVER force push to any branch. Force pushes can:

- Overwrite remote history and cause data loss.
- Accidentally commit files that should be in .gitignore.
- Invalidate pull requests and break collaboration.

Always use `git push` without `--force` or `--force-with-lease`. If you need to undo work, use `git revert` instead of rewriting published history.

## Documentation references

For detailed information about this project, refer to the following internal documentation.

- **Repository overview:** See `README.md`.
- **Product, design, and architecture docs:** See `README.md`, `CONTRIBUTING.md`, `admin/build-and-release.md`, and the documentation under `site/docs/`.

## Repository overview

Tegata is a cross-platform portable authenticator that stores 2FA codes and credentials in an encrypted vault on USB drives or microSD cards.

### Key components

The main components are a Go CLI and terminal UI, a Wails desktop GUI with React frontend, encrypted vault storage, TOTP/HOTP and challenge-response implementations, removable-drive handling, and optional ScalarDL audit logging.

### Architecture patterns

Preserve AES-256-GCM authenticated encryption, Argon2id parameters, memguard handling, atomic vault writes, recovery semantics, RFC-compliant OTP behavior, and offline-first operation. The optional ledger is an audit trail, not a substitute for vault security.

## Development workflow

Follow these guidelines to maintain code quality and project consistency.

### Build, test, and validation commands

- **Build:** `make build`
- **Tests:** `make test`
- **Lint:** `make lint`
- **GUI build:** `make gui`

### Issue creation guidelines

When creating a GitHub issue, provide clear descriptions of bugs or feature requests including steps to reproduce, expected versus actual behavior, environment details, and relevant logs. For features, describe the user need and reference a stable requirement ID when the project tracks one.

Every issue must include a **Model** and **Review effort** annotation in the format described below. Never omit a section; write "N/A" explicitly when one does not apply.

### Model and effort selection

Every issue records a recommended **Model** and **Review effort** at creation time:

- "Model: Sonnet | Review effort: medium"
- "Model: Opus | Review effort: high"

Choose **Opus** with **high** effort for cryptography, key derivation, credential storage, memory clearing, OTP correctness, recovery, import/export, or audit-log integrity. Choose **Sonnet** with **low** or **medium** effort for well-scoped implementation work, UI flows, documentation, and routine tests. Prefer the higher tier when security, privacy, persisted-data correctness, or destructive behavior is uncertain.

Before starting work, check the issue description and use the recommended model and effort.

### Commit practices

- Write clear, descriptive commit messages that start with an uppercase verb.
- Do not use Conventional Commits format. Verb-first is the only style used in this repository.
- Reference issue numbers when commits relate to a specific issue.
- Avoid auto-committing large or non-trivial changes; always give the reviewer an opportunity to inspect changes before committing.
- **AI co-authorship:** When an AI agent authors or co-authors a commit, include `Co-Authored-By: [Agent Name] ([Model]) <[email used for attribution]>` using the official attribution identity when available.

### Branch management

- Create feature branches from `main` using `feature/description` or `fix/description`.
- Keep branches focused on single features or bug fixes.

### Pull request creation guidelines

When creating a pull request, **read and follow `.github/pull_request_template.md` exactly**.

1. Read the actual template every time.
2. Include every section in its original order.
3. Include every checklist item.
4. Do not omit or paraphrase sections.

Check `[x]` only when completed. Leave incomplete items as `[ ]`. If an item does not apply, leave it unchecked and append `not applicable – [brief reason]`. PR titles use the same uppercase verb-first convention as commits.

### Code standards

Follow idiomatic conventions for Go 1.25, Cobra, Bubble Tea, Wails, React, TypeScript, and Docusaurus. Preserve the repository’s established layering, naming, compatibility targets, accessibility requirements, and error-handling patterns. Do not suppress security, concurrency, type, lint, or compiler diagnostics without a documented reason.

## Privacy and security considerations

Credentials, passphrases, recovery material, decrypted vault data, and OTP seeds must never leave the device or appear in logs, telemetry, crash reports, clipboard history beyond documented timing, or test fixtures. Tegata has no analytics. Preserve local-only behavior and explicit opt-in for ScalarDL audit logging.

## Writing style

Follow the [Microsoft Writing Style Guide](https://learn.microsoft.com/en-us/style-guide/welcome/) for documentation, code comments, UI text, issue content, and pull request content. Repository-specific rules take precedence.

### Formatting rules

Add empty newlines after headings and before lists. Never stack headings; include explanatory text between them. End every file with a newline. Use sentence-style capitalization for headings and bold labels. Do not end headings with colons. Avoid excessive lists. Put label colons inside bold text, as in `**Item:**`, and align Markdown tables for source readability.
