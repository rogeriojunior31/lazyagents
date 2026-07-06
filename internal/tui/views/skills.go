package views

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"lazyskills/internal/agent"
	"lazyskills/internal/skill"
	"lazyskills/internal/tui/components"
)

type skMode int

const (
	skModeList skMode = iota
	skModeInstall
	skModePick
	skModeConfirm
	skModeDoc
)

// Skills é a aba principal: matriz skill × agente com toggle por symlink.
type Skills struct {
	svc     *skill.Service
	agents  []agent.Agent // todos os detectados
	targets []agent.Agent // instalados com dir de skills (colunas da matriz)
	skills  []skill.Skill

	list    list.Model
	input   textinput.Model
	picker  picker
	confirm components.Confirm
	vp      viewport.Model
	docName string

	mode          skMode
	pendingRemove skill.Skill
	toast         string
	toastErr      bool
	width, height int
}

type skillsScanMsg struct {
	skills []skill.Skill
	err    error
}

type skillOpMsg struct {
	verb string
	err  error
}

type discoverMsg struct {
	found   []skill.Found
	cleanup string
	err     error
}

type installDoneMsg struct {
	names []string
	err   error
}

type docMsg struct {
	name    string
	content string
	err     error
}

type skillItem struct {
	s     skill.Skill
	badge string
}

func (i skillItem) Title() string {
	name := i.s.Name
	if !i.s.Valid {
		name += " ⚠"
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

func NewSkills(svc *skill.Service) Skills {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	in := textinput.New()
	in.Placeholder = "URL do GitHub, usuario/repo, pasta ou arquivo .zip"
	in.CharLimit = 1024
	in.SetWidth(60)
	return Skills{svc: svc, list: l, input: in, vp: viewport.New()}
}

func (m Skills) Init() tea.Cmd { return nil }

func (m Skills) Capturing() bool {
	return m.mode != skModeList || m.list.SettingFilter()
}

func (m Skills) scanCmd() tea.Cmd {
	svc, agents := m.svc, m.agents
	return func() tea.Msg {
		sk, err := svc.Scan(agents)
		return skillsScanMsg{skills: sk, err: err}
	}
}

func (m Skills) Update(msg tea.Msg) (Skills, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, nil

	case AgentsMsg:
		m.agents = msg.Agents
		m.targets = nil
		for _, ag := range msg.Agents {
			if ag.Installed && ag.SupportsSkills() {
				m.targets = append(m.targets, ag)
			}
		}
		return m, m.scanCmd()

	case skillsScanMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
			return m, nil
		}
		m.skills = msg.skills
		items := make([]list.Item, 0, len(msg.skills))
		for _, s := range msg.skills {
			items = append(items, skillItem{s: s, badge: m.badge(s)})
		}
		return m, m.list.SetItems(items)

	case skillOpMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
		} else {
			m.setToast(msg.verb, false)
		}
		return m, m.scanCmd()

	case discoverMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
			m.mode = skModeList
			return m, nil
		}
		m.picker = newPicker(msg.found, msg.cleanup)
		m.mode = skModePick
		return m, nil

	case installDoneMsg:
		m.mode = skModeList
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
		} else if len(msg.names) > 0 {
			m.setToast("instaladas: "+strings.Join(msg.names, ", "), false)
		} else {
			m.setToast("nada instalado", true)
		}
		return m, m.scanCmd()

	case docMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
			return m, nil
		}
		m.docName = msg.name
		m.vp.SetContent(renderMarkdown(msg.content, m.width-2))
		m.vp.GotoTop()
		m.mode = skModeDoc
		return m, nil

	case tea.MouseWheelMsg:
		switch m.mode {
		case skModeDoc:
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			return m, cmd
		case skModeList:
			if msg.Button == tea.MouseWheelUp {
				m.list.CursorUp()
			} else if msg.Button == tea.MouseWheelDown {
				m.list.CursorDown()
			}
		case skModePick:
			p := &m.picker
			if msg.Button == tea.MouseWheelUp && p.cursor > 0 {
				p.cursor--
			} else if msg.Button == tea.MouseWheelDown && p.cursor < len(p.items)-1 {
				p.cursor++
			}
		}
		return m, nil

	case tea.MouseClickMsg:
		return m.click(msg)

	case tea.KeyPressMsg:
		switch m.mode {
		case skModeInstall:
			return m.updateInstall(msg)
		case skModePick:
			return m.updatePick(msg)
		case skModeConfirm:
			return m.updateConfirm(msg)
		case skModeDoc:
			return m.updateDoc(msg)
		default:
			return m.updateList(msg)
		}
	}
	return m, nil
}

