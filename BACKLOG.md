# BACKLOG — lazyagents

Plan in execution order. **One task at a time**: do not start the next one while the current one is failing.

## How to work (every task)

1. Follow `CLAUDE.md`, especially "How to add a new module" and the inviolable rules.
2. Check: `test -z "$(gofmt -l .)" && go vet ./... && go test -race ./... && go build ./...` all green.
3. Manual test of the TUI via tmux when the task touches UI.
4. Tick the checkbox here and commit in English, Conventional Commits: `feat(scope): description`.

Legend: **touches** = expected files/packages · **acceptance** = verifiable criteria · **verify** = confirm the format in the agent's docs/code before coding, never guess.

> **Layout:** since the 22/09/2026 reorganization, each module is ONE package in `internal/modules/<name>/` (domain + tab + CLI + `Feature()`), registered by one line in `app.features()`. Paths cited in finished tasks are the ones from before the change; new modules follow the new layout, described in CLAUDE.md.

---

## M0 — Foundation: config.yaml, per-module sections and external plugins

Before the new modules: extensible config without editing `core`, and a path for tabs coming from outside the binary.

### M0.1 — config.yaml with comment round-trip + migration
- [x] `core.Config` read/written via `yaml.Node` (comments and foreign keys survive), `Config.Section(id, &out)` for each module/plugin's top-level section, `core.MigrateConfig` (config.json → config.yaml + `.migrated`) called in `app.LoadWith`; boot notices in `Deps.Notices`.
- **Touches:** `internal/core/{config,paths}.go`, `internal/app/app.go`, `main.go`, `internal/skill/ops.go` (fix: `migrate-library` erased `theme`).
- **Acceptance:** round-trip keeps comments; migration is idempotent and lossless; `TestMigrateLibrary_KeepsTheme`; zero new dependencies.

### M0.2 — `internal/plugin` service and protocol v1
- [x] Discovery in `<ConfigDir>/plugins/*` (executable, name `^[a-z0-9][a-z0-9_-]{0,31}$`), `Service.Start` (spawns `<bin> serve`, `init`, manifest within 3 s), `Proc.Send/Events/Close`, `Run` (pass-through), `CleanView` (only SGR passes). JSON Lines protocol documented in `docs/plugins.md`.
- **Touches:** `internal/plugin/{proto,plugin}.go`, `internal/core/paths.go` (`PluginsDir`), `docs/plugins.md`, `examples/plugins/hello`.
- **Acceptance:** `#!/bin/sh` fixtures (skipped without `sh`): manifest ok, timeout, invalid line, line > 1 MiB, crash, `Run` with exit code and env; table-driven `CleanView`; `List` filters.

### M0.3 — Proxy tab + dynamic registration + CLI/doctor
- [x] `internal/tui/modules/plugin` implements `module.Module` + `Commander` over a `plugin.Proc` (dead state on error, `:reload` respawns, interactive `exec` via `tea.ExecProcess`); `app.Deps` appends plugins to `Modules()`/`Commands()`/checks; `cli.PluginCommands`/`PluginChecks`; `Context.In`; `main` closes the processes on exit.
- **Touches:** `internal/tui/modules/plugin/*`, `internal/app/{app,features}.go`, `internal/cli/{cli,plugin}.go`, `main.go`.
- **Acceptance:** `tui/app.go` untouched; fixture tests in `app`, `cli` and the module; tmux with `examples/plugins/hello` (tab, keys, `:reload`, `q` without orphans).

### M0.4 — Docs and rules
- [x] CLAUDE.md (config.yaml, `Section`, `internal/plugin` in the graph, plugin rule 9), README (YAML config, plugins), `internal/tui/theme/README.md`.

---

## M1 — Pending items in skills, sessions and TUI

### M1.1 — Marketplaces in the official format
- [x] Read `.claude-plugin/marketplace.json` from git repos as an extra source of skills.
- **Done:** `Discover` (TUI `i`, search `S`, `lazyagents install`) prefers the marketplace when it declares skills: plugins sourced from the repo itself (`./x`, plain name + `metadata.pluginRoot`, `skills` list) become entries with the plugin name in the picker; `Rel` relative to the root keeps update working. External sources (`github`, `url`, `git-subdir`, `npm`, `archive`, `command`) are neither fetched nor run: they become a warning in the picker/stderr naming the repo to install. A marketplace without skills falls back to the generic scan.
- **Touches:** `internal/skill/marketplace.go`, `internal/skill/install.go`, `tui/modules/skills/{picker,model}.go`, `cli/skills.go`.
- **Acceptance:** fixture repo with marketplace.json lists and installs one entry; invalid JSON = friendly error; tests with `t.TempDir()`.

### M1.2 — Session alias
- [x] Our own alias, without touching the CLI's file.
- **Touches:** `internal/session/alias.go` (`<DataDir>/session-aliases.json` via `fsutil.WriteAtomic`, unknown fields survive), `tui/modules/sessions` (key `m`; `FilterValue` includes the alias).
- **Acceptance:** alias survives a restart; the filter finds it; empty input removes it; round-trip tested.

