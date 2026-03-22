package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/josh-wong/tegata/internal/auth"
	pkgmodel "github.com/josh-wong/tegata/pkg/model"
)

// credTypeNames maps addTypeIdx to display labels and CredentialType constants.
var credTypeNames = []struct {
	label string
	ctype pkgmodel.CredentialType
}{
	{"TOTP", pkgmodel.CredentialTOTP},
	{"HOTP", pkgmodel.CredentialHOTP},
	{"Static", pkgmodel.CredentialStatic},
	{"Challenge-Resp", pkgmodel.CredentialChallengeResponse},
}

// addAlgoLabels and addAlgoValues map addAlgoIdx to display labels and
// credential algorithm strings.
var addAlgoLabels = []string{"SHA-1", "SHA-256", "SHA-512"}
var addAlgoValues = []string{"SHA1", "SHA256", "SHA512"}

// addDigitValues maps addDigitsIdx to digit counts.
var addDigitValues = []int{6, 8}

// Focus slot constants for the add overlay's unified focus model. Tab cycles
// through visible slots; selector slots respond to Left/Right arrows.
const (
	addSlotLabel     = 0
	addSlotIssuer    = 1
	addSlotType      = 2
	addSlotSecret    = 3
	addSlotAlgorithm = 4
	addSlotDigits    = 5
	addSlotPeriod    = 6
	addSlotTags      = 7
)

// resetAddOverlay clears all add-overlay input fields and resets indices.
func (m *model) resetAddOverlay() {
	m.addLabelInput.Reset()
	m.addLabelInput.Blur()
	m.addIssuerInput.Reset()
	m.addIssuerInput.Blur()
	m.addSecretInput.Reset()
	m.addSecretInput.Blur()
	m.addPeriodInput.Reset()
	m.addPeriodInput.Blur()
	m.addTagsInput.Reset()
	m.addTagsInput.Blur()
	m.addTypeIdx = 0
	m.addAlgoIdx = 0
	m.addDigitsIdx = 0
	m.addFocusIdx = 0
	m.errMsg = ""
	m.updateSecretPlaceholder()
}

// updateSecretPlaceholder sets the secret input placeholder text based on the
// current credential type.
func (m *model) updateSecretPlaceholder() {
	switch credTypeNames[m.addTypeIdx].ctype {
	case pkgmodel.CredentialStatic:
		m.addSecretInput.Placeholder = "Password"
	case pkgmodel.CredentialChallengeResponse:
		m.addSecretInput.Placeholder = "Shared secret key"
	default:
		m.addSecretInput.Placeholder = "Secret (base32)"
	}
}

// addVisibleSlots returns the ordered list of focus slot indices that are
// visible for the current credential type.
func (m model) addVisibleSlots() []int {
	ct := credTypeNames[m.addTypeIdx].ctype
	slots := []int{addSlotLabel, addSlotIssuer, addSlotType, addSlotSecret}
	switch ct {
	case pkgmodel.CredentialTOTP:
		slots = append(slots, addSlotAlgorithm, addSlotDigits, addSlotPeriod)
	case pkgmodel.CredentialHOTP:
		slots = append(slots, addSlotAlgorithm, addSlotDigits)
	case pkgmodel.CredentialChallengeResponse:
		slots = append(slots, addSlotAlgorithm)
	}
	slots = append(slots, addSlotTags)
	return slots
}

// addNextSlot returns the next (forward=true) or previous (forward=false)
// visible focus slot index from the current position.
func (m model) addNextSlot(forward bool) int {
	slots := m.addVisibleSlots()
	cur := 0
	for i, s := range slots {
		if s == m.addFocusIdx {
			cur = i
			break
		}
	}
	if forward {
		return slots[(cur+1)%len(slots)]
	}
	return slots[(cur+len(slots)-1)%len(slots)]
}

// clampAddFocus ensures addFocusIdx points to a visible slot. If the current
// slot became invisible (e.g., after changing the credential type), it snaps
// to the nearest preceding visible slot.
func (m *model) clampAddFocus() {
	slots := m.addVisibleSlots()
	for _, s := range slots {
		if s == m.addFocusIdx {
			return
		}
	}
	best := slots[0]
	for _, s := range slots {
		if s <= m.addFocusIdx {
			best = s
		}
	}
	m.addFocusIdx = best
	m.focusAddInput()
}

