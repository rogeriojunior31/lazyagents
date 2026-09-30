# Troubleshooting

## Start with `lazyagents doctor`

```sh
lazyagents doctor          # readable report, exits 1 when something is wrong
lazyagents doctor --json   # the same, for scripts and bug reports
```

It lists the detected agents and checks skills, broken symlinks, providers, hooks, usage limits and plugins. Most entries below match a line it prints. When you open an issue, include its output (see [the CLI reference](reference/cli.md#doctor)).

Notices about the config, themes or plugins are printed on stderr: before the output of a CLI command, and after you quit the TUI.

## Agents

### An agent shows as not installed

**Cause:** lazyagents looks for the agent's binary in `PATH` and for its config dir (`~/.claude`, `~/.codex`, `~/.gemini`, `~/.config/opencode`, `~/.hermes`, `~/.pi/agent`), or the dir its [config variable](reference/agents.md#config-overrides) names, such as `CODEX_HOME`. Neither was found. lazyagents reads that variable from the environment it was started in, so start it from the same shell as the agent.

**Fix:** install the CLI, or make sure it is in the `PATH` of the shell that starts lazyagents, then reopen lazyagents: detection runs once per launch. The [Agents guide](guide/agents.md#how-detection-works) has the rules.

### The agent is detected but has no version, or `enter` says the CLI "is not in PATH"

**Cause:** the config dir exists but the binary is not in `PATH`. Skills and sessions still work; resuming a session needs the CLI.

**Fix:** add the binary's dir to `PATH`. The Agents tab detail shows `config in … (binary not in PATH)` in this case.

## Skills

### "installing from GitHub requires git in PATH"

**Fix:** install `git`. Installing from a local folder or a `.zip` works without it.

### `git clone …` fails

The message carries git's own error. Check the repository name (`user/repo` or a full URL), your network, and, for private repositories, your git credentials.

### "no SKILL.md found in …"

The source has no `SKILL.md` at any depth. A skill is a folder with a `SKILL.md` file; check that you pointed at the right repository or folder.

### "searching requires the GitHub CLI (gh) in PATH, logged in"

Search (`S` in the Skills tab) uses `gh api`. Install the [GitHub CLI](https://cli.github.com/) and run `gh auth login`. Other errors from GitHub show as `GitHub search: …`.

### "… is local (unmanaged) — o to adopt"

**Cause:** the skill is a real folder (or another tool's symlink) inside the agent's skills dir, not a link to the lazyagents library. lazyagents never deletes real content, so it will not disable it.

**Fix:** adopt it (`o`, or `A` for all; `lazyagents adopt <skill> --agent <id>`). The folder moves into the library and a symlink takes its place, so you can enable it in other agents too. A third-party symlink cannot be adopted: manage it with the tool that created it.

### "a skill with this name already exists" when enabling

Something with that name is already in the agent's skills dir. The message ends with its path. Remove or rename it, or adopt it if it is the same skill.

### A skill shows `◆` or turns on in agents you did not pick

OpenCode also reads `~/.claude/skills`, so a skill enabled for Claude Code shows up in OpenCode with `◆`. A skill placed in `~/.agents/skills` by hand, or by an older lazyagents version, is visible to Codex, Gemini CLI, OpenCode and Pi: disable it for the agents that should not have it and lazyagents moves it to the own directory of the others. See the [agent support reference](reference/agents.md#skills).

### `doctor` reports "broken symlink"

A link in an agent's skills dir points to a folder that no longer exists, usually after a library was moved or deleted by hand. Remove the link, or reinstall the skill and enable it again. Removals made by lazyagents keep a `.tar.gz` backup that `b` in the Skills tab restores.

### `migrate-library` refuses to run

| Message | Meaning |
|---|---|
| `destination already exists or is inaccessible: <path>` | a skill with the same name (or a symlink) is already in the new dir. Nothing is assumed to be migrated: move or remove it, then retry |
| `libraries cannot contain each other` / `libraries overlap after resolving symlinks` | the new dir is inside the old one, or the reverse. Pick a separate dir |

When a migration fails partway, the copies are rolled back and the source is kept.

## Sessions

### OpenCode sessions do not show: "opencode sessions live in SQLite: install sqlite3 to list them"

OpenCode keeps sessions in `opencode.db`. Install the `sqlite3` command-line tool.

## Providers

### "TOML config has a multiline string: automatic editing is not supported; file left untouched"

lazyagents edits Codex's `config.toml` line by line, without rewriting the rest, and refuses files with multiline strings (`"""` or `'''`). Nothing was changed. Replace the multiline strings with single-line ones, or apply the provider by hand.

### "table model_providers.lazyagents already exists outside the managed block"

A `[model_providers.lazyagents]` table was written by hand outside the block lazyagents manages. Remove or rename that table and apply again.

### `doctor`: "provider applied to an agent that is not installed"

The agent's config still carries a provider, but the agent is gone. Clear it (`x` in the Providers tab, or `lazyagents provider clear --agent <id>`) or reinstall the agent.

## Hooks

### A hook installed in Codex never runs

Codex has two gates of its own, and lazyagents deliberately writes neither:

- **"hooks are off in Codex: set hooks = true under [features] in config.toml"**: add it to `~/.codex/config.toml`.
- **"a new hook only runs after you confirm trust in Codex itself"**: Codex asks you to approve each new hook command; accept it in Codex.

That approval is your consent to run the command, so only you can give it. See [safety](safety.md).

### `doctor`: "<command>: not in PATH", "not found" or "not executable"

The hook's command cannot run as installed. Fix the path or permissions of the script, or edit the hook in `~/.local/share/lazyagents/hooks/`.

## Usage

### Subscription limits do not show

An agent that has nothing to report is left out of the limits and of `doctor`, with no message:

- **Claude Code** not installed, not signed in (run `/login`), or signed in with an API key, which has no subscription limits (cost is shown instead).
- **Codex** never used on this machine, or no limits recorded yet: Codex writes its limits into its session files, so use it once and refresh.

| Message | Fix |
|---|---|
| `Claude Code session expired: open the CLI to renew it` | open Claude Code once |

Limits are cached for 5 minutes. Press `r` in the Usage tab or run `lazyagents usage limits --refresh` to fetch them again. Claude Code limits need network; Codex limits are read from local files.

### Numbers look stale or wrong after an update

Transcript data is cached in `~/.local/share/lazyagents/transcript-index.gob`. An index from another version or a damaged one is discarded and rebuilt automatically. It is safe to delete: the next launch reads every transcript again, which may take a moment.

## Configuration and themes

### "parsing config: …; using defaults"

`config.yaml` is not valid YAML. lazyagents starts with the defaults and does not touch the file. Fix the syntax; the [configuration guide](configuration.md) lists every key.

### "config tui: …"

An unknown tab id or value in the `tui:` section. It is only a warning: the rest of the layout applies.

### "unknown theme: <id>"

The full notice is `unknown theme: <id> in <config path>; using "sp-night"`. The `theme:` value is neither a built-in theme nor a file in `~/.config/lazyagents/themes/`. Check the id against the [built-in themes](reference/themes.md). A broken user theme file prints its own notice with the reason, such as a missing role or an invalid color.

## Plugins

### "plugin <file> ignored: …"

| Reason | Fix |
|---|---|
| `not executable` | `chmod +x` the file |
| `name must match …` | rename it: lowercase letters, digits, `-` and `_`, up to 32 characters |
| `duplicate id` | two files share a name without extension; keep one |
| `id reserved by a built-in tab or command` | rename it so it does not clash with `skills`, `list`, `doctor`… |

### A plugin tab shows an error

The plugin did not answer within 3 seconds, wrote something that is not a protocol line, or exited. The tab shows the reason and the end of the plugin's stderr. Fix the plugin and press `r` (or `:reload`) to restart it. Run `lazyagents doctor` to see the handshake result outside the TUI. The [plugins guide](guide/plugins.md) has debugging tips.

## Still stuck?

Open an issue with the output of `lazyagents doctor --json`, your OS, and the agents and versions involved. Remove anything private (paths, provider URLs) before posting.