### M1.3 — Help modal truncates the right column
- [x] At 120 columns the second column of `?` cut descriptions with `…`.
- **Touches:** `internal/tui/app.go` (`renderHelp`/`helpColumns`: maximum panel width computed from the content, not fixed at 74).
- **Acceptance:** no truncated description at 100+ columns; a single column below ~64; tmux.

---

## M2 — Usage module (subscription-aware)

First new module because it is **read-only**: it exercises every seam (agent capability, service, tab, `Commander`, CLI, doctor) with no risk to live files. Never at startup: loaded on demand, cached.

**On a subscription the limit is neither tokens nor cost — it is a window percentage** (5h session and weekly, with a reset time). Tokens and USD cost only make sense for an API-key account. Formats verified on this machine:

- **Codex**: the JSONL rollout already has everything, offline. Event `event_msg` with `payload.type == "token_count"` → `payload.rate_limits.{primary,secondary}.{used_percent, window_minutes, resets_at}` (unix), `plan_type`, `credits`. Per-response consumption comes from the `token_usage_record` event (`payload.usage`, where `input_tokens` **already includes** `cached_input_tokens`); older versions use `payload.info.last_token_usage`.
- **Claude Code**: writes no limit to disk. Same source as `/usage` and the Omarchy bar tools: `GET https://api.anthropic.com/api/oauth/usage?at_wall=1&skip_spend=1` with `Authorization: Bearer <accessToken from ~/.claude/.credentials.json>` and `anthropic-beta: oauth-2025-04-20`. Response: `limits[]` with `{kind: session|weekly_all|weekly_scoped, percent, resets_at, severity, scope.model.display_name}`, legacy `five_hour/seven_day.{utilization,resets_at}` and `seven_day_breakdown.rows[]`. Per-session token usage still comes from the JSONL (`message.usage` + `timestamp` + `cwd` per line).

Module rules: network only on demand (never at boot), cached response, token used only in the header and never shown, logged or persisted.

### M2.1 — Agent capabilities
- [x] `internal/agent/usage.go`: `UsageEvent{Time, Model, CWD, Usage}` + `UsageEventReader`; `AuthMode` (`Unknown|Subscription|APIKey`) + `AuthModeReader`. `internal/agent/ratelimit.go`: `RateWindow{Kind, Label, UsedPercent, ResetsAt, Severity}`, `RateStatus{Plan, Windows, FetchedAt, Source}` + `RateLimitReader{ RateLimits(ctx) (RateStatus, error) }`.
- **Details:** decode structs with **only the non-secret fields**; the `accessToken` is only materialized inside Claude's HTTP call. Codex resolves offline from the rollouts.
- **Acceptance:** rollout fixtures (with and without `secondary`, old and new format) and a test HTTP server for Claude; a test ensuring no token shows up in an exported struct, log or error.

### M2.2 — `internal/usage` service
- [x] `Status(agentID)` (limit windows, cached in `<DataDir>/usage-cache.json` with a TTL); `Blocks(agentID)` (ccusage-style 5h windows from the `UsageEvent`s), `Daily(n)`, `ByProject()`; USD cost only with `AuthAPIKey` (reuses `agent.EstimateCost`).
- **Acceptance:** table-driven blocks (5h edges, gaps, current block containing now); the cache respects the TTL and survives a restart; aggregation by day/project.

### M2.3 — Usage tab + CLI
- [x] `tui/modules/usage`: per agent, 5h session and weekly bars with % and reset, plan and auth mode badge; tokens per day/project as detail; USD cost only for an API-key account. `lazyagents usage [--json] [--agent id] [--refresh]`.
- **Acceptance:** registered only via `app.Features`; no network at boot; tmux; stable `--json`.

---

## M3 — Providers module (cc-switch style)

First module that **writes** to live agent config. Introduces the shared editing primitive.

### M3.1 — Live config editing primitive
- [x] `internal/agent/settings.go`: `readSettings` keeps the top-level keys as `json.RawMessage` **in file order** (decoder token stream; a map would lose the order of a hand-edited file), `get`/`set` (nil removes) touch only the target key and `save(backupsDir)` does `fsutil.Backup` + `RotateBackups` + `WriteAtomic` keeping the permission (a new file is created 0600, the default for anything that may hold a token).
- **Acceptance:** round-trip keeps unknown fields and permissions; backup created before every write; test with a 0600 file.

