package agent

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
)

// Entry é uma mensagem de transcript normalizada entre plataformas.
type Entry struct {
	Role string // "user" ou "assistant"
	Text string
}

const (
	maxTranscriptEntries = 500
	maxEntryRunes        = 8000
)

// entryFromLine tenta extrair uma mensagem user/assistant de uma linha JSONL
// de formato desconhecido. Linha inválida ou sem texto → ok=false (o parse é
// resiliente: nunca derruba o transcript por causa de uma linha).
func entryFromLine(line []byte) (Entry, bool) {
	var m map[string]any
	if err := json.Unmarshal(line, &m); err != nil {
		return Entry{}, false
	}
	return transcriptEntry(m)
}

func transcriptEntry(m map[string]any) (Entry, bool) {
	if meta, _ := m["isMeta"].(bool); meta {
		return Entry{}, false
	}
	role, _ := m["role"].(string)
	if role == "" {
		role, _ = m["type"].(string)
	}
	switch role {
	case "user":
	case "assistant", "model", "gemini":
		role = "assistant"
	default:
		// invólucros: {"type":"response_item","payload":{...}} (codex) etc.
		for _, k := range []string{"message", "payload"} {
			if inner, ok := m[k].(map[string]any); ok {
				if e, ok := transcriptEntry(inner); ok {
					return e, true
				}
			}
		}
		return Entry{}, false
	}
	var text string
	for _, k := range []string{"content", "parts", "text"} {
		if t := extractAnyText(m[k]); t != "" {
			text = t
			break
		}
	}
	if text == "" {
		// claude: {"type":"user","message":{"role":"user","content":...}}
		for _, k := range []string{"message", "payload"} {
			if inner, ok := m[k].(map[string]any); ok {
				if e, ok := transcriptEntry(inner); ok {
					return e, true
				}
			}
		}
		return Entry{}, false
	}
	text = strings.TrimSpace(text)
	// tags de harness ("<local-command…>", "<user_instructions>") não são
	// conversa — ficam fora do transcript
	if text == "" || strings.HasPrefix(text, "<") {
		return Entry{}, false
	}
	if r := []rune(text); len(r) > maxEntryRunes {
		text = string(r[:maxEntryRunes]) + "\n[… mensagem truncada]"
	}
	return Entry{Role: role, Text: text}, true
}

// jsonlTranscript lê um transcript em JSONL (claude, codex, gemini). Linhas
// ilegíveis são puladas; para em maxTranscriptEntries mensagens.
func jsonlTranscript(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), maxLineBuf)
	var out []Entry
	for sc.Scan() && len(out) < maxTranscriptEntries {
		line := sc.Bytes()
		if e, ok := entryFromLine(line); ok {
			out = append(out, e)
			continue
		}
		// gemini embrulha lotes de mensagens em updates {"$set":{"messages":[…]}}
		var set struct {
			Set struct {
				Messages []json.RawMessage `json:"messages"`
			} `json:"$set"`
		}
		if json.Unmarshal(line, &set) == nil {
			for _, raw := range set.Set.Messages {
				if e, ok := entryFromLine(raw); ok && len(out) < maxTranscriptEntries {
					out = append(out, e)
				}
			}
		}
	}
	return out, nil
}