// click trata clique do mouse com coordenadas relativas ao corpo da view.
func (m Skills) click(msg tea.MouseClickMsg) (Skills, tea.Cmd) {
	if msg.Button != tea.MouseLeft {
		return m, nil
	}
	switch m.mode {
	case skModeList:
		if msg.X >= m.listWidth() {
			return m, nil
		}
		idx := listIndexAt(&m.list, msg.Y)
		if idx < 0 {
			return m, nil
		}
		if idx == m.list.Index() {
			return m, m.openDocCmd() // segundo clique abre a leitura
		}
		m.list.Select(idx)
	case skModePick:
		// linhas do picker: título + vazia, itens 1 por linha a partir da 2
		row := msg.Y - 2
		if row >= 0 && row < len(m.picker.items) {
			m.picker.cursor = row
			m.picker.sel[row] = !m.picker.sel[row]
		}
	case skModeDoc:
		// clique fecha a leitura (mesmo gesto de esc)
		m.mode = skModeList
	}
	return m, nil
}

// listIndexAt converte uma linha da tela num índice absoluto da lista.
// Layout do delegate padrão: 2 linhas de cabeçalho da lista (status bar +
// espaçamento) e 3 linhas por item (título + descrição + espaçamento).
func listIndexAt(l *list.Model, y int) int {
	row := y - 2
	if row < 0 {
		return -1
	}
	idx := l.Paginator.Page*l.Paginator.PerPage + row/3
	if idx >= len(l.VisibleItems()) {
		return -1
	}
	return idx
}

func (m Skills) openDocCmd() tea.Cmd {
	sel, ok := m.selected()
	if !ok {
		return nil
	}
	return func() tea.Msg {
		data, err := os.ReadFile(filepath.Join(sel.Path, "SKILL.md"))
		if err != nil {
			return docMsg{err: fmt.Errorf("lendo SKILL.md de %s: %w", sel.Name, err)}
		}
		return docMsg{name: sel.Name, content: string(data)}
	}
}

