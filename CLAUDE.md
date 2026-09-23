# CLAUDE.md — lazyagents

TUI em Go para gerenciar **skills**, **sessões**, **uso**, **provedores** e demais configurações (hooks) dos agentes de coding AI (Claude Code, Codex, Gemini CLI, OpenCode, Claude Desktop, Hermes Agent). Organizado em **módulos** — ver "Arquitetura".

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
├── app/                # raiz de composição: Load (boot + migrações) e features() — O REGISTRO
├── feature/            # contrato entre módulo e raiz: Feature (abas, comandos, checks, Close) e Deps
├── core/               # Paths (XDG), config.yaml (Config.Section por módulo), Tilde/ExpandHome — base da pilha
├── fsutil/             # WriteAtomic, Backup, RotateBackups — escrita de configuração e backups compartilhados
├── agent/              # 1 adapter por agente + interfaces de capacidade. ÚNICO lugar que conhece paths/formatos dos CLIs
├── cli/                # SÓ framework headless: Run, Command, Context, Check, doctor agregador
├── tui/                # SÓ framework de TUI
│   ├── app.go          # root: splash, header, abas, ajuda, paleta — não conhece nenhuma aba concreta
│   ├── module/         # contrato module.Module (+ Commander opcional)
│   ├── events/         # mensagens trocadas ENTRE módulos (AgentsDetected, SkillsScanned, SessionsLoaded, TabActivated, Reload)
│   ├── kit/            # estilos, delegate de lista, markdown, foco de painel, helpers de layout
│   ├── components/     # widgets: Panel, Palette, Confirm, Toast, Splash
│   └── theme/          # ÚNICO lugar com literais de cor; theme.AgentColor(id)
└── modules/            # UM PACOTE POR MÓDULO: domínio + aba + CLI + registro juntos
    ├── skills/  sessions/  agents/  providers/  hooks/  usage/
    └── plugins/        # protocolo JSON Lines, processo `<bin> serve` e aba proxy (docs/plugins.md)
