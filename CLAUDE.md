# CLAUDE.md — lazyagents

TUI em Go para gerenciar **skills**, **sessões** e demais configurações (hooks, uso, providers) dos agentes de coding AI (Claude Code, Codex, Gemini CLI, OpenCode, Claude Desktop, Hermes Agent). Organizado em **módulos** — ver "Arquitetura".

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
main.go                 # só dispatch: --version, CLI (args) ou TUI
internal/
├── app/                # raiz de composição: Load (boot + migrações), Deps, Features (O registro)
├── core/               # Paths (XDG), config.yaml (Config.Section por módulo), Tilde/ExpandHome — base da pilha
├── fsutil/             # WriteAtomic, Backup, RotateBackups — TODA escrita em disco passa por aqui
├── agent/              # 1 adapter por agente + interfaces de capacidade. ÚNICO lugar que conhece paths/formatos dos CLIs
├── skill/  session/  usage/  # services de domínio (um pacote por domínio; hooks/usage/provider virão igual)
├── plugin/             # plugins externos: descoberta, processo `<bin> serve`, protocolo JSON Lines (docs/plugins.md)
├── cli/                # framework (Command/Context/Run, doctor agregador) + comandos por domínio
└── tui/
    ├── app.go          # root: splash, header, abas, ajuda, paleta — não conhece nenhuma aba concreta
    ├── module/         # contrato module.Module (+ Commander opcional)
    ├── events/         # mensagens trocadas ENTRE módulos (AgentsDetected, SkillsScanned, SessionsLoaded, TabActivated, Reload)
    ├── modules/<aba>/  # um pacote por aba: model.go, msgs.go, view.go, help.go + arquivos por assunto
    ├── modules/plugin/ # aba proxy de um plugin externo (uma instância por binário, registrada por app)
    ├── kit/            # estilos, delegate de lista, markdown, foco de painel, helpers de layout
    ├── components/     # widgets: Panel, Palette, Confirm, Toast, Splash
    └── theme/          # ÚNICO lugar com literais de cor; theme.AgentColor(id)
```

Dependências (acíclicas): `fsutil ← core ← agent ← {skill, session, plugin, …} ← {cli, tui/events} ← tui/modules/* ← tui ← app ← main`.

### Como adicionar um módulo novo (ex.: hooks)

1. **Capacidade no agente:** interface opcional em `internal/agent/<cap>.go` (ex.: `HooksHost`), implementada só pelos adapters que suportam. `agent.Adapter` **não cresce**.
2. **Service:** `internal/<dominio>/` segurando `[]agent.Adapter` + `core.Paths`, resolvendo a capacidade por type assertion (padrão de `session.Service.SessionUsage`).
3. **Aba:** `internal/tui/modules/<dominio>/` implementando `module.Module` (semântica de ponteiro: `Update(msg) tea.Cmd`). Mensagem lida por outra aba → `internal/tui/events`; o resto fica não exportado no pacote.
4. **CLI:** `internal/cli/<dominio>.go` com `XCommands(svc) []Command` e, se fizer sentido, `XChecks(svc) []Check` para o doctor.
5. **Registro:** UMA entrada em `app.Features` (`internal/app/features.go`) + o service em `app.Deps`/`LoadWith`. `tui/app.go` e `cli/cli.go` nunca são editados para isso. A ordem do registro é a ordem das abas; `Last: true` joga a aba para o fim, depois até das de plugin (é o caso de Uso, que é consulta).
6. **Config do módulo:** seção de topo `<id>:` no `config.yaml`, lida com `deps.Config.Section("<id>", &cfg)` (struct com tags yaml no próprio pacote). Nunca adicionar chave em `core.Config` — só `theme` e `libraryDir` são globais.

Plugins externos (binários em `<ConfigDir>/plugins/`) são abas/comandos/checks descobertos em runtime por `internal/plugin` + `app.Deps`; o contrato está em `docs/plugins.md`.

Regras invioláveis:

1. **Nada fora de `internal/agent/` conhece paths ou formatos de arquivo dos CLIs.**
2. **Toda escrita passa por `fsutil.WriteAtomic`**; mexer em arquivo vivo de CLI exige `fsutil.Backup` antes e preservar chaves desconhecidas. O `config.yaml` só é reescrito via `core.Config.Save` (round-trip por `yaml.Node`: comentários e seções alheias sobrevivem).
3. **Ativação de skill = symlink** da biblioteca (`<DataDir>/skills/<nome>`, DataDir = `~/.local/share/lazyagents`) para o dir de skills do agente. Desativar = remover o symlink. Skill que já é dir real no agente é "local" — nunca deletar dir real ao desativar.
4. **Services não importam `tui/`; `tui/` não faz I/O direto** — sempre via services dentro de `tea.Cmd`.
5. **Erros:** `fmt.Errorf("contexto %s: %w", x, err)`. Na TUI vira toast, nunca panic.
6. **Testes nunca tocam `~/` real** — `core.PathsIn(t.TempDir())`, home injetável.
7. **Segredos** (tokens de provider, credenciais): arquivos 0600, backups com o mesmo modo, valor sempre mascarado na TUI e no `--json` (só `--reveal` explícito na CLI mostra). Nunca ler token para exibir. Credencial de agente só pode ser materializada para autenticar uma chamada do próprio agente (hoje: `Claude.RateLimits`), dentro da função, nunca em struct exportada, log, erro ou disco. Rede só sob demanda, jamais no boot.
8. **Cores só em `theme/`**; cor de agente via `theme.AgentColor(id)`.
9. **Plugins externos só via `internal/plugin`.** O protocolo (`docs/plugins.md`, `plugin.Protocol`) só muda com bump de versão. Tudo que vem do plugin é não confiável: `view` passa por `plugin.CleanView`, manifesto é saneado, falha vira estado morto na aba — nunca panic, nunca derruba a TUI.

## Workflow

1. Trabalhe em **UMA task do BACKLOG.md por vez**, na ordem. Não inicie a próxima com a atual falhando.
2. Antes de declarar concluído: `gofmt -l . && go vet ./... && go test ./... && go build ./...` tudo verde + teste manual via tmux quando tocar UI.
3. Marcar o checkbox no BACKLOG.md e commitar. Commits em PT-BR: `feat(skill): install via zip`.
