package skills

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/list"
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
			kit.Hints(min(m.width, 72)-4, [2]string{"enter", "procura"}, [2]string{"esc", "volta"}))
	case skModeNew:
		return m.inputModal("Nova skill",
			"Nome (vira a pasta em "+core.Tilde(m.svc.Paths().LibraryDir(), m.svc.Paths().Home)+"):",
			kit.Hints(min(m.width, 72)-4, [2]string{"enter", "cria/edita"}, [2]string{"esc", "volta"}))
	case skModePick:
		return m.picker.view(m.width, m.height)
	case skModeRegistry:
		return m.inputModal("Buscar no GitHub",
			"Termo de busca (repositórios com SKILL.md):",
			kit.Hints(min(m.width, 72)-4, [2]string{"enter", "busca"}, [2]string{"esc", "volta"}))
	case skModeRegistryPick:
		return m.regPicker.view(m.width, m.height)
	case skModeConfirm:
		return m.confirm.ViewIn(m.width, m.height)
	case skModeDoc:
		head := kit.StTitle.Render(m.docName) + kit.StHint.Render("  SKILL.md · e edita · esc volta · ↑↓/roda do mouse rola")
		return lipgloss.JoinVertical(lipgloss.Left, head, m.vp.View())
	case skModeBackup:
		return m.backupPicker.view(m.width, m.height)
	case skModeProfiles:
		return m.profilesView()
	case skModeProfileName:
		return m.inputModal("Salvar perfil",
			"Nome do perfil:",
			kit.Hints(min(m.width, 72)-4, [2]string{"enter", "salva"}, [2]string{"esc", "volta"}))
	}

	sp := m.split()
	hints := kit.Hints(m.width, [2]string{"space", "alterna"}, [2]string{"←→", "agente"},
		[2]string{"enter", "ler"}, [2]string{"i", "instalar"}, [2]string{"p", "perfis"},
		[2]string{"/", "filtrar"}, [2]string{"?", "atalhos"})
	body := lipgloss.JoinVertical(lipgloss.Left, m.tableView(sp.ListW, sp.ListH), m.detailView(sp))
	if sp.Side {
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.tableView(sp.ListW, sp.ListH), "  ", m.detailView(sp))
	}
	return lipgloss.JoinVertical(lipgloss.Left, body, hints, m.toastLine())
}

// split reparte o corpo entre a matriz e o detalhe (ao lado ou embaixo).
// Empilhado, a altura que a matriz não usa vai para o detalhe.
func (m Tab) split() kit.Split {
	sp := kit.SplitDetail(m.width, m.bodyHeight())
	if !sp.Side {
		need := len(m.tableHead(sp.ListW)) + max(1, len(m.list.VisibleItems())) + 1
		if spare := sp.ListH - need; spare > 0 {
			sp.ListH -= spare
			sp.DetailH += spare
		}
	}
	return sp
}

// legend explica os marcadores da matriz; some quando não cabe no título.
var legend = kit.StOn.Render("●") + kit.StHint.Render(" ativa  ") +
	kit.StLocal.Render("▪") + kit.StHint.Render(" local  ") +
	kit.StShared.Render("◆") + kit.StHint.Render(" compartilhada  ") +
	kit.StOff.Render("○") + kit.StHint.Render(" inativa")

// tableHead são as linhas acima das skills: título com legenda, filtro (se
// houver) e os nomes das colunas.
func (m Tab) tableHead(w int) []string {
	title := kit.StTitle.Render("BIBLIOTECA") + kit.StHint.Render(fmt.Sprintf("  %d", len(m.skills)))
	if n := len(m.localNames()); n > 0 {
		title += kit.StHint.Render(fmt.Sprintf(" · %d local(is)", n))
	}
	if gap := w - 2 - lipgloss.Width(title) - lipgloss.Width(legend); gap >= 2 {
		title += strings.Repeat(" ", gap) + legend
	}
	lines := []string{"  " + title}
	if m.list.FilterState() != list.Unfiltered {
		m.list.FilterInput.SetWidth(max(1, w-6))
		lines = append(lines, "  "+m.list.FilterInput.View())
	}
	return append(lines, kit.TableHeader(w, m.tableCols(w)))
}

