# CLAUDE.md — lazyagents

TUI em Go para gerenciar **skills**, **sessões** e demais configurações (hooks, uso, providers) dos agentes de coding AI (Claude Code, Codex, Gemini CLI, OpenCode, Claude Desktop, Hermes Agent). Sucessor do lazyskills; organizado em **módulos** — ver "Arquitetura".

## Stack (NÃO desviar)

- Go 1.26+ (módulo único, binário único)
- **Bubble Tea v2** — import `charm.land/bubbletea/v2` (NÃO `github.com/charmbracelet/bubbletea`)
- **Bubbles v2** — `charm.land/bubbles/v2/...`
- **Lip Gloss v2** — `charm.land/lipgloss/v2`
- `gopkg.in/yaml.v3` (frontmatter de SKILL.md)
- Instalação via GitHub: `git clone --depth 1` (shell out), sem lib de git
- Zip: stdlib `archive/zip`

## ⚠️ Bubble Tea v2 — regras de API (você foi treinado majoritariamente na v1)

| v1 (PROIBIDO) | v2 (CORRETO) |
|---|---|
| `import tea "github.com/charmbracelet/bubbletea"` | `import tea "charm.land/bubbletea/v2"` |
| `func (m model) View() string` | `func (m model) View() tea.View` |
| `case tea.KeyMsg:` | `case tea.KeyPressMsg:` |
| `tea.WithAltScreen()` | campo declarativo no `tea.View` retornado |
| tecla espaço `" "` | tecla nomeada `"space"` |

Dúvida de API v2 → https://github.com/charmbracelet/bubbletea/blob/main/UPGRADE_GUIDE_V2.md

## Arquitetura

```
internal/
├── fsutil/   # WriteAtomic, Backup, RotateBackups — TODA escrita em disco passa por aqui
├── agent/    # 1 adapter por agente de código. ÚNICO lugar que conhece paths/formatos dos CLIs
├── skill/    # biblioteca central <DataDir>/skills + ativação por agente (symlink)
├── session/  # leitura read-only das sessões de todos os agentes + comando de resume
└── tui/      # Bubble Tea: app.go (root), views/, components/, styles.go, keys.go
```

Regras invioláveis:

1. **Nada fora de `internal/agent/` conhece paths ou formatos de arquivo dos CLIs.**
2. **Toda escrita passa por `fsutil.WriteAtomic`**; mexer em arquivo vivo de CLI exige `fsutil.Backup` antes.
3. **Ativação de skill = symlink** da biblioteca (`<DataDir>/skills/<nome>`, DataDir = `~/.local/share/lazyagents`) para o dir de skills do agente. Desativar = remover o symlink. Skill que já é dir real no agente é "local" — nunca deletar dir real ao desativar.
4. **Services não importam `tui/`; `tui/` não faz I/O direto** — sempre via services dentro de `tea.Cmd`.
5. **Erros:** `fmt.Errorf("contexto %s: %w", x, err)`. Na TUI vira toast, nunca panic.
6. **Testes nunca tocam `~/` real** — home dir injetável, `t.TempDir()`.

## Workflow

1. Trabalhe em **UMA task do BACKLOG.md por vez**, na ordem. Não inicie a próxima com a atual falhando.
2. Antes de declarar concluído: `gofmt -l . && go vet ./... && go test ./... && go build ./...` tudo verde + teste manual via tmux quando tocar UI.
3. Marcar o checkbox no BACKLOG.md e commitar. Commits em PT-BR: `feat(skill): install via zip`.
