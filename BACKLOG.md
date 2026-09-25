# BACKLOG — lazyagents

Planejamento em ordem de execução. **Uma task por vez**: não iniciar a próxima com a atual falhando.

## Como trabalhar (toda task)

1. Seguir o `CLAUDE.md`, em especial "Como adicionar um módulo novo" e as regras invioláveis.
2. Verificar: `gofmt -l . && go vet ./... && go test ./... && go build ./...` tudo verde.
3. Teste manual na TUI via tmux quando a task tocar UI.
4. Marcar o checkbox aqui e commitar em inglês: `feat(scope): description` (ver M15).

Legenda: **toca** = arquivos/pacotes previstos · **aceite** = critérios verificáveis · **verificar** = confirmar formato na doc/código do agente antes de codar, nunca adivinhar.

> **Layout:** desde a reorganização de 22/09/2026, cada módulo é UM pacote em `internal/modules/<nome>/` (domínio + aba + CLI + `Feature()`), registrado por uma linha em `app.features()`. Os caminhos citados nas tasks já concluídas são os de antes da mudança; para os próximos módulos vale o layout novo, descrito no CLAUDE.md.

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
- [x] `Status(agentID)` (janelas de limite, cacheadas em `<DataDir>/usage-cache.json` com TTL); `Blocks(agentID)` (janelas de 5h estilo ccusage a partir dos `UsageEvent`), `Daily(n)`, `ByProject()`; custo USD só com `AuthAPIKey` (reusa `agent.EstimateCost`).
- **Aceite:** table-driven de blocos (bordas de 5h, gaps, bloco atual contendo agora); cache respeita TTL e sobrevive a restart; agregação por dia/projeto.

### M2.3 — Aba Uso + CLI
- [x] `tui/modules/usage`: por agente, barras de sessão de 5h e semanal com % e reset, badge do plano e do modo de auth; tokens por dia/projeto como detalhe; custo em USD só em conta por chave de API. `lazyagents usage [--json] [--agent id] [--refresh]`.
- **Aceite:** registrada só via `app.Features`; sem rede no boot; tmux; `--json` estável.

---

## M3 — Módulo Providers (estilo cc-switch)

Primeiro módulo que **escreve** em config viva de agente. Introduz o primitivo compartilhado de edição.

### M3.1 — Primitivo de edição de config viva
- [x] `internal/agent/settings.go`: `readSettings` guarda as chaves de topo como `json.RawMessage` **na ordem do arquivo** (token stream do decoder; um map perderia a ordem de um arquivo editado à mão), `get`/`set` (nil remove) mexem só na chave alvo e `save(backupsDir)` faz `fsutil.Backup` + `RotateBackups` + `WriteAtomic` preservando a permissão (arquivo novo nasce 0600, que é o default de quem pode guardar token).
- **Aceite:** round-trip preserva campos desconhecidos e permissões; backup criado antes de toda escrita; teste com arquivo 0600.

### M3.2 — Capacidade e service
- [x] `agent.ProviderHost{ ProviderFile(); ReadProvider() (ProviderProfile, bool, error); ApplyProvider(p, backupsDir) error; ClearProvider(backupsDir) error }`; `internal/provider` com perfis nomeados em `<ConfigDir>/providers.json` (0600), `Status()` por agente e `Apply/Clear` (agente vazio = todos os instalados que suportam). `ProviderProfile.Redacted()` (idempotente) é a única forma que sai para TUI/JSON/log.
- **Por agente:** Claude Code = `env` do `~/.claude/settings.json` (`ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_MODEL`), via o primitivo M3.1. Codex = `model_provider`/`model` no topo + `[model_providers.lazyagents]` no `config.toml`; **estratégia TOML decidida: blocos delimitados, sem dependência nova** (o arquivo carrega `[projects.*]` e `[hooks.state.*]` com hash de confiança, que nenhuma lib reserializa sem reescrever). O Codex não aceita token no arquivo: só `env_key`, e aplicar perfil com token sem `envKey` é erro. OpenCode e Gemini ficam de fora até haver `opencode.json`/`settings.json` para verificar — nunca adivinhar formato.
- **Feito:** leitura do Codex funciona também para provider configurado à mão (mini-leitor de `key = "string"`, nunca usado na escrita).
- **Aceite:** aplicar/limpar por agente com backup; token nunca aparece em log, `View()` ou `--json` sem `--reveal`.

