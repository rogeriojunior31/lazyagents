package skills

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/rogeriojunior31/lazyagents/internal/agent"
)

func TestFindSkillPrefersCanonicalFolderOverDisplayName(t *testing.T) {
	skills := []Skill{{Dir: "other", Name: "selected"}, {Dir: "selected", Name: "zzz"}}
	got, ok := findSkill("selected", skills)
	if !ok || got.Dir != "selected" {
		t.Fatalf("canonical folder resolved to %+v", got)
	}
	if got, ok := findSkill("zzz", skills); !ok || got.Dir != "selected" {
		t.Fatal("display-name compatibility lost")
	}
}

func TestCloneRevisionIgnoresGitDiagnostics(t *testing.T) {
	repo := t.TempDir()
	git := makeGitRepo(t, repo)
	writeSkill(t, repo, "alpha", validMD("alpha", "one"))
	git("add", ".")
	git("commit", "-m", "one")
	want, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_TRACE", "1")
	root, revision, err := cloneSource(localRepositoryURL(repo), "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	if revision != strings.TrimSpace(string(want)) {
		t.Fatalf("diagnostics contaminated revision: %q", revision)
	}
}

func TestReviewedUpdateRejectsChangedBinaryFileLayout(t *testing.T) {
	svc, _, _, skills := reviewFixture(t)
	lib := filepath.Join(svc.paths.LibraryDir(), "alpha")
	a := filepath.Join(lib, "a")
	if err := os.WriteFile(a, []byte("xb\x00y"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := svc.ReviewUpdates(context.Background(), skills[:1], nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if len(r.Checks) != 1 || r.Checks[0].Err != nil {
		t.Fatalf("review failed: %+v", r.Checks)
	}
	if err := os.WriteFile(a, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lib, "b"), []byte("y"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := svc.ApplyUpdates(context.Background(), r, true, nil)
	if result.AsError() == nil || len(result.Targets) != 1 || !strings.Contains(result.Targets[0].Detail, "local content changed since review") {
		t.Fatalf("changed binary file layout was not refused: %+v", result)
	}
	if got, err := os.ReadFile(a); err != nil || string(got) != "x" {
		t.Fatalf("local content not preserved: %q, %v", got, err)
	}
}

func TestLegacyInstallBaselineRemainsUsable(t *testing.T) {
	for _, edited := range []bool{false, true} {
		t.Run(fmt.Sprintf("edited=%t", edited), func(t *testing.T) {
			svc, _, _, skills := reviewFixture(t)
			lib := filepath.Join(svc.paths.LibraryDir(), "alpha")
			o := *readOrigin(lib)
			installed, err := os.ReadFile(filepath.Join(lib, "SKILL.md"))
			if err != nil {
				t.Fatal(err)
			}
			o.Hash = fmt.Sprintf("%x", sha256.Sum256(append([]byte("SKILL.md\x00"), installed...)))
			if err := writeOrigin(lib, o); err != nil {
				t.Fatal(err)
			}
			skills[0].Origin = &o
			if edited {
				writeSkill(t, svc.paths.LibraryDir(), "alpha", validMD("alpha", "local edit"))
			}
			r, err := svc.ReviewUpdates(context.Background(), skills[:1], nil)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			c := r.Checks[0]
			if c.Err != nil {
				t.Fatal(c.Err)
			}
			want := UpdateStatusAvailable
			if edited {
				want = UpdateStatusLocallyEdited
			}
			item := jsonUpdate(svc, c)
			if c.Status != want || item.LocalChanged == nil || *item.LocalChanged != edited || item.UpstreamChanged == nil || !*item.UpstreamChanged {
				t.Fatalf("legacy baseline misread: %+v", item)
			}
			text := reviewState(c)
			if strings.Contains(text, "Local edits detected") != edited {
				t.Fatal(text)
			}
			result := svc.ApplyUpdates(context.Background(), r, false, nil)
			if result.AsError() != nil {
				t.Fatal(result.AsError())
			}
			current := scanOne(t, svc, nil, "alpha")
			if edited {
				if current.Description != "local edit" {
					t.Fatal("legacy local edits overwritten")
				}
			} else if current.Description != "version two" || current.Origin.Hash != c.RemoteHash || current.Origin.Hash == o.Hash {
				t.Fatalf("baseline was not upgraded: %+v", current)
			}
		})
	}
}

func TestTUIProfileApplyUsesReviewedState(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprintf("changed=%t", changed), func(t *testing.T) {
			svc := New(testPaths(t))
			writeSkill(t, svc.paths.LibraryDir(), "alpha", validMD("alpha", "one"))
			writeSkill(t, svc.paths.LibraryDir(), "beta", validMD("beta", "two"))
			agents := []agent.Agent{testAgent(svc.paths.Home, "claude-code", "claude/skills")}
			if err := svc.SaveProfile("work", ProfileSpec{"alpha": {"claude-code"}}); err != nil {
				t.Fatal(err)
			}
			m := newTab(svc)
			m.agents, m.profileNames, m.mode = agents, []string{"work"}, skModeProfiles
			m, cmd := m.updateProfiles(tea.KeyPressMsg{Code: tea.KeyEnter})
			m, _ = m.update(cmd())
			if changed {
				if err := svc.SaveProfile("work", ProfileSpec{"beta": {"claude-code"}}); err != nil {
					t.Fatal(err)
				}
			}
			m, cmd = m.updateConfirm(tea.KeyPressMsg{Code: 'y', Text: "y"})
			if !m.inFlight || m.mode != skModeBatchBusy {
				t.Fatal("profile apply did not block concurrent input")
			}
			if _, extra := m.update(tea.KeyPressMsg{Code: 'y', Text: "y"}); extra != nil {
				t.Fatal("duplicate apply accepted")
			}
			var result *bulkMsg
			for _, run := range cmd().(tea.BatchMsg) {
				if v, ok := run().(bulkMsg); ok {
					result = &v
				}
			}
			if result == nil {
				t.Fatal("missing completion")
			}
			if (result.result.AsError() != nil) != changed {
				t.Fatalf("unexpected apply result: %+v", result.result)
			}
			if scanOne(t, svc, agents, "alpha").States["claude-code"].On != !changed || scanOne(t, svc, agents, "beta").States["claude-code"].On {
				t.Fatal("applied a different profile from the preview")
			}
			m, _ = m.update(*result)
			if m.inFlight || m.report == nil {
				t.Fatal("completed profile apply did not show results")
			}
		})
	}
}

func TestInventoryAndProfileDistinguishDuplicateDisplayNames(t *testing.T) {
	svc := New(testPaths(t))
	for _, dir := range []string{"zeta", "alpha"} {
		writeSkill(t, svc.paths.LibraryDir(), dir, validMD("same", dir))
	}
	agents := []agent.Agent{testAgent(svc.paths.Home, "claude-code", "claude/skills")}
	for range 20 {
		items, err := svc.Scan(agents)
		if err != nil || len(items) != 2 || items[0].Dir != "alpha" || items[1].Dir != "zeta" {
			t.Fatalf("unstable duplicate-name inventory: %+v, %v", items, err)
		}
	}
	if err := svc.SaveProfile("work", ProfileSpec{"alpha": {"claude-code"}, "zeta": {"claude-code"}}); err != nil {
		t.Fatal(err)
	}
	m := newTab(svc)
	m.agents, m.profileNames = agents, []string{"work"}
	m, cmd := m.updateProfiles(tea.KeyPressMsg{Code: tea.KeyEnter})
	m, _ = m.update(cmd())
	for _, dir := range []string{"alpha", "zeta"} {
		if !strings.Contains(m.confirm.Question, dir+" (same)") {
			t.Fatalf("ambiguous confirmation: %s", m.confirm.Question)
		}
	}
}
