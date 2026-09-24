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
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

type sessionItem struct {
	s      agent.Session
	live   bool // processo do agente ainda aberto nesta sessão
	marked bool // selecionada para ação em lote (space)
}

// Title é o que identifica a conversa: o apelido, se houver, e o prompt.
func (i sessionItem) Title() string {
	if i.s.Alias != "" {
		return i.s.Alias + "  " + i.s.Title
	}
	return i.s.Title
}

// FilterValue: agente + apelido + título + basename do CWD. O path completo
// fica fora de propósito — o fuzzy caseando letras espalhadas pelos paths
// tornava o filtro inútil. Só o basename permite filtrar por projeto.
func (i sessionItem) FilterValue() string {
	v := kit.AgentLabel(i.s.AgentID) + " " + i.s.Title
	if i.s.Alias != "" {
		v = kit.AgentLabel(i.s.AgentID) + " " + i.s.Alias + " " + i.s.Title
	}
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
	label string   // "▸ lazyagents · claude (12)"
	ids   []string // IDs das sessões do grupo
}

func (h sessionGroupHeader) Title() string { return h.label }

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

// Colunas da tabela de conversas.
const (
	colMark = iota
	colAgent
	colTitle
	colProject
	colWhen
)

// narrowTable é a largura abaixo da qual a tabela só mostra a cor do agente
// e esconde o projeto (que continua no detalhe e no filtro).
const narrowTable = 64

// tableCols monta as colunas: marca (✓ selecionada, ● aberta agora), agente,
// conversa (flex), projeto e idade.
func (m Tab) tableCols(width int) []kit.Column {
	agentW, projW, markW := 6, 7, 0
	for _, it := range m.list.Items() {
		if it, ok := it.(sessionItem); ok {
			agentW = max(agentW, 2+lipgloss.Width(kit.AgentLabel(it.s.AgentID)))
			projW = max(projW, lipgloss.Width(projectOf(it.s)))
			if it.marked || it.live {
				markW = 1 // a coluna de marca só ocupa espaço quando há o que marcar
			}
		}
	}
	cols := []kit.Column{
		{Width: markW},
		{Title: "agente", Width: agentW},
		{Title: "conversa", Flex: true},
		{Title: "projeto", Width: min(projW, 18)},
		{Title: "quando", Width: 10, Align: lipgloss.Right},
	}
	if width < narrowTable {
		cols[colAgent] = kit.Column{Width: 1}
		cols[colProject] = kit.Column{}
	}
	return cols
}

// cells são as células de uma conversa na tabela.
func (m Tab) cells(it sessionItem, width int) []string {
	mark := ""
	switch {
	case it.marked:
		mark = kit.StShared.Render("✓")
	case it.live:
		mark = kit.StOn.Render("●")
	}
	tag := lipgloss.NewStyle().Foreground(theme.AgentColor(it.s.AgentID))
	agentCell := tag.Render("● " + kit.AgentLabel(it.s.AgentID))
	if width < narrowTable {
		agentCell = tag.Render("●")
	}
	title := it.s.Title
	if it.s.Alias != "" {
		title = kit.StTitle.Render(it.s.Alias) + "  " + kit.StHint.Render(it.s.Title)
	}
	return []string{mark, agentCell, title, kit.StHint.Render(projectOf(it.s)), kit.StHint.Render(relTime(it.s.MTime))}
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

func newSessionItem(s agent.Session, live bool) sessionItem {
	return sessionItem{s: s, live: live}
}

func (m Tab) loadCmd() tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		sessions, err := svc.List()
		return events.SessionsLoaded{Sessions: sessions, Err: err}
	}
}

// applyItems repõe os itens da lista respeitando o filtro de agente ativo e a
// busca full-text, se houver uma ativa; monta flat ou agrupada
// conforme m.grouped.
func (m *Tab) applyItems() tea.Cmd {
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
	return tea.Batch(cmd, m.layout())
}

// flatItems monta um sessionItem por sessão, sem cabeçalhos.
func (m *Tab) flatItems(sessions []agent.Session) []list.Item {
	items := make([]list.Item, 0, len(sessions))
	for _, s := range sessions {
		it := newSessionItem(s, m.svc.IsLive(s))
		it.marked = m.selected[s.ID]
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
func (m *Tab) groupedItems(sessions []agent.Session) []list.Item {
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
		label := fmt.Sprintf("▸ %s · %s (%d)", k.project, kit.AgentLabel(k.agentID), len(group))
		items = append(items, sessionGroupHeader{label: label, ids: ids})
		items = append(items, m.flatItems(group)...)
	}
	return items
}

// nextAgentFilter cicla todas → cada agente com sessões → todas.
func (m Tab) nextAgentFilter() string {
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
