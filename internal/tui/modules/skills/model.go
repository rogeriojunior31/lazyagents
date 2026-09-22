package skills

import (
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/skill"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
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
	skModeRegistry    // input do termo de busca no registry
	skModeRegistryPick
)

// Skills é a aba principal: matriz skill × agente com toggle por symlink.
type Skills struct {
	svc     *skill.Service
	agents  []agent.Agent // todos os detectados
	targets []agent.Agent // instalados com dir de skills (colunas da matriz)
	skills  []skill.Skill

	list       list.Model
	input      textinput.Model
	picker     picker
	confirm    components.Confirm
	vp         viewport.Model
	detailVP   viewport.Model // conteúdo rolável do painel de detalhe
	paneFocus  kit.PaneID     // painel com foco: lista (padrão) ou detalhe
	detailName string         // skill mostrada no detalhe, p/ resetar o scroll ao trocar
	docName    string
	docSource  string // raw markdown, re-rendered after resize/theme changes
	docPath    string // pasta da skill aberta no modo leitura

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
	regPicker      registryPickerState
	toast          string
	toastErr       bool
	toastSeq       int           // guarda o toast atual contra timers de expiração antigos
	spin           spinner.Model // animação de operações de rede
	inFlight       bool          // operação de rede em curso
	width, height  int
}

// skToastExpire pede para limpar o toast se ele ainda for o de número seq.
type skToastExpire struct{ seq int }

func expireToastCmd(seq int) tea.Cmd {
	return tea.Tick(kit.ToastTTL, func(time.Time) tea.Msg { return skToastExpire{seq} })
}

func NewSkills(svc *skill.Service) Skills {
	l := list.New(nil, kit.PlainDelegate{}, 0, 0)
	kit.StyleList(&l)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)  // "N items" fica no título do Panel
	l.SetShowPagination(false) // sem dots crus
	l.DisableQuitKeybindings()
	in := components.NewInput()
	in.Placeholder = "URL do GitHub, usuario/repo, pasta ou arquivo .zip"
	in.CharLimit = 1024
	in.SetWidth(60)
	sp := spinner.New(spinner.WithSpinner(spinner.MiniDot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(theme.Primary)))
	return Skills{svc: svc, list: l, input: in, vp: viewport.New(), detailVP: viewport.New(), spin: sp}
}

// beginSpin liga o spinner com um rótulo de progresso e devolve o tick inicial.
// Se já há uma operação em voo, só troca o rótulo (evita loops de tick duplicados).
func (m *Skills) beginSpin(label string) tea.Cmd {
	m.toast, m.toastErr = label, false
	if m.inFlight {
		return nil
	}
	m.inFlight = true
	return m.spin.Tick
}

func (m Skills) Init() tea.Cmd { return nil }

func (m Skills) Capturing() bool {
	return m.mode != skModeList || m.list.SettingFilter()
}

// Update embrulha update() para agendar o auto-dismiss do toast: quando
// o toast muda para um novo texto, incrementa o seq e agenda a expiração.
func (m Skills) step(msg tea.Msg) (Skills, tea.Cmd) {
	prev := m.toast
	var cmd tea.Cmd
	m, cmd = m.update(msg)
	if m.toast != "" && m.toast != prev {
		m.toastSeq++
		cmd = tea.Batch(cmd, expireToastCmd(m.toastSeq))
	}
	return m, cmd
}

