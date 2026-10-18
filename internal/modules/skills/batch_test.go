package skills

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
)

func TestMarkedScopeFollowsCanonicalVisibleRows(t *testing.T) {
	m := inventoryTab(t)
	_, _, top := m.tableWindow(m.split().ListW, m.split().ListH)
	m, _ = m.click(tea.MouseClickMsg{X: 3, Y: top, Button: tea.MouseLeft})
	if !m.marked["review-folder"] {
		t.Fatal("checkbox click did not mark the row")
	}
	m, _ = m.click(tea.MouseClickMsg{X: 3, Y: top, Button: tea.MouseLeft})
	m.list.Select(1)
	m.Update(tea.KeyPressMsg{Code: 'm', Text: "m"})
	m.list.Select(2)
	m.Update(tea.KeyPressMsg{Code: 'm', Text: "m"})
	if len(m.marked) != 2 || !m.marked["folder-one"] || !m.marked["folder-two"] {
		t.Fatal("duplicate display names lost canonical selection")
	}
	searchInventory(&m, "folder-one")
	if len(m.marked) != 1 || !m.marked["folder-one"] {
		t.Fatalf("hidden marks retained: %v", m.marked)
	}
	m.Update(tea.KeyPressMsg{Code: 'F', Text: "F"})
	if len(m.marked) != 1 {
		t.Fatal("clearing filters restored hidden selections")
	}
	m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	if len(m.marked) != len(m.skills) {
		t.Fatal("select visible omitted off-screen rows")
	}
	m.skills = m.skills[:1]
	m.rebuildListItems()
	if len(m.marked) != 1 || !m.marked["review-folder"] {
		t.Fatal("removed inventory entries remained marked")
	}
	m.Update(tea.KeyPressMsg{Code: 'V', Text: "V"})
	m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if m.mode != skModeList {
		t.Fatal("empty selection opened an actionable batch")
	}
	searchInventory(&m, "no matching skill")
	m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	if len(m.marked) != 0 {
		t.Fatal("empty visible set selected hidden skills")
	}
}

func TestBatchReviewFreezesScopeAndBlocksDuplicateExecution(t *testing.T) {
	p := testPaths(t)
	m := newTab(New(p))
	a, b := testAgent(p.Home, "a", "a/skills"), testAgent(p.Home, "b", "b/skills")
	m.agents, m.targets = []agent.Agent{a, b}, []agent.Agent{a, b}
	writeSkill(t, p.LibraryDir(), "zz-selected-zz", validMD("same", "zz-selected-zz"))
	writeSkill(t, p.LibraryDir(), "hidden", validMD("same", "hidden"))
	m.skills, _ = m.svc.Scan(m.agents)
	m.rebuildListItems()
	m.width, m.height, m.col = 80, 24, 1
	searchInventory(&m, "zz-selected-zz")
	m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if !reflect.DeepEqual(m.batch.plan.ids, []string{"b"}) {
		t.Fatalf("batch expanded the current destination: %v", m.batch.plan.ids)
	}
	m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.mode != skModeBatch || m.batch.err == "" {
		t.Fatal("accepted no destinations")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace}) // only agent A
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	question := m.confirm.Question
	if m.mode != skModeConfirm || !strings.Contains(question, "1 selected / 1 visible / 2 total") || !strings.Contains(question, "Agent destinations: a") || strings.Contains(question, "- hidden") {
		t.Fatalf("unclear review: %s", question)
	}
	// Enter defaults to No, returning to the draft without writes.
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.mode != skModeBatch || scanOne(t, m.svc, m.agents, "zz-selected-zz").EnabledCount() != 0 {
		t.Fatal("cancelled batch mutated the matrix")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	// A background scan cannot add a newly discovered row to the review.
	writeSkill(t, p.LibraryDir(), "new", validMD("new", "new"))
	skills, _ := m.svc.Scan(m.agents)
	m, _ = m.update(scannedMsg{skills: skills})
	m, cmd := m.update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if m.mode != skModeBatchBusy || !m.Capturing() {
		t.Fatal("batch not locked while running")
	}
	for _, key := range []tea.KeyPressMsg{{Code: 'y', Text: "y"}, {Code: 'c', Text: "c"}, {Code: tea.KeyEscape}} {
		if next := m.Update(key); next != nil || m.mode != skModeBatchBusy {
			t.Fatal("busy batch accepted another action")
		}
	}
	// tea.Batch contains only the spinner and the frozen operation.
	var completion skillBatchMsg
	for _, c := range cmd().(tea.BatchMsg) {
		if msg, ok := c().(skillBatchMsg); ok {
			completion = msg
		}
	}
	if len(completion.result.Targets) != 1 || completion.result.Targets[0].ID != "zz-selected-zz / a" || completion.result.AsError() != nil {
		t.Fatalf("completion: %+v", completion)
	}
	m, _ = m.update(completion)
	if m.report == nil || m.mode != skModeList || m.inFlight {
		t.Fatal("batch did not finish in the persistent report")
	}
	for _, dir := range []string{"zz-selected-zz", "hidden", "new"} {
		sk := scanOne(t, m.svc, m.agents, dir)
		if sk.States[a.ID].On != (dir == "zz-selected-zz") || sk.States[b.ID].On {
			t.Fatalf("scope escaped for %s: %+v", dir, sk.States)
		}
	}
}

