# CLI stability guarantees

Tegata follows a versioned stability policy to give users and automation authors confidence when building scripts or workflows around the CLI. Commands are classified as either stable or experimental.

## Stable commands

The following commands have stable interfaces. Their flags, output formats, and exit codes will not change in ways that break existing usage without a major version bump.

- `tegata init`
- `tegata add`
- `tegata list`
- `tegata code`
- `tegata get`
- `tegata remove`
- `tegata resync`
- `tegata sign`
- `tegata export`
- `tegata import`
- `tegata tag`
- `tegata change-passphrase`
- `tegata verify-recovery`
- `tegata config show`

## Experimental commands

The following commands are subject to change without notice and should not be relied upon in production scripts.

- `tegata bench` — performance benchmarking tool for tuning KDF parameters
- Any ledger or audit commands (planned for Phase 4+)

## Breaking-change policy

A breaking change is any modification to a stable command that would cause an existing valid invocation to fail, produce different output, or require updated flags. Breaking changes to stable commands require a major version bump (for example, `v1.x.x` to `v2.x.x`). Additive changes — new flags with non-breaking defaults, additional output fields — are not considered breaking and may be released in minor or patch versions.
