package skills

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

func reviewFixture(t *testing.T) (*Service, string, func(...string), []Skill) {
	t.Helper()
	repo := t.TempDir()
	git := makeGitRepo(t, repo)
	writeSkill(t, repo, "alpha", validMD("alpha", "version one"))
	writeSkill(t, repo, "beta", validMD("beta", "version one"))
	git("add", ".")
	git("commit", "-m", "one")
	svc := New(testPaths(t))
	installGitSkill(t, svc, localRepositoryURL(repo), "alpha", nil)
	for _, name := range []string{"alpha", "beta"} {
		writeSkill(t, repo, name, validMD(name, "version two"))
	}
	git("add", ".")
	git("commit", "-m", "two")
	skills, err := svc.Scan(nil)
	if err != nil {
		t.Fatal(err)
	}
	return svc, repo, git, skills
}

func TestReviewedUpdateAppliesExactStagingWithBackupAndMetadata(t *testing.T) {
	svc, repo, git, skills := reviewFixture(t)
	lib := filepath.Join(svc.paths.LibraryDir(), "alpha")
	if err := os.WriteFile(filepath.Join(lib, "local.txt"), []byte("local change"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "alpha", "new.txt"), []byte("upstream addition"), 0o600); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-m", "extra file")
	r, err := svc.ReviewUpdates(context.Background(), skills, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	c := r.Checks[0]
	if c.Status != UpdateStatusLocallyEdited || !strings.Contains(c.Diff, "-local change") || !strings.Contains(c.Diff, "+upstream addition") || !strings.Contains(strings.Join(c.Files, "\n"), "Modified: SKILL.md") {
		t.Fatalf("incomplete diff: %+v", c)
	}
	if text := reviewState(c); !strings.Contains(text, "Local edits detected") || !strings.Contains(text, "Upstream content changed") {
		t.Fatal(text)
	}
	writeSkill(t, repo, "alpha", validMD("alpha", "unreviewed version three"))
	git("add", ".")
	git("commit", "-m", "three")
	one := &UpdateReview{Checks: []UpdateCheck{c}}
	if result := svc.ApplyUpdates(context.Background(), one, true, nil); result.AsError() != nil {
		t.Fatal(result.AsError())
	}
	if sk := scanOne(t, svc, nil, "alpha"); sk.Description != "version two" || sk.Origin.Revision != c.Revision || sk.Origin.Hash != c.RemoteHash {
		t.Fatalf("unreviewed revision installed: %+v", sk)
	}
	backups, err := svc.ListBackups()
	if err != nil || len(backups) != 1 {
		t.Fatalf("backup: %v %v", backups, err)
	}
	if err := svc.Restore(backups[0]); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(lib, "local.txt"))
	if err != nil || string(data) != "local change" {
		t.Fatalf("local edits absent from archive: %s %v", data, err)
	}
	r.Close()
	for _, root := range r.roots {
		if _, err := os.Stat(root); !os.IsNotExist(err) {
			t.Fatalf("staging leaked: %s", root)
		}
	}
}

