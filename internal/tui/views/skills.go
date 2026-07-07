package views

import (
	"fmt"
	"os"
	"os/exec"
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
	"lazyskills/internal/tui/theme"
)

type skMode int

const (
	skModeList skMode = iota
	skModeInstall
	skModePick
	skModeConfirm
	skModeDoc
	skModeNew         // input do nome de uma skill nova
	skModeProfiles    // lista de perfis
	skModeProfileName // input do nome para salvar perfil
	skModeBackup      // lista de backups de uma skill
)

type confirmKind int

const (
	confirmKindRemove confirmKind = iota
	confirmKindUpdate
	confirmKindApplyProfile
	confirmKindUpdateAll
	confirmKindRestore
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
	docPath string // pasta da skill aberta no modo leitura

	mode           skMode
	ckind          confirmKind
	pendingRemove  skill.Skill
	pendingUpdate  skill.Skill
	profileNames   []string
	profileCursor  int
	pendingProfile string
	pdiff          profileDiff
	updateChecks   []skill.UpdateCheck
	updateStatus   map[string]skill.UpdateStatus
	backupPicker   backupPickerState
	pendingRestore skill.Backup
	toast          string
	toastErr       bool
	width, height  int
}

type profileDiff struct {
	name      string
	toEnable  []string
	toDisable []string
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
	origin  skill.Origin
	cleanup string
	err     error
}

type installDoneMsg struct {
	names []string
	err   error
}

type docMsg struct {
	name    string
	path    string // pasta da skill
	content string
	err     error
}

type editDoneMsg struct {
	name string
	err  error
}

type createdMsg struct {
	name string
	path string
	err  error
}

type updateDoneMsg struct {
	name string
	err  error
}

type profilesLoadMsg struct {
	names []string
	err   error
}

type profileDiffMsg struct {
	name      string
	toEnable  []string
	toDisable []string
	err       error
}

type profileSaveMsg struct {
	name string
	err  error
}

type listBackupsMsg struct {
	backups  []skill.Backup
	skillDir string
	err      error
}

type restoreBackupMsg struct {
	skillDir string
	err      error
}

type checkUpdatesMsg struct {
	checks []skill.UpdateCheck
	err    error
}

type updateAllMsg struct {
	updated int
	skipped []string
	errs    []error
}

