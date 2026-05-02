package main

import (
	"context"
	"fmt"
	"io/fs"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/vault"
)

// auditHistoryMsg carries the result of an async history fetch.
type auditHistoryMsg struct {
	records []historyRecord
	err     error
	warning string // non-fatal issues (e.g. some events couldn't be fetched)
}

// auditVerifyMsg carries the result of an async verify call.
type auditVerifyMsg struct {
	valid      bool
	eventCount int
	skipped    int
	faults     []string // per-event fault descriptions when !valid
	err        error
}

// auditStartMsg carries the result of an async Docker audit setup run.
// steps contains the sequential status lines emitted during setup.
// On success, newCfg is the written AuditConfig. On failure, err is set.
type auditStartMsg struct {
	steps  []string
	newCfg config.AuditConfig
	err    error
}

// auditHistoryCmd creates a tea.Cmd that fetches audit history asynchronously.
func auditHistoryCmd(cfg config.AuditConfig) tea.Cmd {
	return func() tea.Msg {
		client, err := audit.NewClientFromConfig(cfg)
		if err != nil {
			return auditHistoryMsg{err: err}
		}
		defer func() { _ = client.Close() }()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := audit.FetchHistory(ctx, client, cfg.EntityID)
		if err != nil {
			return auditHistoryMsg{err: err}
		}

		hist := make([]historyRecord, len(result.Records))
		for i, r := range result.Records {
			hist[i] = historyRecord{
				ObjectID:  r.ObjectID,
				Operation: r.Operation,
				LabelHash: r.LabelHash,
				Timestamp: r.Timestamp,
				HashValue: r.HashValue,
			}
		}
		return auditHistoryMsg{records: hist, warning: result.Warning}
	}
}

// auditVerifyCmd creates a tea.Cmd that runs audit verification asynchronously.
// vaultHashes is the caller's copy of the audit hash map from the vault; the
// cmd takes ownership and zeros it after use (D-16).
func auditVerifyCmd(cfg config.AuditConfig, vaultHashes map[string]string) tea.Cmd {
	return func() tea.Msg {
		defer vault.ZeroAuditHashes(vaultHashes)

		client, err := audit.NewClientFromConfig(cfg)
		if err != nil {
			return auditVerifyMsg{err: err}
		}
		defer func() { _ = client.Close() }()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := audit.VerifyAll(ctx, client, cfg.EntityID, vaultHashes)
		if err != nil {
			return auditVerifyMsg{err: err}
		}
		return auditVerifyMsg{
			valid:      result.Valid,
			eventCount: result.EventCount,
			skipped:    result.Skipped,
			faults:     result.Faults,
		}
	}
}

// buildLabelMap constructs a hash→label lookup from the credential list.
func (m model) buildLabelMap() map[string]string {
	items := m.credList.Items()
	labels := make([]string, 0, len(items))
	for _, item := range items {
		if ci, ok := item.(credItem); ok {
			labels = append(labels, ci.cred.Label)
		}
	}
	return audit.BuildLabelMap(labels)
}

// buildDeletedLabelMap returns the vault's deleted-label hash→name map,
// used to resolve labels for credentials that have been removed.
func (m model) buildDeletedLabelMap() map[string]string {
	if m.vaultMgr == nil {
		return nil
	}
	return m.vaultMgr.DeletedLabels()
}

// resetAuditOverlay clears all audit overlay state.
func (m *model) resetAuditOverlay() {
	m.auditMenuIdx = 0
	m.auditSubFlow = ""
	m.auditMsg = ""
	m.auditRecords = nil
	m.auditLoading = false
	m.auditCursor = 0
	m.auditScrollOff = 0
	m.auditMsgTime = time.Time{}
}

// auditHistoryPageSize is the number of history rows visible at one time in the TUI.
const auditHistoryPageSize = 10

