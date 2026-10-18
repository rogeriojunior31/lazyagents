package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestSourcesBrowseDoesNotScanAndRecordsPerSourceAttempts(t *testing.T) {
	svc := New(testPaths(t))
	r := repository{Name: "Offline fixture", URL: "https://example.invalid/repo.git"}
	if err := svc.saveRepository(r, nil, false); err != nil {
		t.Fatal(err)
	}
	m := newTab(svc)
	m.width, m.height = 80, 24
	t.Setenv("PATH", "")
	m, cmd := m.update(openSourcesMsg{})
	m, _ = m.update(cmd())
	if m.mode != skModeSources || m.inFlight || !strings.Contains(m.View(), "not scanned in this session") {
		t.Fatal("opening repositories scanned a source")
	}
	m, cmd = m.updateSources(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.inFlight || m.mode != skModeSourceBusy {
		t.Fatal("scan did not block duplicate requests")
	}
	if next := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); next != nil {
		t.Fatal("busy scan duplicated a source request")
	}
	for _, c := range cmd().(tea.BatchMsg) {
		if msg, ok := c().(discoverMsg); ok {
			m, _ = m.update(msg)
		}
	}
	status := m.sources.status[r]
	if status.at.IsZero() || status.failure == "" || m.mode != skModeSources {
		t.Fatalf("scan failure lost: %+v", status)
	}
	for range 20 {
		m, _ = m.updateSources(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModShift})
	}
	if !strings.Contains(m.View(), "git in PATH") {
		t.Fatalf("source error is not readable: %s", ansi.Strip(m.View()))
	}
	m, _ = m.updateSources(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m, _ = m.updateConfirm(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.mode != skModeSources {
		t.Fatal("default No did not return to source list")
	}
	if items, _ := svc.repositories(); len(items) != 1 {
		t.Fatal("cancel removed the source")
	}
	data, _ := os.ReadFile(svc.paths.ConfigPath())
	if strings.Contains(string(data), "lastScan") || strings.Contains(string(data), "failure") {
		t.Fatal("session status leaked into descriptors")
	}
}

func TestNamedSourcePickerRejectsAmbiguousDestinationsBeforeReview(t *testing.T) {
	m := newTab(New(testPaths(t)))
	m.width, m.height = 80, 24
	r := repository{Name: "Fixture", URL: "https://example.test/repo.git"}
	found := []Found{{Name: "same", Rel: "skills/one", Valid: true, Collision: true}, {Name: "same", Rel: "skills/two", Valid: true, Collision: true}}
	m, _ = m.update(discoverMsg{found: found, origin: Origin{Type: "git", Source: r.URL, Ref: "stable", Revision: strings.Repeat("a", 40), ExactPath: true}, repository: &r})
	for _, key := range []tea.KeyPressMsg{{Code: tea.KeyEnter}, {Code: tea.KeyTab}} {
		m, cmd := m.updatePick(key)
		if m.picker.destinations || cmd != nil || m.mode != skModePick {
			t.Fatal("ambiguous selection reached destinations")
		}
	}
	if v := ansi.Strip(m.View()); !strings.Contains(v, "skills/one") || !strings.Contains(v, "skills/two") || !strings.Contains(v, "Name collision") {
		t.Fatalf("paths/collision hidden: %s", v)
	}
	m, _ = m.updatePick(tea.KeyPressMsg{Code: tea.KeySpace})
	m, _ = m.updatePick(tea.KeyPressMsg{Code: tea.KeyEnter})
	m, _ = m.updatePick(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.ckind != confirmKindInstall || !strings.Contains(m.confirm.Question, "same (skills/two)") || !strings.Contains(m.confirm.Question, "Ref: stable") {
		t.Fatalf("review lacks identity: %s", m.confirm.Question)
	}
	m, _ = m.updateConfirm(tea.KeyPressMsg{Code: tea.KeyEscape})
	m, _ = m.updatePick(tea.KeyPressMsg{Code: tea.KeyEscape})
	m, cmd := m.updatePick(tea.KeyPressMsg{Code: tea.KeyEscape})
	cmd()
	if m.mode != skModeSources {
		t.Fatal("cancel did not return to repository list")
	}
}

func TestSourceFormsAndListsFitSmallTerminals(t *testing.T) {
	for _, size := range [][2]int{{36, 11}, {76, 18}, {120, 30}} {
		m := newTab(New(testPaths(t)))
		m.width, m.height = size[0], size[1]
		m.sources.status = map[repository]sourceScanStatus{}
		for i := range 30 {
			r := repository{Name: fmt.Sprintf("source-%02d", i), URL: "https://example.test/" + strings.Repeat("long-repository/", 12), Ref: "release/v1", Subdir: "catalog/skills"}
			m.sources.items = append(m.sources.items, r)
			m.sources.status[r] = sourceScanStatus{at: time.Now(), failure: strings.Repeat("path failure ", 30) + "ERROR_END"}
		}
		m.mode = skModeSources
		for i := range m.sources.items {
			m.sources.cursor = i
			v := m.View()
			if lipgloss.Width(v) > size[0] || lipgloss.Height(v) > size[1] || !strings.Contains(ansi.Strip(v), m.sources.items[i].Name) {
				t.Fatalf("source list %v: %dx%d\n%s", size, lipgloss.Width(v), lipgloss.Height(v), ansi.Strip(v))
			}
		}
		for range 200 {
			m, _ = m.updateSources(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModShift})
		}
		if !strings.Contains(m.View(), "ERROR_END") {
			t.Fatal("long source error cannot be read")
		}
		for field := range 4 {
			m.sourceForm(nil)
			m.sources.field = field
			m.input.SetValue("fixture")
			v := m.View()
			if lipgloss.Width(v) > size[0] || lipgloss.Height(v) > size[1] {
				t.Fatalf("source form %v field %d: %dx%d", size, field, lipgloss.Width(v), lipgloss.Height(v))
			}
			m.sources.err = "Source subfolder must stay inside the repository; use forward slashes"
			v = m.View()
			if lipgloss.Width(v) > size[0] || lipgloss.Height(v) > size[1] || !strings.Contains(v, "esc cancel") {
				t.Fatalf("source validation form %v: %dx%d", size, lipgloss.Width(v), lipgloss.Height(v))
			}
		}
	}
}

func TestSourceFormSavesWithoutDiscoveringAndCancelWritesNothing(t *testing.T) {
	m := newTab(New(testPaths(t)))
	m.width, m.height = 80, 24
	m.sourceForm(nil)
	for _, value := range []string{"Fixture", "example/repo", "", "skills"} {
		m.input.SetValue(value)
		var cmd tea.Cmd
		m, cmd = m.updateSourceForm(tea.KeyPressMsg{Code: tea.KeyEnter})
		if m.mode == skModeSourceBusy {
			for _, c := range cmd().(tea.BatchMsg) {
				if msg, ok := c().(sourceSavedMsg); ok {
					m, cmd = m.update(msg)
					if cmd != nil {
						m, _ = m.update(cmd())
					}
				}
			}
		}
	}
	if len(m.sources.items) != 1 || m.sources.items[0].Subdir != "skills" || len(m.sources.status) != 0 {
		t.Fatal("save scanned or lost the descriptor")
	}
	before, _ := os.ReadFile(m.svc.paths.ConfigPath())
	m.sourceForm(nil)
	m.input.SetValue("Cancelled")
	m, _ = m.updateSourceForm(tea.KeyPressMsg{Code: tea.KeyEscape})
	after, _ := os.ReadFile(m.svc.paths.ConfigPath())
	if string(before) != string(after) {
		t.Fatal("cancel changed source config")
	}
	if _, err := os.Stat(filepath.Join(m.svc.paths.DataDir, "skills")); !os.IsNotExist(err) {
		t.Fatal("adding/cancelling wrote library files")
	}
}