### M3.2 — Capability and service
- [x] `agent.ProviderHost{ ProviderFile(); ReadProvider() (ProviderProfile, bool, error); ApplyProvider(p, backupsDir) error; ClearProvider(backupsDir) error }`; `internal/provider` with named profiles in `<ConfigDir>/providers.json` (0600), `Status()` per agent and `Apply/Clear` (empty agent = every installed agent that supports it). `ProviderProfile.Redacted()` (idempotent) is the only form that goes out to TUI/JSON/log.
- **Per agent:** Claude Code = `env` in `~/.claude/settings.json` (`ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_MODEL`), via the M3.1 primitive. Codex = top-level `model_provider`/`model` + `[model_providers.lazyagents]` in `config.toml`; **TOML strategy decided: delimited blocks, no new dependency** (the file carries `[projects.*]` and `[hooks.state.*]` with a trust hash, which no library reserializes without rewriting). Codex does not accept a token in the file: only `env_key`, and applying a profile with a token but no `envKey` is an error. OpenCode and Gemini stay out until there is an `opencode.json`/`settings.json` to verify — never guess a format.
- **Done:** Codex reading also works for a hand-configured provider (a mini reader for `key = "string"`, never used for writing).
- **Acceptance:** apply/clear per agent with backup; the token never shows in logs, `View()` or `--json` without `--reveal`.

### M3.3 — Providers tab + CLI
- [x] `providers` tab (profile × agent matrix, `1-9` applies/removes on agent N, `space` on all, `x` clears, `d` deletes the profile), every write behind a confirm showing `from → to` and the file. CLI `provider list|apply|clear|add|rm [--agent id] [--json] [--reveal]`; `add --token -` reads the token from standard input. Doctor: "providers" section with what is applied per agent, a problem when the agent is not installed.
- **Done:** validated against the real Codex (`codex doctor` recognized `[model_providers.lazyagents]` and asked for the `env_key` variable); apply→apply→clear returns `config.toml` and `settings.json` to their original content.
- **Left out:** creating/editing a profile in the TUI (the CLI creates; the TUI applies). Add it when it needs more than a 5-field form.

---

## M4 — Hooks module

Formats already seen on this machine:
- Claude Code: `~/.claude/settings.json` → `hooks.<Event>[].{matcher, hooks[].{type, command, timeout}}`.
- Codex: `~/.codex/hooks.json` with the same per-event shape, **plus** `[features] hooks` and `[hooks.state."<file>:<event>:<i>:<j>"] trusted_hash` in `config.toml` — a new hook may require a trust confirmation in Codex itself (**verify** before writing).

