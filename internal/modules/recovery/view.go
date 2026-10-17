package recovery

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
)

func operationLabel(op string) string {
	switch op {
	case "provider.apply":
		return "Apply provider"
	case "provider.clear":
		return "Clear provider"
	case "hook.install":
		return "Install hook"
	case "hook.uninstall":
		return "Uninstall hook"
	case "config.restore":
		return "Restore settings"
	}
	return "Not recorded"
}

func backupFile(e Entry) string {
	if e.Destination != "" {
		return filepath.Base(e.Destination)
	}
	if parts := snapshotName.FindStringSubmatch(e.ID); len(parts) > 1 {
		return parts[1]
	}
	return e.ID
}

func (m *Tab) agentName(e Entry) string {
	if e.Agent == "" || e.Agent == "unknown" {
		return "Not recorded"
	}
	if name := m.agentNames[e.Agent]; name != "" {
		return name
	}
	return kit.AgentLabel(e.Agent)
}

func (m *Tab) counts() (ready, manual int) {
	for _, e := range m.entries {
		if e.Restorable {
			ready++
		} else {
			manual++
		}
	}
	return
}

func (m *Tab) inventoryHeader() string {
	ready, manual := m.counts()
	lines := []string{}
	if m.height >= 18 {
		lines = append(lines, kit.StTitle.Render("Recover agent settings"))
		if m.width >= 60 {
			lines = append(lines, "Review a saved copy before replacing the current file.")
		}
	}
	lines = append(lines, fmt.Sprintf("Preview: %d · Manual only: %d", ready, manual))
	if m.filtering || m.input.Value() != "" {
		lines = append(lines, components.InputView(m.input, m.width))
	}
	return strings.Join(lines, "\n")
}

func (m *Tab) inventoryFooter() string {
	toggle := "all backups"
	if m.showUnavailable {
		toggle = "ready only"
	}
	actions := [][2]string{{"l", toggle}, {"/", "filter"}, {"r", "reload"}, {"?", "help"}}
	if len(m.matches) > 0 {
		action := "details"
		if m.matches[m.cursor].Restorable {
			action = "preview"
		}
		actions = append([][2]string{{"enter", action}}, actions...)
	}
	if m.filtering {
		actions = [][2]string{{"enter", "apply filter"}, {"esc", "clear filter"}}
	}
	return kit.Hints(m.width, actions...)
}

func (m *Tab) split() kit.Split {
	height := max(1, m.height-lipgloss.Height(m.inventoryHeader())-2)
	sp := kit.SplitDetail(m.width, height)
	if m.height < 16 {
		sp.DetailH = 0
		sp.ListH = height
	} else if !sp.Side {
		sp.ListH = min(sp.ListH, max(4, len(m.matches)+3))
		sp.DetailH = min(kit.MaxStripHeight, max(0, height-sp.ListH))
	}
	return sp
}

func (m *Tab) inventoryView() string {
	sp := m.split()
	if len(m.matches) == 0 {
		sp.ListW = min(m.width, 88)
	}
	table := m.backupsView(sp.ListW, sp.ListH)
	body := table
	if sp.DetailH > 0 && len(m.matches) > 0 {
		if sp.Side {
			sp.DetailH = min(sp.DetailH, m.detailViewport(sp).TotalLineCount()+2)
		}
		title := "Backup details"
		detail := kit.DetailView(sp, title, m.detailViewport(sp))
		body = lipgloss.JoinVertical(lipgloss.Left, table, detail)
		if sp.Side {
			body = lipgloss.JoinHorizontal(lipgloss.Top, table, "  ", detail)
		}
	}
	content := m.inventoryHeader() + "\n" + body
	footer := m.inventoryFooter()
	if m.notice != "" {
		footer += "\n" + components.Toast(m.notice, m.noticeErr)
	}
	return kit.Frame(content, footer, m.height)
}

func backupColumns(width int) []kit.Column {
	dateW, agentW, opW := 0, 0, 0
	if width >= 55 {
		dateW = 23
	}
	if width >= 85 {
		agentW = 14
	}
	if width >= 115 {
		opW = 18
	}
	return []kit.Column{{Title: "Saved", Width: dateW}, {Title: "File", Flex: true}, {Title: "Agent", Width: agentW}, {Title: "Saved before", Width: opW}, {Title: "Status", Width: 7}}
}

