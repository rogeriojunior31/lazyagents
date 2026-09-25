package skills

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

type skillItem struct {
	s      Skill
	issues []Issue // lint local do SKILL.md
}

func (i skillItem) Title() string {
	name := i.s.Name
	if !i.s.Valid {
		name += " ⚠"
	}
	if len(i.issues) > 0 {
		name += " " + kit.StWarn.Render("!")
	}
	if !i.s.InLibrary {
		name += " (local)"
	}
	return name
}

func (i skillItem) Description() string {
	if i.s.Warning != "" {
		return i.s.Warning
	}
	if i.s.Description == "" {
		return "(no description)"
	}
	return i.s.Description
}

func (i skillItem) FilterValue() string { return i.s.Name }

func (m Tab) scanCmd() tea.Cmd {
	svc, agents := m.svc, m.agents
	return func() tea.Msg {
		sk, err := svc.Scan(agents)
		return scannedMsg{skills: sk, err: err}
	}
}

// announceCmd publica o resultado do scan para as outras abas (a aba de
// agentes mostra quantas skills estão ativas em cada agente).
func (m Tab) announceCmd() tea.Cmd {
	active := map[string]int{}
	for _, sk := range m.skills {
		for id, st := range sk.States {
			if st.On {
				active[id]++
			}
		}
	}
	total := len(m.skills)
	return func() tea.Msg {
		return events.SkillsScanned{ActiveByAgent: active, Total: total}
	}
}

// click trata clique do mouse com coordenadas relativas ao corpo da view.
func (m Tab) click(msg tea.MouseClickMsg) (Tab, tea.Cmd) {
	if msg.Button != tea.MouseLeft || msg.X < 0 || msg.X >= m.width {
		return m, nil
	}
	switch m.mode {
	case skModeList:
		sp := m.split()
		x, y := msg.X, msg.Y
		if sp.Side && x >= sp.ListW || !sp.Side && y >= sp.ListH {
			return m, nil // detalhe: só a roda age nele
		}
		start, end, top := m.tableWindow(sp.ListW, sp.ListH)
		idx := start + y - top
		if y < top || idx >= end {
			return m, nil
		}
		col := kit.ColumnAt(sp.ListW, m.tableCols(sp.ListW), x) - colAgents
		if col >= 0 && col < len(m.targets) {
			m.col = col // clique na célula escolhe o agente; space alterna
		} else if idx == m.list.Index() {
			return m, m.openDocCmd() // segundo clique no nome abre a leitura
		}
		m.list.Select(idx)
		m.refreshDetail()
	case skModePick:
		start, end := m.picker.window(m.width, m.height)
		row := msg.Y - 1
		if row >= 0 && row < end-start {
			i := start + row
			m.picker.cursor = i
			m.picker.sel[i] = !m.picker.sel[i]
		}
	case skModeRegistryPick:
		start, end := kit.Window(m.regPicker.cursor, len(m.regPicker.items), m.height-4)
		row := msg.Y - 1
		if row >= 0 && row < end-start {
			m.regPicker.cursor = start + row
		}
	case skModeBackup:
		start, end := kit.Window(m.backupPicker.cursor, len(m.backupPicker.backups), m.height-5)
		row := msg.Y - 1
		if row >= 0 && row < end-start {
			m.backupPicker.cursor = start + row
		}
	case skModeDoc:
		// clique fecha a leitura (mesmo gesto de esc)
		m.mode = skModeList
	case skModeProfiles:
		start, end := kit.Window(m.profileCursor, len(m.profileNames), m.height-8)
		row := msg.Y - 1
		if msg.X < min(m.width, 72) && row >= 0 && row < end-start {
			m.profileCursor = start + row
		}

	}
	return m, nil
}

