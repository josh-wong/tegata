package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/auth"
	"github.com/josh-wong/tegata/internal/i18n"
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

// addAlgoValues maps addAlgoIdx to the algorithm strings stored in credentials.
var addAlgoValues = []string{"SHA1", "SHA256", "SHA512"}

// addDigitValues maps addDigitsIdx to digit counts.
var addDigitValues = []int{6, 8}

// algoIndexOf returns the index of algo in addAlgoValues, or 0 (SHA1) if not found.
func algoIndexOf(algo string) int {
	for i, a := range addAlgoValues {
		if a == algo {
			return i
		}
	}
	return 0
}

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
	addSlotCategory  = 8
)

// resetAddOverlay clears all add-overlay input fields and resets indices.
// Placeholders are refreshed here so they always reflect the active language,
// even when the user changes language from the TUI settings mid-session.
func (m *model) resetAddOverlay() {
	m.addLabelInput.Reset()
	m.addLabelInput.Placeholder = i18n.T("tui.add.placeholder.uri")
	m.addLabelInput.Blur()
	m.addIssuerInput.Reset()
	m.addIssuerInput.Placeholder = i18n.T("tui.add.placeholder.issuer")
	m.addIssuerInput.Blur()
	m.addSecretInput.Reset()
	m.addSecretInput.Blur()
	m.addPeriodInput.Reset()
	m.addPeriodInput.Placeholder = i18n.T("tui.add.placeholder.period")
	m.addPeriodInput.SetValue("30")
	m.addPeriodInput.Blur()
	m.addTagsInput.Reset()
	m.addTagsInput.Placeholder = i18n.T("tui.add.placeholder.tags")
	m.addTagsInput.Blur()
	m.addCategoryInput.Reset()
	m.addCategoryInput.Placeholder = i18n.T("tui.add.placeholder.category")
	m.addCategoryInput.Blur()
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
		m.addSecretInput.Placeholder = i18n.T("tui.add.placeholder.password")
	case pkgmodel.CredentialChallengeResponse:
		m.addSecretInput.Placeholder = i18n.T("tui.add.placeholder.sharedSecret")
	default:
		m.addSecretInput.Placeholder = i18n.T("tui.add.placeholder.secret")
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
		slots = append(slots, addSlotDigits)
	case pkgmodel.CredentialChallengeResponse:
		slots = append(slots, addSlotAlgorithm)
	}
	slots = append(slots, addSlotTags, addSlotCategory)
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
	m.addCategoryInput.Blur()
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
	case addSlotCategory:
		m.addCategoryInput.Focus()
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
				// Reset the algo selector to the type-appropriate default whenever
				// the user cycles the type. Any explicit algo choice made while on a
				// different type is intentionally discarded — defaults are per-type
				// and a stale selection from another type would silently mislead.
				if credTypeNames[m.addTypeIdx].ctype == pkgmodel.CredentialChallengeResponse {
					m.addAlgoIdx = algoIndexOf("SHA256")
				} else {
					m.addAlgoIdx = algoIndexOf("SHA1") // RFC 6238/4226 default
				}
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
					m.errMsg = i18n.Tf("tui.add.error.invalidURI", map[string]any{"Err": err})
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
				m.errMsg = i18n.T("tui.add.error.labelAndSecretRequired")
				return m, nil
			}

			// Validate base32 encoding for TOTP and HOTP secrets.
			ct := credTypeNames[m.addTypeIdx]
			switch ct.ctype {
			case pkgmodel.CredentialTOTP, pkgmodel.CredentialHOTP:
				if _, err := decodeBase32Secret(m.addSecretInput.Value()); err != nil {
					m.errMsg = i18n.T("tui.add.error.invalidBase32")
					return m, nil
				}
			}

			// Read algorithm and digits from selectors.
			algo := addAlgoValues[m.addAlgoIdx]
			if ct.ctype == pkgmodel.CredentialHOTP {
				algo = "SHA1"
			}
			digits := addDigitValues[m.addDigitsIdx]

			// Parse period for TOTP credentials.
			period := 30
			if ct.ctype == pkgmodel.CredentialTOTP {
				if v := strings.TrimSpace(m.addPeriodInput.Value()); v != "" {
					p, err := strconv.Atoi(v)
					if err != nil || p < 15 || p > 120 {
						m.errMsg = i18n.T("tui.add.error.invalidPeriod")
						return m, nil
					}
					period = p
				}
			}

			// Parse comma-separated tags and normalize to lowercase.
			var tags []string
			if raw := strings.TrimSpace(m.addTagsInput.Value()); raw != "" {
				for _, t := range strings.Split(raw, ",") {
					if t = strings.TrimSpace(t); t != "" {
						tags = append(tags, strings.ToLower(t))
					}
				}
			}

			// Normalize category to lowercase.
			category := strings.ToLower(strings.TrimSpace(m.addCategoryInput.Value()))

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
				Category:  category,
			}

			if m.vaultMgr == nil {
				m.errMsg = i18n.T("tui.add.error.vaultLocked")
				return m, nil
			}

			if _, err := m.vaultMgr.AddCredential(cred); err != nil {
				m.errMsg = i18n.Tf("tui.add.error.addFailed", map[string]any{"Err": err})
				return m, nil
			}

			if m.builder != nil {
				if logErr := m.builder.LogEvent("credential-add", cred.Label, cred.Issuer, audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(os.Stderr, "%s", i18n.Tf("tui.model.warn.auditFailed", map[string]any{"Err": logErr}))
				}
			}

			m = refreshCredList(m, labelVal)

			label := labelVal
			m.resetAddOverlay()
			m.state = stateMainView
			m.statusMsg = i18n.Tf("tui.add.success", map[string]any{"Label": label})
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
	case addSlotCategory:
		m.addCategoryInput, cmd = m.addCategoryInput.Update(msg)
	}
	return m, cmd
}

