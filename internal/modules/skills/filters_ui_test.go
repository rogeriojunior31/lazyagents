package skills

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

func inventoryTab(t *testing.T) Tab {
	t.Helper()
	m := matrixTab(t, 0, 120, 30)
	m.skills = []Skill{
		{Dir: "review-folder", Name: "Schema review", Description: "Audit database migrations", Path: "/library/review-folder", InLibrary: true, Valid: true, Origin: &Origin{Type: "git", Source: "https://github.com/example/everything-claude-code", Sub: "skills/schema"}, States: map[string]AgentState{"codex": {On: true, Managed: true}}},
		{Dir: "folder-one", Name: "Same display", Description: "Read changes", Path: "/library/folder-one", InLibrary: true, Valid: true, States: map[string]AgentState{"claude-code": {On: true, Via: "/shared"}}},
		{Dir: "folder-two", Name: "Same display", Description: "Native instructions", Path: "/local/folder-two", Valid: false, States: map[string]AgentState{"codex": {On: true, Local: true}}},
		{Dir: "lint", Name: "lint", Path: "/library/lint", InLibrary: true, Valid: true},
	}
	m.issues = map[string][]Issue{"lint": {{Field: "description", Msg: "empty"}}}
	m.updateStatus = map[string]UpdateStatus{"review-folder": UpdateStatusAvailable, "folder-one": UpdateStatusUpToDate, "lint": UpdateStatusLocallyEdited}
	m.rebuildListItems()
	return m
}

func visibleDirs(m Tab) []string {
	out := []string{}
	for _, it := range m.list.VisibleItems() {
		out = append(out, it.(skillItem).s.Dir)
	}
	return out
}

func searchInventory(m *Tab, text string) {
	m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m.Update(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	m.Update(tea.PasteMsg{Content: text})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
}

func TestInventorySearchCombinesMetadataAndStateFilters(t *testing.T) {
	m := inventoryTab(t)
	for _, query := range []string{"database", "EVERYTHING-CLAUDE-CODE", "review-folder", "skills/schema", "/library/review"} {
		searchInventory(&m, query)
		if got := visibleDirs(m); !reflect.DeepEqual(got, []string{"review-folder"}) {
			t.Fatalf("%q: %v", query, got)
		}
		if !strings.Contains(ansi.Strip(m.detailVP.View()), "Schema review") {
			t.Fatal("search detail does not match its selected row")
		}
	}
	m.filters = inventoryFilters{location: "library", agent: "codex", updates: "available"}
	dir, index := m.selection()
	m.applyFilters(dir, index)
	if got := visibleDirs(m); !reflect.DeepEqual(got, []string{"review-folder"}) {
		t.Fatal(got)
	}
	if m.Count() != 4 {
		t.Fatal("filter changed the full inventory count")
	}
	announced := m.announceCmd()().(events.SkillsScanned)
	if announced.Total != 4 || announced.ActiveByAgent["codex"] != 2 {
		t.Fatal("filter changed cross-tab activation counts")
	}
	for _, tt := range []struct {
		filter inventoryFilters
		want   []string
	}{
		{inventoryFilters{location: "library"}, []string{"review-folder", "folder-one", "lint"}},
		{inventoryFilters{location: "local"}, []string{"folder-two"}},
		{inventoryFilters{agent: "*"}, []string{"review-folder", "folder-one", "folder-two"}},
		{inventoryFilters{agent: "-"}, []string{"lint"}},
		{inventoryFilters{agent: "codex"}, []string{"review-folder", "folder-two"}},
		{inventoryFilters{agent: "claude-code"}, []string{"folder-one"}},
		{inventoryFilters{metadata: "invalid"}, []string{"folder-two", "lint"}},
		{inventoryFilters{updates: "available"}, []string{"review-folder"}},
		{inventoryFilters{updates: "edited"}, []string{"lint"}},
		{inventoryFilters{updates: "current"}, []string{"folder-one"}},
		{inventoryFilters{updates: "unknown"}, []string{"folder-two"}},
	} {
		m.Update(tea.KeyPressMsg{Code: 'F', Text: "F"})
		m.filters = tt.filter
		m.applyFilters("", 0)
		if got := visibleDirs(m); !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("%+v: %v, want %v", tt.filter, got, tt.want)
		}
	}
}

