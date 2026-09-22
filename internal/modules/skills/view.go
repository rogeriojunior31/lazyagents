package skills

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
)

func (m Tab) View() string {
	switch m.mode {
	case skModeInstall:
		return m.inputModal("Instalar skill",
			"Origem (GitHub, pasta local ou .zip):",
			components.Keycap("enter")+kit.StHint.Render(" procura skills  ")+components.Keycap("esc")+kit.StHint.Render(" cancela"))
	case skModeNew:
		return m.inputModal("Nova skill",
			"Nome (vira a pasta em "+core.Tilde(m.svc.Paths().LibraryDir(), m.svc.Paths().Home)+"):",
			components.Keycap("enter")+kit.StHint.Render(" cria e abre o editor  ")+components.Keycap("esc")+kit.StHint.Render(" cancela"))
	case skModePick:
		return m.picker.view(m.width, m.height-2)
	case skModeRegistry:
		return m.inputModal("Buscar no GitHub",
			"Termo de busca (repositórios com SKILL.md):",
			components.Keycap("enter")+kit.StHint.Render(" busca  ")+components.Keycap("esc")+kit.StHint.Render(" cancela"))
	case skModeRegistryPick:
		return m.regPicker.view(m.width, m.height-2)
	case skModeConfirm:
		return m.confirm.View()
	case skModeDoc:
		head := kit.StTitle.Render(m.docName) + kit.StHint.Render("  SKILL.md · e edita · esc volta · ↑↓/roda do mouse rola")
		return lipgloss.JoinVertical(lipgloss.Left, head, m.vp.View())
	case skModeBackup:
		return m.backupPicker.view(m.width)
	case skModeProfiles:
		return m.profilesView()
	case skModeProfileName:
		return m.inputModal("Salvar perfil",
			"Nome do perfil:",
			components.Keycap("enter")+kit.StHint.Render(" salva  ")+components.Keycap("esc")+kit.StHint.Render(" cancela"))
	}

	listW := m.listWidth()
	detailW, bodyH := m.detailDims()
	title := fmt.Sprintf("BIBLIOTECA   %d", len(m.skills))
	if n := len(m.localNames()); n > 0 {
		title += fmt.Sprintf(" · %d local(is)", n)
	}
	listPanel := components.Panel{
		Title:   title,
		Focused: m.paneFocus == kit.PaneList,
		Width:   listW,
		Height:  bodyH,
	}.Render(kit.ListView(m.list, "Biblioteca vazia."))
	body := lipgloss.JoinHorizontal(lipgloss.Top, listPanel, "  ", m.detailView(detailW, bodyH))
	if m.width < 76 {
		body = listPanel
		if m.paneFocus == kit.PaneDetail {
			body = m.detailView(detailW, bodyH)
		}
	}
	hints := kit.Hints(m.width, [2]string{"enter", "ler"}, [2]string{"/", "filtrar"},
		[2]string{"?", "atalhos"}, [2]string{"i", "instalar"}, [2]string{"1-9", "alternar"},
		[2]string{"e", "editar"}, [2]string{"u", "atualizar"}, [2]string{"p", "perfis"})
	if m.width < 76 {
		hints = kit.Hints(m.width, [2]string{"←/→", "lista / detalhe"}, [2]string{"?", "atalhos"}, [2]string{"/", "filtrar"})
	}
	return lipgloss.JoinVertical(lipgloss.Left, body, hints, m.toastLine())
}

// detailDims devolve largura/altura do painel de detalhe (alinhado à lista).
func (m Tab) detailDims() (int, int) {
	if m.width < 76 {
		return m.width, m.bodyHeight()
	}
	w := m.width - m.listWidth() - 2 // "  " de gap entre os painéis
	if w < 24 {
		w = 24
	}
	return w, m.bodyHeight()
}

// refreshDetail recomputa o conteúdo do painel de detalhe no viewport, mantendo
// o scroll (só volta ao topo quando a skill selecionada muda).
func (m *Tab) refreshDetail() {
	w, h := m.detailDims()
	p := components.Panel{Width: w, Height: h}
	m.detailVP.SetWidth(p.ContentWidth())
	m.detailVP.SetHeight(p.ContentHeight())
	name := ""
	if sel, ok := m.selected(); ok {
		name = sel.Name
	}
	if name != m.detailName {
		m.detailVP.GotoTop()
		m.detailName = name
	}
	m.detailVP.SetContent(m.detailContent(p.ContentWidth()))
}

// detailView emoldura o viewport do detalhe; a borda acesa segue o foco.
func (m Tab) detailView(w, h int) string {
	return components.Panel{
		Title:   "SOBRE A SKILL",
		Focused: m.paneFocus == kit.PaneDetail,
		Width:   w,
		Height:  h,
	}.Render(m.detailVP.View())
}