func (m Skills) update(msg tea.Msg) (Skills, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, nil

	case skToastExpire:
		if msg.seq == m.toastSeq && !m.inFlight {
			m.toast = ""
		}
		return m, nil

	case spinner.TickMsg:
		if !m.inFlight {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case events.AgentsDetected:
		m.agents = msg.Agents
		m.targets = nil
		for _, ag := range msg.Agents {
			if ag.Installed && ag.SupportsSkills() {
				m.targets = append(m.targets, ag)
			}
		}
		return m, m.scanCmd()

	case events.SkillsScanned:
		if msg.Err != nil {
			m.setToast(msg.Err.Error(), true)
			return m, nil
		}
		m.skills = msg.Skills
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
		m.inFlight = false
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
		m.inFlight = false
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

	case adoptAllDoneMsg:
		m.inFlight = false
		m.mode = skModeList
		if len(msg.errs) > 0 {
			parts := make([]string, len(msg.errs))
			for i, e := range msg.errs {
				parts[i] = e.Error()
			}
			m.setToast(fmt.Sprintf(
				"%d adotada(s), %d falha(s): %s",
				len(msg.adopted), len(msg.errs), strings.Join(parts, "; "),
			), true)
		} else if len(msg.adopted) == 0 {
			m.setToast("nenhuma skill local para adotar", false)
		} else {
			m.setToast(fmt.Sprintf("%d skill(s) adotada(s): %s", len(msg.adopted), strings.Join(msg.adopted, ", ")), false)
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
		m.inFlight = false
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
			m.mode = skModeList
			return m, nil
		}
		if len(msg.found) == 1 && len(msg.origin.Notes) == 0 {
			// repo com 1 skill: instala direto, sem picker (só origens com
			// múltiplas skills ou com avisos de marketplace pedem escolha antes).
			m.mode = skModeList
			svc, origin, cleanup, found := m.svc, msg.origin, msg.cleanup, msg.found
			spin := m.beginSpin("instalando " + found[0].Name + "…")
			return m, tea.Batch(spin, func() tea.Msg {
				names, err := svc.Install(found, origin)
				if cleanup != "" {
					os.RemoveAll(cleanup)
				}
				return installDoneMsg{names: names, err: err}
			})
		}
		m.picker = newPicker(msg.found, msg.origin, msg.cleanup)
		m.mode = skModePick
		return m, nil

	case installDoneMsg:
		m.inFlight = false
		m.mode = skModeList
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
		} else if len(msg.names) > 0 {
			m.setToast("instaladas: "+strings.Join(msg.names, ", "), false)
		} else {
			m.setToast("nada instalado", true)
		}
		return m, m.scanCmd()

	case registrySearchMsg:
		m.inFlight = false
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
			m.mode = skModeList
			return m, nil
		}
		m.regPicker = newRegistryPicker(msg.results)
		m.mode = skModeRegistryPick
		return m, nil

	case docMsg:
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
			return m, nil
		}
		stayPut := m.mode == skModeDoc && m.docName == msg.name // recarga pós-edição
		m.docName, m.docPath = msg.name, msg.path
		m.docSource = msg.content
		m.vp.SetContent(kit.RenderMarkdown(msg.content, m.width-2))
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
		m.inFlight = false
		if msg.err != nil {
			m.setToast(msg.err.Error(), true)
		} else {
			m.setToast(msg.name+" atualizada (backup em "+core.Tilde(m.svc.Paths().BackupsDir(), m.svc.Paths().Home)+")", false)
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
		m.pdiff = profileDiff{name: msg.name, changes: msg.changes}
		var lines []string
		lines = append(lines, fmt.Sprintf("Aplicar perfil %q?", msg.name))
		if len(msg.changes) == 0 {
			lines = append(lines, kit.StHint.Render("(sem mudanças — já está no estado do perfil)"))
		}
		for _, c := range msg.changes {
			if len(c.Add) > 0 {
				lines = append(lines, kit.StOn.Render("+ "+c.Skill+": ")+m.agentLabels(c.Add))
			}
			if len(c.Remove) > 0 {
				lines = append(lines, kit.StErr.Render("− "+c.Skill+": ")+m.agentLabels(c.Remove))
			}
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
			if m.paneFocus == kit.PaneDetail { // roda rola o detalhe focado
				var cmd tea.Cmd
				m.detailVP, cmd = m.detailVP.Update(msg)
				return m, cmd
			}
			if msg.Button == tea.MouseWheelUp {
				m.list.CursorUp()
			} else if msg.Button == tea.MouseWheelDown {
				m.list.CursorDown()
			}
			m.refreshDetail()
		case skModePick:
			p := &m.picker
			if msg.Button == tea.MouseWheelUp && p.cursor > 0 {
				p.cursor--
			} else if msg.Button == tea.MouseWheelDown && p.cursor < len(p.items)-1 {
				p.cursor++
			}
		case skModeRegistryPick:
			p := &m.regPicker
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
		case m.mode == skModeInstall, m.mode == skModeNew, m.mode == skModeProfileName, m.mode == skModeRegistry:
			m.input, cmd = m.input.Update(msg) // textinput trata paste nativamente
		case m.mode == skModeList && m.list.SettingFilter():
			m.list, cmd = kit.FeedTextToList(m.list, msg.Content)
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
		case skModeRegistry:
			return m.updateRegistrySearch(msg)
		case skModeRegistryPick:
			return m.updateRegistryPick(msg)
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

func (m Skills) listWidth() int {
	if m.width < 76 {
		return m.width
	}
	return m.width * 2 / 5
}

// bodyHeight é a altura disponível para o corpo (descontados hints + toast).
func (m Skills) bodyHeight() int {
	h := m.height - 2
	if h < 3 {
		h = 3
	}
	return h
}

func (m *Skills) layout() {
	if m.width == 0 {
		return
	}
	bodyH := m.bodyHeight()
	// A lista vive dentro de um Panel: dimensiona pelo conteúdo útil dele.
	lp := components.Panel{Width: m.listWidth(), Height: bodyH}
	m.list.SetSize(lp.ContentWidth(), lp.ContentHeight())
	m.vp.SetWidth(m.width)
	m.vp.SetHeight(bodyH)
	if m.mode == skModeDoc {
		m.vp.SetContent(kit.RenderMarkdown(m.docSource, m.width-2))
	}
	m.refreshDetail()
}

func (m *Skills) setToast(s string, isErr bool) {
	m.toast, m.toastErr = s, isErr
}

// ClearToast some com o toast (usado ao trocar de aba).
func (m *Skills) ClearToast() { m.toast = "" }

// Count é o total de skills na biblioteca (para o contador do header/aba).
func (m Skills) Count() int { return len(m.skills) }

func (m *Skills) ID() string { return "skills" }

func (m *Skills) Title() string { return "Skills" }

// Update aplica a mensagem e guarda o novo estado (semântica de ponteiro do
// module.Module). events.Reload equivale à tecla r.
func (m *Skills) Update(msg tea.Msg) tea.Cmd {
	if _, ok := msg.(events.Reload); ok {
		msg = tea.KeyPressMsg{Code: 'r', Text: "r"}
	}
	nm, cmd := m.step(msg)
	*m = nm
	return cmd
}
