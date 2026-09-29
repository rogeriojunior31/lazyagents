# Sessions

The Sessions tab puts the conversations of every agent into one list, newest first. You can resume a session in its own CLI, read it, search it, export it, name it and delete it. lazyagents only reads each CLI's session files. The only time it changes them is when you delete a session.

## Concepts

- **Session:** one conversation with an agent, with its first prompt as the title, the folder it ran in and the time it was last updated.
- **Alias:** a name you give a session. lazyagents stores it in its own file and never writes it into the CLI's data. The filter matches aliases as well as titles.
- **Live session:** a session some agent process still has open. It shows a `●` badge and cannot be deleted until you close it.
- **Transcript index:** lazyagents reads each transcript once and afterwards only the part appended since. That keeps reloads fast on large histories. The index is a disposable cache (see [Files](#files)).

## Where sessions come from

| Agent | Read from | Resumed with | Notes |
|---|---|---|---|
| Claude Code | `~/.claude/projects/<project>/*.jsonl` | `claude --resume <id>` | Live badge and token usage per session |
| Codex | `~/.codex/sessions/**/*.jsonl` (rollouts) | `codex resume <id>` | |
| Gemini CLI | `~/.gemini/history/<project>/` and `~/.gemini/tmp/<project>/` chats | `gemini --resume <id>` | The folder comes from Gemini's project map |
| OpenCode | `~/.local/share/opencode/opencode.db` | `opencode --session <id>` | Read through the `sqlite3` binary, read-only; the 500 most recent top-level sessions |
| Pi | `~/.pi/agent/sessions/<project>/*.jsonl` | `pi --session <file>` | Also `PI_CODING_AGENT_SESSION_DIR` or an absolute `sessionDir` in Pi's `settings.json`. The title is the `/name` given in Pi, else the first prompt; the transcript shows the active branch only |

Claude Desktop and Hermes Agent have no local sessions that lazyagents can read. What each agent supports beyond sessions is in the [agent reference](../reference/agents.md#capabilities).

Requirements:

- **OpenCode** needs `sqlite3` in `PATH`, both to list sessions and to read transcripts. Without it the other agents still load and the error names the missing binary.
- **Live badge:** on Linux lazyagents reads `/proc` directly. On other systems it runs `lsof` once per load, and without `lsof` no session shows as live.

## In the TUI

Every key for this tab is in the [key reference](../reference/keys.md#sessions-tab), and `?` shows it inside the TUI. The side panel shows the selected session's folder, id and tokens, when the agent records usage, plus an estimated cost when the agent is authenticated with an API key.

### Resume a session

Press `enter`. lazyagents suspends itself, runs the agent's resume command in the session's folder, and comes back when the agent exits. If the folder no longer exists, lazyagents asks for another one. To choose the folder yourself, press `R`. The prompt accepts `~`.

To copy the command instead of running it, press `c`. It shows `cd <folder> && <command>` in the status line.

The agent's binary has to be in `PATH`. An agent that cannot resume from its CLI shows an error instead.

### Read a transcript

Press `v` to open the transcript as chat cards. Inside the reader:

- `n` and `N` jump between your own prompts. `g` and `G` go to the top and the end.
- `t` shows tool commands (`❯`) one per line or summarized.
- `r` shows reasoning (`💭`) in full or only its first line.
- `esc` goes back to the list.

### Export to Markdown

In the transcript reader, press `x`. lazyagents writes the file to `~/.local/share/lazyagents/exports/` as `<agent>-<id>-<timestamp>.md`, with mode `0600`. The file has a header (agent, date, folder), then one section per turn, with tool commands as a list and reasoning as quotes. An empty transcript is not exported.

### Search every transcript

Press `F` and type a word or phrase. The search is case-insensitive and runs across the full text of all loaded sessions, in parallel. For Claude Code and Codex, files that cannot contain the text are skipped without being decoded. The list then shows only the matches, each with an excerpt around the first hit. `esc` clears the search.

`/` is a different tool: it filters the list by agent, alias, title and folder name, without opening any transcript.

### Name a session

Press `m` and type an alias. Saving an empty alias removes it. The alias shows before the title everywhere, including `lazyagents sessions`.

### Group and filter

- `g` groups the list by project (the folder name) and agent.
- `f` cycles the agent filter: all agents, then one agent at a time.
- `r` reloads from disk. lazyagents does not watch the files.

### Delete sessions

Press `d` to delete the selected session. To delete several, mark them with `space` first; on a group header, `space` marks the whole group. A confirmation lists what will be deleted before anything happens.

- **Claude Code, Codex, Gemini CLI and Pi:** the session file is copied to `~/.local/share/lazyagents/backups/` (named `<file>.<timestamp>`) before it is removed. To restore a session, copy the backup back to its original folder and remove the timestamp suffix.
- **OpenCode:** lazyagents first saves `opencode export <id>` to `~/.local/share/lazyagents/backups/opencode-<id>.<timestamp>.json` (mode 0600), then runs `opencode session delete <id>`. If the export fails, nothing is deleted. To restore, run `opencode import <file>`.
- **Live sessions** are refused: close the agent first.

## From the CLI

```sh
lazyagents sessions                          # every agent, newest first
lazyagents sessions --agent codex --limit 20
lazyagents sessions --here                   # sessions started in this folder
lazyagents sessions --json | jq '.[0].id'    # id, title, alias, cwd, updated, usage
```

`--json` includes token usage when the agent records it, and `cost_usd` only for agents authenticated with an API key. Resuming, reading and deleting sessions are TUI actions. Every option is in the [CLI reference](../reference/cli.md#sessions).

## Configuration

The Sessions tab has no `config.yaml` section. To open lazyagents on this tab, use `startTab: sessions` in the `tui:` section ([configuration](../configuration.md)).

## Files

| Path | Contents |
|---|---|
| `~/.local/share/lazyagents/session-aliases.json` | Aliases, keyed by `<agent>:<session id>` |
| `~/.local/share/lazyagents/exports/` | Exported transcripts |
| `~/.local/share/lazyagents/backups/` | Copies of deleted session files, and OpenCode exports (`opencode-<id>.<timestamp>.json`) |
| `~/.local/share/lazyagents/transcript-index.gob` | Transcript index. Deleting it only makes the next load read everything again |

## Limits

- Costs are estimates at Claude API list rates (Pi: Pi's own recorded cost), only cover known Claude models and only show for API-key accounts: a subscription does not pay per token. The [usage guide](usage.md#cost) explains the rules.
- Session formats are private to each CLI and can change between versions. A file lazyagents cannot parse is skipped, and the other sessions keep loading.
- OpenCode lists at most 500 sessions, and subagent sessions are left out.
