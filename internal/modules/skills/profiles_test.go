package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// enable enables the skill (by folder name) in the agent, rescanning first.
func enable(t *testing.T, svc *Service, agents []agent.Agent, dir string, ag agent.Agent) {
	t.Helper()
	sk := scanOne(t, svc, agents, dir)
	if err := svc.Enable(sk, ag, nil); err != nil {
		t.Fatalf("Enable %s in %s: %v", dir, ag.ID, err)
	}
}

func TestSaveGetListProfile(t *testing.T) {
	p := testPaths(t)
	svc := New(p)

	// missing file → empty list, no error
	names, err := svc.ListProfiles()
	if err != nil || len(names) != 0 {
		t.Fatalf("initial ListProfiles: err=%v names=%v", err, names)
	}

	// save two profiles (per-agent spec, agent lists deduped)
	if err := svc.SaveProfile("work", ProfileSpec{
		"sk-a": {"codex", "claude-code", "claude-code"},
		"sk-b": {"claude-code"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SaveProfile("personal", ProfileSpec{"sk-c": {"claude-code"}}); err != nil {
		t.Fatal(err)
	}

	if err := svc.SaveProfile("", ProfileSpec{"sk-a": {"claude-code"}}); err == nil {
		t.Fatal("empty name should fail")
	}

	names, err = svc.ListProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "personal" || names[1] != "work" {
		t.Fatalf("ListProfiles: %v", names)
	}

	spec, err := svc.GetProfile("work")
	if err != nil {
		t.Fatal(err)
	}
	// sk-a agents deduped and sorted
	if got := spec["sk-a"]; !reflect.DeepEqual(got, []string{"claude-code", "codex"}) {
		t.Fatalf("GetProfile sk-a agents: %v", got)
	}
	if got := spec["sk-b"]; !reflect.DeepEqual(got, []string{"claude-code"}) {
		t.Fatalf("GetProfile sk-b agents: %v", got)
	}

	if _, err := svc.GetProfile("missing"); err == nil {
		t.Fatal("GetProfile of a missing profile should fail")
	}
}

func TestSaveProfileDropsEmpty(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	if err := svc.SaveProfile("x", ProfileSpec{
		"sk-a": {"claude-code"},
		"sk-b": {},              // no agents → dropped
		"":     {"claude-code"}, // empty skill name → dropped
	}); err != nil {
		t.Fatal(err)
	}
	spec, err := svc.GetProfile("x")
	if err != nil {
		t.Fatal(err)
	}
	if len(spec) != 1 || spec["sk-a"] == nil {
		t.Fatalf("empty entries not dropped: %v", spec)
	}
}

func TestDeleteProfile(t *testing.T) {
	p := testPaths(t)
	svc := New(p)

	if err := svc.SaveProfile("temp", ProfileSpec{"sk-a": {"claude-code"}}); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteProfile("temp"); err != nil {
		t.Fatal(err)
	}
	names, _ := svc.ListProfiles()
	for _, n := range names {
		if n == "temp" {
			t.Fatal("profile should be deleted")
		}
	}
	// deleting a missing profile is not an error
	if err := svc.DeleteProfile("missing"); err != nil {
		t.Fatalf("delete missing: %v", err)
	}
}

func TestProfilesRoundTripUnknownFields(t *testing.T) {
	p := testPaths(t)
	svc := New(p)

	// JSON with an unknown "version" field and a new-format profile
	initial := `{"version": 42, "profiles": {"x": {"sk-a": ["claude-code"]}}}`
	if err := os.MkdirAll(filepath.Dir(p.ProfilesPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.ProfilesPath(), []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	// saving a new profile must keep "version"
	if err := svc.SaveProfile("y", ProfileSpec{"sk-b": {"codex"}}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(p.ProfilesPath())
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("invalid JSON after save: %v", err)
	}
	if _, ok := raw["version"]; !ok {
		t.Error("unknown field 'version' lost in the round-trip")
	}
	if _, ok := raw["profiles"]; !ok {
		t.Error("'profiles' field gone")
	}
}

// TestProfileLegacyMigration: a legacy (flat list) profile reads as "all agents"
// and is saved back in the new format with concrete ids.
func TestProfileLegacyMigration(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	lib := p.LibraryDir()
	writeSkill(t, lib, "sk-a", validMD("sk-a", "desc"))
	writeSkill(t, lib, "sk-b", validMD("sk-b", "desc"))

	// legacy format: skill array
	legacy := `{"profiles": {"old": ["sk-a"]}}`
	if err := os.MkdirAll(filepath.Dir(p.ProfilesPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.ProfilesPath(), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	// read as {sk-a: ["*"]}
	spec, err := svc.GetProfile("old")
	if err != nil {
		t.Fatal(err)
	}
	if got := spec["sk-a"]; !reflect.DeepEqual(got, []string{allAgents}) {
		t.Fatalf("legacy migration: %v", got)
	}

	// applying expands "*" to every installed agent
	agC := testAgent(p.Home, "claude-code", ".claude/skills")
	agX := testAgent(p.Home, "codex", ".codex/skills")
	agents := []agent.Agent{agC, agX}
	if err := svc.ApplyProfile("old", agents); err != nil {
		t.Fatalf("legacy ApplyProfile: %v", err)
	}
	for _, ag := range agents {
		if _, err := os.Lstat(filepath.Join(ag.ManagedDir, "sk-a")); err != nil {
			t.Errorf("sk-a should be enabled in %s", ag.ID)
		}
	}

	// saving a snapshot turns "*" into concrete ids
	skills, _ := svc.Scan(agents)
	if err := svc.SaveProfile("old", BuildProfileSpec(skills, agents)); err != nil {
		t.Fatal(err)
	}
	spec, _ = svc.GetProfile("old")
	if got := spec["sk-a"]; !reflect.DeepEqual(got, []string{"claude-code", "codex"}) {
		t.Fatalf("after saving again: %v", got)
	}
}

// TestBuildProfileSpecSnapshot: the snapshot records the exact agents and skips
// local skills.
func TestBuildProfileSpecSnapshot(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	lib := p.LibraryDir()
	writeSkill(t, lib, "sk-a", validMD("sk-a", "desc"))
	writeSkill(t, lib, "sk-b", validMD("sk-b", "desc"))

	agC := testAgent(p.Home, "claude-code", ".claude/skills")
	agX := testAgent(p.Home, "codex", ".codex/skills")
	agents := []agent.Agent{agC, agX}

	// sk-a only in claude-code; sk-b in both
	enable(t, svc, agents, "sk-a", agC)
	enable(t, svc, agents, "sk-b", agC)
	enable(t, svc, agents, "sk-b", agX)

	skills, _ := svc.Scan(agents)
	spec := BuildProfileSpec(skills, agents)
	if got := spec["sk-a"]; !reflect.DeepEqual(got, []string{"claude-code"}) {
		t.Errorf("sk-a snapshot: %v", got)
	}
	if got := spec["sk-b"]; !reflect.DeepEqual(got, []string{"claude-code", "codex"}) {
		t.Errorf("sk-b snapshot: %v", got)
	}
}

// TestApplyProfilePerAgent: applying restores the per-agent matrix, idempotently.
func TestApplyProfilePerAgent(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	lib := p.LibraryDir()
	writeSkill(t, lib, "sk-a", validMD("sk-a", "desc a"))
	writeSkill(t, lib, "sk-b", validMD("sk-b", "desc b"))
	writeSkill(t, lib, "sk-c", validMD("sk-c", "desc c"))

	agC := testAgent(p.Home, "claude-code", ".claude/skills")
	agX := testAgent(p.Home, "codex", ".codex/skills")
	agents := []agent.Agent{agC, agX}

	// profile: sk-a in both, sk-b only in claude-code
	if err := svc.SaveProfile("work", ProfileSpec{
		"sk-a": {"claude-code", "codex"},
		"sk-b": {"claude-code"},
	}); err != nil {
		t.Fatal(err)
	}

	on := func(ag agent.Agent, name string) bool {
		_, err := os.Lstat(filepath.Join(ag.ManagedDir, name))
		return err == nil
	}
	apply := func() {
		t.Helper()
		if err := svc.ApplyProfile("work", agents); err != nil {
			t.Fatalf("ApplyProfile: %v", err)
		}
	}

	// dirty the matrix first: enable sk-c in codex
	enable(t, svc, agents, "sk-c", agX)

	apply()
	check := func() {
		t.Helper()
		if !on(agC, "sk-a") || !on(agX, "sk-a") {
			t.Error("sk-a should be in both agents")
		}
		if !on(agC, "sk-b") || on(agX, "sk-b") {
			t.Error("sk-b should be only in claude-code")
		}
		if on(agC, "sk-c") || on(agX, "sk-c") {
			t.Error("sk-c should be disabled everywhere")
		}
	}
	check()

	apply()
	check()
}

func TestDiffProfile(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	lib := p.LibraryDir()
	writeSkill(t, lib, "sk-a", validMD("sk-a", "desc a"))
	writeSkill(t, lib, "sk-b", validMD("sk-b", "desc b"))

	agC := testAgent(p.Home, "claude-code", ".claude/skills")
	agX := testAgent(p.Home, "codex", ".codex/skills")
	agents := []agent.Agent{agC, agX}

	// profile wants sk-a in both; now sk-a is only in claude-code, sk-b in codex
	if err := svc.SaveProfile("work", ProfileSpec{"sk-a": {"claude-code", "codex"}}); err != nil {
		t.Fatal(err)
	}
	enable(t, svc, agents, "sk-a", agC)
	enable(t, svc, agents, "sk-b", agX)

	changes, err := svc.DiffProfile("work", agents)
	if err != nil {
		t.Fatal(err)
	}
	byName := make(map[string]ProfileChange, len(changes))
	for _, c := range changes {
		byName[c.Skill] = c
	}
	if got := byName["sk-a"].Add; !reflect.DeepEqual(got, []string{"codex"}) {
		t.Errorf("sk-a Add: %v", got)
	}
	if got := byName["sk-b"].Remove; !reflect.DeepEqual(got, []string{"codex"}) {
		t.Errorf("sk-b Remove: %v", got)
	}

	// after applying, the diff is empty
	if err := svc.ApplyProfile("work", agents); err != nil {
		t.Fatal(err)
	}
	changes, err = svc.DiffProfile("work", agents)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 0 {
		t.Errorf("diff after applying should be empty: %v", changes)
	}
}

// TestProfileSharedReadEcho: an agent reading another's managed dir (opencode
// reading ~/.claude/skills) is neither snapshotted nor disabled by Apply:
// disabling it would remove the other agent's symlink.
func TestProfileSharedReadEcho(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	writeSkill(t, p.LibraryDir(), "sk-a", validMD("sk-a", "desc"))

	agC := testAgent(p.Home, "claude-code", ".claude/skills")
	// opencode: its own managed dir, plus reads claude's
	agO := testAgent(p.Home, "opencode", ".config/opencode/skills", ".claude/skills")
	agents := []agent.Agent{agC, agO}

	// sk-a only in claude; opencode "sees" it via the shared dir (echo)
	enable(t, svc, agents, "sk-a", agC)

	sk := scanOne(t, svc, agents, "sk-a")
	if !sk.States["claude-code"].Managed {
		t.Fatal("sk-a should be managed in claude-code")
	}
	if sk.States["opencode"].Managed {
		t.Fatal("echo in opencode should not count as Managed")
	}

	// the snapshot only has claude
	spec := BuildProfileSpec([]Skill{sk}, agents)
	if got := spec["sk-a"]; !reflect.DeepEqual(got, []string{"claude-code"}) {
		t.Fatalf("snapshot with echo: %v", got)
	}

	// a profile without sk-a: Apply disables it in claude; the echo is the same
	// symlink, already gone.
	if err := svc.SaveProfile("empty", ProfileSpec{}); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyProfile("empty", agents); err != nil {
		t.Fatalf("empty ApplyProfile: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(agC.ManagedDir, "sk-a")); err == nil {
		t.Error("sk-a should be disabled in claude")
	}
}

func TestApplyProfileMissingSkill(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	writeSkill(t, p.LibraryDir(), "sk-a", validMD("sk-a", "desc"))

	ag := testAgent(p.Home, "claude-code", ".claude/skills")

	if err := svc.SaveProfile("bad", ProfileSpec{
		"sk-a":    {"claude-code"},
		"missing": {"claude-code"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyProfile("bad", []agent.Agent{ag}); err == nil {
		t.Fatal("a missing skill should fail")
	}
}

func TestApplyProfileNotFound(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	if err := svc.ApplyProfile("missing", nil); err == nil {
		t.Fatal("a missing profile should fail")
	}
}

// d in the profile list asks, then deletes the profile and reloads the list.
func TestProfileDeleteFromTab(t *testing.T) {
	svc := New(core.PathsIn(t.TempDir()))
	if err := svc.SaveProfile("work", ProfileSpec{"sk": {"claude-code"}}); err != nil {
		t.Fatal(err)
	}
	m := newTab(svc)
	m.profileNames, m.mode = []string{"work"}, skModeProfiles

	m, _ = m.updateProfiles(tea.KeyPressMsg{Code: 'd', Text: "d"})
	if m.mode != skModeConfirm {
		t.Fatalf("d should ask first, mode = %v", m.mode)
	}
	m, cmd := m.updateConfirm(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if m.mode != skModeProfiles || cmd == nil {
		t.Fatalf("confirm should go back to profiles and delete, mode = %v", m.mode)
	}
	if msg, ok := cmd().(profileDeleteMsg); !ok || msg.err != nil {
		t.Fatalf("delete result = %#v", msg)
	}
	if names, _ := svc.ListProfiles(); len(names) != 0 {
		t.Errorf("profiles after delete = %v", names)
	}
}

// A confirmation acts on its own kind: d on a skill after a profile delete
// removes the skill, not the profile again.
func TestRemoveConfirmDoesNotReusePreviousKind(t *testing.T) {
	svc := New(core.PathsIn(t.TempDir()))
	m := newTab(svc)
	m.ckind = confirmKindDeleteProfile // left over from an earlier confirmation
	m.skills = []Skill{{Dir: "sk", Name: "sk", InLibrary: true, Valid: true, States: map[string]AgentState{}}}
	m.rebuildListItems()

	m, _ = m.updateList(tea.KeyPressMsg{Code: 'd', Text: "d"})
	if m.ckind != confirmKindRemove {
		t.Fatalf("ckind = %v, want confirmKindRemove", m.ckind)
	}
	_, cmd := m.updateConfirm(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd == nil {
		t.Fatal("confirm produced no action")
	}
	if _, ok := cmd().(skillOpMsg); !ok {
		t.Error("confirming a skill removal must run the removal")
	}
}
