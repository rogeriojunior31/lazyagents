package agent

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func openCodeUsageFixture(t *testing.T) *OpenCode {
	t.Helper()
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 not installed")
	}
	home := t.TempDir()
	if err := copyFixtureHome(filepath.Join(fixturesDir, "opencode", "2.0.32", "home"), home); err != nil {
		t.Fatal(err)
	}
	return NewOpenCode(home)
}
func TestRecordedOpenCodeUsageIsPerResponseAndProvider(t *testing.T) {
	o := openCodeUsageFixture(t)
	sessions, err := o.ListSessions()
	if err != nil || len(sessions) != 1 {
		t.Fatal(err)
	}
	for range 2 {
		events, err := o.UsageEvents(sessions[0])
		if err != nil || len(events) != 2 {
			t.Fatalf("events: %+v %v", events, err)
		}
		sum := sumRecordedUsage(events)
		if sum.Input != 170 || sum.Output != 14 || sum.CacheRead != 100 || !sum.CostKnown || sum.Cost != 0 || sum.Incomplete {
			t.Fatal(sum)
		}
		for _, e := range events {
			if e.Provider != "fake" || e.Model != "fake-model" {
				t.Fatalf("wrong response identity: %+v", e)
			}
		}
	}
	// Exercise the recorded schema: new provider/model and separate reasoning, never native settings.
	q := `UPDATE session_message SET data=json_set(data,'$.model.id','other-model','$.model.providerID','other','$.tokens.reasoning',3,'$.cost',1.25) WHERE type='assistant' AND seq=14;`
	if raw, err := exec.Command("sqlite3", o.dbPath(), q).CombinedOutput(); err != nil {
		t.Fatalf("%s %v", raw, err)
	}
	events, err := o.UsageEvents(sessions[0])
	if err != nil || events[1].Model != "other-model" || events[1].Provider != "other" || events[1].Usage.Output != 9 || events[1].Usage.Cost != 1.25 {
		t.Fatalf("updated row: %+v %v", events, err)
	}
	if raw, err := exec.Command("sqlite3", o.dbPath(), `UPDATE session_message SET data=json_remove(data,'$.tokens','$.cost') WHERE type='assistant' AND seq=14;`).CombinedOutput(); err != nil {
		t.Fatalf("%s %v", raw, err)
	}
	events, err = o.UsageEvents(sessions[0])
	if err != nil || !events[1].Usage.Incomplete || events[1].Usage.CostKnown {
		t.Fatal("absent tokens/cost became zero")
	}
}

func TestSessionUsageDoesNotExposePartialRecordedCostAsKnown(t *testing.T) {
	sum := sumRecordedUsage([]UsageEvent{{Model: "m", Usage: Usage{CostKnown: true, Cost: 3}}, {Model: "m", Usage: Usage{Input: 10}}})
	if sum.CostKnown || sum.Cost != 0 {
		t.Fatal("partial native sum could be mistaken for full recorded cost")
	}
}
