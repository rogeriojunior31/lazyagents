# BACKLOG — lazyagents

Planejamento em ordem de execução. **Uma task por vez**: não iniciar a próxima com a atual falhando.

## Como trabalhar (toda task)

1. Seguir o `CLAUDE.md`, em especial "Como adicionar um módulo novo" e as regras invioláveis.
2. Verificar: `gofmt -l . && go vet ./... && go test ./... && go build ./...` tudo verde.
3. Teste manual na TUI via tmux quando a task tocar UI.
4. Marcar o checkbox aqui e commitar: `feat(escopo): descrição` em PT-BR.

Legenda: **toca** = arquivos/pacotes previstos · **aceite** = critérios verificáveis · **verificar** = confirmar formato na doc/código do agente antes de codar, nunca adivinhar.

---

## M0 — Fundação: config.yaml, seções por módulo e plugins externos

Antes dos módulos novos: config extensível sem editar `core`, e um caminho para abas vindas de fora do binário.

### M0.1 — config.yaml com round-trip de comentários + migração
- [x] `core.Config` lido/escrito via `yaml.Node` (comentários e chaves alheias sobrevivem), `Config.Section(id, &out)` para a seção de topo de cada módulo/plugin, `core.MigrateConfig` (config.json → config.yaml + `.migrated`) chamado em `app.LoadWith`; avisos de boot em `Deps.Notices`.
- **Toca:** `internal/core/{config,paths}.go`, `internal/app/app.go`, `main.go`, `internal/skill/ops.go` (fix: `migrate-library` apagava o `theme`).
- **Aceite:** round-trip preserva comentários; migração idempotente e sem perda; `TestMigrateLibrary_KeepsTheme`; zero dependência nova.

### M0.2 — Service `internal/plugin` e protocolo v1
- [ ] Descoberta em `<ConfigDir>/plugins/*` (executável, nome `^[a-z0-9][a-z0-9_-]{0,31}$`), `Service.Start` (spawn `<bin> serve`, `init`, manifesto em até 3 s), `Proc.Send/Events/Close`, `Run` (pass-through), `CleanView` (só SGR passa). Protocolo JSON Lines documentado em `docs/plugins.md`.
- **Toca:** `internal/plugin/{proto,plugin}.go`, `internal/core/paths.go` (`PluginsDir`), `docs/plugins.md`, `examples/plugins/hello`.
- **Aceite:** fixtures `#!/bin/sh` (pulam sem `sh`): manifesto ok, timeout, linha inválida, linha > 1 MiB, crash, `Run` com exit code e env; `CleanView` table-driven; `List` filtra.

### M0.3 — Aba proxy + registro dinâmico + CLI/doctor
- [ ] `internal/tui/modules/plugin` implementa `module.Module` + `Commander` sobre um `plugin.Proc` (estado morto em erro, `:reload` respawna, `exec` interativo via `tea.ExecProcess`); `app.Deps` apende plugins em `Modules()`/`Commands()`/checks; `cli.PluginCommands`/`PluginChecks`; `Context.In`; `main` fecha os processos ao sair.
- **Toca:** `internal/tui/modules/plugin/*`, `internal/app/{app,features}.go`, `internal/cli/{cli,plugin}.go`, `main.go`.
- **Aceite:** `tui/app.go` intocado; testes com fixture em `app`, `cli` e no módulo; tmux com `examples/plugins/hello` (aba, teclas, `:reload`, `q` sem órfão).

### M0.4 — Documentação e regras
- [ ] CLAUDE.md (config.yaml, `Section`, `internal/plugin` no grafo, regra 9 de plugins), README (config YAML, plugins), `internal/tui/theme/README.md`.

---

## M1 — Pendências de skills, sessões e TUI

### M1.1 — Marketplaces no formato oficial
- [ ] Ler `.claude-plugin/marketplace.json` de repos git como fonte adicional de skills.
- **Toca:** `internal/skill/marketplace.go` (`LoadMarketplace(url) ([]Entry, error)` via clone raso), `tui/modules/skills/install.go`.
- **Aceite:** repo fixture com marketplace.json lista e instala uma entry; JSON inválido = erro amigável; testes com `t.TempDir()`.

### M1.2 — Apelido de sessão
- [ ] Apelido próprio, sem mexer no arquivo do CLI.
- **Toca:** `internal/session/alias.go` (`<DataDir>/session-aliases.json` via `fsutil.WriteAtomic`, campos desconhecidos sobrevivem), `tui/modules/sessions` (tecla `m`; `FilterValue` inclui o apelido).
- **Aceite:** apelido sobrevive a restart; filtro encontra; input vazio remove; round-trip testado.

### M1.3 — Modal de ajuda trunca a coluna direita
- [ ] Em 120 colunas a segunda coluna do `?` corta as descrições com `…`.
- **Toca:** `internal/tui/app.go` (`renderHelp`/`helpColumns`: largura máxima do painel calculada pelo conteúdo, não fixa em 74).
- **Aceite:** nenhuma descrição truncada em 100+ colunas; uma coluna abaixo de ~64; tmux.

---

## M2 — Módulo Uso (assinatura-aware)

Primeiro módulo novo porque é **somente leitura**: exercita todas as costuras (capacidade no agente, service, aba, `Commander`, CLI, doctor) sem risco para arquivos vivos. Nunca no startup: carga sob demanda, cacheada.

