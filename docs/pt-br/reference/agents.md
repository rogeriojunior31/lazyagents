# Suporte dos agentes
<!-- source: 9aa0a039d665 -->

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
| Gemini CLI | `~/.gemini/settings.json` | — | — | sim | — |
| OpenCode | `~/.config/opencode/opencode.json` | — | — | sim | — |
| Claude Desktop | — | — | — | — | — |
| Hermes Agent | — | — | — | — | — |
| Pi | `~/.pi/agent/models.json` | — | — | sim | — |
| Crush | `~/.config/crush/crushrc` | `~/.config/crush/crushrc` | — | — | — |

<a id="config-overrides"></a>

## Variáveis de configuração

As variáveis abaixo movem arquivos dos agentes. O lazyagents usa as mesmas para procurar onde a CLI procura; valores relativos são ignorados.

| Agente | Variável | O que move |
|---|---|---|
| Gemini CLI | `GEMINI_CLI_HOME` | home que contém `.gemini/` e `.agents/skills` compartilhadas |
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

<a id="recorded-compatibility-evidence"></a>

## Evidências de compatibilidade gravadas

Estas versões têm resultados esperados dos adaptadores preservados a partir de gravações nativas. Eles verificam os formatos gravados de sessão/resposta, não todos os recursos acima. Configurações de entrada do gravador não são padrões nativos. Argumentos de retomada são snapshots, não uma validação completa de retomada/exclusão. Detectar um agente ou gravar um arquivo não acrescenta suporte a providers, hooks ou uso. A apresentação de ferramentas e os horários das sessões Gemini ainda precisam de refinamento; Hermes ainda não tem gravação de uma CLI executável. Veja reprodução e detalhes de origem na [matriz de fixtures](../../../internal/agent/testdata/fixtures/README.md). Sistema operacional/arquitetura ausentes das gravações antigas continuam desconhecidos.

| Agente | Versão | Gravado em | Sistema/arquitetura | Evidência |
|---|---|---|---|---|
| `claude-code` | 2.1.284 | 2026-09-30 | desconhecido | golden de sessão/conversa |
| `claude-code` | 2.1.301 | 2026-10-18 | linux/amd64 | golden de sessão/conversa |
| `codex` | 0.156.1 | 2026-09-30 | desconhecido | formato de limites com valores sintéticos |
| `codex` | 0.158.0 | 2026-09-30 | desconhecido | golden de sessão/conversa |
| `codex` | 0.163.0 | 2026-10-18 | linux/amd64 | golden de sessão/conversa |
| `crush` | 0.96.1 | 2026-09-30 | desconhecido | golden de sessão/conversa |
| `crush` | 0.98.1 | 2026-10-18 | linux/amd64 | golden de sessão/conversa |
| `gemini-cli` | 0.64.0 | 2026-10-18 | linux/amd64 | golden de sessão/conversa; aplicação/limpeza de provider (GenerateContent, credenciais sintéticas/inferência local) |
| `opencode` | 1.18.33 | 2026-09-30 | desconhecido | golden de sessão/conversa |
| `opencode` | 2.0.32 | 2026-10-18 | linux/amd64 | golden de sessão/conversa; aplicação/limpeza de provider (Chat Completions, credenciais fictícias/inferência local) |
| `pi` | 0.87.1 | 2026-09-30 | desconhecido | golden de sessão/conversa |
| `pi` | 1.3.0 | 2026-10-18 | linux/amd64 | golden de sessão/conversa |