type profileApplyDoneMsg struct {
	name string
	err  error
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
	l := list.New(nil, plainDelegate{}, 0, 0)
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
		return m, m.rebuildListItems()

	case listBackupsMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
			return m, nil
		}
		if len(msg.backups) == 0 {
			m.setToast(fmt.Sprintf("nenhum backup encontrado para %q", msg.skillDir), false)
			return m, nil
		}
		m.backupPicker = newBackupPicker(msg.backups, msg.skillDir)
		m.mode = skModeBackup
		return m, nil

	case restoreBackupMsg:
		m.mode = skModeList
		if msg.err != nil {
			m.setToast(fmt.Sprintf("erro ao restaurar %s: %s", msg.skillDir, msg.err), true)
		} else {
			m.setToast(fmt.Sprintf("%s restaurada com sucesso", msg.skillDir), false)
		}
		return m, m.scanCmd()

	case checkUpdatesMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
			m.mode = skModeList
			return m, nil
		}
		if len(msg.checks) == 0 {
			m.setToast("nenhuma skill git para verificar", false)
			return m, nil
		}
		m.updateChecks = msg.checks
		m.updateStatus = make(map[string]skill.UpdateStatus, len(msg.checks))
		var available, localEdited int
		for _, ch := range msg.checks {
			m.updateStatus[ch.Skill.Dir] = ch.Status
			switch ch.Status {
			case skill.UpdateStatusAvailable:
				available++
			case skill.UpdateStatusLocallyEdited:
				localEdited++
			}
		}
		if available == 0 {
			count := len(msg.checks)
			m.setToast(fmt.Sprintf("%d skill(s) em dia", count), false)
			m.updateChecks = nil
			return m, m.rebuildListItems()
		}
		cmsg := fmt.Sprintf("Atualizar %d skill(s) do GitHub?", available)
		if localEdited > 0 {
			cmsg += fmt.Sprintf(" (%d editada(s) localmente serão puladas)", localEdited)
		}
		m.ckind = confirmKindUpdateAll
		m.confirm = components.NewConfirm(cmsg)
		m.mode = skModeConfirm
		return m, m.rebuildListItems()

	case updateAllMsg:
		m.mode = skModeList
		m.updateChecks = nil
		if len(msg.errs) > 0 {
			parts := make([]string, len(msg.errs))
			for i, e := range msg.errs {
				parts[i] = e.Error()
			}
			m.setToast(fmt.Sprintf(
				"%d atualizada(s), %d falha(s): %s",
				msg.updated, len(msg.errs), strings.Join(parts, "; "),
			), true)
		} else if len(msg.skipped) > 0 {
			m.setToast(fmt.Sprintf(
				"%d atualizada(s), puladas (editadas localmente): %s",
				msg.updated, strings.Join(msg.skipped, ", "),
			), false)
		} else {
			m.setToast(fmt.Sprintf("%d skill(s) atualizadas", msg.updated), false)
		}
		return m, m.scanCmd()

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
		m.picker = newPicker(msg.found, msg.origin, msg.cleanup)
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
		stayPut := m.mode == skModeDoc && m.docName == msg.name // recarga pós-edição
		m.docName, m.docPath = msg.name, msg.path
		m.vp.SetContent(renderMarkdown(msg.content, m.width-2))
		if !stayPut {
			m.vp.GotoTop()
		}
		m.mode = skModeDoc
		return m, nil

	case editDoneMsg:
		if msg.err != nil {
			m.setToast("editor terminou com erro: "+msg.err.Error(), true)
		} else {
			m.setToast("SKILL.md de "+msg.name+" salvo", false)
		}
		cmds := []tea.Cmd{m.scanCmd()}
		if m.mode == skModeDoc {
			cmds = append(cmds, loadDocCmd(m.docName, m.docPath))
		}
		return m, tea.Batch(cmds...)

	case createdMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
			return m, nil // continua no input para corrigir o nome
		}
		m.mode = skModeList
		m.setToast("skill "+msg.name+" criada — abrindo editor", false)
		return m, editCmd(msg.name, msg.path)

	case updateDoneMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
		} else {
			m.setToast(msg.name+" atualizada (backup em ~/.lazyskills/backups)", false)
		}
		return m, m.scanCmd()

	case profilesLoadMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
			return m, nil
		}
		m.profileNames = msg.names
		m.profileCursor = 0
		m.mode = skModeProfiles
		return m, nil

	case profileDiffMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
			return m, nil
		}
		m.pendingProfile = msg.name
		m.pdiff = profileDiff{name: msg.name, toEnable: msg.toEnable, toDisable: msg.toDisable}
		var lines []string
		lines = append(lines, fmt.Sprintf("Aplicar perfil %q?", msg.name))
		if len(msg.toEnable) > 0 {
			lines = append(lines, stOn.Render("+ ativar: ")+strings.Join(msg.toEnable, ", "))
		}
		if len(msg.toDisable) > 0 {
			lines = append(lines, stErr.Render("− desativar: ")+strings.Join(msg.toDisable, ", "))
		}
		if len(msg.toEnable) == 0 && len(msg.toDisable) == 0 {
			lines = append(lines, stHint.Render("(sem mudanças — já está no estado do perfil)"))
		}
		m.confirm = components.NewConfirm(strings.Join(lines, "\n"))
		m.ckind = confirmKindApplyProfile
		m.mode = skModeConfirm
		return m, nil

	case profileSaveMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
			m.mode = skModeProfileName
			return m, m.input.Focus()
		}
		m.setToast("perfil \""+msg.name+"\" salvo", false)
		return m, m.loadProfilesCmd()

	case profileApplyDoneMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
		} else {
			m.setToast("perfil \""+msg.name+"\" aplicado", false)
		}
		m.mode = skModeList
		return m, m.scanCmd()

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
		case skModeProfiles:
			if msg.Button == tea.MouseWheelUp && m.profileCursor > 0 {
				m.profileCursor--
			} else if msg.Button == tea.MouseWheelDown && m.profileCursor < len(m.profileNames)-1 {
				m.profileCursor++
			}
		}
		return m, nil

	case tea.MouseClickMsg:
		return m.click(msg)

	case tea.PasteMsg:
		var cmd tea.Cmd
		switch {
		case m.mode == skModeInstall, m.mode == skModeNew, m.mode == skModeProfileName:
			m.input, cmd = m.input.Update(msg) // textinput trata paste nativamente
		case m.mode == skModeList && m.list.SettingFilter():
			m.list, cmd = feedTextToList(m.list, msg.Content)
		}
		return m, cmd

	case tea.KeyPressMsg:
		switch m.mode {
		case skModeInstall:
			return m.updateInstall(msg)
		case skModeNew:
			return m.updateNew(msg)
		case skModePick:
			return m.updatePick(msg)
		case skModeConfirm:
			return m.updateConfirm(msg)
		case skModeDoc:
			return m.updateDoc(msg)
		case skModeProfiles:
			return m.updateProfiles(msg)
		case skModeProfileName:
			return m.updateProfileName(msg)
		case skModeBackup:
			return m.updateBackupPicker(msg)
		default:
			return m.updateList(msg)
		}
	}
	// mensagens internas dos bubbles (ex.: list.FilterMatchesMsg, que entrega
	// o resultado assíncrono do filtro) precisam chegar à lista
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
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
	case skModeProfiles:
		// título ocupa linhas 0-1; itens começam na linha 2, um por linha
		row := msg.Y - 2
		if row >= 0 && row < len(m.profileNames) {
			m.profileCursor = row
		}
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
	return loadDocCmd(sel.Name, sel.Path)
}

