package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func writeLines(t *testing.T, path string, lines ...string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestClaudeUsageEvents(t *testing.T) {
	path := writeLines(t, filepath.Join(t.TempDir(), "s.jsonl"),
		`{"type":"user","message":{"role":"user","content":"oi"}}`,
		`{"type":"assistant","timestamp":"2026-09-22T10:00:00Z","cwd":"/p/um","message":{"model":"claude-opus-4","usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":2,"cache_creation_input_tokens":1}}}`,
		`{"type":"assistant","timestamp":"quebrado","message":{"model":"claude-opus-4","usage":{"input_tokens":7,"output_tokens":1}}}`,
		`linha ilegível`,
	)
	s := Session{Path: path, CWD: "/p/fallback", MTime: time.Date(2026, 9, 22, 9, 0, 0, 0, time.UTC)}
	events, err := (&Claude{}).UsageEvents(s)
	if err != nil || len(events) != 2 {
		t.Fatalf("events = %+v, err = %v", events, err)
	}
	e := events[0]
	if !e.Time.Equal(time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)) || e.CWD != "/p/um" || e.Model != "claude-opus-4" {
		t.Errorf("primeiro evento = %+v", e)
	}
	if e.Usage != (Usage{Input: 10, Output: 5, CacheRead: 2, CacheWrite: 1, Model: "claude-opus-4"}) {
		t.Errorf("usage = %+v", e.Usage)
	}
	if !events[1].Time.Equal(s.MTime) || events[1].CWD != "/p/fallback" {
		t.Errorf("linha sem data/cwd deve cair na sessão: %+v", events[1])
	}
}

func TestCodexUsageEvents(t *testing.T) {
	// formato atual: token_usage_record; input_tokens já inclui o cache
	path := writeLines(t, filepath.Join(t.TempDir(), "rollout-novo.jsonl"),
		`{"type":"session_meta","timestamp":"2026-09-22T10:00:00Z","payload":{"cwd":"/p/proj"}}`,
		`{"type":"turn_context","timestamp":"2026-09-22T10:00:01Z","payload":{"model":"gpt-5-codex","cwd":"/p/proj"}}`,
		`{"type":"token_usage_record","timestamp":"2026-09-22T10:00:02Z","payload":{"usage":{"input_tokens":100,"cached_input_tokens":80,"cache_write_input_tokens":5,"output_tokens":20,"total_tokens":120}}}`,
		`{"type":"event_msg","timestamp":"2026-09-22T10:00:03Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"cached_input_tokens":80,"output_tokens":20}}}}`,
	)
	events, err := (&Codex{}).UsageEvents(Session{Path: path})
	if err != nil || len(events) != 1 {
		t.Fatalf("o formato novo deve vencer o legado: %+v %v", events, err)
	}
	e := events[0]
	if e.Model != "gpt-5-codex" || e.CWD != "/p/proj" {
		t.Errorf("contexto herdado = %+v", e)
	}
	// input descontado do cache: 100-80
	if e.Usage != (Usage{Input: 20, Output: 20, CacheRead: 80, CacheWrite: 5, Model: "gpt-5-codex"}) {
		t.Errorf("usage = %+v", e.Usage)
	}

	// rollout antigo: só token_count
	old := writeLines(t, filepath.Join(t.TempDir(), "rollout-velho.jsonl"),
		`{"type":"event_msg","timestamp":"2026-09-22T10:00:03Z","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":50,"cached_input_tokens":10,"output_tokens":3}}}}`,
	)
	events, err = (&Codex{}).UsageEvents(Session{Path: old, CWD: "/p/velho"})
	if err != nil || len(events) != 1 || events[0].Usage.Input != 40 || events[0].CWD != "/p/velho" {
		t.Fatalf("fallback legado = %+v %v", events, err)
	}
}

func TestCodexRateLimits(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".codex", "sessions", "2026", "09", "22")
	writeLines(t, filepath.Join(dir, "rollout-2026-09-22T10-00-00-abc.jsonl"),
		`{"type":"session_meta","timestamp":"2026-09-22T10:00:00Z","payload":{"id":"abc","cwd":"/p"}}`,
		`{"type":"event_msg","timestamp":"2026-09-22T10:00:01Z","payload":{"type":"token_count","rate_limits":{"primary":{"used_percent":11.5,"window_minutes":10080,"resets_at":1790607529},"secondary":{"used_percent":40,"window_minutes":300,"resets_at":1790600000},"plan_type":"prolite"}}}`,
	)
	st, err := (&Codex{Home: home}).RateLimits(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Plan != "prolite" || st.Source != "rollout" || len(st.Windows) != 2 {
		t.Fatalf("status = %+v", st)
	}
	if st.Windows[0].Kind != WindowSession || st.Windows[0].UsedPercent != 40 || st.Windows[0].Label != "session 5h" {
		t.Errorf("janela de sessão = %+v", st.Windows[0])
	}
	if st.Windows[1].Kind != WindowWeekly || st.Windows[1].UsedPercent != 11.5 || !st.Windows[1].ResetsAt.Equal(time.Unix(1790607529, 0)) {
		t.Errorf("janela semanal = %+v", st.Windows[1])
	}
	if _, err := (&Codex{Home: t.TempDir()}).RateLimits(context.Background()); err == nil {
		t.Error("sem sessões deveria falhar com mensagem amigável")
	}
}