```

Dependências (acíclicas): `fsutil ← core ← agent ← {cli, tui/*} ← feature ← modules/* ← app ← main`.
Nem `cli` nem `tui` conhecem módulo algum: os dois são framework, e é o módulo que importa os dois.

### Anatomia de um módulo (`internal/modules/<nome>/`)

| Arquivo | Conteúdo |
|---|---|
| `service.go` (+ arquivos por assunto) | domínio: `Service`, tipos, I/O. Não importa `tui/` |
| `tab.go`, `view.go`, `help.go`, `msgs.go` | a aba: o model chama-se **`Tab`**, o construtor **`newTab`** |
| `cli.go` | `commands(svc) []cli.Command` e `checks(svc) []cli.Check`, ambos NÃO exportados |
| `feature.go` | `Feature() feature.Feature` — o que app registra |

Arquivo da aba que repete o assunto de um arquivo de domínio leva o sufixo `_ui`
(`alias.go`/`alias_ui.go`, `install.go`/`install_ui.go`).

### Como adicionar um módulo novo

1. **Pasta:** `internal/modules/<nome>/` com os arquivos acima. Só `Feature()` (e o que outro módulo precise) é exportado.
2. **Capacidade no agente, se tocar os CLIs:** interface opcional em `internal/agent/<cap>.go` (ex.: `HooksHost`), implementada só pelos adapters que suportam, resolvida por type assertion. `agent.Adapter` **não cresce**.
3. **Registro:** UMA linha em `app.features()`. A ordem é a ordem das abas; `Last: true` joga a aba para o fim, depois até das de plugin (é o caso de Uso e Agentes, que são consulta). `app/app.go`, `tui/app.go` e `cli/cli.go` nunca são editados para isso.
4. **Config do módulo:** seção de topo `<id>:` no `config.yaml`, lida com `d.Config.Section("<id>", &cfg)` (struct com tags yaml no próprio pacote). Nunca adicionar chave em `core.Config` — só `theme` e `libraryDir` são globais. A seção `tui:` (id reservado) é do app: `app/layout.go` resolve ordem, abas ocultas, aba inicial e splash e entrega `tui.Options` ao root. Aba oculta continua viva (recebe broadcasts, nunca teclado nem `TabActivated`); feature que sobe processo por aba consulta `Deps.TabHidden` e nem cria a oculta.
5. **Dependências:** o service nasce dentro da `Feature()`, nunca em `feature.Deps` — é assim que Deps não cresce a cada módulo.
6. **Conversa entre abas:** mensagem lida por outra aba vai para `internal/tui/events` carregando **agregado, nunca tipo de módulo** (`SkillsScanned` leva `map[agente]int`, não `[]Skill`); o resto fica não exportado no pacote.

Plugins externos (binários em `<ConfigDir>/plugins/`) são abas/comandos/checks descobertos em runtime pelo módulo `plugins`, que por isso é o último do registro: quando ele roda, os nomes embutidos já estão reservados (`feature.Deps.Reserved`). O contrato está em `docs/plugins.md`.

Regras invioláveis:

1. **Nada fora de `internal/agent/` conhece paths ou formatos de arquivo dos CLIs.**
2. **Escritas de configuração passam por `fsutil.WriteAtomic`**; cópias de árvores e backups de sessões podem usar streaming para arquivos novos, limpando cópias parciais em erro; mexer em arquivo vivo de CLI exige `fsutil.Backup` antes e preservar chaves desconhecidas. O `config.yaml` só é reescrito via `core.Config.Save` (round-trip por `yaml.Node`: comentários e seções alheias sobrevivem). Config viva de agente em JSON passa pelo primitivo `settings` (`internal/agent/settings.go`), que mexe só na chave alvo e mantém a ordem do arquivo. **TOML (Codex): sem lib e sem reserializar** — o `config.toml` carrega estado alheio (`[projects.*]`, `[hooks.state.*]` com hash de confiança), então o lazyagents edita apenas blocos delimitados por `# lazyagents — …` e copia o resto linha a linha.
3. **Ativação de skill = symlink** da biblioteca (`<DataDir>/skills/<nome>`, DataDir = `~/.local/share/lazyagents`) para o dir de skills do agente. Desativar = remover o symlink. Skill que já é dir real no agente é "local" — nunca deletar dir real ao desativar.
4. **O service de um módulo não importa `tui/`; a aba não faz I/O direto** — sempre via service dentro de `tea.Cmd`. Os dois convivem no mesmo pacote, mas a separação continua valendo por arquivo.
5. **Erros:** `fmt.Errorf("contexto %s: %w", x, err)`. Na TUI vira toast, nunca panic.
6. **Testes nunca tocam `~/` real** — `core.PathsIn(t.TempDir())`, home injetável.
7. **Segredos** (tokens de provider, credenciais): arquivos 0600, backups com o mesmo modo, valor sempre mascarado na TUI e no `--json` (só `--reveal` explícito na CLI mostra). Nunca ler token para exibir. Credencial de agente só pode ser materializada para autenticar uma chamada do próprio agente (hoje: `Claude.RateLimits`), dentro da função, nunca em struct exportada, log, erro ou disco. Token de provedor só trafega em `ProviderProfile.Token`, entre `providers.json` (0600) e a config do agente; tudo que é exibido passa por `Redacted()`. Rede só sob demanda, jamais no boot.
8. **Cores só em `theme/`**; cor de agente via `theme.AgentColor(id)`.
9. **Confirmação que o CLI pede ao usuário nunca é forjada.** O `trusted_hash` de hook do Codex (`[hooks.state]` no `config.toml`) é o registro de que o usuário aceitou rodar aquele comando: o lazyagents instala o hook e **avisa** (`HooksHost.HooksNote`), mas não escreve o hash nem liga `[features] hooks`. Vale para qualquer mecanismo de consentimento que apareça depois.
10. **Plugins externos só via `internal/modules/plugins`.** O protocolo (`docs/plugins.md`, `plugins.Protocol`) só muda com bump de versão. Tudo que vem do plugin é não confiável: `view` passa por `CleanView`, manifesto é saneado, falha vira estado morto na aba — nunca panic, nunca derruba a TUI.
11. **Transcript é lido pelo índice** (`internal/agent/index.go`): o JSONL só cresce, então cada arquivo é lido uma vez e depois só a parte anexada. Adapter novo que extrai algo do transcript inteiro (prévia, tokens, uso, limites) põe isso no `lineScanner` dele, nunca num scan próprio por chamada; varredura de muitos arquivos usa `refreshAll` (um worker por CPU). Mudou `indexEntry`? Suba `indexVersion`.

## Workflow

1. Trabalhe em **UMA task do BACKLOG.md por vez**, na ordem. Não inicie a próxima com a atual falhando.
2. Antes de declarar concluído: `test -z "$(gofmt -l .)" && go vet ./... && go test -race ./... && go build ./...` tudo verde + teste manual via tmux quando tocar UI.
3. Marcar o checkbox no BACKLOG.md e commitar. Commits em PT-BR: `feat(skill): install via zip`.