func TestBatchPartialRetryKeepsSuccessfulPairsAndForeignContent(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	a, b, c := testAgent(p.Home, "a", "a/skills"), testAgent(p.Home, "b", "b/skills"), testAgent(p.Home, "c", "c/skills")
	agents := []agent.Agent{a, b, c}
	for _, dir := range []string{"one", "two", "unrelated"} {
		writeSkill(t, p.LibraryDir(), dir, validMD(dir, "library"))
	}
	localPath := writeSkill(t, b.ManagedDir, "one", validMD("one", "local"))
	foreignPath := writeSkill(t, t.TempDir(), "two", validMD("two", "foreign"))
	mustSymlink(t, foreignPath, filepath.Join(b.ManagedDir, "two"))
	if err := os.MkdirAll(filepath.Dir(c.ManagedDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.ManagedDir, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan := skillBatch{action: batchEnable, agents: agents, ids: []string{"a", "b", "c"}, skills: []Skill{scanOne(t, svc, agents, "one"), scanOne(t, svc, agents, "two")}}
	r := svc.runBatch(plan, nil)
	if !reflect.DeepEqual(r.FailedIDs(), []string{"one / c", "two / c"}) || r.Targets[1].Outcome != core.Skipped {
		t.Fatalf("partial result: %+v", r)
	}
	before, _ := os.Lstat(filepath.Join(a.ManagedDir, "one"))
	if err := os.Remove(c.ManagedDir); err != nil {
		t.Fatal(err)
	}
	m := newTab(svc)
	m.finishBatch(skillBatchMsg{plan: plan, result: r})
	next := m.report.Retry(r.FailedIDs())
	if next.AsError() != nil || len(next.Targets) != 2 {
		t.Fatalf("retry: %+v", next)
	}
	m, _ = m.update(components.OperationFinished{Report: m.report, Result: next})
	after, _ := os.Lstat(filepath.Join(a.ManagedDir, "one"))
	if !os.SameFile(before, after) || m.report.Result.AsError() != nil {
		t.Fatal("retry repeated a successful pair or failed to merge")
	}
	if r := svc.runBatch(plan, []string{}); len(r.Targets) != 0 || r.AsError() != nil {
		t.Fatal("empty retry repeated the original batch")
	}
	plan.action, plan.ids = batchDisable, []string{"c"}
	if r := svc.runBatch(plan, nil); r.AsError() != nil {
		t.Fatal(r.AsError())
	}
	if !scanOne(t, svc, agents, "one").States[a.ID].On || scanOne(t, svc, agents, "one").States[c.ID].On || scanOne(t, svc, agents, "unrelated").EnabledCount() != 0 {
		t.Fatal("explicit disable changed other destinations or skills")
	}
	plan.action = batchRemove
	if r := svc.runBatch(plan, nil); r.AsError() != nil || len(r.Targets) != 2 {
		t.Fatalf("remove: %+v", r)
	}
	for _, path := range []string{filepath.Join(localPath, "SKILL.md"), filepath.Join(foreignPath, "SKILL.md"), filepath.Join(b.ManagedDir, "two"), filepath.Join(p.LibraryDir(), "unrelated", "SKILL.md")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("preserved content missing: %s: %v", path, err)
		}
	}
	if backups, err := svc.ListBackups(); err != nil || len(backups) != 2 {
		t.Fatalf("remove backups: %v, %v", backups, err)
	}
}

func TestBatchUpdatesPreserveEditsAndReviewedSource(t *testing.T) {
	repo := t.TempDir()
	git := makeGitRepo(t, repo)
	for _, dir := range []string{"chosen", "edited", "unknown", "redirected", "hidden"} {
		writeSkill(t, repo, dir, validMD(dir, "v1"))
	}
	git("add", ".")
	git("commit", "-m", "v1")
	p := testPaths(t)
	svc := New(p)
	installGitSkill(t, svc, "file://"+repo, "chosen", nil)
	skills, _ := svc.Scan(nil)
	plan := skillBatch{action: batchUpdate}
	for _, sk := range skills {
		if sk.Dir != "hidden" {
			plan.skills = append(plan.skills, sk)
		}
	}
	writeSkill(t, p.LibraryDir(), "edited", validMD("edited", "local edit"))
	unknown := scanOne(t, svc, nil, "unknown")
	o := *unknown.Origin
	o.Hash = ""
	if err := writeOrigin(unknown.Path, o); err != nil {
		t.Fatal(err)
	}
	redirected := scanOne(t, svc, nil, "redirected")
	o = *redirected.Origin
	o.Source = "file:///unreviewed-source"
	if err := writeOrigin(redirected.Path, o); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, repo, "chosen", validMD("chosen", "v2"))
	writeSkill(t, repo, "hidden", validMD("hidden", "v2"))
	git("add", ".")
	git("commit", "-m", "v2")
	r := svc.runBatch(plan, nil)
	if !reflect.DeepEqual(r.FailedIDs(), []string{"redirected / library"}) {
		t.Fatalf("update outcomes: %+v", r)
	}
	for _, tt := range []struct{ dir, desc string }{{"chosen", "v2"}, {"edited", "local edit"}, {"unknown", "v1"}, {"hidden", "v1"}} {
		if got := scanOne(t, svc, nil, tt.dir).Description; got != tt.desc {
			t.Fatalf("%s = %s, want %s", tt.dir, got, tt.desc)
		}
	}
	if r.Targets[0].Outcome != core.Succeeded || r.Targets[1].Outcome != core.Skipped || r.Targets[len(r.Targets)-1].Outcome != core.Skipped {
		t.Fatalf("update outcomes: %+v", r)
	}
	if backups, _ := svc.ListBackups(); len(backups) != 1 {
		t.Fatal("skipped updates wrote backups")
	}
	// Outside-library rows and symlink library entries never become writable.
	local := writeSkill(t, t.TempDir(), "external", validMD("external", "foreign"))
	mustSymlink(t, local, filepath.Join(p.LibraryDir(), "external"))
	for _, action := range []batchAction{batchEnable, batchDisable, batchUpdate, batchRemove} {
		p := skillBatch{action: action, ids: []string{"a"}, skills: []Skill{scanOne(t, svc, nil, "external")}}
		if r := svc.runBatch(p, nil); len(r.Targets) != 1 || r.Targets[0].Outcome != core.Skipped {
			t.Fatalf("linked library entry: %+v", r)
		}
	}
}

