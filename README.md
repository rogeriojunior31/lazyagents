# lazyagents

TUI em Go para gerenciar, num lugar só, o que os seus agentes de coding AI usam: **skills**, **sessões** e, no roadmap, **hooks**, **uso** e **providers**. Funciona com Claude Code, Codex, Gemini CLI, OpenCode, Claude Desktop e Hermes Agent. Inspirado no [cc-switch](https://github.com/farion1231/cc-switch) e no lazygit.

![demo](demo.gif)

```sh
go install github.com/rogeriojunior31/lazyagents@latest
lazyagents          # abre a TUI
lazyagents doctor   # diagnóstico sem TUI
```

## O que faz

**Skills**
- **Matriz skill × agente:** cada skill em cada agente, alternada individualmente (`1-9`) ou em todos de uma vez (`space`, `a`, `x`).
- **Instalação** (`i`) de repositório GitHub (`usuario/repo` ou URL), pasta local ou `.zip`, com descoberta recursiva e seleção do que instalar.
- **Busca no GitHub** (`S`) por repositórios com `SKILL.md`, via `gh api`.
- **Adoção** (`o`, `A`): skill que já vive dentro de um agente vai para a biblioteca e vira symlink, pronta para os demais agentes.
- **Ciclo de vida:** criar (`n`), editar no `$EDITOR` (`e`), atualizar da origem (`u`, `U`), perfis por agente (`p`), backups e restore (`b`), lint do `SKILL.md`.

**Sessões**
- **Histórico unificado** de todos os agentes; `enter` suspende a TUI e retoma a sessão no CLI de origem, no diretório certo.
- **Transcript** em cards de chat (`v`), export para Markdown (`x`), busca full-text (`F`).
- **Organização:** agrupamento por agente e projeto (`g`), filtro por agente (`f`), tokens e custo estimado, badge de sessão ativa.
- **Higiene:** deletar com backup (`d`), inclusive em lote (`space`).

**Agentes**
- Card por agente instalado: versão, skills ativas, sessões e os diretórios de skills que ele lê, com alerta quando um diretório é compartilhado entre agentes. Os não instalados ficam numa linha só.

**Plugins**
- Qualquer executável em `~/.config/lazyagents/plugins/` vira uma aba, um subcomando (`lazyagents <id> …`) e uma seção do `doctor`. Contrato em JSON Lines, em qualquer linguagem: [docs/plugins.md](docs/plugins.md), exemplo em [examples/plugins/hello](examples/plugins/hello).

**CLI headless** para scripts e para os próprios agentes: `lazyagents help`.

## Instalar

```sh
go install github.com/rogeriojunior31/lazyagents@latest
```

Ou, do clone: `go build -o lazyagents . && ./lazyagents`. Binários para Linux, macOS e Windows serão publicados em [Releases](https://github.com/rogeriojunior31/lazyagents/releases) a partir da primeira tag.

Requisitos: Go 1.26+ para compilar. Em runtime, opcionais: `git` para instalar skills do GitHub, `gh` autenticado para a busca, `sqlite3` para sessões do OpenCode e `lsof` para o badge de sessão ativa.

## Como funciona

A biblioteca central fica em `~/.local/share/lazyagents/skills/`, no padrão XDG. Ativar uma skill num agente cria um **symlink** no diretório de skills dele e desativar remove o symlink. Conteúdo real, como skills locais e symlinks de outras ferramentas, nunca é apagado. Remoções e updates geram backup `.tar.gz` em `~/.local/share/lazyagents/backups/`.

| Agente | Gerencia em | Também lê |
|---|---|---|
| Claude Code | `~/.claude/skills` | — |
| Codex | `~/.agents/skills` ⚠ compartilhado | `~/.codex/skills` |
| Gemini CLI | `~/.gemini/skills` | `~/.agents/skills` |
| OpenCode | `~/.config/opencode/skills` | `~/.claude/skills`, `~/.agents/skills` |
| Hermes Agent | `~/.hermes/skills` | `~/.agents/skills` |
| Claude Desktop | — (skills e conversas ficam na conta claude.ai) | — |

⚠ `~/.agents/skills` é o diretório cross-agente: Codex, Gemini e OpenCode leem dele. A matriz mostra isso (`◆` = visível via diretório compartilhado). Para usá-lo como biblioteca: `lazyagents migrate-library ~/.agents/skills`.

Sessões lidas (somente leitura): Claude Code (`~/.claude/projects/*.jsonl`), Codex (`~/.codex/sessions/`), Gemini (`~/.gemini/{history,tmp}/*/chats/`), OpenCode (`opencode.db` via `sqlite3`).

## Configuração

Tudo opcional, em `~/.config/lazyagents/config.yaml` (ou `$XDG_CONFIG_HOME/lazyagents/`). Comentários e chaves que o lazyagents não conhece são preservados quando ele reescreve o arquivo. Um `config.json` de versões anteriores é migrado automaticamente na primeira abertura (o original fica como `config.json.migrated`).

```yaml
theme: garoa                 # noite | garoa | jaragua
libraryDir: ~/.agents/skills

# uma seção por módulo ou plugin, com o id da aba como chave
hello:
  greeting: olá
```

| Chave | Padrão | Efeito |
|---|---|---|
| `theme` | `noite` | tema da TUI: `noite`, `garoa` ou `jaragua` (ver [Temas](#temas)). Vale na próxima abertura; valor desconhecido cai em `noite` com aviso ao sair |
| `libraryDir` | `~/.local/share/lazyagents/skills` | onde fica a biblioteca. Prefira `lazyagents migrate-library <dir>`, que move as skills e refaz os symlinks |
| `<id>` | — | seção livre do módulo ou plugin de id `<id>`; um plugin a recebe inteira no `init` |

Arquivos do lazyagents:

| Caminho | Conteúdo |
|---|---|
| `~/.config/lazyagents/config.yaml` | configuração |
| `~/.config/lazyagents/plugins/` | plugins externos (executáveis) |
| `~/.local/share/lazyagents/skills/` | biblioteca de skills |
| `~/.local/share/lazyagents/profiles.json` | perfis de ativação |
| `~/.local/share/lazyagents/backups/` | backups de skills e sessões deletadas |
| `~/.local/share/lazyagents/exports/` | transcripts exportados |

## Temas

As cores da TUI vêm do **[SP Night](https://sp-night.github.io/)**, uma paleta com São Paulo como referência. Os três flavors escuros dele são os temas padrão do lazyagents e já vêm embutidos no binário:

| `theme` | Tema | |
|---|---|---|
| `noite` (padrão) | **Noite Paulista** | A cidade às 3h: escuro azul-violeta e o laranja do poste de sódio por cima. |
| `garoa` | **Garoa** | A mesma janela vista através do chuvisco: cinza chapado, cores desbotadas. |
| `jaragua` | **Pico do Jaraguá** | A mesma noite vista do alto: o escuro puxado para o verde da mata. |

Para trocar, defina `theme` no `config.yaml` (acima) e abra o lazyagents de novo. Para deixar o terminal e o editor com a mesma cara, o SP Night tem ports para outras ferramentas em [sp-night.github.io](https://sp-night.github.io/). A paleta é MIT; o aviso de licença está em [internal/tui/theme/LICENSE-SP-Night](internal/tui/theme/LICENSE-SP-Night).

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
| `d` | remove da biblioteca (com backup) | `←/→` | foca lista ou detalhe |

Em terminais estreitos (< 76 colunas), lista e detalhe viram uma tela cada; `←`/`→` alterna.

**Mouse:** a roda rola listas e leitores, o clique seleciona abas e itens, e clicar de novo no item selecionado abre a leitura.

## CLI

```text
lazyagents list [--json]
lazyagents enable <skill> [--agent id|--all]
lazyagents disable <skill> [--agent id|--all]
lazyagents install <origem>
lazyagents remove <skill>
lazyagents adopt <skill> --agent <id>
lazyagents migrate-library <dir>
lazyagents sessions [--json]
lazyagents <plugin> [args…]
lazyagents doctor
```

`doctor` lista os agentes detectados, valida cada `SKILL.md`, procura symlinks quebrados e faz o handshake de cada plugin; sai com código 1 se achar problema.

## Arquitetura

O projeto é organizado em módulos. Cada módulo é um service de domínio, uma aba da TUI, comandos da CLI, uma seção própria no `config.yaml` e, se precisar, uma capacidade opcional nos adapters de agente. Todos são registrados num único lugar, `internal/app/features.go`; plugins externos entram pelo mesmo caminho em runtime. O passo a passo para adicionar um módulo e as regras do projeto estão no [CLAUDE.md](CLAUDE.md).

Próximos módulos, descritos no [BACKLOG.md](BACKLOG.md):
- **Uso:** tokens por janela de 5h, por dia e por projeto, ciente de conta por assinatura ou chave de API.
- **Providers:** perfis de endpoint e modelo aplicados na config de cada agente, estilo cc-switch.
- **Hooks:** biblioteca de hooks ativados por agente, para os agentes que suportam.

## Desenvolvimento

```sh
gofmt -l . && go vet ./... && go test ./... && go build ./...
go run scripts/preview.go -theme garoa -page 2   # TUI com dados fictícios e config isolada
scripts/record-demo.sh                          # regrava demo.gif (requer vhs, ttyd e ffmpeg)
```

As paletas saem de um checkout do SP Night: `go run ./internal/tui/theme/generate -source <sp-night>` (detalhes em [internal/tui/theme/README.md](internal/tui/theme/README.md)).

## Licença

[MIT](LICENSE). Os temas são do [SP Night](https://sp-night.github.io/), também MIT ([aviso](internal/tui/theme/LICENSE-SP-Night)).