### M3.3 — Aba Providers + CLI
- [x] Aba `providers` (matriz perfil × agente, `1-9` aplica/remove no agente N, `space` em todos, `x` limpa, `d` apaga o perfil), toda escrita atrás de um confirm que mostra `de → para` e o arquivo. CLI `provider list|apply|clear|add|rm [--agent id] [--json] [--reveal]`; `add --token -` lê o token da entrada padrão. Doctor: seção "provedores" com o aplicado por agente, problema quando o agente não está instalado.
- **Feito:** validado com o Codex real (`codex doctor` reconheceu `[model_providers.lazyagents]` e pediu a variável do `env_key`); apply→apply→clear devolve o `config.toml` e o `settings.json` ao conteúdo original.
- **Ficou de fora:** criar/editar perfil pela TUI (a CLI cria; a TUI aplica). Adicionar quando pedir mais que um formulário de 5 campos.

---

## M4 — Módulo Hooks

Formatos já observados nesta máquina:
- Claude Code: `~/.claude/settings.json` → `hooks.<Evento>[].{matcher, hooks[].{type, command, timeout}}`.
- Codex: `~/.codex/hooks.json` com a mesma forma por evento, **mais** `[features] hooks` e `[hooks.state."<arquivo>:<evento>:<i>:<j>"] trusted_hash` no `config.toml` — hook novo pode exigir confirmação de confiança no próprio Codex (**verificar** antes de escrever).

### M4.1 — Capacidade `HooksHost`
- [x] `internal/agent/hooks.go`: `Hook{Event, Matcher, Command, Timeout}` (identidade = a tripla evento+matcher+comando) e `HooksHost{ HookEvents(); HooksFile(); ReadHooks(); AddHook(h, backupsDir); RemoveHook(h, backupsDir); HooksNote() }`. **AddHook/RemoveHook em vez de WriteHooks(conjunto):** escrita cirúrgica nunca reescreve grupo alheio, então campo desconhecido dentro dele sobrevive.
- **Verificado nesta máquina:** os dois CLIs usam o mesmo formato (`"hooks": {Evento: [{matcher?, hooks:[{type,command,timeout}]}]}`), o Claude Code dentro do `settings.json` e o Codex no `hooks.json`. Vocabulário de eventos em CamelCase; o Codex normaliza para snake_case (o `trusted_hash` real da máquina aponta `hooks.json:session_start:0:0`), então a comparação de evento ignora caixa e separador.
- **Codex, limite deliberado:** o lazyagents **não escreve** `[features] hooks` nem `[hooks.state] trusted_hash` no `config.toml`. O hash é a confirmação do usuário de que aquele comando pode rodar; forjá-lo seria aprovar execução em nome dele. `HooksNote()` avisa o que falta (recurso desligado, ou confirmação pendente no próprio Codex).
- **Fora de escopo:** Gemini e OpenCode — sem arquivo de config nesta máquina para verificar o formato.
- **Aceite:** testes cobrem preservação de hook alheio, campo desconhecido no grupo, entrada de tipo não-comando, idempotência do add, limpeza do evento/chave vazios, remoção dentro de grupo compartilhado e a não-escrita do config.toml.

