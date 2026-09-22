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
- [x] Descoberta em `<ConfigDir>/plugins/*` (executável, nome `^[a-z0-9][a-z0-9_-]{0,31}$`), `Service.Start` (spawn `<bin> serve`, `init`, manifesto em até 3 s), `Proc.Send/Events/Close`, `Run` (pass-through), `CleanView` (só SGR passa). Protocolo JSON Lines documentado em `docs/plugins.md`.
- **Toca:** `internal/plugin/{proto,plugin}.go`, `internal/core/paths.go` (`PluginsDir`), `docs/plugins.md`, `examples/plugins/hello`.
- **Aceite:** fixtures `#!/bin/sh` (pulam sem `sh`): manifesto ok, timeout, linha inválida, linha > 1 MiB, crash, `Run` com exit code e env; `CleanView` table-driven; `List` filtra.

### M0.3 — Aba proxy + registro dinâmico + CLI/doctor
- [x] `internal/tui/modules/plugin` implementa `module.Module` + `Commander` sobre um `plugin.Proc` (estado morto em erro, `:reload` respawna, `exec` interativo via `tea.ExecProcess`); `app.Deps` apende plugins em `Modules()`/`Commands()`/checks; `cli.PluginCommands`/`PluginChecks`; `Context.In`; `main` fecha os processos ao sair.
- **Toca:** `internal/tui/modules/plugin/*`, `internal/app/{app,features}.go`, `internal/cli/{cli,plugin}.go`, `main.go`.
- **Aceite:** `tui/app.go` intocado; testes com fixture em `app`, `cli` e no módulo; tmux com `examples/plugins/hello` (aba, teclas, `:reload`, `q` sem órfão).

### M0.4 — Documentação e regras
- [x] CLAUDE.md (config.yaml, `Section`, `internal/plugin` no grafo, regra 9 de plugins), README (config YAML, plugins), `internal/tui/theme/README.md`.

---

## M1 — Pendências de skills, sessões e TUI

### M1.1 — Marketplaces no formato oficial
- [x] Ler `.claude-plugin/marketplace.json` de repos git como fonte adicional de skills.
- **Feito:** `Discover` (TUI `i`, busca `S`, `lazyagents install`) prefere o marketplace quando ele declara skills: plugins com origem no próprio repo (`./x`, nome simples + `metadata.pluginRoot`, lista `skills`) viram entradas com o nome do plugin no picker; `Rel` relativo à raiz mantém o update funcionando. Origens externas (`github`, `url`, `git-subdir`, `npm`, `archive`, `command`) não são buscadas nem executadas: viram aviso no picker/stderr com o repo a instalar. Marketplace sem skill cai na varredura genérica.
- **Toca:** `internal/skill/marketplace.go`, `internal/skill/install.go`, `tui/modules/skills/{picker,model}.go`, `cli/skills.go`.
- **Aceite:** repo fixture com marketplace.json lista e instala uma entry; JSON inválido = erro amigável; testes com `t.TempDir()`.

### M1.2 — Apelido de sessão
- [x] Apelido próprio, sem mexer no arquivo do CLI.
- **Toca:** `internal/session/alias.go` (`<DataDir>/session-aliases.json` via `fsutil.WriteAtomic`, campos desconhecidos sobrevivem), `tui/modules/sessions` (tecla `m`; `FilterValue` inclui o apelido).
- **Aceite:** apelido sobrevive a restart; filtro encontra; input vazio remove; round-trip testado.

### M1.3 — Modal de ajuda trunca a coluna direita
- [x] Em 120 colunas a segunda coluna do `?` corta as descrições com `…`.
- **Toca:** `internal/tui/app.go` (`renderHelp`/`helpColumns`: largura máxima do painel calculada pelo conteúdo, não fixa em 74).
- **Aceite:** nenhuma descrição truncada em 100+ colunas; uma coluna abaixo de ~64; tmux.

---

## M2 — Módulo Uso (assinatura-aware)

Primeiro módulo novo porque é **somente leitura**: exercita todas as costuras (capacidade no agente, service, aba, `Commander`, CLI, doctor) sem risco para arquivos vivos. Nunca no startup: carga sob demanda, cacheada.

**Em assinatura o limite não é token nem custo — é percentual de janela** (sessão de 5h e semanal, com horário de reset). Tokens e custo em USD só fazem sentido em conta por chave de API. Formatos verificados nesta máquina:

