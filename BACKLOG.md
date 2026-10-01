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
- [x] Pin a matrix of CLI versions with versioned fixtures for private session and limit formats. **Acceptance:** version and origin of each fixture, without credentials/personal data. **Done:** `scripts/record-fixtures.go` records Claude Code 2.1.284, Codex 0.158.0, OpenCode 1.18.33 and Pi 0.87.1 against a built-in fake model in a throwaway home (sanitized, fails on anything naming the machine); Codex 0.156.1 limits from a real rollout shape with synthetic values. `TestRecordedFixtures` checks each adapter against a golden; the matrix and origins are in `internal/agent/testdata/fixtures/README.md`. It found three format drifts, fixed here: OpenCode 1.18 keeps message content in the `part` table (the transcript was empty), Codex 0.158 writes `rate_limits` with no window for API-key/custom providers (shown as zero windows instead of no data), and a harness message had to keep its leading `<` to be skipped. Not pinned: Gemini CLI and Hermes (not installed here). Claude Code 2.1.284 limits were pinned later with `-claude-limits` (a real call with the user's login, only the fields read, values synthetic).
- [x] Broaden adapter configuration: Hermes external directories and CLI overrides. **Acceptance:** read explicit configuration, without announcing directories the agent does not load. **Done:** `agent.ConfigOverrides` lists the variables each CLI reads to move its files, each checked against the CLI (Claude Code `CLAUDE_CONFIG_DIR`, Codex `CODEX_HOME`, OpenCode `XDG_CONFIG_HOME`/`XDG_DATA_HOME`/`OPENCODE_DB`, Hermes `HERMES_HOME`, Pi's two); relative values are ignored and the reference table is generated from the list. Hermes also resolves its active profile and reads `skills.create_dir` and `skills.external_dirs` from `config.yaml` as `agent/skill_utils.py` does (Hermes source at f42f579; not installed here), dropping missing dirs; `~/.agents/skills` listed there makes it a shared-dir reader. Hermes on Windows defaults to `%LOCALAPPDATA%\hermes` (done later, `Hermes.LocalAppData`). Not done: per-project skill dirs (gated by `hermes skills trust`).
- [x] Refine usage estimates: cost per event/model, 1h cache, fast/batch/region. **Acceptance:** keep "unavailable" when data is missing, without applying the last model's rate to the total. **Done:** prices from the pricing page (checked 2026-09-30, Fable/Mythos/Sonnet 5.5 added); `Usage.CacheWrite1h` and `Usage.Tier` come from Claude's `cache_creation`, `speed`, `service_tier` and `inference_geo` (`indexVersion` 5, buckets split by tier); `EstimateCost` applies 1h writes (2x), fast mode (Opus 5.5/5/4.8 only), batch (0.5x) and US inference (1.1x, 4.6+), and leaves priority and unknown regions unpriced. Session cost is summed per event (`agent.EventsCost`, `sessions.Service.SessionCost`) instead of the last model's rate over the total; one unpriced response makes the total unknown.
- [x] Resource limits: aggregate cap for decompressed files (done: zip installs, `installBudget`: 10,000 entries / 64 MB per entry / 512 MB in total; lazyagents' own backups keep only the 64 MB per entry, since they are written with no cap), deadline for plugin doctor (done: `Service.RunDoctor`, 30 s, no terminal, the tree is stopped on timeout) and shutdown of subprocess trees (done: `serve` plugins and their non-interactive `exec` run in their own process group on Unix, signalled and then reaped as a group; `taskkill /T` on Windows while the leader runs — on Windows a suspended start into a kill-on-close Job Object, terminated after the leader exits; `taskkill /T` still applies while it runs). **Acceptance:** controlled failure and no orphan subprocesses on the supported OSes.
- [x] Advanced TOML editing and concurrency between instances: define support for multiline strings (done: `tomlValueLines` finds multiline strings, arrays and inline tables so their lines are content, never a key, table or marker; an unterminated one is refused) and simultaneous changes (done: `fsutil.WriteAtomicIfUnchanged` replaces a live agent config only if it still holds what was read; the TOML editor redoes its edit up to 3 times, the JSON `settings` primitive fails with nothing written). **Acceptance:** never lose external state; the current editor refuses multiline strings and keeps a backup before writing.

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

## M18 — Crush (30/09/2026)

[Crush](https://github.com/charmbracelet/crush) (Charm) as the eighth agent: id `crush`, name `Crush`, short `R`. Everything below was checked against crush 0.96.1 in a throwaway home with the fake model, not taken from docs.

### M18.1 — Adapter and skills
- [x] Detected by the `crush` binary, `~/.config/crush` or `~/.local/share/crush`. Crush loads skills from `~/.config/crush/skills` (its own, the managed dir), `~/.config/agents/skills`, `~/.claude/skills` and `~/.agents/skills` (the shared dir), and follows symlinks; `~/.crush/skills` and the data dir are not read. `CRUSH_SKILLS_DIR` replaces every global skill dir (then it is the managed dir, alone); `CRUSH_GLOBAL_DATA` moves `projects.json`; `CRUSH_GLOBAL_CONFIG` moves only `crush.json` (not the skills), so it is not an override for lazyagents.
- **Acceptance:** enabling a skill links it where Crush loads it; a skill enabled for every reader of `~/.agents/skills` reaches Crush through it.

### M18.2 — Sessions
- [x] Sessions live per project: `~/.local/share/crush/projects.json` lists `{path, data_dir}`, each `data_dir/crush.db` is SQLite (`sessions`: id, `parent_session_id`, title, created/updated; `messages`: role and JSON parts `text` / `tool_call` / `tool_result`). Read with the `sqlite3` binary like OpenCode; sub-agent sessions (`parent_session_id` set) are hidden. Resume `crush --session <id>` run in the project; delete through `crush session delete` after a `crush session show --json` backup, both with `-D <data dir>`: crush 0.96.1 ignores `--cwd` in session commands (checked: `session list --cwd` returned nothing), and run elsewhere it would create a `.crush` there. `show --json` carries every message, but Crush has no import. Recorder (project kept in the home, `XDG_RUNTIME_DIR` isolated) and fixture `crush/0.96.1`.

### M18.3 — Session cost
- [x] Crush keeps only the last request's tokens per session (`prompt_tokens` stayed 150 after a second turn) but a cumulative `cost` from its own prices (0.26 → 0.458): the Sessions tab shows that cost as exact (no `~`, no token line); no token history for the Usage tab. A zero cost is an unpriced model, so it is not shown. `Crush.AuthMode` reads only whether a provider has a key: an `api_key` in the config or data-dir `crush.json` (onboarding saves keys there), or one of the provider key variables crush 0.96.1 reads (`CRUSH_GLOBAL_CONFIG` moves that file, so it joined the overrides).

### M18.4 — Providers and hooks
- [x] Written in `crushrc`, Crush's preferred format (`crush.json` is deprecated; both are merged, `crushrc` winning). The reference is the `crush-config` skill embedded in the crush 0.96.1 binary. lazyagents owns blocks delimited by `# lazyagents — managed block start: provider|hooks`, appended at the end of the global crushrc (later statements win, so clearing gives the user's own `model large` back with no saved state), values single-quoted (`shQuote`), env keys as `"$VAR"` with a validated name. Provider: `provider add lazyagents` + `model add` + `model large`; wireApi chat/anthropic; needs endpoint and model. Hooks: `PreToolUse` only, `hook add … --name lazyagents-<hash>`; `ReadHooks` also lists `crush.json` hooks. Same safety as the other editors: backup, conditional write with retry, an emptied crushrc lazyagents created is removed.
- **Acceptance:** with crush 0.96.1 and the fake model, a profile applied by lazyagents made `crush run` use the `lazyagents` provider; a `PreToolUse` hook installed by lazyagents fired on a bash call; uninstall and clear gave `crushrc` back byte for byte.

**Out of scope (decided):** `crush stats` (writes HTML and opens a browser), token history (not recorded).

## M19 — Transcript reader (01/10/2026)

The reader shows a coding session as a phone chat: one bordered bubble per agent turn (screens tall), progress narration as loud as the answer, subagents reduced to "Agent ×2", raw Markdown tables, a truncated header and empty side margins. Goal: read a session as a log — what was asked, what the agent did, what it answered — using what each agent records.

### M19.1 — Log layout, folded steps (render only)
- [x] No bubbles: a prompt is a section header (`#n You` + rule) with the text under a gutter in the user color; an agent turn is its name plus a gutter in the agent color. By default a turn shows the messages written after its last command (the answer, often several when a background task returns), everything before (narration, reasoning, commands) folded into one line `⋯ 2 messages · 3 commands: …`; `e` unfolds, and `t`/`r` imply unfolded. Markdown tables are drawn as tables in `kit.RenderChat`. The header keeps the counts (scroll position moves to the title line).
- **Acceptance:** tests for folding (final reply visible, steps summarized, a turn with no text reply), tables (fit the width, wrap cells) and the gutter; manual check via tmux on a long Claude Code session.

### M19.2 — Structured entries
- [x] `agent.Entry` grows `Time` and `Failed`; new `RoleEvent` (compaction, interruption, model change). Tool calls keep the `"Name · arg"` text: the separator already is the contract and M19.3 can split it. Prompt header shows the time (date when the day changes), agent turn its duration; a failed command is marked `✗`; events are centered lines between turns; export carries both. Checked against real sessions:
  - **Claude Code:** `timestamp` per line; `tool_result.is_error` matched to the call by `tool_use_id`; `isCompactSummary` (was shown as a prompt) → event; `[Request interrupted by user…]` → event; `<command-name>/x</command-name>…<command-args>` → the user's `/x args` (was dropped); `message.model` change → event (`<synthetic>` ignored).
  - **Codex:** `timestamp`; `compacted` and `event_msg/turn_aborted` → events; `turn_context.model` change → event. **No `✗`:** failures live in `item_completed` events whose ids do not link to the `call_id`.
  - **Pi:** `timestamp`; `toolResult.isError` by `toolCallId`; `compaction` and later `model_change` → events.
- [x] OpenCode: `part.time_created` (ms); `✗` from `state.metadata.exit` ≠ 0 (seen in the 1.18.33 recording) or `state.status == "error"`; `filePath` added to the argument keys (edit/read/write schemas checked in the opencode binary). Crush: `messages.created_at` (s); `✗` from `tool_result.is_error` by `tool_call_id`; `is_summary_message` → compaction event, with a fallback query for databases without the column. Times in UTC so goldens do not depend on the machine.
- [ ] **Open:** Gemini goes through the same JSONL reader, but no session was available to verify its `timestamp`.

### M19.3 — Tools by what they do
- [x] `Entry.Kind` (shell, edit, read, agent, plan, todo) from the tool name in `agent/toolkind.go`, plus `Added`/`Removed` for edits and `Body` for plans and task lists. Edit/MultiEdit/Write (`old_string`/`new_string`/`content`, shared context lines left out) and Codex patches, both as `apply_patch` input and inside `exec` code (`tools.apply_patch("*** Begin Patch…")`, checked in real sessions: Codex sends almost everything through `exec`) → `✎ files +a −d`; shell → `$ cmd`; reads, searches and web lookups dimmed; Agent/Task → `⎇ Agent  description`; TodoWrite/`update_plan`/`write_todos` → checklist with `n/m done`; ExitPlanMode → the plan as a document, shown even folded. The `⋯` line counts files edited and lines changed.
- **Moved to M19.5:** the subagent card's call count (needs the subagent's own transcript).

### M19.4 — Navigation
- [x] On ≥ 150 columns, a 36-column prompt rail (`n · time · first line · ⎇✎✗`) highlighting the prompt at the top of the screen. `]`/`[` pick a turn with steps (its gutter shows `▶`), `enter` unfolds or folds it alone, keeping it in place. Views on `m` (`v` already closes the reader): log · conversation (no steps line unless there is no answer) · actions (calls one per line, plans and task lists whole); the header shows a view other than log. Counting moved out of the renderers (`countTurn`), so every view reports the same totals.

### M19.5 — Structure
- [x] `Entry.Sub` (a `Session.Path` the same adapter reads) and `Entry.Calls`. **Claude Code:** `<session>/subagents/agent-<id>.jsonl` linked through `agent-<id>.meta.json` `toolUseId` (checked in real sessions); calls counted from the subagent file. **Pi:** at a fork, `branch k of n` plus one `other branch · <first line>` event per sibling, pointing at `<file>#<leaf>` (the sibling subtree's last entry); the active leaf is still the last one written. In the reader `]`/`[` pick subagent cards and branch lines too, `enter` opens them over a stack, `esc` pops back to the same place; a subagent's prompt is labeled `Task`.
- [ ] **Open:** OpenCode subtasks and Crush child sessions (`parent_session_id`): no local data to verify the link from the call to the child session.

## M20 — Public launch (01/10/2026)

The product is ahead of how it is presented: the repo has 0 stars, its GitHub description still lists four agents, and the README explains `SKILL.md` before showing why anyone would care. Goal: a visitor understands in ten seconds that lazyagents is **one TUI for every coding agent they use**, can install it in one line with their package manager, and the announcement lands in a short window (trending ranks measure speed, not totals). Positioning: **"The lazygit for AI coding agents."** Checked before planning: `install` works unchanged on this week's trending skill repos (`DietrichGebert/ponytail` 6 skills, `tt-a1i/archify` 2, `mattpocock/skills` ~38, `tigerless-labs/autoharness` 1 skill + 1 hook via `--hooks`), and "enable everywhere" already exists (`a` in the TUI, `enable --all`).

### M20.1 — Repository metadata
- [x] `gh repo edit`: description `The lazygit for AI coding agents: skills, sessions, usage, providers and hooks for Claude Code, Codex, Gemini CLI, OpenCode, Pi, Crush and Hermes Agent.`; topics `ai-coding`, `coding-agents`, `ai-coding-assistant`, `terminal-ui`, `crush`, `pi-agent` added (17 of GitHub's 20). Tagline confirmed by the maintainer.
- [ ] Social preview image (1280×640: logo + tagline + one TUI frame), source kept in `docs/assets/`. GitHub has no API for it: the upload is manual, in the repo settings. Generated: `docs/assets/social-preview.svg` (source, uses `logo.svg` and a hero.gif frame in `social-preview-tui.png`) rendered with `rsvg-convert` to `social-preview.png`; the upload is still open.
- **Acceptance:** `gh repo view --json description,repositoryTopics` shows the new values; a link pasted in a chat unfurls with the preview image.

### M20.2 — Install a subset and enable in one step
- [x] `lazyagents install <source> [skill...] [--all] [--hooks]`: names after the source pick skills from what `Discover` found (unknown name → error listing what exists, nothing installed); `--all` enables every installed skill in every installed agent with a skills dir, the same path as `enable --all`. Enables the launch line "install once, use everywhere" as one command and avoids pulling ~38 skills to try one of `mattpocock/skills`.
- **Touches:** `internal/modules/skills/cli.go`, `cli_test.go`, `docs/guide/skills.md` (CLI section), `go test ./docs -update`, `CHANGELOG.md`.
- **Acceptance:** tests for a subset, an unknown name, `--all` creating the links (temp home); a name that already exists in the library still fails alone, as today. Flags may follow the skill names (`positionals`); `--all` with no installed agent fails before cloning; a partial install now prints what it did install. Checked live in a throwaway home: `install DietrichGebert/ponytail ponytail --all` linked it into Claude Code and Codex.

### M20.3 — Hero GIF
- [x] `hero.tape` (10–12 s, same fake home as `demo.tape`): Skills matrix → `a` enables a skill everywhere (every column lights up) → Sessions with several agents → `enter` on a transcript → Usage limits. `scripts/record-demo.sh [tape]` takes the tape as an argument, default `demo.tape`. 1000 px wide so it reads on GitHub without zoom.
- **Acceptance:** ≤ 12 s, ≤ 1.5 MB, no real data (recorded only through `scripts/demo-home.sh`); `demo.gif` is kept for the long tour. Result: 12.4 s, 372 KB. The splash is turned off in the demo home's `config.yaml` (a key timed to skip it opened the SKILL.md reader instead), and `demo-home.sh` gained a Codex session so the list shows more than one agent; `demo.gif` was not re-recorded.

### M20.4 — README that sells before it documents
- [x] First screen: logo, `# lazyagents`, tagline **The lazygit for AI coding agents.**, one line `Claude Code · Codex · Gemini CLI · OpenCode · Pi · Crush · Hermes Agent`, `hero.gif`, install in one line. Then **Why** — a short before/after block (`~/.claude`, `~/.codex`, `~/.gemini`, `~/.config/opencode`… each with its own skills, sessions, config → one TUI). Then the current "What it does" (the `SKILL.md` explanation moves into the Skills bullet), **Try it with this week's skills** (the `install … --all` lines verified in M20.2), the long `demo.gif`, then Quick start, Documentation, safety, pre-1.0 note, Contributing, License as today. GitHub description, README tagline and `docs/README.md` intro say the same thing.
- **Acceptance:** tagline, agents, GIF and an install command visible without scrolling on a 1080p screen; `go test ./docs` green (README links); every command shown was run in a temp home. Done with "Try it with popular skills" (a dated "this week" would go stale): ponytail, archify and two of mattpocock/skills, all run with `--all` in a throwaway home. autoharness was left out: `--hooks` only puts its hook in the library, so it is not one command. The "Why" block lists only what holds for every agent shown (skills, sessions, config), since hooks are not supported for all of them.

### M20.5 — Release 0.4.3
- [x] CHANGELOG entry led by the positioning and M20.2; release notes written by hand on top of `--generate-notes`, with the hero GIF. Tag after M20.2–M20.4 are on `main`.
- **Acceptance:** release workflow green, five archives + `SHA256SUMS`, `lazyagents --version` = `0.4.3`. Done 01/10/2026: workflow green, the linux/amd64 archive matched `SHA256SUMS` and answered `lazyagents 0.4.3`; hand-written notes applied with `gh release edit`.

### M20.6 — Package managers

**Postponed (01/10/2026):** not now; needs the tap repo, its token and the maintainer's AUR key.

- [ ] **Homebrew:** tap repo `rogeriojunior31/homebrew-tap`; a step at the end of `release.yml` renders `Formula/lazyagents.rb` (darwin/linux × amd64/arm64 URLs and sha256 from `SHA256SUMS`) and pushes it with a fine-grained token secret limited to the tap repo. Still no third-party action (M6.2).
- [ ] **AUR:** `lazyagents-bin` (release archives, `sha256sums` from `SHA256SUMS`, installs LICENSE and theme notices); `PKGBUILD` kept in `packaging/aur/`, `scripts/aur-bump.sh <version>` updates `pkgver`/sums and `.SRCINFO`. Publishing is manual (the maintainer's AUR key).
- [ ] README and `docs/getting-started.md` install sections: `brew install rogeriojunior31/tap/lazyagents`, `yay -S lazyagents-bin`, `go install`, Releases.
- **Acceptance:** `brew install` on macOS (or Linuxbrew) and `makepkg -si` on Arch install a binary answering the released version.

### M20.7 — Launch window

**Postponed (01/10/2026):** dates not set yet.

- [ ] Prepared before day 1: one 20–30 s video (same tape, MP4), 3–4 screenshots per tab in `docs/assets/`, a text per community (not the same post): **Show HN:** "lazyagents – a lazygit-style TUI for managing AI coding agents"; **r/commandline:** the TUI and keys; **r/ClaudeAI / r/ChatGPTCoding:** the multi-agent mess and "install a skill once, use it in every agent"; **X / LinkedIn / dev.to:** the before/after + GIF.
- [ ] Days: **1** (Tue/Wed, ~14:00 UTC) Show HN + r/commandline, answer every comment the same day · **2** r/ClaudeAI · **3** r/ChatGPTCoding + X thread · **4** dev.to post "Managing skills across Claude Code, Codex and OpenCode" · **5** issues/PRs to curated lists (awesome-tuis, awesome-claude-code, awesome-agent-skills) · **6** follow-up fixes from feedback as a patch release · **7** LinkedIn + recap. Spread posts so each day adds stars instead of one spike.
- **Acceptance:** none measurable in code; record the stars per day and where they came from (GitHub traffic → referrers) here at the end of the week, to decide the next round.

**Deferred:** `lazyagents trending` (Trendshift has no public API; scraping is a permanent maintenance cost, and `S` already searches GitHub) — a curated list in the README covers it. `curl | sh` installer (at odds with the safety stance; checksummed releases and package managers suffice). Translated READMEs and `awesome-lazyagents` until there is traction.

## M21 — Layout across screen sizes (01/10/2026)

The same TUI looked very different on a laptop, a 1080p and a 2K screen and in the demo GIF: on wide screens the detail panel and the Usage bars grew into long blank stretches, at ~115 columns detail fields broke paths mid-word, and short terminals lost rows to chrome. Checked with tmux captures at 80×24, 115×33, 150×40, 210×55 and 280×75.

- [x] `kit.SplitDetail`: side detail at 2/5 of the width capped at `MaxDetailWidth` (88), the table takes the rest; the strip under a table grows to `MaxStripHeight` (12). `kit.Field` replaces the four per-module label/value helpers and `kit.Wrap` breaks after `/`. Root: under 30 rows (`compactHeight`) the header gap and the body's bottom padding go. Usage: `barMax` (40) for every bar; the top rows show when the table leaves `topsMin` (60) columns. Sessions: the project column grows with the table (18–32).
- **Acceptance:** the captures above show no blank-stretched bar, no mid-word path break and no column lost compared with before; gofmt, vet, `go test -race` and build green.

## Out of scope (decided)

- Automatic filesystem watch (`r` reloads)
- Cloud sync, system tray, auto-updater
- Local API proxy (providers only write agent config)
- Managing skills embedded in Claude Code plugins
