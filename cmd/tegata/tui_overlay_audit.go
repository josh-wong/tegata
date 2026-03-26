package main

import (
	"context"
	"fmt"
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

// auditHistoryCmd creates a tea.Cmd that fetches audit history asynchronously.
func auditHistoryCmd(cfg config.AuditConfig) tea.Cmd {
	return func() tea.Msg {
		client, err := audit.NewClientFromConfig(cfg.Server, cfg.PrivilegedServer, cfg.EntityID, cfg.KeyVersion, cfg.SecretKey, cfg.Insecure)
		if err != nil {
			return auditHistoryMsg{err: err}
		}
		defer func() { _ = client.Close() }()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		collectionID := audit.CollectionID(cfg.EntityID)
		eventIDs, err := client.CollectionGet(ctx, collectionID)
		if err != nil {
			return auditHistoryMsg{err: err}
		}

		var hist []historyRecord
		var skipped int
		for _, id := range eventIDs {
			evts, err := client.Get(ctx, id)
			if err != nil {
				skipped++
				continue
			}
			if len(evts) == 0 {
				continue
			}
			r := evts[0]
			hist = append(hist, historyRecord{
				ObjectID:  r.ObjectID,
				Operation: audit.MetadataString(r.Metadata, "operation"),
				LabelHash: audit.MetadataString(r.Metadata, "label_hash"),
				Timestamp: audit.MetadataInt64(r.Metadata, "timestamp"),
				HashValue: r.HashValue,
			})
		}
		msg := auditHistoryMsg{records: hist}
		if skipped > 0 {
			msg.warning = fmt.Sprintf("%d of %d events could not be fetched", skipped, len(eventIDs))
		}
		return msg
	}
}

// auditVerifyCmd creates a tea.Cmd that runs audit verification asynchronously.
func auditVerifyCmd(cfg config.AuditConfig) tea.Cmd {
	return func() tea.Msg {
		client, err := audit.NewClientFromConfig(cfg.Server, cfg.PrivilegedServer, cfg.EntityID, cfg.KeyVersion, cfg.SecretKey, cfg.Insecure)
		if err != nil {
			return auditVerifyMsg{err: err}
		}
		defer func() { _ = client.Close() }()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		collectionID := audit.CollectionID(cfg.EntityID)
		eventIDs, err := client.CollectionGet(ctx, collectionID)
		if err != nil {
			return auditVerifyMsg{err: err}
		}

		var faults int
		for _, id := range eventIDs {
			result, err := client.Validate(ctx, id)
			if err != nil {
				faults++
				continue
			}
			if !result.Valid {
				faults++
			}
		}
		return auditVerifyMsg{
			valid:      faults == 0,
			eventCount: len(eventIDs),
			detail:     fmt.Sprintf("%d of %d events have issues", faults, len(eventIDs)),
		}
	}
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
			if m.auditMenuIdx == 0 {
				m.auditSubFlow = "history"
				m.auditLoading = true
				return m, auditHistoryCmd(m.cfg.Audit)
			}
			m.auditSubFlow = "verify"
			m.auditLoading = true
			return m, auditVerifyCmd(m.cfg.Audit)
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

	help := helpBarStyle.Render("[j/k] Navigate  [Enter] Select  [Esc] Close")
	return title + "\n\n" + menu.String() + "\n" + help
}

func (m model) viewAuditHistory() string {
	title := titleStyle.Render("Audit history")

	if m.auditLoading {
		return title + "\n\n" + m.spinner.View() + " Loading...\n\n" +
			helpBarStyle.Render("[Esc] Back")
	}

	var body strings.Builder
	if len(m.auditRecords) > 0 {
		body.WriteString(fmt.Sprintf("%-12s %-12s %-20s %s\n", "Operation", "Label", "Timestamp", "Hash"))
		body.WriteString(strings.Repeat("─", 65) + "\n")
		for _, r := range m.auditRecords {
			label := r.LabelHash
			if len(label) > 12 {
				label = label[:12]
			}
			ts := time.Unix(r.Timestamp, 0).UTC().Format("2006-01-02 15:04:05")
			hash := r.HashValue
			if len(hash) > 16 {
				hash = hash[:16] + "…"
			}
			body.WriteString(fmt.Sprintf("%-12s %-12s %-20s %s\n", r.Operation, label, ts, hash))
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
