<p align="center"><img src="docs/assets/logo.svg" width="180" alt="Logo do lazyagents: uma preguiça dormindo na rede enquanto três robôs carregam mini terminais"></p>

# lazyagents

[![CI](https://github.com/rogeriojunior31/lazyagents/actions/workflows/ci.yml/badge.svg)](https://github.com/rogeriojunior31/lazyagents/actions/workflows/ci.yml)

TUI em Go para gerenciar, num lugar só, o que os seus agentes de coding AI usam: **skills**, **sessões**, **uso**, **provedores** e **hooks**. Funciona com Claude Code, Codex, Gemini CLI, OpenCode, Claude Desktop e Hermes Agent. Inspirado no [cc-switch](https://github.com/farion1231/cc-switch) e no lazygit.

![demo](demo.gif)

```sh
go install github.com/rogeriojunior31/lazyagents@latest
lazyagents          # abre a TUI
lazyagents doctor   # diagnóstico sem TUI
```

## O que faz

**Skills**
- **Matriz skill × agente:** cada skill em cada agente, alternada individualmente (`1-9`) ou em todos de uma vez (`space`, `a`, `x`).
- **Instalação** (`i`) de repositório GitHub (`usuario/repo` ou URL), pasta local ou `.zip`, com descoberta recursiva e seleção do que instalar. Repositórios de marketplace do Claude Code (`.claude-plugin/marketplace.json`) são lidos pelo manifesto, com as skills agrupadas por plugin.
- **Busca no GitHub** (`S`) por repositórios com `SKILL.md`, via `gh api`.
- **Adoção** (`o`, `A`): skill que já vive dentro de um agente vai para a biblioteca e vira symlink, pronta para os demais agentes.
- **Ciclo de vida:** criar (`n`), editar no `$EDITOR` (`e`), atualizar da origem (`u`, `U`), perfis por agente (`p`), backups e restore (`b`), lint do `SKILL.md`.

**Sessões**
- **Histórico unificado** de Claude Code, Codex, Gemini CLI e OpenCode; `enter` suspende a TUI e retoma a sessão no CLI de origem, no diretório certo.
- **Transcript** em cards de chat (`v`), export para Markdown (`x`), busca full-text (`F`).
- **Organização:** apelido próprio (`m`, guardado pelo lazyagents sem tocar no arquivo do CLI e usado no filtro), agrupamento por agente e projeto (`g`), filtro por agente (`f`), tokens e custo estimado, badge de sessão ativa.
- **Higiene:** deletar com backup (`d`), inclusive em lote (`space`).

**Uso**
- **Limites da assinatura** por agente: barras de sessão e semana com percentual usado e horário de reset, mais o plano e o modo de conta. O Codex sai dos próprios rollouts, sem rede; o Claude Code consulta o mesmo endpoint do `/usage`, só quando você abre a aba, com cache.
- **Consumo filtrável** a partir dos transcripts: período (`p`: hoje, 7, 30, 90 dias ou tudo), agente (`a`), visão (`←/→`: por dia, agente, projeto ou modelo) e texto (`/` filtra as linhas pelo nome), com sparkline, bloco de 5h atual e participação de cada linha. Os filtros também estão na paleta (`:usage period 30d`, `:usage view projects`) e o padrão vem do `config.yaml`. Custo em dólar só aparece em conta por chave de API, onde se paga por token.

**Provedores**
- **Perfis de endpoint e modelo** aplicados na config viva de cada CLI, estilo cc-switch: matriz perfil × agente, `1-9` aplica (de novo remove) e todo write passa por um confirm que mostra o arquivo e o que muda. Backup automático antes de escrever. Claude Code recebe o bloco `env` do `settings.json`; o Codex recebe `model_provider` e `[model_providers.lazyagents]` no `config.toml`, em blocos delimitados que preservam o resto do arquivo.
- **Token nunca aparece**: fica no `providers.json` (0600); no Claude Code é aplicado no `settings.json`. No Codex, configure `--env-key VAR` e exporte a variável: o token salvo não é copiado para o TOML; na TUI e no `--json` só sai `token ✓`, e em claro apenas com `provider list --reveal`.

**Hooks**
- **Importados junto com as skills:** repositório que traz `hooks/hooks.json` (o formato de plugin do Claude Code) aparece no mesmo picker do `i`, desmarcado — instalar um hook é rodar comando de terceiro a cada evento. As pastas que os comandos citam são copiadas para a biblioteca preservando o layout, e o `${CLAUDE_PLUGIN_ROOT}` passa a apontar para essa cópia (e é exportado, para o script que o lê por dentro). Na CLI, `lazyagents install <origem> --hooks`.
- **Biblioteca própria** de hooks (`~/.local/share/lazyagents/hooks/`, um JSON por hook) instalada por agente: matriz hook × agente, `1-9` instala (de novo remove) e confirm mostrando `evento → comando` antes de reescrever a config, com backup. O agente que não dispara aquele evento aparece marcado com `–`.
- **Hook que não é seu não é tocado:** o lazyagents reconhece os próprios pela tripla evento + matcher + comando e conta os demais à parte. No Codex, ele instala e avisa — o `trusted_hash`, que é a sua confirmação de que aquele comando pode rodar, quem escreve é o próprio Codex.

**Agentes**
- Card por agente instalado: versão, skills ativas, sessões e os diretórios de skills que ele lê, com alerta quando um diretório é compartilhado entre agentes. Os não instalados ficam numa linha só.

**Plugins**
- Qualquer executável em `~/.config/lazyagents/plugins/` vira uma aba, um subcomando (`lazyagents <id> …`) e uma seção do `doctor`. Contrato em JSON Lines, em qualquer linguagem: [docs/plugins.md](docs/plugins.md), exemplo em [examples/plugins/hello](examples/plugins/hello).

**CLI headless** para scripts e para os próprios agentes: `lazyagents help`.

## Instalar

```sh
go install github.com/rogeriojunior31/lazyagents@latest
```

Ou baixe o binário pronto em [Releases](https://github.com/rogeriojunior31/lazyagents/releases) — Linux, macOS (Intel e Apple Silicon) e Windows, com `SHA256SUMS` para conferir:

```sh
tar -xzf lazyagents_<versão>_linux_amd64.tar.gz && ./lazyagents
```

Do clone: `go build -o lazyagents . && ./lazyagents`.

Requisitos: Go 1.26+ para compilar. Em runtime, opcionais: `git` para instalar skills do GitHub, `gh` autenticado para a busca, `sqlite3` para sessões do OpenCode e `lsof` para o badge de sessão ativa.

## Como funciona

A biblioteca central fica em `~/.local/share/lazyagents/skills/`, no padrão XDG. Ativar uma skill num agente cria um **symlink** no diretório de skills dele e desativar remove o symlink. Conteúdo real, como skills locais e symlinks de outras ferramentas, nunca é apagado. Remoções e updates geram backup `.tar.gz` em `~/.local/share/lazyagents/backups/`.

| Agente | Gerencia em | Também lê |
|---|---|---|
| Claude Code | `~/.claude/skills` | — |
| Codex | `~/.agents/skills` ⚠ compartilhado | `~/.codex/skills` |
| Gemini CLI | `~/.gemini/skills` | `~/.agents/skills` |
| OpenCode | `~/.config/opencode/skills` | `~/.claude/skills`, `~/.agents/skills` |
| Hermes Agent | `~/.hermes/skills` | — (diretórios externos personalizados ainda não são detectados) |
| Claude Desktop | — (skills e conversas ficam na conta claude.ai) | — |

⚠ `~/.agents/skills` é o diretório cross-agente: Codex, Gemini e OpenCode leem dele. A matriz mostra isso (`◆` = visível via diretório compartilhado). Para usá-lo como biblioteca: `lazyagents migrate-library ~/.agents/skills`.

Formatos de sessões lidos (exclusão apenas por ação explícita, com backup): Claude Code (`~/.claude/projects/*.jsonl`), Codex (`~/.codex/sessions/`), Gemini (`~/.gemini/{history,tmp}/*/chats/`), OpenCode (`opencode.db` via `sqlite3`).

## Configuração

Tudo opcional. No Linux, em `~/.config/lazyagents/config.yaml` (ou `$XDG_CONFIG_HOME/lazyagents/`). No macOS, a configuração segue `~/Library/Application Support/lazyagents/`; no Windows, `%AppData%/lazyagents/`. A pasta de dados usa `$XDG_DATA_HOME/lazyagents/`, com fallback em `~/.local/share/lazyagents/`. Os exemplos abaixo usam o layout Linux. Comentários e chaves que o lazyagents não conhece são preservados quando ele reescreve o arquivo. Um `config.json` de versões anteriores é migrado automaticamente na primeira abertura (o original fica como `config.json.migrated`).

```yaml
theme: garoa                 # embutido (ver Temas) ou o id de um arquivo em themes/
libraryDir: ~/.agents/skills

usage:
  period: 30d                   # filtros com que a aba Uso abre: today | 7d | 30d | 90d | all
  view: projects                # daily | agents | projects | models

tui:
  splash: true                  # false pula a tela inicial
  splashSeconds: 2              # quanto ela fica (0 pula; teto de 10)
  startTab: sessions            # aba em que o app abre
  tabs: [sessions, skills, usage]   # ordem; as não listadas vêm depois, na ordem padrão
  hidden: [hooks, providers]        # somem da barra e da paleta

# uma seção por módulo ou plugin, com o id da aba como chave
hello:
  greeting: olá
```

| Chave | Padrão | Efeito |
|---|---|---|
| `theme` | `noite` | tema da TUI: um dos embutidos ou o id de um tema seu em `themes/` (ver [Temas](#temas)). Vale na próxima abertura; valor desconhecido cai em `noite` com aviso ao sair |
| `libraryDir` | `~/.local/share/lazyagents/skills` | onde fica a biblioteca. Prefira `lazyagents migrate-library <dir>`, que move as skills e refaz os symlinks |
| `usage` | `7d`, `daily` | filtros iniciais da aba Uso: `period` (`today`, `7d`, `30d`, `90d`, `all`) e `view` (`daily`, `agents`, `projects`, `models`); valor desconhecido fica no padrão |
| `tui` | — | layout da TUI: `splash`, `splashSeconds`, `startTab`, `tabs` (ordem) e `hidden`. Ids de aba: `skills`, `sessions`, `agents`, `providers`, `hooks`, `usage` e o id de cada plugin (os mesmos da paleta `:`). Aba embutida oculta continua carregando em segundo plano (Uso e Agentes dependem das sessões); plugin oculto nem é iniciado, mas o comando dele continua na CLI. Id desconhecido vira aviso ao sair, nunca erro |
| `<id>` | — | seção livre do módulo ou plugin de id `<id>`; um plugin a recebe inteira no `init` |

Arquivos do lazyagents:

| Caminho | Conteúdo |
|---|---|
| `~/.config/lazyagents/config.yaml` | configuração |
| `~/.config/lazyagents/providers.json` | perfis de provedor (0600, pode ter token) |
| `~/.config/lazyagents/plugins/` | plugins externos (executáveis) |
| `~/.config/lazyagents/themes/` | temas do usuário (`<id>.yaml`) |
| `~/.local/share/lazyagents/skills/` | biblioteca de skills |
| `~/.local/share/lazyagents/profiles.json` | perfis de ativação |
| `~/.local/share/lazyagents/hooks/` | biblioteca de hooks (um JSON por hook, mais os scripts importados) |
| `~/.local/share/lazyagents/session-aliases.json` | apelidos de sessão |
| `~/.local/share/lazyagents/usage-cache.json` | cache dos limites de assinatura |
| `~/.local/share/lazyagents/transcript-index.gob` | índice dos transcripts (título, tokens, uso por faixa de 15 min); descartável, apagar só faz a próxima carga reler tudo |
| `~/.local/share/lazyagents/backups/` | backups de skills e sessões deletadas |
| `~/.local/share/lazyagents/exports/` | transcripts exportados |

Migração de biblioteca recusa destinos com nomes já existentes, inclusive symlinks, e diretórios sobrepostos. Resolva os conflitos antes de repetir; os arquivos de origem são preservados se a preparação falhar.

Custos são estimativas das tarifas padrão da Claude API, com cache de escrita de 5 minutos. Modelo desconhecido exibe apenas tokens. Na TUI e no `sessions --json`, agregado com modelos diferentes também; o `lazyagents usage` soma o custo resposta a resposta, pela tarifa de cada modelo, e mostra `—` se faltar preço para alguma; fast mode, batch, cache de 1 hora e tarifas regionais não são calculados. Confira as [tarifas oficiais](https://platform.claude.com/docs/en/about-claude/pricing).

O Codex atual aceita `--wire-api responses` (também é seu padrão). A edição automática de provedores recusa TOML com strings multilinha, preservando o arquivo para ajuste manual. Hooks importados preservam o comando original e definem `CLAUDE_PLUGIN_ROOT` no shell; a compatibilidade do payload entre CLIs depende do script. Scripts POSIX precisam de um shell compatível.

## Temas

As cores da TUI vêm do **[SP Night](https://sp-night.github.io/)**, uma paleta com São Paulo como referência. Os três flavors dele são os temas padrão e vêm embutidos com a paleta original inteira (as 23 cores de cada flavor) e todos os papéis semânticos do projeto (`ui`, `syntax`, `diagnostic`, `git`, `ansi`), sem adaptação:

| `theme` | Tema | |
|---|---|---|
| `noite` (padrão) | **Noite Paulista** | A cidade às 3h: escuro azul-violeta e o laranja do poste de sódio por cima. |
| `garoa` | **Garoa** | A mesma janela vista através do chuvisco: cinza chapado, cores desbotadas. |
| `jaragua` | **Pico do Jaraguá** | A mesma noite vista do alto: o escuro puxado para o verde da mata. |

Também vêm embutidos temas conhecidos da comunidade, com as paletas oficiais. Onde a distribuição original deixaria texto ilegível (contraste abaixo de 3:1), o papel usa outra cor da mesma paleta; só em Nord, Dracula e Rosé Pine Dawn entra uma cor derivada, marcada no arquivo:

| `theme` | Tema |
|---|---|
| `tokyonight`, `tokyonight-storm` | Tokyo Night (Night, Storm) |
| `dracula` | Dracula |
| `gruvbox-dark` | Gruvbox Dark |
| `nord` | Nord |
| `rose-pine`, `rose-pine-moon`, `rose-pine-dawn` | Rosé Pine (Main, Moon, Dawn — claro) |
| `kanagawa` | Kanagawa Wave |
| `everforest-dark` | Everforest Dark |
| `onedark` | One Dark |

Para trocar, defina `theme` no `config.yaml` (acima) e abra o lazyagents de novo.

### Tema próprio

Crie `~/.config/lazyagents/themes/<id>.yaml` e use `theme: <id>`. O formato é o mesmo dos temas embutidos ([exemplos](internal/tui/theme/themes/)): uma `palette` opcional de cores com nome e os papéis por grupo. Um papel aceita `"#rrggbb"` (com aspas) ou um nome da `palette`. O que faltar vem do tema de `extends` (padrão `noite`), então dá para mudar só o necessário:

```yaml
# ~/.config/lazyagents/themes/meu-dracula.yaml
label: Meu Dracula
extends: dracula
palette:
  purple: "#caa9fa"        # repinta todo papel do dracula que usa purple
ui:
  accent: "#ff79c6"        # destaque principal (aba ativa, bordas em foco…)
  bg: "#1e1f29"
diagnostic:
  ok: "#50fa7b"
```

Os papéis são os do SP Night; a lista completa e o que cada um pinta na TUI estão em [internal/tui/theme/README.md](internal/tui/theme/README.md). Arquivo inválido (hex errado, papel desconhecido, `extends` em ciclo) é ignorado com aviso no stderr (ao sair da TUI; no início de um comando da CLI), e os outros temas continuam valendo. Um arquivo não pode usar o id de um tema embutido; para derivar, use `extends`. `appearance` (`dark`/`light`) é informativo: descreve o tema, mas não muda o desenho.

Para deixar o terminal e o editor com a mesma cara, o SP Night tem ports para outras ferramentas em [sp-night.github.io](https://sp-night.github.io/). As paletas são MIT (Tokyo Night: Apache-2.0); os avisos estão em [internal/tui/theme/LICENSE-SP-Night](internal/tui/theme/LICENSE-SP-Night) e [internal/tui/theme/LICENSES-themes.md](internal/tui/theme/LICENSES-themes.md).

## Teclas

Globais: `tab`/`shift+tab` trocam de aba (ou clique), `:` abre a paleta de comandos, `?` mostra todas as teclas da aba atual, `q` sai.

| Skills | | Sessões | |
|---|---|---|---|
| `enter` | lê o SKILL.md | `enter` | retoma no CLI de origem |
| `e` / `n` | edita / cria skill | `v` | transcript (`x` exporta) |
| `1-9` | alterna no agente N | `R` | retoma em outra pasta |
| `space` `a` `x` | alterna / ativa / desativa em todos | `c` | mostra o comando de resume |
| `i` / `S` | instala / busca no GitHub | `d` / `space` | deleta com backup / seleciona lote |
| `o` / `A` | adota / adota todas as locais | `g` | agrupa por agente + projeto |
| `u` / `U` | atualiza / verifica updates | `f` / `F` | filtra por agente / busca nos transcripts |
| `p` / `b` | perfis / backups | `/` `r` | filtra / recarrega |
| | | `m` | apelido (vazio remove) |
| `d` | remove da biblioteca (com backup) | `←/→` | foca lista ou detalhe |

Em Provedores: `1-9` aplica o perfil no agente N (de novo remove), `space` aplica em todos os instalados, `x` limpa todos, `d` apaga o perfil. Perfis são criados pela CLI: `lazyagents provider add <nome> --base-url <url> --token -` (o `-` lê o token da entrada padrão, fora do histórico do shell).

Em Hooks as teclas são as mesmas, instalando em vez de aplicar; hooks são criados com `lazyagents hooks add <nome> --event SessionStart --command "..."`.

Em Uso: `p`/`P` troca o período, `a`/`A` o agente, `←/→` (ou `v`) a visão, `/` filtra as linhas da tabela pelo nome (`enter` mantém, `esc` limpa; `esc` de novo volta a todos os agentes), `r` atualiza os limites e `pgup`/`pgdn` rolam uma página.

Em terminais estreitos (< 76 colunas), lista e detalhe viram uma tela cada; `←`/`→` alterna.

**Mouse:** a roda rola listas e leitores, o clique seleciona abas e itens, e clicar de novo no item selecionado abre a leitura.

## CLI

```text
lazyagents help [comando]                     # lista os comandos ou detalha um
lazyagents list [--json]                      # ou: lazyagents skills list
lazyagents enable <skill> [--agent id|--all]
lazyagents disable <skill> [--agent id|--all]
lazyagents install <origem> [--hooks]
lazyagents remove <skill>
lazyagents adopt <skill> --agent <id>
lazyagents migrate-library <dir>
lazyagents sessions [--agent id] [--here] [--limit n] [--json]
lazyagents provider list|apply <perfil>|clear|add <perfil>|rm <perfil> [--agent id] [--json] [--reveal]
lazyagents hooks list|enable <nome>|disable <nome>|add <nome>|rm <nome> [--agent id] [--json]
lazyagents usage [limits|daily|agents|projects|models] [--agent id] [--since 7d] [--limit n] [--refresh] [--json]
lazyagents <plugin> [args…]
lazyagents doctor [--json]
```

`usage` sem visão é um painel: limites da assinatura com barras e reset, o bloco atual de 5h e o resumo do período (sparkline por dia, participação de cada agente, projetos que mais gastaram). As visões `daily`, `agents`, `projects` e `models` são tabelas de tokens (entrada, saída, cache, respostas); `limits` mostra só as janelas. `--since` aceita `7d`, `24h` ou uma data `2026-09-01`. O custo em USD aparece só para agente autenticado por API key, somado evento a evento pela tarifa de cada modelo, e fica `—` quando falta preço.

A saída é colorida no terminal e sai sem cores em pipe, arquivo ou com `NO_COLOR`; para scripts, todo comando de consulta tem `--json`.

`doctor` lista os agentes detectados, valida cada `SKILL.md`, procura symlinks quebrados e faz o handshake de cada plugin; sai com código 1 se achar problema.

## Arquitetura

O projeto é organizado em módulos, **um pacote por módulo** em `internal/modules/<nome>/`: o service de domínio, a aba da TUI, os comandos da CLI e o registro moram juntos, e nada além de uma linha em `internal/app/features.go` precisa ser tocado para adicionar um. `internal/cli` e `internal/tui` são só framework e não conhecem módulo algum; cada módulo pode ter uma seção própria no `config.yaml` e, se precisar mexer nos CLIs, uma capacidade opcional nos adapters de `internal/agent`. Plugins externos entram pelo mesmo contrato, em runtime. O passo a passo e as regras estão no [CLAUDE.md](CLAUDE.md).

Os próximos passos estão no [BACKLOG.md](BACKLOG.md). A revisão da versão atual, suas evidências e limitações estão em [docs/review.md](docs/review.md).

## Desenvolvimento

```sh
test -z "$(gofmt -l .)" && go vet ./... && go test -race ./... && go build ./...
go build -o /tmp/lazyagents-preview scripts/preview.go
bash -n scripts/*.sh
go mod verify
go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./... # consulta a base de vulnerabilidades
go run scripts/preview.go -theme garoa -page 2   # TUI com dados fictícios e config isolada
scripts/record-demo.sh                          # regrava demo.gif (requer vhs, ttyd e ffmpeg)
```

Os temas SP Night saem de um checkout do SP Night: `go run ./internal/tui/theme/generate -source <sp-night>` (detalhes em [internal/tui/theme/README.md](internal/tui/theme/README.md)).

## Licença

[MIT](LICENSE). Os temas são do [SP Night](https://sp-night.github.io/), também MIT ([aviso](internal/tui/theme/LICENSE-SP-Night)).
