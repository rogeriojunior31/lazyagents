<!-- Generated from the code by `go test ./docs -update`. Do not edit by hand. -->

# Agent support

What lazyagents can do with each agent, read from the adapters in `internal/agent`. Paths are the defaults on Linux and macOS. How sessions are read per agent is in the [sessions guide](../guide/sessions.md#where-sessions-come-from).

## Skills

A directory marked (shared) is read by several agents: lazyagents links a skill there only while it is enabled for every installed agent that reads it, and otherwise in each agent's own directory.

| Agent | id | Enables skills in | Also reads |
|---|---|---|---|
| Claude Code | `claude-code` | `~/.claude/skills` | — |
| Codex | `codex` | `~/.codex/skills` | `~/.agents/skills` (shared) |
| Gemini CLI | `gemini-cli` | `~/.gemini/skills` | `~/.agents/skills` (shared) |
| OpenCode | `opencode` | `~/.config/opencode/skills` | `~/.claude/skills`, `~/.agents/skills` (shared) |
| Claude Desktop | `claude-desktop` | — (not manageable locally) | — |
| Hermes Agent | `hermes-agent` | `~/.hermes/skills` | — |
| Pi | `pi` | `~/.pi/agent/skills` | `~/.agents/skills` (shared) |
| Crush | `crush` | `~/.config/crush/skills` | `~/.config/agents/skills`, `~/.claude/skills`, `~/.agents/skills` (shared) |

## Capabilities

| Agent | Providers (writes) | Hooks (writes) | Subscription limits | Usage history | Live-session badge |
|---|---|---|---|---|---|
| Claude Code | `~/.claude/settings.json` | `~/.claude/settings.json` | yes | yes | yes |
| Codex | `~/.codex/config.toml` | `~/.codex/hooks.json` | yes | yes | — |
| Gemini CLI | — | — | — | — | — |
| OpenCode | `~/.config/opencode/opencode.json` | — | — | — | — |
| Claude Desktop | — | — | — | — | — |
| Hermes Agent | — | — | — | — | — |
| Pi | `~/.pi/agent/models.json` | — | — | yes | — |
| Crush | `~/.config/crush/crushrc` | `~/.config/crush/crushrc` | — | — | — |

## Config overrides

Environment variables that move an agent's files. lazyagents reads the same ones, so it looks where the CLI does; a relative value is ignored.

| Agent | Variable | Moves |
|---|---|---|
| Claude Code | `CLAUDE_CONFIG_DIR` | `~/.claude`: sessions, skills, settings, hooks, login |
| Codex | `CODEX_HOME` | `~/.codex`: sessions, skills, `config.toml`, hooks |
| OpenCode | `XDG_CONFIG_HOME` | `~/.config`: `opencode/` config and skills |
| OpenCode | `XDG_DATA_HOME` | `~/.local/share`: `opencode/opencode.db` |
| OpenCode | `OPENCODE_DB` | the session database file itself |
| Hermes Agent | `HERMES_HOME` | `~/.hermes` (or the `hermes profile use` profile): skills and `config.yaml` |
| Hermes Agent | `LOCALAPPDATA` | Windows only: Hermes' default home is `%LOCALAPPDATA%\hermes` instead of `~/.hermes` |
| Crush | `XDG_CONFIG_HOME` | `~/.config`: `crush/` config and skills, and `agents/skills` |
| Crush | `XDG_DATA_HOME` | `~/.local/share`: `crush/projects.json`, the index of project sessions |
| Crush | `CRUSH_GLOBAL_CONFIG` | the dir of `crushrc` and `crush.json` (not the skills, which stay in `~/.config/crush/skills`) |
| Crush | `CRUSH_GLOBAL_DATA` | `~/.local/share/crush` itself |
| Crush | `CRUSH_SKILLS_DIR` | replaces every global skill dir: Crush then loads skills from that dir only |
| Pi | `PI_CODING_AGENT_DIR` | `~/.pi/agent`: everything |
| Pi | `PI_CODING_AGENT_SESSION_DIR` | `<agent dir>/sessions` |

## Hook events

A hook only installs in agents that fire its event; the TUI marks the others with `–`.

| Event | Claude Code | Codex | Crush |
|---|---|---|---|
| `SessionStart` | yes | yes | — |
| `UserPromptSubmit` | yes | yes | — |
| `PreToolUse` | yes | yes | yes |
| `PostToolUse` | yes | yes | — |
| `Notification` | yes | — | — |
| `Stop` | yes | — | — |
| `SubagentStop` | yes | — | — |
| `PreCompact` | yes | yes | — |
| `SessionEnd` | yes | yes | — |

## Recorded compatibility evidence

These versions have retained adapter goldens from native recordings. They verify the recorded session/response formats, not every capability above. Recorder configuration inputs are not native defaults. Resume arguments are snapshots, not end-to-end resume/delete verification. No new provider, hook or usage support follows from detection or from a recorded file. Gemini tool-call rendering and session timestamps still need refinement; Hermes has no runnable CLI recording yet. See the [fixture matrix](../../internal/agent/testdata/fixtures/README.md) for reproduction and origin details. Missing historical OS/architecture stays unknown.

| Agent | Version | Recorded | OS/architecture | Evidence |
|---|---|---|---|---|
| `claude-code` | 2.1.284 | 2026-09-30 | unknown | session/transcript golden |
| `claude-code` | 2.1.301 | 2026-10-18 | linux/amd64 | session/transcript golden |
| `codex` | 0.156.1 | 2026-09-30 | unknown | limits shape with synthetic values |
| `codex` | 0.158.0 | 2026-09-30 | unknown | session/transcript golden |
| `codex` | 0.163.0 | 2026-10-18 | linux/amd64 | session/transcript golden |
| `crush` | 0.96.1 | 2026-09-30 | unknown | session/transcript golden |
| `crush` | 0.98.1 | 2026-10-18 | linux/amd64 | session/transcript golden |
| `gemini-cli` | 0.64.0 | 2026-10-18 | linux/amd64 | session/transcript golden |
| `opencode` | 1.18.33 | 2026-09-30 | unknown | session/transcript golden |
| `opencode` | 2.0.32 | 2026-10-18 | linux/amd64 | session/transcript golden; provider apply/clear (Chat Completions, synthetic credentials/local inference) |
| `pi` | 0.87.1 | 2026-09-30 | unknown | session/transcript golden |
| `pi` | 1.3.0 | 2026-10-18 | linux/amd64 | session/transcript golden |
