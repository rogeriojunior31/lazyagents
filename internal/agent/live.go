package agent

// LiveChecker é implementado opcionalmente pelos adapters que sabem dizer se
// uma sessão está em andamento agora (M8.A2). Fica fora da interface Adapter
// de propósito — nem todo agente suporta a checagem — quem consome faz type
// assertion.
type LiveChecker interface {
	// IsLive diz se a sessão tem um processo do agente com ela aberta agora.
	// false também cobre "não sei dizer".
	IsLive(s Session) bool
}
