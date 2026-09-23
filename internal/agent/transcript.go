package agent

import (
	"bufio"
	"encoding/json"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Entry é uma mensagem de transcript normalizada entre plataformas.
type Entry struct {
	Role string // RoleUser, RoleAssistant ou RoleTool
	Text string // mensagem; para RoleTool, "Nome · argumento principal"
}

// Papéis de Entry. RoleTool é uma chamada de ferramenta do agente, já
// resumida numa linha — o resultado dela não entra (é volume, não conversa).
// RoleThinking é o raciocínio que o agente gravou em texto: Claude Code
// ("thinking"), Codex (resumo do "reasoning"), Gemini ("thoughts") e
// OpenCode (parte "reasoning"). Raciocínio criptografado ou vazio não entra.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
	RoleThinking  = "thinking"
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
	case "reasoning":
		// codex: {"type":"response_item","payload":{"type":"reasoning",
		// "summary":[{"type":"summary_text","text":…}],"content":[…]}}; só o
		// encrypted_content = nada legível
		return thinkingEntry(append(textsOf(m["summary"]), textsOf(m["content"])...))
	default:
		// invólucros: {"type":"response_item","payload":{...}} (codex) etc.
		return unwrap(m)
	}

	var parts []Entry
	if role == RoleAssistant {
		parts = append(parts, geminiThoughts(m["thoughts"])...)
	}
	for _, k := range []string{"content", "parts", "text"} {
		if ps := contentParts(m[k]); len(ps) > 0 {
			parts = append(parts, ps...)
			break
		}
	}
	if len(parts) == 0 {
		// claude: {"type":"user","message":{"role":"user","content":...}}
		return unwrap(m)
	}
	var out []Entry
	for _, e := range mergeTexts(parts) {
		switch {
		case e.Role == RoleAssistant: // texto: vira o papel da mensagem
			e.Text = strings.TrimSpace(e.Text)
			// tags de harness ("<local-command…>", "<user_instructions>") não
			// são conversa — ficam fora do transcript
			if e.Text == "" || strings.HasPrefix(e.Text, "<") {
				continue
			}
			e.Role, e.Text = role, capRunes(e.Text)
		case role != RoleAssistant:
			continue // do usuário só entra o texto
		}
		out = append(out, e)
	}
	if len(out) == 0 {
		return unwrap(m)
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

// contentParts lê o conteúdo de uma mensagem, na ordem: texto (com papel
// RoleAssistant provisório), raciocínio e chamadas de ferramenta. Aceita
// string, bloco único ou lista de blocos; resultados de ferramenta ficam de
// fora.
func contentParts(v any) []Entry {
	switch c := v.(type) {
	case string:
		if strings.TrimSpace(c) != "" {
			return []Entry{{Role: RoleAssistant, Text: c}}
		}
	case []any:
		var out []Entry
		for _, b := range c {
			out = append(out, contentParts(b)...)
		}
		return out
	case map[string]any:
		switch c["type"] {
		case "tool_use", "function_call", "custom_tool_call":
			if e, ok := toolEntry(c); ok {
				return []Entry{e}
			}
		case "thinking": // claude; vazio nas versões que não gravam o texto
			t, _ := c["thinking"].(string)
			return thinkingEntry([]string{t})
		case "reasoning": // opencode
			t, _ := c["text"].(string)
			return thinkingEntry([]string{t})
		case "redacted_thinking", "tool_result":
		default:
			if t, ok := c["text"].(string); ok && strings.TrimSpace(t) != "" {
				return []Entry{{Role: RoleAssistant, Text: t}}
			}
		}
	}
	return nil
}

// mergeTexts junta blocos de texto vizinhos numa mensagem só.
func mergeTexts(parts []Entry) []Entry {
	var out []Entry
	for _, e := range parts {
		if n := len(out); n > 0 && e.Role == RoleAssistant && out[n-1].Role == RoleAssistant {
			out[n-1].Text += "\n\n" + e.Text
			continue
		}
		out = append(out, e)
	}
	return out
}

// thinkingEntry vira os textos de raciocínio numa entrada (nada se vazios).
func thinkingEntry(texts []string) []Entry {
	var keep []string
	for _, t := range texts {
		if t = strings.TrimSpace(t); t != "" {
			keep = append(keep, t)
		}
	}
	if len(keep) == 0 {
		return nil
	}
	return []Entry{{Role: RoleThinking, Text: capRunes(strings.Join(keep, "\n\n"))}}
}

// textsOf extrai o "text" de uma lista de blocos (resumo do codex).
func textsOf(v any) []string {
	list, _ := v.([]any)
	var out []string
	for _, b := range list {
		if m, ok := b.(map[string]any); ok {
			if t, ok := m["text"].(string); ok {
				out = append(out, t)
			}
		}
	}
	return out
}

// geminiThoughts lê {"thoughts":[{"subject":…,"description":…}]}.
func geminiThoughts(v any) []Entry {
	list, _ := v.([]any)
	var texts []string
	for _, b := range list {
		m, ok := b.(map[string]any)
		if !ok {
			continue
		}
		subject, _ := m["subject"].(string)
		desc, _ := m["description"].(string)
		switch {
		case subject != "" && desc != "":
			texts = append(texts, "**"+subject+"** "+desc)
		default:
			texts = append(texts, subject+desc)
		}
	}
	return thinkingEntry(texts)
}

func capRunes(text string) string {
	if r := []rune(text); len(r) > maxEntryRunes {
		return string(r[:maxEntryRunes]) + "\n[… mensagem truncada]"
	}
	return text
}

// execCmdRe acha o cmd de um exec_command do codex dentro do código JS.
var execCmdRe = regexp.MustCompile(`\bcmd"?\s*:\s*"((?:[^"\\]|\\.)*)"`)

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
		// codex "exec": o argumento é código ({cmd:"…"}); o comando é o que
		// interessa ler
		if sub := execCmdRe.FindStringSubmatch(a); sub != nil {
			if cmd, err := strconv.Unquote(`"` + sub[1] + `"`); err == nil {
				arg = cmd
			}
		}
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