### M4.2 — Service do módulo (`internal/modules/hooks/service.go`)
- [x] Biblioteca em `<DataDir>/hooks/<nome>.json` (um JSON por hook, editável à mão); `Library()` devolve os hooks válidos e os problemas por arquivo, sem que um arquivo quebrado derrube a listagem. `Status()` por agente separa o que é do lazyagents do que é do próprio usuário (`Foreign`, nunca tocado). `Enable/Disable` com agente nomeado ou todos os instalados; instalar hook de evento que o agente não dispara é **erro**, não silêncio. `CommandProblem` acha comando fora do PATH ou sem permissão.
- **Aceite:** enable/disable idempotentes; hook alheio sobrevive; validações de nome/evento/comando cobertas.

### M4.3 — Aba Hooks + CLI
- [x] Aba `hooks` (matriz hook × agente, `1-9` instala/remove no agente N, `space` em todos, `x` remove de todos, `d` apaga da biblioteca), com `–` no agente que não dispara o evento e toda escrita atrás de um confirm que mostra `evento → comando` e o arquivo. CLI `hooks list|enable|disable|add|rm [--agent id] [--json]`. Doctor: comando fora do PATH ou sem permissão, arquivo de biblioteca inválido e o aviso de cada agente.
- **Feito:** validado num HOME isolado — os dois arquivos saem na forma exata dos reais (`settings.json` do Claude Code e `hooks.json` do Codex), com `matcher` só quando o hook tem um, e o `config.toml` do Codex sai intocado.
- **Ficou de fora:** criar/editar hook pela TUI (a CLI cria, a TUI instala) e Gemini/OpenCode, sem formato verificável nesta máquina.

---

## M5 — Hooks vindos de repositório

Repo de skills quase sempre traz hooks junto (no marketplace oficial do Claude Code, `<plugin>/hooks/hooks.json`, por convenção — nem o `marketplace.json` nem o `plugin.json` declaram). Instalar só as skills era entregar metade do pacote.

### M5.1 — Descoberta e importação
- [x] `hooks.DiscoverIn(root, rootName)` acha `<plugin>/hooks/hooks.json` na origem já materializada (ignora dirs ocultos, profundidade 5) e `hooks.Import(paths, found, source)` copia para `<DataDir>/hooks/<plugin>/` **as pastas de primeiro nível que os comandos citam**, preservando o layout da raiz do plugin, e grava a entrada com os comandos reescritos.
- **O nó era o `${CLAUDE_PLUGIN_ROOT}`:** variável que só o Claude Code expande, e só para plugin instalado por ele. Ela passa a apontar para a cópia **e é exportada no comando** — script real se localiza por ela ou por caminho relativo à raiz, então o layout precisa ser o mesmo. Pasta citada que não existe na origem faz o import falhar na hora, em vez de deixar hook que quebraria em silêncio no evento.
- **Ajustado depois do primeiro repo real (`dog_stack`):** copiar só `hooks/` recusava o repo inteiro, porque os comandos também citam `scripts/`; o nome do pacote saía como o diretório temporário do clone quando `hooks/` está na raiz; e `async: true` se perdia na tradução, transformando hook assíncrono em bloqueante. Copiado: 404 KB (`hooks/` + `scripts/`) de um repo de 6 MB.
- **Entrada da biblioteca virou pacote** (`Hook.Hooks []agent.Hook`): o `security-guidance` real tem 12 comandos em 5 eventos e, um por entrada, inundava a biblioteca e a matriz. Comando repetido dentro do pacote entra uma vez (o plugin repete a mesma tripla em matchers diferentes).
- **Aceite:** fixture no layout real + os dois plugins de verdade do marketplace oficial; import é tudo ou nada; `Delete` leva os scripts junto.