func TestBatchExplicitTargetsPreserveSharedReadersAndLocalRows(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	a, b := testAgent(p.Home, "a", "a/skills"), testAgent(p.Home, "b", "b/skills")
	shared := filepath.Join(p.Home, "shared")
	a.SharedDir, b.SharedDir = shared, shared
	a.ReadDirs, b.ReadDirs = append(a.ReadDirs, shared), append(b.ReadDirs, shared)
	agents := []agent.Agent{a, b}
	writeSkill(t, p.LibraryDir(), "shared", validMD("shared", "library"))
	writeSkill(t, a.ManagedDir, "native", validMD("native", "local"))
	if err := svc.EnableAll(scanOne(t, svc, agents, "shared"), agents); err != nil {
		t.Fatal(err)
	}
	plan := skillBatch{action: batchDisable, agents: agents, ids: []string{"a"}, skills: []Skill{scanOne(t, svc, agents, "shared"), scanOne(t, svc, agents, "native")}}
	r := svc.runBatch(plan, nil)
	if r.AsError() != nil || len(r.Targets) != 2 || r.Targets[1].Outcome != core.Skipped {
		t.Fatalf("shared/local results: %+v", r)
	}
	sk := scanOne(t, svc, agents, "shared")
	if sk.States[a.ID].On || !sk.States[b.ID].On {
		t.Fatalf("shared reader scope: %+v", sk.States)
	}
	for _, action := range []batchAction{batchEnable, batchUpdate, batchRemove} {
		plan.action, plan.skills = action, []Skill{scanOne(t, svc, agents, "native")}
		r := svc.runBatch(plan, nil)
		if len(r.Targets) != 1 || r.Targets[0].Outcome != core.Skipped || !scanOne(t, svc, agents, "native").States[a.ID].Local {
			t.Fatalf("native row changed: %+v", r)
		}
	}
}