func TestClaudeRateLimitsAPI(t *testing.T) {
	home := t.TempDir()
	creds := filepath.Join(home, ".claude", ".credentials.json")
	writeLines(t, creds, `{"claudeAiOauth":{"accessToken":"segredo-nao-vazar","expiresAt":`+
		itoa(time.Now().Add(time.Hour).UnixMilli())+`,"subscriptionType":"max"}}`)

	var gotAuth, gotBeta string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotBeta = r.Header.Get("Authorization"), r.Header.Get("anthropic-beta")
		_, _ = w.Write([]byte(`{"limits":[
			{"kind":"weekly_scoped","percent":22,"resets_at":"2026-09-24T21:00:00Z","scope":{"model":{"display_name":"Fable"}}},
			{"kind":"session","percent":3,"severity":"normal","resets_at":"2026-09-22T23:10:00Z"},
			{"kind":"weekly_all","percent":15,"resets_at":"2026-09-24T21:00:00Z"}]}`))
	}))
	defer srv.Close()

	c := &Claude{Home: home, UsageURL: srv.URL}
	st, err := c.RateLimits(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer segredo-nao-vazar" || gotBeta != claudeOAuthBeta {
		t.Errorf("headers = %q / %q", gotAuth, gotBeta)
	}
	if st.Plan != "max" || st.Source != "api" || len(st.Windows) != 3 {
		t.Fatalf("status = %+v", st)
	}
	kinds := []string{st.Windows[0].Kind, st.Windows[1].Kind, st.Windows[2].Kind}
	if kinds[0] != WindowSession || kinds[1] != WindowWeekly || kinds[2] != WindowWeeklyModel {
		t.Errorf("ordem das janelas = %v", kinds)
	}
	if st.Windows[2].Label != "week · Fable" || st.Windows[0].UsedPercent != 3 {
		t.Errorf("janelas = %+v", st.Windows)
	}

	// formato antigo (sem limits[])
	srvOld := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"five_hour":{"utilization":7,"resets_at":"2026-09-22T23:10:00Z"},"seven_day":{"utilization":9,"resets_at":null}}`))
	}))
	defer srvOld.Close()
	st, err = (&Claude{Home: home, UsageURL: srvOld.URL}).RateLimits(context.Background())
	if err != nil || len(st.Windows) != 2 || st.Windows[0].UsedPercent != 7 || !st.Windows[1].ResetsAt.IsZero() {
		t.Fatalf("fallback legado = %+v %v", st, err)
	}
}

// O erro e o status nunca podem carregar o token.
func TestClaudeRateLimitsNeverLeaksToken(t *testing.T) {
	home := t.TempDir()
	writeLines(t, filepath.Join(home, ".claude", ".credentials.json"),
		`{"claudeAiOauth":{"accessToken":"segredo-nao-vazar","expiresAt":`+itoa(time.Now().Add(time.Hour).UnixMilli())+`}}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	_, err := (&Claude{Home: home, UsageURL: srv.URL}).RateLimits(context.Background())
	if err == nil || strings.Contains(err.Error(), "segredo") {
		t.Fatalf("erro = %v", err)
	}
	// token expirado não chega a fazer requisição
	writeLines(t, filepath.Join(home, ".claude", ".credentials.json"),
		`{"claudeAiOauth":{"accessToken":"segredo-nao-vazar","expiresAt":1}}`)
	if _, err := (&Claude{Home: home, UsageURL: srv.URL}).RateLimits(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "expired") || strings.Contains(err.Error(), "segredo") {
		t.Fatalf("erro de expiração = %v", err)
	}
}

func TestAuthModes(t *testing.T) {
	home := t.TempDir()
	writeLines(t, filepath.Join(home, ".claude", ".credentials.json"),
		`{"claudeAiOauth":{"accessToken":"x","subscriptionType":"max","rateLimitTier":"default_max_20x"}}`)
	if mode, detail := (&Claude{Home: home}).AuthMode(); mode != AuthSubscription || detail != "max · default_max_20x" {
		t.Errorf("claude = %v %q", mode, detail)
	}
	if mode, _ := (&Claude{Home: t.TempDir()}).AuthMode(); mode != AuthUnknown {
		t.Errorf("sem credencial = %v", mode)
	}
	codexHome := t.TempDir()
	writeLines(t, filepath.Join(codexHome, ".codex", "auth.json"),
		`{"auth_mode":"chatgpt","OPENAI_API_KEY":null,"tokens":{"access_token":"segredo"}}`)
	if mode, detail := (&Codex{Home: codexHome}).AuthMode(); mode != AuthSubscription || detail != "ChatGPT" {
		t.Errorf("codex assinatura = %v %q", mode, detail)
	}
	keyHome := t.TempDir()
	writeLines(t, filepath.Join(keyHome, ".codex", "auth.json"), `{"OPENAI_API_KEY":"sk-segredo"}`)
	if mode, _ := (&Codex{Home: keyHome}).AuthMode(); mode != AuthAPIKey {
		t.Errorf("codex api key = %v", mode)
	}
	if AuthSubscription.String() != "subscription" || AuthAPIKey.String() != "API key" {
		t.Error("rótulos de AuthMode")
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