func TestInventoryFilterSelectionClearAndNoMatchesStayConsistent(t *testing.T) {
	m := inventoryTab(t)
	m.list.Select(2)
	m.refreshDetail()
	m.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight}) // library
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight}) // local
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.filters != (inventoryFilters{}) || len(visibleDirs(m)) != 4 {
		t.Fatal("cancel changed filters")
	}
	m.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if sk, _ := m.selected(); sk.Dir != "folder-two" {
		t.Fatal("state filter lost the selected directory")
	}
	m.Update(tea.KeyPressMsg{Code: 'F', Text: "F"})
	if sk, _ := m.selected(); sk.Dir != "folder-two" {
		t.Fatal("clearing chose another skill with the same display name")
	}
	if !strings.Contains(ansi.Strip(m.detailContent(100)), "outside the library") {
		t.Fatal("details describe the wrong duplicate name")
	}
	searchInventory(&m, "nothing-matches-this-query")
	if m.list.FilterValue() != "nothing-matches-this-query" || m.list.SettingFilter() || len(visibleDirs(m)) != 0 {
		t.Fatal("accepting no results discarded the query")
	}
	for _, key := range []tea.KeyPressMsg{{Code: tea.KeySpace}, {Code: 'd', Text: "d"}, {Code: tea.KeyEnter}} {
		if cmd := m.Update(key); cmd != nil || m.mode != skModeList {
			t.Fatal("no-result view acted on a hidden row")
		}
	}
	m.Update(list.FilterMatchesMsg{}) // delayed replies cannot replace the latest projection
	if len(visibleDirs(m)) != 0 {
		t.Fatal("late filter reply restored hidden rows")
	}
	m.Update(tea.KeyPressMsg{Code: 'F', Text: "F"})
	if sk, ok := m.selected(); !ok || sk.Dir != visibleDirs(m)[m.list.Index()] || m.detailName != sk.Dir {
		t.Fatal("clear left invalid selection/detail")
	}
	// Both search and state-filter surfaces must fit even when no rows remain.
	for _, size := range [][2]int{{40, 12}, {80, 24}, {120, 30}, {220, 34}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for _, filter := range []inventoryFilters{{}, {agent: "missing", metadata: "invalid", updates: "available"}} {
			m.filters = filter
			m.applyFilters("", 0)
			for _, mode := range []skMode{skModeList, skModeFilters} {
				m.mode = mode
				m.filterDraft = m.filters
				view := m.View()
				if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] {
					t.Fatalf("mode %v at %v: %dx%d\n%s", mode, size, lipgloss.Width(view), lipgloss.Height(view), ansi.Strip(view))
				}
			}
		}
	}
}

func TestInventoryFilteringUsesScannedLintWithoutReadingFilesAgain(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	good := writeSkill(t, paths.LibraryDir(), "good", "---\nname: good\ndescription: Review changes\n---\nInstructions\n")
	bad := writeSkill(t, paths.LibraryDir(), "bad", "---\nname: bad\n---\nInstructions\n")
	m := newTab(New(paths))
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.Update(m.scanCmd()())
	if len(m.inventory) != 2 || len(m.issues["bad"]) != 1 {
		t.Fatal("scan did not cache lint")
	}
	if err := os.RemoveAll(good); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(bad); err != nil {
		t.Fatal(err)
	}
	// Any metadata reread now would turn the valid good entry into an IO issue.
	for range 3 {
		m.Update(tea.KeyPressMsg{Code: 'F', Text: "F"})
		m.filters = inventoryFilters{metadata: "invalid"}
		m.applyFilters("", 0)
		if got := visibleDirs(m); !reflect.DeepEqual(got, []string{"bad"}) {
			t.Fatal("filters reread metadata instead of using the scan", got)
		}
		m.Update(tea.KeyPressMsg{Code: 'F', Text: "F"})
		searchInventory(&m, "Review changes")
		if got := visibleDirs(m); !reflect.DeepEqual(got, []string{"good"}) {
			t.Fatal(got)
		}
	}
	if _, err := os.Stat(filepath.Join(paths.DataDir, "backups")); !os.IsNotExist(err) {
		t.Fatal("filtering created backups")
	}
}

