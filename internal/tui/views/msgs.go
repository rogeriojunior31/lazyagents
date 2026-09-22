// Package views contém as abas da TUI. Nenhum I/O acontece fora de tea.Cmd.
package views

import "github.com/rogeriojunior31/lazyagents/internal/agent"

// AgentsMsg é emitida pelo root após a detecção dos agentes e broadcast para
// todas as views — dispara o primeiro scan de skills e a carga de sessões.
type AgentsMsg struct {
	Agents []agent.Agent
}
