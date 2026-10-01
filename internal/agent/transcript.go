package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Entry is a transcript message, normalized across agents.
type Entry struct {
	Role   string    // RoleUser, RoleAssistant, RoleTool, RoleThinking or RoleEvent
	Text   string    // message; for RoleTool, "Name · main argument"
	Time   time.Time `json:",omitzero"` // when the agent recorded it; zero if it does not record one
	Failed bool      `json:",omitzero"` // RoleTool: the call returned an error

	// RoleTool: what the call did (ToolShell, ToolEdit…; "" unknown), lines an
	// edit added and removed, and the text of a plan or task list.
	Kind    string `json:",omitzero"`
	Added   int    `json:",omitzero"`
	Removed int    `json:",omitzero"`
	Body    string `json:",omitzero"`
}

// Entry roles. RoleTool is an agent tool call summarized in one line (its
// result is volume, not conversation, and is left out). RoleThinking is
// reasoning the agent saved as text: Claude Code "thinking", Codex "reasoning"
// summary, Gemini "thoughts", OpenCode "reasoning" part. Encrypted or empty
// reasoning is left out. RoleEvent is something that happened to the session
// rather than something said: compaction, interruption, model change.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
	RoleThinking  = "thinking"
	RoleEvent     = "event"
)

// Event texts, shown as they are.
const (
	eventCompacted   = "context compacted"
	eventInterrupted = "interrupted by the user"
	eventModel       = "model changed to %s"
)

// slashCmdRe reads a Claude Code slash command saved as a user message:
// "<command-name>/plan</command-name>…<command-args>do X</command-args>".
var slashCmdRe = regexp.MustCompile(`(?s)^<command-name>(/[^<]+)</command-name>(?:.*?<command-args>(.*?)</command-args>)?`)

const (
	maxTranscriptEntries = 2000 // counts tool calls, which are many
	maxEntryRunes        = 8000
	maxToolRunes         = 160
)

// entryFromLine returns the first message of a JSONL line, for callers that
// want one (OpenCode stores one message per record).
func entryFromLine(line []byte) (Entry, bool) {
	if es := entriesFromLine(line); len(es) > 0 {
		return es[0], true
	}
	return Entry{}, false
}

// entriesFromLine extracts the messages of a JSONL line of unknown shape: text
// and each tool call. An invalid or empty line gives nothing; one bad line
// never breaks the transcript.
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
	if compact, _ := m["isCompactSummary"].(bool); compact {
		// claude: the summary that replaces the history is not a prompt
		return []Entry{{Role: RoleEvent, Text: eventCompacted}}
	}
	role, _ := m["role"].(string)
	if role == "" {
		role, _ = m["type"].(string)
	}
	switch role {
	case "compacted": // codex
		return []Entry{{Role: RoleEvent, Text: eventCompacted}}
	case "turn_aborted": // codex: {"type":"event_msg","payload":{"type":"turn_aborted"}}
		return []Entry{{Role: RoleEvent, Text: eventInterrupted}}
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
		// "summary":[{"type":"summary_text","text":…}],"content":[…]}}; only
		// encrypted_content = nothing readable
		return thinkingEntry(append(textsOf(m["summary"]), textsOf(m["content"])...))
	default:
		// envelopes: {"type":"response_item","payload":{...}} (codex) etc.
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
		case e.Role == RoleAssistant: // text: becomes the message role
			e.Text = strings.TrimSpace(e.Text)
			if role == "user" {
				if ev, ok := userMarker(e.Text); ok {
					out = append(out, ev)
					continue
				}
			}
			// harness tags ("<local-command…>", "<user_instructions>") are not conversation
			if e.Text == "" || strings.HasPrefix(e.Text, "<") {
				continue
			}
			e.Role, e.Text = role, capRunes(e.Text)
		case role != RoleAssistant:
			continue // from the user, only text
		}
		out = append(out, e)
	}
	if len(out) == 0 {
		return unwrap(m)
	}
	return out
}

// userMarker turns what Claude Code saves as user text but the user did not
// write as a prompt: an interruption becomes an event, a slash command the
// command line the user typed.
func userMarker(text string) (Entry, bool) {
	if strings.HasPrefix(text, "[Request interrupted by user") {
		return Entry{Role: RoleEvent, Text: eventInterrupted}, true
	}
	if sub := slashCmdRe.FindStringSubmatch(text); sub != nil {
		return Entry{Role: RoleUser, Text: strings.TrimSpace(sub[1] + " " + strings.TrimSpace(sub[2]))}, true
	}
	return Entry{}, false
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

// contentParts reads a message's content in order: text (with a provisional
// RoleAssistant), reasoning and tool calls. Accepts a string, one block or a
// list of blocks; tool results are left out.
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
		case "tool_use", "function_call", "custom_tool_call", "toolCall": // toolCall: pi
			if e, ok := toolEntry(c); ok {
				return []Entry{e}
			}
		case "thinking": // claude; empty in versions that do not save the text
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

// mergeTexts joins adjacent text blocks into one message.
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

// thinkingEntry turns reasoning texts into one entry (nothing if empty).
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

// textsOf extracts "text" from a list of blocks (codex summary).
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

// geminiThoughts reads {"thoughts":[{"subject":…,"description":…}]}.
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
		return string(r[:maxEntryRunes]) + "\n[… message truncated]"
	}
	return text
}