### M5.2 — Picker unificado e alcance
- [x] O mesmo `i` da aba Skills e o `lazyagents install <origem>` listam skills e hooks da origem. Hook entra **desmarcado** e o CLI exige `--hooks`: instalar um hook é passar a rodar comando de terceiro a cada evento.
- [x] Hook importado é instalável em qualquer agente que dispare o evento; `Enable` instala o subconjunto suportado (o Codex não tem `Stop`/`SubagentStop`), a matriz mostra `◐` para parcial e a entrada carrega "importado de X · feito para o Claude Code" — o payload que cada CLI manda no stdin não foi verificado.
- **Aceite:** tmux com os dois plugins reais; hook do próprio usuário sobrevive e continua contado à parte.

## M6 — Release e CI

### M6.1 — CI
- [x] `.github/workflows/ci.yml`: a mesma sequência do CLAUDE.md (gofmt, vet, test, build) em push e PR, mais o build do `scripts/preview.go` (tem tag `ignore`, não entra em `./...` e quebraria sem ninguém ver) e um job que compila os cinco alvos publicados.

### M6.2 — Release por tag
- [x] `.github/workflows/release.yml`: tag `v*` roda vet e test, compila com `-trimpath -ldflags "-s -w -X main.version=<tag>"` para linux/{amd64,arm64}, darwin/{amd64,arm64} e windows/amd64, empacota (`.tar.gz`, `.zip` no Windows), gera `SHA256SUMS` e publica com `gh release create --generate-notes`.
- **Sem GoReleaser e sem action de terceiro:** `go build` num loop e o `gh` que já vem no runner — menos dependência para auditar numa ferramenta que mexe na config dos agentes do usuário.
- **Aceite:** o job de release foi simulado localmente antes da tag — os cinco artefatos saem, `SHA256SUMS` confere e o binário responde `lazyagents 0.1.0` (a injeção de versão funciona).

## M7 — Refinamento da versão atual (22/09/2026)

Relatório, evidências e limites: [docs/review.md](docs/review.md). O histórico acima
registra a implementação original; correções posteriores prevalecem sobre descrições antigas.

- [x] Integridade: instalação segura, restore/update com staging, migração com preflight e preservação de configuração.
- [x] Hooks/provedores: proteção de caminhos, colisões e quoting, preservação de campos e provedor anterior, permissões de tokens.
- [x] Robustez: JSON null, ZIPs grandes, ordenação de backups, exportação, metadados e encerramento de plugins.
- [x] Compatibilidade: Hermes, protocolo de provedor Codex, preços por versão, agregação por caminho/modelo.
- [x] Manutenção: docs, preview, script de demo, CI com race, release/licenças e remoção de GoReleaser residual.
- [ ] Executar testes nativos em macOS e Windows; validar symlinks, shells, permissões e caminhos. **Aceite:** matriz com resultados reais por SO, não só compilação cruzada.
- [ ] Fixar uma matriz de versões dos CLIs com fixtures versionadas para formatos privados de sessões e limites. **Aceite:** versão e origem de cada fixture, sem credenciais/dados pessoais.
- [ ] Ampliar configuração de adapters: diretórios externos do Hermes e overrides dos CLIs. **Aceite:** ler configuração explícita, sem anunciar diretórios que o agente não carrega.
- [ ] Refinar estimativas de uso: custo por evento/modelo, cache de 1h, fast/batch/região. **Aceite:** manter “indisponível” quando faltarem dados, sem aplicar a tarifa do último modelo ao total.
- [ ] Limites de recursos: teto agregado para arquivos descompactados, prazo para doctor de plugins e encerramento de árvores de subprocessos. **Aceite:** falha controlada e sem subprocessos órfãos nos SOs suportados.
- [ ] Edição TOML avançada e concorrência entre instâncias: definir suporte a strings multilinha e alterações simultâneas. **Aceite:** não perder estado externo; editor atual recusa strings multilinha e mantém backup antes de escrever.

## M8 — CLI refinada (23/09/2026)

