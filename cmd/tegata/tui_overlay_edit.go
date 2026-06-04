package main

import (
	"fmt"
	"os"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/i18n"
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
// Placeholders are refreshed here so they always reflect the active language.
func (m *model) resetEditOverlay() {
	m.editLabelInput.Reset()
	m.editLabelInput.Placeholder = i18n.T("tui.edit.placeholder.label")
	m.editLabelInput.Blur()
	m.editIssuerInput.Reset()
	m.editIssuerInput.Placeholder = i18n.T("tui.edit.placeholder.issuer")
	m.editIssuerInput.Blur()
	m.editCategoryInput.Reset()
	m.editCategoryInput.Placeholder = i18n.T("tui.edit.placeholder.category")
	m.editCategoryInput.Blur()
	m.editTagsInput.Reset()
	m.editTagsInput.Placeholder = i18n.T("tui.edit.placeholder.tags")
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
				m.errMsg = i18n.T("tui.edit.error.labelRequired")
				return m, nil
			}

			// Parse and validate tags.
			tags := parseTags(rawTags)
			if dup := hasDuplicateTags(tags); dup != "" {
				m.errMsg = i18n.Tf("tui.edit.error.duplicateTag", map[string]any{"Tag": dup})
				return m, nil
			}

			// Get the original credential to check for duplicates and audit changes.
			if m.vaultMgr == nil {
				m.errMsg = i18n.T("tui.edit.error.vaultLocked")
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
				m.errMsg = i18n.T("tui.edit.error.notFound")
				return m, nil
			}

			// Check for duplicate label if label changed.
			if labelVal != originalCred.Label {
				for _, c := range m.vaultMgr.ListCredentials() {
					if strings.EqualFold(c.Label, labelVal) && c.ID != m.editCredID {
						m.errMsg = i18n.Tf("tui.edit.error.labelExists", map[string]any{"Label": labelVal})
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
				m.errMsg = i18n.Tf("tui.edit.error.updateFailed", map[string]any{"Err": err})
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
							_, _ = fmt.Fprintf(os.Stderr, "%s", i18n.Tf("tui.edit.warn.auditFailed", map[string]any{"Err": logErr}))
						}
					}
				}
			}

			// Refresh credential list and return to main view.
			m = refreshCredList(m, labelVal)
			m.resetEditOverlay()
			m.state = stateMainView
			m.statusMsg = i18n.Tf("tui.edit.success", map[string]any{"Label": labelVal})
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
	lines = append(lines, titleStyle.Render(i18n.T("tui.edit.title")))
	lines = append(lines, "")

	labelL    := i18n.T("tui.edit.field.label")
	issuerL   := i18n.T("tui.edit.field.issuer")
	categoryL := i18n.T("tui.edit.field.category")
	tagsL     := i18n.T("tui.edit.field.tags")

	editCol := lipgloss.Width(labelL)
	for _, l := range []string{issuerL, categoryL, tagsL} {
		if w := lipgloss.Width(l); w > editCol {
			editCol = w
		}
	}
	editCol += 1
	editPad := func(label string) string {
		return label + strings.Repeat(" ", editCol-lipgloss.Width(label))
	}

	opt := helpBarStyle.Render(i18n.T("tui.edit.hint.optional"))
	lines = append(lines, editPad(labelL)+m.editLabelInput.View())
	lines = append(lines, editPad(issuerL)+m.editIssuerInput.View()+" "+opt)
	lines = append(lines, editPad(categoryL)+m.editCategoryInput.View()+" "+opt)
	lines = append(lines, editPad(tagsL)+m.editTagsInput.View()+" "+opt)

	if m.errMsg != "" {
		lines = append(lines, "")
		lines = append(lines, errorStyle.Render(m.errMsg))
	}

	lines = append(lines, "")
	lines = append(lines, helpBarStyle.Render(i18n.T("tui.edit.helpBar")))

	content := strings.Join(lines, "\n")
	overlay := overlayBoxStyle.Render(content)
	return overlayOnBackground(overlay, m.width, m.height)
}