### M4.1 — `HooksHost` capability
- [x] `internal/agent/hooks.go`: `Hook{Event, Matcher, Command, Timeout}` (identity = the event+matcher+command triple) and `HooksHost{ HookEvents(); HooksFile(); ReadHooks(); AddHook(h, backupsDir); RemoveHook(h, backupsDir); HooksNote() }`. **AddHook/RemoveHook instead of WriteHooks(set):** a surgical write never rewrites a foreign group, so an unknown field inside it survives.
- **Verified on this machine:** both CLIs use the same format (`"hooks": {Event: [{matcher?, hooks:[{type,command,timeout}]}]}`), Claude Code inside `settings.json` and Codex in `hooks.json`. Event names in CamelCase; Codex normalizes to snake_case (the machine's real `trusted_hash` points to `hooks.json:session_start:0:0`), so event comparison ignores case and separator.
- **Codex, deliberate limit:** lazyagents **does not write** `[features] hooks` nor `[hooks.state] trusted_hash` in `config.toml`. The hash is the user's confirmation that the command may run; forging it would approve execution on their behalf. `HooksNote()` says what is missing (feature off, or confirmation pending in Codex itself).
- **Out of scope:** Gemini and OpenCode — no config file on this machine to verify the format.
- **Acceptance:** tests cover preserving a foreign hook, an unknown field in the group, a non-command entry type, idempotent add, cleanup of empty event/key, removal inside a shared group and not writing config.toml.

### M4.2 — Module service (`internal/modules/hooks/service.go`)
- [x] Library in `<DataDir>/hooks/<name>.json` (one JSON per hook, hand-editable); `Library()` returns the valid hooks and the problems per file, without one broken file taking down the listing. `Status()` per agent separates what belongs to lazyagents from the user's own (`Foreign`, never touched). `Enable/Disable` with a named agent or every installed one; installing a hook for an event the agent does not fire is an **error**, not silence. `CommandProblem` finds a command outside PATH or without permission.
- **Acceptance:** idempotent enable/disable; a foreign hook survives; name/event/command validations covered.

### M4.3 — Hooks tab + CLI
- [x] `hooks` tab (hook × agent matrix, `1-9` installs/removes on agent N, `space` on all, `x` removes from all, `d` deletes from the library), with `–` on the agent that does not fire the event and every write behind a confirm showing `event → command` and the file. CLI `hooks list|enable|disable|add|rm [--agent id] [--json]`. Doctor: command outside PATH or without permission, invalid library file and each agent's note.
- **Done:** validated in an isolated HOME — both files come out in the exact shape of the real ones (Claude Code's `settings.json` and Codex's `hooks.json`), with `matcher` only when the hook has one, and Codex's `config.toml` left untouched.
- **Left out:** creating/editing a hook in the TUI (the CLI creates, the TUI installs) and Gemini/OpenCode, with no verifiable format on this machine.

---

## M5 — Hooks from a repository

A skills repo almost always ships hooks too (in the official Claude Code marketplace, `<plugin>/hooks/hooks.json`, by convention — neither `marketplace.json` nor `plugin.json` declares them). Installing only the skills delivered half the package.

### M5.1 — Discovery and import
- [x] `hooks.DiscoverIn(root, rootName)` finds `<plugin>/hooks/hooks.json` in the already materialized source (skips hidden dirs, depth 5) and `hooks.Import(paths, found, source)` copies into `<DataDir>/hooks/<plugin>/` **the top-level folders the commands reference**, keeping the plugin root layout, and writes the entry with the commands rewritten.
- **The crux was `${CLAUDE_PLUGIN_ROOT}`:** a variable only Claude Code expands, and only for plugins it installed. It now points to the copy **and is exported in the command** — real scripts locate themselves through it or through a path relative to the root, so the layout must be the same. A referenced folder missing from the source makes the import fail right away, instead of leaving a hook that would break silently on the event.
- **Adjusted after the first real repo (`dog_stack`):** copying only `hooks/` rejected the whole repo, because the commands also reference `scripts/`; the package name came out as the clone's temp directory when `hooks/` is at the root; and `async: true` was lost in translation, turning an async hook into a blocking one. Copied: 404 KB (`hooks/` + `scripts/`) out of a 6 MB repo.
- **A library entry became a package** (`Hook.Hooks []agent.Hook`): the real `security-guidance` has 12 commands in 5 events and, one per entry, flooded the library and the matrix. A command repeated inside the package goes in once (the plugin repeats the same triple under different matchers).
- **Acceptance:** fixture in the real layout + the two actual plugins from the official marketplace; import is all or nothing; `Delete` takes the scripts along.

### M5.2 — Unified picker and reach
- [x] The same `i` in the Skills tab and `lazyagents install <source>` list the source's skills and hooks. A hook comes **unchecked** and the CLI requires `--hooks`: installing a hook means running a third-party command on every event.
- [x] An imported hook can be installed on any agent that fires the event; `Enable` installs the supported subset (Codex has no `Stop`/`SubagentStop`), the matrix shows `◐` for partial and the entry says "imported from X · made for Claude Code" — the payload each CLI sends on stdin was not verified.
- **Acceptance:** tmux with the two real plugins; the user's own hook survives and is still counted separately.

## M6 — Release and CI

### M6.1 — CI
- [x] `.github/workflows/ci.yml`: the same sequence as CLAUDE.md (gofmt, vet, test, build) on push and PR, plus building `scripts/preview.go` (it has the `ignore` tag, is not part of `./...` and would break unnoticed) and a job that compiles the five published targets.

### M6.2 — Release by tag
- [x] `.github/workflows/release.yml`: a `v*` tag runs vet and test, builds with `-trimpath -ldflags "-s -w -X main.version=<tag>"` for linux/{amd64,arm64}, darwin/{amd64,arm64} and windows/amd64, packages (`.tar.gz`, `.zip` on Windows), generates `SHA256SUMS` and publishes with `gh release create --generate-notes`.
- **No GoReleaser and no third-party action:** `go build` in a loop and the `gh` already on the runner — fewer dependencies to audit in a tool that edits the user's agent config.
- **Acceptance:** the release job was simulated locally before the tag — the five artifacts come out, `SHA256SUMS` checks out and the binary answers `lazyagents 0.1.0` (version injection works).

## M7 — Refinement of the current version (22/09/2026)

Report, evidence and limits: [docs/dev/review.md](docs/dev/review.md). The history above
records the original implementation; later fixes take precedence over older descriptions.

- [x] Integrity: safe install, restore/update with staging, migration with preflight and config preservation.
- [x] Hooks/providers: path protection, collisions and quoting, preserving fields and the previous provider, token permissions.
- [x] Robustness: JSON null, large ZIPs, backup ordering, export, metadata and plugin shutdown.
- [x] Compatibility: Hermes, Codex provider protocol, per-version prices, aggregation by path/model.
- [x] Maintenance: docs, preview, demo script, CI with race, release/licenses and removal of leftover GoReleaser.
- [x] Run native tests on macOS and Windows; validate symlinks, shells, permissions and paths. **Acceptance:** a matrix with real results per OS, not just cross-compilation. **Done:** `.github/workflows/native.yml` runs the suite on both and blocks like CI. Fixed on the way: macOS live badge vs `/private/var`, Windows exec-bit false positives in hooks, Windows root as a project, `~\` in paths. Plugin tests run a Go-built fake plugin (`internal/modules/plugins/testdata/fakeplugin`) on every OS instead of shell scripts. Still open: no manual TUI pass on macOS or Windows yet.
- [x] Pin a matrix of CLI versions with versioned fixtures for private session and limit formats. **Acceptance:** version and origin of each fixture, without credentials/personal data. **Done:** `scripts/record-fixtures.go` records Claude Code 2.1.284, Codex 0.158.0, OpenCode 1.18.33 and Pi 0.87.1 against a built-in fake model in a throwaway home (sanitized, fails on anything naming the machine); Codex 0.156.1 limits from a real rollout shape with synthetic values. `TestRecordedFixtures` checks each adapter against a golden; the matrix and origins are in `internal/agent/testdata/fixtures/README.md`. It found three format drifts, fixed here: OpenCode 1.18 keeps message content in the `part` table (the transcript was empty), Codex 0.158 writes `rate_limits` with no window for API-key/custom providers (shown as zero windows instead of no data), and a harness message had to keep its leading `<` to be skipped. Not pinned: Gemini CLI and Hermes (not installed here) and Claude Code limits (network only).
- [x] Broaden adapter configuration: Hermes external directories and CLI overrides. **Acceptance:** read explicit configuration, without announcing directories the agent does not load. **Done:** `agent.ConfigOverrides` lists the variables each CLI reads to move its files, each checked against the CLI (Claude Code `CLAUDE_CONFIG_DIR`, Codex `CODEX_HOME`, OpenCode `XDG_CONFIG_HOME`/`XDG_DATA_HOME`/`OPENCODE_DB`, Hermes `HERMES_HOME`, Pi's two); relative values are ignored and the reference table is generated from the list. Hermes also resolves its active profile and reads `skills.create_dir` and `skills.external_dirs` from `config.yaml` as `agent/skill_utils.py` does (Hermes source at f42f579; not installed here), dropping missing dirs; `~/.agents/skills` listed there makes it a shared-dir reader. Not done: Hermes on Windows (`%LOCALAPPDATA%\hermes`) and per-project skill dirs.
- [x] Refine usage estimates: cost per event/model, 1h cache, fast/batch/region. **Acceptance:** keep "unavailable" when data is missing, without applying the last model's rate to the total. **Done:** prices from the pricing page (checked 2026-09-30, Fable/Mythos/Sonnet 5.5 added); `Usage.CacheWrite1h` and `Usage.Tier` come from Claude's `cache_creation`, `speed`, `service_tier` and `inference_geo` (`indexVersion` 5, buckets split by tier); `EstimateCost` applies 1h writes (2x), fast mode (Opus 5.5/5/4.8 only), batch (0.5x) and US inference (1.1x, 4.6+), and leaves priority and unknown regions unpriced. Session cost is summed per event (`agent.EventsCost`, `sessions.Service.SessionCost`) instead of the last model's rate over the total; one unpriced response makes the total unknown.
- [ ] Resource limits: aggregate cap for decompressed files (done: zip install and tar.gz restore share `extractBudget`, 10,000 entries / 64 MB per entry / 512 MB in total), deadline for plugin doctor and shutdown of subprocess trees (done: `serve` plugins and their non-interactive `exec` run in their own process group on Unix, signalled and then reaped as a group; `taskkill /T` on Windows while the leader runs — children of an exited leader on Windows would need a Job Object, not done). **Acceptance:** controlled failure and no orphan subprocesses on the supported OSes.
- [ ] Advanced TOML editing and concurrency between instances: define support for multiline strings and simultaneous changes. **Acceptance:** never lose external state; the current editor refuses multiline strings and keeps a backup before writing.

## M8 — Refined CLI (23/09/2026)

### M8.1 — Full `usage`
- [x] `usage` without a view becomes a dashboard: limits with a bar and "resets at", current 5h block, period sparkline, share per agent and top projects.
- [x] Views `limits`, `daily`, `agents`, `projects`, `models` with `--agent`, `--since 7d|24h|YYYY-MM-DD`, `--limit`, `--refresh` and stable `--json` (`rows` + `total`).
- [x] Cost summed event by event (each model's rate), only for an API-key agent; a single event without a price makes the aggregate "—".
- **Acceptance:** same numbers as the Usage tab; no ANSI outside a terminal; an invalid `--agent` lists the valid ids.

### M8.2 — Framework and polish
- [x] `Command.Summary`/`Help`, `lazyagents help <command>`, `cli.Flags` with help text (in PT-BR at the time; English since M15) and `Context.KnownAgent` (usage, sessions, hooks, provider).
- [x] `doctor --json`; `sessions --agent/--here/--limit`; `skills <sub>` grouping the skill commands; truncated description in `list`.

## M9 — Configurable TUI layout (23/09/2026)

- [x] `tui:` section in `config.yaml`: `splash`, `splashSeconds`, `startTab`, `tabs` (order; unlisted ones follow the default order) and `hidden`.
- [x] A hidden built-in tab stays alive in the background (Sessions feeds Usage and Agents); a hidden plugin does not spawn a process, but keeps its command.
- **Acceptance:** an invalid value or unknown id becomes a warning on exit, never an error; the zero value of `tui.Options` reproduces the previous behavior; tmux with a temporary config.

## M10 — Query engine for large histories (23/09/2026)

Measured with 2000 sessions (860 MB of transcripts) and 1000 skills, on tmpfs.

- [x] Incremental transcript index (`transcript-index.gob`): each JSONL is read once and then only the appended part; a rewritten file is read again. One pass extracts preview, tokens, usage (15-min slots) and Codex limits, with one worker per CPU. `sessions --json` 2.3 s → 0.11 s; `usage` 3.3 s → 0.15 s.
- [x] Live session: `/proc` on Linux (`lsof` cost a fixed ~130 ms), only for sessions modified in the last 24h; delete checks the exact file at that moment.
- [x] Full-text search with a pre-filter on the raw bytes, in parallel: 8 s → 0.36 s.
- **Acceptance:** `--json` outputs identical to the previous version (bulk and real data), including after appending lines and with an incomplete last line.

## M11 — Filterable Usage tab (23/09/2026)

- [x] Filters for period (today/7/30/90 days/all), agent, view (day/agent/project/model) and text, in memory over the whole history; table shared with the CLI, with columns that shrink with the width.
- [x] Filters in the palette (`usage period …`, `usage view …`, `usage clear`) and defaults in the `usage:` section of `config.yaml`.
- [x] Memoized body: scrolling costs 0.1 ms even with 460 thousand usage slots; changing a filter, up to ~130 ms in that worst case.
- **Acceptance:** tmux with real data at 130, 60 and 40 columns; `TestResponsiveLayout` green.

## M12 — Incremental TUI review (24/09/2026)

- [x] Help, confirms, palette, fields and pickers usable on small screens, with essential shortcuts kept.
- [x] Tab navigation and scrolling of the Agents/Providers details; Sessions deletion with pinned targets and cancel as the default.
- [x] Usage with pinned filters, full errors and progress waiting for both queries; scrollable plugin diagnostics with an accessible restart.
- [x] Hooks with compact selection, full-screen reading of commands/scripts and editing in the editor with confirmation, backup and failure handling.
- **Validation:** tests, local checks equivalent to CI, tmux runs and VHS captures. Scope, limits and pending visual items detailed in [docs/dev/tui-design-review.md](docs/dev/tui-design-review.md).

## M13 — Tabs redesigned around their goal

- [x] Phase 0 — base: `kit.TableRow`/`TableHeader` (1 line, cell colors kept on selection), `AgentColumns`, `TableDelegate`, `SplitDetail` and `Frame` (footer on the last line).
- [x] Phase 1 — Skills as a skill × agent matrix.
- [x] Phase 2 — Sessions as a 1-line table; preview without actually resuming.
- [x] Phase 3 — Agents as a diagnostics panel.
- [x] Phase 4 — Providers with "in use" at the top and a profiles table.
- [x] Phase 5 — Hooks with the library as a matrix.
- [x] Phase 6 — Usage: limits → period → view, pinned footer.
- [x] Phase 7 — cleanup (`PlainDelegate`/`ListRow`), plugins in the `Frame`, captures in the three themes.

## M14 — Pluggable themes

- [x] Full SP Night: the three flavors embed the 23 colors and every role (`ui`, `syntax`, `diagnostic`, `git`, `ansi`) from upstream; new tokens (Info, Hint, Link, Match, Added/Removed, syntax, `ANSI`) used in markdown, the profile diff and agent colors.
- [x] Community themes with the official palette: Tokyo Night (Night/Storm), Dracula, Gruvbox Dark, Nord, Rosé Pine (Main/Moon/Dawn), Kanagawa Wave, Everforest Dark, One Dark.
- [x] User themes in `<ConfigDir>/themes/<id>.yaml` with `extends`, `palette` and partial roles; an invalid file becomes a warning.
- [x] Contrast: text roles ≥ 3:1 over background and selection in every built-in theme; theme license notices in the release package.
- **Acceptance:** `generate -check` green; every built-in theme resolves every role; `TestLoadUser` covers inheritance, cycles, reserved ids and invalid hex; `TestBuiltinContrast` green.

## M15 — English as the project language

The project is open source: code, comments, UI, CLI, docs and commits move to English. PT-BR comes back later as a translation (message catalog), not as the language of the code.

- [x] Phase 0 — rules and guard rail: CLAUDE.md in English with the language rule; `scripts/check-english.sh` (accented Portuguese in tracked files, with allowlist and a `check-english:allow` line marker) running in CI as report only; this milestone.
- [x] Phase 1 — compatibility: Codex `config.toml` managed-block markers read in PT and EN, written in EN; rate-limit window labels in English with a versioned usage cache that drops the old PT one; `label` documented as display text and the change recorded in `CHANGELOG.md`; plugin protocol and `config.yaml` values checked (already English).
- [x] Phase 2 — TUI and CLI text, one commit per module (framework, skills, sessions, hooks, providers, usage, agents/plugins, cli/app), tests updated in the same commit. Dates in ISO order (`2006-01-02`, `Tue 09-23`), relative times `3min ago`/`yesterday`, `?` hint is `help`.
- [x] Phase 3 — error messages and domain (`agent`, `fsutil`, `core`, services): Go-style English errors, `AuthMode` `subscription`/`unknown`, agent details and notes, `(no prompt)`/`(untitled)`; `agent.DetailNotInstalled` replaces the text comparison in the Agents tab.
- [x] Phase 4 — themes: READMEs, community theme descriptions, SP Night labels/descriptions via the generator. SP Night ids renamed to `sp-night`, `sp-night-garoa`, `sp-night-jaragua` (default `sp-night`); the old `noite`/`garoa`/`jaragua` still work in `theme:` and `extends:` and stay reserved.
- [x] Phase 5 — comments and tests in English and short: restatements removed, the why kept in one or two lines (comment lines 2258 → 1884); test names, messages and fixtures in English, except fixtures that reproduce legacy data or test non-ASCII input (`check-english:allow`). No Go file is flagged by `check-english.sh`.
- [x] Phase 6 — README rewritten for GitHub (keys and layout checked against the code), docs, BACKLOG, CI, scripts and the example plugin in English; `demo.tape` follows the real tab order and runs with a restricted `PATH` (no host paths or CLI versions in the GIF); `demo.gif` re-recorded; migration plan deleted (its record lives here); `check-english.sh` required in CI.
- **Acceptance:** `scripts/check-english.sh` green and required; a Codex config with the old PT markers is read and migrated (test); every tab, help, palette, confirm and toast in English (tmux); `help`, `--help` and `doctor` in English.

**Conventions kept from the migration:**
- **Glossary:** session · usage · provider / profile · agent · library · enable / disable · adopt · alias · tab · command palette · window (`session`, `weekly`, `weekly · <model>`) · managed block · warning / error / hint · resume · diagnostics (`doctor`) · source · update from source. The `?` hint reads `help`; "folder" in the TUI, "directory" in CLI flags; relative times `just now`, `3min ago`, `yesterday`, `2d ago`; dates in ISO order (`2006-01-02`, `Tue 09-23` where width is fixed).
- **Style:** short sentence-case text, no trailing period in hints and toasts; Go-style errors (`reading %s: %w`). Each user-facing sentence is whole in one string (`fmt.Sprintf("%d skills enabled in %s", n, agent)`), never assembled from fragments.
- **Guard rail:** `scripts/check-english.sh` flags accented Portuguese; a line ending in `check-english:allow` is exempt (legacy data still parsed, non-ASCII test input). Proper names Rosé, Jaraguá and São Paulo are allowed.
- **Compatibility kept:** Codex managed-block markers are read in PT and EN (`codexLegacy*`); the usage cache is versioned; old SP Night ids are aliases.

**Next: PT-BR translation** (not started):
- Message catalog keyed by the English string (gettext style), `pt-BR` embedded; a missing entry falls back to English.
- Language from `language:` in `config.yaml` (a new global key — needs a decision, today only `theme` and `libraryDir` are global), falling back to `LANG`/`LC_ALL`.
- `--json` output is never translated (it is a contract); only human text is.

## M16 — Documentation for a public release (28/09/2026)

Docs that survive change: lists that live in the code are generated from it, the rest is written by hand and checked by tests.

- [x] `docs/reference/` (CLI, keys and palette, agent support, themes) generated from the registry by `go test ./docs -update`; `go test ./docs` fails on a stale page, a command without `Help`, a module without a guide, and a broken link or anchor in any Markdown file.
- [x] `Help` on every CLI command, also shown by `lazyagents help <command>`.
- [x] Guides per module, getting started, configuration, themes, safety and data, troubleshooting, architecture; README reduced to a landing page; internal reviews moved to `docs/dev/`.
- [x] `CONTRIBUTING.md`, `SECURITY.md`, issue and PR templates, CHANGELOG for 0.1.0 and 0.2.0.
- [x] Code/doc mismatches found while writing, all fixed with regression tests: CLI `enable`/`disable` without `--agent` targets agents that are not installed; OpenCode session delete has no backup and the delete confirm names `backups/sessions`; sessions show `~$` cost for subscription accounts; Claude provider apply drops an existing `ANTHROPIC_AUTH_TOKEN`/`ANTHROPIC_MODEL` the profile does not set; `hooks list` columns misaligned; untranslated strings `verificando updates…`, `recarregando…` (skills) and `reparando hooks` (hooks); no key deletes a skill profile.

## M17 — Pi coding agent (28/09/2026)

[Pi](https://github.com/earendil-works/pi) (`@earendil-works/pi-coding-agent`, formerly `badlogic/pi-mono`) as the seventh agent: id `pi`, name `Pi`, short `P`. Everything lives under `~/.pi/agent` (`PI_CODING_AGENT_DIR` overrides it); formats verified against the docs shipped with pi 0.87.1 (`<install>/docs/*.md`).

### M17.1 — Adapter and skills
- [x] `Pi{Home, Look}` adapter: detected by the `pi` binary or `~/.pi/agent` (`pi` is a generic name — a binary alone needs `pi --version` to print a bare semver); `ManagedDir` `~/.pi/agent/skills`, `ReadDirs` also `~/.agents/skills` (pi reads it natively, follows symlinks); session methods refuse, like Hermes.
- **Touches:** `internal/agent/pi.go`, `registry.go`, `agent_test.go` (`TestRegistry` 6 → 7, empty-home test), `docs/docs_test.go` (`loadApp` creates `~/.pi/agent`), `theme.AgentColor` (optional case), `skills/cli.go` help, README, CLAUDE.md, `docs/guide/{agents,skills}.md`, `docs/troubleshooting.md`, `docs/reference/` (regenerated), CHANGELOG.
- **Acceptance:** enabling a skill symlinks it into `~/.pi/agent/skills` and pi lists it as `/skill:<name>`; `PI_CODING_AGENT_DIR` is honored.

### M17.2 — Sessions through the index
- [x] `ListSessions` over `sessions/--<cwd>--/<ts>_<uuid>.jsonl` (also `sessionDir` in `settings.json` and `PI_CODING_AGENT_SESSION_DIR`) via `refreshAll` + `retain` + `save`; `piIndexLine` takes `cwd` from the `session` header, title from `session_info.name`, preview from the first `user` message, model from `model_change` / assistant messages. `Transcript` follows the active branch only (last leaf → root by `parentId`). `ResumeCmd` = `pi --session <file>` in the session cwd. `DeleteSession` backs up and removes the `.jsonl`.
- **Verify:** record a real short session as the fixture (the doc samples are not enough); bump `indexVersion` if `indexEntry` changes. Done: `internal/agent/testdata/pi-session.jsonl` was recorded by pi 0.87.1 against a local fake OpenAI-compatible server (a named session, two turns, bash tool calls); `indexEntry` unchanged. A relative `sessionDir` resolves per project in pi, so only an absolute one is followed.
- **Acceptance:** sessions tab lists, previews, searches, resumes and deletes pi sessions; a branched session shows only the active branch.

### M17.3 — Usage and cost
- [x] `UsageReader`, `UsageEventReader`, `AuthModeReader`: sum `usage` from assistant messages, `usage` entries, `compaction` and `branch_summary`; `reasoning` is already inside `output` (never add it twice); cost comes from pi's own `usage.cost.total`, not `pricing.go`. Auth mode reads only the `type` (`oauth` / `api_key`) of the default provider in `auth.json`, never a value.
- **Acceptance:** usage tab shows pi tokens and cost per period; sessions show cost; no rate limits (pi exposes none). Done: `Usage.Cost` and the index bucket carry the agent-recorded USD (`indexVersion` 3) and `EstimateCost` prefers it; each call is priced by its own provider (the index keeps provider with model; OAuth providers are `Covered`, cost 0); the fixture totals match pi's own footer (input 340, output 28, cache read 200).

### M17.4 — Providers
- [x] `ProviderHost`: `ProviderFile` = `~/.pi/agent/models.json`; apply writes `providers.lazyagents` (`baseUrl`, `apiKey`, `api`, `models`) there and `defaultProvider`/`defaultModel` in `settings.json`, both through the `settings` primitive with `fsutil.Backup` first; `models.json` kept 0600; clear removes only our keys. `auth.json` (pi's own `/login`) is never touched.
- **Acceptance:** applying a profile makes `pi` start on it; clearing restores the previous default; foreign providers and keys survive (test). Done: checked with pi 0.87.1 against a local fake server (`pi -p` with no `--provider` used the profile). Previous defaults live in `pi-provider-state.json` (lazyagents data dir); the profile needs `baseUrl` and `model`; `wireApi` maps to pi's `api` (chat/responses/anthropic); `envKey` becomes `"apiKey": "$VAR"`.

### M17.5 — Per-agent skill activation with shared dirs
- [x] Enabling a skill for one agent never shows it to another. `Agent.SharedDir` (`~/.agents/skills`: Codex, Gemini CLI, OpenCode, Pi) is separate from `ManagedDir`, now always the agent's own dir (Codex → `~/.codex/skills`, verified in Codex 0.157.1 with a symlinked skill). One placement rule (`skills.place`) behind enable, disable, enable/disable all and profiles: the shared dir gets a single link only while every installed reader (≥ 2) has the skill; otherwise each agent gets its own link, and disabling one reader splits the shared link into the others' dirs. New links are created before old ones are removed. `migrate-library` also relinks the shared dir.
- **Known limit:** OpenCode also reads `~/.claude/skills` (Claude's own dir), so a skill enabled for Claude Code still shows in OpenCode (`◆`).
- **Acceptance:** `TestSharedDirOnlyWhenEveryReaderHasIt` walks enable one → all readers (consolidates) → disable one (splits) → enable all → disable all; legacy shared links split on disable; one reader never uses the shared dir; someone else's entry in the shared dir is left alone and the skill goes to own dirs; disabling Claude keeps the skill in OpenCode (which sees it through `~/.claude/skills`); `--agent` on an agent not installed still links it.

**Out of scope (decided):** hooks — pi has no shell hooks, only TypeScript extensions (`pi.on("tool_call", …)`); a `HooksHost` would have to generate a `.ts` into `extensions/`. Also out: rate limits, prompt templates, project-local `.pi/` resources (gated by pi's project trust).

## Out of scope (decided)

- Automatic filesystem watch (`r` reloads)
- Cloud sync, system tray, auto-updater
- Local API proxy (providers only write agent config)
- Managing skills embedded in Claude Code plugins
