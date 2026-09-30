# Architecture

How the code is organized, and where a change goes. The short, rule-by-rule version for AI assistants is [CLAUDE.md](../CLAUDE.md); this page is for people.

## The big picture

lazyagents is one Go binary. Without arguments it opens a [Bubble Tea v2](https://github.com/charmbracelet/bubbletea) TUI; with a subcommand it runs headless. Both are built from the same **modules**: each module brings its domain logic, its tab, its CLI commands and its `doctor` checks in a single package.

```
main.go                 dispatch only: --version, CLI or TUI
internal/
├── app/                composition root: boot, migrations, THE registry (features.go)
├── feature/            the contract between a module and the root: Feature and Deps
├── core/               paths (XDG), config.yaml — bottom of the stack
├── fsutil/             atomic writes, backups, rotation
├── agent/              one adapter per agent + optional capability interfaces
├── cli/                headless framework: dispatch, help, flags, doctor
├── tui/                TUI framework: root, tabs, help, palette, widgets, themes
└── modules/            one package per module
    ├── skills/  sessions/  providers/  hooks/  usage/  agents/
    └── plugins/        external plugins: process, protocol, proxy tab
```

Dependencies only point one way:

```
fsutil ← core ← agent ← {cli, tui/*} ← feature ← modules/* ← app ← main
```

`cli` and `tui` are frameworks and know no module. A module imports them, never the other way around.

## Modules

A module is `internal/modules/<name>/`:

| File | Contents |
|---|---|
| `service.go` and one file per subject | The domain: `Service`, types and I/O. Never imports `tui/` |
| `tab.go`, `view.go`, `help.go`, `msgs.go` | The tab. Its model is `Tab`, built by `newTab` |
| `cli.go` | `commands(svc)` and `checks(svc)`, unexported |
| `feature.go` | `Feature() feature.Feature`, the only exported entry point |

A tab file about the same subject as a domain file gets the `_ui` suffix (`install.go` / `install_ui.go`).

The tab never does I/O itself: it calls the service inside a `tea.Cmd` and gets the result back as a message. Errors become toasts, never panics.

Tabs talk to each other only through messages in `internal/tui/events`, and those carry aggregates (`map[agent]int`), never a module's types.

### Adding a module

1. Create `internal/modules/<name>/` with the files above.
2. Register it with one line in `features()` in `internal/app/features.go`. The order there is the tab order; `Last: true` sends a read-only tab to the end.
3. If it needs settings, read its own `config.yaml` section with `d.Config.Section("<name>", &cfg)`. Never add a global key.
4. If it touches agent files, add a capability (next section).
5. Document it: `docs/guide/<name>.md`, linked from [docs/README.md](README.md). A test fails until the guide exists ([Documentation](#documentation)).

`app/app.go`, `tui/app.go` and `cli/cli.go` are never edited to add a module.

## Agents

`internal/agent` is the only package that knows where agents keep things and in which format. Every agent implements `Adapter` (detect, list sessions, resume, read a transcript, delete a session). Anything more is an optional interface, checked with a type assertion:

| Interface | Used by | What it adds |
|---|---|---|
| `ProviderHost` | providers | apply and clear an endpoint profile in the live config |
| `HooksHost` | hooks | read, add and remove hooks; the events the agent fires |
| `RateLimitReader` | usage | subscription limit windows |
| `UsageEventReader`, `UsageReader`, `AuthModeReader` | usage, sessions | tokens per response, per session, and whether the account pays per token |
| `LiveChecker` | sessions | whether a session is running now |
| `TranscriptProber` | sessions | a fast pre-check for full-text search |

`Adapter` itself does not grow: a new ability is a new interface, implemented only by the agents that support it.

### Adding an agent

1. Write the adapter in `internal/agent/<agent>.go`: `Detect` (binary in `PATH` or config dir; `--version` only here), sessions and transcripts. Confirm the formats against the agent's docs or source; never guess.
2. Add it to `AllWithIndex` in `internal/agent/registry.go`. The order there is the column order in the TUI.
3. Implement the capabilities it supports. A new transcript reader that extracts tokens or previews does it in its `lineScanner`, through the shared index (`internal/agent/index.go`), so each file is read once.
4. Give it a color in the themes (`theme.AgentColor`).
5. Tests use fixtures under a temporary home (`core.PathsIn(t.TempDir())`), never the real `~`. Pin the CLI's real formats too: add a recorder for it to `scripts/record-fixtures.go`, which runs the CLI in a throwaway home against a local fake model, then `go test ./internal/agent -run TestRecordedFixtures -update`. See [Recorded CLI fixtures](#recorded-cli-fixtures).
6. Run `go test ./docs -update`: the [agent support reference](reference/agents.md) picks the new agent up. Mention it in the [sessions guide](guide/sessions.md#where-sessions-come-from) if it has sessions.

### Recorded CLI fixtures

Agents change their private session formats between releases without notice. `internal/agent/testdata/fixtures/<agent>/<version>/` holds sessions written by the real CLIs, one dir per version, and `TestRecordedFixtures` runs each adapter over them against a `golden.json`. The [fixture matrix](../internal/agent/testdata/fixtures/README.md) lists every pinned version and where each fixture came from.

- `go run scripts/record-fixtures.go` records every CLI installed on the machine: each runs in a throwaway home against fakellm, a deterministic local model server inside the script (OpenAI Chat Completions, OpenAI Responses, Anthropic Messages), with no account and no network. Paths become `/work/proj` and `/home/user`, system prompts are cut, and a recording that still names the machine (user, hostname, kernel) fails.
- A new CLI release gets a new version dir. Keep the old ones: they are the matrix. Then `go test ./internal/agent -run TestRecordedFixtures -update` writes the goldens; review their diff, since that is where a format change shows.
- Codex limits cannot come from a fake model. `-codex-limits-from ~/.codex/sessions` copies the `rate_limits` shape of the newest real rollout, with every value that could describe the account replaced.
- Claude Code limits come from the network and have no fixture.

## Rules that protect user data

These hold everywhere; review checks them.

- **Config writes go through `fsutil.WriteAtomic`.** A live agent file gets `fsutil.Backup` first, and unknown keys are preserved.
- **JSON agent config** is edited through the `settings` primitive (`internal/agent/settings.go`), which touches only the target key and keeps the file order.
- **TOML (Codex) is never reserialized.** lazyagents edits only blocks delimited by `# lazyagents — …` comments and copies every other line as it is.
- **Skill activation is a symlink.** Deactivating removes the symlink; a real folder is never deleted.
- **Secrets** live in 0600 files, are masked in the TUI and in `--json`, and never reach a log, an error or an exported struct.
- **No network at boot.** Calls happen on demand only.
- **Consent is never forged.** When an agent asks the user to approve something (Codex's hook `trusted_hash`), lazyagents warns and leaves the approval to the agent.
- **Plugins are untrusted.** Everything they send is sanitized; a failing plugin becomes a dead tab.

The user-facing version of these rules is [Safety and data](safety.md).

## Themes

Colors exist only in `internal/tui/theme`. UI code uses tokens (`theme.Primary`, `theme.AgentColor(id)`) that resolve when rendering, so a style must never be rendered at init. Details: [internal/tui/theme/README.md](../internal/tui/theme/README.md) and [Themes](themes.md#contributing-a-theme).

## Documentation

The docs are built to survive change. Anything that is a list in the code is generated from the code; everything else is written by hand and checked by tests.

| Kind | Where | Source of truth | Kept current by |
|---|---|---|---|
| Reference | `docs/reference/` | the code: `cli.Command` Usage/Summary/Help, each tab's `Help()` and palette `Commands()`, the adapters, the theme list | `go test ./docs` fails when a page is stale; `go test ./docs -update` rewrites them |
| Guides | `docs/guide/<module>.md` | written by hand, one per module | a test fails if a registered module has no guide or the index does not link it |
| Topics | `docs/*.md` | written by hand | the link test |
| Plugin protocol | `docs/plugins.md` | written by hand | changes only with a protocol version bump |

Every relative link and `#anchor` in every Markdown file of the repository is checked by `go test ./docs`, so renaming a heading or a file breaks the build instead of the docs.

What to update when you change something:

| Change | Update |
|---|---|
| New or changed CLI command or flag | Its `Help` in `cli.go` (a test requires one), then `go test ./docs -update` |
| New or changed key or palette entry | The tab's `Help()`/`Commands()`, then `go test ./docs -update` |
| New agent, capability or hook event | The adapter, then `go test ./docs -update` |
| New theme | The YAML, then `go test ./docs -update` |
| New module | `docs/guide/<name>.md` and a line in `docs/README.md` |
| Behavior a user would notice | The guide of that module and `CHANGELOG.md` |
| New file on disk, network call or write to an agent file | [Configuration](configuration.md#where-files-live) and [Safety and data](safety.md) |

Guides describe tasks and concepts. They name a key or a flag where a task needs it and link to the reference for the full list, instead of copying tables that would go stale.

## Checks

```sh
test -z "$(gofmt -l .)" && go vet ./... && go test -race ./... && go build ./...
scripts/check-english.sh
```

CI runs the same steps; `go test ./...` includes the docs checks.
