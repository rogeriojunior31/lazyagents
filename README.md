<p align="center"><img src="docs/assets/logo.svg" width="180" alt="lazyagents logo: a sloth asleep in a hammock while three robots carry mini terminals"></p>

# lazyagents

[![CI](https://github.com/rogeriojunior31/lazyagents/actions/workflows/ci.yml/badge.svg)](https://github.com/rogeriojunior31/lazyagents/actions/workflows/ci.yml)

A Go TUI to manage, in one place, what your AI coding agents use: **skills**, **sessions**, **usage**, **providers** and **hooks**. Works with Claude Code, Codex, Gemini CLI, OpenCode, Claude Desktop and Hermes Agent. Inspired by [cc-switch](https://github.com/farion1231/cc-switch) and lazygit.

![demo](demo.gif)

```sh
go install github.com/rogeriojunior31/lazyagents@latest
lazyagents          # open the TUI
lazyagents doctor   # diagnostics without the TUI
```

## What it does

**Skills**
- **Skill × agent matrix:** every skill in every agent, toggled per agent (`space` on the picked column, or `1-9`) or in all of them at once (`a`, `x`).
- **Install** (`i`) from a GitHub repository (`user/repo` or URL), a local folder or a `.zip`, with recursive discovery and a picker for what to install. Claude Code marketplace repositories (`.claude-plugin/marketplace.json`) are read through the manifest, with skills grouped by plugin.
- **GitHub search** (`S`) for repositories with a `SKILL.md`, through `gh api`.
- **Adoption** (`o`, `A`): a skill that already lives inside an agent moves into the library and becomes a symlink, ready for the other agents.
- **Lifecycle:** create (`n`), edit in `$EDITOR` (`e`), update from source (`u`, `U`), per-agent profiles (`p`), backups and restore (`b`), `SKILL.md` lint.

**Sessions**
- **One history** for Claude Code, Codex, Gemini CLI and OpenCode; `enter` suspends the TUI and resumes the session in its own CLI, in the right folder.
- **Transcript** as chat cards (`v`), Markdown export (`x`), full-text search (`F`).
- **Organization:** your own alias (`m`, stored by lazyagents without touching the CLI's file, and used by the filter), grouping by agent and project (`g`), agent filter (`f`), tokens and estimated cost, badge for live sessions.
- **Cleanup:** delete with backup (`d`), also in batches (`space`).

**Usage**
- **Subscription limits** per agent: session and week bars with percentage used and reset time, plus plan and account mode. Codex reads its own rollouts, no network; Claude Code queries the same endpoint as `/usage`, only when you open the tab, with a cache.
- **Filterable consumption** from the transcripts: period (`p`: today, 7, 30, 90 days or all), agent (`a`), view (`←/→`: by day, agent, project or model) and text (`/` filters rows by name), with a sparkline, the current 5h block and each row's share. The filters are also in the palette (`:usage period 30d`, `:usage view projects`) and the default comes from `config.yaml`. Dollar cost only shows for API-key accounts, which pay per token.

**Providers**
- **Endpoint and model profiles** applied to each CLI's live config, cc-switch style: a profile × agent matrix, `space`/`1-9` applies (again clears), and every write goes through a confirm that shows the file and what changes. Automatic backup before writing. Claude Code gets the `env` block of `settings.json`; Codex gets `model_provider` and `[model_providers.lazyagents]` in `config.toml`, in delimited blocks that preserve the rest of the file.
- **Tokens never show**: they live in `providers.json` (0600); Claude Code gets the token in `settings.json`. For Codex, set `--env-key VAR` and export the variable: the saved token is not copied into the TOML. The TUI and `--json` only show `token ✓`; plain text only with `provider list --reveal`.

**Hooks**
- **Imported with skills:** a repository that ships `hooks/hooks.json` (Claude Code's plugin format) shows up in the same `i` picker, unchecked — installing a hook means running a third-party command on every event. The folders the commands reference are copied into the library with their layout, and `${CLAUDE_PLUGIN_ROOT}` points to that copy (and is exported, for scripts that read it). From the CLI: `lazyagents install <source> --hooks`.
- **Its own library** of hooks (`~/.local/share/lazyagents/hooks/`, one JSON per hook) installed per agent: a hook × agent matrix, `space`/`1-9` installs (again uninstalls), and a confirm showing `event → command` before rewriting the config, with backup. An agent that does not fire that event is marked `–`.
- **Hooks that are not yours are left alone:** lazyagents recognizes its own by the event + matcher + command triple and counts the others separately. In Codex it installs and warns — the `trusted_hash`, your confirmation that the command may run, is written by Codex itself.

**Agents**
- A diagnostics table of every agent: version, active skills, sessions and hooks/provider/usage support; the detail shows the skill dirs it reads, with a warning when a dir is shared between agents.

**Plugins**
- Any executable in `~/.config/lazyagents/plugins/` becomes a tab, a subcommand (`lazyagents <id> …`) and a `doctor` section. JSON Lines contract, any language: [docs/plugins.md](docs/plugins.md), example in [examples/plugins/hello](examples/plugins/hello).

**Headless CLI** for scripts and for the agents themselves: `lazyagents help`.

## Install

```sh
go install github.com/rogeriojunior31/lazyagents@latest
```

Or download a prebuilt binary from [Releases](https://github.com/rogeriojunior31/lazyagents/releases) — Linux, macOS (Intel and Apple Silicon) and Windows, with `SHA256SUMS` to verify. Changes between versions are in the [CHANGELOG](CHANGELOG.md).

```sh
tar -xzf lazyagents_<version>_linux_amd64.tar.gz && ./lazyagents
```

From a clone: `go build -o lazyagents . && ./lazyagents`.

Requirements: Go 1.26+ to build. Optional at runtime: `git` to install skills from GitHub, an authenticated `gh` for search, `sqlite3` for OpenCode sessions and `lsof` for the live-session badge.

## How it works

The central library lives in `~/.local/share/lazyagents/skills/` (XDG). Enabling a skill in an agent creates a **symlink** in that agent's skills dir; disabling removes the symlink. Real content, such as local skills and other tools' symlinks, is never deleted. Removals and updates leave a `.tar.gz` backup in `~/.local/share/lazyagents/backups/`.

| Agent | Manages in | Also reads |
|---|---|---|
| Claude Code | `~/.claude/skills` | — |
| Codex | `~/.agents/skills` ⚠ shared | `~/.codex/skills` |
| Gemini CLI | `~/.gemini/skills` | `~/.agents/skills` |
| OpenCode | `~/.config/opencode/skills` | `~/.claude/skills`, `~/.agents/skills` |
| Hermes Agent | `~/.hermes/skills` | — (custom external dirs are not detected yet) |
| Claude Desktop | — (skills and chats live in the claude.ai account) | — |

⚠ `~/.agents/skills` is the cross-agent dir: Codex, Gemini and OpenCode read it. The matrix shows this (`◆` = visible through a shared dir). To use it as the library: `lazyagents migrate-library ~/.agents/skills`.

Session formats read (deleted only on explicit action, with backup): Claude Code (`~/.claude/projects/*.jsonl`), Codex (`~/.codex/sessions/`), Gemini (`~/.gemini/{history,tmp}/*/chats/`), OpenCode (`opencode.db` through `sqlite3`).

## Configuration

Everything is optional. On Linux, in `~/.config/lazyagents/config.yaml` (or `$XDG_CONFIG_HOME/lazyagents/`). On macOS the config goes in `~/Library/Application Support/lazyagents/`; on Windows, `%AppData%/lazyagents/`. Data uses `$XDG_DATA_HOME/lazyagents/`, falling back to `~/.local/share/lazyagents/`. The examples below use the Linux layout. Comments and keys lazyagents does not know survive when it rewrites the file. A `config.json` from earlier versions is migrated on first launch (the original is kept as `config.json.migrated`).

```yaml
theme: sp-night-garoa        # built-in (see Themes) or the id of a file in themes/
libraryDir: ~/.agents/skills

usage:
  period: 30d                   # filters the Usage tab opens with: today | 7d | 30d | 90d | all
  view: projects                # daily | agents | projects | models

tui:
  splash: true                  # false skips the start screen
  splashSeconds: 2              # how long it stays (0 skips; at most 10)
  startTab: sessions            # tab the app opens on
  tabs: [sessions, skills, usage]   # order; unlisted tabs follow in the default order
  hidden: [hooks, providers]        # removed from the tab bar and the palette

# one section per module or plugin, keyed by the tab id
hello:
  greeting: hi
```

| Key | Default | Effect |
|---|---|---|
| `theme` | `sp-night` | TUI theme: a built-in one or the id of your own theme in `themes/` (see [Themes](#themes)). Applies on the next launch; an unknown value falls back to `sp-night` with a warning on exit. The old ids `noite`, `garoa` and `jaragua` still work |
| `libraryDir` | `~/.local/share/lazyagents/skills` | where the library lives. Prefer `lazyagents migrate-library <dir>`, which moves the skills and redoes the symlinks |
| `usage` | `7d`, `daily` | initial filters of the Usage tab: `period` (`today`, `7d`, `30d`, `90d`, `all`) and `view` (`daily`, `agents`, `projects`, `models`); an unknown value keeps the default |
| `tui` | — | TUI layout: `splash`, `splashSeconds`, `startTab`, `tabs` (order) and `hidden`. Tab ids: `skills`, `sessions`, `agents`, `providers`, `hooks`, `usage` and each plugin's id (the same as in the `:` palette). A hidden built-in tab keeps loading in the background (Usage and Agents depend on sessions); a hidden plugin is not even started, but its command stays in the CLI. An unknown id is a warning on exit, never an error |
| `<id>` | — | free-form section of the module or plugin `<id>`; a plugin receives it whole in `init` |

lazyagents files:

| Path | Contents |
|---|---|
| `~/.config/lazyagents/config.yaml` | configuration |
| `~/.config/lazyagents/providers.json` | provider profiles (0600, may hold tokens) |
| `~/.config/lazyagents/plugins/` | external plugins (executables) |
| `~/.config/lazyagents/themes/` | user themes (`<id>.yaml`) |
| `~/.local/share/lazyagents/skills/` | skill library |
| `~/.local/share/lazyagents/profiles.json` | activation profiles |
| `~/.local/share/lazyagents/hooks/` | hook library (one JSON per hook, plus imported scripts) |
| `~/.local/share/lazyagents/session-aliases.json` | session aliases |
| `~/.local/share/lazyagents/usage-cache.json` | subscription limits cache |
| `~/.local/share/lazyagents/transcript-index.gob` | transcript index (title, tokens, usage per 15-min slot); disposable, deleting it only makes the next load reread everything |
| `~/.local/share/lazyagents/backups/` | backups of skills and deleted sessions |
| `~/.local/share/lazyagents/exports/` | exported transcripts |

Library migration refuses targets whose names already exist, symlinks included, and overlapping dirs. Resolve the conflicts and retry; the source files are kept if preparation fails.

Costs are estimates from the standard Claude API rates, with 5-minute cache writes. An unknown model shows tokens only. In the TUI and in `sessions --json`, so does an aggregate with mixed models; `lazyagents usage` sums the cost response by response at each model's rate and shows `—` if a rate is missing. Fast mode, batch, 1-hour cache and regional rates are not priced. See the [official pricing](https://platform.claude.com/docs/en/about-claude/pricing).

Current Codex accepts `--wire-api responses` (also its default). Automatic provider edits refuse TOML with multiline strings and leave the file for a manual fix. Imported hooks keep the original command and set `CLAUDE_PLUGIN_ROOT` in the shell; payload compatibility between CLIs depends on the script. POSIX scripts need a compatible shell.

## Themes

The TUI colors come from **[SP Night](https://sp-night.github.io/)**, a palette with São Paulo as its reference. Its three flavors are the default themes and ship with the full original palette (23 colors per flavor) and every semantic role of the project (`ui`, `syntax`, `diagnostic`, `git`, `ansi`), unadapted:

| `theme` | Theme | |
|---|---|---|
| `sp-night` (default) | **SP Night** | The city at 3 a.m.: tokyodark's blue-violet dark, with the sodium streetlight burning warm on top. |
| `sp-night-garoa` | **SP Night Garoa** | The same window seen through the drizzle. Flat grey: the garoa does not cool the city, it fades it. |
| `sp-night-jaragua` | **SP Night Jaraguá** | The same night from the city's highest point: the dark turned toward the green of dense forest, with the red-and-white tower lit at the top. |

The SP Night ids used up to v0.2 (`noite`, `garoa`, `jaragua`) still work in `theme:` and `extends:`.

Well-known community themes also ship built in, with their official palettes. Where the original assignment would leave text unreadable (contrast below 3:1), the role uses another color from the same palette; only Nord, Dracula and Rosé Pine Dawn get a derived color, marked in the file:

| `theme` | Theme |
|---|---|
| `tokyonight`, `tokyonight-storm` | Tokyo Night (Night, Storm) |
| `dracula` | Dracula |
| `gruvbox-dark` | Gruvbox Dark |
| `nord` | Nord |
| `rose-pine`, `rose-pine-moon`, `rose-pine-dawn` | Rosé Pine (Main, Moon, Dawn — light) |
| `kanagawa` | Kanagawa Wave |
| `everforest-dark` | Everforest Dark |
| `onedark` | One Dark |

To switch, set `theme` in `config.yaml` (above) and reopen lazyagents.

### Your own theme

Create `~/.config/lazyagents/themes/<id>.yaml` and set `theme: <id>`. The format is the same as the built-in themes ([examples](internal/tui/theme/themes/)): an optional `palette` of named colors and the roles per group. A role takes `"#rrggbb"` (quoted) or a `palette` name. Anything missing comes from the `extends` theme (default `sp-night`), so you only change what you need:

```yaml
# ~/.config/lazyagents/themes/my-dracula.yaml
label: My Dracula
extends: dracula
palette:
  purple: "#caa9fa"        # repaints every dracula role that uses purple
ui:
  accent: "#ff79c6"        # main highlight (active tab, focused borders…)
  bg: "#1e1f29"
diagnostic:
  ok: "#50fa7b"
```

The roles are SP Night's; the full list and what each one paints in the TUI are in [internal/tui/theme/README.md](internal/tui/theme/README.md). An invalid file (bad hex, unknown role, `extends` cycle) is skipped with a warning on stderr (when the TUI exits; at the start of a CLI command), and the other themes still load. A file cannot use a built-in theme's id; to derive one, use `extends`. `appearance` (`dark`/`light`) is informative: it describes the theme but does not change the drawing.

To give your terminal and editor the same look, SP Night has ports for other tools at [sp-night.github.io](https://sp-night.github.io/). The palettes are MIT (Tokyo Night: Apache-2.0); the notices are in [internal/tui/theme/LICENSE-SP-Night](internal/tui/theme/LICENSE-SP-Night) and [internal/tui/theme/LICENSES-themes.md](internal/tui/theme/LICENSES-themes.md).

## Keys

Global: `tab`/`shift+tab` switch tabs (or click), `:` opens the command palette, `?` shows every key of the current tab, `q` quits.

| Skills | | Sessions | |
|---|---|---|---|
| `enter` | read SKILL.md | `enter` | resume in its CLI |
| `e` / `n` | edit / new skill | `v` | transcript (`x` exports) |
| `←/→` | pick agent (column) | `R` | resume in another folder |
| `space` / `1-9` | toggle in picked agent / agent N | `c` | show the resume command |
| `a` / `x` | enable / disable in all | `d` / `space` | delete with backup / select batch |
| `i` / `S` | install / search GitHub | `g` | group by agent + project |
| `o` / `A` | adopt / adopt all local skills | `f` / `F` | agent filter / search transcripts |
| `u` / `U` | update / check for updates | `/` `r` | filter / reload |
| `p` / `b` | profiles / backups | `m` | alias (empty removes it) |
| `d` | remove from the library (with backup) | `shift+↑/↓` | scroll the detail |

Providers: `←/→` picks the agent, `space` or `1-9` applies the profile to it (again clears), `a` applies to every installed agent, `x` clears all, `n` / `e` create / edit a profile (an empty token keeps the saved one), `d` deletes it. Profiles can also be created from the CLI: `lazyagents provider add <name> --base-url <url> --token -` (`-` reads the token from stdin, out of the shell history).

Hooks use the same keys, installing instead of applying; `enter` picks which commands of a pack to install and `v` opens the full-screen reader (`e` edits). Hooks are also created with `lazyagents hooks add <name> --event SessionStart --command "..."`.

Usage: `p`/`P` changes the period, `a`/`A` the agent, `←/→` (or `v`) the view, `/` filters table rows by name (`enter` keeps it, `esc` clears; `esc` again goes back to all agents), `r` refreshes the limits and `pgup`/`pgdn` scroll a page.

The detail sits beside the table from 110 columns up; below that it becomes a strip under the table, scrolled with `shift+↑/↓`.

**Mouse:** the wheel scrolls lists and readers, a click selects tabs and items, and clicking the selected item again opens it.

## CLI

```text
lazyagents help [command]                     # list commands or detail one
lazyagents list [--json]                      # or: lazyagents skills list
lazyagents enable <skill> [--agent id|--all]
lazyagents disable <skill> [--agent id|--all]
lazyagents install <source> [--hooks]
lazyagents remove <skill>
lazyagents adopt <skill> --agent <id>
lazyagents migrate-library <dir>
lazyagents sessions [--agent id] [--here] [--limit n] [--json]
lazyagents provider list|apply <profile>|clear|add <profile>|rm <profile> [--agent id] [--json] [--reveal]
lazyagents hooks list|enable <name>|disable <name>|add <name>|rm <name> [--agent id] [--json]
lazyagents usage [limits|daily|agents|projects|models] [--agent id] [--since 7d] [--limit n] [--refresh] [--json]
lazyagents <plugin> [args…]
lazyagents doctor [--json]
```

`usage` with no view is a dashboard: subscription limits with bars and reset time, the current 5h block and the period summary (daily sparkline, each agent's share, the projects that spent the most). The `daily`, `agents`, `projects` and `models` views are token tables (input, output, cache, responses); `limits` shows only the windows. `--since` takes `7d`, `24h` or a date like `2026-09-01`. USD cost only shows for agents authenticated with an API key, summed event by event at each model's rate, and is `—` when a rate is missing. In `usage --json`, match limit windows on `kind` (`session`, `weekly`, `weekly_model`); `label` is display text and may change (see the [CHANGELOG](CHANGELOG.md)).

Output is colored in a terminal and plain in a pipe, a file or with `NO_COLOR`; for scripts, every query command has `--json`.

`doctor` lists the detected agents, validates every `SKILL.md`, looks for broken symlinks and handshakes every plugin; it exits with code 1 when it finds a problem.

## Architecture

The project is organized in modules, **one package per module** in `internal/modules/<name>/`: the domain service, the TUI tab, the CLI commands and the registration live together, and adding one only takes a line in `internal/app/features.go`. `internal/cli` and `internal/tui` are frameworks and know no module; each module may have its own `config.yaml` section and, if it touches the CLIs, an optional capability on the `internal/agent` adapters. External plugins come in through the same contract, at runtime. The step by step and the rules are in [CLAUDE.md](CLAUDE.md).

Next steps are in [BACKLOG.md](BACKLOG.md). The review of the current version, its evidence and limits are in [docs/review.md](docs/review.md).

## Development

```sh
test -z "$(gofmt -l .)" && go vet ./... && go test -race ./... && go build ./...
go build -o /tmp/lazyagents-preview scripts/preview.go
bash -n scripts/*.sh
scripts/check-english.sh
go mod verify
go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./... # checks the vulnerability database
go run scripts/preview.go -theme sp-night-garoa -page 2   # TUI with fake data and an isolated config
scripts/record-demo.sh                          # re-records demo.gif (needs vhs, ttyd and ffmpeg)
```

The SP Night themes are generated from an SP Night checkout: `go run ./internal/tui/theme/generate -source <sp-night>` (details in [internal/tui/theme/README.md](internal/tui/theme/README.md)).

## Contributing

- English only: code, comments, UI text, docs and commit messages. Comments say why, briefly.
- Commits follow [Conventional Commits](https://www.conventionalcommits.org/): `feat(skills): install from zip`.
- Before opening a PR, run the check above (`gofmt`, `vet`, `test -race`, `build`) and `scripts/check-english.sh`; CI runs the same steps.

## License

[MIT](LICENSE). The themes come from [SP Night](https://sp-night.github.io/), also MIT ([notice](internal/tui/theme/LICENSE-SP-Night)).