// focusAddInput blurs all add text inputs, then focuses the one corresponding
// to addFocusIdx. Selector slots (Type, Algorithm, Digits) have no text input
// to focus — all inputs stay blurred so the user sees visual highlighting only.
func (m *model) focusAddInput() {
	m.addLabelInput.Blur()
	m.addIssuerInput.Blur()
	m.addSecretInput.Blur()
	m.addPeriodInput.Blur()
	m.addTagsInput.Blur()
	switch m.addFocusIdx {
	case addSlotLabel:
		m.addLabelInput.Focus()
	case addSlotIssuer:
		m.addIssuerInput.Focus()
	case addSlotSecret:
		m.addSecretInput.Focus()
	case addSlotPeriod:
		m.addPeriodInput.Focus()
	case addSlotTags:
		m.addTagsInput.Focus()
	}
}

// updateOverlayAdd handles key events in stateOverlayAdd.
func (m model) updateOverlayAdd(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.resetAddOverlay()
			m.state = stateMainView
			return m, nil

		case tea.KeyTab:
			m.addFocusIdx = m.addNextSlot(true)
			m.focusAddInput()
			return m, nil

		case tea.KeyShiftTab:
			m.addFocusIdx = m.addNextSlot(false)
			m.focusAddInput()
			return m, nil

		case tea.KeyLeft, tea.KeyRight:
			delta := 1
			if msg.Type == tea.KeyLeft {
				delta = -1
			}
			switch m.addFocusIdx {
			case addSlotType:
				m.addTypeIdx = (m.addTypeIdx + delta + len(credTypeNames)) % len(credTypeNames)
				m.updateSecretPlaceholder()
				m.clampAddFocus()
				return m, nil
			case addSlotAlgorithm:
				m.addAlgoIdx = (m.addAlgoIdx + delta + len(addAlgoValues)) % len(addAlgoValues)
				return m, nil
			case addSlotDigits:
				m.addDigitsIdx = (m.addDigitsIdx + delta + len(addDigitValues)) % len(addDigitValues)
				return m, nil
			}

		case tea.KeyEnter:
			labelVal := m.addLabelInput.Value()

			// URI auto-populate: if label starts with "otpauth://", parse and fill fields.
			if strings.HasPrefix(labelVal, "otpauth://") {
				cred, err := auth.ParseOTPAuthURI(labelVal)
				if err != nil {
					m.errMsg = fmt.Sprintf("Invalid URI: %v", err)
					return m, nil
				}
				m.addLabelInput.SetValue(cred.Label)
				m.addIssuerInput.SetValue(cred.Issuer)
				m.addSecretInput.SetValue(cred.Secret)
				for i, ct := range credTypeNames {
					if ct.ctype == cred.Type {
						m.addTypeIdx = i
						break
					}
				}
				for i, a := range addAlgoValues {
					if strings.EqualFold(a, cred.Algorithm) {
						m.addAlgoIdx = i
						break
					}
				}
				for i, d := range addDigitValues {
					if d == cred.Digits {
						m.addDigitsIdx = i
						break
					}
				}
				if cred.Period > 0 {
					m.addPeriodInput.SetValue(strconv.Itoa(cred.Period))
				}
				m.updateSecretPlaceholder()
				m.errMsg = ""
				return m, nil
			}

			// Validate required fields.
			if labelVal == "" || m.addSecretInput.Value() == "" {
				m.errMsg = "Label and secret are required"
				return m, nil
			}

			// Validate base32 encoding for TOTP and HOTP secrets.
			ct := credTypeNames[m.addTypeIdx]
			switch ct.ctype {
			case pkgmodel.CredentialTOTP, pkgmodel.CredentialHOTP:
				if _, err := decodeBase32Secret(m.addSecretInput.Value()); err != nil {
					m.errMsg = "Secret is not valid base32 — TOTP and HOTP secrets use characters A-Z and 2-7 only"
					return m, nil
				}
			}

			// Read algorithm and digits from selectors.
			algo := addAlgoValues[m.addAlgoIdx]
			digits := addDigitValues[m.addDigitsIdx]

			// Parse period for TOTP credentials.
			period := 30
			if ct.ctype == pkgmodel.CredentialTOTP {
				if v := strings.TrimSpace(m.addPeriodInput.Value()); v != "" {
					p, err := strconv.Atoi(v)
					if err != nil || p < 15 || p > 120 {
						m.errMsg = "Period must be between 15 and 120 seconds"
						return m, nil
					}
					period = p
				}
			}

			// Parse comma-separated tags.
			var tags []string
			if raw := strings.TrimSpace(m.addTagsInput.Value()); raw != "" {
				for _, t := range strings.Split(raw, ",") {
					if t = strings.TrimSpace(t); t != "" {
						tags = append(tags, t)
					}
				}
			}

			// Build credential from inputs.
			cred := pkgmodel.Credential{
				Label:     labelVal,
				Issuer:    m.addIssuerInput.Value(),
				Type:      ct.ctype,
				Secret:    m.addSecretInput.Value(),
				Algorithm: algo,
				Digits:    digits,
				Period:    period,
				Tags:      tags,
			}

			if m.vaultMgr == nil {
				m.errMsg = "Vault not unlocked"
				return m, nil
			}

			if _, err := m.vaultMgr.AddCredential(cred); err != nil {
				m.errMsg = fmt.Sprintf("Add failed: %v", err)
				return m, nil
			}

			m = refreshCredList(m, labelVal)

			label := labelVal
			m.resetAddOverlay()
			m.state = stateMainView
			m.statusMsg = fmt.Sprintf("Added %q", label)
			return m, nil
		}
	}

	// Delegate to the focused text input. Selector slots (2, 4, 5) have no
	// text input — key events are silently dropped for those.
	var cmd tea.Cmd
	switch m.addFocusIdx {
	case addSlotLabel:
		m.addLabelInput, cmd = m.addLabelInput.Update(msg)
	case addSlotIssuer:
		m.addIssuerInput, cmd = m.addIssuerInput.Update(msg)
	case addSlotSecret:
		m.addSecretInput, cmd = m.addSecretInput.Update(msg)
	case addSlotPeriod:
		m.addPeriodInput, cmd = m.addPeriodInput.Update(msg)
	case addSlotTags:
		m.addTagsInput, cmd = m.addTagsInput.Update(msg)
	}
	return m, cmd
}

