//go:build ignore

// Run with go run scripts/preview.go. All data and settings are disposable;
// nothing is read from the user's agents or configuration directories.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/app"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
	"github.com/rogeriojunior31/lazyagents/internal/modules/hooks"
	"github.com/rogeriojunior31/lazyagents/internal/modules/providers"
	"github.com/rogeriojunior31/lazyagents/internal/modules/skills"
	"github.com/rogeriojunior31/lazyagents/internal/tui"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

type preview struct {
	inner tea.Model
	start tea.Cmd
}

func (p preview) Init() tea.Cmd  { return p.start }
func (p preview) View() tea.View { return p.inner.View() }
func (p preview) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	p.inner, cmd = p.inner.Update(msg)
	return p, cmd
}

func main() {
	name := flag.String("theme", "sp-night", "theme id (sp-night, sp-night-garoa, sp-night-jaragua, dracula, nord…)")
	page := flag.Int("page", 0, "0 skills, 1 sessions, 2 providers, 3 hooks, 4 usage, 5 agents")
	flag.Parse()
	if err := run(*name, *page); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(name string, page int) error {
	if err := theme.Apply(name); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "lazyagents-preview-")
	if err != nil {
		return err
	}
	// The isolated directory is retained for inspection; its path is printed on exit.
	defer fmt.Fprintln(os.Stderr, "Preview data:", tmp)
	paths := core.PathsIn(tmp)
	// Disposable data to inspect and edit a script without running hooks.
	scriptPath := filepath.Join(paths.HooksDir(), "preview", "review.sh")
	script := "#!/bin/sh\n# Sample script for visual review.\nset -eu\n\n" +
		"# The TUI reads this file without running it.\n" +
		"project=demo\nmode=review\n\n" +
		"if [ \"$mode\" = review ]; then\n  printf '%s\\n' \"Reviewing $project\"\nfi\n\n" +
		"# Review the changes before saving.\n# The original gets a backup.\n# END_SCRIPT\n"
	if err := fsutil.WriteAtomic(scriptPath, []byte(script), 0o700); err != nil {
		return err
	}
	if err := hooks.New(nil, paths).Save(hooks.Hook{
		Name: "preview", Description: "Sample commands to review navigation, reading and editing.", Files: filepath.Dir(scriptPath),
		Hooks: []agent.Hook{
			{Event: agent.HookSessionStart, Command: "echo start"},
			{Event: agent.HookPreToolUse, Command: "echo review", Matcher: "Bash"},
			{Event: agent.HookStop, Command: fmt.Sprintf("sh %q", scriptPath), Async: true, Timeout: 120},
		}, Off: []int{1},
	}); err != nil {
		return err
	}
	if err := hooks.New(nil, paths).Save(hooks.Hook{
		Name: "format", Description: "Formats files after each edit (sample).",
		Hooks: []agent.Hook{{Event: agent.HookPostToolUse, Command: "echo format", Matcher: "Write|Edit"}},
	}); err != nil {
		return err
	}
	// Resuming a session never runs a real agent: stubs first in PATH only
	// print the command that would run.
	bin := filepath.Join(tmp, "bin")
	for _, name := range []string{"claude", "codex", "gemini"} {
		stub := "#!/bin/sh\necho \"preview: $(basename \"$0\") $* — nothing was run\"\nsleep 2\n"
		if err := fsutil.WriteAtomic(filepath.Join(bin, name), []byte(stub), 0o755); err != nil {
			return err
		}
	}
	if err := os.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH")); err != nil {
		return err
	}
	// Sample profiles only in the preview providers.json; no agent is
	// configured until someone applies one from the tab.
	for _, pr := range []agent.ProviderProfile{
		{Name: "work", BaseURL: "https://gateway.work.example/v1", Model: "claude-sonnet-5", Token: "fake-preview"},
		{Name: "local", BaseURL: "http://localhost:4000", Model: "qwen3-coder", EnvKey: "LOCAL_KEY"},
	} {
		if err := providers.New(nil, paths).Save(pr); err != nil {
			return err
		}
	}
	a, err := app.LoadWith(paths, "preview")
	if err != nil {
		return err
	}
	defer a.Close()
	mods := a.Modules()
	var model tea.Model = tui.New(mods, nil, "preview", tui.Options{})
	agents := []agent.Agent{
		{ID: "claude-code", Name: "Claude Code", Short: "C", Installed: true, Version: "2.1", ManagedDir: filepath.Join(tmp, "claude/skills"), ReadDirs: []string{filepath.Join(tmp, "claude/skills")}},
		{ID: "codex", Name: "Codex", Short: "X", Installed: true, Version: "0.110", ManagedDir: filepath.Join(tmp, "codex/skills"), ReadDirs: []string{filepath.Join(tmp, "codex/skills")}},
		{ID: "gemini-cli", Name: "Gemini CLI", Short: "G", Installed: true, Version: "0.30", ManagedDir: filepath.Join(tmp, "gemini/skills"), ReadDirs: []string{filepath.Join(tmp, "gemini/skills")}},
		{ID: "opencode", Name: "OpenCode", Short: "O", Installed: true, Version: "1.18", ManagedDir: filepath.Join(tmp, "opencode/skills"), ReadDirs: []string{filepath.Join(tmp, "opencode/skills")}},
		{ID: "pi", Name: "Pi", Short: "P", Installed: true, Version: "0.99", ManagedDir: filepath.Join(tmp, "pi/skills"), ReadDirs: []string{filepath.Join(tmp, "pi/skills")}},
		{ID: "crush", Name: "Crush", Short: "R", Installed: true, Version: "0.97", ManagedDir: filepath.Join(tmp, "crush/skills"), ReadDirs: []string{filepath.Join(tmp, "crush/skills")}},
		{ID: "hermes", Name: "Hermes Agent", Short: "H", Installed: true, Version: "1.4", ManagedDir: filepath.Join(tmp, "hermes/skills"), ReadDirs: []string{filepath.Join(tmp, "hermes/skills")}},
		{ID: "claude-desktop", Name: "Claude Desktop", Short: "D", Installed: false},
	}
	model, _ = model.Update(events.AgentsDetected{Agents: agents})
	start := mods[0].Update(events.AgentsDetected{Agents: agents}) // only the skills scan; sessions below are samples
	var lib []skills.Skill
	for i, entry := range [][2]string{
		{"code-review", "Reviews diffs, finds regressions and suggests improvements before merge."},
		{"frontend-design", "Interfaces with personality, visual hierarchy and accessibility."},
		{"release-notes", "Turns the commit history into a clear changelog."},
		{"sql-tuning", "Analyzes queries, execution plans and indexes."},
		{"docker-debug", "Diagnoses containers, networks and volumes."},
	} {
		path := filepath.Join(paths.LibraryDir(), entry[0])
		data := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n# %s\n\n%s\n", entry[0], entry[1], entry[0], entry[1])
		if err := fsutil.WriteAtomic(filepath.Join(path, "SKILL.md"), []byte(data), 0o600); err != nil {
			return err
		}
		lib = append(lib, skills.Skill{Dir: entry[0], Name: entry[0], Description: entry[1], Path: path, Valid: true, InLibrary: true,
			States: map[string]skills.AgentState{"claude-code": {On: true, Managed: true}, "codex": {On: i%2 == 0, Managed: true},
				"pi": {On: i%3 == 0, Managed: true}, "crush": {On: i < 2, Managed: true}}})
		// Real activations (symlinks), so the matrix shows the same state.
		for _, ag := range agents {
			if st := lib[i].States[ag.ID]; st.On {
				if err := os.MkdirAll(ag.ManagedDir, 0o755); err != nil {
					return err
				}
				if err := os.Symlink(path, filepath.Join(ag.ManagedDir, entry[0])); err != nil {
					return err
				}
			}
		}
	}
	// A local skill (real dir in the agent) for the ▪ marker.
	local := filepath.Join(agents[2].ManagedDir, "local-notes", "SKILL.md")
	if err := fsutil.WriteAtomic(local, []byte("---\nname: local-notes\ndescription: Skill created directly in Gemini, outside the library.\n---\n"), 0o600); err != nil {
		return err
	}
	// The skills tab scans the library itself (written above); the event only
	// feeds the aggregate the agents tab shows.
	active := map[string]int{}
	for _, sk := range lib {
		for id, st := range sk.States {
			if st.On {
				active[id]++
			}
		}
	}
	model, _ = model.Update(events.SkillsScanned{ActiveByAgent: active, Total: len(lib)})
	var sessions []agent.Session
	for i, sess := range [][2]string{
		{"Polish the workspace experience", "workspace"}, {"Review API authentication", "api"},
		{"Prepare release 1.4", "workspace"}, {"Optimize dashboard queries", "dashboard"},
		{"Fix order list pagination", "api"}, {"Write checkout integration tests", "shop"},
		{"Move CI to dependency caching", "workspace"}, {"Investigate the worker memory leak", "api"},
		{"Document the deploy flow", "shop"}, {"Tune the dark theme contrast", "workspace"},
	} {
		// "dashboard" has no folder: resume falls back to home and the detail warns.
		cwd := filepath.Join(tmp, "projects", sess[1])
		if sess[1] != "dashboard" {
			if err := os.MkdirAll(cwd, 0o755); err != nil {
				return err
			}
		}
		sessions = append(sessions, agent.Session{ID: fmt.Sprintf("preview-%d", i), Title: sess[0], AgentID: agents[i%7].ID, AgentName: agents[i%7].Name,
			CWD: cwd, MTime: time.Now().Add(-time.Duration(i*i*45+3) * time.Minute)})
	}
	sessions[1].Alias = "API auth"
	model, _ = model.Update(events.SessionsLoaded{Sessions: sessions})
	var activate tea.Cmd
	model, activate = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	for i := 0; i < page%len(mods); i++ {
		model, activate = model.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	}
	_, err = tea.NewProgram(preview{inner: model, start: tea.Batch(start, activate)}).Run()
	return err
}