func (m Skills) updateDoc(msg tea.KeyPressMsg) (Skills, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "enter":
		m.mode = skModeList
		return m, nil
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m Skills) updateList(msg tea.KeyPressMsg) (Skills, tea.Cmd) {
	if m.list.SettingFilter() {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	sel, ok := m.selected()
	key := msg.String()
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
	case key == "o":
		if ok {
			return m, m.adoptCmd(sel)
		}
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
		m.input.SetValue("")
		return m, m.input.Focus()
	case key == "r":
		m.setToast("recarregando…", false)
		return m, m.scanCmd()
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Skills) updateInstall(msg tea.KeyPressMsg) (Skills, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = skModeList
		m.input.Blur()
		return m, nil
	case "enter":
		src := strings.TrimSpace(m.input.Value())
		if src == "" {
			return m, nil
		}
		m.input.Blur()
		m.setToast("procurando skills em "+src+"…", false)
		svc := m.svc
		return m, func() tea.Msg {
			found, cleanup, err := svc.Discover(src)
			return discoverMsg{found: found, cleanup: cleanup, err: err}
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Skills) updatePick(msg tea.KeyPressMsg) (Skills, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = skModeList
		if m.picker.cleanup != "" {
			os.RemoveAll(m.picker.cleanup)
		}
		return m, nil
	case "enter":
		chosen := m.picker.chosen()
		if len(chosen) == 0 {
			m.setToast("nenhuma skill selecionada (space marca)", true)
			return m, nil
		}
		svc, cleanup := m.svc, m.picker.cleanup
		return m, func() tea.Msg {
			names, err := svc.Install(chosen)
			if cleanup != "" {
				os.RemoveAll(cleanup)
			}
			return installDoneMsg{names: names, err: err}
		}
	default:
		m.picker = m.picker.update(msg)
		return m, nil
	}
}

func (m Skills) updateConfirm(msg tea.KeyPressMsg) (Skills, tea.Cmd) {
	var res components.Result
	m.confirm, res = m.confirm.Update(msg)
	switch res {
	case components.Yes:
		m.mode = skModeList
		sel := m.pendingRemove
		return m, m.opCmd("removida (backup em ~/.lazyskills/backups)", func() error {
			return m.svc.Remove(sel, m.agents)
		})
	case components.No:
		m.mode = skModeList
	}
	return m, nil
}

// toggleCmd alterna a skill num agente específico, com mensagens claras para
// os estados não alternáveis (local / via dir compartilhado).
func (m Skills) toggleCmd(sk skill.Skill, ag agent.Agent) tea.Cmd {
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
func (m Skills) smartToggleCmd(sk skill.Skill) tea.Cmd {
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

func (m Skills) adoptCmd(sk skill.Skill) tea.Cmd {
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

func (m Skills) opCmd(okMsg string, op func() error) tea.Cmd {
	return func() tea.Msg { return skillOpMsg{verb: okMsg, err: op()} }
}

func (m Skills) selected() (skill.Skill, bool) {
	it, ok := m.list.SelectedItem().(skillItem)
	if !ok {
		return skill.Skill{}, false
	}
	return it.s, true
}

// badge monta a coluna de status por agente: letra = ativa, · = inativa.
func (m Skills) badge(s skill.Skill) string {
	var b strings.Builder
	for _, ag := range m.targets {
		st := s.States[ag.ID]
		switch {
		case st.On && st.Managed:
			b.WriteString(stOn.Render(ag.Short))
		case st.On && st.Local:
			b.WriteString(stLocal.Render(ag.Short))
		case st.On:
			b.WriteString(stShared.Render(ag.Short))
		default:
			b.WriteString(stOff.Render("·"))
		}
	}
	return b.String()
}

func (m Skills) listWidth() int { return m.width * 2 / 5 }

func (m *Skills) layout() {
	if m.width == 0 {
		return
	}
	bodyH := m.height - 2 // toast + hints
	if bodyH < 3 {
		bodyH = 3
	}
	m.list.SetSize(m.listWidth(), bodyH)
	m.vp.SetWidth(m.width)
	m.vp.SetHeight(bodyH)
}

func (m *Skills) setToast(s string, isErr bool) {
	m.toast, m.toastErr = s, isErr
}

func (m Skills) View() string {
	switch m.mode {
	case skModeInstall:
		return lipgloss.JoinVertical(lipgloss.Left,
			stTitle.Render("Instalar skill"),
			"",
			"Origem (GitHub, pasta local ou .zip):",
			m.input.View(),
			"",
			stHint.Render("enter procura skills · esc cancela"),
			"",
			m.toastLine(),
		)
	case skModePick:
		return m.picker.view(m.height - 2)
	case skModeConfirm:
		return m.confirm.View()
	case skModeDoc:
		head := stTitle.Render(m.docName) + stHint.Render("  SKILL.md · esc volta · ↑↓/roda do mouse rola")
		return lipgloss.JoinVertical(lipgloss.Left, head, m.vp.View())
	}

	listW := m.listWidth()
	detailW := m.width - listW - 3
	if detailW < 20 {
		detailW = 20
	}
	detail := lipgloss.NewStyle().Width(detailW).Render(m.detailView())
	body := lipgloss.JoinHorizontal(lipgloss.Top, m.list.View(), "  ", detail)
	hints := stHint.Render("enter lê · 1-9 alterna no agente · space/a/x todos · i instala · o adota · d remove · / filtra · r recarrega")
	return lipgloss.JoinVertical(lipgloss.Left, body, hints, m.toastLine())
}

func (m Skills) detailView() string {
	sel, ok := m.selected()
	if !ok {
		return stHint.Render("Nenhuma skill. Pressione i para instalar (GitHub, pasta ou zip).")
	}
	var b strings.Builder
	b.WriteString(stTitle.Render(sel.Name) + "\n")
	if sel.Description != "" {
		b.WriteString(stText.Render(truncate(sel.Description, 300)) + "\n")
	}
	b.WriteString("\n")
	if sel.InLibrary {
		b.WriteString(stHint.Render("biblioteca: "+sel.Path) + "\n")
	} else {
		b.WriteString(stLocal.Render("fora da biblioteca — o para adotar") + "\n")
	}
	if sel.Warning != "" {
		b.WriteString(stErr.Render("⚠ "+sel.Warning) + "\n")
	}
	b.WriteString("\n")
	for i, ag := range m.targets {
		st := sel.States[ag.ID]
		var line string
		switch {
		case st.On && st.Managed:
			line = stOn.Render("●") + fmt.Sprintf(" %s — ativa (gerenciada)", ag.Name)
		case st.On && st.Local:
			line = stLocal.Render("▪") + fmt.Sprintf(" %s — local em %s", ag.Name, st.Via)
		case st.On:
			line = stShared.Render("◆") + fmt.Sprintf(" %s — via %s", ag.Name, st.Via)
		default:
			line = stOff.Render("○") + fmt.Sprintf(" %s — inativa", ag.Name)
		}
		b.WriteString(fmt.Sprintf("%d %s\n", i+1, line))
	}
	for _, ag := range m.agents {
		if ag.Installed && !ag.SupportsSkills() {
			b.WriteString(stOff.Render("— " + ag.Name + ": sem skills locais\n"))
		}
	}
	return b.String()
}

func (m Skills) toastLine() string {
	if m.toast == "" {
		return ""
	}
	if m.toastErr {
		return stErr.Render(m.toast)
	}
	return stOn.Render(m.toast)
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
