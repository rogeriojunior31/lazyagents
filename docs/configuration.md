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
| `~/.local/share/lazyagents/operation-history.json` | recent operation metadata (0600); bounded independently of backups ([history](guide/recovery.md#recent-operation-history)) |
| `~/.local/share/lazyagents/usage-cache.json` | subscription limits cache |
| `~/.local/share/lazyagents/claude-provider-state.json` | which Claude Code `env` keys lazyagents wrote (fingerprints, never the values), so it only removes its own |
| `~/.local/share/lazyagents/pi-provider-state.json` | Pi's default provider and model before a profile replaced them, restored on clear (no secret) |
| `~/.local/share/lazyagents/transcript-index.gob` | transcript index; disposable, deleting it only makes the next load read everything again |
| `~/.local/share/lazyagents/backups/` | backups of skills, sessions and agent config files |
| `~/.local/share/lazyagents/backups/.config-backup-meta/` | configuration snapshot provenance/checksums, one 0600 JSON per new snapshot ([recovery](guide/recovery.md)) |
| `~/.local/share/lazyagents/exports/` | exported transcripts |

What lazyagents writes outside these dirs, in the agents' own files, is listed in [Safety and data](safety.md#what-lazyagents-writes).

Provider endpoint/catalog checks use the existing profile fields and create no new files or persistent cache. Catalog credentials come from the saved/typed token first, then the named `envKey` in the lazyagents process environment; they never come from native login. Network requests have fixed bounds and run only on explicit action, as described in [endpoint checks](guide/providers.md#endpoint-checks-and-model-discovery) and [Safety](safety.md#network).

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

skills:
  installTargets: [codex]         # last explicit TUI install destinations; [] = library only
  sources:                       # optional, user-maintained; no automatic scans
    - name: ECC
      url: https://github.com/affaan-m/everything-claude-code
      # ref: main                # optional branch/tag; empty follows remote default
      # subdir: skills           # optional repository-relative scope

usage:
  period: 30d                    # today | 7d | 30d | 90d | all
  view: projects                 # daily | agents | projects | models

hello:                           # a plugin's section, keyed by its id
  greeting: hi
```

lazyagents rewrites this file when changing a setting it owns: `migrate-library` sets `libraryDir`, a confirmed TUI skill install remembers `skills.installTargets`, and source management updates `skills.sources`. Comments, key order and sections it does not know survive the rewrite.

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

Tab ids are the ones the palette shows: `skills`, `sessions`, `providers`, `hooks`, `recovery`, `usage`, `agents`, plus each plugin's id. A hidden built-in tab keeps loading in the background, because Usage and Agents are fed by the others. A hidden plugin is not started at all, but its CLI command still works.

#### Edit preferences from the TUI

Open `:` and choose `settings` from any visible tab. This optional panel reads the saved theme, initial tab, tab order and visibility. The theme row also identifies the running theme; all edits apply **on the next launch**, so saving does not interrupt the current tab, start a hidden plugin or change its colors. See the [preference keys](reference/keys.md#preferences).

Move through the rows with arrows. Left/right cycles themes or visible initial tabs. On a tab row, space toggles visibility, `s` makes a visible available tab initial, and brackets move it earlier/later. At least one available tab must stay visible. Unloaded entries remain listed and retained, so a temporarily unavailable plugin's saved order/visibility is not silently dropped. An unavailable entry cannot be the initial tab; a hidden known plugin can be shown for the next launch.

`ctrl+s` writes only `theme`, `tui.startTab`, `tui.tabs` and `tui.hidden`. Saving rereads the file, preserves comments, other global/module/plugin fields and other `tui` fields such as splash, and refuses a draft if those editable preferences changed externally. Unrelated edits made while the panel was open are retained. The final atomic write also checks the file has not changed since that read. Invalid configuration or a failed write leaves the draft open with an explanation, without quoting parser input. Repeated saves use the new saved revision.

`esc` or `q` closes and discards unsaved edits; `r` reloads saved preferences and discards the draft. A save already in progress completes before closing. Reopen lazyagents to apply saved changes. No new configuration keys, mandatory setup wizard, startup download or agent configuration write is introduced.

### `usage`

The filters the Usage tab opens with: `period` and `view`. See the [usage guide](guide/usage.md).

### `skills`

`installTargets` is the list of agent IDs preselected for the next TUI skill install. Missing or `[]` means library only; it never means all agents. Only installed, skill-capable saved IDs are offered as selected; unavailable IDs produce warnings. The final confirmation shows the effective destinations. Destination warnings appear in the source picker when installing, rather than at startup. Library-only still permits visibility to agents reading the library directly. Hooks-only imports, cancelled installs, CLI installs and batch actions do not change this preference. Saving reads the current config, preserves other keys/comments and reports errors without undoing installed files. See the [skills workflow](guide/skills.md#install-from-github-a-folder-or-a-zip).

`sources` is a list of repository descriptors: unique `name`, Git `url`, optional branch/tag `ref` and optional `subdir` with forward slashes. It defaults to an empty list. `R` in Skills opens the list without fetching; explicit scan clones only the selected descriptor. Last scan/errors are session-only and never written to config. URL/ref changes affect future scans only; each installed skill keeps its own provenance. See [saved repositories](guide/skills.md#saved-source-repositories). Removing `sources` or an entry never deletes installed skills. Other keys in the skills section, including `installTargets`, survive source changes. A malformed list or a descriptor changed since review is preserved and reported instead of overwritten.

Update reviews keep temporary source clones until application/dismissal, without adding configuration keys. Progress, cancellation and review state are session-only; normal shutdown cleans staging. CLI source scans and update checks/application clean staging before returning and do not persist reviews or scan status; source/target preferences remain unchanged. Skill profile save/remove uses the existing `profiles.json`. See [update review](guide/skills.md#update-from-source).

### Plugin sections

A plugin receives the whole section named after its id when it starts. What goes in it is up to the plugin.

## Invalid values never block startup

- A `config.yaml` that does not parse is ignored: lazyagents starts with the defaults and prints a warning.
- An unknown tab id, theme or filter value becomes a warning and falls back to the default.
- Warnings print on stderr when the TUI exits. Config, theme and plugin warnings also print before the output of a CLI command; `tui:` layout warnings only show with the TUI, since the CLI has no tabs.

## Upgrading from `config.json`

Development builds before v0.1.0 kept the config in `config.json`. On the first launch lazyagents migrates it to `config.yaml` and keeps the original as `config.json.migrated`.
