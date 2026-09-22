package agent

import (
	"encoding/json"
	"os"
	"time"
)

// Usage é o consumo de tokens agregado de uma sessão.
type Usage struct {
	Input      int
	Output     int
	CacheRead  int
	CacheWrite int
	Model      string // último modelo visto na sessão (best-effort)
}

// UsageReader é implementado opcionalmente pelos adapters que sabem somar o
// uso de tokens de uma sessão. Fica fora da interface Adapter de propósito —
// nem todo agente registra usage no transcript — quem consome faz type
// assertion.
type UsageReader interface {
	// SessionUsage soma o usage da sessão. ok=false quando o agente não
	// registra usage ou o transcript não tem a informação.
	SessionUsage(s Session) (Usage, bool)
}

// UsageEvent é um consumo pontual de tokens: uma resposta do modelo, com o
// instante em que aconteceu. É o insumo das janelas do módulo de uso.
type UsageEvent struct {
	AgentID string // preenchido por quem agrega (o adapter não precisa saber)
	Time    time.Time
	Model   string
	CWD     string
	Usage   Usage
}

// UsageEventReader é implementado pelos adapters que registram usage com
// data no transcript. Opcional, fora da interface Adapter: quem consome faz
// type assertion (padrão de UsageReader).
type UsageEventReader interface {
	// UsageEvents devolve os eventos de uso da sessão em ordem de arquivo.
	// Sessão sem registro de uso devolve lista vazia sem erro.
	UsageEvents(s Session) ([]UsageEvent, error)
}

// AuthMode diz como o agente está autenticado — o que decide se faz sentido
// estimar custo em USD (assinatura não é cobrada por token).
type AuthMode int

const (
	AuthUnknown AuthMode = iota
	AuthSubscription
	AuthAPIKey
)

func (m AuthMode) String() string {
	switch m {
	case AuthSubscription:
		return "assinatura"
	case AuthAPIKey:
		return "API key"
	default:
		return "desconhecido"
	}
}

// AuthModeReader é implementado pelos adapters que sabem dizer como estão
// autenticados. detail é um complemento curto e nunca secreto (ex.: o plano).
type AuthModeReader interface {
	AuthMode() (mode AuthMode, detail string)
}

// secret marca a presença de um campo secreto sem guardar o valor: os
// decoders deste pacote nunca materializam token, chave ou senha em memória.
type secret bool

func (s *secret) UnmarshalJSON(b []byte) error {
	*s = secret(string(b) != "null" && string(b) != `""`)
	return nil
}

// decodeJSONFile decodifica um arquivo direto do disco (streaming, sem manter
// o conteúdo inteiro em memória) em v. Ausente ou inválido devolve erro.
func decodeJSONFile(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(v)
}
