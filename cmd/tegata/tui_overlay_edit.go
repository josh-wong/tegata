package main

import (
	"fmt"
	"os"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/josh-wong/tegata/internal/audit"
	pkgmodel "github.com/josh-wong/tegata/pkg/model"
)

// Focus slot constants for the edit overlay's unified focus model.
const (
	editSlotLabel    = 0
	editSlotIssuer   = 1
	editSlotCategory = 2
	editSlotTags     = 3
)

// resetEditOverlay clears all edit-overlay input fields and resets indices.
func (m *model) resetEditOverlay() {
	m.editLabelInput.Reset()
	m.editLabelInput.Blur()
	m.editIssuerInput.Reset()
	m.editIssuerInput.Blur()
	m.editCategoryInput.Reset()
	m.editCategoryInput.Blur()
	m.editTagsInput.Reset()
	m.editTagsInput.Blur()
	m.editFocusIdx = 0
	m.editCredID = ""
	m.errMsg = ""
}

// loadEditOverlay pre-populates edit-overlay fields from a credential.
func (m *model) loadEditOverlay(cred pkgmodel.Credential) {
	m.resetEditOverlay()
	m.editLabelInput.SetValue(cred.Label)
	m.editIssuerInput.SetValue(cred.Issuer)
	m.editCategoryInput.SetValue(cred.Category)
	if len(cred.Tags) > 0 {
		m.editTagsInput.SetValue(strings.Join(cred.Tags, ", "))
	}
	m.editCredID = cred.ID
	m.editFocusIdx = 0
	m.focusEditInput()
}

// editVisibleSlots returns the ordered list of focus slot indices that are
// visible for the edit overlay. All slots are always visible.
func (m model) editVisibleSlots() []int {
	return []int{editSlotLabel, editSlotIssuer, editSlotCategory, editSlotTags}
}

// editNextSlot returns the next (forward=true) or previous (forward=false)
// visible focus slot index from the current position.
func (m model) editNextSlot(forward bool) int {
	slots := m.editVisibleSlots()
	cur := 0
	for i, s := range slots {
		if s == m.editFocusIdx {
			cur = i
			break
		}
	}
	if forward {
		return slots[(cur+1)%len(slots)]
	}
	return slots[(cur+len(slots)-1)%len(slots)]
}

// focusEditInput blurs all edit text inputs, then focuses the one corresponding
// to editFocusIdx.
func (m *model) focusEditInput() {
	m.editLabelInput.Blur()
	m.editIssuerInput.Blur()
	m.editCategoryInput.Blur()
	m.editTagsInput.Blur()
	switch m.editFocusIdx {
	case editSlotLabel:
		m.editLabelInput.Focus()
	case editSlotIssuer:
		m.editIssuerInput.Focus()
	case editSlotCategory:
		m.editCategoryInput.Focus()
	case editSlotTags:
		m.editTagsInput.Focus()
	}
}

// parseTags parses comma-separated tag input, trims whitespace, filters empty strings, and normalizes to lowercase.
func parseTags(raw string) []string {
	var tags []string
	if trimmed := strings.TrimSpace(raw); trimmed != "" {
		for _, t := range strings.Split(trimmed, ",") {
			if t = strings.TrimSpace(t); t != "" {
				tags = append(tags, strings.ToLower(t))
			}
		}
	}
	return tags
}

// hasDuplicateTags returns the first duplicate tag if found, or empty string.
func hasDuplicateTags(tags []string) string {
	seen := make(map[string]struct{})
	for _, tag := range tags {
		if _, exists := seen[tag]; exists {
			return tag
		}
		seen[tag] = struct{}{}
	}
	return ""
}