// addLabelWidth is the column width for field labels in the add overlay.
const addLabelWidth = 13

// viewOverlayAdd renders the add-credential overlay.
func (m model) viewOverlayAdd() string {
	ct := credTypeNames[m.addTypeIdx]
	var lines []string
	lines = append(lines, titleStyle.Render("Add Credential"))
	lines = append(lines, "")

	lines = append(lines, fmt.Sprintf("%-*s%s", addLabelWidth, "Label:", m.addLabelInput.View()))
	lines = append(lines, fmt.Sprintf("%-*s%s %s", addLabelWidth, "Issuer:", m.addIssuerInput.View(), helpBarStyle.Render("(optional)")))

	// Type selector row.
	lines = append(lines, renderAddSelector("Type:", addSlotType, m.addFocusIdx, m.addTypeIdx, credTypeDisplayLabels()))

	// Secret row with type-dependent label.
	var secretLabel string
	switch ct.ctype {
	case pkgmodel.CredentialStatic:
		secretLabel = "Password:"
	case pkgmodel.CredentialChallengeResponse:
		secretLabel = "Shared secret key:"
	default:
		secretLabel = "Secret:"
	}
	lines = append(lines, fmt.Sprintf("%-*s%s", addLabelWidth, secretLabel, m.addSecretInput.View()))

	// Algorithm selector — hidden for Static.
	if ct.ctype != pkgmodel.CredentialStatic {
		lines = append(lines, renderAddSelector("Algorithm:", addSlotAlgorithm, m.addFocusIdx, m.addAlgoIdx, addAlgoLabels))
	}

	// Digits selector — TOTP and HOTP only.
	if ct.ctype == pkgmodel.CredentialTOTP || ct.ctype == pkgmodel.CredentialHOTP {
		lines = append(lines, renderAddSelector("Digits:", addSlotDigits, m.addFocusIdx, m.addDigitsIdx, []string{"6", "8"}))
	}

	// Period text input — TOTP only.
	if ct.ctype == pkgmodel.CredentialTOTP {
		lines = append(lines, fmt.Sprintf("%-*s%s %s", addLabelWidth, "Period:", m.addPeriodInput.View(), helpBarStyle.Render("seconds")))
	}

	// Tags text input.
	lines = append(lines, fmt.Sprintf("%-*s%s %s", addLabelWidth, "Tags:", m.addTagsInput.View(), helpBarStyle.Render("(optional)")))

	if m.errMsg != "" {
		lines = append(lines, "")
		lines = append(lines, errorStyle.Render(m.errMsg))
	}

	lines = append(lines, "")
	lines = append(lines, helpBarStyle.Render("[Tab] Navigate  [\u2190/\u2192] Change  [Enter] Save  [Esc] Cancel"))

	content := strings.Join(lines, "\n")
	overlay := overlayBoxStyle.Render(content)
	return overlayOnBackground(overlay, m.width, m.height)
}

