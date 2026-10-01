# Changelog

## Unreleased

### Changed

- Wide terminals: the Usage tab puts limits and the period summary on the left and the table on the right (from 140 columns), and its limit and share bars grow with the room; the Agents tab shows the detail beside the table instead of below it.

### Fixed

- Usage: limit bars of a window without a reset time (Codex) were longer than the others; they align now.
- Providers: the `profile` column header was cut (`profi…`) when every profile name was short.

## 0.4.1 — 2026-10-01

### Changed

- `1`–`9` now jump to tab N, as in lazygit, in every tab. They no longer toggle a skill, install a hook or apply a provider in agent N: that wrote to an agent's config on a key that is easy to press by habit. Pick the agent with `←/→` and press `space`. A plugin tab gets digits only while it is capturing input.

### Fixed

- Agents tab and `--json` output: an agent's version is just the number (`0.159.2`, not `codex-cli 0.159.2`; `0.97.1`, not `crush version v0.97.1`).
- Skills: the `▸` in the detail pane, which marks the agent `space` toggles, now moves with `←/→`; it stayed on the previous agent until the selection changed.

## 0.4.0 — 2026-09-30

### Added

- Crush support: detection (binary, `~/.config/crush` or `~/.local/share/crush`) and skills in `~/.config/crush/skills`, or in `CRUSH_SKILLS_DIR`; Crush also reads `~/.agents/skills`, `~/.config/agents/skills` and `~/.claude/skills`.
- Crush sessions in the Sessions tab and `lazyagents sessions`: every project Crush knows, the transcript, resume with `crush --session`, and delete through `crush session delete` after a JSON backup.
- Crush session cost in the Sessions tab: the cost Crush summed from its own prices, shown as exact for an API-key setup.
- Crush providers and hooks: `provider apply --agent crush` and hook installs write delimited blocks at the end of Crush's `crushrc` (values single-quoted, the rest of the file untouched); clearing removes them.
- Pi coding agent support: detection (`~/.pi/agent` or `PI_CODING_AGENT_DIR`) and skills in `~/.pi/agent/skills`. Pi also reads `~/.agents/skills`, so skills enabled for Codex show up in Pi too.
- Pi sessions in the Sessions tab and `lazyagents sessions`: list, search, read the transcript (active branch), resume with `pi --session` and delete with a backup.
- Pi token usage and cost in the Usage tab, `lazyagents usage` and the session detail. The cost is the one Pi records for each response; responses from a provider signed in with OAuth (a subscription) cost $0.
- Agents whose files were moved with their own variable are found there: `CLAUDE_CONFIG_DIR`, `CODEX_HOME`, `XDG_CONFIG_HOME`/`XDG_DATA_HOME`/`OPENCODE_DB` (OpenCode), `HERMES_HOME`, `PI_CODING_AGENT_DIR`/`PI_CODING_AGENT_SESSION_DIR`.
- Hermes Agent: the skill dirs its `config.yaml` adds (`skills.external_dirs`, `skills.create_dir`), the profile picked with `hermes profile use`, and its Windows home `%LOCALAPPDATA%\hermes`.
- Provider profiles for Pi: `provider apply --agent pi` writes a `lazyagents` provider to `~/.pi/agent/models.json` and makes it Pi's default; `provider clear` gives the previous default back.

### Changed

- Cost estimates cover Claude Fable 5 and 5.1, Mythos 5 and 5.1 and Sonnet 5.5 (prices checked 2026-09-30).
- The transcript index format changed twice (1-hour cache writes and pricing tiers, Codex limits): the first launch after upgrading reads every transcript once more, then only what was appended.
- Skills: enabling a skill for one agent no longer shows it to the other agents that read `~/.agents/skills`. Codex skills now go to `~/.codex/skills`; a skill is linked in `~/.agents/skills` only while it is enabled for every installed agent that reads it (Codex, Gemini CLI, OpenCode, Pi), and disabling it for one of them moves it to the own directory of the others. Links already in `~/.agents/skills` keep working.
- Usage limits and `doctor` leave out agents with nothing to report (not installed, not signed in, API key account, never used) instead of showing a message or a failure for them.

