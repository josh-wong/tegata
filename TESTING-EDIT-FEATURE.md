# Manual Testing Guide: Credential Editing Feature (Issue #83)

This document outlines the manual testing steps required to verify the credential editing functionality across all interfaces.

## Prerequisites

- Build the project: `make build`
- Create a test vault with at least 3 test credentials
- Have audit logging enabled (optional but recommended for full verification)

---

## TUI (Terminal User Interface) Testing

### Setup
1. Run `./bin/tegata ui`
2. Unlock the vault
3. Verify the main view displays credentials in the sidebar
4. Confirm the help bar shows the new `e Edit` keybinding

### Basic Edit Overlay Opening
- [ ] Press `e` on a selected credential → Edit overlay should open
- [ ] Overlay should display "Edit Credential" title
- [ ] Label field should be pre-populated with current label
- [ ] Issuer field should be pre-populated with current issuer (or empty)
- [ ] Tags field should be pre-populated with comma-separated tags (or empty)
- [ ] Focus starts on Label field (highlighted)

### Field Navigation
- [ ] Press Tab → Focus moves to next field (Issuer)
- [ ] Press Tab again → Focus moves to Tags
- [ ] Press Tab again → Focus wraps back to Label
- [ ] Press Shift-Tab → Focus moves backward through fields

### Editing Label
- [ ] Clear label field and type new name (e.g., "github-personal")
- [ ] Press Enter → Credential should be updated with new label
- [ ] Credential list should refresh and show new label
- [ ] Status message should show "Updated 'github-personal'"
- [ ] Return to main view

### Editing Issuer
- [ ] Press `e` on a credential
- [ ] Press Tab to move to Issuer field
- [ ] Change issuer (e.g., "GitHub Inc")
- [ ] Press Enter → Credential updated with new issuer
- [ ] Return to main view and verify issuer shown in detail panel

### Editing Tags
- [ ] Press `e` on a credential without tags
- [ ] Press Tab twice to reach Tags field
- [ ] Type comma-separated tags: "work, personal, 2fa"
- [ ] Press Enter → Tags should be added
- [ ] Edit credential again → Tags should appear in Tags field as "work, personal, 2fa"

### Tag Validation - Duplicate Tags
- [ ] Press `e` on a credential
- [ ] Navigate to Tags field
- [ ] Type tags with duplicates: "work, personal, work"
- [ ] Press Enter → Error message should appear: "Duplicate tag: 'work'"
- [ ] Overlay should remain open for correction
- [ ] Change to "work, personal" and press Enter → Should succeed

### Tag Validation - Empty Strings
- [ ] Press `e` on a credential
- [ ] Navigate to Tags field
- [ ] Type tags with extra spaces: "work,  , personal"
- [ ] Press Enter → Empty strings should be silently filtered
- [ ] Tags should be saved as "work, personal"

### Label Validation - Required Field
- [ ] Press `e` on a credential
- [ ] Clear the Label field
- [ ] Press Enter → Error message should appear: "Label is required"
- [ ] Overlay should remain open

### Label Validation - Duplicate Labels
- [ ] Have two credentials: "github" and "gitlab"
- [ ] Press `e` on "github"
- [ ] Change label to "gitlab"
- [ ] Press Enter → Error message should appear: "A credential with label 'gitlab' already exists"
- [ ] Overlay should remain open

### Cancel / Esc
- [ ] Press `e` on a credential
- [ ] Make changes to label/issuer/tags
- [ ] Press Esc → Overlay should close without saving
- [ ] Return to main view → Credential should be unchanged
- [ ] Edit credential again → Fields should show original values

### Complex Edit Scenario
- [ ] Press `e` on a credential
- [ ] Change Label: "old-label" → "new-label"
- [ ] Change Issuer: empty → "New Corp"
- [ ] Change Tags: empty → "work, important"
- [ ] Press Enter → All changes should save
- [ ] Status should show "Updated 'new-label'"
- [ ] Verify in credential list and detail panel

### Idle Lock During Edit
- [ ] Press `e` on a credential to open edit overlay
- [ ] Wait for idle timeout (default 5 minutes, can be reduced in config)
- [ ] Vault should lock automatically
- [ ] UI should return to unlock screen
- [ ] No credentials should remain visible

---

## CLI Testing

### Setup
Ensure you have a test vault created with credentials.

### Basic Edit Command
```bash
./bin/tegata edit github --issuer "GitHub Inc"
```
- [ ] Prompts for vault passphrase
- [ ] Should show: "Updated 'github'"
- [ ] Should show: "Tags: (none)" or list of tags
- [ ] Command should exit successfully

### Edit Label
```bash
./bin/tegata edit github --label github-personal
```
- [ ] Credential should be renamed to "github-personal"
- [ ] Output should confirm update
- [ ] Verify with `./bin/tegata list` that label changed

### Edit Tags
```bash
./bin/tegata edit github --tags "work, personal, 2fa"
```
- [ ] Credential should have new tags
- [ ] Output should show: "Tags: work, personal, 2fa"

### Replace Tags (Remove All)
```bash
./bin/tegata edit github --tags ""
```
- [ ] All tags should be removed
- [ ] Output should show: "Tags: (none)"

### Combined Edit
```bash
./bin/tegata edit github --label github-work --issuer "GitHub" --tags "work, enterprise"
```
- [ ] Label should change
- [ ] Issuer should change
- [ ] Tags should change
- [ ] Output should confirm all updates

### Validation - No Flags Error
```bash
./bin/tegata edit github
```
- [ ] Should error: "at least one of --label, --issuer, or --tags must be provided"
- [ ] Command should exit with non-zero status

### Validation - Duplicate Label Error
```bash
./bin/tegata edit github --label gitlab
# (assuming gitlab already exists)
```
- [ ] Should error: "a credential with label 'gitlab' already exists"
- [ ] Command should exit with non-zero status

