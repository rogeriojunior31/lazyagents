package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"time"
)

// Limites de assinatura do Claude Code: a mesma fonte do /usage do CLI. Não
// há nada em disco, então é uma chamada HTTP — feita só sob demanda, nunca no
// boot, e cacheada por quem chama (internal/usage).
const (
	claudeUsageURL  = "https://api.anthropic.com/api/oauth/usage?at_wall=1&skip_spend=1"
	claudeOAuthBeta = "oauth-2025-04-20"
	maxUsageBody    = 1 << 20
)

// claudeUsageResponse cobre só o que a aba mostra.
type claudeUsageResponse struct {
	Limits []struct {
		Kind     string  `json:"kind"` // session | weekly_all | weekly_scoped
		Percent  float64 `json:"percent"`
		Severity string  `json:"severity"`
		ResetsAt string  `json:"resets_at"`
		Scope    *struct {
			Model *struct {
				DisplayName string `json:"display_name"`
			} `json:"model"`
		} `json:"scope"`
	} `json:"limits"`
	FiveHour *claudeLegacyWindow `json:"five_hour"`
	SevenDay *claudeLegacyWindow `json:"seven_day"`
}

// claudeLegacyWindow é o formato antigo, usado como fallback quando a
// resposta não traz limits[].
type claudeLegacyWindow struct {
	Utilization float64 `json:"utilization"`
	ResetsAt    string  `json:"resets_at"`
}

// RateLimits busca as janelas de limite da assinatura.
//
// Este é o ÚNICO ponto do lazyagents que materializa o token do Claude Code:
// ele é lido do .credentials.json, usado no header Authorization e descartado
// — nunca é exibido, logado, persistido nem guardado em struct exportada.
func (c *Claude) RateLimits(ctx context.Context) (RateStatus, error) {
	var creds struct {
		OAuth struct {
			AccessToken      string `json:"accessToken"`
			ExpiresAt        int64  `json:"expiresAt"` // epoch em milissegundos
			SubscriptionType string `json:"subscriptionType"`
		} `json:"claudeAiOauth"`
	}
	if err := decodeJSONFile(filepath.Join(c.configDir(), ".credentials.json"), &creds); err != nil {
		return RateStatus{}, errors.New("sem credenciais do Claude Code: entre com /login no CLI")
	}
	if creds.OAuth.AccessToken == "" {
		return RateStatus{}, errors.New("conta do Claude Code sem sessão OAuth (conta por API key não tem limite de assinatura)")
	}
	if creds.OAuth.ExpiresAt > 0 && time.UnixMilli(creds.OAuth.ExpiresAt).Before(time.Now()) {
		return RateStatus{}, errors.New("sessão do Claude Code expirada: abra o CLI para renovar")
	}
	url := c.UsageURL
	if url == "" {
		url = claudeUsageURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return RateStatus{}, err
	}
	req.Header.Set("Authorization", "Bearer "+creds.OAuth.AccessToken)
	req.Header.Set("anthropic-beta", claudeOAuthBeta)
	req.Header.Set("Accept", "application/json")
	client := &http.Client{
		Timeout: 15 * time.Second,
		// sem redirect: o header Authorization nunca segue para outro host.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		return RateStatus{}, fmt.Errorf("consultando o uso do Claude Code: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return RateStatus{}, fmt.Errorf("consultando o uso do Claude Code: HTTP %d", resp.StatusCode)
	}
	var body claudeUsageResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxUsageBody)).Decode(&body); err != nil {
		return RateStatus{}, fmt.Errorf("resposta de uso do Claude Code inválida: %w", err)
	}
	return RateStatus{
		Plan:      creds.OAuth.SubscriptionType,
		Windows:   claudeWindows(body),
		FetchedAt: time.Now(),
		Source:    "api",
	}, nil
}

// claudeWindows traduz a resposta para RateWindow, preferindo limits[] e
// caindo no formato antigo quando ele não vem.
func claudeWindows(body claudeUsageResponse) []RateWindow {
	var out []RateWindow
	for _, l := range body.Limits {
		w := RateWindow{UsedPercent: l.Percent, Severity: l.Severity, ResetsAt: parseTime(l.ResetsAt)}
		switch l.Kind {
		case "session":
			w.Kind, w.Label = WindowSession, "sessão 5h"
		case "weekly_all":
			w.Kind, w.Label = WindowWeekly, "semana"
		case "weekly_scoped":
			w.Kind, w.Label = WindowWeeklyModel, "semana"
			if l.Scope != nil && l.Scope.Model != nil && l.Scope.Model.DisplayName != "" {
				w.Label = "semana · " + l.Scope.Model.DisplayName
			}
		default:
			continue
		}
		out = append(out, w)
	}
	if len(out) == 0 {
		if w := body.FiveHour; w != nil {
			out = append(out, RateWindow{Kind: WindowSession, Label: "sessão 5h", UsedPercent: w.Utilization, ResetsAt: parseTime(w.ResetsAt)})
		}
		if w := body.SevenDay; w != nil {
			out = append(out, RateWindow{Kind: WindowWeekly, Label: "semana", UsedPercent: w.Utilization, ResetsAt: parseTime(w.ResetsAt)})
		}
	}
	sortWindows(out)
	return out
}

// parseTime aceita o ISO-8601 da API; vazio ou inválido vira zero.
func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// sortWindows ordena sessão → semana → semana por modelo (ordem de exibição).
func sortWindows(ws []RateWindow) {
	rank := map[string]int{WindowSession: 0, WindowWeekly: 1, WindowWeeklyModel: 2}
	sort.SliceStable(ws, func(i, j int) bool { return rank[ws[i].Kind] < rank[ws[j].Kind] })
}
