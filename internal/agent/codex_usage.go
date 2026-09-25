package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// O rollout do Codex já traz consumo e limites; nada aqui vai à rede.
//
// Consumo por resposta: evento `token_usage_record` (payload.usage). Em
// versões antigas, `event_msg` com payload.type == "token_count"
// (payload.info.last_token_usage). Os dois convivem no mesmo arquivo, então o
// legado só é usado quando não há nenhum token_usage_record.
//
// Limites da assinatura: o último `token_count` com payload.rate_limits.

type codexUsageNumbers struct {
	InputTokens          int `json:"input_tokens"` // já inclui cached_input_tokens
	CachedInputTokens    int `json:"cached_input_tokens"`
	CacheWriteInputToken int `json:"cache_write_input_tokens"`
	OutputTokens         int `json:"output_tokens"`
}

// usage converte para o Usage do app, tirando o cache do input para não
// contar o mesmo token duas vezes na estimativa de custo.
func (n codexUsageNumbers) usage(model string) Usage {
	return Usage{
		Input:      max(0, n.InputTokens-n.CachedInputTokens),
		Output:     n.OutputTokens,
		CacheRead:  n.CachedInputTokens,
		CacheWrite: n.CacheWriteInputToken,
		Model:      model,
	}
}

type codexUsageLine struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Payload   struct {
		Type  string             `json:"type"` // em event_msg: "token_count"
		CWD   string             `json:"cwd"`  // session_meta / turn_context
		Model string             `json:"model"`
		Usage *codexUsageNumbers `json:"usage"` // token_usage_record
		Info  *struct {
			LastTokenUsage *codexUsageNumbers `json:"last_token_usage"`
		} `json:"info"`
		RateLimits *codexRateLimits `json:"rate_limits"`
	} `json:"payload"`
}

type codexRateLimits struct {
	Primary   *codexWindow `json:"primary"`
	Secondary *codexWindow `json:"secondary"`
	PlanType  string       `json:"plan_type"`
}

type codexWindow struct {
	UsedPercent   float64 `json:"used_percent"`
	WindowMinutes int     `json:"window_minutes"`
	ResetsAt      int64   `json:"resets_at"` // epoch em segundos
}

// UsageEvents devolve um evento por resposta do modelo no rollout. Modelo e
// pasta não vêm no evento de tokens: são herdados do último turn_context /
// session_meta lido antes dele.
func (c *Codex) UsageEvents(s Session) ([]UsageEvent, error) {
	e, err := c.index().refresh(s.Path, codexIndexLine)
	if err != nil {
		return nil, fmt.Errorf("lendo rollout: %w", err)
	}
	if len(e.Events) == 0 {
		return e.events(e.Legacy, s), nil // rollout de versão antiga
	}
	return e.events(e.Events, s), nil
}

// codexKeys são os trechos que uma linha útil ao índice contém: evento de
// tokens (token_usage_record, token_count, que também traz os limites) ou
// contexto (session_meta e turn_context trazem cwd e modelo). As mensagens,
// a maior parte do rollout, são puladas sem decodificar.
var codexKeys = [][]byte{[]byte(`"token_`), []byte(`"cwd"`), []byte(`"model"`)}

// codexIndexLine extrai de uma linha do rollout as respostas (com o modelo e
// a pasta herdados do último contexto lido) e os limites da assinatura.
func codexIndexLine(e *indexEntry, line []byte) {
	if !slices.ContainsFunc(codexKeys, func(k []byte) bool { return bytes.Contains(line, k) }) {
		return
	}
	var l codexUsageLine
	if json.Unmarshal(line, &l) != nil {
		return // linha ilegível: best-effort
	}
	if l.Payload.Model != "" {
		e.Model = l.Payload.Model
	}
	if l.Payload.CWD != "" {
		e.CtxCWD = l.Payload.CWD
		if e.CWD == "" {
			e.CWD = l.Payload.CWD // pasta da sessão (session_meta): as faixas só guardam a diferente
		}
	}
	ts, _ := time.Parse(time.RFC3339, l.Timestamp) // sem data: zero, a da sessão vale
	switch {
	case l.Type == "token_usage_record" && l.Payload.Usage != nil:
		e.addEvent(&e.Events, ts, e.Model, e.CtxCWD, l.Payload.Usage.usage(e.Model))
	case l.Payload.Type == "token_count" && l.Payload.Info != nil && l.Payload.Info.LastTokenUsage != nil:
		e.addEvent(&e.Legacy, ts, e.Model, e.CtxCWD, l.Payload.Info.LastTokenUsage.usage(e.Model))
	}
	if l.Payload.RateLimits != nil {
		rl := *l.Payload.RateLimits
		e.Rate, e.RateAt = &rl, 0
		if !ts.IsZero() {
			e.RateAt = ts.UnixNano()
		}
	}
}

