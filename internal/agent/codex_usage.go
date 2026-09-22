package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	var events, legacy []UsageEvent
	ctx := codexCtx{cwd: s.CWD}
	err := scanCodexRollout(s.Path, func(l codexUsageLine) {
		if l.Payload.Model != "" {
			ctx.model = l.Payload.Model
		}
		if l.Payload.CWD != "" {
			ctx.cwd = l.Payload.CWD
		}
		switch {
		case l.Type == "token_usage_record" && l.Payload.Usage != nil:
			events = append(events, codexEvent(l, *l.Payload.Usage, s, ctx))
		case l.Payload.Type == "token_count" && l.Payload.Info != nil && l.Payload.Info.LastTokenUsage != nil:
			legacy = append(legacy, codexEvent(l, *l.Payload.Info.LastTokenUsage, s, ctx))
		}
	})
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return legacy, nil // rollout de versão antiga
	}
	return events, nil
}

// codexCtx é o contexto corrente do rollout (a última pasta e modelo vistos).
type codexCtx struct{ model, cwd string }

func codexEvent(l codexUsageLine, n codexUsageNumbers, s Session, ctx codexCtx) UsageEvent {
	ts, err := time.Parse(time.RFC3339, l.Timestamp)
	if err != nil {
		ts = s.MTime
	}
	return UsageEvent{Time: ts, Model: ctx.model, CWD: ctx.cwd, Usage: n.usage(ctx.model)}
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
	// ListSessions devolve as mais recentes primeiro; poucos arquivos bastam
	// porque todo turno registra os limites.
	for i, s := range sessions {
		if i >= 5 {
			break
		}
		var last *codexRateLimits
		var at time.Time
		err := scanCodexRollout(s.Path, func(l codexUsageLine) {
			if l.Payload.RateLimits == nil {
				return
			}
			rl := *l.Payload.RateLimits
			last = &rl
			if t, err := time.Parse(time.RFC3339, l.Timestamp); err == nil {
				at = t
			}
		})
		if err != nil || last == nil {
			continue
		}
		return RateStatus{Plan: last.PlanType, Windows: codexWindows(*last), FetchedAt: at, Source: "rollout"}, nil
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
		return "janela"
	case minutes >= 7*24*60:
		return fmt.Sprintf("semana (%dd)", minutes/(24*60))
	case minutes >= 24*60:
		return fmt.Sprintf("%dd", minutes/(24*60))
	case minutes >= 60:
		return fmt.Sprintf("sessão %dh", minutes/60)
	default:
		return fmt.Sprintf("%dmin", minutes)
	}
}

// scanCodexRollout aplica fn a cada linha decodificável do rollout.
func scanCodexRollout(path string, fn func(codexUsageLine)) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("lendo rollout: %w", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), maxLineBuf)
	for sc.Scan() {
		var l codexUsageLine
		if json.Unmarshal(sc.Bytes(), &l) != nil {
			continue // linha ilegível: best-effort
		}
		fn(l)
	}
	return nil
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
