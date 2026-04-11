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
	detail     string
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
func auditVerifyCmd(cfg config.AuditConfig) tea.Cmd {
	return func() tea.Msg {
		client, err := audit.NewClientFromConfig(cfg)
		if err != nil {
			return auditVerifyMsg{err: err}
		}
		defer func() { _ = client.Close() }()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := audit.VerifyAll(ctx, client, cfg.EntityID)
		if err != nil {
			return auditVerifyMsg{err: err}
		}
		return auditVerifyMsg{
			valid:      result.Valid,
			eventCount: result.EventCount,
			detail:     result.ErrorDetail,
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

// resetAuditOverlay clears all audit overlay state.
func (m *model) resetAuditOverlay() {
	m.auditMenuIdx = 0
	m.auditSubFlow = ""
	m.auditMsg = ""
	m.auditRecords = nil
	m.auditLoading = false
}

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
				return m, nil
			}
			m.resetAuditOverlay()
			m.state = stateMainView
			return m, nil

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
				return m, auditHistoryCmd(m.cfg.Audit)
			case 1:
				m.auditSubFlow = "verify"
				m.auditLoading = true
				return m, auditVerifyCmd(m.cfg.Audit)
			}
		}
	}
	return m, nil
}

// viewOverlayAudit renders the audit overlay.
func (m model) viewOverlayAudit() string {
	var content string

	switch m.auditSubFlow {
	case "":
		content = m.viewAuditMenu()
	case "history":
		content = m.viewAuditHistory()
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
			menu.WriteString(successStyle.Render("▸ " + item))
		} else {
			menu.WriteString("  " + item)
		}
		menu.WriteString("\n")
	}

	help := helpBarStyle.Render("[↑↓] Navigate  [Enter] Select  [Esc] Close")
	return title + "\n\n" + menu.String() + "\n" + help
}

func (m model) viewAuditHistory() string {
	title := titleStyle.Render("Audit history")

	if m.auditLoading {
		return title + "\n\n" + m.spinner.View() + " Loading...\n\n" +
			helpBarStyle.Render("[Esc] Back")
	}

	// Build hash→label lookup from loaded credentials.
	labelMap := m.buildLabelMap()

	var body strings.Builder
	if len(m.auditRecords) > 0 {
		body.WriteString(fmt.Sprintf("%-20s %-20s %-20s %s\n", "Operation", "Label", "Timestamp", "Hash"))
		body.WriteString(strings.Repeat("─", 80) + "\n")
		for _, r := range m.auditRecords {
			label := audit.ResolveLabel(r.LabelHash, labelMap)
			if len(label) > 20 {
				label = label[:19] + "…"
			}
			op := audit.FormatOperation(r.Operation)
			ts := time.Unix(r.Timestamp, 0).UTC().Format("2006-01-02 15:04:05")
			hash := r.HashValue
			if len(hash) > 16 {
				hash = hash[:16] + "…"
			}
			body.WriteString(fmt.Sprintf("%-20s %-20s %-20s %s\n", op, label, ts, hash))
		}
	}

	if m.auditMsg != "" {
		body.WriteString("\n" + m.auditMsg)
	}

	help := helpBarStyle.Render("[Esc] Back")
	return title + "\n\n" + body.String() + "\n\n" + help
}

func (m model) viewAuditVerify() string {
	title := titleStyle.Render("Verify audit log")

	if m.auditLoading {
		return title + "\n\n" + m.spinner.View() + " Verifying...\n\n" +
			helpBarStyle.Render("[Esc] Back")
	}

	var body string
	if strings.Contains(m.auditMsg, "TAMPER DETECTED") {
		body = errorStyle.Render(m.auditMsg)
	} else if strings.Contains(m.auditMsg, "verified") {
		body = successStyle.Render(m.auditMsg)
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
			body = successStyle.Render(m.auditMsg)
		}
	}

	help := helpBarStyle.Render("[Esc] Continue")
	return title + "\n\n" + body + "\n\n" + help
}