// addLabelWidthForType returns the column width for field labels in the add
// overlay based on the currently selected credential type.
func addLabelWidthForType(ct pkgmodel.CredentialType) int {
	labels := []string{
		i18n.T("tui.add.field.label"),
		i18n.T("tui.add.field.issuer"),
		i18n.T("tui.add.field.type"),
		i18n.T("tui.add.field.secret"),
		i18n.T("tui.add.field.algorithm"),
		i18n.T("tui.add.field.digits"),
		i18n.T("tui.add.field.period"),
		i18n.T("tui.add.field.tags"),
		i18n.T("tui.add.field.category"),
	}
	switch ct {
	case pkgmodel.CredentialStatic:
		labels[3] = i18n.T("tui.add.field.password")
	case pkgmodel.CredentialChallengeResponse:
		labels[3] = i18n.T("tui.add.field.sharedSecret")
	}

	max := 0
	for _, l := range labels {
		if w := lipgloss.Width(l); w > max {
			max = w
		}
	}
	// Keep one space between the label column and the field content.
	return max + 1
}

// viewOverlayAdd renders the add-credential overlay.
func (m model) viewOverlayAdd() string {
	ct := credTypeNames[m.addTypeIdx]
	col := addLabelWidthForType(ct.ctype)

	// pad pads label to col display columns using lipgloss.Width for
	// correct East Asian wide-character measurement.
	pad := func(label string) string {
		return label + strings.Repeat(" ", col-lipgloss.Width(label))
	}

	var lines []string
	lines = append(lines, titleStyle.Render(i18n.T("tui.add.title")))
	lines = append(lines, "")

	lines = append(lines, pad(i18n.T("tui.add.field.label"))+m.addLabelInput.View())
	lines = append(lines, pad(i18n.T("tui.add.field.issuer"))+m.addIssuerInput.View()+" "+helpBarStyle.Render(i18n.T("tui.add.hint.optional")))

	// Type selector row.
	lines = append(lines, renderAddSelector(col, pad, i18n.T("tui.add.field.type"), addSlotType, m.addFocusIdx, m.addTypeIdx, credTypeDisplayLabels()))

	// Secret row with type-dependent label.
	var secretLabel string
	switch ct.ctype {
	case pkgmodel.CredentialStatic:
		secretLabel = i18n.T("tui.add.field.password")
	case pkgmodel.CredentialChallengeResponse:
		secretLabel = i18n.T("tui.add.field.sharedSecret")
	default:
		secretLabel = i18n.T("tui.add.field.secret")
	}
	lines = append(lines, pad(secretLabel)+m.addSecretInput.View())

	// Algorithm selector — shown only for TOTP and challenge-response.
	if ct.ctype == pkgmodel.CredentialTOTP || ct.ctype == pkgmodel.CredentialChallengeResponse {
		lines = append(lines, renderAddSelector(col, pad, i18n.T("tui.add.field.algorithm"), addSlotAlgorithm, m.addFocusIdx, m.addAlgoIdx, addAlgoDisplayLabels()))
		if ct.ctype == pkgmodel.CredentialTOTP {
			lines = append(lines, strings.Repeat(" ", col)+helpBarStyle.Render(i18n.T("tui.add.help.algorithmNote1")))
			lines = append(lines, strings.Repeat(" ", col)+helpBarStyle.Render(i18n.T("tui.add.help.algorithmNote2")))
		}
	}

	// Digits selector — TOTP and HOTP only.
	if ct.ctype == pkgmodel.CredentialTOTP || ct.ctype == pkgmodel.CredentialHOTP {
		lines = append(lines, renderAddSelector(col, pad, i18n.T("tui.add.field.digits"), addSlotDigits, m.addFocusIdx, m.addDigitsIdx, []string{"6", "8"}))
	}

	// Period text input — TOTP only.
	if ct.ctype == pkgmodel.CredentialTOTP {
		periodView := strings.TrimRight(m.addPeriodInput.View(), " ")
		lines = append(lines, pad(i18n.T("tui.add.field.period"))+periodView+" "+helpBarStyle.Render(i18n.T("tui.add.unit.seconds")))
	}

	lines = append(lines, pad(i18n.T("tui.add.field.tags"))+m.addTagsInput.View()+" "+helpBarStyle.Render(i18n.T("tui.add.hint.optional")))
	lines = append(lines, pad(i18n.T("tui.add.field.category"))+m.addCategoryInput.View()+" "+helpBarStyle.Render(i18n.T("tui.add.hint.optional")))

	if m.errMsg != "" {
		lines = append(lines, "")
		lines = append(lines, errorStyle.Render(m.errMsg))
	}

	lines = append(lines, "")
	lines = append(lines, helpBarStyle.Render(i18n.T("tui.add.helpBar")))

	content := strings.Join(lines, "\n")
	// Grow the box with the terminal so wide languages (Japanese) fit all
	// credential types on one line without wrapping. Minimum 70, maximum 120,
	// leaving 8 columns margin (4 each side) so it doesn't fill the screen.
	boxWidth := m.width - 8
	if boxWidth < 70 {
		boxWidth = 70
	}
	if boxWidth > 120 {
		boxWidth = 120
	}
	overlay := overlayBoxStyle.Width(boxWidth).Render(content)
	return overlayOnBackground(overlay, m.width, m.height)
}

