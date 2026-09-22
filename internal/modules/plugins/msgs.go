package plugins

// Toda mensagem carrega o id do plugin: o root faz broadcast das mensagens
// assíncronas para todas as abas, e cada proxy ignora as alheias.

// frameMsg é uma mensagem do plugin lida de Proc.Events.
type frameMsg struct {
	id  string
	msg Msg
}

// exitMsg é Events fechado: o plugin morreu ou violou o protocolo.
type exitMsg struct {
	id     string
	err    error
	stderr string
}

// commandMsg é uma entrada da paleta do plugin escolhida pelo usuário.
type commandMsg struct {
	id   string
	name string
}

// execDoneMsg é o fim de um exec pedido pelo plugin.
type execDoneMsg struct {
	id     string
	execID int
	code   int
	stdout string
	stderr string
	err    error
}
