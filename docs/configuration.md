# Configuration

Everything is optional: lazyagents runs with no config file. When you want to change something, create `config.yaml` in the config dir.

## Where files live

| | Linux | macOS | Windows |
|---|---|---|---|
| Config dir | `$XDG_CONFIG_HOME/lazyagents`, default `~/.config/lazyagents` | `~/Library/Application Support/lazyagents` | `%AppData%\lazyagents` |
| Data dir | `$XDG_DATA_HOME/lazyagents`, default `~/.local/share/lazyagents` | same as Linux | same as Linux |

The rest of the docs use the Linux paths.

| Path | Contents |
|---|---|
| `~/.config/lazyagents/config.yaml` | this configuration |
| `~/.config/lazyagents/providers.json` | provider profiles (mode 0600, may hold tokens) |
| `~/.config/lazyagents/plugins/` | external plugins, one executable each ([plugins guide](guide/plugins.md)) |
| `~/.config/lazyagents/themes/` | your themes, `<id>.yaml` ([themes](themes.md)) |
| `~/.local/share/lazyagents/skills/` | the skill library (movable, see `libraryDir`) |
| `~/.local/share/lazyagents/profiles.json` | skill activation profiles |
| `~/.local/share/lazyagents/hooks/` | the hook library: one JSON per hook, plus imported scripts |
| `~/.local/share/lazyagents/session-aliases.json` | session aliases |
| `~/.local/share/lazyagents/usage-cache.json` | subscription limits cache |
| `~/.local/share/lazyagents/claude-provider-state.json` | which Claude Code `env` keys lazyagents wrote (fingerprints, never the values), so it only removes its own |
| `~/.local/share/lazyagents/pi-provider-state.json` | Pi's default provider and model before a profile replaced them, restored on clear (no secret) |
| `~/.local/share/lazyagents/transcript-index.gob` | transcript index; disposable, deleting it only makes the next load read everything again |
| `~/.local/share/lazyagents/backups/` | backups of skills, sessions and agent config files |
| `~/.local/share/lazyagents/exports/` | exported transcripts |

What lazyagents writes outside these dirs, in the agents' own files, is listed in [Safety and data](safety.md#what-lazyagents-writes).

## config.yaml

```yaml
theme: sp-night-garoa           # a built-in theme or one of yours
libraryDir: ~/.agents/skills     # where the skill library lives

tui:
  splash: true                   # false skips the start screen
  splashSeconds: 2               # how long it stays (0 skips; at most 10)
  startTab: sessions             # tab the app opens on
  tabs: [sessions, skills, usage]    # order; unlisted tabs follow in the default order
  hidden: [hooks, providers]         # off the tab bar and the palette

usage:
  period: 30d                    # today | 7d | 30d | 90d | all
  view: projects                 # daily | agents | projects | models

hello:                           # a plugin's section, keyed by its id
  greeting: hi
```

lazyagents rewrites this file only when a command changes a setting it owns (today `migrate-library`, which sets `libraryDir`). Comments, key order and sections it does not know survive the rewrite.

### Top-level keys

| Key | Default | Effect |
|---|---|---|
| `theme` | `sp-night` | The TUI theme: a [built-in id](reference/themes.md) or the id of a file in `themes/`. Read at startup. An unknown id falls back to `sp-night` with a warning. The ids used before v0.3 (`noite`, `garoa`, `jaragua`) still work |
| `libraryDir` | `~/.local/share/lazyagents/skills` | Where the skill library lives. Prefer `lazyagents migrate-library <dir>`, which moves the skills and recreates the symlinks ([skills guide](guide/skills.md)) |

Only these two keys are global. Every other top-level key is a section owned by one module or plugin, named after its id.

### `tui`: tab layout

| Key | Effect |
|---|---|
| `splash` | `false` skips the start screen |
| `splashSeconds` | How long the start screen stays, at most 10; `0` skips it |
| `startTab` | The tab the app opens on |
| `tabs` | Tab order. Tabs you leave out follow in the default order |
| `hidden` | Tabs removed from the tab bar and the palette |

Tab ids are the ones the palette shows: `skills`, `sessions`, `providers`, `hooks`, `usage`, `agents`, plus each plugin's id. A hidden built-in tab keeps loading in the background, because Usage and Agents are fed by the others. A hidden plugin is not started at all, but its CLI command still works.

### `usage`

The filters the Usage tab opens with: `period` and `view`. See the [usage guide](guide/usage.md).

### Plugin sections

A plugin receives the whole section named after its id when it starts. What goes in it is up to the plugin.

## Invalid values never block startup

- A `config.yaml` that does not parse is ignored: lazyagents starts with the defaults and prints a warning.
- An unknown tab id, theme or filter value becomes a warning and falls back to the default.
- Warnings print on stderr when the TUI exits. Config, theme and plugin warnings also print before the output of a CLI command; `tui:` layout warnings only show with the TUI, since the CLI has no tabs.

## Upgrading from `config.json`

Development builds before v0.1.0 kept the config in `config.json`. On the first launch lazyagents migrates it to `config.yaml` and keeps the original as `config.json.migrated`.
