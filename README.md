# lazyskills

TUI em Go para gerenciar **skills** e **sessões** de todos os seus agentes de coding AI num lugar só. Irmão focado do vultrix-tui, inspirado no cc-switch.

```
 lazyskills  skills e sessões de todos os seus agentes
  Skills    Sessões    Agentes
──────────────────────────────────────────────────────────────
  │ CXGO  omarchy (local)        1 ● Claude Code — ativa
  │ C··O  ponytail (local)       2 ● Codex — ativa (gerenciada)
    ····  da-lib                 3 ○ Gemini CLI — inativa
                                 4 ◆ OpenCode — via ~/.claude/skills
```

## O que faz

- **Matriz skill × agente** — vê e alterna cada skill em cada agente (Claude Code, Codex, Gemini CLI, OpenCode, Claude Desktop, Hermes Agent), individualmente (`1-9`) ou em todos de uma vez (`space`/`a`/`x`).
- **Detecção de agentes** — a aba *Agentes* mostra o que está instalado, versão, e quais diretórios de skills cada um lê.
- **Instalação de skills** (`i`) — de repositório GitHub (`usuario/repo` ou URL), pasta local ou `.zip`. Aceita qualquer layout de repo: `SKILL.md` na raiz, `skills/<nome>/`, categorias aninhadas — tudo é descoberto recursivamente, com dedupe, e você escolhe o que instalar num multi-select.
- **Adoção** (`o`) — skill que já vive dentro de um agente é copiada para a biblioteca central e substituída por symlink, pronta para ser ativada nos demais.
- **Sessões unificadas** — histórico de todos os agentes numa lista só; `enter` suspende a TUI e retoma a sessão no CLI de origem, no diretório certo; ao sair, a TUI volta.

## Como funciona

A biblioteca central fica em `~/.lazyskills/skills/`. Ativar uma skill num agente cria um **symlink** no diretório de skills dele; desativar remove o symlink. Conteúdo real (skills locais, symlinks de outras ferramentas) nunca é deletado — remoções sempre geram backup `.tar.gz` em `~/.lazyskills/backups/`.

Diretórios por agente:

| Agente | Gerencia em | Também lê |
|---|---|---|
| Claude Code | `~/.claude/skills` | — |
| Codex | `~/.agents/skills` ⚠ compartilhado | `~/.codex/skills` |
| Gemini CLI | `~/.gemini/skills` | `~/.agents/skills` |
| OpenCode | `~/.config/opencode/skills` | `~/.claude/skills`, `~/.agents/skills` |

⚠ `~/.agents/skills` é o diretório padrão cross-agente: codex, gemini e opencode leem dele. A matriz mostra isso honestamente (`◆` = visível via diretório compartilhado).

Sessões: Claude Code (`~/.claude/projects/*.jsonl`), Codex (`~/.codex/sessions/`), Gemini (`~/.gemini/{history,tmp}/*/chats/`), OpenCode (`opencode.db` via `sqlite3`).

## Instalar e rodar

```sh
go build -o lazyskills . && ./lazyskills
```

Requisitos: Go 1.26+; `git` para instalar skills do GitHub; `sqlite3` para listar sessões do OpenCode.

## Teclas

| Tecla | Ação |
|---|---|
| `tab` / `shift+tab` | troca de aba (ou clique na aba) |
| `enter` | abre o SKILL.md para leitura (scroll com ↑↓/roda do mouse, `esc` volta) |
| `e` | edita o SKILL.md no `$EDITOR` (funciona na lista e na leitura) |
| `n` | cria uma skill nova (template + abre o editor) |
| `1-9` | alterna a skill no agente N |
| `space` | ativa em todos (ou desativa, se já ativa em todos) |
| `a` / `x` | ativa / desativa em todos |
| `i` | instala (GitHub, pasta ou zip) |
| `o` | adota skill local para a biblioteca |
| `d` | remove da biblioteca (com backup) |
| `/` | filtra a lista |
| `r` | recarrega |
| `enter` (Sessões) | retoma a sessão no CLI de origem |
| `v` (Sessões) | lê o transcript da conversa na TUI |
| `f` (Sessões) | cicla o filtro por agente (todas → claude → gemini → …) |
| `c` (Sessões) | mostra o comando de resume |
| `q` | sai |

**Mouse:** roda rola listas e a leitura de SKILL.md; clique seleciona (abas, skills, sessões); clicar de novo no item selecionado abre a leitura (Skills) ou retoma a sessão (Sessões).

Sessões do Claude Code renomeadas (via `/rename`) aparecem com o nome dado — o lazyskills lê a última linha `ai-title` do transcript.
