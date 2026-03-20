# Contributing to Tegata

Thank you for your interest in contributing to Tegata. This document describes how to set up a development environment, follow project conventions, and submit changes.

## Development setup

Tegata is written in Go for the CLI/TUI and uses Wails with React/TypeScript for the optional desktop GUI.

### Prerequisites

The following tools are required for CLI and TUI development.

- **Go 1.25+** (check with `go version`)
- **Git**
- **Make**

For GUI development, you also need the following.

- **Wails CLI:** Install with `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- **Node.js 18+** (check with `node --version`)
- **Platform WebView dependencies:**
  - **Windows:** WebView2 runtime (included in Windows 10 1803+ and Windows 11)
  - **macOS:** WKWebView (built-in, no additional installation needed)
  - **Linux:** `libwebkit2gtk-4.0-dev` (install via your package manager)

### Clone and build

Clone the repository and run the standard build:

```bash
git clone https://github.com/josh-wong/tegata.git
cd tegata
make build
```

The binary is placed in `bin/tegata`. Run the test suite with:

```bash
make test
```

This runs `go test -race -count=1 ./...` to detect race conditions and ensure tests are not cached.

## Coding standards

These guidelines apply to all contributions.

### Go

Write idiomatic Go code formatted with `gofmt`. Handle errors at every call site and never use blank identifiers (`_`) to discard errors in production code paths. Use meaningful variable names that convey intent without unnecessary abbreviation.

### Security

Security is the top constraint in this project, not a feature to add later.

- **Zero sensitive memory** immediately after use. Every `[]byte` holding a passphrase, derived key, or plaintext credential must be zeroed with a `defer zeroBytes(...)` or explicit zeroing before the variable goes out of scope.
- **Never log secrets** or key material at any log level, including debug.
- **Validate inputs** at system boundaries. CLI flag values, IPC parameters, and deserialized data must be validated before use.
- **Use the guard wrapper.** All guarded memory allocations must go through `internal/crypto/guard`, never importing `memguard` directly.

### TypeScript and React (GUI)

Use TypeScript strict mode for all frontend code. Prefer functional components and hooks for state management. Clear sensitive state (such as passphrase input values) from React state immediately after passing to the backend.

### Testing

Add tests for new functionality. Run the full suite before submitting:

```bash
make test
```

Use the `-race` flag (already included in `make test`) to detect data races. For tests that require a vault, create a temporary vault in the test and clean it up afterward.

## Commit conventions

Write clear commit messages that describe the change, not the development process.

- Start with a **capitalized verb** (Add, Fix, Implement, Update, Remove)
- Do not use conventional commit prefixes (no `feat:`, `fix:`, `chore:`)
- Reference issue numbers when applicable (e.g., "Fix counter desync on HOTP resync #42")
- Keep commits focused on a single concern

**Examples of good commit messages:**

```
Add HOTP counter resynchronization with look-ahead window
Fix clipboard auto-clear race condition on WSL2
Update Argon2id parameters to match OWASP recommendations
Remove deprecated base32 validation path
```

## Pull request process

Follow these steps to submit a change.

1. **Fork** the repository and create a feature branch from `main`. Use the naming pattern `feature/description` or `fix/description`.
2. **Make your changes** following the coding standards above.
3. **Run the test suite** and confirm all tests pass: `make test`
4. **Write a clear PR description** that explains what changed, why it changed, and how to test the changes. Include the platforms you tested on.
5. **Submit the PR.** One approval is required for merge.

Keep PRs focused on a single feature or fix. Large PRs are harder to review and more likely to introduce unintended changes.

## Filing issues

When filing a bug report, include the following.

- Steps to reproduce the issue
- Expected behavior vs. actual behavior
- Platform and OS version (e.g., Windows 11 23H2, macOS 14.3, Ubuntu 24.04)
- Tegata version (`tegata version`)
- Relevant error output or logs (with `--verbose` flag if applicable)

For feature requests, describe the user need and your proposed solution. Explain why the feature fits within Tegata's scope as a portable authenticator.

## Code of conduct

Tegata is a small project built on mutual respect. Be constructive in code reviews, patient with newcomers, and focused on making the project better. Harassment, discrimination, and personal attacks are not tolerated.