- **Codex**: o rollout JSONL já traz tudo, offline. Evento `event_msg` com `payload.type == "token_count"` → `payload.rate_limits.{primary,secondary}.{used_percent, window_minutes, resets_at}` (unix), `plan_type`, `credits`. Consumo por resposta vem do evento `token_usage_record` (`payload.usage`, onde `input_tokens` **já inclui** `cached_input_tokens`); versões antigas usam `payload.info.last_token_usage`.
- **Claude Code**: não grava limite em disco. Mesma fonte que o `/usage` e as ferramentas de barra do Omarchy usam: `GET https://api.anthropic.com/api/oauth/usage?at_wall=1&skip_spend=1` com `Authorization: Bearer <accessToken de ~/.claude/.credentials.json>` e `anthropic-beta: oauth-2025-04-20`. Resposta: `limits[]` com `{kind: session|weekly_all|weekly_scoped, percent, resets_at, severity, scope.model.display_name}`, legado `five_hour/seven_day.{utilization,resets_at}` e `seven_day_breakdown.rows[]`. Uso de tokens por sessão continua vindo do JSONL (`message.usage` + `timestamp` + `cwd` por linha).

Regras do módulo: rede só sob demanda (nunca no boot), resposta cacheada, token usado apenas no header e jamais exibido, logado ou persistido.

### M2.1 — Capacidades no agente
- [x] `internal/agent/usage.go`: `UsageEvent{Time, Model, CWD, Usage}` + `UsageEventReader`; `AuthMode` (`Unknown|Subscription|APIKey`) + `AuthModeReader`. `internal/agent/ratelimit.go`: `RateWindow{Kind, Label, UsedPercent, ResetsAt, Severity}`, `RateStatus{Plan, Windows, FetchedAt, Source}` + `RateLimitReader{ RateLimits(ctx) (RateStatus, error) }`.
- **Detalhes:** structs de decode com **só os campos não secretos**; o `accessToken` só é materializado dentro da chamada HTTP do Claude. Codex resolve offline pelos rollouts.
- **Aceite:** fixtures de rollout (com e sem `secondary`, formato antigo e novo) e servidor HTTP de teste para o Claude; teste garantindo que nenhum token aparece em struct exportada, log ou erro.

### M2.2 — Service `internal/usage`
- [ ] `Status(agentID)` (janelas de limite, cacheadas em `<DataDir>/usage-cache.json` com TTL); `Blocks(agentID)` (janelas de 5h estilo ccusage a partir dos `UsageEvent`), `Daily(n)`, `ByProject()`; custo USD só com `AuthAPIKey` (reusa `agent.EstimateCost`).
- **Aceite:** table-driven de blocos (bordas de 5h, gaps, bloco atual contendo agora); cache respeita TTL e sobrevive a restart; agregação por dia/projeto.

### M2.3 — Aba Uso + CLI
- [ ] `tui/modules/usage`: por agente, barras de sessão de 5h e semanal com % e reset, badge do plano e do modo de auth; tokens por dia/projeto como detalhe; custo em USD só em conta por chave de API. `lazyagents usage [--json] [--agent id] [--refresh]`.
- **Aceite:** registrada só via `app.Features`; sem rede no boot; tmux; `--json` estável.

---

## M3 — Módulo Providers (estilo cc-switch)

Primeiro módulo que **escreve** em config viva de agente. Introduz o primitivo compartilhado de edição.

### M3.1 — Primitivo de edição de config viva
- [ ] `internal/agent/settings.go`: ler JSON como `map[string]json.RawMessage`, mutar uma chave, `fsutil.Backup` + `fsutil.WriteAtomic` preservando modo e chaves desconhecidas (padrão de `core.SaveConfig`).
- **Aceite:** round-trip preserva campos desconhecidos e permissões; backup criado antes de toda escrita; teste com arquivo 0600.

### M3.2 — Capacidade e service
- [ ] `agent.ProviderHost{ ProviderFile(); ReadProvider() (ProviderProfile, bool, error); ApplyProvider(p, backupsDir) error; ClearProvider(backupsDir) error }`; `internal/provider` com perfis nomeados em `<ConfigDir>/providers.json` (0600); preferências não secretas via `deps.Config.Section("providers", &cfg)`.
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
