# Changelog

## Unreleased

### Fixed

- macOS: the live badge on sessions now lights when the session path goes through a symlink (for example `/var` → `/private/var`).
- Windows: `doctor` no longer reports every hook script as not executable, a session started at a drive root no longer shows `\` as its project, and `~\` expands to the home dir like `~/`.
- CI runs the whole test suite on macOS and Windows, as a required check.

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