// credTypeDisplayLabels returns the display labels for credential types.
func credTypeDisplayLabels() []string {
	labels := make([]string, len(credTypeNames))
	for i, ct := range credTypeNames {
		labels[i] = ct.label
	}
	return labels
}

// renderAddSelector renders a label + selectable options row. The selected
// option is highlighted in green. When the selector has focus, left/right
// arrows flank the selected option.
func renderAddSelector(label string, slot, focusIdx, selectedIdx int, options []string) string {
	focused := focusIdx == slot
	var parts []string
	for i, opt := range options {
		if i == selectedIdx {
			if focused {
				parts = append(parts, successStyle.Render("\u2190 "+opt+" \u2192"))
			} else {
				parts = append(parts, successStyle.Render(opt))
			}
		} else {
			parts = append(parts, opt)
		}
	}
	return fmt.Sprintf("%-*s%s", addLabelWidth, label, strings.Join(parts, "  "))
}

// updateOverlayRemove handles key events in stateOverlayRemove.
func (m model) updateOverlayRemove(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyEsc ||
			(len(msg.Runes) == 1 && msg.Runes[0] == 'n'):
			m.state = stateMainView
			return m, nil

		case len(msg.Runes) == 1 && msg.Runes[0] == 'y':
			selected := m.credList.SelectedItem()
			if selected == nil {
				m.state = stateMainView
				return m, nil
			}
			item, ok := selected.(credItem)
			if !ok {
				m.state = stateMainView
				return m, nil
			}

			if m.vaultMgr != nil {
				if err := m.vaultMgr.RemoveCredential(item.cred.ID); err != nil {
					m.errMsg = fmt.Sprintf("Remove failed: %v", err)
					m.state = stateMainView
					return m, nil
				}
			}

			m = refreshCredList(m)
			m.state = stateMainView
			m.statusMsg = "Removed"
			return m, nil
		}
	}
	return m, nil
}

// viewOverlayRemove renders the remove-confirmation overlay.
func (m model) viewOverlayRemove() string {
	label := "(none selected)"
	if selected := m.credList.SelectedItem(); selected != nil {
		if item, ok := selected.(credItem); ok {
			label = item.cred.Label
		}
	}

	var lines []string
	lines = append(lines, titleStyle.Render("Remove credential?"))
	lines = append(lines, "")
	lines = append(lines, "Credential: "+label)
	lines = append(lines, "")
	lines = append(lines, helpBarStyle.Render("[y] Remove  [n] Cancel"))

	content := strings.Join(lines, "\n")
	overlay := overlayBoxStyle.Render(content)
	return overlayOnBackground(overlay, m.width, m.height)
}

// refreshCredList rebuilds the credential list from the vault manager,
// sorted alphabetically by label. The list selection moves to the item
// matching selectLabel (if non-empty), or resets to the top.
func refreshCredList(m model, selectLabel ...string) model {
	if m.vaultMgr == nil {
		return m
	}
	creds := m.vaultMgr.ListCredentials()
	sort.Slice(creds, func(i, j int) bool {
		return strings.ToLower(creds[i].Label) < strings.ToLower(creds[j].Label)
	})
	items := make([]list.Item, 0, len(creds))
	selectedIdx := 0
	for i, c := range creds {
		items = append(items, credItem{cred: c})
		if len(selectLabel) > 0 && c.Label == selectLabel[0] {
			selectedIdx = i
		}
	}
	m.credList.SetItems(items)
	m.credList.Select(selectedIdx)
	m.cursor = selectedIdx
	if len(creds) == 1 {
		m.credList.Title = "1 credential"
	} else if len(creds) > 1 {
		m.credList.Title = fmt.Sprintf("%d credentials", len(creds))
	} else {
		m.credList.Title = "No credentials"
	}
	return m
}

// overlayOnBackground places an overlay box centered on the screen.
func overlayOnBackground(overlay string, width, height int) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, overlay)
}
