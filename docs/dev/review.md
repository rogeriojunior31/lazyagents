# Review of the current version — 2026-09-22

Base: `04c771d` (clean tree at the start). Cross-cutting review of code, tests,
contracts, documentation, examples, scripts and distribution. The fixes were
committed to `main`; publishing a release was out of scope for this round.

## Findings fixed

Severity: high = possible unintended change or data loss; medium = wrong result,
unavailability or broken integration; low = documentation, diagnostics or
maintenance. Each row names the original reproduction, the fix and the
executable evidence.

| ID | Severity | Location and original evidence | Fix and validation |
| --- | --- | --- | --- |
| R01 | High | `skills.Install`: `name: ../../escaped` from the frontmatter reached the destination unvalidated | Unsafe components are rejected before copying; `TestInstallRejectsEscapingName` |
| R02 | High | `skills.Restore`: an invalid backup was opened after the current state had been removed | Extraction into staging, gzip validation, backup and swap with rollback; `TestRestoreCorruptBackupKeepsExistingSkill` |
| R03 | High | `skills.replaceDir`: removed content before confirming the remote copy | The new state is prepared in staging and the previous one kept until the swap; `TestUpdateMissingSourcePreservesContents` |
| R04 | High | `skills.MigrateLibrary`: ignored link conflicts/errors and discarded an invalid config | Preflight, refuses overlap, keeps the sources until the config is saved, rolls back copies/links; `TestMigrateLibrary_*` and `TestMigrationRejectsConflictAndInvalidConfig` |
| R05 | High | `hooks.Delete`: `Files` in the JSON could point outside the library | Script deletion limited to direct children of the library; loaded names must match the file; `TestDeleteRefusesExternalScripts` |
| R06 | High | Hook import could overwrite an existing entry and interpolated paths into shell code | Validation before copying; refuses collisions and exports the variable instead of replacing text inside quotes; `TestImportDoesNotOverwriteLibraryEntry`, `TestRewriteCommandPreservesShellQuoting` |
| R07 | High | `scripts/demo-home.sh`: `rm -rf` on an arbitrary argument | Refuses any existing destination; the demo uses a unique temp folder. Smoke test kept a sentinel file |
| R08 | High | Codex provider duplicated `model_provider`; an unclosed marker swallowed the rest of the config | Keeps/restores the previous provider, validates delimiters and refuses multi-line TOML the editor does not parse; regressions in `provider_test.go` |
| R09 | Medium | Claude provider dropped unknown non-string values in `env`; a new token could keep mode 0644 | Unknown values preserved and writes with a token restricted to 0600; `TestClaudeProviderKeepsUnknownEnvValues`, `TestClaudeTokenTightensPermissions` |
| R10 | Medium | Cache, aliases and profiles accepted JSON `null` and then wrote into a nil map | Maps initialized; `TestNullCacheAndFutureTimestamp`, `TestNullAliases`, `TestNullProfiles` |
| R11 | Medium | ZIP entries over 64 MiB were truncated without error; backups were sorted by skill name | Explicit limit with an error and sorting by date; `TestZipRejectsOversizedEntry`, `TestBackupsSortedAcrossSkills` |
| R12 | Medium | Export used an external ID as a path and the transcript was written 0644 | Refuses separators/traversal, nanosecond names and mode 0600; `TestExportRefusesTraversal` and the export tests |
| R13 | Medium | Plugin metadata/errors could carry terminal controls; exec did not follow app shutdown | Sanitization, cancellation context and plugin environment; `TestManifestStripsTerminalControls`, `TestExecStopsWhenServiceCloses` |
| R14 | Medium | The Opus 4 price prefix applied one version's rate to another; aggregates mixed models and same-named projects | Explicit per-version rates, no estimate for mixed/unknown models, projects grouped by path; `TestVersionSpecificPricing`, `TestAggregationSeparatesPathsAndMixedModels` |
| R15 | Medium | Hermes was marked as an automatic reader of `~/.agents/skills`; help offered `wire_api=chat` to Codex | Hermes announces only its default dir; Codex refuses the removed protocol and help uses `responses`; `TestCodexRejectsUnsupportedWireAPI` |
| R16 | Low | Preview dropped the command that loads the sample skills | Init runs only the skills scan, keeping the sample sessions; tmux showed 5 skills and 4 sessions |
| R17 | Low | Leftover GoReleaser, diverging architecture/support docs, packages without license notices | The shell workflow is the only definition; docs aligned; licenses in the packages; CI/release run race and script syntax checks |
| R18 | Medium | CI, `TestStartFailures/exit`: the plugin exited before reading `init` and `EPIPE` hid exit code 3 | The writer waits for the process to be reaped so the exit error survives; the case passed 100 times with `-race` and the full suite passed |
| R19 | Low | `demo.tape` did not show Providers, Hooks and Usage; the setup had invalid VHS syntax and lacked the `bin` folder | Sample fixture covers the tabs, tape updated and `demo.gif` re-recorded |
| R20 | Low | Actions warned that `checkout@v4` and `setup-go@v5` use the deprecated Node.js 20 | CI and release use the Node.js 24 versions (`checkout@v5`, `setup-go@v6`) |

