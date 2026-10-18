package agent

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

type geminiUsageResponse struct {
	At    time.Time
	Usage Usage
	Valid bool
}

func (g *Gemini) index() *Index {
	g.indexOnce.Do(func() {
		if g.Index == nil {
			g.Index = NewIndex("")
		}
	})
	return g.Index
}

func geminiIndexLine(e *indexEntry, line []byte) {
	geminiMetadataLine(e, line)
	var msg struct {
		Type, ID, Model, Timestamp string
		Tokens                     json.RawMessage
	}
	decodeErr := json.Unmarshal(line, &msg)
	if msg.Type != "gemini" {
		return
	}
	if e.GeminiResponses == nil {
		e.GeminiResponses = map[string]geminiUsageResponse{}
	}
	at, err := time.Parse(time.RFC3339Nano, msg.Timestamp)
	r := geminiUsageResponse{At: at, Usage: Usage{Model: msg.Model, Incomplete: true}, Valid: decodeErr == nil && err == nil && msg.ID != "" && msg.Model != ""}
	var n struct{ Input, Output, Cached, Thoughts, Tool, Total *int }
	if json.Unmarshal(msg.Tokens, &n) == nil {
		counts := []*int{n.Input, n.Output, n.Cached, n.Thoughts, n.Tool, n.Total}
		valid := true
		for _, v := range counts {
			if v == nil || *v < 0 {
				valid = false
			}
		}
		if valid && *n.Cached <= *n.Input {
			// Gemini records cached input inside prompt tokens; thoughts are separate from candidates.
			r.Usage = Usage{Input: *n.Input - *n.Cached, CacheRead: *n.Cached, Output: *n.Output + *n.Thoughts, Model: msg.Model}
			// Tool tokens have no verified price mapping here. Keep them unavailable instead of guessing.
			r.Usage.Incomplete = *n.Tool != 0 || *n.Total != *n.Input+*n.Output+*n.Thoughts
		}
	}
	e.GeminiResponses[msg.ID] = r
}

// Rebuild only after an append, replacing repeated snapshots by durable message ID.
func (e *indexEntry) rebuildGeminiUsage() {
	e.Events, e.Models = nil, nil
	responses := make([]geminiUsageResponse, 0, len(e.GeminiResponses))
	for _, r := range e.GeminiResponses {
		if r.Valid {
			responses = append(responses, r)
		}
	}
	sort.Slice(responses, func(i, j int) bool {
		if responses[i].At.Equal(responses[j].At) {
			return responses[i].Usage.Model < responses[j].Usage.Model
		}
		return responses[i].At.Before(responses[j].At)
	})
	for _, r := range responses {
		e.addEvent(&e.Events, r.At, r.Usage.Model, "", r.Usage)
	}
}
func (g *Gemini) UsageEvents(s Session) ([]UsageEvent, error) {
	e, err := g.index().refresh(s.Path, geminiIndexLine)
	if err != nil {
		return nil, fmt.Errorf("reading Gemini usage: %w", err)
	}
	for _, r := range e.GeminiResponses {
		if !r.Valid {
			return nil, fmt.Errorf("Gemini usage contains a response without a valid ID, timestamp or model")
		}
	}
	return e.events(e.Events, s), nil
}
func (g *Gemini) SessionUsage(s Session) (Usage, bool) {
	events, err := g.UsageEvents(s)
	if err != nil || len(events) == 0 {
		return Usage{}, false
	}
	return sumRecordedUsage(events), true
}
func sumRecordedUsage(events []UsageEvent) Usage {
	sum := Usage{CostKnown: len(events) > 0}
	for _, e := range events {
		sum.Input += e.Usage.Input
		sum.Output += e.Usage.Output
		sum.CacheRead += e.Usage.CacheRead
		sum.CacheWrite += e.Usage.CacheWrite
		sum.CacheWrite1h += e.Usage.CacheWrite1h
		sum.Cost += e.Usage.Cost
		sum.CostKnown = sum.CostKnown && e.Usage.CostKnown
		sum.Incomplete = sum.Incomplete || e.Usage.Incomplete
		if sum.Model == "" {
			sum.Model = e.Model
		} else if sum.Model != e.Model {
			sum.Model = "mixed"
		}
	}
	if !sum.CostKnown {
		sum.Cost = 0
	} // A known component is not the full session cost.
	return sum
}