### M8.1 — `usage` completo
- [x] `usage` sem visão vira painel: limites com barra e "reseta em", bloco atual de 5h, sparkline do período, participação por agente e top projetos.
- [x] Visões `limits`, `daily`, `agents`, `projects`, `models` com `--agent`, `--since 7d|24h|AAAA-MM-DD`, `--limit`, `--refresh` e `--json` estável (`rows` + `total`).
- [x] Custo somado evento a evento (tarifa de cada modelo), só para agente por API key; basta um evento sem preço para o agregado ficar "—".
- **Aceite:** mesmos números da aba Uso; sem ANSI fora de terminal; `--agent` inválido lista os ids válidos.

### M8.2 — Framework e polimento
- [x] `Command.Summary`/`Help`, `lazyagents help <comando>`, `cli.Flags` com ajuda em PT-BR e `Context.KnownAgent` (usage, sessions, hooks, provider).
- [x] `doctor --json`; `sessions --agent/--here/--limit`; `skills <sub>` agrupando os comandos de skill; descrição truncada no `list`.

## M9 — Layout da TUI configurável (23/09/2026)

- [x] Seção `tui:` no `config.yaml`: `splash`, `splashSeconds`, `startTab`, `tabs` (ordem; não listadas seguem a ordem padrão) e `hidden`.
- [x] Aba embutida oculta segue viva em segundo plano (Sessões alimenta Uso e Agentes); plugin oculto não sobe processo, mas mantém o comando.
- **Aceite:** valor inválido ou id desconhecido vira aviso ao sair, nunca erro; valor zero de `tui.Options` reproduz o comportamento anterior; tmux com config temporária.

## M10 — Motor de consulta para histórico grande (23/09/2026)

Medido com 2000 sessões (860 MB de transcripts) e 1000 skills, em tmpfs.

- [x] Índice incremental dos transcripts (`transcript-index.gob`): cada JSONL é lido uma vez e depois só a parte anexada; arquivo reescrito é relido. Uma passada extrai prévia, tokens, uso (faixas de 15 min) e limites do Codex, com um worker por CPU. `sessions --json` 2,3 s → 0,11 s; `usage` 3,3 s → 0,15 s.
- [x] Sessão viva: `/proc` no Linux (o `lsof` custava ~130 ms fixos), só para as modificadas nas últimas 24h; deletar confere o arquivo exato na hora.
- [x] Busca full-text com pré-filtro nos bytes crus e em paralelo: 8 s → 0,36 s.
- **Aceite:** saídas `--json` idênticas às da versão anterior (massa e dados reais), inclusive após anexar linhas e com linha final incompleta.

## M11 — Aba Uso filtrável (23/09/2026)

- [x] Filtros de período (hoje/7/30/90 dias/tudo), agente, visão (dia/agente/projeto/modelo) e texto, em memória sobre o histórico inteiro; tabela compartilhada com a CLI, com colunas que encolhem com a largura.
- [x] Filtros na paleta (`usage period …`, `usage view …`, `usage clear`) e padrão na seção `usage:` do `config.yaml`.
- [x] Corpo memorizado: rolar custa 0,1 ms mesmo com 460 mil faixas de uso; trocar filtro, até ~130 ms nesse pior caso.
- **Aceite:** tmux com dados reais em 130, 60 e 40 colunas; `TestResponsiveLayout` verde.

## M12 — Revisão incremental da TUI (24/09/2026)

- [x] Ajuda, confirmações, paleta, campos e seletores acessíveis em telas pequenas, com atalhos essenciais preservados.
- [x] Navegação por abas e rolagem dos detalhes de Agentes/Provedores; exclusão de Sessões com alvos fixados e cancelamento padrão.
- [x] Uso com filtros fixos, erros completos e progresso aguardando ambas as consultas; diagnóstico de plugins rolável com reinício acessível.
- [x] Hooks com seleção compacta, leitura de comandos/scripts em tela cheia e edição no editor com confirmação, backup e tratamento de falhas.
- **Validação:** testes, checks locais equivalentes à CI, ensaios em tmux e capturas VHS. Escopo, limites e pendências visuais detalhados em [docs/tui-design-review.md](docs/tui-design-review.md).