// credTypeDisplayLabels returns the localized display labels for credential types.
func credTypeDisplayLabels() []string {
	return []string{
		i18n.T("tui.add.type.totp"),
		i18n.T("tui.add.type.hotp"),
		i18n.T("tui.add.type.static"),
		i18n.T("tui.add.type.cr"),
	}
}

// addAlgoDisplayLabels returns the localized display labels for algorithms.
func addAlgoDisplayLabels() []string {
	return []string{
		i18n.T("tui.add.algo.sha1"),
		i18n.T("tui.add.algo.sha256"),
		i18n.T("tui.add.algo.sha512"),
	}
}

// renderAddSelector renders a label + selectable options row. The selected
// option is highlighted. When the selector has focus, left/right arrows flank
// the selected option. pad is a display-width-aware padding function.
func renderAddSelector(_ int, pad func(string) string, label string, slot, focusIdx, selectedIdx int, options []string) string {
	focused := focusIdx == slot
	var parts []string
	for i, opt := range options {
		if i == selectedIdx {
			if focused {
				parts = append(parts, tipStyle.Render("\u2190 "+opt+" \u2192"))
			} else {
				parts = append(parts, tipStyle.Render(opt))
			}
		} else {
			parts = append(parts, opt)
		}
	}
	return pad(label) + strings.Join(parts, "  ")
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
				if m.builder != nil {
					if logErr := m.builder.LogEvent("credential-remove", item.cred.Label, item.cred.Issuer, audit.Hostname(), true); logErr != nil {
						_, _ = fmt.Fprintf(os.Stderr, "%s", i18n.Tf("tui.model.warn.auditFailed", map[string]any{"Err": logErr}))
					}
				}
			}

			m = refreshCredList(m)
			m.state = stateMainView
			m.statusMsg = i18n.Tf("cmd.remove.success", map[string]any{"Label": item.cred.Label})
			return m, nil
		}
	}
	return m, nil
}