// updateOverlayAudit handles input for the audit overlay.
func (m model) updateOverlayAudit(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyEsc:
			if m.auditSubFlow != "" {
				// After the ledger start completes, go straight to main view
				// (no menu to return to, and audit history would be empty).
				if m.auditSubFlow == "start" && !m.auditLoading {
					m = loadCredentials(m)
					m.resetAuditOverlay()
					m.state = stateMainView
					return m, tickCmd()
				}
				m.auditSubFlow = ""
				m.auditMsg = ""
				m.auditRecords = nil
				m.auditCursor = 0
				m.auditScrollOff = 0
				return m, nil
			}
			m.resetAuditOverlay()
			m.state = stateMainView
			return m, nil

		// History sub-flow navigation: j/↓ and k/↑ scroll through records.
		case m.auditSubFlow == "history" && !m.auditLoading &&
			(msg.Type == tea.KeyDown || (len(msg.Runes) == 1 && msg.Runes[0] == 'j')):
			if m.auditCursor < len(m.auditRecords)-1 {
				m.auditCursor++
				if m.auditCursor >= m.auditScrollOff+auditHistoryPageSize {
					m.auditScrollOff++
				}
			}
			return m, nil

		case m.auditSubFlow == "history" && !m.auditLoading &&
			(msg.Type == tea.KeyUp || (len(msg.Runes) == 1 && msg.Runes[0] == 'k')):
			if m.auditCursor > 0 {
				m.auditCursor--
				if m.auditCursor < m.auditScrollOff {
					m.auditScrollOff--
				}
			}
			return m, nil

		// History sub-flow: Enter or 'c' copies the selected record's full hash.
		case m.auditSubFlow == "history" && !m.auditLoading && len(m.auditRecords) > 0 &&
			(msg.Type == tea.KeyEnter || (len(msg.Runes) == 1 && msg.Runes[0] == 'c')):
			if m.auditCursor < len(m.auditRecords) {
				hash := m.auditRecords[m.auditCursor].HashValue
				if m.clipMgr != nil {
					if err := m.clipMgr.CopyWithAutoClear(hash, m.cfg.ClipboardTimeout); err != nil {
						m.auditMsg = fmt.Sprintf("Hash: %s  (clipboard unavailable)", hash)
						m.auditMsgTime = time.Time{} // don't auto-dismiss error messages
					} else {
						m.auditMsg = fmt.Sprintf("Hash copied to clipboard (auto-clears in %ds)",
							int(m.cfg.ClipboardTimeout.Seconds()))
						m.auditMsgTime = m.now // set time for auto-dismiss
					}
				} else {
					m.auditMsg = fmt.Sprintf("Hash: %s", hash)
					m.auditMsgTime = time.Time{}
				}
			}
			return m, nil

		// Menu navigation (when not in a sub-flow).
		case m.auditSubFlow == "" && (msg.Type == tea.KeyDown || (len(msg.Runes) == 1 && msg.Runes[0] == 'j')):
			if m.auditMenuIdx < 1 {
				m.auditMenuIdx++
			}
			return m, nil

		case m.auditSubFlow == "" && (msg.Type == tea.KeyUp || (len(msg.Runes) == 1 && msg.Runes[0] == 'k')):
			if m.auditMenuIdx > 0 {
				m.auditMenuIdx--
			}
			return m, nil

		case m.auditSubFlow == "" && msg.Type == tea.KeyEnter:
			switch m.auditMenuIdx {
			case 0:
				m.auditSubFlow = "history"
				m.auditLoading = true
				m.auditCursor = 0
				m.auditScrollOff = 0
				return m, auditHistoryCmd(m.cfg.Audit)
			case 1:
				m.auditSubFlow = "verify"
				m.auditLoading = true
				var hashes map[string]string
				if m.vaultMgr != nil {
					hashes = m.vaultMgr.AuditHashes()
				}
				return m, auditVerifyCmd(m.cfg.Audit, hashes)
			}
		}
	}
	return m, nil
}

