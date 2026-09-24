package kit

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

func testCols() []Column {
	return []Column{
		{Title: "nome", Width: 12},
		{Title: "descrição", Flex: true},
		{Title: "agente", Width: 6, Align: lipgloss.Center},
	}
}

func TestTableRowExactWidth(t *testing.T) {
	cells := []string{"frontend-design-系統", "Interfaces com personalidade e hierarquia 🎨", StOn.Render("●")}
	for _, w := range []int{0, 1, 5, 40, 80, 120, 200} {
		for _, sel := range []bool{false, true} {
			row := TableRow(w, sel, testCols(), cells...)
			if strings.Contains(row, "\n") {
				t.Fatalf("w=%d: linha quebrada", w)
			}
			if got := lipgloss.Width(row); got != w {
				t.Errorf("w=%d sel=%v: largura %d", w, sel, got)
			}
		}
	}
}

func TestTableHeaderAlignsWithRow(t *testing.T) {
	cols := testCols()
	head := ansi.Strip(TableHeader(80, cols))
	row := ansi.Strip(TableRow(80, false, cols, "a", "b", "●"))
	col := func(s, sub string) int { return lipgloss.Width(s[:strings.Index(s, sub)]) }
	if col(head, "agente")+(len("agente")-1)/2 != col(row, "●") { // centro de 1 em 6
		t.Errorf("coluna desalinhada:\n%q\n%q", head, row)
	}
	if !strings.HasPrefix(head, "  nome") {
		t.Errorf("cabeçalho = %q", head)
	}
}

// A linha selecionada mantém a cor da célula e o fundo de seleção depois dela.
func TestTableRowSelectedKeepsCellColor(t *testing.T) {
	cell := lipgloss.NewStyle().Foreground(theme.OK).Render("●")
	row := TableRow(40, true, testCols(), "nome", "desc", cell)
	if !strings.Contains(row, cell[:strings.Index(cell, "●")]) {
		t.Fatal("cor da célula removida na linha selecionada")
	}
	after := row[strings.Index(row, "●"):]
	if !strings.Contains(after, selSGR()) {
		t.Error("fundo de seleção não reaberto depois da célula colorida")
	}
	if !strings.HasPrefix(ansi.Strip(row), "▎ nome") {
		t.Errorf("linha = %q", ansi.Strip(row))
	}
}

func TestTableRowTruncatesFlex(t *testing.T) {
	row := ansi.Strip(TableRow(40, false, testCols(), "nome", strings.Repeat("x", 100), "●"))
	if !strings.Contains(row, "…") || !strings.Contains(row, "●") {
		t.Errorf("flex deveria truncar e manter a coluna fixa: %q", row)
	}
}

func TestAgentColumnsFallbackToShort(t *testing.T) {
	ags := []agent.Agent{{ID: "claude-code", Short: "C"}, {ID: "gemini-cli", Short: "G"}, {ID: "hermes-agent", Short: "H"}}
	wide := AgentColumns(ags, 80)
	if got := ansi.Strip(wide[0].Title) + ansi.Strip(wide[1].Title) + ansi.Strip(wide[2].Title); got != "claudegeminihermes" {
		t.Errorf("rótulos = %q", got)
	}
	narrow := AgentColumns(ags, 10)
	for i, want := range []string{"C", "G", "H"} {
		if ansi.Strip(narrow[i].Title) != want || narrow[i].Width != 1 {
			t.Errorf("coluna %d = %q/%d", i, narrow[i].Title, narrow[i].Width)
		}
	}
}

type cellItem struct{ name string }

func (c cellItem) Cells() []string     { return []string{c.name, "desc", "●"} }
func (c cellItem) FilterValue() string { return c.name }

type groupItem struct{}

func (groupItem) Title() string       { return "projeto · claude (2)" }
func (groupItem) FilterValue() string { return "" }

func TestTableDelegateOneLinePerItem(t *testing.T) {
	d := TableDelegate{Cols: func(int) []Column { return testCols() }}
	l := list.New([]list.Item{groupItem{}, cellItem{"a"}, cellItem{"b"}}, d, 60, 10)
	var b strings.Builder
	d.Render(&b, l, 1, cellItem{"a"})
	if strings.Contains(b.String(), "\n") || lipgloss.Width(b.String()) != 60 {
		t.Errorf("linha = %q", b.String())
	}
	b.Reset()
	d.Render(&b, l, 0, groupItem{})
	if ansi.Strip(b.String()) != "  projeto · claude (2)" {
		t.Errorf("grupo = %q", ansi.Strip(b.String()))
	}
}

func TestSplitDetail(t *testing.T) {
	s := SplitDetail(120, 30)
	if !s.Side || s.ListW+2+s.DetailW != 120 || s.ListH != 30 {
		t.Errorf("lado a lado: %+v", s)
	}
	for _, h := range []int{1, 4, 11, 30} {
		s = SplitDetail(80, h)
		if s.Side || s.ListH+s.DetailH != h || s.DetailH > 6 || (h >= 6 && s.ListH < 3) {
			t.Errorf("h=%d empilhado: %+v", h, s)
		}
	}
}

func TestFrameKeepsFooterOnLastLine(t *testing.T) {
	for _, body := range []string{"a", strings.Repeat("x\n", 30) + "x"} {
		out := strings.Split(Frame(body, "rodapé", 10), "\n")
		if len(out) != 10 || out[9] != "rodapé" {
			t.Errorf("frame = %q", out)
		}
	}
	if out := strings.Split(Frame("a", "1\n2\n3", 2), "\n"); len(out) != 2 || out[1] != "3" {
		t.Errorf("rodapé maior que a altura = %q", out)
	}
}
