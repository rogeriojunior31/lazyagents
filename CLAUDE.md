# CLAUDE.md — lazyskills

TUI em Go para gerenciar **skills** e **sessões** de agentes de coding AI (Claude Code, Codex, Gemini CLI, OpenCode, Claude Desktop, Hermes Agent). Irmão focado do vultrix-tui: sem providers, sem proxy — só skills e sessions.

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
├── skill/    # biblioteca central ~/.lazyskills/skills + ativação por agente (symlink)
├── session/  # leitura read-only das sessões de todos os agentes + comando de resume
└── tui/      # Bubble Tea: app.go (root), views/, components/, styles.go, keys.go
```

Regras invioláveis:

1. **Nada fora de `internal/agent/` conhece paths ou formatos de arquivo dos CLIs.**
2. **Toda escrita passa por `fsutil.WriteAtomic`**; mexer em arquivo vivo de CLI exige `fsutil.Backup` antes.
3. **Ativação de skill = symlink** da biblioteca (`~/.lazyskills/skills/<nome>`) para o dir de skills do agente. Desativar = remover o symlink. Skill que já é dir real no agente é "local" — nunca deletar dir real ao desativar.
4. **Services não importam `tui/`; `tui/` não faz I/O direto** — sempre via services dentro de `tea.Cmd`.
5. **Erros:** `fmt.Errorf("contexto %s: %w", x, err)`. Na TUI vira toast, nunca panic.
6. **Testes nunca tocam `~/` real** — home dir injetável, `t.TempDir()`.

## Workflow

Antes de declarar concluído: `gofmt -l . && go vet ./... && go test ./... && go build ./...` tudo verde.
Commits em PT-BR: `feat(skill): install via zip`.