// viewOverlayAudit renders the audit overlay.
func (m model) viewOverlayAudit() string {
	// History needs a wider box to display four columns without wrapping.
	//
	// Lipgloss Width() sets the content width *before* padding.
	// wrapAt = Width - leftPad - rightPad = Width - 2 (for Padding(1)).
	// Outer box width = Width + 2 (padding) + 2 (border) = Width + 4.
	//
	// We want the outer box to leave at least 4 cols of breathing room:
	//   outer ≤ m.width - 4  →  Width ≤ m.width - 8
	// lineW (actual text wrap boundary) = Width - 2 = m.width - 10.
	if m.auditSubFlow == "history" {
		lineW := m.width - 10
		if lineW > 96 {
			lineW = 96
		}
		if lineW < 64 {
			lineW = 64
		}
		boxW := lineW + 2 // Width() = lineW + leftPad + rightPad
		content := m.viewAuditHistory(lineW)
		box := overlayBoxStyle.Width(boxW).Render(content)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	}

	var content string
	switch m.auditSubFlow {
	case "":
		content = m.viewAuditMenu()
	case "verify":
		content = m.viewAuditVerify()
	case "start":
		content = m.viewAuditStart()
	}

	box := overlayBoxStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) viewAuditMenu() string {
	title := titleStyle.Render("Audit")

	items := []string{"View history", "Verify integrity"}
	var menu strings.Builder
	for i, item := range items {
		if i == m.auditMenuIdx {
			menu.WriteString(tipStyle.Render("▸ " + item))
		} else {
			menu.WriteString("  " + item)
		}
		menu.WriteString("\n")
	}

	help := helpBarStyle.Render("[↑↓] Navigate  [Enter] Select  [Esc] Close")
	return title + "\n\n" + menu.String() + "\n" + help
}

// viewAuditHistory renders the audit history sub-flow.
// boxW is the lipgloss Width value (text area, before padding and border).
func (m model) viewAuditHistory(boxW int) string {
	title := titleStyle.Render("Audit history")

	if m.auditLoading {
		return title + "\n\n" + m.spinner.View() + " Loading...\n\n" +
			helpBarStyle.Render("[Esc] Back")
	}

	// Derive column widths from available space.
	// Each row: 2-char prefix + opW + 1 + labelW + 1 + 19 (timestamp) + 1 + hashW
	const tsW = 19
	const prefixW = 2
	remainder := boxW - prefixW - 5 - tsW // 5 = spaces between the 4 columns (3 before hash)
	opW := remainder * 2 / 5
	if opW < 15 {
		opW = 15
	}
	labelW := remainder * 3 / 10
	if labelW < 10 {
		labelW = 10
	}
	hashW := remainder - opW - labelW
	if hashW < 10 {
		hashW = 10
	}
	lineW := prefixW + opW + 1 + labelW + 1 + tsW + 1 + hashW

	// Build hash→label lookup from loaded credentials and deleted labels.
	labelMap := m.buildLabelMap()
	deletedMap := m.buildDeletedLabelMap()

	// Filter out lock/unlock events by default.
	filtered := make([]historyRecord, 0, len(m.auditRecords))
	for _, r := range m.auditRecords {
		op := strings.ToLower(r.Operation)
		if op != "vault lock" && op != "vault unlock" {
			filtered = append(filtered, r)
		}
	}

	var body strings.Builder
	if len(filtered) > 0 {
		body.WriteString(fmt.Sprintf("  %-*s %-*s %-*s   %s\n", opW, "Operation", labelW, "Label", tsW, "Timestamp", "Hash"))
		body.WriteString(strings.Repeat("─", lineW) + "\n")

		end := m.auditScrollOff + auditHistoryPageSize
		if end > len(filtered) {
			end = len(filtered)
		}
		for idx := m.auditScrollOff; idx < end; idx++ {
			r := filtered[idx]
			label := audit.ResolveLabelWithDeleted(r.LabelHash, labelMap, deletedMap)
			if len(label) > labelW {
				label = label[:labelW-1] + "…"
			}
			op := audit.FormatOperation(r.Operation)
			if len(op) > opW {
				op = op[:opW-1] + "…"
			}
			ts := time.Unix(r.Timestamp, 0).Local().Format("2006-01-02 15:04:05")
			hash := r.HashValue
			if len(hash) > hashW {
				hash = hash[:hashW-1] + "…"
			}
			line := fmt.Sprintf("%-*s %-*s %-*s   %s", opW, op, labelW, label, tsW, ts, hash)
			if idx == m.auditCursor {
				body.WriteString(tipStyle.Render("▸ " + line))
			} else {
				body.WriteString("  " + line)
			}
			body.WriteString("\n")
		}

		// Show scroll indicator when the list overflows.
		if len(filtered) > auditHistoryPageSize {
			body.WriteString(fmt.Sprintf("\n  %d–%d of %d",
				m.auditScrollOff+1, end, len(filtered)))
		}
	}

	if m.auditMsg != "" {
		body.WriteString("\n" + m.auditMsg)
	}

	help := helpBarStyle.Render("[↑↓] Navigate  [Enter/c] Copy hash  [Esc] Back")
	return title + "\n\n" + body.String() + "\n\n" + help
}