### Fixed

- Codex limits: a window whose reset time passed since the last Codex session showed its old use with "resets now"; it now shows 0%, since that use is from before the reset.
- A change the agent (or another lazyagents) made to its config while lazyagents was writing it is no longer overwritten: Codex's `config.toml` is edited again on the new content, and a JSON config is left as it is with an error.
- Codex providers: a `config.toml` with a multiline string (`developer_instructions = """…"""`) is edited instead of refused, and an array spread over several lines can no longer receive the lazyagents block in its middle; both are kept as written.
- `lazyagents doctor` no longer hangs on a plugin whose `doctor` never ends: after 30 s the check fails and the plugin is stopped with its child processes.
- Plugins: a process a plugin started (in `serve` mode or through a background `exec`) is stopped with the plugin instead of being left running.
- Installing a skill from a zip stops at 512 MB uncompressed or 10,000 entries in total; before, only each entry was limited (64 MB), so a small archive could fill the disk.
- Cost estimates: Claude Code's API-error and interrupt lines (model `<synthetic>`, no tokens) no longer make a session or usage row show no cost; a response with no tokens costs $0.
- Cost estimates: a session that switched models was priced entirely at the last model's rate; it is now summed per response. 1-hour cache writes, fast mode, Batch API and US-only inference are priced as the pricing page defines them (they were priced as standard 5-minute usage), and Priority Tier or another region shows no cost instead of a wrong one.
- OpenCode: the session transcript was empty with current OpenCode (1.18), which keeps message content in its `part` table.
- Codex: an API-key or custom-provider account no longer shows empty limits (0 windows) in the Usage tab and `doctor`; Codex 0.158 records a limits block with no window for them.

## 0.3.2 — 2026-09-28

### Fixed

- `lazyagents --version` reports the real version when installed with `go install …@latest` instead of `dev`.

## 0.3.1 — 2026-09-28

### Fixed

