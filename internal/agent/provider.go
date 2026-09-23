package agent

// ProviderProfile é um perfil de endpoint/modelo aplicável na config viva de
// um agente (estilo cc-switch): trocar de provedor sem editar arquivo à mão.
// Token é o único campo secreto e nunca sai deste struct para exibição — quem
// mostra usa Redacted.
type ProviderProfile struct {
	Name    string `json:"name"`              // nome do perfil no lazyagents
	BaseURL string `json:"baseUrl,omitempty"` // endpoint compatível
	Model   string `json:"model,omitempty"`   // modelo padrão ("" = não mexe)
	Token   string `json:"token,omitempty"`   // SEGREDO: só em providers.json (0600) e na config do agente
	EnvKey  string `json:"envKey,omitempty"`  // nome da variável de ambiente com o token (agentes que não aceitam token no arquivo)
	WireAPI string `json:"wireApi,omitempty"` // Codex: responses ("" = default do próprio Codex)

	// HasToken só é preenchido por Redacted e pela leitura da config viva:
	// diz que existe um token sem revelar qual.
	HasToken bool `json:"hasToken,omitempty"`
}

// Redacted devolve o perfil sem o segredo, com HasToken no lugar. É a forma
// usada na TUI, no --json e em qualquer log (regra 7). Idempotente: redigir
// um perfil já redigido não apaga o HasToken que veio da config viva.
func (p ProviderProfile) Redacted() ProviderProfile {
	p.HasToken = p.HasToken || p.Token != ""
	p.Token = ""
	return p
}

// ProviderHost é implementado pelos adapters que sabem aplicar um perfil de
// provedor na própria config. Opcional, fora da interface Adapter: quem
// consome faz type assertion (padrão de UsageReader).
type ProviderHost interface {
	// ProviderFile é o arquivo de config que Apply/Clear escrevem.
	ProviderFile() string
	// ReadProvider devolve o provedor aplicado agora. O Token NUNCA vem
	// preenchido — só HasToken. ok=false quando não há nada aplicado.
	ReadProvider() (p ProviderProfile, ok bool, err error)
	// ApplyProvider grava o perfil, com backup do arquivo vivo em backupsDir.
	ApplyProvider(p ProviderProfile, backupsDir string) error
	// ClearProvider desfaz o que ApplyProvider escreveu, preservando o que
	// não é gerenciado pelo lazyagents.
	ClearProvider(backupsDir string) error
}
