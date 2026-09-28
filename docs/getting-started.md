# Getting started

This page takes you from install to a skill enabled in two agents and a session resumed, in about five minutes.

## Install

With Go 1.26 or newer:

```sh
go install github.com/rogeriojunior31/lazyagents@latest
```

Or download a binary from [Releases](https://github.com/rogeriojunior31/lazyagents/releases) (Linux, macOS Intel and Apple Silicon, Windows) and check it against `SHA256SUMS`:

```sh
tar -xzf lazyagents_<version>_linux_amd64.tar.gz
./lazyagents --version
```

Nothing else is required. Some features use tools you may already have:

| Tool | Used for |
|---|---|
| `git` | installing skills from GitHub |
| `gh`, authenticated | searching GitHub for skills (`S` in the Skills tab) |
| `sqlite3` | reading OpenCode sessions |
| `lsof` | outside Linux, the "live" badge on Claude Code sessions that are running now (Linux reads `/proc`) |

## Check what lazyagents sees

```sh
lazyagents doctor
```

`doctor` lists the agents it found, validates every skill and reports broken links, hooks and plugins. An agent counts as installed when its binary is in `PATH` or its config dir exists. The full matrix of what each agent supports is in the [agent support reference](reference/agents.md).

## Open the TUI

```sh
lazyagents
```

Tabs sit at the top: `tab` and `shift+tab` move between them (a click works too), `?` lists every key of the current tab and `:` opens the command palette. `q` quits. The detail pane sits beside the table on wide terminals and below it on narrow ones; the mouse wheel scrolls and a click selects.

### Enable a skill in two agents

1. In the **Skills** tab, press `i` and type a source: a GitHub repository (`user/repo` or its URL), a local folder or a `.zip`.
2. If the source has several skills, pick which ones to install; a single skill installs directly. They are copied into your library, `~/.local/share/lazyagents/skills/`.
3. Select a skill, move to an agent column with `←/→` and press `space`. lazyagents creates a symlink in that agent's skills dir. `space` again removes it.
4. `a` enables the skill in every agent at once, `x` disables it everywhere.

Skills you already had inside an agent show up too, as local skills. `o` adopts one into the library so the other agents can use it. More in the [skills guide](guide/skills.md).

### Resume a session

1. Open the **Sessions** tab. It lists conversations from Claude Code, Codex, Gemini CLI and OpenCode, newest first.
2. Type `/` to filter, or `f` to show one agent.
3. Press `enter`: lazyagents suspends itself and resumes the session in its own CLI, in the right folder. When you exit the agent, you are back in lazyagents.

`v` reads the transcript without resuming. More in the [sessions guide](guide/sessions.md).

### See your usage

The **Usage** tab shows subscription limits (session and weekly windows, with reset times) and token consumption from your local transcripts. `p` changes the period and `←/→` the view. Claude Code limits are fetched from the network only when you open the tab or run `lazyagents usage` or `doctor`, and cached for 5 minutes. Until you have signed in to an agent or used it once, its limits show as not available yet: that is expected. More in the [usage guide](guide/usage.md).

## Use it from scripts

Every tab has a headless counterpart, and query commands take `--json`:

```sh
lazyagents list --json                     # skills and where they are enabled
lazyagents enable my-skill --all           # enable in every agent
lazyagents sessions --here --limit 5       # recent sessions in this folder
lazyagents usage daily --since 30d
```

`lazyagents help <command>` explains each command; the same text is in the [CLI reference](reference/cli.md).

## Uninstall

Undo what lazyagents placed in the agents' dirs before deleting its data, or their skill links will point to a library that no longer exists (`lazyagents doctor` lists broken links):

1. `lazyagents disable <skill>` for each enabled skill (`lazyagents list` shows them). A skill you want to keep in one agent: copy its folder from the library into that agent's skills dir first.
2. `lazyagents provider clear` and `lazyagents hooks disable <name>` for each installed hook.
3. Delete the binary, `~/.config/lazyagents` and `~/.local/share/lazyagents`. The backups dir is inside the latter: keep it if you may want anything back.

## Next steps

- Change the theme, tab order or start tab: [Configuration](configuration.md) and [Themes](themes.md).
- Point an agent at another API endpoint: [Providers](guide/providers.md).
- Run a command on agent events: [Hooks](guide/hooks.md).
- Know what lazyagents changes on disk before you trust it: [Safety and data](safety.md).