func TestBatchPanelFitsAndNamesEveryReviewedFolder(t *testing.T) {
	for _, size := range [][2]int{{40, 12}, {80, 24}, {120, 36}} {
		m := matrixTab(t, 40, size[0], size[1])
		for i := range m.agents {
			m.agents[i].Installed = true
			m.agents[i].ManagedDir = "/fixture/" + m.agents[i].ID
		}
		m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
		m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
		for _, action := range []batchAction{batchEnable, batchDisable, batchUpdate, batchRemove} {
			m.batch.plan.action = action
			view := m.View()
			if plain := ansi.Strip(view); !strings.Contains(plain, "40 selected / 40 visible") || !strings.Contains(plain, "40 total in inventory") {
				t.Fatalf("panel counts clipped: %s", plain)
			}
			if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] {
				t.Fatalf("panel %v: %dx%d", size, lipgloss.Width(view), lipgloss.Height(view))
			}
			m, _ = m.updateBatch(tea.KeyPressMsg{Code: tea.KeyEnter})
			if !strings.Contains(m.confirm.Question, "skill-39") || !strings.Contains(m.confirm.Question, "40 selected") {
				t.Fatal("review omitted off-screen rows")
			}
			if view = m.View(); lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] {
				t.Fatalf("review %v: %dx%d", size, lipgloss.Width(view), lipgloss.Height(view))
			}
			m.mode = skModeBatch
		}
	}
}

func TestRemoveKeepsLibraryWhenLinkCleanupFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix directory permissions")
	}
	p := testPaths(t)
	svc := New(p)
	ag := testAgent(p.Home, "a", "a/skills")
	writeSkill(t, p.LibraryDir(), "sk", validMD("sk", "library"))
	sk := scanOne(t, svc, []agent.Agent{ag}, "sk")
	if err := svc.Enable(sk, ag, []agent.Agent{ag}); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ag.ManagedDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ag.ManagedDir, 0o755) })
	probe := filepath.Join(ag.ManagedDir, "probe")
	if err := os.WriteFile(probe, nil, 0o600); err == nil {
		_ = os.Remove(probe)
		t.Skip("filesystem bypasses directory permissions")
	}
	plan := skillBatch{action: batchRemove, agents: []agent.Agent{ag}, skills: []Skill{sk}}
	r := svc.runBatch(plan, nil)
	if !slices.Equal(r.FailedIDs(), []string{"sk / library"}) || !scanOne(t, svc, plan.agents, "sk").InLibrary {
		t.Fatalf("library removed after failed cleanup: %+v", r)
	}
	if err := os.Chmod(ag.ManagedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if r := svc.runBatch(plan, r.FailedIDs()); r.AsError() != nil || r.Targets[0].Outcome != core.Succeeded {
		t.Fatalf("remove retry: %+v", r)
	}
}
