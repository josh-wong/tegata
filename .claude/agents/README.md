# Custom agents for Tegata

These Claude Code subagents encode project-specific workflows and constraints. Every agent must follow `AGENTS.md`.

## Available agents

- **builder:** Builds the project and diagnoses dependency or compiler failures.
- **lint-checker:** Runs the configured linter and reports actionable violations.
- **test-runner:** Runs the real test suites and analyzes failures.
- **debugger:** Diagnoses failures across the project’s load-bearing boundaries.
- **performance-profiler:** Profiles measured hot paths and resource use.
- **privacy-auditor:** Audits sensitive data and external data flows.
- **github-issue-creator:** Creates complete issues using the repository template.
- **pr-creator:** Creates pull requests using every template section and checklist item.

Invoke agents by name. They must report only commands and checks actually performed.
