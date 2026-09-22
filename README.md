# lazyagents

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
- **Histórico unificado** de todos os agentes; `enter` suspende a TUI e retoma a sessão no CLI de origem, no diretório certo.
- **Transcript** em cards de chat (`v`), export para Markdown (`x`), busca full-text (`F`).
- **Organização:** apelido próprio (`m`, guardado pelo lazyagents sem tocar no arquivo do CLI e usado no filtro), agrupamento por agente e projeto (`g`), filtro por agente (`f`), tokens e custo estimado, badge de sessão ativa.
- **Higiene:** deletar com backup (`d`), inclusive em lote (`space`).

**Uso**
- **Limites da assinatura** por agente: barras de sessão e semana com percentual usado e horário de reset, mais o plano e o modo de conta. O Codex sai dos próprios rollouts, sem rede; o Claude Code consulta o mesmo endpoint do `/usage`, só quando você abre a aba, com cache.
- **Detalhe local:** bloco de 5h atual, últimos 7 dias e por projeto, a partir dos transcripts. Custo em dólar só aparece em conta por chave de API, onde se paga por token.

**Provedores**
- **Perfis de endpoint e modelo** aplicados na config viva de cada CLI, estilo cc-switch: matriz perfil × agente, `1-9` aplica (de novo remove) e todo write passa por um confirm que mostra o arquivo e o que muda. Backup automático antes de escrever. Claude Code recebe o bloco `env` do `settings.json`; o Codex recebe `model_provider` e `[model_providers.lazyagents]` no `config.toml`, em blocos delimitados que preservam o resto do arquivo.
- **Token nunca aparece**: fica no `providers.json` (0600) e vai direto para a config do agente; na TUI e no `--json` só sai `token ✓`, e em claro apenas com `provider list --reveal`.

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
| `~/.config/lazyagents/providers.json` | perfis de provedor (0600, pode ter token) |
| `~/.config/lazyagents/plugins/` | plugins externos (executáveis) |
| `~/.local/share/lazyagents/skills/` | biblioteca de skills |
| `~/.local/share/lazyagents/profiles.json` | perfis de ativação |
| `~/.local/share/lazyagents/hooks/` | biblioteca de hooks (um JSON por hook, mais os scripts importados) |
| `~/.local/share/lazyagents/session-aliases.json` | apelidos de sessão |
| `~/.local/share/lazyagents/usage-cache.json` | cache dos limites de assinatura |
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
| | | `m` | apelido (vazio remove) |
| `d` | remove da biblioteca (com backup) | `←/→` | foca lista ou detalhe |

Em Provedores: `1-9` aplica o perfil no agente N (de novo remove), `space` aplica em todos os instalados, `x` limpa todos, `d` apaga o perfil. Perfis são criados pela CLI: `lazyagents provider add <nome> --base-url <url> --token -` (o `-` lê o token da entrada padrão, fora do histórico do shell).

Em Hooks as teclas são as mesmas, instalando em vez de aplicar; hooks são criados com `lazyagents hooks add <nome> --event SessionStart --command "..."`.

Em terminais estreitos (< 76 colunas), lista e detalhe viram uma tela cada; `←`/`→` alterna.

**Mouse:** a roda rola listas e leitores, o clique seleciona abas e itens, e clicar de novo no item selecionado abre a leitura.

## CLI

```text
lazyagents list [--json]
lazyagents enable <skill> [--agent id|--all]
lazyagents disable <skill> [--agent id|--all]
lazyagents install <origem> [--hooks]
lazyagents remove <skill>
lazyagents adopt <skill> --agent <id>
lazyagents migrate-library <dir>
lazyagents sessions [--json]
lazyagents provider list|apply <perfil>|clear|add <perfil>|rm <perfil> [--agent id] [--json] [--reveal]
lazyagents hooks list|enable <nome>|disable <nome>|add <nome>|rm <nome> [--agent id] [--json]
lazyagents usage [--json] [--agent id] [--refresh]
lazyagents <plugin> [args…]
lazyagents doctor
```

`doctor` lista os agentes detectados, valida cada `SKILL.md`, procura symlinks quebrados e faz o handshake de cada plugin; sai com código 1 se achar problema.

## Arquitetura

O projeto é organizado em módulos, **um pacote por módulo** em `internal/modules/<nome>/`: o service de domínio, a aba da TUI, os comandos da CLI e o registro moram juntos, e nada além de uma linha em `internal/app/features.go` precisa ser tocado para adicionar um. `internal/cli` e `internal/tui` são só framework e não conhecem módulo algum; cada módulo pode ter uma seção própria no `config.yaml` e, se precisar mexer nos CLIs, uma capacidade opcional nos adapters de `internal/agent`. Plugins externos entram pelo mesmo contrato, em runtime. O passo a passo e as regras estão no [CLAUDE.md](CLAUDE.md).

Os próximos passos estão no [BACKLOG.md](BACKLOG.md).

## Desenvolvimento

```sh
gofmt -l . && go vet ./... && go test ./... && go build ./...
go run scripts/preview.go -theme garoa -page 2   # TUI com dados fictícios e config isolada
scripts/record-demo.sh                          # regrava demo.gif (requer vhs, ttyd e ffmpeg)
```

As paletas saem de um checkout do SP Night: `go run ./internal/tui/theme/generate -source <sp-night>` (detalhes em [internal/tui/theme/README.md](internal/tui/theme/README.md)).

## Licença

[MIT](LICENSE). Os temas são do [SP Night](https://sp-night.github.io/), também MIT ([aviso](internal/tui/theme/LICENSE-SP-Night)).