func loadDocCmd(name, path string) tea.Cmd {
	return func() tea.Msg {
		data, err := os.ReadFile(filepath.Join(path, "SKILL.md"))
		if err != nil {
			return docMsg{err: fmt.Errorf("lendo SKILL.md de %s: %w", name, err)}
		}
		return docMsg{name: name, path: path, content: string(data)}
	}
}

// editCmd suspende a TUI e abre o SKILL.md no $EDITOR (fallback vi).
func editCmd(name, path string) tea.Cmd {
	fields := strings.Fields(os.Getenv("EDITOR"))
	if len(fields) == 0 {
		fields = []string{"vi"}
	}
	args := append(fields[1:], filepath.Join(path, "SKILL.md"))
	c := exec.Command(fields[0], args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editDoneMsg{name: name, err: err}
	})
}

func (m Skills) updateDoc(msg tea.KeyPressMsg) (Skills, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "enter":
		m.mode = skModeList
		return m, nil
	case "e":
		return m, editCmd(m.docName, m.docPath)
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
		m.setToast("verificando updates…", false)
		return m, m.checkUpdatesCmd()
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
	case key == "p":
		return m, m.loadProfilesCmd()
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
			found, origin, cleanup, err := svc.Discover(src)
			return discoverMsg{found: found, origin: origin, cleanup: cleanup, err: err}
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Skills) updateNew(msg tea.KeyPressMsg) (Skills, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = skModeList
		m.input.Blur()
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.input.Value())
		if name == "" {
			return m, nil
		}
		m.input.Blur()
		svc := m.svc
		return m, func() tea.Msg {
			path, err := svc.Create(name)
			return createdMsg{name: name, path: path, err: err}
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
		svc, origin, cleanup := m.svc, m.picker.origin, m.picker.cleanup
		return m, func() tea.Msg {
			names, err := svc.Install(chosen, origin)
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
		switch m.ckind {
		case confirmKindUpdate:
			sel := m.pendingUpdate
			svc := m.svc
			return m, func() tea.Msg {
				return updateDoneMsg{name: sel.Name, err: svc.Update(sel)}
			}
		case confirmKindApplyProfile:
			name := m.pendingProfile
			svc, agents := m.svc, m.agents
			return m, func() tea.Msg {
				return profileApplyDoneMsg{name: name, err: svc.ApplyProfile(name, agents)}
			}
		case confirmKindUpdateAll:
			return m, m.updateAllCmd()
		case confirmKindRestore:
			return m, m.restoreBackupCmd()
		default:
			sel := m.pendingRemove
			return m, m.opCmd("removida (backup em ~/.lazyskills/backups)", func() error {
				return m.svc.Remove(sel, m.agents)
			})
		}
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
// Quando há resultado de CheckUpdates, acrescenta ↑ (disponível) ou ~ (editada).
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
	switch m.updateStatus[s.Dir] {
	case skill.UpdateStatusAvailable:
		b.WriteString(stLocal.Render("↑"))
	case skill.UpdateStatusLocallyEdited:
		b.WriteString(stHint.Render("~"))
	}
	return b.String()
}

func (m *Skills) rebuildListItems() tea.Cmd {
	items := make([]list.Item, 0, len(m.skills))
	for _, s := range m.skills {
		items = append(items, skillItem{s: s, badge: m.badge(s)})
	}
	return m.list.SetItems(items)
}

func (m Skills) checkUpdatesCmd() tea.Cmd {
	svc, skills := m.svc, m.skills
	return func() tea.Msg {
		checks, err := svc.CheckUpdates(skills)
		return checkUpdatesMsg{checks: checks, err: err}
	}
}

func (m Skills) updateAllCmd() tea.Cmd {
	svc, checks := m.svc, m.updateChecks
	return func() tea.Msg {
		updated, skipped, errs := svc.UpdateAll(checks)
		return updateAllMsg{updated: updated, skipped: skipped, errs: errs}
	}
}

// backupPickerState gerencia a lista de backups disponíveis para uma skill.
type backupPickerState struct {
	backups  []skill.Backup
	skillDir string
	cursor   int
}

func newBackupPicker(backups []skill.Backup, skillDir string) backupPickerState {
	return backupPickerState{backups: backups, skillDir: skillDir}
}

func (p *backupPickerState) update(msg tea.KeyPressMsg) {
	switch msg.String() {
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
	case "down", "j":
		if p.cursor < len(p.backups)-1 {
			p.cursor++
		}
	}
}

func (p backupPickerState) selected() skill.Backup {
	if p.cursor < len(p.backups) {
		return p.backups[p.cursor]
	}
	return skill.Backup{}
}

func (p backupPickerState) view(maxW int) string {
	var b strings.Builder
	b.WriteString(stTitle.Render(fmt.Sprintf("Backups de %q (%d)", p.skillDir, len(p.backups))) + "\n\n")
	start, end := window(p.cursor, len(p.backups), 16)
	for i := start; i < end; i++ {
		bk := p.backups[i]
		ts := bk.Time.Format("02/01/2006 15:04:05")
		line := fmt.Sprintf("%s  %s", ts, stHint.Render(truncate(bk.Path, maxW-30)))
		if i == p.cursor {
			line = stOn.Render("> ") + line
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + stHint.Render("enter restaura · esc volta"))
	return b.String()
}

func (m *Skills) updateBackupPicker(msg tea.KeyPressMsg) (Skills, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.mode = skModeList
	case "enter":
		sel := m.backupPicker.selected()
		if sel.Path == "" {
			return *m, nil
		}
		m.pendingRestore = sel
		m.ckind = confirmKindRestore
		m.confirm = components.NewConfirm(fmt.Sprintf("Restaurar backup de %q (%s)?", sel.SkillDir, sel.Time.Format("02/01/2006 15:04")))
		m.mode = skModeConfirm
	default:
		m.backupPicker.update(msg)
	}
	return *m, nil
}

func (m Skills) listBackupsCmd(skillDir string) tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		all, err := svc.ListBackups()
		if err != nil {
			return listBackupsMsg{skillDir: skillDir, err: err}
		}
		var filtered []skill.Backup
		for _, b := range all {
			if b.SkillDir == skillDir {
				filtered = append(filtered, b)
			}
		}
		return listBackupsMsg{backups: filtered, skillDir: skillDir}
	}
}

