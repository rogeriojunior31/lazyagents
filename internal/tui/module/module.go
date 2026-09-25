// Package module define o contrato de uma aba da TUI. Adicionar uma aba nova é
// implementar Module e registrá-la em internal/app — o root (tui/app.go) nunca
// é editado para isso.
package module

import tea "charm.land/bubbletea/v2"

// Module é uma aba. Semântica de ponteiro: Update muta o receptor e devolve só
// o Cmd, então o root guarda []Module sem conhecer o tipo concreto.
type Module interface {
	ID() string    // slug estável: "skills", "sessions"... (nome na paleta)
	Title() string // rótulo da aba sem contador: "Skills", "Sessions"
	Count() int    // contador na pill; -1 = sem contador
	Init() tea.Cmd
	// Update recebe sempre WindowSizeMsg e os broadcasts (events.*), e só
	// quando ativa as entradas (tecla, mouse, paste, events.Reload).
	Update(msg tea.Msg) tea.Cmd
	View() string    // só o corpo; o root desenha header, abas e rodapé
	Capturing() bool // true enquanto um input/filtro/modal é dono do teclado
	ClearToast()     // chamado ao trocar de aba
	Help() []HelpGroup
}

// HelpGroup é um bloco de teclas do modal de ajuda (?): título + {tecla, descrição}.
type HelpGroup struct {
	Title string
	Keys  [][2]string
}

// Commander é opcional: comandos próprios do módulo na paleta (:). Ao
// escolher, o root ativa o módulo e entrega Msg ao Update dele.
type Commander interface {
	Commands() []Command
}

// Command é uma entrada da paleta contribuída por um módulo.
type Command struct {
	Name string
	Desc string
	Msg  tea.Msg
}