func (m *Tab) backupsView(width, height int) string {
	inner := max(1, width-4)
	title := "Ready to preview"
	if m.showUnavailable {
		title = "All backups"
	}
	title += fmt.Sprintf(" (%d)", len(m.matches))
	if len(m.matches) > 0 {
		title += fmt.Sprintf(" · %d/%d", m.cursor+1, len(m.matches))
	}
	lines := []string{}
	if len(m.matches) == 0 {
		text := "No backups ready to preview."
		switch {
		case m.input.Value() != "":
			text = "No backups match this filter."
		case len(m.entries) == 0:
			text = "No configuration backups yet. Provider and hook changes create them automatically. Skill archives are in Skills."
		default:
			text += "\n\nGuided recovery needs a verified agent and destination. Older copies may not record that information; a filename such as settings.json is used by several agents.\n\nUse l to inspect these copies for manual recovery. New provider and hook backups record their origin automatically."
		}
		lines = append(lines, kit.Wrap(text, inner, ""))
	} else {
		cols := backupColumns(inner)
		lines = append(lines, kit.TableHeader(inner, cols))
		start, end := kit.Window(m.cursor, len(m.matches), max(1, height-3))
		for i := start; i < end; i++ {
			e := m.matches[i]
			action := kit.StOn.Render("Preview")
			if !e.Restorable {
				action = kit.StWarn.Render("Manual")
			}
			name := m.agentName(e)
			if name == "Not recorded" {
				name = "—"
			}
			lines = append(lines, kit.TableRow(inner, i == m.cursor, cols, e.Time.Local().Format("2006-01-02 15:04:05.000"), backupFile(e), name, operationLabel(e.Operation), action))
		}
	}
	room := max(0, height-2)
	if m.split().Side || len(m.matches) == 0 {
		room = min(room, lipgloss.Height(strings.Join(lines, "\n")))
	}
	content := kit.Frame(strings.Join(lines, "\n"), "", room)
	return components.Panel{Title: title, Focused: true, Width: width}.Render(content)
}

func (m *Tab) entryDetails(e Entry) string {
	lines := []string{}
	if e.Restorable {
		lines = append(lines, kit.StOn.Render("Available for preview"), "", m.agentName(e)+" · "+backupFile(e), "Saved before: "+operationLabel(e.Operation), "", "Enter reviews the differences. Nothing is restored until you confirm replacing the whole file.")
	} else {
		lines = append(lines, kit.StWarn.Render("Guided restore unavailable"), "", "This copy has no verified restore destination, or cannot be used safely. Older backups may not record their original agent and path.", "", "A filename such as settings.json is used by several agents. Its name alone cannot identify where it belongs.", "", "Keep this copy for manual recovery. New provider/hook backups record their origin automatically.", "", "Reason: "+e.Reason)
	}
	lines = append(lines, "", "Saved: "+e.Time.Local().Format("2006-01-02 15:04:05.000"))
	if e.Destination != "" {
		lines = append(lines, "Restore to: "+e.Destination)
	}
	lines = append(lines, "Backup file: "+e.ID, "Backup folder: "+m.svc.dir)
	return strings.Join(lines, "\n")
}

func (m *Tab) detailViewport(sp kit.Split) viewport.Model {
	w, h := kit.DetailSize(sp)
	vp := viewport.New(viewport.WithWidth(w), viewport.WithHeight(h))
	text := "Guided recovery needs a recorded agent and destination. Use l to inspect copies that need manual recovery. New provider and hook changes create backups with their origin recorded."
	if len(m.entries) == 0 {
		text = "Provider and hook changes save the previous configuration here. Select a backup, review its differences, then confirm the file to restore. Skill library archives are in Skills (B)."
	}
	if len(m.matches) > 0 {
		text = m.entryDetails(m.matches[m.cursor])
	}
	vp.SetContent(lipgloss.NewStyle().Width(w).Render(text))
	vp.SetYOffset(m.detailOff)
	return vp
}
