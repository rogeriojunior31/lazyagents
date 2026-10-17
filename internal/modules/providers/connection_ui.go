package providers

import (
	"bytes"
	"context"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
)

type connectionPicker struct {
	request                 uint64
	cancel                  context.CancelFunc
	busy, catalog, fromForm bool
	source                  agent.ProviderProfile // redacted snapshot of the requested connection
	result                  ConnectionResult
	input                   textinput.Model
	cursor, scroll          int
}

type connectionActionMsg struct{ catalog bool }

type connectionMsg struct {
	request uint64
	result  ConnectionResult
	source  agent.ProviderProfile // never the credential used by the request
}

func (m *Tab) connectionCmd(catalog bool) tea.Cmd {
	p, ok := m.current()
	original := ""
	fromForm := m.form != nil
	if fromForm {
		p, ok, original = m.form.profile(), true, m.form.orig
	}
	if !ok {
		return nil
	}
	m.closeConnection()
	m.connectionRequest++
	request, svc := m.connectionRequest, m.svc
	ctx, cancel := context.WithCancel(context.Background())
	input := components.NewInput()
	input.Prompt = "Filter: "
	input.CharLimit = 256
	m.connection = &connectionPicker{request: request, cancel: cancel, busy: true, catalog: catalog, fromForm: fromForm, source: p.Redacted(), input: input}
	return func() tea.Msg {
		if !fromForm {
			saved, err := svc.Profile(p.Name)
			if err != nil {
				r := connectionResult(catalog)
				r.Error = "The saved profile could not be read; reload the library before checking"
				return connectionMsg{request: request, result: r}
			}
			p = saved
		}
		result := svc.CheckDraft(ctx, p, original, catalog)
		return connectionMsg{request: request, result: result, source: p.Redacted()}
	}
}

func (m *Tab) closeConnection() {
	if m.connection != nil && m.connection.cancel != nil {
		m.connection.cancel()
	}
	m.connection = nil
}

func (p *connectionPicker) models() []string {
	query := strings.ToLower(strings.TrimSpace(p.input.Value()))
	if query == "" {
		return p.result.Models
	}
	var ids []string
	for _, id := range p.result.Models {
		if strings.Contains(strings.ToLower(id), query) {
			ids = append(ids, id)
		}
	}
	return ids
}

func (m *Tab) connectionKey(msg tea.KeyPressMsg) tea.Cmd {
	p := m.connection
	key := msg.String()
	if key == "esc" {
		m.closeConnection()
		return nil
	}
	if p.busy {
		return nil
	}
	if p.result.Catalog && !kit.DetailScroll[key] {
		ids := p.models()
		switch key {
		case "up":
			p.cursor = max(0, p.cursor-1)
		case "down":
			p.cursor = min(max(0, len(ids)-1), p.cursor+1)
		case "enter":
			if len(ids) == 0 {
				return nil
			}
			if !p.fromForm {
				m.form = newProfileForm(p.source, true)
			}
			if m.form != nil {
				m.form.inputs[fModel].SetValue(ids[p.cursor])
				m.form.setFocus(fModel)
			}
			m.closeConnection()
		default:
			var cmd tea.Cmd
			p.input, cmd = p.input.Update(msg)
			p.cursor = 0
			return cmd
		}
		return nil
	}
	if key == "enter" {
		m.closeConnection()
		return nil
	}
	vp, _ := m.connectionViewport().Update(kit.DetailScrollMsg(msg))
	p.scroll = vp.YOffset()
	return nil
}

func (m Tab) connectionLayout() (int, int, int, string) {
	p := m.connection
	w := max(1, min(m.width, 96)-4)
	footer := "↑↓ scroll · esc back"
	if p.busy {
		footer = "esc cancel"
	} else if p.result.Catalog {
		footer = "↑↓ model · type filter · enter use · esc back\nshift+↑↓ scroll details"
		if m.height < 12 {
			footer = "↑↓ · type filter · enter use · esc"
		}
	}
	footer = kit.Wrap(footer, w, "")
	available := max(1, m.height-2-lipgloss.Height(footer))
	summary, rows := available, 0
	if p.result.Catalog {
		summary = max(1, min(8, min(available/2, available-2)))
		rows = max(1, available-summary-1)
	}
	return w, summary, rows, footer
}

func (m Tab) connectionViewport() viewport.Model {
	p := m.connection
	w, h, _, _ := m.connectionLayout()
	vp := viewport.New(viewport.WithWidth(w), viewport.WithHeight(h))
	var b bytes.Buffer
	if p.busy {
		if p.catalog {
			b.WriteString("Fetching an OpenAI-compatible catalog from the configured endpoint using profile credentials when present.\n")
		} else {
			b.WriteString("Checking the endpoint address without credentials.\n")
		}
		b.WriteString("No inference or config writes. Request timeout: 10 seconds.")
	} else {
		printConnection(&b, p.result)
	}
	vp.SetContent(kit.Wrap(strings.TrimSpace(b.String()), w, ""))
	vp.SetYOffset(p.scroll)
	return vp
}

func (m Tab) connectionView() string {
	p := m.connection
	title := "Check endpoint address"
	if p.catalog {
		title = "Discover models · OpenAI-compatible catalog"
	}
	panel := components.Panel{Title: title, Focused: true, Width: min(m.width, 96)}
	w, _, rows, footer := m.connectionLayout()
	lines := []string{m.connectionViewport().View()}
	if p.result.Catalog {
		p.input.SetWidth(max(1, w-8))
		lines = append(lines, p.input.View())
		ids := p.models()
		start, end := kit.Window(p.cursor, len(ids), rows)
		if len(ids) == 0 {
			lines = append(lines, "No matching models; edit the model manually")
		}
		for i := start; i < end; i++ {
			prefix := "  "
			if i == p.cursor {
				prefix = "▸ "
			}
			lines = append(lines, ansi.Truncate(prefix+ids[i], w, "…"))
		}
	}
	lines = append(lines, footer)
	return panel.Render(strings.Join(lines, "\n"))
}
