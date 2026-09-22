package agent

import (
	"context"
	"time"
)

// Em conta por assinatura o limite não é token nem custo: é percentual de uma
// janela (sessão e semana) com horário de reset. Este arquivo é o contrato
// dessa leitura; cada adapter resolve pela fonte do seu CLI.

// Tipos de janela conhecidos.
const (
	WindowSession     = "session"      // janela curta (5h no Claude Code)
	WindowWeekly      = "weekly"       // janela semanal da conta
	WindowWeeklyModel = "weekly_model" // janela semanal de um modelo específico
)

// RateWindow é uma janela de limite em uso.
type RateWindow struct {
	Kind        string    // WindowSession | WindowWeekly | WindowWeeklyModel
	Label       string    // rótulo curto já pronto para exibição
	UsedPercent float64   // 0–100
	ResetsAt    time.Time // zero = desconhecido
	Severity    string    // "normal", "warning"… best-effort, pode ser vazio
}

// RateStatus é a situação dos limites de um agente num instante.
type RateStatus struct {
	Plan      string       // plano da assinatura (ex.: "max", "prolite")
	Windows   []RateWindow // ordenadas: sessão, semana, semana por modelo
	FetchedAt time.Time
	Source    string // "api" (rede) | "rollout" (arquivo local)
}

// RateLimitReader é implementado pelos adapters que sabem informar os limites
// da assinatura. Opcional, fora da interface Adapter (padrão de UsageReader).
// Implementações que fazem rede respeitam o ctx e nunca são chamadas no boot.
type RateLimitReader interface {
	RateLimits(ctx context.Context) (RateStatus, error)
}
