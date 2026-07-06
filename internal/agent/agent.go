// Package agent é o ÚNICO lugar do projeto que conhece paths e formatos de
// arquivo dos agentes de coding AI (Claude Code, Codex, Gemini CLI, OpenCode,
// Claude Desktop, Hermes Agent). Cada agente tem um adapter em seu próprio
// arquivo; o registry monta a lista completa.
package agent

import "time"

// Agent descreve um agente de código detectado (ou não) na máquina.
type Agent struct {
	ID         string   // identificador estável, ex.: "claude-code"
	Name       string   // nome de exibição, ex.: "Claude Code"
	Short      string   // letra única para a matriz da TUI, ex.: "C"
	Installed  bool     // binário no PATH e/ou dir de config presente
	Version    string   // saída de --version, se disponível
	ManagedDir string   // dir onde o lazyskills ativa skills ("" = sem suporte)
	ReadDirs   []string // TODOS os dirs de skills que o agente lê (inclui ManagedDir)
	SharedNote string   // aviso quando ManagedDir é compartilhado com outros agentes
	Detail     string   // como foi detectado / observações
}

// SupportsSkills informa se o agente tem um diretório de skills gerenciável.
func (a Agent) SupportsSkills() bool { return a.ManagedDir != "" }

// Session é uma sessão/conversa de um agente, normalizada entre plataformas.
type Session struct {
	AgentID   string    // qual agente originou a sessão
	AgentName string    // nome de exibição do agente
	ID        string    // identificador usado no resume
	Path      string    // arquivo/registro de origem
	CWD       string    // diretório de trabalho da sessão ("" se desconhecido)
	Title     string    // primeiro prompt ou título da conversa
	MTime     time.Time // última modificação
}

// Adapter é a interface implementada por cada agente suportado.
type Adapter interface {
	// ID devolve o identificador estável do agente (barato, sem I/O).
	ID() string
	// Detect verifica instalação, versão e diretórios de skills.
	Detect() Agent
	// ListSessions lista as sessões locais do agente (vazio se não houver).
	ListSessions() ([]Session, error)
	// ResumeCmd retorna o argv que retoma a sessão e o diretório onde rodar.
	// ok=false quando a plataforma não suporta resume via CLI.
	ResumeCmd(s Session) (argv []string, dir string, ok bool)
}
