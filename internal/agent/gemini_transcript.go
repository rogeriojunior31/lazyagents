package agent

import (
	"bufio"
	"encoding/json"
	"os"
	"time"
)

func geminiMetadataLine(e *indexEntry, line []byte) {
	var m struct {
		SessionID   string          `json:"sessionId"`
		StartTime   string          `json:"startTime"`
		LastUpdated string          `json:"lastUpdated"`
		Set         json.RawMessage `json:"$set"`
	}
	if json.Unmarshal(line, &m) != nil {
		return
	}
	if m.SessionID != "" {
		e.GeminiSessionID = m.SessionID
	}
	if at, err := time.Parse(time.RFC3339Nano, m.StartTime); err == nil {
		e.GeminiStarted = at
	}
	if at, err := time.Parse(time.RFC3339Nano, m.LastUpdated); err == nil {
		e.GeminiUpdated = at
	}
	if e.FirstPrompt == "" {
		e.FirstPrompt = cleanTitle(looseUserText(line), 80)
	}
	if len(m.Set) > 0 {
		geminiMetadataLine(e, m.Set)
	}
}

// Gemini appends complete message snapshots under the same durable ID.
// Keep their original order, but render only the most recent snapshot.
func geminiTranscript(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), maxLineBuf)
	var messages []map[string]any
	ids := map[string]int{}
	add := func(m map[string]any) {
		if calls, ok := m["toolCalls"].([]any); ok {
			for _, raw := range calls {
				if call, ok := raw.(map[string]any); ok {
					delete(call, "result")
					delete(call, "resultDisplay")
				}
			}
		}
		id, _ := m["id"].(string)
		if i, ok := ids[id]; id != "" && ok {
			messages[i] = m
			return
		}
		if len(messages) >= maxTranscriptEntries {
			return
		}
		if id != "" {
			ids[id] = len(messages)
		}
		messages = append(messages, m)
	}
	reindex := func() {
		ids = map[string]int{}
		for i, msg := range messages {
			if id, _ := msg["id"].(string); id != "" {
				ids[id] = i
			}
		}
	}
	for sc.Scan() {
		var m map[string]any
		if json.Unmarshal(sc.Bytes(), &m) != nil {
			continue
		}
		if id, ok := m["$rewindTo"].(string); ok {
			if i, found := ids[id]; found {
				messages = messages[:i]
			} else {
				messages = nil
			}
			reindex()
			continue
		}
		if patch, ok := m["$patch"].(map[string]any); ok {
			apply := func(p map[string]any) {
				id, _ := p["id"].(string)
				if i, ok := ids[id]; ok {
					if content, ok := p["content"]; ok {
						messages[i]["content"] = content
					}
				}
			}
			apply(patch)
			if updates, ok := patch["updates"].([]any); ok {
				for _, raw := range updates {
					if p, ok := raw.(map[string]any); ok {
						apply(p)
					}
				}
			}
			removed := map[string]bool{}
			if remove, ok := patch["removeIds"].([]any); ok {
				for _, raw := range remove {
					if id, ok := raw.(string); ok {
						removed[id] = true
					}
				}
			}
			var keep []map[string]any
			for _, msg := range messages {
				if id, _ := msg["id"].(string); !removed[id] {
					keep = append(keep, msg)
				}
			}
			messages = keep
			reindex()
			if order, ok := patch["orderIds"].([]any); ok {
				selected := map[string]bool{}
				var ordered []map[string]any
				for _, raw := range order {
					id, _ := raw.(string)
					if i, ok := ids[id]; ok && !selected[id] {
						ordered = append(ordered, messages[i])
						selected[id] = true
					}
				}
				var prefix []map[string]any
				for _, msg := range messages {
					if id, _ := msg["id"].(string); !selected[id] {
						prefix = append(prefix, msg)
					}
				}
				messages = append(prefix, ordered...)
				reindex()
			}
			continue
		}
		if set, ok := m["$set"].(map[string]any); ok {
			if batch, ok := set["messages"].([]any); ok {
				messages, ids = nil, map[string]int{}
				for _, v := range batch {
					if msg, ok := v.(map[string]any); ok {
						add(msg)
					}
				}
			}
			continue
		}
		// Metadata has no role and must not consume the message limit.
		if m["type"] != nil || m["role"] != nil {
			add(m)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	var out []Entry
	for _, m := range messages {
		at := entryTime(m)
		for _, e := range transcriptEntries(m) {
			if e.Time.IsZero() {
				e.Time = at
			}
			out = append(out, e)
		}
		if m["type"] != "gemini" {
			continue
		}
		tools, _ := m["toolCalls"].([]any)
		for _, raw := range tools {
			call, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			e, ok := toolEntry(map[string]any{"name": call["name"], "input": call["args"]})
			if !ok {
				continue
			}
			e.Time = entryTime(call)
			if e.Time.IsZero() {
				e.Time = at
			}
			e.Failed = call["status"] == "error"
			out = append(out, e)
		}
	}
	return out[:min(len(out), maxTranscriptEntries)], nil
}