An empty import block was also removed. No new module dependencies, no plugin
protocol version change and no migration of persisted formats. Previously
imported hooks keep their stored command: the quoting fix applies to new
imports. To replace them, disable/remove/reimport the entry.

## Coverage

| Area | What was examined |
| --- | --- |
| Boot, core, fsutil, composition | Paths, YAML config/migration, atomic writes, backups, registration and shutdown |
| Skills | Local/git/ZIP/marketplace discovery, install, enable, adopt, profiles, hash/update, removal, backup/restore and migration |
| Sessions and adapters | Listing, parsing, resume, transcript, aliases, export, deletion with backup and protection of live sessions |
| Providers and hooks | Libraries, JSON/TOML editing, foreign keys, tokens, consent, import and scripts |
| Usage | Aggregation, estimate, cache, on-demand auth/limits; real authenticated network calls were not exercised |
| Plugins | Discovery, protocol, manifest, process, failures, output, CLI/doctor and the shell example |
| TUI | Routing, events, help, palette, empty states and layout; layout tests from 40 to 120 columns and preview in tmux |
| Distribution/documentation | README, CLAUDE, backlog, plugins, themes/licenses, examples, demo, scripts, CI/release and dependencies |

Coverage combines targeted reading of the flows, cross-cutting searches,
existing tests, new regressions and smoke tests. It is not a formal security
certification nor a run of every combination of agents and operating systems.

## Compatibility references

Checked on 2026-09-22. Official pages move; local tests use fixtures and do not
establish compatibility with every future CLI version.

- [Claude Code — hooks](https://code.claude.com/docs/en/hooks): per-event configuration and commands; keep the CLI's trust review.
- [Codex — configuration](https://learn.chatgpt.com/docs/config-file/config-reference): `model_providers`, `env_key` and `wire_api=responses`.
- [Codex — skills](https://learn.chatgpt.com/docs/build-skills): user skills in `.agents/skills` and symlink support.
- [Claude plugins in Codex](https://developers.openai.com/plugins/guides/submit-claude-plugin): hooks need adaptation and trust; sharing an event name does not prove payload equivalence.
- [Gemini — skills](https://geminicli.com/docs/cli/using-agent-skills/): personal dirs `.gemini/skills` and `.agents/skills`.
- [OpenCode — skills](https://opencode.ai/docs/skills/): skill discovery and permissions.
- [Hermes — skills](https://hermes-agent.nousresearch.com/docs/user-guide/features/skills/): external dirs need explicit configuration.
- [Claude API — pricing](https://platform.claude.com/docs/en/about-claude/pricing): standard per-version rates; the local estimate uses 5-minute cache writes.

## Checks and limits

Environment: Linux amd64, Go 1.27.1. The module still declares Go 1.26 and CI
uses that version; this round does not change toolchain or dependencies.

- Baseline: `go test ./...` passed. The first attempt in the sandbox failed on a read-only cache and a blocked test HTTP server; the authorized rerun outside it passed.
- `gofmt -l .` empty, `git diff --check` clean; tests, race detector, `go vet`, project build and the separate preview build passed.
- Tmux: Garoa in 120×40, palette in 64×24, Noite in 80×24 and Jaraguá in 100×40 (the SP Night flavors, since renamed `sp-night-garoa`, `sp-night`, `sp-night-jaragua`); skills, sessions, help, agents and the empty hooks state checked.
- `go mod verify`: modules intact. `govulncheck` v1.8.0: no vulnerabilities found.
- CLI with a sample home/XDG: version, help and JSON of skills, sessions, providers and hooks valid.
- Builds for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 and windows/amd64; the exact build/packaging block of the release workflow ran in a temp copy, without publishing, and every SHA256SUMS matched. Only the Linux amd64 binary was run.
- Local documentation links and script syntax checked.
- No real agent data was changed; no credentials were used for real limit calls.

Non-blocking follow-ups are in M7 of the backlog: native tests on the other
platforms, a CLI version matrix, advanced configuration variants, aggregate
file and process limits, and more precise estimates.
