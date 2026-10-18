package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

func TestInstallAndBatchHistoryUseCompletedResultsOnce(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	source := t.TempDir()
	writeSkill(t, source, "first", validMD("first", "PRIVATE_INSTRUCTIONS"))
	found, origin, _, err := svc.Discover(source)
	if err != nil {
		t.Fatal(err)
	}
	a, b := testAgent(p.Home, "a", "a/skills"), testAgent(p.Home, "b", "b/skills")
	if err := os.MkdirAll(filepath.Dir(b.ManagedDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b.ManagedDir, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := svc.installWithTargets(found, origin, []agent.Agent{a, b}, []string{"a", "b"})
	if out.result.AsError() == nil || out.result.HistoryWarning != "" {
		t.Fatalf("result=%+v", out.result)
	}
	entries, err := core.ReadHistory(p)
	if err != nil || len(entries) != 1 || entries[0].Action != "install" || !entries[0].Failed {
		t.Fatalf("aggregate duplicated or lost: %+v %v", entries, err)
	}
	if len(entries[0].Targets) != len(out.result.Targets) {
		t.Fatal("target outcomes omitted")
	}
	if err := os.Remove(b.ManagedDir); err != nil {
		t.Fatal(err)
	}
	next := svc.retryInstall(out, out.result.FailedIDs())
	if next.AsError() != nil {
		t.Fatal(next.AsError())
	}
	entries, err = core.ReadHistory(p)
	if err != nil || len(entries) != 2 || entries[0].Action != "retry-install" || len(entries[0].Targets) != 1 {
		t.Fatalf("retry history=%+v %v", entries, err)
	}
	sk := scanOne(t, svc, []agent.Agent{a, b}, "first")
	result := svc.runBatch(skillBatch{action: batchRemove, skills: []Skill{sk}, agents: []agent.Agent{a, b}}, nil)
	if result.AsError() != nil {
		t.Fatal(result.AsError())
	}
	entries, err = core.ReadHistory(p)
	if err != nil || len(entries) != 3 || entries[0].Action != "batch-remove" || len(entries[0].Backups) != 1 {
		t.Fatalf("batch history=%+v %v", entries, err)
	}
	data, _ := os.ReadFile(p.HistoryPath())
	if strings.Contains(string(data), "PRIVATE_INSTRUCTIONS") || strings.Contains(string(data), source) {
		t.Fatal("history retained source payload/path")
	}
}