func TestBulkReviewProtectsEditsAndStaleContent(t *testing.T) {
	for _, change := range []string{"already edited", "missing baseline", "edit after review", "origin after review", "staged source after review"} {
		t.Run(change, func(t *testing.T) {
			svc, _, _, skills := reviewFixture(t)
			lib := filepath.Join(svc.paths.LibraryDir(), "alpha")
			if change == "already edited" {
				writeSkill(t, svc.paths.LibraryDir(), "alpha", validMD("alpha", "local edit"))
			}
			if change == "missing baseline" {
				o := *readOrigin(lib)
				o.Hash = ""
				if err := writeOrigin(lib, o); err != nil {
					t.Fatal(err)
				}
				skills[0].Origin = &o
			}
			r, err := svc.ReviewUpdates(context.Background(), skills, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			switch change {
			case "edit after review":
				writeSkill(t, svc.paths.LibraryDir(), "alpha", validMD("alpha", "concurrent local edit"))
			case "origin after review":
				o := *readOrigin(lib)
				o.Ref = "another"
				if err := writeOrigin(lib, o); err != nil {
					t.Fatal(err)
				}
			case "staged source after review":
				if err := os.WriteFile(filepath.Join(r.Checks[0].source, "SKILL.md"), []byte("tampered"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := hashDir(lib)
			result := svc.ApplyUpdates(context.Background(), r, false, nil)
			if installTarget(t, result, "alpha / library").Outcome == core.Succeeded {
				t.Fatalf("unsafe bulk replacement: %+v", result)
			}
			after, _ := hashDir(lib)
			if before != after || scanOne(t, svc, nil, "beta").Description != "version two" {
				t.Fatalf("wrong scope: %+v", result)
			}
			backups, _ := svc.ListBackups()
			backups = filterBackups(backups, "alpha")
			if len(backups) != 0 {
				t.Fatal("unsafe item was backed up/applied")
			}
		})
	}
}

func TestCancelBetweenItemsAndBeforeAtomicSwap(t *testing.T) {
	svc, _, _, skills := reviewFixture(t)
	r, err := svc.ReviewUpdates(context.Background(), skills, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := svc.ApplyUpdates(ctx, r, false, func(label string) {
		if strings.Contains(label, "2/2") {
			cancel()
		}
	})
	if installTarget(t, result, "alpha / library").Outcome != core.Succeeded || installTarget(t, result, "beta / library").Outcome == core.Succeeded || scanOne(t, svc, nil, "beta").Description != "version one" {
		t.Fatalf("cancellation replaced a later item: %+v", result)
	}
	dst, src := t.TempDir(), t.TempDir()
	_ = os.WriteFile(filepath.Join(dst, "old"), []byte("old"), 0o600)
	_ = os.WriteFile(filepath.Join(src, "new"), []byte("new"), 0o600)
	ctx2, stop := context.WithCancel(context.Background())
	defer stop()
	err = replaceDirContext(ctx2, src, dst, nil, func() error { stop(); return ctx2.Err() })
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(dst); len(entries) != 1 || entries[0].Name() != "old" {
		t.Fatal("cancelled staging touched the live directory")
	}
	stages, _ := filepath.Glob(filepath.Join(filepath.Dir(dst), ".update-*"))
	if len(stages) > 0 {
		t.Fatalf("stages leaked: %v", stages)
	}
}

func TestDiffIsBoundedAndDoesNotEmitTerminalControls(t *testing.T) {
	old, next := t.TempDir(), t.TempDir()
	_ = os.WriteFile(filepath.Join(old, "removed.txt"), []byte("remove"), 0o600)
	_ = os.WriteFile(filepath.Join(next, "binary"), []byte{0, 1, 2}, 0o600)
	_ = os.WriteFile(filepath.Join(next, "large"), []byte(strings.Repeat("large", 3000)), 0o600)
	_ = os.WriteFile(filepath.Join(next, "text"), []byte("\x1b[2Junsafe\r\n"+strings.Repeat("long line\n", 10000)), 0o600)
	_ = os.WriteFile(filepath.Join(next, "control"), []byte("\x1b[2Junsafe\r\n"), 0o600)
	_ = os.WriteFile(filepath.Join(next, ".secret"), []byte("must not show"), 0o600)
	files, diff, err := skillDiff(context.Background(), old, next)
	if err != nil || len(files) != 5 || !strings.Contains(diff, "content diff omitted") || !strings.Contains(diff, `\u001b`) || strings.ContainsAny(diff, "\x1b\r") || len(diff) > 32<<10 || strings.Contains(diff, "must not show") {
		t.Fatalf("unbounded/unsafe diff: %v %v %q", files, err, diff)
	}
}

func TestReviewUIRequiresReviewForAllUpdateEntrypoints(t *testing.T) {
	svc, _, _, skills := reviewFixture(t)
	for _, entry := range []string{"u", "U", "batch"} {
		t.Run(entry, func(t *testing.T) {
			m := newTab(svc)
			m.skills = skills
			m.width, m.height = 76, 20
			m.rebuildListItems()
			var cmd tea.Cmd
			if entry == "batch" {
				m.marked = map[string]bool{"alpha": true}
				m.openBatch()
				m.batch.plan.action = batchUpdate
				m, cmd = m.updateBatch(tea.KeyPressMsg{Code: tea.KeyEnter})
			} else {
				m, cmd = m.updateList(tea.KeyPressMsg{Code: rune(entry[0]), Text: entry})
			}
			if m.mode != skModeUpdateBusy || !m.Capturing() {
				t.Fatal("update bypassed review")
			}
			if _, dup := m.update(tea.KeyPressMsg{Code: 'U', Text: "U"}); dup != nil {
				t.Fatal("duplicated request")
			}
			for _, c := range cmd().(tea.BatchMsg) {
				if msg, ok := c().(reviewReadyMsg); ok {
					m, _ = m.update(msg)
				}
			}
			if m.mode != skModeUpdateReview || m.review == nil {
				t.Fatal("review did not open")
			}
			defer m.review.Close()
			for _, size := range [][2]int{{36, 12}, {76, 20}, {120, 30}} {
				m.width, m.height = size[0], size[1]
				v := ansi.Strip(m.View())
				if lipgloss.Width(v) > m.width || lipgloss.Height(v) > m.height || !strings.Contains(v, "esc cancel") {
					t.Fatalf("review does not fit %v: %s", size, v)
				}
				for range 100 {
					m, _ = m.updateReview(tea.KeyPressMsg{Code: tea.KeyDown})
				}
				if !strings.Contains(m.reviewViewport().View(), "+description: version two") {
					t.Fatalf("diff unreachable: %s", m.reviewViewport().View())
				}
			}
			m, _ = m.updateReview(tea.KeyPressMsg{Code: tea.KeyEnter})
			if m.ckind != confirmKindReviewedUpdate || !strings.Contains(m.confirm.Question, "alpha") {
				t.Fatal("confirmation lost reviewed scope")
			}
			m, _ = m.updateConfirm(tea.KeyPressMsg{Code: tea.KeyEnter})
			if m.mode != skModeUpdateReview {
				t.Fatal("default No lost review")
			}
			r := m.review
			m, cmd = m.updateReview(tea.KeyPressMsg{Code: tea.KeyEscape})
			cmd()
			if m.review != nil || m.mode != skModeList {
				t.Fatal("cancel retained review")
			}
			for _, root := range r.roots {
				if _, err := os.Stat(root); !os.IsNotExist(err) {
					t.Fatal("dismiss leaked staging")
				}
			}
			if scanOne(t, svc, nil, "alpha").Description != "version one" {
				t.Fatal("review wrote library")
			}
		})
	}
}

func TestCancelSourceCloneReapsChildrenAndCleansStaging(t *testing.T) {
	fixture, bin := t.TempDir(), t.TempDir()
	exe := "git"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	build := exec.Command("go", "build", "-o", filepath.Join(bin, exe), "./testdata/sourcegit")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("fixture build: %s %v", out, err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SOURCEGIT_FIXTURE", fixture)
	for _, operation := range []string{"discover", "repository", "review"} {
		t.Run(operation, func(t *testing.T) {
			_ = os.Remove(filepath.Join(fixture, "heartbeat"))
			_ = os.Remove(filepath.Join(fixture, "staging"))
			m := newTab(New(testPaths(t)))
			m.width, m.height = 76, 20
			spin := m.beginSource("Cloning source…", skModeSourceBusy, skModeSources)
			_ = spin
			op := m.sourceOp
			var cmd tea.Cmd
			switch operation {
			case "discover":
				cmd = m.discoverCmd("https://example.invalid/repo")
			case "repository":
				cmd = m.repositoryCmd(repository{Name: "fixture", URL: "https://example.invalid/repo"})
			case "review":
				cmd = op.command(func(ctx context.Context, notify func(string)) tea.Msg {
					r, err := m.svc.ReviewUpdates(ctx, []Skill{{Dir: "x", InLibrary: true, Origin: &Origin{Type: "git", Source: "https://example.invalid/repo"}}}, notify)
					return reviewReadyMsg{r, err, false, op}
				})
			}
			done := make(chan tea.Msg, 1)
			go func() { done <- cmd() }()
			deadline := time.After(10 * time.Second)
			for {
				if _, err := os.Stat(filepath.Join(fixture, "heartbeat")); err == nil {
					break
				}
				select {
				case <-deadline:
					op.cancel()
					t.Fatal("fixture did not start")
				default:
					time.Sleep(10 * time.Millisecond)
				}
			}
			pidBytes, _ := os.ReadFile(filepath.Join(fixture, "pid"))
			pid, _ := strconv.Atoi(string(pidBytes))
			t.Cleanup(func() {
				if pid > 0 {
					if p, err := os.FindProcess(pid); err == nil {
						_ = p.Kill()
					}
				}
			})
			m, _ = m.update(tea.KeyPressMsg{Code: tea.KeyEscape})
			if !m.inFlight || !strings.Contains(m.toast, "Cancelling") {
				t.Fatal("cancel did not wait for worker")
			}
			var msg tea.Msg
			select {
			case msg = <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("cancel left worker running")
			}
			m, _ = m.update(msg)
			if m.inFlight || m.mode == skModePick || m.mode == skModeUpdateReview {
				t.Fatal("cancel opened partial results")
			}
			stage, _ := os.ReadFile(filepath.Join(fixture, "staging"))
			if _, err := os.Stat(string(stage)); !os.IsNotExist(err) {
				t.Fatalf("staging leaked: %s %v", stage, err)
			}
			heartbeat, _ := os.ReadFile(filepath.Join(fixture, "heartbeat"))
			time.Sleep(150 * time.Millisecond)
			after, _ := os.ReadFile(filepath.Join(fixture, "heartbeat"))
			if !reflect.DeepEqual(heartbeat, after) {
				t.Fatal("clone child survived cancellation")
			}
		})
	}
}

func TestLateCancelAndClosingPendingOperationsCleanStaging(t *testing.T) {
	svc, _, _, skills := reviewFixture(t)
	m := newTab(svc)
	m.beginSource("discover", skModeInstallBusy, skModeList)
	cmd := m.discoverCmd(skills[0].Origin.Source)
	msg := cmd().(discoverMsg)
	if msg.cleanup == "" {
		t.Fatal("fixture has no staging")
	}
	m, _ = m.update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m, cleanup := m.update(msg)
	cleanup()
	if _, err := os.Stat(msg.cleanup); !os.IsNotExist(err) || m.mode != skModeList {
		t.Fatal("late cancellation leaked discovery staging")
	}
	r, err := svc.ReviewUpdates(context.Background(), skills, nil)
	if err != nil {
		t.Fatal(err)
	}
	m.review, m.reviewSingle = r, false
	m, cmd = m.applyUpdateReview()
	// Close before the command starts, then run its cancelled work.
	m.sourceOp.close()
	for _, root := range r.roots {
		if _, err := os.Stat(root); !os.IsNotExist(err) {
			t.Fatal("pending apply leaked staging on close")
		}
	}
	for _, c := range cmd().(tea.BatchMsg) {
		if msg, ok := c().(reviewAppliedMsg); ok {
			m, _ = m.update(msg)
			if !errors.Is(m.report.Result.Err, context.Canceled) {
				t.Fatal("pending apply lost cancellation")
			}
		}
	}
	if scanOne(t, svc, nil, "alpha").Description != "version one" {
		t.Fatal("closed pending operation changed library")
	}
}

func TestApplyReviewUIReportsSuccessAndNeedsFreshReviewForRetry(t *testing.T) {
	svc, _, _, skills := reviewFixture(t)
	m := newTab(svc)
	m.width, m.height = 76, 20
	r, err := svc.ReviewUpdates(context.Background(), skills[:1], nil)
	if err != nil {
		t.Fatal(err)
	}
	m.review, m.reviewSingle, m.mode = r, true, skModeUpdateReview
	m, _ = m.updateReview(tea.KeyPressMsg{Code: tea.KeyEnter})
	m, cmd := m.updateConfirm(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if m.mode != skModeUpdateBusy || m.review != nil {
		t.Fatal("confirmation did not transfer staging ownership")
	}
	for _, c := range cmd().(tea.BatchMsg) {
		if msg, ok := c().(reviewAppliedMsg); ok {
			m, _ = m.update(msg)
		}
	}
	if m.report == nil || m.report.Result.AsError() != nil || m.report.Retry != nil || m.sourceOp != nil || scanOne(t, svc, nil, "alpha").Description != "version two" {
		t.Fatalf("apply did not finish: %+v", m.report)
	}
	for _, root := range r.roots {
		if _, err := os.Stat(root); !os.IsNotExist(err) {
			t.Fatal("applied review leaked staging")
		}
	}
}

func TestReviewedBulkSkipsIneligibleEntries(t *testing.T) {
	svc, _, _, skills := reviewFixture(t)
	r, err := svc.ReviewUpdates(context.Background(), []Skill{
		{Dir: "local", InLibrary: false},
		{Dir: "manual", InLibrary: true},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	result := svc.ApplyUpdates(context.Background(), r, false, nil)
	for _, target := range result.Targets {
		if target.Outcome != core.Skipped {
			t.Fatalf("ineligible entry was treated as a failure: %+v", target)
		}
	}
	for _, c := range r.Checks {
		if !strings.HasPrefix(reviewState(c), "Skipped:") {
			t.Fatal("review mislabels a protected entry")
		}
	}
	t.Run("external library link", func(t *testing.T) {
		lib := filepath.Join(svc.paths.LibraryDir(), skills[0].Dir)
		if err := os.RemoveAll(lib); err != nil {
			t.Fatal(err)
		}
		outside := t.TempDir()
		mustSymlink(t, outside, lib)
		review, err := svc.ReviewUpdates(context.Background(), skills[:1], nil)
		if err != nil {
			t.Fatal(err)
		}
		defer review.Close()
		if outcome := svc.ApplyUpdates(context.Background(), review, false, nil).Targets[0]; outcome.Outcome != core.Skipped {
			t.Fatalf("external link was not skipped: %+v", outcome)
		}
	})
}
