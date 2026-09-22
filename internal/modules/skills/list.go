package skills

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
)

type skillItem struct {
	s      Skill
	badge  string
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
	return i.badge + "  " + name
}

func (i skillItem) Description() string {
	if i.s.Warning != "" {
		return i.s.Warning
	}
	if i.s.Description == "" {
		return "(sem descrição)"
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
	if msg.Button != tea.MouseLeft {
		return m, nil
	}
	switch m.mode {
	case skModeList:
		if m.width < 76 && m.paneFocus == kit.PaneDetail {
			return m, nil
		}
		if msg.X >= m.listWidth() {
			m.paneFocus = kit.PaneDetail // clique no painel de detalhe o foca
			return m, nil
		}
		m.paneFocus = kit.PaneList
		idx := kit.ListIndexAt(&m.list, msg.Y)
		if idx < 0 {
			return m, nil
		}
		if idx == m.list.Index() {
			return m, m.openDocCmd() // segundo clique abre a leitura
		}
		m.list.Select(idx)
		m.refreshDetail()
	case skModePick:
		// itens no Panel: borda superior (1), depois 1 item por linha
		row := msg.Y - 1
		if row >= 0 && row < len(m.picker.items) {
			m.picker.cursor = row
			m.picker.sel[row] = !m.picker.sel[row]
		}
	case skModeDoc:
		// clique fecha a leitura (mesmo gesto de esc)
		m.mode = skModeList
	case skModeProfiles:
		// itens no Panel: borda superior (1), depois 1 item por linha
		row := msg.Y - 1
		if row >= 0 && row < len(m.profileNames) {
			m.profileCursor = row
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
	// ←/→ movem o foco entre lista e detalhe.
	switch key {
	case "left":
		m.paneFocus = kit.PaneList
		return m, nil
	case "right":
		m.paneFocus = kit.PaneDetail
		return m, nil
	}
	// Com o detalhe focado, as teclas de rolagem vão para o viewport dele; as
	// demais continuam agindo sobre a skill selecionada.
	if m.paneFocus == kit.PaneDetail && kit.DetailScrollKeys[key] {
		var cmd tea.Cmd
		m.detailVP, cmd = m.detailVP.Update(msg)
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
		if ok {
			return m, m.smartToggleCmd(sel)
		}
	case key == "a":
		if ok {
			return m, m.opCmd("ativada em todos os agentes", func() error {
				return m.svc.EnableAll(sel, m.agents)
			})
		}
	case key == "x":
		if ok {
			return m, m.opCmd("desativada em todos os agentes", func() error {
				return m.svc.DisableAll(sel, m.agents)
			})
		}
	case key == "u":
		if ok {
			if sel.Origin == nil || sel.Origin.Type != "git" {
				m.setToast("skill sem origem git — instale do GitHub para poder atualizar", true)
				return m, nil
			}
			m.pendingUpdate = sel
			m.ckind = confirmKindUpdate
			m.confirm = components.NewConfirm(fmt.Sprintf("Atualizar %q do GitHub?", sel.Name))
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
			m.setToast("nenhuma skill local para adotar", false)
			return m, nil
		}
		m.ckind = confirmKindAdoptAll
		m.confirm = components.NewConfirm(fmt.Sprintf("Adotar %d skill(s) local(is) para a biblioteca?\n%s",
			len(names), strings.Join(names, ", ")))
		m.mode = skModeConfirm
		return m, nil
	case key == "d":
		if ok {
			if !sel.InLibrary {
				m.setToast("skill local: remova pela ferramenta que a criou (ou adote com o)", true)
				return m, nil
			}
			m.pendingRemove = sel
			m.confirm = components.NewConfirm(fmt.Sprintf("Remover %q da biblioteca e de todos os agentes?", sel.Name))
			m.mode = skModeConfirm
		}
		return m, nil
	case key == "i":
		m.mode = skModeInstall
		m.input.Placeholder = "URL do GitHub, usuario/repo, pasta ou arquivo .zip"
		m.input.SetValue("")
		return m, m.input.Focus()
	case key == "n":
		m.mode = skModeNew
		m.input.Placeholder = "nome-da-skill (kebab-case)"
		m.input.SetValue("")
		return m, m.input.Focus()
	case key == "S":
		m.mode = skModeRegistry
		m.input.Placeholder = "termo de busca (SKILL.md no GitHub)"
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
		msg := fmt.Sprintf("%s em %s é local (não gerenciada) — o para adotar", sk.Name, ag.Name)
		return func() tea.Msg { return skillOpMsg{err: fmt.Errorf("%s", msg)} }
	case st.On && !st.Managed:
		msg := fmt.Sprintf("%s chega a %s via %s (compartilhado) — x desativa em todos", sk.Name, ag.Name, st.Via)
		return func() tea.Msg { return skillOpMsg{err: fmt.Errorf("%s", msg)} }
	case st.On:
		return m.opCmd(fmt.Sprintf("%s desativada em %s", sk.Name, ag.Name), func() error {
			return m.svc.Disable(sk, ag)
		})
	default:
		return m.opCmd(fmt.Sprintf("%s ativada em %s", sk.Name, ag.Name), func() error {
			return m.svc.Enable(sk, ag)
		})
	}
}

// smartToggleCmd (space): se está desativada em algum agente, ativa em todos;
// senão, desativa em todos.
func (m Tab) smartToggleCmd(sk Skill) tea.Cmd {
	someOff := false
	for _, ag := range m.targets {
		if !sk.States[ag.ID].On {
			someOff = true
			break
		}
	}
	if someOff {
		return m.opCmd(sk.Name+" ativada em todos os agentes", func() error {
			return m.svc.EnableAll(sk, m.agents)
		})
	}
	return m.opCmd(sk.Name+" desativada em todos os agentes", func() error {
		return m.svc.DisableAll(sk, m.agents)
	})
}

func (m Tab) adoptCmd(sk Skill) tea.Cmd {
	if sk.InLibrary {
		return func() tea.Msg { return skillOpMsg{err: fmt.Errorf("%s já está na biblioteca", sk.Name)} }
	}
	for _, ag := range m.agents {
		st := sk.States[ag.ID]
		if st.On && st.Local {
			agCopy := ag
			return m.opCmd(fmt.Sprintf("%s adotada para a biblioteca (origem: %s)", sk.Name, ag.Name), func() error {
				return m.svc.Adopt(sk, agCopy)
			})
		}
	}
	return func() tea.Msg {
		return skillOpMsg{err: fmt.Errorf("%s não tem cópia local adotável", sk.Name)}
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

// badge monta a coluna de status por agente: letra = ativa, · = inativa.
// Quando há resultado de CheckUpdates, acrescenta ↑ (disponível) ou ~ (editada).
func (m Tab) badge(s Skill) string {
	var b strings.Builder
	for _, ag := range m.targets {
		st := s.States[ag.ID]
		switch {
		case st.On && st.Managed:
			b.WriteString(kit.StOn.Render(ag.Short))
		case st.On && st.Local:
			b.WriteString(kit.StLocal.Render(ag.Short))
		case st.On:
			b.WriteString(kit.StShared.Render(ag.Short))
		default:
			b.WriteString(kit.StOff.Render("·"))
		}
	}
	switch m.updateStatus[s.Dir] {
	case UpdateStatusAvailable:
		b.WriteString(kit.StLocal.Render("↑"))
	case UpdateStatusLocallyEdited:
		b.WriteString(kit.StHint.Render("~"))
	}
	return b.String()
}

func (m *Tab) rebuildListItems() tea.Cmd {
	items := make([]list.Item, 0, len(m.skills))
	for _, s := range m.skills {
		items = append(items, skillItem{s: s, badge: m.badge(s), issues: Validate(s)})
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