// tableWindow devolve a faixa de skills visíveis e a linha onde a primeira é
// desenhada — a mesma conta para renderizar e para o clique.
func (m Tab) tableWindow(w, h int) (start, end, top int) {
	top = len(m.tableHead(w))
	start, end = kit.Window(m.list.Index(), len(m.list.VisibleItems()), max(1, h-top))
	return start, end, top
}

// tableView desenha a matriz skill × agente em w×h.
func (m Tab) tableView(w, h int) string {
	lines := m.tableHead(w)
	items := m.list.VisibleItems()
	switch {
	case len(items) == 0 && m.list.FilterState() == list.FilterApplied:
		lines = append(lines, kit.StHint.Render("  Nada encontrado para “"+m.list.FilterValue()+"”."))
	case len(m.skills) == 0:
		lines = append(lines, kit.StHint.Render("  Biblioteca vazia — ")+components.Keycap("i")+kit.StHint.Render(" instala"))
	}
	cols := m.tableCols(w)
	start, end, _ := m.tableWindow(w, h)
	for i := start; i < end; i++ {
		if it, ok := items[i].(skillItem); ok {
			sel := i == m.list.Index()
			lines = append(lines, kit.TableRow(w, sel, cols, m.cells(it, sel)...))
		}
	}
	// Sem espaço no título, a legenda desce para o pé da matriz, se sobrar linha.
	foot := ""
	if !strings.Contains(lines[0], "inativa") && h-len(lines) >= 2 {
		foot = "  " + legend
	}
	return kit.Frame(strings.Join(lines, "\n"), foot, h)
}

// refreshDetail recomputa o conteúdo do painel de detalhe no viewport, mantendo
// o scroll (só volta ao topo quando a skill selecionada muda).
func (m *Tab) refreshDetail() {
	w, h := kit.DetailSize(m.split())
	m.detailVP.SetWidth(w)
	m.detailVP.SetHeight(h)
	name := ""
	if sel, ok := m.selected(); ok {
		name = sel.Name
	}
	if name != m.detailName {
		m.detailVP.GotoTop()
		m.detailName = name
	}
	m.detailVP.SetContent(m.detailContent(w))
}

// detailView mostra o detalhe da skill ao lado ou sob a matriz.
func (m Tab) detailView(sp kit.Split) string { return kit.DetailView(sp, "SOBRE A SKILL", m.detailVP) }

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
		b.WriteString(clampLines(kit.StText.Render(sel.Description), inner, descLines) + "\n")
	}
	if sel.Warning != "" {
		b.WriteString(kit.StErr.Render("⚠ "+sel.Warning) + "\n")
	}
	for _, is := range m.selectedIssues() {
		b.WriteString(kit.StWarn.Render(fmt.Sprintf("! %s: %s", is.Field, is.Msg)) + "\n")
	}
	b.WriteString("\n" + kit.StHint.Render("DISPONÍVEL NOS AGENTES") + "\n")
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
		cursor := "  "
		if i == m.col {
			cursor = kit.StTitle.Render("▸ ") // o agente que space alterna
		}
		b.WriteString(fmt.Sprintf("%s%s %s %s  %s\n", cursor,
			components.Keycap(fmt.Sprintf("%d", i+1)), mark, kit.CardValue.Render(name), status))
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
	return lipgloss.NewStyle().Width(inner).Render(strings.TrimRight(b.String(), "\n"))
}

// descLines é quanto da descrição cabe no card antes dos agentes; o texto
// inteiro está no SKILL.md (enter).
const descLines = 4

// clampLines quebra text em width colunas e corta em n linhas, avisando que
// há mais.
func clampLines(text string, width, n int) string {
	lines := strings.Split(lipgloss.NewStyle().Width(width).Render(text), "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[:n], "\n") + "\n" +
		kit.StHint.Render("… ") + components.Keycap("enter") + kit.StHint.Render(" lê o SKILL.md inteiro")
}

// inputModal emoldura um prompt de texto (install/nova/perfil) num Panel, com
// o toast abaixo. Largura limitada para não virar uma faixa vazia.
func (m Tab) inputModal(title, prompt, hint string) string {
	w := m.width
	if w > 72 {
		w = 72
	}
	content := lipgloss.JoinVertical(lipgloss.Left, lipgloss.NewStyle().Width(max(1, w-4)).Render(prompt), "", components.InputView(m.input, w-4), "", hint)
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