### Validation - Duplicate Tags Error
```bash
./bin/tegata edit github --tags "work, work, personal"
```
- [ ] Should error: "duplicate tag 'work'"
- [ ] Command should exit with non-zero status

### Validation - Non-existent Credential
```bash
./bin/tegata edit non-existent
```
- [ ] Should error (credential not found)
- [ ] Command should exit with non-zero status

### Vault Flag
```bash
./bin/tegata edit github --vault /path/to/vault.tegata --issuer "New"
```
- [ ] Should work with explicit vault path
- [ ] Should update credential in specified vault

### Verbose Logging
```bash
./bin/tegata edit github --issuer "New" -v
```
- [ ] Should produce debug output to stderr
- [ ] Command should still succeed

---

## GUI Testing

### Prerequisites
- Build GUI: `cd cmd/tegata-gui && wails build` (or `wails dev` for development)
- Launch the GUI application
- Unlock the vault

### Edit Button/Keybinding Visibility
- [ ] Verify credentials are displayed in the credentials list
- [ ] Check if there's an "Edit" button or keybinding hint for each credential

### Open Edit Dialog
- [ ] Click Edit button or use appropriate keybinding for a credential
- [ ] Edit dialog should open
- [ ] Dialog should display "Edit Credential" or similar title
- [ ] Three input fields should be visible: Label, Issuer, Tags

### Pre-population
- [ ] Label field should show current credential label
- [ ] Issuer field should show current issuer (or be empty)
- [ ] Tags field should show comma-separated tags (or be empty)

### Edit Label in GUI
- [ ] Clear label field and type new label
- [ ] Click Save/OK button
- [ ] Credential should update in the list
- [ ] New label should be immediately visible

### Edit Issuer in GUI
- [ ] Navigate to Issuer field
- [ ] Update issuer
- [ ] Save → Should persist
- [ ] Credential detail should show new issuer

### Edit Tags in GUI
- [ ] Navigate to Tags field
- [ ] Add tags: "work, personal"
- [ ] Save → Tags should be added
- [ ] Re-open edit dialog → Tags should appear in field

### Cancel/Discard Changes
- [ ] Open edit dialog
- [ ] Make changes
- [ ] Click Cancel/Close button
- [ ] Dialog should close without saving
- [ ] Credential should remain unchanged

### Validation Messages
- [ ] Clear label and try to save → Error message for empty label
- [ ] Try to change label to existing credential label → Error message for duplicate
- [ ] Add duplicate tags → Error message for duplicates
- [ ] Try to save without changes → Should succeed (no-op)

### Dialog Responsiveness
- [ ] Edit dialog should be responsive and usable on different window sizes
- [ ] All fields should be visible and clickable
- [ ] Buttons should be accessible

---

## Audit Logging Verification

### Prerequisites
- Enable audit logging in config
- Have a ScalarDL Ledger instance running (optional, or queue offline events)

### Audit Event Type Selection
1. Edit only tags on a credential
   - [ ] Should log `credential-tag-update` event
   - [ ] Event should reference the label and issuer

2. Edit label or issuer (with or without tags)
   - [ ] Should log `credential-update` event
   - [ ] Event should reference the new label and issuer

3. Edit multiple fields at once
   - [ ] Should log `credential-update` event (not `credential-tag-update`)

### Verify Audit Events
```bash
./bin/tegata history
```
- [ ] Recent edit events should appear in audit history
- [ ] Event types should be correct (credential-update vs credential-tag-update)
- [ ] Labels should be shown (if audit logging configured correctly)

### Offline Queue
- [ ] Stop ledger server
- [ ] Edit a credential
- [ ] Event should be queued locally
- [ ] Restart ledger
- [ ] Events should be flushed to ledger
- [ ] History should reflect the updates

---

## Cross-Interface Consistency

### Edit in TUI, Verify in CLI
1. Edit credential label in TUI
2. Run `./bin/tegata list` in CLI
3. [ ] Should show updated label

### Edit in CLI, Verify in TUI
1. Run `./bin/tegata edit github --issuer "New"`
2. Open UI in TUI
3. [ ] Credential should show new issuer

### Edit in GUI, Verify in Others
1. Edit credential in GUI
2. [ ] TUI should show changes
3. [ ] CLI list should show changes

---

## Edge Cases

### Special Characters in Tags
```bash
./bin/tegata edit github --tags "work-project, c++, node.js"
```
- [ ] Tags with hyphens, plus signs, and dots should save correctly
- [ ] Should appear in edit overlay as provided

### Whitespace Handling
- [ ] Tags: "  work  ,  personal  " → Should save as "work, personal"
- [ ] Label with trailing spaces should be trimmed/handled correctly

### Very Long Labels/Tags
- [ ] Edit credential with very long label (100+ characters)
- [ ] Should display and edit correctly in all interfaces
- [ ] Should persist and load correctly

### Rapid Successive Edits
- [ ] Edit credential 1, save
- [ ] Immediately edit credential 2, save
- [ ] Both should be saved correctly
- [ ] No data loss or corruption

### Unicode/Emoji Support
```bash
./bin/tegata edit github --tags "🔒-work, mötorhead"
```
- [ ] Unicode characters in tags should be preserved
- [ ] Should display correctly in all interfaces

---

## Sign-Off Checklist

- [ ] TUI editing works correctly
- [ ] CLI editing works correctly
- [ ] GUI editing works correctly (if applicable)
- [ ] All validation rules enforced
- [ ] Audit events logged correctly
- [ ] No data loss or corruption
- [ ] All three interfaces show consistent state
- [ ] No crashes or unexpected errors
- [ ] Documentation updated (if needed)

