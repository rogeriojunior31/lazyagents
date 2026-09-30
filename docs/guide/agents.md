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
- **(shared)** after a dir read by several agents (`~/.agents/skills`). lazyagents links a skill there only while it is enabled for every installed agent that reads it; see [shared directories](skills.md#concepts).

An agent that is not installed only shows a hint to install its CLI and reopen lazyagents.

## How detection works

- An agent counts as installed when its binary is in `PATH` **or** its config dir exists (for example `~/.claude` or `~/.codex`). With only the config dir, the detail says the binary is not in `PATH`: skills and sessions still work, but resuming a session needs the CLI.
- The version comes from running `<cli> --version` once, in the background, when the TUI starts (each CLI has up to 6 seconds). The CLI runs it only for the commands that need it, such as `doctor`.
- Claude Desktop is detected by its config dir only. Its skills and chats live in the claude.ai account, so there is nothing local to manage.
- Hermes Agent enables skills in `~/.hermes/skills` (`%LOCALAPPDATA%\hermes\skills` on Windows), or in the profile picked with `hermes profile use`, or in `HERMES_HOME`. It also reads the dirs its `config.yaml` lists under `skills.create_dir` and `skills.external_dirs`, resolved as Hermes does (`~` and `${VAR}` expanded, relative to the Hermes home); a dir that does not exist is left out. Listing `~/.agents/skills` there makes Hermes one of the agents that share that dir.
- Pi keeps everything in `~/.pi/agent`, or in `PI_CODING_AGENT_DIR` when set. Other tools also ship a binary named `pi`, so without that dir the binary only counts when `pi --version` prints a bare version.

- An agent whose files were moved with its own variable (`CLAUDE_CONFIG_DIR`, `CODEX_HOME`, `XDG_DATA_HOME` for OpenCode…) is found there too, as long as lazyagents runs with the same environment. The list is in the [config overrides reference](../reference/agents.md#config-overrides).

- Crush counts as installed with its binary, `~/.config/crush` or `~/.local/share/crush`. It loads skills from `~/.config/crush/skills` (where lazyagents enables them), `~/.config/agents/skills`, `~/.claude/skills` and `~/.agents/skills`. With `CRUSH_SKILLS_DIR` set, Crush loads skills from that dir only, and lazyagents enables them there.

Detection runs once per launch. After installing a CLI, reopen lazyagents.

From the CLI, `lazyagents doctor` prints the same detection in its first section (`--json` includes the version).

## Keys

`↑/↓` selects an agent and `shift+↑/↓` scrolls the detail. The full list is in the [key reference](../reference/keys.md#agents-tab).

## Asking for a new agent

An agent is supported through an adapter in `internal/agent`, the only place that knows its paths and file formats. To ask for one, open an issue with the agent's name, where it keeps skills, sessions and config, and links to its docs. To write the adapter yourself, see [adding an agent](../architecture.md#adding-an-agent).