## M13 — Redesenho das abas por objetivo

- [x] Fase 0 — base: `kit.TableRow`/`TableHeader` (1 linha, cor das células preservada na seleção), `AgentColumns`, `TableDelegate`, `SplitDetail` e `Frame` (rodapé na última linha).
- [x] Fase 1 — Skills como matriz skill × agente.
- [x] Fase 2 — Sessões em tabela de 1 linha; preview sem retomar de verdade.
- [x] Fase 3 — Agentes como painel de diagnóstico.
- [x] Fase 4 — Provedores com "em uso" no topo e tabela de perfis.
- [x] Fase 5 — Hooks com biblioteca em matriz.
- [x] Fase 6 — Uso: limites → período → visão, rodapé fixo.
- [x] Fase 7 — limpeza (`PlainDelegate`/`ListRow`), plugins no `Frame`, capturas nos três temas.

## M14 — Temas plugáveis

- [x] SP Night completo: os três flavors embutem as 23 cores e todos os papéis (`ui`, `syntax`, `diagnostic`, `git`, `ansi`) do upstream; tokens novos (Info, Hint, Link, Match, Added/Removed, sintaxe, `ANSI`) usados no markdown, no diff de perfil e nas cores de agente.
- [x] Temas da comunidade com paleta oficial: Tokyo Night (Night/Storm), Dracula, Gruvbox Dark, Nord, Rosé Pine (Main/Moon/Dawn), Kanagawa Wave, Everforest Dark, One Dark.
- [x] Temas do usuário em `<ConfigDir>/themes/<id>.yaml` com `extends`, `palette` e papéis parciais; arquivo inválido vira aviso.
- [x] Contraste: papéis de texto ≥ 3:1 sobre fundo e seleção em todo tema embutido; avisos de licença dos temas no pacote de release.
- **Aceite:** `generate -check` verde; todo tema embutido resolve todos os papéis; `TestLoadUser` cobre herança, ciclo, id reservado e hex inválido; `TestBuiltinContrast` verde.

## M15 — English as the project language

The project is open source: code, comments, UI, CLI, docs and commits move to English. Full plan, glossary and compatibility notes: [docs/english-migration-plan.md](docs/english-migration-plan.md). PT-BR comes back later as a translation (message catalog), not as the language of the code.

- [x] Phase 0 — rules and guard rail: CLAUDE.md in English with the language rule; `scripts/check-english.sh` (accented Portuguese in tracked files, with allowlist and a `check-english:allow` line marker) running in CI as report only; this milestone.
- [ ] Phase 1 — compatibility: Codex `config.toml` managed-block markers read in PT and EN, written in EN; `usage --json` `label` change noted for the release; plugin protocol and `config.yaml` values checked for Portuguese.
- [ ] Phase 2 — TUI and CLI text, one commit per module (framework, skills, sessions, hooks, providers, usage, agents/plugins, cli/app), tests updated in the same commit.
- [ ] Phase 3 — error messages and domain (`agent`, `fsutil`, `core`, services).
- [ ] Phase 4 — themes: READMEs, community theme descriptions, SP Night labels/descriptions via the generator.
- [ ] Phase 5 — comments and tests; `check-english.sh` becomes required in CI.
- [ ] Phase 6 — README, docs, BACKLOG, CI, scripts, `demo.gif`; delete the migration plan.
- **Acceptance:** `scripts/check-english.sh` green and required; a Codex config with the old PT markers is read and migrated (test); every tab, help, palette, confirm and toast in English (tmux); `help`, `--help` and `doctor` in English.

## Fora de escopo (decidido)

- Watch automático de filesystem (`r` recarrega)
- Sync em nuvem, system tray, auto-updater
- Proxy local de API (providers só escrevem config do agente)
- Gerenciar skills embutidas em plugins do Claude Code
