package agent

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
)

// Entry é uma mensagem de transcript normalizada entre plataformas.
type Entry struct {
	Role string // RoleUser, RoleAssistant ou RoleTool
	Text string // mensagem; para RoleTool, "Nome · argumento principal"
}

// Papéis de Entry. RoleTool é uma chamada de ferramenta do agente, já
// resumida numa linha — o resultado dela não entra (é volume, não conversa).
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

const (
	maxTranscriptEntries = 2000 // conta chamadas de ferramenta, que são muitas
	maxEntryRunes        = 8000
	maxToolRunes         = 160
)

// entryFromLine devolve a primeira mensagem de uma linha JSONL (quem só quer
// uma, como o OpenCode, que guarda uma mensagem por registro).
func entryFromLine(line []byte) (Entry, bool) {
	if es := entriesFromLine(line); len(es) > 0 {
		return es[0], true
	}
	return Entry{}, false
}

// entriesFromLine extrai as mensagens de uma linha JSONL de formato
// desconhecido: o texto e cada chamada de ferramenta. Linha inválida ou sem
// nada → vazio (o parse é resiliente: nunca derruba o transcript por causa
// de uma linha).
func entriesFromLine(line []byte) []Entry {
	var m map[string]any
	if err := json.Unmarshal(line, &m); err != nil {
		return nil
	}
	return transcriptEntries(m)
}

func transcriptEntries(m map[string]any) []Entry {
	if meta, _ := m["isMeta"].(bool); meta {
		return nil
	}
	role, _ := m["role"].(string)
	if role == "" {
		role, _ = m["type"].(string)
	}
	switch role {
	case "user":
	case "assistant", "model", "gemini":
		role = RoleAssistant
	case "function_call", "custom_tool_call", "local_shell_call":
		// codex: {"type":"response_item","payload":{"type":"function_call",…}}
		if e, ok := toolEntry(m); ok {
			return []Entry{e}
		}
		return nil
	default:
		// invólucros: {"type":"response_item","payload":{...}} (codex) etc.
		return unwrap(m)
	}

	var texts []string
	var tools []Entry
	for _, k := range []string{"content", "parts", "text"} {
		t, calls := contentParts(m[k])
		if len(t) > 0 || len(calls) > 0 {
			texts, tools = t, calls
			break
		}
	}
	if len(texts) == 0 && len(tools) == 0 {
		// claude: {"type":"user","message":{"role":"user","content":...}}
		return unwrap(m)
	}
	var out []Entry
	text := strings.TrimSpace(strings.Join(texts, "\n\n"))
	// tags de harness ("<local-command…>", "<user_instructions>") não são
	// conversa — ficam fora do transcript
	if text != "" && !strings.HasPrefix(text, "<") {
		if r := []rune(text); len(r) > maxEntryRunes {
			text = string(r[:maxEntryRunes]) + "\n[… mensagem truncada]"
		}
		out = append(out, Entry{Role: role, Text: text})
	}
	if role == RoleAssistant {
		out = append(out, tools...)
	}
	return out
}

func unwrap(m map[string]any) []Entry {
	for _, k := range []string{"message", "payload"} {
		if inner, ok := m[k].(map[string]any); ok {
			if es := transcriptEntries(inner); len(es) > 0 {
				return es
			}
		}
	}
	return nil
}

// contentParts separa o conteúdo de uma mensagem em textos e chamadas de
// ferramenta. Aceita string, bloco único ou lista de blocos; "thinking" e
// resultados de ferramenta ficam de fora.
func contentParts(v any) (texts []string, tools []Entry) {
	switch c := v.(type) {
	case string:
		if strings.TrimSpace(c) != "" {
			texts = append(texts, c)
		}
	case []any:
		for _, b := range c {
			t, calls := contentParts(b)
			texts, tools = append(texts, t...), append(tools, calls...)
		}
	case map[string]any:
		switch c["type"] {
		case "tool_use", "function_call", "custom_tool_call":
			if e, ok := toolEntry(c); ok {
				tools = append(tools, e)
			}
		case "thinking", "redacted_thinking", "tool_result", "reasoning":
		default:
			if t, ok := c["text"].(string); ok && strings.TrimSpace(t) != "" {
				texts = append(texts, t)
			}
		}
	}
	return texts, tools
}

// toolArgKeys é a ordem de preferência do argumento que resume a chamada:
// o que diz, numa linha, o que a ferramenta fez.
var toolArgKeys = []string{"command", "cmd", "file_path", "path", "pattern", "query", "url", "description", "prompt"}

// toolEntry resume uma chamada de ferramenta em "Nome · argumento".
func toolEntry(m map[string]any) (Entry, bool) {
	name, _ := m["name"].(string)
	if name == "" {
		name, _ = m["type"].(string)
	}
	if name == "" {
		return Entry{}, false
	}
	var args any = m["input"]
	if args == nil {
		args = m["arguments"]
	}
	if raw, ok := args.(string); ok { // codex manda os argumentos como JSON em string
		var parsed map[string]any
		if json.Unmarshal([]byte(raw), &parsed) == nil {
			args = parsed
		}
	}
	arg := ""
	switch a := args.(type) {
	case map[string]any:
		for _, k := range toolArgKeys {
			if v, ok := a[k]; ok {
				arg = argText(v)
				break
			}
		}
	case string:
		arg = a
	}
	text := name
	if arg = oneLine(arg); arg != "" {
		text += " · " + arg
	}
	if r := []rune(text); len(r) > maxToolRunes {
		text = string(r[:maxToolRunes-1]) + "…"
	}
	return Entry{Role: RoleTool, Text: text}, true
}

// argText achata um argumento: lista (argv do codex) vira linha de comando.
func argText(v any) string {
	switch a := v.(type) {
	case string:
		return a
	case []any:
		parts := make([]string, 0, len(a))
		for _, p := range a {
			if s, ok := p.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, " ")
	}
	return ""
}

// oneLine colapsa espaços e quebras: a chamada ocupa uma linha no leitor.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

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
		if es := entriesFromLine(line); len(es) > 0 {
			out = append(out, es...)
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
				if len(out) < maxTranscriptEntries {
					out = append(out, entriesFromLine(raw)...)
				}
			}
		}
	}
	return out, nil
}
