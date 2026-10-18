package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func geminiUsageLine(id, model, at string, input, output, cached, thoughts int) string {
	data, _ := json.Marshal(map[string]any{"type": "gemini", "id": id, "model": model, "timestamp": at, "tokens": map[string]int{"input": input, "output": output, "cached": cached, "thoughts": thoughts, "tool": 0, "total": input + output + thoughts}})
	return string(data)
}
func TestGeminiUsageSnapshotsSurviveAppendRestartAndRewrite(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "session.jsonl")
	indexPath := filepath.Join(home, "index.gob")
	first := geminiUsageLine("one", "gemini-a", "2026-10-18T10:00:00Z", 100, 8, 20, 2)
	writeLines(t, path, first, first)
	g := NewGemini(home)
	g.Index = NewIndex(indexPath)
	s := Session{Path: path, CWD: "/recorded-project"}
	assert := func(in, out, cache, n int) {
		t.Helper()
		events, err := g.UsageEvents(s)
		if err != nil {
			t.Fatal(err)
		}
		sum := sumRecordedUsage(events)
		responses := 0
		for _, e := range events {
			responses += e.N
			if e.CWD != s.CWD || e.Provider != "" {
				t.Fatal("invented provider/project")
			}
		}
		if sum.Input != in || sum.Output != out || sum.CacheRead != cache || responses != n || sum.CostKnown || sum.Incomplete {
			t.Fatalf("usage=%+v responses=%d", sum, responses)
		}
	}
	assert(80, 10, 20, 1)
	// Updating the same durable response also moves it to another time slot/model.
	updated := geminiUsageLine("one", "gemini-b", "2026-10-18T11:00:00Z", 200, 10, 50, 5)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(updated + "\n" + geminiUsageLine("two", "gemini-a", "2026-10-18T11:01:00Z", 50, 4, 0, 0) + "\n")
	_ = f.Close()
	assert(200, 19, 50, 2)
	g.Index.save()
	g = NewGemini(home)
	g.Index = NewIndex(indexPath)
	assert(200, 19, 50, 2)
	events, _ := g.UsageEvents(s)
	if events[0].Model != "gemini-b" || !events[0].Time.Equal(time.Date(2026, 10, 18, 11, 0, 0, 0, time.UTC)) {
		t.Fatal("stale model/time bucket")
	}
	writeLines(t, path, first)
	assert(80, 10, 20, 1)
}
func TestGeminiUsageMissingCountersRemainUnavailable(t *testing.T) {
	for _, tokens := range []string{"null", "{}", `{"input":"invalid","output":2,"cached":0,"thoughts":0,"tool":0,"total":3}`, `{"input":1,"output":2,"cached":0,"thoughts":0,"tool":3,"total":6}`, `{"input":1,"output":2,"cached":5,"thoughts":0,"tool":0,"total":3}`} {
		path := writeLines(t, filepath.Join(t.TempDir(), "s.jsonl"), `{"id":"one","type":"gemini","model":"gemini-a","timestamp":"2026-10-18T10:00:00Z","tokens":`+tokens+`}`)
		events, err := NewGemini(t.TempDir()).UsageEvents(Session{Path: path})
		if err != nil || len(events) != 1 || !events[0].Usage.Incomplete {
			t.Fatalf("missing counters were presented as complete: %+v %v", events, err)
		}
	}
}

func TestGeminiBucketRetainsActualLatestResponse(t *testing.T) {
	path := writeLines(t, filepath.Join(t.TempDir(), "s.jsonl"), geminiUsageLine("a", "gemini-a", "2026-10-18T10:00:00Z", 1, 1, 0, 0), geminiUsageLine("b", "gemini-a", "2026-10-18T10:14:00Z", 1, 1, 0, 0))
	events, err := NewGemini(t.TempDir()).UsageEvents(Session{Path: path})
	if err != nil || len(events) != 1 || events[0].N != 2 || events[0].LastTime.UTC().Format(time.RFC3339) != "2026-10-18T10:14:00Z" {
		t.Fatalf("lost latest response: %+v %v", events, err)
	}
}

func TestGeminiMalformedSnapshotInvalidatesOldCounters(t *testing.T) {
	first := geminiUsageLine("same", "gemini-a", "2026-10-18T10:00:00Z", 10, 2, 0, 0)
	bad := `{"type":"gemini","id":"same","model":"gemini-a","timestamp":"2026-10-18T10:00:00Z","tokens":{"input":"invalid"}}`
	path := writeLines(t, filepath.Join(t.TempDir(), "s.jsonl"), first, bad)
	events, err := NewGemini(t.TempDir()).UsageEvents(Session{Path: path})
	if err != nil || len(events) != 1 || !events[0].Usage.Incomplete || events[0].N != 1 {
		t.Fatalf("kept obsolete complete counters: %+v %v", events, err)
	}
}
