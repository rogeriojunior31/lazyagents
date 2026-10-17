# lazyagents roadmap

Updated 2026-10-17. The released baseline is **0.2.2**. This is the active
execution plan. The detailed [M0–M22 history](docs/dev/backlog-history.md)
and [CC Switch comparison](docs/dev/cc-switch-comparison.md) explain what
already exists and why the next work matters.

## Current work

**Next task: [M23.1](#m231-keep-installed-hook-scripts-valid).**
No M23–M30 feature is implemented by this planning change.

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

The [changelog](CHANGELOG.md) is the release record. These are delivered
capabilities, not a claim that every workflow is free of defects.

| Release | Delivered outcome | Historical milestones |
|---|---|---|
| 0.1.0 | Central skill library and symlinks; sessions, usage, providers, hooks; external plugins; YAML config; CLI, doctor and release builds | M0–M6 |
| 0.2.0 | Usage filters/dashboard, configurable tabs and splash, incremental transcript index and initial protection fixes | M7 initial work, M8–M11 |
| 0.2.1 | TUI redesign, accessible secondary screens, theme system, English UI, generated references and native platform tests | M12–M16 and platform fixes |
| 0.2.2 | Pi/Crush, shared skill placement, stronger config/process protection, structured transcript reader, responsive layouts, selective install, public EN/PT-BR docs, demos and version consolidation | M7 follow-ups, M17–M22 and subsequent documentation |

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

For every task, preserve the architecture and safeguards in [CLAUDE.md](CLAUDE.md).
Reuse current services and adapter capabilities; introduce shared machinery
only when actual implementations need it. Keep the single Go binary and
terminal-first operation.

## M23 Safety and recovery

**Target:** 0.2.3. **Priority:** highest. **Dependency:** 0.2.2.
The operator must be able to remove an item, recover it and identify what
changed when a multi-target operation partly fails.

### M23.1 Keep installed hook scripts valid

- [ ] Refuse deletion of an imported hook's script package while any agent
  still uses its commands. Explain the dependent agents and the uninstall
  action. An unreadable agent config must not be treated as proof of no usage.
- **Acceptance:** import → enable in Claude Code → attempt deletion leaves
  the entry, scripts and live hook intact; uninstall → delete succeeds.
  Shared scripts survive while another library entry uses them. The same
  rule holds in CLI and TUI. Preserve the documented behavior for inline
  hooks that have no managed scripts.
- **Areas:** hooks service, CLI/TUI deletion, isolated regression tests.

### M23.2 Restore skills independently of the current list

- [ ] Add an all-skills backup entry point reachable from an empty library.
  Search by original skill name/date and restore a deleted skill. Expose
  backup listing and restore in the CLI through the existing service.
- **Acceptance:** remove the last skill, reopen, locate its backup and
  restore it without first creating a placeholder skill. Corrupt archives
  preserve current data. Show that restoring files does not automatically
  recreate historical enablements unless those were captured.
- **Areas:** skills backup service, picker, palette, CLI and guide.

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
- **Areas:** fsutil, adapter-owned file validation and module recovery UI/CLI.
  Reuse existing backup primitives; add a recovery module only where needed
  to expose the shared inventory.

### M23.4 Explain every target of a bulk operation

- [ ] Report succeeded, failed and skipped targets for provider, hook and
  skill operations. Keep the result available until dismissed, with a retry
  path for failed targets and links to relevant recovery information.
- **Acceptance:** a provider valid for Claude but invalid for Codex shows
  both outcomes and the actual resulting state; it never implies that
  nothing changed. Retry does not reapply successful targets by accident.
  Existing CLI exit codes and JSON fields remain compatible.
- **Areas:** existing module operation results, TUI result views and CLI output.

**Release gate:** M23.1–M23.4 checked; isolated regressions cover dangling
scripts, an empty library, corrupt backups, concurrent file edits and partial
writes; all common checks pass.

## M24 Provider control

**Target:** 0.2.4. **Dependency:** M23.
Configure a connection knowing which agents can use it and what applying it
will change.

### M24.1 Validate the selected agents before applying

- [ ] Choose explicit destination agents and validate endpoint, protocol,
  model requirements and credential references for each. Show incompatible
  targets before any write; explain activation/restart requirements.
- **Acceptance:** missing Codex envKey or incompatible wire API is found
  before changing Claude. Choosing the valid subset is explicit. Validation
  never equates a present env-var name with a valid credential. Revalidate
  files at write time and report runtime failures through M23.4.
- **Areas:** provider forms/service and optional adapter capabilities.

### M24.2 Add a small maintained set of connection templates

- [ ] Offer native agent defaults and verified direct-connection templates
  for common protocol/local-server setups, plus the current custom form.
  Explain fields beside the focused input and retain custom editing.
- **Acceptance:** a user can configure a supported direct endpoint without
  guessing the protocol. Each template declares compatible agents,
  required inputs and its verification source/date. Do not promise protocol
  conversion or bundle a large sponsored vendor catalog.
- **Areas:** providers form/templates and documentation.

### M24.3 Adopt existing settings and inspect configuration previews

- [ ] Save an externally configured provider as a named profile without
  changing the agent. Preview managed fields and affected files before
  apply/clear. Keep official login under the native agent's control.
- **Acceptance:** import → preview → apply → clear preserves unknown
  settings and the previous provider. Tokens stay masked; credential
  references or explicit re-entry cover secrets that cannot safely be
  adopted. Changing a library profile clearly distinguishes saved settings
  from the settings currently applied.
- **Areas:** provider inventory, form, confirmation and adapters.

### M24.4 Discover models and check endpoint reachability on request

- [ ] Add bounded, cancellable connectivity checks and model discovery for
  endpoints with verified support; retain manual model entry.
- **Acceptance:** distinguish reachable address, authenticated catalog and
  verified model availability. HTTP 401/404 is not reported as successful
  authentication. A reachable URL does not prove inference works.
  No inference is sent for a connectivity check; credentials are not
  forwarded to another origin through redirects, logs or error text.
- **Areas:** provider service, HTTP helpers and local HTTP-server tests.

### M24.5 Show drift and resolve ambiguous profile matches

- [ ] Compare saved profiles with the live provider, explain external
  edits and distinguish profiles sharing an endpoint/model. Support an
  explicit refresh and a choice to retain external settings or reapply.
- **Acceptance:** two profiles differing in protocol or credential reference
  are not silently shown as the same profile. Changes made outside
  lazyagents are visible and preserved until a chosen operation applies.
- **Areas:** provider matching/status, adapter metadata and TUI detail.

**Release gate:** create/adopt → validate → preview → apply → inspect drift
→ clear/recover works for each currently supported provider adapter. Model
discovery failures leave configuration untouched.

## M25 Skills and sources

**Target:** 0.2.5. **Dependency:** M23 and M24's validation/result conventions.
Make hundreds of skills manageable while preserving ownership and local edits.

### M25.1 Search and filter the whole skill inventory

- [ ] Search name, description, directory and source. Filter by library/local,
  enabled agent, nowhere enabled, invalid metadata and update state.
- **Acceptance:** find a skill by its purpose or repository; combine a text
  search with an agent/state filter; clearing filters preserves a valid
  selection and matching detail. Use a small fixed filter set.
- **Areas:** skills list/filter state and responsive layouts.

### M25.2 Operate on selected rows with an explicit target scope

- [ ] Select multiple skills and apply enable, disable, update or remove
  to that selection. Make the selected count, agent destinations and the
  distinction between selected, filtered and all rows visible.
- **Acceptance:** a filtered bulk action touches exactly the displayed or
  explicitly selected set, never a hidden unrelated item. Local/foreign
  skills remain protected. Partial results use M23.4.
- **Areas:** skills selection, matrix, confirmation and existing services.

### M25.3 Install and activate in one TUI workflow

- [ ] Extend the source picker with agent destinations and a library-only
  option. Remember the last explicit target selection in the skills config.
  Show separate install and activation outcomes.
- **Acceptance:** install a subset from a source such as ECC, enable only
  the chosen agents and adjust the matrix afterward. Imported hooks remain
  separately unchecked. A failed activation keeps the installed skill and
  offers a retry without reinstalling it.
- **Areas:** skills discovery/picker, module config and install results.

### M25.4 Manage reusable source repositories

- [ ] Add a user-maintained repository list with source URL, optional
  verified ref/subdirectory, last scan and per-source errors. Reuse source
  materialization and existing origin metadata. ECC is an optional source.
- **Acceptance:** add → scan on request → install selected skills → remove
  the source without removing installed skills. Source changes cannot
  silently redirect updates. Distinguish same-name skills and reject
  destination collisions. No scans or downloads at startup.
- **Areas:** skills source config, discovery, origin metadata and CLI/TUI.

### M25.5 Review updates and cancel long source operations

- [ ] Show changed files and a bounded diff before replacing a skill;
  distinguish upstream changes from local edits. Carry cancellation through
  clone/discovery/update and show progress per source/item.
- **Acceptance:** local edits remain protected in bulk updates; an explicit
  single-item replacement has a backup and visible diff. Cancelling leaves
  no partial library replacement or unmanaged child process. Legacy origin
  files still load when adding source revision metadata.
- **Areas:** skills updates/install, process context and diff viewer.

### M25.6 Bring maintenance commands to the CLI

- [ ] Expose source listing/scanning, update checking/application and skill
  profile listing/save/apply/remove using existing services. Share preview
  and target semantics with the TUI, with stable JSON.
- **Acceptance:** a scripted check → selected update → profile application
  yields the same state as the TUI. An invalid selection causes no writes.
  Guide/reference generation and backward compatibility checks pass.
- **Areas:** skills CLI, help/reference generation and services.

**Release gate:** source → search → subset → targets → install → update
review → remove → global restore succeeds with a large synthetic library.
Measure filtering/scrolling against a recorded baseline and investigate
regressions rather than setting hardware-independent timing claims.

## M26 Diagnostics and daily operation

**Target:** 0.2.6. **Dependency:** M23–M25.
Let the operator understand the current environment and the next useful action.

### M26.1 Expose doctor results inside the TUI

- [ ] Reuse feature-contributed doctor checks through a read-only report
  shared with the CLI. Show problem severity, affected resource, explanation
  and a concrete next step; separate local checks from explicit network checks.
- **Acceptance:** a broken link, invalid config, missing executable and
  unavailable remote query each have a readable cause and retry/navigation
  action. Long diagnostics remain reachable at 40×16. Existing doctor JSON
  fields remain valid; extensions are additive.
- **Areas:** cli checks, feature/app composition and diagnostic view.

### M26.2 Diagnose environment and configuration precedence

- [ ] Detect verified conflicts between the current process environment and
  the agent's selected configuration, plus missing credential references.
  Show variable names and origins when known, without values.
- **Acceptance:** explain a conflicting endpoint or missing env var without
  printing a token. Confirm each adapter's actual precedence rules. Never
  execute shell startup files or automatically remove variables.
- **Areas:** agent-owned diagnostics and provider/doctor presentation.

### M26.3 Refresh agent detection without restarting

- [ ] Invalidate the memoized detection on an explicit refresh and propagate
  fresh capabilities/counts to all tabs, preserving stable selections.
- **Acceptance:** a newly available CLI or changed binary path is reflected
  without quitting; a refresh failure does not erase other agents' data or
  trigger unsolicited network calls.
- **Areas:** feature detection cache, app composition and existing TUI events.

### M26.4 Guide first use and expose local preferences

- [ ] Make empty states point to import/adopt, source discovery or provider
  creation as appropriate. Expose current theme, initial tab, tab order and
  visibility through the palette or a small settings view.
- **Acceptance:** an empty installation can reach its first enabled skill
  without reading source code. Preferences persist with config comments and
  foreign sections intact. No required onboarding wizard.
- **Areas:** current tabs, palette, app layout and existing config sections.

### M26.5 Keep a bounded local operation history

- [ ] Record recent writes with time, module/action, affected targets,
  per-target outcome and backup references. Offer inspect/retry/recover
  entry points using the existing operation services.
- **Acceptance:** after restarting, identify a partial write and its backup.
  Use a documented bounded retention default; clear history independently
  of backups. Never store tokens, full config bodies, transcripts or raw
  command payloads in the history. Concurrent writes cannot silently
  replace one another's records.
- **Areas:** local metadata storage and result/recovery views.
  This is an operation log, not a workflow engine or compliance audit system.

### M26.6 Make interrupted and unsaved states predictable

- [ ] Preserve useful progress/results until dismissed, distinguish empty,
  loading, stale and failed states, and prevent accidental loss of edited
  form content. Keep focus, scroll position and essential actions visible.
- **Acceptance:** Escape, resize, mouse and keyboard have consistent
  behavior across forms, pickers and result views; no secret is restored
  into a visible input. Resuming work does not repeat a completed write.
- **Areas:** shared components and affected module state machines.

**Release gate:** diagnose → inspect cause → navigate/retry → refresh →
recover is usable in the TUI and CLI. Opening a view alone never starts a
paid model request or runs an imported command.

## M27 Compatibility sessions and usage

**Target:** 0.2.7. **Dependency:** M24–M26.
Expand support through recorded formats and show the limits of each measurement.

### M27.1 Refresh the versioned compatibility evidence

- [ ] Record representative current CLI versions and retain previous
  fixtures. Prioritize missing Gemini/Hermes evidence and config variants
  relevant to the next adapters. Record OS, CLI version and fixture origin.
- **Acceptance:** each newly advertised capability has sanitized real-format
  fixtures, foreign-setting preservation tests and a generated support
  entry. Detection alone never implies provider, hook or usage support.
- **Areas:** adapters, fixture recorder/matrix and generated references.

### M27.2 Add OpenCode provider control

- [ ] Verify OpenCode's current config and implement provider inventory,
  apply/clear and coexistence behavior through an optional adapter capability.
- **Acceptance:** add/select/clear a managed provider while retaining foreign
  providers and defaults; unknown fields survive; apply/clear round-trip is
  demonstrated with the pinned CLI. Respect its native auth ownership.
- **Areas:** OpenCode adapter, provider capability and recorded fixtures.

### M27.3 Add Gemini provider control

- [ ] Verify the current native auth/config/environment paths and implement
  the supported direct provider flow with explicit protocol restrictions.
- **Acceptance:** switching and returning to native defaults preserve login,
  unrelated settings and environment files. Unsupported configurations fail
  during preflight. Do not copy CC Switch formats without checking Gemini.
- **Areas:** Gemini adapter, provider validation and recorded fixtures.

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
- **Areas:** usage/pricing capabilities, incremental index, usage CLI/TUI.

### M27.5 Close verified transcript gaps

- [ ] Verify Gemini timestamps and link OpenCode subtasks/Crush child
  sessions to their originating calls when real data establishes the link.
  Evaluate Hermes history/resume after obtaining its own recorded fixtures.
- **Acceptance:** correct time/ordering and child navigation are proved by
  fixture; absent linkage is shown honestly rather than guessed. Resume
  uses verified native commands. No source transcript is rewritten.
- **Areas:** adapter transcript parsers, shared reader and fixture matrix.

**Release gate:** every added capability is independently evidenced and
documented. If a format cannot be verified, explicitly defer that capability
with a reason and retain the support matrix's current limitation; it is not
counted as delivered. No parser, price or credential format is guessed to
fill a cell.

## M28 Distribution and release validation

**Target:** 0.2.8. **Dependency:** M23–M27.
Make the refined release easy to obtain and demonstrate complete operations
on the supported systems.

### M28.1 Package installation and upgrade paths

- [ ] Prepare Homebrew and AUR recipes based on checksummed release
  archives, including licenses and upgrade/uninstall instructions.
  Publish only after the required maintainer repository/credentials exist.
- **Acceptance:** a clean package-manager installation and upgrade report
  the intended version and preserve user data. Repository/token/AUR-key
  dependencies are recorded explicitly; an unsubmitted recipe is not
  described as a published package.
- **Areas:** packaging, release workflow and installation guides.

### M28.2 Validate full flows across platforms and sizes

- [ ] Record manual Linux/macOS/Windows results for install/activate,
  providers, hooks, failure/retry and recovery; cover each platform's
  supported capabilities and permissions. Exercise 40×16, 80×24, 120×34 and
  a wide terminal in the three SP Night variants with filled and error states.
- **Acceptance:** every field, result, question and exit action is reachable.
  Native CI already exists and remains required; manual results are
  separate. Windows without symlink permission gets an actionable diagnosis;
  a copy fallback requires a separate architecture decision.
- **Areas:** isolated preview/demo fixtures, targeted flow tests and QA notes.

### M28.3 Complete guides and product presentation

- [ ] Update EN/PT-BR public guides, screenshots/demos and capability
  statements for implemented flows. Upload the existing social preview
  manually in repository settings when available.
- **Acceptance:** examples run against the release, docs/reference checks
  and translation checks pass, and presentation never advertises a planned
  feature. Record the social-preview upload result separately from asset creation.
- **Areas:** public docs/translations, demos, README and repository metadata.

### M28.4 Verify the release as a whole

- [ ] Run all required checks on the chosen release revision, verify the
  platform archives/checksums and version reporting, and exercise the
  documented pinned Go install. Record startup/index/filter baselines.
- **Acceptance:** no unresolved high-severity issue in the released scope;
  core manual flows and native CI pass; migration/compatibility notes and
  recovery instructions ship with the release.
- **Areas:** CI/release, changelog, release evidence and installation docs.

**Release gate:** M28.1–M28.4 verified or external publication dependencies
explicitly moved to the follow-up queue before release review. The 0.2.x
consolidation is complete only when the lifecycle scenarios below work.

## M29 MCP management

**Target:** 0.3.0, after an explicit minor release review.
**Dependency:** the 0.2.x consolidation and its recovery/diagnostic primitives.
This is a new managed resource, with its own guide, CLI and optional adapter
capabilities. The first supported agents are Claude Code and Codex.

### M29.1 Verify formats and establish MCP inventory

- [ ] Record each initial agent's MCP scopes, transports, auth references,
  trust behavior and config variants. Read existing server definitions
  through optional adapter capabilities and show their ownership.
- **Acceptance:** real-format fixtures cover supported transports, disabled
  entries, unknown fields and source scopes. Unsupported transports/scopes
  are explicit. No server is started while browsing inventory.

### M29.2 Manage a shared server library and imports

- [ ] Add/edit server definitions and import selected existing definitions
  or reviewed JSON/TOML. Handle duplicate names, command/argument separation,
  credential masking and environment references.
- **Acceptance:** malformed or conflicting imports write nothing until
  resolved; import alone never installs a package or executes a command.
  Exported definitions exclude secret values by default.

### M29.3 Enable and disable MCP per agent

- [ ] Add a server × agent matrix, preview affected config and apply/clear
  only managed definitions, with backup and conflict protection.
- **Acceptance:** enable → native agent recognizes definition → disable →
  recover preserves foreign servers and trust state. Partial results and
  concurrent edits follow M23/M24 conventions.

### M29.4 Integrate MCP diagnostics and CLI

- [ ] Expose matching CLI operations, help, stable JSON, local requirement
  checks and explicit optional connection tests. Extend the support matrix,
  safety/config docs and Portuguese guides.
- **Acceptance:** missing executables, unsupported transport and unresolved
  credential references are actionable. Any test that starts a process is
  explicitly requested, bounded and cleans up its process tree.

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

### M30.1 Inventory and edit instruction files

- [ ] Discover verified global instruction files, starting with Claude Code
  and Codex, and offer read/edit through the existing Markdown reader/editor.
  Show actual file paths, ownership and activation behavior.
- **Acceptance:** opening a missing file does not create it; saving backs up
  the original and refuses stale edits. Never treat files from different
  agents as semantically interchangeable without verification.

### M30.2 Maintain named instruction variants safely

- [ ] Store named variants and preview applying them. Define managed-block
  versus whole-file ownership before implementation; preserve unmanaged
  content and show the chosen replacement scope.
- **Acceptance:** apply/change/disable can recover the preceding file without
  silently emptying unrelated instructions. Markdown from an imported
  package is data and never grants permission to execute its commands.

### M30.3 Compose environment profiles from existing modules

- [ ] Reference a provider, skill selection, hooks, MCP servers and optional
  instruction variants per agent. Show one ordered plan with prerequisites
  and the expected effects before applying.
- **Acceptance:** a work/personal switch uses the existing module services,
  reports every result and can recover a partial switch. Capture previous
  state only within the declared scope; no unrelated agent is modified.
  Do not claim cross-file atomicity.

### M30.4 Export and import portable setups

- [ ] Export a versioned manifest of profile references, sources and required
  credential variable names; import with a preview and explicit conflict
  handling. Keep credentials and native login stores local.
- **Acceptance:** a clean isolated home can recreate a setup after supplying
  missing dependencies/credentials. Import does not enable executable hooks,
  launch plugins/MCP servers or apply settings until explicitly requested.
  Machine-specific paths and unknown schema versions are handled visibly.

### M30.5 Add explicit project scope

- [ ] Extend the verified profile/instruction operations to a user-selected
  project directory after documenting native discovery and trust rules.
  Separate project state from global profiles and from agent session history.
- **Acceptance:** switching project scope never edits a neighboring repo,
  follows no path outside the declared scope unexpectedly and preserves the
  agent's own trust decisions. Global/project precedence is explained.

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

Historical entries remain intact in the archive; the destinations below are
the active tasks. This prevents completed headings from hiding unfinished work.

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
| Old whole-repo and TUI review notes | Historical evidence; current findings are in the comparison and M23–M28 |

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
5. When a new adjustment is found, first decide whether it belongs to the
   current milestone, a later milestone, a follow-up candidate or an exclusion.
   Add an ID, user outcome, dependency, acceptance criteria and target batch.
   Check for an existing task before adding another.
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

- Local API proxy, protocol conversion, request routing, aggregation and failover.
- Cloud credential/config sync, cross-vendor account brokerage and telemetry.
- Desktop shell, system tray, background daemon and automatic self-update.
- Automatic filesystem watching; explicit refresh remains the default.
- Automatic execution of imported hook/plugin/MCP code or forged native trust.
- Management of skills embedded inside Claude Code plugins.

These preserve the current product boundaries. Reopening one requires a
separate product decision with its operating cost and compatibility impact.