// RateLimits lê os limites do rollout mais recente que os registrou. Offline:
// o Codex grava used_percent, a janela e o reset a cada turno.
func (c *Codex) RateLimits(context.Context) (RateStatus, error) {
	sessions, err := c.ListSessions()
	if err != nil {
		return RateStatus{}, err
	}
	if len(sessions) == 0 {
		return RateStatus{}, errors.New("nenhuma sessão do Codex encontrada")
	}
	// ListSessions devolve as mais recentes primeiro (e já pôs o índice em
	// dia); poucos arquivos bastam porque todo turno registra os limites.
	for i, s := range sessions {
		if i >= 5 {
			break
		}
		e, ok := c.index().get(s.Path)
		if !ok || e.Rate == nil {
			continue
		}
		var at time.Time
		if e.RateAt != 0 {
			at = time.Unix(0, e.RateAt)
		}
		return RateStatus{Plan: e.Rate.PlanType, Windows: codexWindows(*e.Rate), FetchedAt: at, Source: "rollout"}, nil
	}
	return RateStatus{}, errors.New("nenhum limite registrado nas sessões recentes do Codex")
}

// codexWindows traduz as janelas do Codex; o tipo sai de window_minutes
// (10080 = semana), não da posição, que varia por conta.
func codexWindows(rl codexRateLimits) []RateWindow {
	var out []RateWindow
	for _, w := range []*codexWindow{rl.Primary, rl.Secondary} {
		if w == nil {
			continue
		}
		rw := RateWindow{Kind: WindowSession, Label: codexWindowLabel(w.WindowMinutes), UsedPercent: w.UsedPercent}
		if w.WindowMinutes >= 7*24*60 {
			rw.Kind = WindowWeekly
		}
		if w.ResetsAt > 0 {
			rw.ResetsAt = time.Unix(w.ResetsAt, 0)
		}
		out = append(out, rw)
	}
	sortWindows(out)
	return out
}

func codexWindowLabel(minutes int) string {
	switch {
	case minutes <= 0:
		return "window"
	case minutes >= 7*24*60:
		return fmt.Sprintf("week (%dd)", minutes/(24*60))
	case minutes >= 24*60:
		return fmt.Sprintf("%dd", minutes/(24*60))
	case minutes >= 60:
		return fmt.Sprintf("session %dh", minutes/60)
	default:
		return fmt.Sprintf("%dmin", minutes)
	}
}

// AuthMode lê só o modo em ~/.codex/auth.json; a chave nunca é decodificada,
// apenas a presença dela.
func (c *Codex) AuthMode() (AuthMode, string) {
	var auth struct {
		AuthMode string `json:"auth_mode"`
		APIKey   secret `json:"OPENAI_API_KEY"`
	}
	if err := decodeJSONFile(filepath.Join(c.configDir(), "auth.json"), &auth); err == nil {
		switch auth.AuthMode {
		case "chatgpt", "oauth":
			return AuthSubscription, "ChatGPT"
		case "apikey", "api_key":
			return AuthAPIKey, "auth.json"
		}
		if bool(auth.APIKey) {
			return AuthAPIKey, "auth.json"
		}
		if auth.AuthMode != "" {
			return AuthSubscription, auth.AuthMode
		}
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		return AuthAPIKey, "OPENAI_API_KEY"
	}
	return AuthUnknown, ""
}