// viewOverlayRemove renders the remove-confirmation overlay.
func (m model) viewOverlayRemove() string {
	label := i18n.T("tui.remove.noneSelected")
	if selected := m.credList.SelectedItem(); selected != nil {
		if item, ok := selected.(credItem); ok {
			label = item.cred.Label
		}
	}

	var lines []string
	lines = append(lines, titleStyle.Render(i18n.T("tui.remove.title")))
	lines = append(lines, "")
	lines = append(lines, i18n.T("tui.remove.credentialLabel")+label)
	lines = append(lines, "")
	lines = append(lines, helpBarStyle.Render(i18n.T("tui.remove.helpBar")))

	content := strings.Join(lines, "\n")
	overlay := overlayBoxStyle.Render(content)
	return overlayOnBackground(overlay, m.width, m.height)
}

// refreshCredList rebuilds the credential list from the vault manager,
// grouped by category, with each category section alphabetically sorted.
// The list selection moves to the item matching selectLabel (if non-empty), or resets to the top.
func refreshCredList(m model, selectLabel ...string) model {
	if m.vaultMgr == nil {
		return m
	}
	creds := m.vaultMgr.ListCredentials()

	// Group credentials by category
	groups := make(map[string][]pkgmodel.Credential)
	for _, c := range creds {
		key := c.Category
		if key == "" {
			key = "[Uncategorized]"
		}
		groups[key] = append(groups[key], c)
	}

	// Sort category keys (alphabetically, with [Uncategorized] at the end)
	var categories []string
	for cat := range groups {
		categories = append(categories, cat)
	}
	sort.Slice(categories, func(i, j int) bool {
		if categories[i] == "[Uncategorized]" {
			return false
		}
		if categories[j] == "[Uncategorized]" {
			return true
		}
		return categories[i] < categories[j]
	})

	// Build items list with category headers and sorted credentials
	items := make([]list.Item, 0, len(creds)+len(categories))
	selectedIdx := 0
	itemIdx := 0

	for _, cat := range categories {
		// Add category header
		items = append(items, categoryHeaderItem{category: cat})
		itemIdx++

		// Sort credentials within this category by label
		catCreds := groups[cat]
		sort.Slice(catCreds, func(i, j int) bool {
			return strings.ToLower(catCreds[i].Label) < strings.ToLower(catCreds[j].Label)
		})

		// Add credentials
		for _, c := range catCreds {
			items = append(items, credItem{cred: c})
			if len(selectLabel) > 0 && c.Label == selectLabel[0] {
				selectedIdx = itemIdx
			}
			itemIdx++
		}
	}

	m.credList.SetItems(items)
	// If no specific label to select, find the first credItem (skip category headers)
	if len(selectLabel) == 0 && selectedIdx == 0 {
		for i, item := range items {
			if _, ok := item.(credItem); ok {
				selectedIdx = i
				break
			}
		}
	}
	m.credList.Select(selectedIdx)
	m.cursor = selectedIdx
	switch len(creds) {
	case 0:
		m.credList.Title = i18n.T("cmd.list.empty")
	case 1:
		m.credList.Title = i18n.T("tui.credList.one")
	default:
		m.credList.Title = i18n.Tf("tui.credList.many", map[string]any{"Count": len(creds)})
	}
	return m
}

// overlayOnBackground places an overlay box centered on the screen.
func overlayOnBackground(overlay string, width, height int) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, overlay)
}
