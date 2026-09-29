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

## Capabilities

| Agent | Providers (writes) | Hooks (writes) | Subscription limits | Usage history | Live-session badge |
|---|---|---|---|---|---|
| Claude Code | `~/.claude/settings.json` | `~/.claude/settings.json` | yes | yes | yes |
| Codex | `~/.codex/config.toml` | `~/.codex/hooks.json` | yes | yes | — |
| Gemini CLI | — | — | — | — | — |
| OpenCode | — | — | — | — | — |
| Claude Desktop | — | — | — | — | — |
| Hermes Agent | — | — | — | — | — |
| Pi | — | — | — | yes | — |

## Hook events

A hook only installs in agents that fire its event; the TUI marks the others with `–`.

| Event | Claude Code | Codex |
|---|---|---|
| `SessionStart` | yes | yes |
| `UserPromptSubmit` | yes | yes |
| `PreToolUse` | yes | yes |
| `PostToolUse` | yes | yes |
| `Notification` | yes | — |
| `Stop` | yes | — |
| `SubagentStop` | yes | — |
| `PreCompact` | yes | yes |
| `SessionEnd` | yes | yes |
