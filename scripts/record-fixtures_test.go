//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRecordingIsolationAndPreservation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture shell")
	}
	t.Setenv("FIXTURE_PARENT_SECRET", "must-not-inherit")
	bin := filepath.Join(t.TempDir(), "fixture-cli")
	if err := os.WriteFile(bin, []byte(`#!/bin/sh
set -eu
test -z "${FIXTURE_PARENT_SECRET:-}"
test "$PWD" != "$HOME"
test -d "$HOME"
test -n "$XDG_CONFIG_HOME"
if [ "$1" = --version ]; then
  printf '1.2.3\n'
  printf isolated > "$HOME/version-probe"
else
  test -f "$HOME/version-probe"
  printf '{"cwd":"%s","home":"%s","content":"fixture"}\n' "$PWD" "$HOME" > "$HOME/session.jsonl"
fi
`), 0o700); err != nil {
		t.Fatal(err)
	}
	rec := recorder{id: "fixture", bin: bin, setup: func(home, base string) error { return nil },
		run: func(r *run) error { return r.cli(bin, "record") }, collect: []string{"session.jsonl"}, notes: "test CLI"}
	out := t.TempDir()
	dest, err := record(rec, "http://127.0.0.1:1", out)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dest, "home", "session.jsonl")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(before, []byte("/work/proj")) || !bytes.Contains(before, []byte("/home/user")) {
		t.Fatalf("unsanitized session: %s", before)
	}
	var origin map[string]string
	raw, err := os.ReadFile(filepath.Join(dest, "origin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &origin); err != nil {
		t.Fatal(err)
	}
	if origin["os"] != runtime.GOOS || origin["arch"] != runtime.GOARCH || origin["version"] != "1.2.3" {
		t.Fatalf("origin: %v", origin)
	}
	if _, err := record(rec, "http://127.0.0.1:1", out); err == nil {
		t.Fatal("overwrote existing evidence")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("existing recording changed")
	}
	// Config inputs alone are never presented as a successful session recording.
	rec.collect, rec.config = []string{"missing.jsonl"}, []string{"version-probe"}
	if _, err := record(rec, "http://127.0.0.1:1", t.TempDir()); err == nil {
		t.Fatal("accepted missing session output")
	}
	rec.collect = []string{"session.jsonl"}
	rec.run = func(r *run) error { return write(filepath.Join(r.home, "session.jsonl"), "not JSON\n") }
	failedOut := t.TempDir()
	if _, err := record(rec, "http://127.0.0.1:1", failedOut); err == nil {
		t.Fatal("accepted malformed JSONL")
	}
	if _, err := os.Stat(filepath.Join(failedOut, "fixture", "1.2.3")); !os.IsNotExist(err) {
		t.Fatal("published partial recording")
	}
}

func TestGenerateContentToolRoundTrip(t *testing.T) {
	tool := map[string]any{"functionDeclarations": []any{map[string]any{"name": "run_shell_command", "parameters": map[string]any{"properties": map[string]any{"command": map[string]any{"type": "string"}}}}}}
	for _, streamed := range []bool{false, true} {
		for _, result := range []bool{false, true} {
			part := map[string]any{"text": prompt}
			if result {
				part = map[string]any{"functionResponse": map[string]any{"name": "run_shell_command", "response": map[string]any{"output": "fixture"}}}
			}
			body, _ := json.Marshal(map[string]any{"contents": []any{map[string]any{"role": "user", "parts": []any{part}}}, "tools": []any{tool}})
			endpoint := "/v1beta/models/gemini-2.5-flash:generateContent"
			if streamed {
				endpoint = "/v1beta/models/gemini-2.5-flash:streamGenerateContent"
			}
			w := httptest.NewRecorder()
			handle(w, httptest.NewRequest("POST", endpoint, bytes.NewReader(body)))
			raw := strings.TrimSpace(w.Body.String())
			if streamed {
				if w.Header().Get("Content-Type") != "text/event-stream" || strings.Contains(raw, "[DONE]") {
					t.Fatalf("invalid Google SSE: %s", raw)
				}
				raw = strings.TrimPrefix(raw, "data: ")
			}
			var response map[string]any
			if err := json.Unmarshal([]byte(raw), &response); err != nil {
				t.Fatal(err)
			}
			p := list(dig(list(response["candidates"])[0], "content", "parts"))[0]
			if result && dig(p, "text") != reply {
				t.Fatalf("reply: %v", p)
			}
			if !result && dig(p, "functionCall", "args", "command") != command {
				t.Fatalf("tool call: %v", p)
			}
		}
	}
}

func TestOpenCodeFixtureRejectsWrongCredential(t *testing.T) {
	openCodeProviderRoundtrip = true
	t.Cleanup(func() { openCodeProviderRoundtrip = false })
	for _, token := range []string{"", "Bearer wrong-fixture-key", "Bearer fixture-managed-key"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"managed-fixture-model-token","messages":[]}`))
		r.Header.Set("Authorization", token)
		handle(w, r)
		if token == "Bearer fixture-managed-key" {
			if w.Code != 200 {
				t.Fatal("correct fixture key rejected")
			}
		} else if w.Code != 401 {
			t.Fatal("unauthenticated fixture accepted")
		}
	}
}

func TestGeminiProviderRecordingAuthenticatesNativeRequests(t *testing.T) {
	geminiProviderRoundtrip = true
	t.Cleanup(func() { geminiProviderRoundtrip = false })
	for _, key := range []string{"", "wrong-key", "fixture-managed-key"} {
		req := httptest.NewRequest("POST", "/v1beta/models/gemini-2.5-pro:generateContent", strings.NewReader(`{"contents":[]}`))
		req.Header.Set("x-goog-api-key", key)
		w := httptest.NewRecorder()
		handle(w, req)
		if key == "fixture-managed-key" && w.Code != 200 || key != "fixture-managed-key" && w.Code != 401 {
			t.Fatalf("credential %q: status %d", key, w.Code)
		}
	}
}
