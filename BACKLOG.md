# lazyagents roadmap

Updated 2026-10-17. The released baseline is **0.2.2**. This is the single
roadmap for lazyagents: delivered work, product decisions, implementation
steps, references and release gates are kept together here. The next work
completes the management lifecycle: inspect actual state → validate → preview
→ apply → verify each target → recover.

CC Switch is the implementation reference for guided management. Its source
links below are pinned to 797e0276ffc20e54cbdf55e3de84505d77231c91, reviewed
against lazyagents at 1387fb5. ECC is an optional skill source used to exercise
large-library workflows. Verify native CLI formats again when implementing
an adapter; a reference implementation is not a compatibility guarantee.

## Current work

**Next task: [M23.2](#m232-restore-skills-independently-of-the-current-list).**
M23.1 is implemented and verified. The remaining M23–M30 tasks are planned.

Work on one task at a time, in the order below. Finish its checks before
starting another. A task with missing external evidence stays open; record
the missing input and explicitly reschedule it before advancing. Never
turn a partial implementation or an unsupported format into a completed item.

Status: unchecked = planned; checked = implemented and verified. Version
targets below are release batches, not dates or automatic publication.
Future command names, storage formats and capabilities require implementation
review; only the current reference documents describe commands available today.

## Release policy

- 0.2.3, 0.2.4 and later 0.2.x versions are patch releases that consolidate
  the current product. A merged PR does not require a release.
- New minor versions, such as 0.3.0, require a completed product milestone
  and the acceptance criteria below. A layout change or another adapter
  alone does not advance the minor version.
- Preserve current CLI behavior, machine-readable fields, local data and
  foreign agent settings. Additive features can remain on 0.2.x. A breaking
  change needs a documented migration and an explicit version decision.
- Release targets may be split or combined before publication. There is
  no limit at 0.2.9: 0.2.10 is valid. Never move a published tag or repeat
  the historical version consolidation.
- 0.3.0 and 0.4.0 are planned scope boundaries, not evidence of readiness.
  1.0 has a separate stability gate.

## Delivered baseline

The baseline below consolidates the completed work. The [changelog](CHANGELOG.md)
and Git history retain the detailed release record. Known gaps are assigned
in the implementation steps below.

| Release | Delivered outcome |
|---|---|
| 0.1.0 | Central skill library and symlinks; sessions, usage, providers, hooks; external plugins; YAML config; CLI, doctor and release builds |
| 0.2.0 | Usage filters/dashboard, configurable tabs and splash, incremental transcript index and initial protection fixes |
| 0.2.1 | TUI redesign, accessible secondary screens, theme system, English UI, generated references and native platform tests |
| 0.2.2 | Pi/Crush, shared skill placement, stronger config/process protection, structured transcript reader, responsive layouts, selective install, public EN/PT-BR docs, demos and version consolidation |

Current coverage: skill directories for seven coding agents; sessions for
Claude Code, Codex, Gemini CLI, OpenCode, Pi and Crush; providers for Claude
Code, Codex, Pi and Crush; hooks for Claude Code, Codex and Crush. Claude
Desktop is detected but has no managed capability. Usage history is available
for Claude Code, Codex and Pi, and subscription limits for Claude Code/Codex.
The generated [agent reference](docs/reference/agents.md) is authoritative.

## Release map

| Order | Target | Milestone | User-visible outcome | Depends on |
|---|---|---|---|---|
| 1 | 0.2.3 | [M23 Safety and recovery](#m23-safety-and-recovery) | Remove, recover and understand partial operations safely | 0.2.2 |
| 2 | 0.2.4 | [M24 Provider control](#m24-provider-control) | Configure compatible providers, preview changes and inspect actual state | M23 |
| 3 | 0.2.5 | [M25 Skills and sources](#m25-skills-and-sources) | Manage large skill sources, selected rows, targets and reviewed updates | M23; M24 validation/result conventions |
| 4 | 0.2.6 | [M26 Diagnostics and daily operation](#m26-diagnostics-and-daily-operation) | Diagnose problems, refresh detection, adjust preferences and inspect recent operations | M23–M25 |
| 5 | 0.2.7 | [M27 Compatibility sessions and usage](#m27-compatibility-sessions-and-usage) | Expand verified coverage and explain the origin and limits of the data | M24–M26 |
| 6 | 0.2.8 | [M28 Distribution and release validation](#m28-distribution-and-release-validation) | Install easily and complete real workflows on supported platforms | M23–M27 |
| 7 | 0.3.0 | [M29 MCP management](#m29-mcp-management) | Manage MCP server definitions across verified agents | M23–M28 and explicit minor release review |
| 8 | 0.4.0 | [M30 Instructions and environment profiles](#m30-instructions-and-environment-profiles) | Switch a coherent local setup across modules and recover partial switches | M29 and instruction/profile design review |

## Implementation rules

- Start with a reproducible user flow and the current module service. The
  **Build** field of each task names the existing entry point and the smallest
  useful change. Tests cover its **Acceptance** conditions.
- Keep path/format knowledge in [adapters](internal/agent/agent.go), domain
  writes in module services, and UI I/O inside commands that return messages.
  Register new features in [app.features](internal/app/features.go); the CLI
  and TUI frameworks must not import concrete modules.
- Reuse [guarded writes and backups](internal/fsutil/fsutil.go), the
  [JSON settings editor](internal/agent/settings.go) and the
  [Codex TOML editor](internal/agent/codex_toml.go). Keep unknown fields,
  comments, permissions and changes made by another process.
- Build validation and preview from the same prepared change used to apply.
  Recheck the source version before writing. Report partial success per target;
  a multi-file operation is not an atomic transaction.
- Treat live agent files as actual state and saved profiles as desired state.
  A reachable endpoint does not prove that credentials or inference work.
  A historical backup with missing metadata does not prove its destination.
- Mask secrets in previews, history, errors and JSON. Portable setups carry
  references/placeholders; native login stores and trust approvals stay with
  their agents. Imported definitions are data until explicitly enabled.
- Keep network requests on demand, bounded and cancellable. No paid inference
  for connectivity checks, startup repository scans or execution while browsing.
- Preserve symlink placement, the user's editor and the single Go binary.
  Add an optional capability for an actual adapter need; extract shared code
  only when more than one implementation needs it. Avoid a generic workflow
  engine, background scheduler or growing service container.

These rules apply to every step below. Existing project conventions remain in
[CLAUDE.md](CLAUDE.md); the roadmap is the only planning document.

## M23 Safety and recovery

**Target:** 0.2.3. **Priority:** highest. **Dependency:** 0.2.2.
The operator must be able to remove an item, recover it and identify what
changed when a multi-target operation partly fails.

**Reference:** [backup and recovery surface](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/docs/user-manual/en/1-getting-started/1.5-settings.md).

### M23.1 Keep installed hook scripts valid

**Original failure:** reproduced in an isolated home: saving a script-backed
hook, enabling it in Claude Code and deleting the library entry left the
live command installed after its script was removed. The deletion confirmation
said installed hooks remained, so scripts had to remain usable too.

- [x] Refuse deletion of an imported hook's script package while any agent
  still uses its commands. Explain the dependent agents and the uninstall
  action. An unreadable agent config must not be treated as proof of no usage.
- **Acceptance:** import → enable in Claude Code → attempt deletion leaves
  the entry, scripts and live hook intact; uninstall → delete succeeds.
  Shared scripts survive while another library entry uses them. The same
  rule holds in CLI and TUI. Preserve the documented behavior for inline
  hooks that have no managed scripts.
- **Build:** Change Delete in [hooks/service.go](internal/modules/hooks/service.go) before
  either removal. Inspect every HooksHost.ReadHooks and retain agent identities
  and errors. hostsWith currently discards read failures, so do not use it
  unchanged to authorize cleanup. Reuse command identity matching; add
  regressions to [service_test.go](internal/modules/hooks/service_test.go).
- **Verified:** import/enable/refused delete/uninstall/delete regressions cover
  Claude Code and partial installation in Codex, shared scripts, unreadable
  configurations and retained inline commands. CLI and TUI flows pass; Crush
  now propagates JSON read errors instead of reporting an empty hook list.
  Full format/vet/race/build/docs/language checks pass. Manual tmux verification
  in an isolated home confirms the refusal preserves all files and uninstalling
  allows deletion. This completes M23.1, not the 0.2.3 release gate.

### M23.2 Restore skills independently of the current list

**Current problem:** removing the last library-only skill leaves a retained
archive but an empty selectable list. The b shortcut requires a selected row
and filters backups to that row, so deleted items have no direct restore path.

- [ ] Add an all-skills backup entry point reachable from an empty library.
  Search by original skill name/date and restore a deleted skill. Expose
  backup listing and restore in the CLI through the existing service.
- **Acceptance:** remove the last skill, reopen, locate its backup and
  restore it without first creating a placeholder skill. Corrupt archives
  preserve current data. Show that restoring files does not automatically
  recreate historical enablements unless those were captured.
- **Build:** Keep ListBackups/Restore in [skills/ops.go](internal/modules/skills/ops.go).
  Let [backups.go](internal/modules/skills/backups.go) request an unfiltered
  inventory and expose it through the skills palette/CLI even when no row is
  selected. Reuse staged extraction and safety backup; do not add another
  archive format or restore engine.

### M23.3 Recover provider and hook configuration changes

- [ ] Make existing agent-config backups discoverable by agent, file,
  operation and date. Add a redacted before/after preview and a guarded
  restore for known supported files; back up the current file first.
  Label missing legacy metadata as unknown. A basename alone does not prove
  which agent/file produced a backup; require a verified destination.
- **Acceptance:** apply/clear a provider or install/remove a hook, find the
  corresponding backup, preview it and recover it. A restore never silently
  replaces unrelated edits made since the backup: show those differences
  and require an explicit restore decision, rechecking the current file
  before writing. Reject unexpected destinations, preserve permissions and
  protect secrets. Skill archives and config-file backups remain distinct.
- **Build:** Start from Backup/WriteAtomicIfUnchanged in
  [fsutil.go](internal/fsutil/fsutil.go). Add only the provenance needed to map
  new backups to adapter-approved destinations; distinguish legacy unknowns.
  Read both files, redact the diff, verify the destination and source hash,
  back up current state, then conditionally restore. Refuse stale or ambiguous
  restores instead of guessing a path from a filename.

### M23.4 Explain every target of a bulk operation

**Current problem:** reproduced with Claude Code and Codex: a profile with a
fake token and no envKey changes Claude, then Codex rejects it. The service
returns the Codex error without listing Claude's successful write. Keep the
partial-operation semantics explicit instead of implying nothing changed.

- [ ] Report succeeded, failed and skipped targets for provider, hook and
  skill operations. Keep the result available until dismissed, with a retry
  path for failed targets and links to relevant recovery information.
- **Acceptance:** a provider valid for Claude but invalid for Codex shows
  both outcomes and the actual resulting state; it never implies that
  nothing changed. Retry does not reapply successful targets by accident.
  Existing CLI exit codes and JSON fields remain compatible.
- **Build:** Extend result-producing operations beside the current error-returning
  wrappers in [providers/service.go](internal/modules/providers/service.go),
  [hooks/service.go](internal/modules/hooks/service.go) and
  [skills/ops.go](internal/modules/skills/ops.go). Keep existing callers working.
  Render per-target results and rescan actual state; retain failed target IDs
  for retry instead of repeating the original all-target action.

**Release gate:** M23.1–M23.4 checked; isolated regressions cover dangling
scripts, an empty library, corrupt backups, concurrent file edits and partial
writes; all common checks pass.

## M24 Provider control

**Target:** 0.2.4. **Dependency:** M23.
Configure a connection knowing which agents can use it and what applying it
will change.

**Reference:** [provider setup and discovery](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/docs/user-manual/en/2-providers/2.1-add.md), [config editing and conflicts](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/docs/user-manual/en/2-providers/2.3-edit.md), [connectivity limits](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/docs/user-manual/en/4-proxy/4.5-model-test.md).

### M24.1 Validate the selected agents before applying

- [ ] Choose explicit destination agents and validate endpoint, protocol,
  model requirements and credential references for each. Show incompatible
  targets before any write; explain activation/restart requirements.
- **Acceptance:** missing Codex envKey or incompatible wire API is found
  before changing Claude. Choosing the valid subset is explicit. Validation
  never equates a present env-var name with a valid credential. Revalidate
  files at write time and report runtime failures through M23.4.
- **Build:** Keep validation in adapters beside ProviderHost rather than in the form.
  Prepare all selected targets without writing, then pass the prepared effects
  to the existing confirmation. [providers/service.go](internal/modules/providers/service.go)
  and [form_ui.go](internal/modules/providers/form_ui.go) consume the same
  validation; apply still rechecks live files before each write.

### M24.2 Add a small maintained set of connection templates

- [ ] Offer native agent defaults and verified direct-connection templates
  for common protocol/local-server setups, plus the current custom form.
  Explain fields beside the focused input and retain custom editing.
- **Acceptance:** a user can configure a supported direct endpoint without
  guessing the protocol. Each template declares compatible agents,
  required inputs and its verification source/date. Do not promise protocol
  conversion or bundle a large sponsored vendor catalog.
- **Build:** Add small declarative templates beside
  [form_ui.go](internal/modules/providers/form_ui.go), filling the existing
  ProviderProfile fields. Describe protocol, required credential reference and
  compatible agents explicitly. Keep custom inputs and native defaults; avoid
  a new preset framework, vendor-specific credential store or protocol proxy.

### M24.3 Adopt existing settings and inspect configuration previews

- [ ] Save an externally configured provider as a named profile without
  changing the agent. Preview managed fields and affected files before
  apply/clear. Keep official login under the native agent's control.
- **Acceptance:** import → preview → apply → clear preserves unknown
  settings and the previous provider. Tokens stay masked; credential
  references or explicit re-entry cover secrets that cannot safely be
  adopted. Changing a library profile clearly distinguishes saved settings
  from the settings currently applied.
- **Build:** Build on Status and read-only ProviderHost.ReadProvider in
  [providers/service.go](internal/modules/providers/service.go) and
  [provider.go](internal/agent/provider.go). Save an adopted library profile
  separately from live config. Generate redacted previews inside adapters from
  the same edits used by apply/clear; preserve existing previous-state records.

### M24.4 Discover models and check endpoint reachability on request

- [ ] Add bounded, cancellable connectivity checks and model discovery for
  endpoints with verified support; retain manual model entry.
- **Acceptance:** distinguish reachable address, authenticated catalog and
  verified model availability. HTTP 401/404 is not reported as successful
  authentication. A reachable URL does not prove inference works.
  No inference is sent for a connectivity check; credentials are not
  forwarded to another origin through redirects, logs or error text.
- **Build:** Use Go net/http in the provider service with context cancellation,
  bounded response bodies and explicit timeouts. Separate a no-inference
  reachability probe from an authenticated model-catalog request. Preserve
  endpoint subpaths, avoid duplicate API prefixes and test redirects, 401,
  unsupported catalogs, cancellation and malformed data with a local server.

### M24.5 Show drift and resolve ambiguous profile matches

- [ ] Compare saved profiles with the live provider, explain external
  edits and distinguish profiles sharing an endpoint/model. Support an
  explicit refresh and a choice to retain external settings or reapply.
- **Acceptance:** two profiles differing in protocol or credential reference
  are not silently shown as the same profile. Changes made outside
  lazyagents are visible and preserved until a chosen operation applies.
- **Build:** Refine matchProfile/Status in
  [providers/service.go](internal/modules/providers/service.go) to compare the
  adapter's non-secret effective fields. Represent ambiguous matches instead
  of selecting the first profile. Show saved versus live differences in
  [view.go](internal/modules/providers/view.go); never reapply during rendering.

**Release gate:** create/adopt → validate → preview → apply → inspect drift
→ clear/recover works for each currently supported provider adapter. Model
discovery failures leave configuration untouched.

## M25 Skills and sources

**Target:** 0.2.5. **Dependency:** M23 and M24's validation/result conventions.
Make hundreds of skills manageable while preserving ownership and local edits.

**Reference:** [skills inventory, discovery and destinations](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/docs/user-manual/en/3-extensions/3.3-skills.md).

### M25.1 Search and filter the whole skill inventory

**Current problem:** skillItem.FilterValue currently contains only the skill
name. A user cannot find a skill by its description or repository, making
large sources such as ECC difficult to manage.

- [ ] Search name, description, directory and source. Filter by library/local,
  enabled agent, nowhere enabled, invalid metadata and update state.
- **Acceptance:** find a skill by its purpose or repository; combine a text
  search with an agent/state filter; clearing filters preserves a valid
  selection and matching detail. Use a small fixed filter set.
- **Build:** Extend skillItem.FilterValue in [list.go](internal/modules/skills/list.go)
  with already-loaded metadata. Keep fixed state/agent filters in the tab and
  derive the displayed rows from that state. Do not rescan disk or fetch a
  source per keystroke; update the detail from the visible selected row.

### M25.2 Operate on selected rows with an explicit target scope

- [ ] Select multiple skills and apply enable, disable, update or remove
  to that selection. Make the selected count, agent destinations and the
  distinction between selected, filtered and all rows visible.
- **Acceptance:** a filtered bulk action touches exactly the displayed or
  explicitly selected set, never a hidden unrelated item. Local/foreign
  skills remain protected. Partial results use M23.4.
- **Build:** Keep selected skills by canonical directory identity in the existing
  [skills tab](internal/modules/skills/tab.go), not by row index. Resolve and
  freeze the affected set and agents before confirmation; pass each item to
  existing operations. Reconcile selections after filtering/removal and use
  M23.4 results instead of adding separate bulk implementations.

### M25.3 Install and activate in one TUI workflow

- [ ] Extend the source picker with agent destinations and a library-only
  option. Remember the last explicit target selection in the skills config.
  Show separate install and activation outcomes.
- **Acceptance:** install a subset from a source such as ECC, enable only
  the chosen agents and adjust the matrix afterward. Imported hooks remain
  separately unchecked. A failed activation keeps the installed skill and
  offers a retry without reinstalling it.
- **Build:** Extend [picker.go](internal/modules/skills/picker.go) and
  [install_ui.go](internal/modules/skills/install_ui.go); retain Discover and
  Install in [install.go](internal/modules/skills/install.go). Install once,
  then use existing enable operations for explicit agents. Persist destinations
  under the skills config section and keep hook selection a separate choice.

### M25.4 Manage reusable source repositories

- [ ] Add a user-maintained repository list with source URL, optional
  verified ref/subdirectory, last scan and per-source errors. Reuse source
  materialization and existing origin metadata. ECC is an optional source.
- **Acceptance:** add → scan on request → install selected skills → remove
  the source without removing installed skills. Source changes cannot
  silently redirect updates. Distinguish same-name skills and reject
  destination collisions. No scans or downloads at startup.
- **Build:** Store only source descriptors in the skills config and keep per-skill
  provenance in Origin in [install.go](internal/modules/skills/install.go).
  Scan each source once per explicit request; reuse marketplace/subfolder
  discovery. Pass validated refs as separate Git arguments, never shell text;
  keep URL/ref changes and installed source identities distinct.

### M25.5 Review updates and cancel long source operations

- [ ] Show changed files and a bounded diff before replacing a skill;
  distinguish upstream changes from local edits. Carry cancellation through
  clone/discovery/update and show progress per source/item.
- **Acceptance:** local edits remain protected in bulk updates; an explicit
  single-item replacement has a backup and visible diff. Cancelling leaves
  no partial library replacement or unmanaged child process. Legacy origin
  files still load when adding source revision metadata.
- **Build:** Extend CheckUpdates/checkOne in
  [updates.go](internal/modules/skills/updates.go) to compare staged remote
  files with local files before replacement. Reuse the current content hashes
  and backups. Thread cancellation through clone/discovery commands; the tab
  owns cancellation state and receives progress messages. Always clean staging
  directories and subprocesses without replacing the last good library state.

### M25.6 Bring maintenance commands to the CLI

- [ ] Expose source listing/scanning, update checking/application and skill
  profile listing/save/apply/remove using existing services. Share preview
  and target semantics with the TUI, with stable JSON.
- **Acceptance:** a scripted check → selected update → profile application
  yields the same state as the TUI. An invalid selection causes no writes.
  Guide/reference generation and backward compatibility checks pass.
- **Build:** Wrap existing operations in [cli.go](internal/modules/skills/cli.go),
  including ListProfiles, DiffProfile and ApplyProfile from
  [profiles.go](internal/modules/skills/profiles.go). Reuse the same target and
  preview logic as the TUI; add help and regenerate the reference. Do not
  maintain a second implementation of install, update or profile application.

**Release gate:** source → search → subset → targets → install → update
review → remove → global restore succeeds with a large synthetic library.
Measure filtering/scrolling against a recorded baseline and investigate
regressions rather than setting hardware-independent timing claims.

## M26 Diagnostics and daily operation

**Target:** 0.2.6. **Dependency:** M23–M25.
Let the operator understand the current environment and the next useful action.

**Reference:** [environment conflict diagnostics](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/docs/user-manual/en/5-faq/5.4-env-conflict.md), [preferences and backups](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/docs/user-manual/en/1-getting-started/1.5-settings.md).

### M26.1 Expose doctor results inside the TUI

- [ ] Reuse feature-contributed doctor checks through a read-only report
  shared with the CLI. Show problem severity, affected resource, explanation
  and a concrete next step; separate local checks from explicit network checks.
- **Acceptance:** a broken link, invalid config, missing executable and
  unavailable remote query each have a readable cause and retry/navigation
  action. Long diagnostics remain reachable at 40×16. Existing doctor JSON
  fields remain valid; extensions are additive.
- **Build:** Build on Check/DoctorCommand in [cli/doctor.go](internal/cli/doctor.go)
  and feature-contributed checks. Resolve the report in app composition and
  hand a read-only view to the UI; modules must not import app. Preserve the
  existing JSON shape while adding structured problems where needed. Start
  with local checks and make remote refresh an explicit action.

### M26.2 Diagnose environment and configuration precedence

- [ ] Detect verified conflicts between the current process environment and
  the agent's selected configuration, plus missing credential references.
  Show variable names and origins when known, without values.
- **Acceptance:** explain a conflicting endpoint or missing env var without
  printing a token. Confirm each adapter's actual precedence rules. Never
  execute shell startup files or automatically remove variables.
- **Build:** Add only verified precedence checks within the relevant
  [adapters](internal/agent/registry.go). Return structured names, source and
  next-step hints to doctor/providers. Inspect the current process environment
  and known config fields without evaluating shell files; redact secret values
  before constructing reports, not only when rendering them.

### M26.3 Refresh agent detection without restarting

**Current problem:** feature.Deps.Agents uses sync.Once, and users currently
have to reopen lazyagents after installing a CLI. Refresh needs deliberate
cache invalidation and propagation, not another independent scan in each tab.

- [ ] Invalidate the memoized detection on an explicit refresh and propagate
  fresh capabilities/counts to all tabs, preserving stable selections.
- **Acceptance:** a newly available CLI or changed binary path is reflected
  without quitting; a refresh failure does not erase other agents' data or
  trigger unsolicited network calls.
- **Build:** Replace the one-shot memoization boundary in
  [feature.go](internal/feature/feature.go) with an explicit synchronized refresh
  path. Publish one updated detection result through the existing
  [events](internal/tui/events/events.go), updating counts/capabilities together.
  Protect cached readers and preserve per-tab selections by resource identity.

### M26.4 Guide first use and expose local preferences

- [ ] Make empty states point to import/adopt, source discovery or provider
  creation as appropriate. Expose current theme, initial tab, tab order and
  visibility through the palette or a small settings view.
- **Acceptance:** an empty installation can reach its first enabled skill
  without reading source code. Preferences persist with config comments and
  foreign sections intact. No required onboarding wizard.
- **Build:** Reuse existing empty-state views, the
  [palette](internal/tui/components/palette.go) and
  [app layout](internal/app/layout.go). Save only owned values through the
  existing comment-preserving config API. Theme/tab controls belong to current
  config fields/sections; do not add a mandatory setup flow or rewrite foreign
  configuration to expose a handful of preferences.

### M26.5 Keep a bounded local operation history

- [ ] Record recent writes with time, module/action, affected targets,
  per-target outcome and backup references. Offer inspect/retry/recover
  entry points using the existing operation services.
- **Acceptance:** after restarting, identify a partial write and its backup.
  Use a documented bounded retention default; clear history independently
  of backups. Never store tokens, full config bodies, transcripts or raw
  command payloads in the history. Concurrent writes cannot silently
  replace one another's records.
- **Build:** Persist bounded metadata derived from M23.4's completed results and
  validated backup references. Use atomic guarded updates, a documented
  retention default and owner-only file permissions. Store IDs/status instead
  of config bodies or command payloads. On restart, inspect the current state
  before offering retry/recover; a log entry alone never authorizes replay.

### M26.6 Make interrupted and unsaved states predictable

- [ ] Preserve useful progress/results until dismissed, distinguish empty,
  loading, stale and failed states, and prevent accidental loss of edited
  form content. Keep focus, scroll position and essential actions visible.
- **Acceptance:** Escape, resize, mouse and keyboard have consistent
  behavior across forms, pickers and result views; no secret is restored
  into a visible input. Resuming work does not repeat a completed write.
- **Build:** Reuse [Confirm](internal/tui/components/confirm.go), panels and inputs;
  make the affected module modes explicitly represent editing, pending,
  complete and failed states. Keep messages/results reachable until dismissed.
  Track dirty non-secret form fields and cancellation separately from a write
  already committed; test transitions and focus, not just final frame size.

**Release gate:** diagnose → inspect cause → navigate/retry → refresh →
recover is usable in the TUI and CLI. Opening a view alone never starts a
paid model request or runs an imported command.

## M27 Compatibility sessions and usage

**Target:** 0.2.7. **Dependency:** M24–M26.
Expand support through recorded formats and show the limits of each measurement.

**Reference:** [usage sources and price overrides](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/docs/user-manual/en/4-proxy/4.4-usage.md), [session controls](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/docs/user-manual/en/3-extensions/3.4-sessions.md).

### M27.1 Refresh the versioned compatibility evidence

- [ ] Record representative current CLI versions and retain previous
  fixtures. Prioritize missing Gemini/Hermes evidence and config variants
  relevant to the next adapters. Record OS, CLI version and fixture origin.
- **Acceptance:** each newly advertised capability has sanitized real-format
  fixtures, foreign-setting preservation tests and a generated support
  entry. Detection alone never implies provider, hook or usage support.
- **Build:** Extend [record-fixtures.go](scripts/record-fixtures.go) and
  [fixtures_test.go](internal/agent/fixtures_test.go), retaining prior version
  directories. Record real CLI formats in isolated homes against the fake
  model, sanitize them and review golden changes. Verify each capability
  independently before regenerating the agent reference.

### M27.2 Add OpenCode provider control

- [ ] Verify OpenCode's current config and implement provider inventory,
  apply/clear and coexistence behavior through an optional adapter capability.
- **Acceptance:** add/select/clear a managed provider while retaining foreign
  providers and defaults; unknown fields survive; apply/clear round-trip is
  demonstrated with the pinned CLI. Respect its native auth ownership.
- **Build:** Implement an optional provider capability beside
  [opencode.go](internal/agent/opencode.go) after recording the actual config.
  Keep foreign provider entries separate from lazyagents' managed entries and
  model/default selection. Reuse M24 preflight/preview and existing write
  primitives; do not force coexistence into a destructive replace-all model.

### M27.3 Add Gemini provider control

- [ ] Verify the current native auth/config/environment paths and implement
  the supported direct provider flow with explicit protocol restrictions.
- **Acceptance:** switching and returning to native defaults preserve login,
  unrelated settings and environment files. Unsupported configurations fail
  during preflight. Do not copy CC Switch formats without checking Gemini.
- **Build:** Extend [gemini.go](internal/agent/gemini.go) only after pinning its
  current direct-provider/auth behavior. Describe all affected files in the
  prepared change and back up each before writing. Handle partial multi-file
  results through M23.4; native login and unrelated environment values remain
  untouched. Keep protocol restrictions in the adapter.

### M27.4 Improve usage coverage and price provenance

- [ ] Extract Gemini/OpenCode usage only where recorded data supports it.
  Add validated user price overrides for otherwise unpriced API models,
  carrying source/date/currency and showing measured versus estimated cost.
  Show freshness and unavailable data beside the figures.
- **Acceptance:** sum by the model/provider actually recorded per response;
  missing prices or token history stay unavailable. Subscription limits,
  tokens and money remain separate. Agent-recorded costs stay authoritative;
  user rates fill otherwise unpriced API events. Refresh/index fixtures prove
  no double counting; price edits do not rewrite transcripts or secretly
  fetch catalogs.
- **Build:** Add verified extraction to the relevant lineScanner and buckets in
  [index.go](internal/agent/index.go); bump indexVersion when persisted fields
  change. Add price overrides in the usage config and resolve them through
  [pricing.go](internal/agent/pricing.go), preserving recorded cost precedence.
  Reuse [aggregate.go](internal/modules/usage/aggregate.go) for CLI/TUI totals.

### M27.5 Close verified transcript gaps

- [ ] Verify Gemini timestamps and link OpenCode subtasks/Crush child
  sessions to their originating calls when real data establishes the link.
  Evaluate Hermes history/resume after obtaining its own recorded fixtures.
- **Acceptance:** correct time/ordering and child navigation are proved by
  fixture; absent linkage is shown honestly rather than guessed. Resume
  uses verified native commands. No source transcript is rewritten.
- **Build:** Begin with recorded parent/call IDs and timestamp units in
  [gemini.go](internal/agent/gemini.go), [opencode.go](internal/agent/opencode.go)
  and [crush_sessions.go](internal/agent/crush_sessions.go). Populate the shared
  Entry fields and reuse [transcript.go](internal/modules/sessions/transcript.go).
  Add Hermes only after the same evidence exists; no title/time heuristics
  that silently attach an unrelated child session.

**Release gate:** every added capability is independently evidenced and
documented. If a format cannot be verified, explicitly defer that capability
with a reason and retain the support matrix's current limitation; it is not
counted as delivered. No parser, price or credential format is guessed to
fill a cell.

## M28 Distribution and release validation

**Target:** 0.2.8. **Dependency:** M23–M27.
Make the refined release easy to obtain and demonstrate complete operations
on the supported systems.

**Reference:** [distribution and installation](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/README.md#download--installation).

### M28.1 Package installation and upgrade paths

- [ ] Prepare Homebrew and AUR recipes based on checksummed release
  archives, including licenses and upgrade/uninstall instructions.
  Publish only after the required maintainer repository/credentials exist.
- **Acceptance:** a clean package-manager installation and upgrade report
  the intended version and preserve user data. Repository/token/AUR-key
  dependencies are recorded explicitly; an unsubmitted recipe is not
  described as a published package.
- **Build:** Use the archive names and SHA256SUMS produced by
  [release.yml](.github/workflows/release.yml) as the packaging inputs. Keep
  recipes and small version/sum update scripts in the repo. Validate with the
  native package manager before publishing; avoid a custom download/install
  framework or embedding maintainer credentials in automation.

### M28.2 Validate full flows across platforms and sizes

- [ ] Record manual Linux/macOS/Windows results for install/activate,
  providers, hooks, failure/retry and recovery; cover each platform's
  supported capabilities and permissions. Exercise 40×16, 80×24, 120×34 and
  a wide terminal in the three SP Night variants with filled and error states.
- **Acceptance:** every field, result, question and exit action is reachable.
  Native CI already exists and remains required; manual results are
  separate. Windows without symlink permission gets an actionable diagnosis;
  a copy fallback requires a separate architecture decision.
- **Build:** Use [preview.go](scripts/preview.go) and isolated demo fixtures for
  keyboard, mouse, resize and complete/error flows. Keep native CI in
  [native.yml](.github/workflows/native.yml); record manual OS/CLI results in
  this task. Exercise reachable actions and actual file effects, including
  Windows symlink prerequisites, rather than only screen dimensions.

### M28.3 Complete guides and product presentation

- [ ] Update EN/PT-BR public guides, screenshots/demos and capability
  statements for implemented flows. Upload the existing social preview
  manually in repository settings when available.
- **Acceptance:** examples run against the release, docs/reference checks
  and translation checks pass, and presentation never advertises a planned
  feature. Record the social-preview upload result separately from asset creation.
- **Build:** Update the current module guides as behavior lands, regenerate
  references from code and review Portuguese source markers. Reuse
  [record-demo.sh](scripts/record-demo.sh) with isolated data. Keep screenshots
  representative of released behavior and mark repository metadata work here;
  do not create separate plans or claim that an asset was uploaded when it
  was only generated.

### M28.4 Verify the release as a whole

- [ ] Run all required checks on the chosen release revision, verify the
  platform archives/checksums and version reporting, and exercise the
  documented pinned Go install. Record startup/index/filter baselines.
- **Acceptance:** no unresolved high-severity issue in the released scope;
  core manual flows and native CI pass; migration/compatibility notes and
  recovery instructions ship with the release.
- **Build:** Choose the release revision after its tasks pass. Use existing CI,
  native checks and release packaging; download the archives, verify sums and
  execute the native binary's version/help. Verify the pinned Go install and
  record comparable performance baselines in this task. Keep tag creation
  separate from documenting a proposed target.

**Release gate:** M28.1–M28.4 verified or external publication dependencies
explicitly moved to the follow-up queue before release review. The 0.2.x
consolidation is complete only when the lifecycle scenarios below work.

## M29 MCP management

**Target:** 0.3.0, after an explicit minor release review.
**Dependency:** the 0.2.x consolidation and its recovery/diagnostic primitives.
This is a new managed resource, with its own guide, CLI and optional adapter
capabilities. The first supported agents are Claude Code and Codex.

**Reference:** [MCP definitions and per-app controls](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/docs/user-manual/en/3-extensions/3.1-mcp.md).

### M29.1 Verify formats and establish MCP inventory

- [ ] Record each initial agent's MCP scopes, transports, auth references,
  trust behavior and config variants. Read existing server definitions
  through optional adapter capabilities and show their ownership.
- **Acceptance:** real-format fixtures cover supported transports, disabled
  entries, unknown fields and source scopes. Unsupported transports/scopes
  are explicit. No server is started while browsing inventory.
- **Build:** Add a narrow optional MCP inventory capability under internal/agent,
  following [provider.go](internal/agent/provider.go) and
  [hooks.go](internal/agent/hooks.go). Pin actual Claude/Codex MCP paths/scopes;
  do not assume Claude MCP shares its hooks file. Keep unsupported transport,
  disabled, foreign and managed states explicit in fixtures and inventory.

### M29.2 Manage a shared server library and imports

- [ ] Add/edit server definitions and import selected existing definitions
  or reviewed JSON/TOML. Handle duplicate names, command/argument separation,
  credential masking and environment references.
- **Acceptance:** malformed or conflicting imports write nothing until
  resolved; import alone never installs a package or executes a command.
  Exported definitions exclude secret values by default.
- **Build:** Create a module in internal/modules/mcp using the current
  [feature contract](internal/feature/feature.go). Normalize imported
  definitions into validated typed data, keeping command and argument arrays
  separate. Reuse module config/storage and secret redaction patterns; no
  package installation, shell execution or authentication broker during import.

### M29.3 Enable and disable MCP per agent

- [ ] Add a server × agent matrix, preview affected config and apply/clear
  only managed definitions, with backup and conflict protection.
- **Acceptance:** enable → native agent recognizes definition → disable →
  recover preserves foreign servers and trust state. Partial results and
  concurrent edits follow M23/M24 conventions.
- **Build:** Build the matrix with the existing
  [table helpers](internal/tui/kit/table.go). Keep per-agent preparation and
  editing inside the adapters, reusing guarded JSON/TOML editing and M23
  backup/results. Test that changing providers, hooks and MCP entries in a
  shared live file preserves the other resources and external changes.

### M29.4 Integrate MCP diagnostics and CLI

- [ ] Expose matching CLI operations, help, stable JSON, local requirement
  checks and explicit optional connection tests. Extend the support matrix,
  safety/config docs and Portuguese guides.
- **Acceptance:** missing executables, unsupported transport and unresolved
  credential references are actionable. Any test that starts a process is
  explicitly requested, bounded and cleans up its process tree.
- **Build:** Follow the registration and CLI/help pattern in
  [hooks/feature.go](internal/modules/hooks/feature.go) and
  [hooks/cli.go](internal/modules/hooks/cli.go). Add local requirement checks
  to the same doctor report; implement optional running probes with bounded
  lifetime and process cleanup. Document capabilities before advertising a
  connection as tested.


**Areas:** internal/agent capabilities, a module in internal/modules/mcp,
existing editors/backup primitives and one registry entry. CLI and TUI
frameworks remain independent of concrete modules.

**Minor release gate:** inventory → import/create → validate → enable →
disable/recover works on both initial agents, all foreign settings survive,
credentials stay masked, and the common release checks pass.

## M30 Instructions and environment profiles

**Target:** 0.4.0, after its scope and compatibility review.
**Dependency:** M29 and verified instruction-file capabilities.
An environment profile composes existing managed resources; today's skill
profiles remain supported with their existing meaning.

**Reference:** [instruction variants](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/docs/user-manual/en/3-extensions/3.2-prompts.md), [project composition](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/README.md#provider-management).

### M30.1 Inventory and edit instruction files

- [ ] Discover verified global instruction files, starting with Claude Code
  and Codex, and offer read/edit through the existing Markdown reader/editor.
  Show actual file paths, ownership and activation behavior.
- **Acceptance:** opening a missing file does not create it; saving backs up
  the original and refuses stale edits. Never treat files from different
  agents as semantically interchangeable without verification.
- **Build:** Add a verified instruction-file capability in internal/agent for the
  initial global files. Reuse the Markdown/editor flow in
  [skills/doc.go](internal/modules/skills/doc.go), but route saves through
  guarded writes with source hashes and backups. Browsing missing files is
  read-only; do not walk unrelated projects to infer instruction scope.

### M30.2 Maintain named instruction variants safely

- [ ] Store named variants and preview applying them. Define managed-block
  versus whole-file ownership before implementation; preserve unmanaged
  content and show the chosen replacement scope.
- **Acceptance:** apply/change/disable can recover the preceding file without
  silently emptying unrelated instructions. Markdown from an imported
  package is data and never grants permission to execute its commands.
- **Build:** Start with an explicit ownership model: a managed section for added
  content or an explicitly chosen full-file variant with a recoverable prior
  snapshot. Keep original content, deletion semantics and external edits in
  the adapter tests. Reuse the existing reader/editor and diff presentation;
  disabling a variant must not empty unmanaged instructions.

### M30.3 Compose environment profiles from existing modules

- [ ] Reference a provider, skill selection, hooks, MCP servers and optional
  instruction variants per agent. Show one ordered plan with prerequisites
  and the expected effects before applying.
- **Acceptance:** a work/personal switch uses the existing module services,
  reports every result and can recover a partial switch. Capture previous
  state only within the declared scope; no unrelated agent is modified.
  Do not claim cross-file atomicity.
- **Build:** Compose narrow operations exposed by the existing modules; preserve
  [skill profiles](internal/modules/skills/profiles.go) and their current
  format. Resolve references and validate every selected resource before
  writing an ordered plan. Record completed steps and backup references for
  recovery, without introducing a global service container or claiming
  cross-file atomicity.

### M30.4 Export and import portable setups

- [ ] Export a versioned manifest of profile references, sources and required
  credential variable names; import with a preview and explicit conflict
  handling. Keep credentials and native login stores local.
- **Acceptance:** a clean isolated home can recreate a setup after supplying
  missing dependencies/credentials. Import does not enable executable hooks,
  launch plugins/MCP servers or apply settings until explicitly requested.
  Machine-specific paths and unknown schema versions are handled visibly.
- **Build:** Define a small versioned declarative manifest of resource references,
  source revisions and credential placeholders. Validate schema, conflicts
  and destination mappings before import. Reuse existing source materializers
  and module operations; no embedded installer/scripts, raw native auth stores
  or implicit enablement of imported executable resources.

### M30.5 Add explicit project scope

- [ ] Extend the verified profile/instruction operations to a user-selected
  project directory after documenting native discovery and trust rules.
  Separate project state from global profiles and from agent session history.
- **Acceptance:** switching project scope never edits a neighboring repo,
  follows no path outside the declared scope unexpectedly and preserves the
  agent's own trust decisions. Global/project precedence is explained.
- **Build:** Extend verified adapter paths with an explicit selected project root.
  Resolve canonical paths, check scope boundaries and native trust, then use
  the same prepared-change/backup flow as global operations. Keep project
  precedence and state separate from session history and global defaults;
  never apply an entire environment because the working directory changed.


**Areas:** optional instruction capabilities, a narrowly scoped profile
composition feature and existing module services. Keep path/format knowledge
in adapters; avoid a generic orchestration framework or a growing service
container in feature.Deps.

**Minor release gate:** capture → plan → apply → inspect → switch → recover
→ export/import is verified in isolated homes, including missing resources,
credential placeholders, concurrent edits and failure midway through a switch.

## Follow-up candidates

These are recorded possibilities with no assigned release. Promote one into
an ordered task only after naming the user outcome, evidence and dependencies.

| ID | Candidate | Admission condition |
|---|---|---|
| F01 | PT-BR UI | Message-catalog design, ownership under existing config sections, English fallback and untranslated machine-readable contracts; public PT-BR docs already exist |
| F02 | More provider quotas and balances | A verified read-only API and account-owned auth boundary; explicit opt-in, bounded cache, no embedded arbitrary script runtime |
| F03 | CLI installation/upgrade assistance | Begin with version/source/duplicate-install diagnostics and native package-manager commands; managed execution needs a separate scope decision |
| F04 | Native account sign-in shortcuts | Verified native login/status commands; never introduce cross-vendor OAuth/token brokerage or upload credentials |
| F05 | Hermes providers/history and other capability gaps not verified in M27 | Recorded native formats and preservation/resume evidence for the specific capability |
| F06 | Additional coding agents | A concrete user need and versioned fixtures; complete existing agents' priority gaps first |
| F07 | Launch campaign and community material | Product/recovery gates met, distribution status accurate and announcement dates explicitly chosen |
| F08 | Public skill-registry discovery | A documented, usable API, bounded on-demand queries and provenance/conflict handling compatible with M25; no permanent scraping dependency |

## Reconciled historical follow-ups

The completed work is summarized above; its detailed chronology remains in
Git and the changelog. These previously open items are assigned here so they
do not require consulting an old plan.

| Previous item | Current destination |
|---|---|
| M7 manual macOS/Windows TUI pass; M12/M13 filled/error states and three-theme visual review | M28.2 |
| M7 missing Gemini/Hermes version fixtures | M27.1; unverified capabilities tracked in F05 |
| M7 Hermes project-local directories and trust | M30.5 after native trust verification |
| M15 PT-BR UI translation | F01; EN/PT-BR READMEs and public docs are already delivered |
| M19 Gemini timestamp verification | M27.5 |
| M19 OpenCode subtasks and Crush child-session links | M27.5 |
| M20 social preview upload | M28.3; asset creation alone is not completion |
| M20 Homebrew/AUR and corresponding install docs | M28.1 |
| M20 announcement video, screenshots, community posts and dates | F07; newer demos are already part of 0.2.2 |
| Old whole-repo and TUI review notes | Current findings and construction guidance are integrated into M23–M28 |

## Required lifecycle scenarios

| Resource | Successful path | Failure and recovery path |
|---|---|---|
| Skills | source → subset → explicit agents → enable → reviewed update → disable/remove | cancel clone; retain local edits; restore last removed skill from an empty library |
| Providers | create/adopt → validate → preview → apply → read actual state → clear | incompatible target before write; runtime partial failure; external edits; config recovery |
| Hooks | import/read → choose supported events/agents → install → edit → uninstall → delete | shared or active scripts not deleted; unreadable agent config; restore previous configuration |
| Sessions | filter → read/search → child navigation where supported → export/resume | missing CLI/file, unsupported child link, malformed transcript and protected live session |
| Usage | choose period/agent → inspect source/freshness → refresh/export | remote failure keeps stale values labeled; unavailable prices stay unavailable; no double counting |
| MCP from 0.3 | import/create → validate → per-agent enable → native recognition → disable | duplicate name, missing executable/credential, unsupported transport, concurrent config change and recover |
| Environment profiles from 0.4 | capture → ordered preview → apply/switch → inspect → portable export/import | missing resource, project/global scope conflict, partial switch and guarded recovery |

## Completion and roadmap maintenance

1. Implement the next task's smallest complete behavior using existing services.
   Add a regression for a real failure mode and keep temporary data isolated.
2. Run gofmt, go vet, go test -race and go build as required by CLAUDE.md.
   Run docs/reference/translation checks; manually inspect affected UI flows
   in narrow and wide terminals when UI changes.
3. Update public guides/config/safety docs for actual behavior and stored
   files. Add implemented changes to Unreleased in CHANGELOG.md.
4. Check the task only after its acceptance criteria pass. Record meaningful
   evidence beside it, commit in English and advance Current work.
5. Update this roadmap in place when an adjustment is found. Choose the
   current milestone, a later milestone, a follow-up candidate or an exclusion.
   Keep its cause, code entry points, build approach, reference, acceptance
   criteria and target together. Check for an existing task before adding one.
   Do not create another roadmap, comparison report or history companion.
6. Before tagging, review every task in the batch, move any deferral
   explicitly, write release notes and validate the chosen revision.
   Do not turn this planning document into a release claim.

## Version 1.0 readiness

Consider 1.0 only when core resources have documented compatibility and data
ownership contracts, complete recovery paths, verified platform flows,
stable machine-readable outputs and explicit migrations for persisted formats.
There must be no unresolved high-severity defect in those flows. Record a
separate release review with evidence; completion of a numbered list or a
feature count is not sufficient.

## Scope exclusions

- Local API proxy, protocol conversion, request routing, aggregation and failover;
  M24 validates direct connections instead of translating protocols.
- Cloud credential/config sync, cross-vendor account brokerage and telemetry.
- Desktop shell, system tray, background daemon and automatic self-update.
- Desktop URI/deep-link and custom icon systems; use CLI/manifests and existing themes.
- OpenClaw workspace and personal-memory editing without a separate product decision.
- Automatic filesystem watching; explicit refresh remains the default.
- Automatic execution of imported hook/plugin/MCP code or forged native trust.
- Management of skills embedded inside Claude Code plugins.

These preserve the current product boundaries. Reopening one requires a
separate product decision with its operating cost and compatibility impact.
