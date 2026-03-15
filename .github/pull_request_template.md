## Summary

<!-- Brief description of what this PR accomplishes, starting with "This PR ..." -->

## Related issues or PRs

<!-- Use format: Resolves #123, Relates to #456. If none exist, write "N/A" -->
-

## Changes made

<!-- List specific changes with bullet points -->
-
-
-

## Technical implementation

<!-- Describe architectural decisions, patterns used, etc. -->
- **Approach:**
- **Key components modified:**
- **Dependencies added/removed:**
- **Design patterns used:**

## Testing performed

<!-- Mark completed testing with [x]. Add PR-specific items as needed. -->
- [ ] Builds successfully on Linux (amd64)
- [ ] Builds successfully on macOS (if applicable)
- [ ] Builds successfully on Windows (if applicable)
- [ ] Unit tests pass
- [ ] Integration tests pass (if applicable)
- [ ] CLI commands tested manually with expected inputs
- [ ] Edge cases and error handling tested (invalid inputs, missing files, permission errors)
- [ ] Cryptographic operations verified (if applicable)
- [ ] ScalarDL integration tested (if applicable)
- [ ] Cross-platform compatibility verified (if applicable)

## Security considerations

<!-- Mark applicable items with [x]. Add PR-specific security notes. -->
- [ ] No sensitive data (keys, passwords, secrets) in code or tests
- [ ] Cryptographic operations use secure, well-tested libraries
- [ ] Memory is zeroed after handling sensitive data
- [ ] Input validation implemented for all user-provided data
- [ ] No SQL injection, command injection, or path traversal vulnerabilities
- [ ] Error messages don't leak sensitive information
- [ ] Vault files remain encrypted and properly protected
- [ ] No new dependencies with known vulnerabilities
- [ ] Authentication/authorization logic is sound (if applicable)

## Code quality

<!-- Verify these items -->
- [ ] Code follows project conventions (Go style guidelines)
- [ ] Added appropriate comments for complex cryptographic or security logic
- [ ] No hardcoded secrets or configuration values
- [ ] Proper error handling and user-friendly error messages
- [ ] No debugging code or print/println statements left in
- [ ] Removed unused imports, variables, and functions
- [ ] Dependencies pinned to specific versions
- [ ] Documentation updated (README, user guides, API docs if applicable)

## Breaking changes

<!-- Does this PR introduce breaking changes to the CLI API, vault format, or configuration? -->
- [ ] No breaking changes
- [ ] Breaking changes (describe below):

## Additional context

<!-- Any extra information for reviewers, screenshots, performance notes, etc. -->
