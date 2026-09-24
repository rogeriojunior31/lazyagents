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
	name := flag.String("theme", "noite", "noite, garoa or jaragua")
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
	// Dados descartáveis para inspecionar e editar um script sem executar hooks.
	scriptPath := filepath.Join(paths.HooksDir(), "preview", "review.sh")
	script := "#!/bin/sh\n# Script fictício para revisão visual.\nset -eu\n\n" +
		"# A TUI lê este arquivo, sem executá-lo.\n" +
		"project=demo\nmode=review\n\n" +
		"if [ \"$mode\" = review ]; then\n  printf '%s\\n' \"Revisando $project\"\nfi\n\n" +
		"# Revise as alterações antes de salvar.\n# O original recebe um backup.\n# FIM_SCRIPT\n"
	if err := fsutil.WriteAtomic(scriptPath, []byte(script), 0o700); err != nil {
		return err
	}
	if err := hooks.New(nil, paths).Save(hooks.Hook{
		Name: "preview", Description: "Comandos fictícios para revisar navegação, leitura e edição.", Files: filepath.Dir(scriptPath),
		Hooks: []agent.Hook{
			{Event: agent.HookSessionStart, Command: "echo inicio"},
			{Event: agent.HookPreToolUse, Command: "echo revisar", Matcher: "Bash"},
			{Event: agent.HookStop, Command: fmt.Sprintf("sh %q", scriptPath), Async: true, Timeout: 120},
		}, Off: []int{1},
	}); err != nil {
		return err
	}
	a, err := app.LoadWith(paths, "preview")
	if err != nil {
		return err
	}
	defer a.Close()
	mods := a.Modules()
	var model tea.Model = tui.New(mods, nil, "preview", tui.Options{})
	agents := []agent.Agent{
		{ID: "claude-code", Name: "Claude Code", Short: "C", Installed: true, Version: "2.1", ManagedDir: filepath.Join(tmp, "claude/skills")},
		{ID: "codex", Name: "Codex", Short: "X", Installed: true, Version: "0.110", ManagedDir: filepath.Join(tmp, "codex/skills")},
		{ID: "gemini-cli", Name: "Gemini CLI", Short: "G", Installed: true, Version: "0.30", ManagedDir: filepath.Join(tmp, "gemini/skills")},
		{ID: "opencode", Name: "OpenCode", Short: "O", Installed: false},
	}
	model, _ = model.Update(events.AgentsDetected{Agents: agents})
	start := mods[0].Update(events.AgentsDetected{Agents: agents}) // só o scan de skills; sessões abaixo são fictícias
	var lib []skills.Skill
	for i, entry := range [][2]string{
		{"code-review", "Revisa diffs, encontra regressões e sugere melhorias antes do merge."},
		{"frontend-design", "Interfaces com personalidade, hierarquia visual e acessibilidade."},
		{"release-notes", "Transforma o histórico de commits em um changelog claro."},
		{"sql-tuning", "Analisa consultas, planos de execução e índices."},
		{"docker-debug", "Diagnostica containers, redes e volumes."},
	} {
		path := filepath.Join(paths.LibraryDir(), entry[0])
		data := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n# %s\n\n%s\n", entry[0], entry[1], entry[0], entry[1])
		if err := fsutil.WriteAtomic(filepath.Join(path, "SKILL.md"), []byte(data), 0o600); err != nil {
			return err
		}
		lib = append(lib, skills.Skill{Dir: entry[0], Name: entry[0], Description: entry[1], Path: path, Valid: true, InLibrary: true,
			States: map[string]skills.AgentState{"claude-code": {On: true, Managed: true}, "codex": {On: i%2 == 0, Managed: true}}})
	}
	// A aba de skills varre a biblioteca sozinha (escrita acima); o evento só
	// alimenta o agregado que a aba de agentes mostra.
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
	for i, title := range []string{"Refinar a experiência do workspace", "Revisar autenticação da API", "Preparar release 1.4", "Otimizar consultas do dashboard"} {
		sessions = append(sessions, agent.Session{ID: fmt.Sprintf("preview-%d", i), Title: title, AgentID: agents[i%3].ID, AgentName: agents[i%3].Name,
			CWD: filepath.Join(tmp, "projects", "workspace"), MTime: time.Now().Add(-time.Duration(i*45+3) * time.Minute)})
	}
	model, _ = model.Update(events.SessionsLoaded{Sessions: sessions})
	var activate tea.Cmd
	model, activate = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	for i := 0; i < page%len(mods); i++ {
		model, activate = model.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	}
	_, err = tea.NewProgram(preview{inner: model, start: tea.Batch(start, activate)}).Run()
	return err
}
