# Suporte dos agentes
<!-- source: 7384987a8011 -->

Esta é a tradução da referência gerada dos adaptadores em `internal/agent`: o que o lazyagents faz com cada agente. Os caminhos são padrões de Linux/macOS. A leitura de sessões está no [guia de sessões](../guide/sessions.md#where-sessions-come-from).

## Skills

Um diretório compartilhado é lido por vários agentes. O lazyagents só coloca o link ali quando a skill está ativada para todos os agentes instalados que o leem; caso contrário, usa o diretório próprio de cada agente.

| Agente | ID | Ativa skills em | Também lê |
|---|---|---|---|
| Claude Code | `claude-code` | `~/.claude/skills` | — |
| Codex | `codex` | `~/.codex/skills` | `~/.agents/skills` (compartilhado) |
| Gemini CLI | `gemini-cli` | `~/.gemini/skills` | `~/.agents/skills` (compartilhado) |
| OpenCode | `opencode` | `~/.config/opencode/skills` | `~/.claude/skills`, `~/.agents/skills` (compartilhado) |
| Claude Desktop | `claude-desktop` | — (não gerenciado localmente) | — |
| Hermes Agent | `hermes-agent` | `~/.hermes/skills` | — |
| Pi | `pi` | `~/.pi/agent/skills` | `~/.agents/skills` (compartilhado) |
| Crush | `crush` | `~/.config/crush/skills` | `~/.config/agents/skills`, `~/.claude/skills`, `~/.agents/skills` (compartilhado) |

<a id="capabilities"></a>

## Recursos

| Agente | Provedores (escrita) | Hooks (escrita) | Limites de assinatura | Histórico de consumo | Indicador de sessão em execução |
|---|---|---|---|---|---|
| Claude Code | `~/.claude/settings.json` | `~/.claude/settings.json` | sim | sim | sim |
| Codex | `~/.codex/config.toml` | `~/.codex/hooks.json` | sim | sim | — |
| Gemini CLI | — | — | — | — | — |
| OpenCode | — | — | — | — | — |
| Claude Desktop | — | — | — | — | — |
| Hermes Agent | — | — | — | — | — |
| Pi | `~/.pi/agent/models.json` | — | — | sim | — |
| Crush | `~/.config/crush/crushrc` | `~/.config/crush/crushrc` | — | — | — |

<a id="config-overrides"></a>

## Variáveis de configuração

As variáveis abaixo movem arquivos dos agentes. O lazyagents usa as mesmas para procurar onde a CLI procura; valores relativos são ignorados.

| Agente | Variável | O que move |
|---|---|---|
| Claude Code | `CLAUDE_CONFIG_DIR` | `~/.claude`: sessões, skills, configurações, hooks e autenticação |
| Codex | `CODEX_HOME` | `~/.codex`: sessões, skills, `config.toml` e hooks |
| OpenCode | `XDG_CONFIG_HOME` | `~/.config`: configuração e skills de `opencode/` |
| OpenCode | `XDG_DATA_HOME` | `~/.local/share`: `opencode/opencode.db` |
| OpenCode | `OPENCODE_DB` | arquivo do banco de sessões |
| Hermes Agent | `HERMES_HOME` | `~/.hermes` ou perfil de `hermes profile use`: skills e `config.yaml` |
| Hermes Agent | `LOCALAPPDATA` | só Windows: padrão `%LOCALAPPDATA%\hermes` no lugar de `~/.hermes` |
| Crush | `XDG_CONFIG_HOME` | `~/.config`: configurações/skills de `crush/` e `agents/skills` |
| Crush | `XDG_DATA_HOME` | `~/.local/share`: índice de sessões `crush/projects.json` |
| Crush | `CRUSH_GLOBAL_CONFIG` | pasta de `crushrc` e `crush.json`; não muda skills em `~/.config/crush/skills` |
| Crush | `CRUSH_GLOBAL_DATA` | o próprio `~/.local/share/crush` |
| Crush | `CRUSH_SKILLS_DIR` | substitui todos os diretórios globais de skills; só esse é lido |
| Pi | `PI_CODING_AGENT_DIR` | tudo em `~/.pi/agent` |
| Pi | `PI_CODING_AGENT_SESSION_DIR` | `<diretório do agente>/sessions` |

<a id="hook-events"></a>

## Eventos de hooks

Um hook só instala em agentes que emitem seu evento; os demais recebem `–` na TUI. Os identificadores dos eventos permanecem literais.

| Evento | Claude Code | Codex | Crush |
|---|---|---|---|
| `SessionStart` | sim | sim | — |
| `UserPromptSubmit` | sim | sim | — |
| `PreToolUse` | sim | sim | sim |
| `PostToolUse` | sim | sim | — |
| `Notification` | sim | — | — |
| `Stop` | sim | — | — |
| `SubagentStop` | sim | — | — |
| `PreCompact` | sim | sim | — |
| `SessionEnd` | sim | sim | — |