Formatos já observados nesta máquina (só chaves, sem valores):
- Claude Code: `~/.claude/.credentials.json` → `claudeAiOauth.{subscriptionType, rateLimitTier, expiresAt, …}`; JSONL das sessões com `message.usage` por mensagem.
- Codex: `~/.codex/auth.json` → `auth_mode`, `OPENAI_API_KEY` (null quando OAuth), `tokens.{…}`.

### M2.1 — Capacidades no agente
- [ ] `internal/agent/usage.go`: `UsageEvent{Time, Model, CWD, Usage}` + `UsageEventReader{ UsageEvents(s Session) ([]UsageEvent, error) }`; `AuthMode` (`Unknown|Subscription|APIKey`) + `AuthModeReader{ AuthMode() (AuthMode, detail string) }`.
- **Detalhes:** structs de decode com **só os campos não secretos** (tokens nunca entram na memória do app). Claude: eventos a partir do parser JSONL existente. Codex: **verificar** evento `token_count` do rollout.
- **Aceite:** fixtures com assinatura e com API key; teste garantindo que nenhum campo de token é decodificado.

### M2.2 — Service `internal/usage`
- [ ] `Blocks(agentID)` (janelas de 5h estilo ccusage: bloco começa no primeiro evento, gap > 5h abre outro), `Daily(n)`, `ByProject()`; custo USD só com `AuthAPIKey` (reusa `agent.EstimateCost`).
- **Aceite:** table-driven de blocos (bordas de 5h, gaps, bloco atual contendo agora); agregação por dia/projeto.

### M2.3 — Aba Uso + CLI
- [ ] `tui/modules/usage` (bloco atual com barra de progresso do tempo, últimos 7 dias, por projeto, badge "assinatura"/"API key"); consome `events.SessionsLoaded`. `lazyagents usage [--json] [--agent id]`.
- **Aceite:** registrada só via `app.Features`; tmux; `--json` estável.

---

## M3 — Módulo Providers (estilo cc-switch)

Primeiro módulo que **escreve** em config viva de agente. Introduz o primitivo compartilhado de edição.

### M3.1 — Primitivo de edição de config viva
- [ ] `internal/agent/settings.go`: ler JSON como `map[string]json.RawMessage`, mutar uma chave, `fsutil.Backup` + `fsutil.WriteAtomic` preservando modo e chaves desconhecidas (padrão de `core.SaveConfig`).
- **Aceite:** round-trip preserva campos desconhecidos e permissões; backup criado antes de toda escrita; teste com arquivo 0600.

### M3.2 — Capacidade e service
- [ ] `agent.ProviderHost{ ProviderFile(); ReadProvider() (ProviderProfile, bool, error); ApplyProvider(p, backupsDir) error; ClearProvider(backupsDir) error }`; `internal/provider` com perfis nomeados em `<ConfigDir>/providers.json` (0600).
- **Por agente:** Claude Code = `env` do `~/.claude/settings.json` (`ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_MODEL`); OpenCode = bloco `provider` do `opencode.json` (**verificar** chaves); Codex = `model_provider` + `[model_providers.x]` em `config.toml` (**decidir** estratégia TOML antes: dependência nova ou edição de linhas — registrar no CLAUDE.md); Gemini = **verificar**.
- **Aceite:** aplicar/limpar por agente com backup; token nunca aparece em log, `View()` ou `--json` sem `--reveal`.

### M3.3 — Aba Providers + CLI
- [ ] Matriz perfil × agente com o ativo marcado; `apply`/`clear` com confirm mostrando o diff; `lazyagents provider list|apply|clear [--reveal]`.
- **Aceite:** tmux; doctor avisa perfil que referencia agente não instalado.

---

## M4 — Módulo Hooks

Formatos já observados nesta máquina:
- Claude Code: `~/.claude/settings.json` → `hooks.<Evento>[].{matcher, hooks[].{type, command, timeout}}`.
- Codex: `~/.codex/hooks.json` com a mesma forma por evento, **mais** `[features] hooks` e `[hooks.state."<arquivo>:<evento>:<i>:<j>"] trusted_hash` no `config.toml` — hook novo pode exigir confirmação de confiança no próprio Codex (**verificar** antes de escrever).

### M4.1 — Capacidade `HooksHost`
- [ ] `internal/agent/hooks.go`: `Hook{Event, Matcher, Command, Timeout}` + `HooksHost{ HookEvents(); HooksFile(); ReadHooks(); WriteHooks(hooks, backupsDir) }`. Claude Code primeiro (reusa M3.1); Codex depois de verificar o `trusted_hash`; Gemini/OpenCode = **verificar** suporte.
- **Aceite:** ler/escrever preserva hooks não gerenciados e chaves desconhecidas; backup antes de escrever.

### M4.2 — Service `internal/hooks`
- [ ] Biblioteca em `<DataDir>/hooks/<nome>.json`; o conjunto gerenciado é reconciliado por identidade `(evento, matcher, comando)` — hooks fora da biblioteca nunca são tocados (mesma regra das skills locais).
- **Aceite:** enable/disable idempotentes; hook alheio sobrevive; testes table-driven.

### M4.3 — Aba Hooks + CLI
- [ ] Matriz hook × agente igual à de skills; `lazyagents hooks list|enable|disable|add`; doctor avisa comando de hook inexistente ou não executável.
- **Aceite:** tmux; registrado só via `app.Features`.

---

## Fora de escopo (decidido)

- Watch automático de filesystem (`r` recarrega)
- Sync em nuvem, system tray, auto-updater, i18n
- Proxy local de API (providers só escrevem config do agente)
- Gerenciar skills embutidas em plugins do Claude Code
