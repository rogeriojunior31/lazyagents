package sessions

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

type sessionItem struct {
	s     agent.Session
	title string
	desc  string
}

func (i sessionItem) Title() string { return i.title }

func (i sessionItem) Description() string { return i.desc }

// FilterValue: agente + título + basename do CWD. O path completo fica fora
// de propósito — o fuzzy caseando letras espalhadas pelos paths tornava o
// filtro inútil. Só o basename permite filtrar por projeto.
func (i sessionItem) FilterValue() string {
	v := tagLabel(i.s.AgentID) + " " + i.s.Title
	if base := filepath.Base(i.s.CWD); base != "" && base != "." {
		v += " " + base
	}
	return v
}

// sessionGroupHeader é a linha de cabeçalho da vista agrupada — não
// carrega uma sessão, então as ações que fazem type assertion pra
// sessionItem (enter, v, d, ...) já não fazem nada nela de graça; só o space
// (seleção em lote) trata o header explicitamente.
type sessionGroupHeader struct {
	label string   // "▸ claude · lazyagents (12)"
	ids   []string // IDs das sessões do grupo
}

func (h sessionGroupHeader) Title() string { return h.label }

func (h sessionGroupHeader) Description() string { return "" }

func (h sessionGroupHeader) FilterValue() string { return h.label }

// projectOf devolve o "projeto" de agrupamento de uma sessão: o basename do
// CWD, ou "sem projeto" se vazio/raiz.
func projectOf(s agent.Session) string {
	base := filepath.Base(s.CWD)
	if s.CWD == "" || base == "" || base == "." || base == "/" {
		return "sem projeto"
	}
	return base
}

func tagLabel(id string) string {
	return strings.TrimSuffix(strings.TrimSuffix(id, "-cli"), "-code")
}

// agentTag devolve a tag colorida do agente, com largura fixa para os títulos
// da lista ficarem alinhados em coluna.
func agentTag(id string) string {
	label := tagLabel(id)
	pad := strings.Repeat(" ", max(0, 8-len(label)))
	return lipgloss.NewStyle().Foreground(theme.AgentColor(id)).Render("⏺ "+label) + pad
}

// relTime formata a idade da sessão de forma humana.
func relTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "agora"
	case d < time.Hour:
		return fmt.Sprintf("há %dmin", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("há %dh", int(d.Hours()))
	case d < 48*time.Hour:
		return "ontem"
	case d < 7*24*time.Hour:
		return fmt.Sprintf("há %dd", int(d.Hours()/24))
	default:
		return t.Format("02/01/2006")
	}
}

// humanCount formata uma contagem de tokens de forma compacta (12345 → 12.3k).
func humanCount(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// formatUsage resume o consumo de tokens de uma sessão.
func formatUsage(u agent.Usage) string {
	parts := []string{humanCount(u.Input) + " in", humanCount(u.Output) + " out"}
	if cache := u.CacheRead + u.CacheWrite; cache > 0 {
		parts = append(parts, "cache "+humanCount(cache))
	}
	return strings.Join(parts, " · ")
}

func newSessionItem(s agent.Session, home string, live bool) sessionItem {
	cwd := s.CWD
	if cwd == "" {
		cwd = "(pasta desconhecida)"
	}
	title := agentTag(s.AgentID) + " " + s.Title
	if live {
		title = kit.StOn.Render("● ") + title
	}
	return sessionItem{
		s:     s,
		title: title,
		desc:  "  " + relTime(s.MTime) + " · " + core.Tilde(cwd, home),
	}
}

func (m Sessions) loadCmd() tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		sessions, err := svc.List()
		return events.SessionsLoaded{Sessions: sessions, Err: err}
	}
}

// applyItems repõe os itens da lista respeitando o filtro de agente ativo e a
// busca full-text, se houver uma ativa; monta flat ou agrupada
// conforme m.grouped.
func (m *Sessions) applyItems() tea.Cmd {
	var filtered []agent.Session
	for _, s := range m.sessions {
		if m.agentFilter != "" && s.AgentID != m.agentFilter {
			continue
		}
		if m.searchIDs != nil && !m.searchIDs[s.ID] {
			continue
		}
		filtered = append(filtered, s)
	}
	var items []list.Item
	if m.grouped {
		items = m.groupedItems(filtered)
	} else {
		items = m.flatItems(filtered)
	}
	cmd := m.list.SetItems(items)
	return tea.Batch(cmd, m.refreshDetail())
}

// flatItems monta um sessionItem por sessão, sem cabeçalhos.
func (m *Sessions) flatItems(sessions []agent.Session) []list.Item {
	items := make([]list.Item, 0, len(sessions))
	for _, s := range sessions {
		it := newSessionItem(s, m.home, m.svc.IsLive(s))
		if m.selected[s.ID] {
			it.title = "✓ " + it.title
		}
		items = append(items, it)
	}
	return items
}

// sessionGroupKey identifica um grupo agente+projeto.
type sessionGroupKey struct{ agentID, project string }

// groupedItems agrupa por agente+projeto, um sessionGroupHeader seguido dos
// sessionItem do grupo. sessions já vem ordenado por MTime (session.List) —
// a ordem dentro do grupo sai de graça preservando essa ordem; os grupos
// aparecem na ordem da primeira sessão que os originou.
func (m *Sessions) groupedItems(sessions []agent.Session) []list.Item {
	var order []sessionGroupKey
	byGroup := make(map[sessionGroupKey][]agent.Session)
	for _, s := range sessions {
		k := sessionGroupKey{s.AgentID, projectOf(s)}
		if _, ok := byGroup[k]; !ok {
			order = append(order, k)
		}
		byGroup[k] = append(byGroup[k], s)
	}
	items := make([]list.Item, 0, len(sessions)+len(order))
	for _, k := range order {
		group := byGroup[k]
		ids := make([]string, len(group))
		for i, s := range group {
			ids[i] = s.ID
		}
		label := fmt.Sprintf("▸ %s · %s (%d)", tagLabel(k.agentID), k.project, len(group))
		items = append(items, sessionGroupHeader{label: label, ids: ids})
		items = append(items, m.flatItems(group)...)
	}
	return items
}

// nextAgentFilter cicla todas → cada agente com sessões → todas.
func (m Sessions) nextAgentFilter() string {
	var cycle []string
	seen := map[string]bool{}
	for _, s := range m.sessions {
		if !seen[s.AgentID] {
			seen[s.AgentID] = true
			cycle = append(cycle, s.AgentID)
		}
	}
	if len(cycle) == 0 {
		return ""
	}
	for i, id := range cycle {
		if id == m.agentFilter {
			if i+1 < len(cycle) {
				return cycle[i+1]
			}
			return "" // fim do ciclo: volta para todas
		}
	}
	return cycle[0]
}