// execCmdRe finds the cmd of a codex exec_command inside JS code.
var execCmdRe = regexp.MustCompile(`\bcmd"?\s*:\s*"((?:[^"\\]|\\.)*)"`)

// toolArgKeys is the preference order of the argument that best says, in one
// line, what a tool call did.
var toolArgKeys = []string{"command", "cmd", "file_path", "path", "pattern", "query", "url", "description", "prompt"}

// toolEntry summarizes a tool call as "Name · argument".
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
	if raw, ok := args.(string); ok { // codex sends arguments as a JSON string
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
		// codex "exec": the argument is code ({cmd:"…"}); the command is what matters
		if sub := execCmdRe.FindStringSubmatch(a); sub != nil {
			if cmd, err := strconv.Unquote(`"` + sub[1] + `"`); err == nil {
				arg = cmd
			}
		}
	}
	e := Entry{Role: RoleTool}
	arg = classify(&e, name, args, arg)
	e.Body = capRunes(e.Body)
	text := name
	if arg = oneLine(arg); arg != "" {
		text += " · " + arg
	}
	if r := []rune(text); len(r) > maxToolRunes {
		text = string(r[:maxToolRunes-1]) + "…"
	}
	e.Text = text
	return e, true
}

// argText flattens an argument: a list (codex argv) becomes a command line.
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

// oneLine collapses whitespace so a call fits one line in the reader.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// jsonlTranscript reads a JSONL transcript (claude, codex, gemini). Unreadable
// lines are skipped; stops at maxTranscriptEntries messages. What needs more
// than one line (a failed call, a model change) is resolved here.
func jsonlTranscript(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), maxLineBuf)
	var out []Entry
	calls := map[string]int{} // tool_use id → index in out
	model := ""
	add := func(m map[string]any) {
		at := entryTime(m)
		if next := lineModel(m); next != "" {
			if model != "" && next != model {
				out = append(out, Entry{Role: RoleEvent, Text: fmt.Sprintf(eventModel, next), Time: at})
			}
			model = next
		}
		ids := toolIDs(m)
		for _, e := range transcriptEntries(m) {
			if e.Time.IsZero() {
				e.Time = at
			}
			if e.Role == RoleTool && len(ids) > 0 {
				calls[ids[0]], ids = len(out), ids[1:]
			}
			out = append(out, e)
		}
		for _, id := range failedIDs(m) {
			if i, ok := calls[id]; ok {
				out[i].Failed = true
			}
		}
	}
	for sc.Scan() && len(out) < maxTranscriptEntries {
		var m map[string]any
		if json.Unmarshal(sc.Bytes(), &m) != nil {
			continue
		}
		// gemini wraps message batches in updates {"$set":{"messages":[…]}}
		if set, ok := m["$set"].(map[string]any); ok {
			msgs, _ := set["messages"].([]any)
			for _, raw := range msgs {
				if msg, ok := raw.(map[string]any); ok && len(out) < maxTranscriptEntries {
					add(msg)
				}
			}
			continue
		}
		add(m)
	}
	return out, nil
}

// entryTime reads the record's "timestamp" (claude, codex, gemini).
func entryTime(m map[string]any) time.Time {
	s, _ := m["timestamp"].(string)
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

// lineModel is the model a record says is answering: claude assistant
// messages, codex turn_context. "<synthetic>" marks claude's own error replies.
func lineModel(m map[string]any) string {
	var model string
	switch m["type"] {
	case "assistant":
		msg, _ := m["message"].(map[string]any)
		model, _ = msg["model"].(string)
	case "turn_context":
		p, _ := m["payload"].(map[string]any)
		model, _ = p["model"].(string)
	}
	if model == "<synthetic>" {
		return ""
	}
	return model
}

// contentBlocks is a claude message's content list.
func contentBlocks(m map[string]any) []map[string]any {
	msg, _ := m["message"].(map[string]any)
	list, _ := msg["content"].([]any)
	var out []map[string]any
	for _, b := range list {
		if bm, ok := b.(map[string]any); ok {
			out = append(out, bm)
		}
	}
	return out
}

// toolIDs are the ids of a claude message's tool calls, in the order
// contentParts emits them.
func toolIDs(m map[string]any) []string {
	var ids []string
	for _, b := range contentBlocks(m) {
		if id, _ := b["id"].(string); id != "" && b["type"] == "tool_use" {
			ids = append(ids, id)
		}
	}
	return ids
}

// failedIDs are the calls a claude tool_result reports as errors.
func failedIDs(m map[string]any) []string {
	var ids []string
	for _, b := range contentBlocks(m) {
		if failed, _ := b["is_error"].(bool); failed && b["type"] == "tool_result" {
			if id, _ := b["tool_use_id"].(string); id != "" {
				ids = append(ids, id)
			}
		}
	}
	return ids
}