func (m Tab) updateList(msg tea.KeyPressMsg) (Tab, tea.Cmd) {
	if m.list.SettingFilter() {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	sel, ok := m.selected()
	key := msg.String()
	// ←/→ escolhem o agente (coluna da matriz); shift+↑/↓ rolam o detalhe.
	switch key {
	case "left", "h":
		m.col = max(0, m.col-1)
		return m, nil
	case "right", "l":
		m.col = max(0, min(len(m.targets)-1, m.col+1))
		return m, nil
	}
	if kit.DetailScroll[key] {
		var cmd tea.Cmd
		m.detailVP, cmd = m.detailVP.Update(kit.DetailScrollMsg(msg))
		return m, cmd
	}
	switch {
	case key == "enter":
		if ok {
			return m, m.openDocCmd()
		}
		return m, nil
	case key >= "1" && key <= "9":
		idx := int(key[0] - '1')
		if ok && idx < len(m.targets) {
			return m, m.toggleCmd(sel, m.targets[idx])
		}
		return m, nil
	case key == "space":
		if ok && m.col < len(m.targets) {
			return m, m.toggleCmd(sel, m.targets[m.col])
		}
		return m, nil
	case key == "a":
		if ok {
			return m, m.opCmd("enabled in all agents", func() error {
				return m.svc.EnableAll(sel, m.agents)
			})
		}
	case key == "x":
		if ok {
			return m, m.opCmd("disabled in all agents", func() error {
				return m.svc.DisableAll(sel, m.agents)
			})
		}
	case key == "u":
		if ok {
			if sel.Origin == nil || sel.Origin.Type != "git" {
				m.setToast("skill has no git source — install it from GitHub to update it", true)
				return m, nil
			}
			m.pendingUpdate = sel
			m.ckind = confirmKindUpdate
			m.confirm = components.NewConfirm(fmt.Sprintf("Update %q from GitHub?", sel.Name))
			m.mode = skModeConfirm
		}
		return m, nil
	case key == "U":
		return m, tea.Batch(m.beginSpin("verificando updates…"), m.checkUpdatesCmd())
	case key == "b":
		if ok {
			return m, m.listBackupsCmd(sel.Dir)
		}
		return m, nil
	case key == "e":
		if ok {
			return m, editCmd(sel.Name, sel.Path)
		}
	case key == "o":
		if ok {
			return m, m.adoptCmd(sel)
		}
	case key == "A":
		names := m.localNames()
		if len(names) == 0 {
			m.setToast("no local skills to adopt", false)
			return m, nil
		}
		m.ckind = confirmKindAdoptAll
		m.confirm = components.NewConfirm(fmt.Sprintf("Adopt %d local skill(s) into the library?\n%s",
			len(names), strings.Join(names, ", ")))
		m.mode = skModeConfirm
		return m, nil
	case key == "d":
		if ok {
			if !sel.InLibrary {
				m.setToast("local skill: remove it with the tool that created it (or adopt it with o)", true)
				return m, nil
			}
			m.pendingRemove = sel
			m.confirm = components.NewConfirm(fmt.Sprintf("Remove %q from the library and all agents?", sel.Name))
			m.mode = skModeConfirm
		}
		return m, nil
	case key == "i":
		m.mode = skModeInstall
		m.input.Placeholder = "GitHub URL, user/repo, folder or .zip file"
		m.input.SetValue("")
		return m, m.input.Focus()
	case key == "n":
		m.mode = skModeNew
		m.input.Placeholder = "skill-name (kebab-case)"
		m.input.SetValue("")
		return m, m.input.Focus()
	case key == "S":
		m.mode = skModeRegistry
		m.input.Placeholder = "search term (SKILL.md on GitHub)"
		m.input.SetValue("")
		return m, m.input.Focus()
	case key == "p":
		return m, m.loadProfilesCmd()
	case key == "r":
		m.setToast("recarregando…", false)
		return m, m.scanCmd()
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	m.refreshDetail() // o cursor pode ter mudado
	return m, cmd
}

// toggleCmd alterna a skill num agente específico, com mensagens claras para
// os estados não alternáveis (local / via dir compartilhado).
func (m Tab) toggleCmd(sk Skill, ag agent.Agent) tea.Cmd {
	st := sk.States[ag.ID]
	switch {
	case st.On && st.Local:
		msg := fmt.Sprintf("%s in %s is local (unmanaged) — o to adopt", sk.Name, ag.Name)
		return func() tea.Msg { return skillOpMsg{err: fmt.Errorf("%s", msg)} }
	case st.On && !st.Managed:
		msg := fmt.Sprintf("%s reaches %s via %s (shared) — x disables in all", sk.Name, ag.Name, st.Via)
		return func() tea.Msg { return skillOpMsg{err: fmt.Errorf("%s", msg)} }
	case st.On:
		return m.opCmd(fmt.Sprintf("%s disabled in %s", sk.Name, ag.Name), func() error {
			return m.svc.Disable(sk, ag)
		})
	default:
		return m.opCmd(fmt.Sprintf("%s enabled in %s", sk.Name, ag.Name), func() error {
			return m.svc.Enable(sk, ag)
		})
	}
}

func (m Tab) adoptCmd(sk Skill) tea.Cmd {
	if sk.InLibrary {
		return func() tea.Msg { return skillOpMsg{err: fmt.Errorf("%s is already in the library", sk.Name)} }
	}
	for _, ag := range m.agents {
		st := sk.States[ag.ID]
		if st.On && st.Local {
			agCopy := ag
			return m.opCmd(fmt.Sprintf("%s adopted into the library (from %s)", sk.Name, ag.Name), func() error {
				return m.svc.Adopt(sk, agCopy)
			})
		}
	}
	return func() tea.Msg {
		return skillOpMsg{err: fmt.Errorf("%s has no local copy to adopt", sk.Name)}
	}
}

func (m Tab) opCmd(okMsg string, op func() error) tea.Cmd {
	return func() tea.Msg { return skillOpMsg{verb: okMsg, err: op()} }
}

// localNames lista os nomes das skills locais (fora da biblioteca) — o que o
// contador do título e a tecla A (adota todas) precisam saber.
func (m Tab) localNames() []string {
	var names []string
	for _, sk := range m.skills {
		if !sk.InLibrary {
			names = append(names, sk.Name)
		}
	}
	return names
}

func (m Tab) selected() (Skill, bool) {
	it, ok := m.list.SelectedItem().(skillItem)
	if !ok {
		return Skill{}, false
	}
	return it.s, true
}

// stateMark é o marcador de uma célula da matriz, o mesmo do detalhe:
// ● ativa, ▪ local, ◆ via outro diretório, ○ inativa.
func stateMark(st AgentState) string {
	switch {
	case st.On && st.Managed:
		return kit.StOn.Render("●")
	case st.On && st.Local:
		return kit.StLocal.Render("▪")
	case st.On:
		return kit.StShared.Render("◆")
	}
	return kit.StOff.Render("○")
}

// updateMark é o resultado de CheckUpdates: ↑ disponível, ~ editada.
func (m Tab) updateMark(s Skill) string {
	switch m.updateStatus[s.Dir] {
	case UpdateStatusAvailable:
		return kit.StLocal.Render("↑")
	case UpdateStatusLocallyEdited:
		return kit.StHint.Render("~")
	}
	return ""
}

// Índices fixos das colunas da matriz; os agentes vêm a partir de colAgents.
const (
	colName = iota
	colDesc
	colUpdate
	colAgents
)

// tableCols monta as colunas da matriz para width: nome, descrição (flex),
// updates (só depois de U) e um agente por coluna, a coluna do cursor
// sublinhada. Sem espaço para a descrição, o nome vira a coluna flexível.
func (m Tab) tableCols(width int) []kit.Column {
	agents := kit.AgentColumns(m.targets, width/2)
	agentsW := 0
	for i, ag := range m.targets {
		if i == m.col {
			label := ansi.Strip(agents[i].Title)
			agents[i].Title = lipgloss.NewStyle().Foreground(theme.AgentColor(ag.ID)).Underline(true).Bold(true).Render(label)
		}
		agentsW += agents[i].Width + 2
	}
	nameW := 8
	for _, it := range m.list.Items() {
		nameW = max(nameW, lipgloss.Width(it.(skillItem).Title()))
	}
	nameW = min(nameW, 28)
	updW := 0
	if len(m.updateStatus) > 0 {
		updW = 1
	}
	cols := []kit.Column{
		{Title: "skill", Width: nameW},
		{Title: "description", Flex: true},
		{Width: updW},
	}
	if width-2-nameW-2-agentsW-updW*3 < 12 {
		cols[colName] = kit.Column{Title: "skill", Flex: true}
		cols[colDesc] = kit.Column{}
	}
	return append(cols, agents...)
}

// cells são as células de uma linha da matriz. Na linha selecionada, a
// célula do agente sob o cursor aparece invertida: é a que space alterna.
func (m Tab) cells(it skillItem, selected bool) []string {
	out := []string{it.Title(), kit.StHint.Render(it.Description()), m.updateMark(it.s)}
	for i, ag := range m.targets {
		mark := stateMark(it.s.States[ag.ID])
		if selected && i == m.col {
			mark = lipgloss.NewStyle().Reverse(true).Render(ansi.Strip(mark))
		}
		out = append(out, mark)
	}
	return out
}

func (m *Tab) rebuildListItems() tea.Cmd {
	items := make([]list.Item, 0, len(m.skills))
	for _, s := range m.skills {
		items = append(items, skillItem{s: s, issues: Validate(s)})
	}
	cmd := m.list.SetItems(items)
	m.refreshDetail()
	return cmd
}

// selectedIssues devolve as issues de lint da skill selecionada, já
// calculadas em rebuildListItems (sem reler o SKILL.md a cada render).
func (m Tab) selectedIssues() []Issue {
	if it, ok := m.list.SelectedItem().(skillItem); ok {
		return it.issues
	}
	return nil
}