// detailContent monta o texto do card da skill selecionada, quebrado em inner
// colunas (descrição completa — o viewport rola quando não couber).
func (m Tab) detailContent(inner int) string {
	sel, ok := m.selected()
	if !ok {
		// Linha a linha: um Render multilinha alinharia "Pressione " à largura
		// da linha mais longa e abriria um buraco antes do keycap.
		return lipgloss.NewStyle().Width(inner).Render(lipgloss.JoinVertical(lipgloss.Left,
			kit.StText.Render("Nenhuma skill por aqui."),
			"",
			kit.StHint.Render("Pressione ")+components.Keycap("i")+
				kit.StHint.Render(" para instalar do GitHub, de uma pasta ou de um zip.")))
	}
	home := m.svc.Paths().Home
	nameW := 0
	for _, ag := range m.targets {
		nameW = max(nameW, len(ag.Name))
	}

	var b strings.Builder
	b.WriteString(kit.StTitle.Render(sel.Name) + "\n")
	if sel.Description != "" {
		b.WriteString(kit.StText.Render(sel.Description) + "\n")
	}
	b.WriteString("\n" + kit.StHint.Render("ORIGEM") + "\n")
	if sel.InLibrary {
		b.WriteString(kit.CardLabel.Render("biblioteca  ") + kit.CardValue.Render(core.Tilde(sel.Path, home)) + "\n")
		if o := sel.Origin; o != nil {
			src := o.Source
			if o.Type != "git" {
				src = core.Tilde(src, home)
			}
			line := kit.CardLabel.Render("origem      ") + kit.CardValue.Render(o.Type+" "+src)
			if !o.InstalledAt.IsZero() {
				line += kit.CardLabel.Render("  (" + o.InstalledAt.Format("02/01/2006") + ")")
			}
			b.WriteString(line + "\n")
		}
	} else {
		b.WriteString(kit.StLocal.Render("▪ fora da biblioteca — ") + components.Keycap("o") + kit.StLocal.Render(" adota") + "\n")
	}
	if sel.Warning != "" {
		b.WriteString(kit.StErr.Render("⚠ "+sel.Warning) + "\n")
	}
	for _, is := range m.selectedIssues() {
		b.WriteString(kit.StWarn.Render(fmt.Sprintf("! %s: %s", is.Field, is.Msg)) + "\n")
	}
	b.WriteString("\n")
	b.WriteString(kit.StHint.Render("DISPONÍVEL NOS AGENTES") + "\n\n")
	for i, ag := range m.targets {
		st := sel.States[ag.ID]
		name := fmt.Sprintf("%-*s", nameW, ag.Name)
		var mark, status string
		switch {
		case st.On && st.Managed:
			mark, status = kit.StOn.Render("●"), kit.StOn.Render("ativa")
		case st.On && st.Local:
			mark, status = kit.StLocal.Render("▪"), kit.StLocal.Render("local · "+core.Tilde(st.Via, home))
		case st.On:
			mark, status = kit.StShared.Render("◆"), kit.StShared.Render("via "+core.Tilde(st.Via, home))
		default:
			mark, status = kit.StOff.Render("○"), kit.StOff.Render("inativa")
		}
		b.WriteString(fmt.Sprintf("%s %s %s  %s\n",
			components.Keycap(fmt.Sprintf("%d", i+1)), mark, kit.CardValue.Render(name), status))
	}
	return lipgloss.NewStyle().Width(inner).Render(strings.TrimRight(b.String(), "\n"))
}

// inputModal emoldura um prompt de texto (install/nova/perfil) num Panel, com
// o toast abaixo. Largura limitada para não virar uma faixa vazia.
func (m Tab) inputModal(title, prompt, hint string) string {
	w := m.width
	if w > 72 {
		w = 72
	}
	content := lipgloss.JoinVertical(lipgloss.Left, prompt, "", m.input.View(), "", hint)
	panel := components.Panel{Title: title, Focused: true, Width: w}.Render(content)
	return lipgloss.JoinVertical(lipgloss.Left, panel, "", m.toastLine())
}

func (m Tab) toastLine() string {
	if m.toast == "" {
		return ""
	}
	if m.inFlight {
		return m.spin.View() + " " + kit.StHint.Render(m.toast)
	}
	return components.Toast(m.toast, m.toastErr)
}

// agentLabels traduz IDs de agente para nomes amigáveis, juntando com vírgula.
func (m Tab) agentLabels(ids []string) string {
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		label := id
		for _, ag := range m.agents {
			if ag.ID == id {
				label = ag.Name
				break
			}
		}
		names = append(names, label)
	}
	return strings.Join(names, ", ")
}