func TestInventoryFilterPanelCombinesWithSearchAndResetsDraftOnly(t *testing.T) {
	m := inventoryTab(t)
	searchInventory(&m, "Same display")
	for _, key := range []tea.KeyPressMsg{
		{Code: 'f', Text: "f"},
		{Code: tea.KeyRight}, {Code: tea.KeyRight},
		{Code: tea.KeyDown},
		{Code: tea.KeyRight}, {Code: tea.KeyRight}, {Code: tea.KeyRight}, {Code: tea.KeyRight},
		{Code: tea.KeyDown}, {Code: tea.KeyRight},
		{Code: tea.KeyDown}, {Code: tea.KeyLeft},
		{Code: tea.KeyEnter},
	} {
		m.Update(key)
	}
	wanted := inventoryFilters{location: "local", agent: "codex", metadata: "invalid", updates: "unknown"}
	if m.filters != wanted || m.list.FilterValue() != "Same display" || !reflect.DeepEqual(visibleDirs(m), []string{"folder-two"}) {
		t.Fatalf("combined picker state: %+v, %v", m.filters, visibleDirs(m))
	}
	m.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})
	m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.filters != wanted {
		t.Fatal("cancel applied the reset draft")
	}
	m.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})
	m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.filters != (inventoryFilters{}) || m.list.FilterValue() != "Same display" || !reflect.DeepEqual(visibleDirs(m), []string{"folder-one", "folder-two"}) {
		t.Fatal("reset discarded search or kept a state filter")
	}
	if sk, _ := m.selected(); sk.Dir != "folder-two" {
		t.Fatal("reset lost the still-visible directory")
	}
}

func TestUpdateFiltersDoNotTreatUnknownOrEditedChecksAsUpToDate(t *testing.T) {
	m := inventoryTab(t)
	m.filters = inventoryFilters{updates: "available"}
	m.applyFilters("", 0)
	m.Update(checkUpdatesMsg{checks: []UpdateCheck{
		{Skill: m.skills[0], Status: UpdateStatusLocallyEdited},
		{Skill: m.skills[1], Status: UpdateStatusUnknown},
	}})
	if len(visibleDirs(m)) != 0 || !strings.Contains(m.toast, "1 locally edited, 1 unknown") {
		t.Fatalf("false up-to-date evidence: %s", m.toast)
	}
	m.Update(checkUpdatesMsg{})
	if len(m.updateStatus) != 0 || len(visibleDirs(m)) != 0 {
		t.Fatal("empty check retained old evidence")
	}
	m.updateStatus = map[string]UpdateStatus{"review-folder": UpdateStatusAvailable}
	m.rebuildListItems()
	m.Update(updateDoneMsg{name: "Schema review"})
	if len(m.updateStatus) != 0 {
		t.Fatal("update left cached availability behind")
	}
	m.updateStatus = map[string]UpdateStatus{"removed": UpdateStatusAvailable}
	m.Update(scannedMsg{skills: m.skills, issues: m.issues})
	if len(m.updateStatus) != 0 {
		t.Fatal("scan retained evidence for a removed skill")
	}
	if len(visibleDirs(m)) != 0 {
		t.Fatal("refresh kept a row excluded by the current update filter")
	}
}
