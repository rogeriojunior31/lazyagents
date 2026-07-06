package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode"
)

// maxLineBuf amplia o buffer do scanner: respostas longas do assistente podem
// passar de 1 MB numa única linha de JSONL.
const maxLineBuf = 4 * 1024 * 1024

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// version roda `bin --version` com timeout curto e devolve a primeira linha.
func version(bin string, args ...string) string {
	if bin == "" {
		return ""
	}
	if len(args) == 0 {
		args = []string{"--version"}
	}
	// CLIs em Node/Bun (gemini, opencode) levam >2s só pra imprimir a versão
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, args...).Output()
	if err != nil {
		return ""
	}
	line, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	if len(line) > 40 {
		line = line[:40]
	}
	return line
}

// firstLines lê até max linhas de um arquivo (JSONL). Erros viram lista vazia:
// listagem de sessões é best-effort e nunca derruba a TUI.
func firstLines(path string, max int) [][]byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), maxLineBuf)
	var out [][]byte
	for len(out) < max && sc.Scan() {
		line := make([]byte, len(sc.Bytes()))
		copy(line, sc.Bytes())
		out = append(out, line)
	}
	return out
}

// looseUserText tenta extrair o texto de uma mensagem de usuário a partir de
// uma linha JSONL de formato desconhecido. Cobre os formatos observados:
// {"role":"user","content":...}, {"type":"user","message":{...}},
// {"role":"user","parts":[{"text":...}]} e payloads aninhados do Codex.
func looseUserText(line []byte) string {
	var m map[string]any
	if err := json.Unmarshal(line, &m); err != nil {
		return ""
	}
	return userTextFromMap(m)
}

func userTextFromMap(m map[string]any) string {
	if meta, _ := m["isMeta"].(bool); meta {
		return ""
	}
	role, _ := m["role"].(string)
	typ, _ := m["type"].(string)
	if role == "user" || typ == "user" {
		for _, k := range []string{"content", "parts", "text", "display"} {
			if t := extractAnyText(m[k]); t != "" {
				return t
			}
		}
	}
	// desce em invólucros comuns: message (Claude), payload (Codex)
	for _, k := range []string{"message", "payload"} {
		if inner, ok := m[k].(map[string]any); ok {
			if t := userTextFromMap(inner); t != "" {
				return t
			}
		}
	}
	return ""
}

// extractAnyText obtém texto de content em qualquer formato: string direta,
// lista de blocos {"type":"text"/"input_text","text":...} ou {"text":...}.
func extractAnyText(v any) string {
	switch c := v.(type) {
	case string:
		return c
	case []any:
		for _, b := range c {
			if t := extractAnyText(b); t != "" {
				return t
			}
		}
	case map[string]any:
		if t, ok := c["text"].(string); ok {
			return t
		}
	}
	return ""
}

// cleanTitle normaliza um prompt para virar título de lista: colapsa espaços,
// descarta tags de harness ("<local-command...>") e corta em max runes.
func cleanTitle(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "<") {
		return ""
	}
	s = strings.Join(strings.FieldsFunc(s, unicode.IsSpace), " ")
	r := []rune(s)
	if len(r) > max {
		return string(r[:max-1]) + "…"
	}
	return s
}