func (m model) viewAuditVerify() string {
	title := titleStyle.Render("Verify audit log")

	if m.auditLoading {
		return title + "\n\n" + m.spinner.View() + " Verifying...\n\n" +
			helpBarStyle.Render("[Esc] Back")
	}

	var body string
	if strings.Contains(m.auditMsg, "TAMPERING DETECTED") {
		body = errorStyle.Render(m.auditMsg)
	} else if strings.Contains(m.auditMsg, "verified") {
		body = tipStyle.Render(m.auditMsg)
	} else {
		body = m.auditMsg
	}

	help := helpBarStyle.Render("[Esc] Back")
	return title + "\n\n" + body + "\n\n" + help
}

// auditStartCmd creates a tea.Cmd that runs Docker audit setup asynchronously.
// cfg is the full Config (not just AuditConfig) — needed for vaultDir.
// vaultID is the vault's stable UUID (per D-04), captured at unlock time
// from Manager.VaultID() and stored in model.vaultID.
func auditStartCmd(cfg config.Config, vaultPath, vaultID string) tea.Cmd {
	return func() tea.Msg {
		vaultDir := filepath.Dir(vaultPath)

		u, err := user.Current()
		if err != nil {
			return auditStartMsg{err: fmt.Errorf("resolving home directory: %w", err)}
		}
		composeDir := filepath.Join(u.HomeDir, ".tegata", "docker")

		bundleFS, err := fs.Sub(dockerBundle, "docker-bundle")
		if err != nil {
			return auditStartMsg{err: fmt.Errorf("accessing docker bundle: %w", err)}
		}

		var steps []string
		progress := func(msg string) {
			steps = append(steps, msg)
		}

		newCfg, err := audit.SetupStack(bundleFS, composeDir, vaultID, progress, nil)
		if err != nil {
			return auditStartMsg{steps: steps, err: err}
		}

		if writeErr := config.WriteAuditSection(vaultDir, newCfg); writeErr != nil {
			return auditStartMsg{steps: steps, err: fmt.Errorf("writing audit config: %w", writeErr)}
		}

		return auditStartMsg{steps: steps, newCfg: newCfg}
	}
}

// viewAuditStart renders the ledger server setup sub-flow. Shows a spinner
// while running, then a success or error message.
func (m model) viewAuditStart() string {
	title := titleStyle.Render("Start ledger server")

	if m.auditLoading {
		return title + "\n\n" + m.spinner.View() + " Starting ledger server...\n\n" +
			helpBarStyle.Render("")
	}

	var body string
	if m.auditMsg != "" {
		if strings.Contains(m.auditMsg, "failed") || strings.Contains(m.auditMsg, "Failed") ||
			strings.Contains(m.auditMsg, "error") || strings.Contains(m.auditMsg, "Error") {
			body = errorStyle.Render(m.auditMsg)
		} else {
			body = tipStyle.Render(m.auditMsg)
		}
	}

	help := helpBarStyle.Render("[Esc] Continue")
	return title + "\n\n" + body + "\n\n" + help
}
