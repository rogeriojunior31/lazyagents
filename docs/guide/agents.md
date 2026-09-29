# Agents

The Agents tab is a read-only overview of every agent lazyagents knows: whether it is installed here, which version, and what lazyagents can manage in it. Use it to check why an agent is missing from another tab, or where its skills live.

What each agent supports (skill dirs, providers, hooks, limits, hook events) is generated from the code in the [agent support reference](../reference/agents.md). This page explains how to read the tab.

## The table

One row per agent, installed ones first:

| Column | Meaning |
|---|---|
| agent | name, in the agent's color; `○` and dimmed when not installed |
| version | first line of `<cli> --version` |
| skills | skills enabled in the agent; `—` when the agent has no local skills dir |
| sessions | sessions found on disk for that agent |
| hooks · provider · usage | `✓` when lazyagents can manage hooks, apply a provider profile, or read usage for that agent |

The counter on the tab is the number of installed agents. On a narrow terminal (under 75 columns) the three capability columns move into the detail pane as a `manages` line.

## The detail pane

Below the table, for the selected agent:

- **skills in**: the dir where lazyagents creates skill symlinks for this agent.
- **reads too**: other skill dirs the agent loads. A skill there shows in the agent even if lazyagents did not enable it.
- **detection**: the binary path, or the config dir when the binary is not in `PATH`.
- **⚠ warning** when the managed dir is shared with other agents. `~/.agents/skills` is read by Codex, Gemini CLI, OpenCode and Pi, so a skill enabled for Codex also shows up in the other three. The Skills tab marks those cells with `◆`.

An agent that is not installed only shows a hint to install its CLI and reopen lazyagents.

## How detection works

- An agent counts as installed when its binary is in `PATH` **or** its config dir exists (for example `~/.claude` or `~/.codex`). With only the config dir, the detail says the binary is not in `PATH`: skills and sessions still work, but resuming a session needs the CLI.
- The version comes from running `<cli> --version` once, in the background, when the TUI starts (each CLI has up to 6 seconds). The CLI runs it only for the commands that need it, such as `doctor`.
- Claude Desktop is detected by its config dir only. Its skills and chats live in the claude.ai account, so there is nothing local to manage.
- Hermes Agent announces `~/.hermes/skills` only; extra skill dirs set in its own config are not detected yet.
- Pi keeps everything in `~/.pi/agent`, or in `PI_CODING_AGENT_DIR` when set. Other tools also ship a binary named `pi`, so without that dir the binary only counts when `pi --version` prints a bare version.

Detection runs once per launch. After installing a CLI, reopen lazyagents.

From the CLI, `lazyagents doctor` prints the same detection in its first section (`--json` includes the version).

## Keys

`↑/↓` selects an agent and `shift+↑/↓` scrolls the detail. The full list is in the [key reference](../reference/keys.md#agents-tab).

## Asking for a new agent

An agent is supported through an adapter in `internal/agent`, the only place that knows its paths and file formats. To ask for one, open an issue with the agent's name, where it keeps skills, sessions and config, and links to its docs. To write the adapter yourself, see [adding an agent](../architecture.md#adding-an-agent).