// updateOverlayEdit handles key events in stateOverlayEdit.
func (m model) updateOverlayEdit(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.resetEditOverlay()
			m.state = stateMainView
			return m, nil

		case tea.KeyTab:
			m.editFocusIdx = m.editNextSlot(true)
			m.focusEditInput()
			return m, nil

		case tea.KeyShiftTab:
			m.editFocusIdx = m.editNextSlot(false)
			m.focusEditInput()
			return m, nil

		case tea.KeyEnter:
			labelVal := strings.TrimSpace(m.editLabelInput.Value())
			issuerVal := m.editIssuerInput.Value()
			categoryVal := strings.ToLower(strings.TrimSpace(m.editCategoryInput.Value()))
			rawTags := m.editTagsInput.Value()

			// Validate label is not empty (after trimming whitespace).
			if labelVal == "" {
				m.errMsg = "Label is required"
				return m, nil
			}

			// Parse and validate tags.
			tags := parseTags(rawTags)
			if dup := hasDuplicateTags(tags); dup != "" {
				m.errMsg = fmt.Sprintf("Duplicate tag: %q", dup)
				return m, nil
			}

			// Get the original credential to check for duplicates and audit changes.
			if m.vaultMgr == nil {
				m.errMsg = "Vault not unlocked"
				return m, nil
			}

			// Find the credential being edited by ID.
			var originalCred pkgmodel.Credential
			found := false
			for _, c := range m.vaultMgr.ListCredentials() {
				if c.ID == m.editCredID {
					originalCred = c
					found = true
					break
				}
			}
			if !found {
				m.errMsg = "Credential not found"
				return m, nil
			}

			// Check for duplicate label if label changed.
			if labelVal != originalCred.Label {
				for _, c := range m.vaultMgr.ListCredentials() {
					if strings.EqualFold(c.Label, labelVal) && c.ID != m.editCredID {
						m.errMsg = fmt.Sprintf("A credential with label %q already exists", labelVal)
						return m, nil
					}
				}
			}

			// Build updated credential.
			updatedCred := originalCred
			updatedCred.Label = labelVal
			updatedCred.Issuer = issuerVal
			updatedCred.Category = categoryVal
			updatedCred.Tags = tags

			// Save to vault.
			if err := m.vaultMgr.UpdateCredential(&updatedCred); err != nil {
				m.errMsg = fmt.Sprintf("Update failed: %v", err)
				return m, nil
			}

			// Log one audit event per changed field.
			if m.builder != nil {
				type fieldEvent struct {
					changed bool
					opType  string
				}
				events := []fieldEvent{
					{labelVal != originalCred.Label, "credential-label-update"},
					{issuerVal != originalCred.Issuer, "credential-issuer-update"},
					{categoryVal != originalCred.Category, "credential-category-update"},
					{!slices.Equal(originalCred.Tags, tags), "credential-tag-update"},
				}
				for _, fe := range events {
					if fe.changed {
						if logErr := m.builder.LogEvent(fe.opType, labelVal, issuerVal, audit.Hostname(), true); logErr != nil {
							_, _ = fmt.Fprintf(os.Stderr, "Warning: Audit log failed: %v\n", logErr)
						}
					}
				}
			}

			// Refresh credential list and return to main view.
			m = refreshCredList(m, labelVal)
			m.resetEditOverlay()
			m.state = stateMainView
			m.statusMsg = fmt.Sprintf("Updated %q", labelVal)
			return m, nil
		}
	}

	// Delegate to the focused text input.
	var cmd tea.Cmd
	switch m.editFocusIdx {
	case editSlotLabel:
		m.editLabelInput, cmd = m.editLabelInput.Update(msg)
	case editSlotIssuer:
		m.editIssuerInput, cmd = m.editIssuerInput.Update(msg)
	case editSlotCategory:
		m.editCategoryInput, cmd = m.editCategoryInput.Update(msg)
	case editSlotTags:
		m.editTagsInput, cmd = m.editTagsInput.Update(msg)
	}
	return m, cmd
}

// viewOverlayEdit renders the edit-credential overlay.
func (m model) viewOverlayEdit() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Edit Credential"))
	lines = append(lines, "")

	const editLabelWidth = 10
	lines = append(lines, fmt.Sprintf("%-*s%s", editLabelWidth, "Label:", m.editLabelInput.View()))
	lines = append(lines, fmt.Sprintf("%-*s%s %s", editLabelWidth, "Issuer:", m.editIssuerInput.View(), helpBarStyle.Render("(optional)")))
	lines = append(lines, fmt.Sprintf("%-*s%s %s", editLabelWidth, "Category:", m.editCategoryInput.View(), helpBarStyle.Render("(optional)")))
	lines = append(lines, fmt.Sprintf("%-*s%s %s", editLabelWidth, "Tags:", m.editTagsInput.View(), helpBarStyle.Render("(optional)")))

	if m.errMsg != "" {
		lines = append(lines, "")
		lines = append(lines, errorStyle.Render(m.errMsg))
	}

	lines = append(lines, "")
	lines = append(lines, helpBarStyle.Render("[Tab] Navigate  [Enter] Save  [Esc] Cancel"))

	content := strings.Join(lines, "\n")
	overlay := overlayBoxStyle.Render(content)
	return overlayOnBackground(overlay, m.width, m.height)
}