func (m Skills) restoreBackupCmd() tea.Cmd {
	svc, b := m.svc, m.pendingRestore
	return func() tea.Msg {
		err := svc.Restore(b)
		return restoreBackupMsg{skillDir: b.SkillDir, err: err}
	}
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
	case skModeNew:
		return lipgloss.JoinVertical(lipgloss.Left,
			stTitle.Render("Nova skill"),
			"",
			"Nome (vira a pasta em ~/.lazyskills/skills):",
			m.input.View(),
			"",
			stHint.Render("enter cria e abre o editor · esc cancela"),
			"",
			m.toastLine(),
		)
	case skModePick:
		return m.picker.view(m.height - 2)
	case skModeConfirm:
		return m.confirm.View()
	case skModeDoc:
		head := stTitle.Render(m.docName) + stHint.Render("  SKILL.md · e edita · esc volta · ↑↓/roda do mouse rola")
		return lipgloss.JoinVertical(lipgloss.Left, head, m.vp.View())
	case skModeBackup:
		return m.backupPicker.view(m.width)
	case skModeProfiles:
		return m.profilesView()
	case skModeProfileName:
		return lipgloss.JoinVertical(lipgloss.Left,
			stTitle.Render("Salvar perfil"),
			"",
			"Nome do perfil:",
			m.input.View(),
			"",
			stHint.Render("enter salva · esc cancela"),
			"",
			m.toastLine(),
		)
	}

	listW := m.listWidth()
	detailW := m.width - listW - 3
	if detailW < 24 {
		detailW = 24
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, m.list.View(), "  ", m.detailView(detailW))
	hints := stHint.Render("enter lê · e edita · u atualiza · U verifica updates · b backups · 1-9 alterna · space/a/x todos · p perfis · i instala · n nova · o adota · d remove · / filtra · r recarrega")
	return lipgloss.JoinVertical(lipgloss.Left, body, hints, m.toastLine())
}

