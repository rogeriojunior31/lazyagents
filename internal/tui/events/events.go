// Package events define as mensagens Bubble Tea trocadas ENTRE módulos da TUI.
// Regra: mensagem consumida por um módulo diferente do que a produziu mora
// aqui; mensagens internas de um módulo ficam não exportadas no pacote dele.
// O root faz broadcast de toda mensagem não-input para todos os módulos.
package events

import (
	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/skill"
)

// AgentsDetected é emitida pelo root após agent.DetectAll — dispara o
// primeiro scan de skills e a carga de sessões.
type AgentsDetected struct {
	Agents []agent.Agent
}

// SkillsScanned é emitida pelo módulo de skills após cada scan.
type SkillsScanned struct {
	Skills []skill.Skill
	Err    error
}

// SessionsLoaded é emitida pelo módulo de sessões após cada carga.
type SessionsLoaded struct {
	Sessions []agent.Session
	Err      error
}

// Reload pede ao módulo ativo que recarregue seus dados (paleta :reload).
type Reload struct{}