- macOS: the live badge on sessions now lights when the session path goes through a symlink (for example `/var` → `/private/var`).
- Windows: `doctor` no longer reports every hook script as not executable, a session started at a drive root no longer shows `\` as its project, and `~\` expands to the home dir like `~/`.
- CI runs the whole test suite on macOS and Windows, as a required check. Plugin tests use a fake plugin built in Go instead of shell scripts, so they run on Windows too.

## 0.3.0 — 2026-09-28

### Added

- Documentation in `docs/`: getting started, one guide per module, configuration, themes, safety and data, troubleshooting and architecture. The CLI, key, agent and theme references are generated from the code and checked by `go test ./docs`.
- `lazyagents help <command>` explains every command, with its options.
- `CONTRIBUTING.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md` and issue and pull request templates.
- Skills: `d` in the profile list deletes a profile.
- CI runs the test suite on macOS and Windows too (informational for now).
- Full SP Night palette, 11 community themes and your own themes in `~/.config/lazyagents/themes/`.
- Tabs redesigned around a matrix (skill × agent, profile × agent, hook × agent) with the detail beside or below the table.
- Transcript reader by turns, with reasoning, tool calls and navigation between your prompts.
- Providers: create and edit a profile from the tab. Hooks: read and edit a hook's commands and scripts, and toggle the commands of an imported pack.

### Fixed

- `enable` and `disable` without `--agent` now act on the same agents as the TUI `a`/`x` keys: installed agents with a skills directory. They no longer create skills directories for agents that are not installed, nor fail on agents without one. `--agent` together with `--all` is a usage error.
- Disabling a skill in every agent no longer fails when several agents see it through the same shared directory.
- Deleting an OpenCode session now saves `opencode export` to the backups dir first, and nothing is deleted if the export fails.
- The session delete confirmation names the backups folder the files actually go to.
- Providers: applying a Claude Code profile no longer removes an `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_MODEL` or `ANTHROPIC_BASE_URL` you set yourself. lazyagents only removes the keys it wrote, and clearing puts back an endpoint or model a profile had replaced.
- Providers: the `env` block of Claude Code's `settings.json` keeps its key order instead of being rewritten sorted by name.
- `hooks list`: the agent table is aligned on its own instead of inheriting the hook table's column widths.
- Leftover Portuguese text in the Skills tab and in hook repair errors is now English.
- Skills: confirming a skill removal no longer runs the action of the previous confirmation (for example updating another skill or deleting a profile).
- `enable` with no installed agent that has a skills directory fails with an explanation instead of reporting success.
- Providers: `clear` without a lazyagents record (nothing applied yet, or applied by v0.2) no longer removes Claude Code `ANTHROPIC_*` keys you set yourself. The record is saved before `settings.json`, so a failed write never leaves values nobody recorded.
- Providers: `clear` removes a Claude Code `settings.json` that lazyagents itself created for the provider, instead of leaving `{}` behind. A file that also holds hooks or your own keys stays.
- `doctor` no longer reports a problem for an agent you have not signed in to or used yet; the Usage tab shows it as not available yet instead of "Refresh failed". `usage --json` marks it with `no_data`.

### Changed

- `provider list --json` uses snake_case like every other command: `base_url`, `has_token`, `env_key`, `wire_api`, and `applied` only when a provider is applied. `providers.json` on disk keeps its format.
- Sessions shows an estimated cost, and `sessions --json` includes `cost_usd`, only for agents authenticated with an API key, the same rule as Usage.
- English is now the project language: the TUI, CLI text, error messages, themes and docs are in English.
- SP Night themes are renamed: `sp-night` (default), `sp-night-garoa` and `sp-night-jaragua`. The old ids `noite`, `garoa` and `jaragua` still work in `theme:` and in a user theme's `extends:`. Plugins now receive `sp-night`… as `theme.id` in `init`.
- Dates are shown in ISO order (`2026-09-25`, `Tue 09-23`) and relative times in English (`3min ago`, `yesterday`).
- `usage --json`: rate-limit window `label` values are now in English (`session 5h`, `week`, `week · <model>`). `label` is display text and may change again; scripts should match on `kind` (`session`, `weekly`, `weekly_model`). Other human-readable text in `--json` output is English too: `auth` (`subscription`, `unknown`), placeholder session titles (`(no prompt)`, `(untitled)`), agent details and error messages.
- Codex `config.toml`: the comments that delimit the lazyagents managed blocks are now written in English. Blocks written by earlier versions are still recognized and are rewritten on the next provider apply or clear.
- The usage limits cache (`usage-cache.json`) has a new format; a cache from an earlier version is discarded once and fetched again.

## 0.2.0 — 2026-09-23

### Added

- Usage tab filterable by period, agent, view and text.
- `lazyagents usage` dashboard and views, per-command help and `doctor --json`.
- Configurable TUI layout: splash, tab order, hidden tabs and start tab (`tui:` in `config.yaml`).
- Incremental transcript index: each transcript is read once, then only what was appended.

### Fixed

- Data protection fixes across skills, hooks, providers and sessions (details in [docs/dev/review.md](docs/dev/review.md)).

## 0.1.0 — 2026-09-22

First release.

- Skills: central library, activation by symlink, install from GitHub, folders, zips and Claude Code marketplaces, GitHub search, adoption, profiles, updates, backups and `SKILL.md` lint.
- Sessions for Claude Code, Codex, Gemini CLI and OpenCode: resume, transcript, Markdown export, full-text search, aliases, grouping, deletion with backup.
- Usage: subscription limits, tokens and estimated cost.
- Providers: endpoint profiles applied to Claude Code and Codex.
- Hooks: a hook library installed per agent, and hooks imported with skills.
- External plugins over JSON Lines, `config.yaml` with per-module sections, headless CLI and `doctor`.