var keyChip = lipgloss.NewStyle().
	Foreground(theme.Bg).
	Background(theme.Border).
	Padding(0, 1)

func (m Skills) detailView(w int) string {
	sel, ok := m.selected()
	if !ok {
		return cardOff.Width(w).Render(
			stHint.Render("Nenhuma skill por aqui.\n\nPressione ") +
				keyChip.Render("i") +
				stHint.Render(" para instalar do GitHub, de uma pasta ou de um zip."))
	}
	home := m.svc.Paths().Home
	inner := w - 4
	nameW := 0
	for _, ag := range m.targets {
		nameW = max(nameW, len(ag.Name))
	}

	var b strings.Builder
	b.WriteString(stTitle.Render(sel.Name) + "\n")
	if sel.Description != "" {
		b.WriteString(stText.Render(truncate(sel.Description, 280)) + "\n")
	}
	b.WriteString("\n")
	if sel.InLibrary {
		b.WriteString(cardLabel.Render("biblioteca  ") + cardValue.Render(tilde(sel.Path, home)) + "\n")
		if o := sel.Origin; o != nil {
			src := o.Source
			if o.Type != "git" {
				src = tilde(src, home)
			}
			line := cardLabel.Render("origem      ") + cardValue.Render(o.Type+" "+src)
			if !o.InstalledAt.IsZero() {
				line += cardLabel.Render("  (" + o.InstalledAt.Format("02/01/2006") + ")")
			}
			b.WriteString(line + "\n")
		}
	} else {
		b.WriteString(stLocal.Render("▪ fora da biblioteca — ") + keyChip.Render("o") + stLocal.Render(" adota") + "\n")
	}
	if sel.Warning != "" {
		b.WriteString(stErr.Render("⚠ "+sel.Warning) + "\n")
	}
	b.WriteString("\n")
	for i, ag := range m.targets {
		st := sel.States[ag.ID]
		name := fmt.Sprintf("%-*s", nameW, ag.Name)
		var mark, status string
		switch {
		case st.On && st.Managed:
			mark, status = stOn.Render("●"), stOn.Render("ativa")
		case st.On && st.Local:
			mark, status = stLocal.Render("▪"), stLocal.Render("local · "+tilde(st.Via, home))
		case st.On:
			mark, status = stShared.Render("◆"), stShared.Render("via "+tilde(st.Via, home))
		default:
			mark, status = stOff.Render("○"), stOff.Render("inativa")
		}
		b.WriteString(fmt.Sprintf("%s %s %s  %s\n",
			keyChip.Render(fmt.Sprintf("%d", i+1)), mark, cardValue.Render(name), status))
	}
	return cardOn.Width(w).Render(lipgloss.NewStyle().Width(inner).Render(strings.TrimRight(b.String(), "\n")))
}

func (m Skills) toastLine() string {
	if m.toast == "" {
		return ""
	}
	if m.toastErr {
		return stErr.Render("✗ " + m.toast)
	}
	return stOn.Render("✓ " + m.toast)
}

func (m Skills) loadProfilesCmd() tea.Cmd {
	svc := m.svc
	return func() tea.Msg {
		names, err := svc.ListProfiles()
		return profilesLoadMsg{names: names, err: err}
	}
}

func (m Skills) updateProfiles(msg tea.KeyPressMsg) (Skills, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = skModeList
		return m, nil
	case "up", "k":
		if m.profileCursor > 0 {
			m.profileCursor--
		}
		return m, nil
	case "down", "j":
		if m.profileCursor < len(m.profileNames)-1 {
			m.profileCursor++
		}
		return m, nil
	case "enter":
		if len(m.profileNames) == 0 {
			return m, nil
		}
		name := m.profileNames[m.profileCursor]
		svc, skills, agents := m.svc, m.skills, m.agents
		return m, func() tea.Msg {
			wanted, err := svc.GetProfile(name)
			if err != nil {
				return profileDiffMsg{err: err}
			}
			wantedSet := make(map[string]bool, len(wanted))
			for _, w := range wanted {
				wantedSet[w] = true
			}
			var toEnable, toDisable []string
			for _, sk := range skills {
				if !sk.InLibrary {
					continue
				}
				if wantedSet[sk.Dir] {
					anyOff := false
					for _, ag := range agents {
						if ag.Installed && ag.SupportsSkills() && !sk.States[ag.ID].On {
							anyOff = true
							break
						}
					}
					if anyOff {
						toEnable = append(toEnable, sk.Name)
					}
				} else {
					anyManaged := false
					for _, ag := range agents {
						if sk.States[ag.ID].Managed {
							anyManaged = true
							break
						}
					}
					if anyManaged {
						toDisable = append(toDisable, sk.Name)
					}
				}
			}
			return profileDiffMsg{name: name, toEnable: toEnable, toDisable: toDisable}
		}
	case "s":
		m.mode = skModeProfileName
		m.input.Placeholder = "nome do perfil (ex: trabalho)"
		m.input.SetValue("")
		return m, m.input.Focus()
	}
	return m, nil
}

func (m Skills) updateProfileName(msg tea.KeyPressMsg) (Skills, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = skModeProfiles
		m.input.Blur()
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.input.Value())
		if name == "" {
			return m, nil
		}
		m.input.Blur()
		var active []string
		for _, sk := range m.skills {
			if sk.InLibrary && sk.EnabledCount() > 0 {
				active = append(active, sk.Dir)
			}
		}
		svc := m.svc
		return m, func() tea.Msg {
			err := svc.SaveProfile(name, active)
			return profileSaveMsg{name: name, err: err}
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Skills) profilesView() string {
	var b strings.Builder
	b.WriteString(stTitle.Render("Perfis de skills") + "\n\n")
	if len(m.profileNames) == 0 {
		b.WriteString(stHint.Render("Nenhum perfil salvo.") + "\n\n")
	} else {
		start, end := window(m.profileCursor, len(m.profileNames), m.height-8)
		for i := start; i < end; i++ {
			name := m.profileNames[i]
			if i == m.profileCursor {
				b.WriteString(lipgloss.NewStyle().Foreground(theme.Primary).Render("› ") + stText.Render(name) + "\n")
			} else {
				b.WriteString("  " + stHint.Render(name) + "\n")
			}
		}
		if end < len(m.profileNames) {
			b.WriteString(stHint.Render(fmt.Sprintf("… mais %d", len(m.profileNames)-end)) + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString(stHint.Render("enter aplica · s salva estado atual como perfil · esc volta"))
	if m.toast != "" {
		b.WriteString("\n" + m.toastLine())
	}
	return b.String()
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
