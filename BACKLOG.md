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

**Next task: [M25.4](#m254-manage-reusable-source-repositories).**
M23.1–M23.4 are implemented and verified; the 0.2.3 batch is ready for release
review and remains unreleased. M24.1–M24.5 are implemented and verified for the
0.2.4 batch, merged in PR #14 and still unreleased. M25.1–M25.3 are implemented
and verified for the 0.2.5 batch; M25.4–M30 remain planned.

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

**Original failure:** removing the last library-only skill left a retained
archive but an empty selectable list. The b shortcut required a selected row
and filtered backups to that row, so deleted items had no direct restore path.

- [x] Add an all-skills backup entry point reachable from an empty library.
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
- **Verified:** `B` and the `skills backups` palette entry browse all skill
  archives; `b` keeps selected-skill filtering and opens all when empty. `/`
  searches original folder names and local dates. CLI `backups --query text
  --json` and `restore <backup>` also work under `skills`, using exact listed
  filenames/paths. Regressions cover reopening an empty library, search,
  cancellation, small terminals, corrupt archives preserving current data,
  safety backups and removed agent links staying removed. Documentation and
  confirmations explain that restoring files does not recreate enablements.
  Full format/vet/race/build/docs/language checks pass; isolated tmux checks
  confirm palette/filter/restore at 120 and 40 columns. M23.3 is next; the
  0.2.3 release gate remains open.

### M23.3 Recover provider and hook configuration changes

- [x] Make existing agent-config backups discoverable by agent, file,
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
- **Verified:** [Recovery](internal/modules/recovery/service.go) shares the same
  inventory/preview/restore flow between its tab and `config-backups` CLI.
  Provider apply/clear and hook install/uninstall snapshots carry provenance,
  source hashes and original permissions in private metadata. Unknown legacy
  origins remain ineligible; adapter-approved paths distinguish identical
  basenames and honor current config overrides. Review tokens bind source,
  metadata, destination contents, permissions and resolved parent directory;
  the final conditional rename rechecks them after the safety backup.
  All configuration values, commands and comments are hidden in previews;
  unrelated field/line changes still appear and whole-file replacement needs
  an explicit reviewed decision. Regression tests cover corrupt snapshots,
  unexpected/symlinked paths, stale reviews, stricter permissions, missing
  destinations, repaired JSON, metadata rotation and CLI/TUI restore flows.
  Format/vet/race/build/docs/language checks pass. Isolated CLI/tmux checks
  verify apply/clear, hook install/uninstall, unknown legacy origins, preview
  cancellation, preservation of an edit made after preview, 40-column scrolling,
  reviewed restore and refresh of Providers/Hooks state.
- **Scope:** recovery is per file; Pi's two files need separate reviews.
  Saved profiles, libraries and provider ownership records stay unchanged.
  Codex TOML with hook-related text or escaped keys requires native/manual
  review to preserve approvals. These limits and supported files are in the
  [recovery guide](docs/guide/recovery.md). Completing this task does not
  publish a release.

### M23.4 Explain every target of a bulk operation

**Current problem:** reproduced with Claude Code and Codex: a profile with a
fake token and no envKey changes Claude, then Codex rejects it. The service
returns the Codex error without listing Claude's successful write. Keep the
partial-operation semantics explicit instead of implying nothing changed.

- [x] Report succeeded, failed and skipped targets for provider, hook and
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
- **Verified:** [shared outcomes](internal/core/operation.go) retain per-agent
  success/failure/skip reasons and state observed after each attempt. Provider
  apply/clear, hook install/uninstall/command toggles and skill activation/profile
  services expose results alongside compatible error-returning wrappers.
  [Persistent reports](internal/tui/components/operation.go) scroll in narrow
  terminals, confirm failed-ID-only retries, preserve other outcomes and link
  to configuration recovery or skill archives. Matrices rescan after every
  attempt. CLI bulk diagnostics use stderr; exit codes, existing success
  stdout and list JSON fields stay compatible. No operation journal or new
  persistent format was added.
  Regressions reproduce Claude success/Codex rejection, verify read-back and
  token masking, correct the saved profile and prove Claude is not rewritten
  on retry. Hook tests cover a command failing inside a partially installed
  package and retry after its library flag already changed. Skill tests cover
  blocked paths, preserved local content, safe shared-link moves, failed-only
  profile retries and empty retry selections performing no writes. Per-skill
  read-back reuses scan classification without rescanning the whole library
  for each profile entry.
  Format/vet/race/build/docs/language checks pass. Isolated tmux checks at
  120 and 40 columns verify retained results, cancellation, failed-only retry,
  clear, recovery navigation, Codex warnings and actual matrix refresh.
  Public behavior is documented in the [Providers](docs/guide/providers.md#bulk-results),
  [Hooks](docs/guide/hooks.md#bulk-results) and [Skills](docs/guide/skills.md#bulk-results)
  guides, with current Portuguese translations.

**M23 review (2026-10-17):** reviewed all four tasks and their shared
adapter/filesystem/CLI/TUI call sites against the branch base. Regression tests
first reproduced and then verified fixes for shared script paths/aliases,
unknown dependencies in a damaged hook library, legacy dotted skill archive
names, Crush removal with duplicate/unreadable unmanaged JSON hooks, stale
Recovery inventories and recovery navigation discarding reports when the tab
is hidden. Reports now label state as observed after each attempt; opening
Recovery retains them, while the Skills shortcut opens its archive picker.
Deletion verifies dependencies before removing the library entry. No additional
roadmap or comparison document was created.

Fresh format/vet/race/build/docs/translation/language checks and the CI helper
builds pass. Cross-builds pass for linux/amd64, linux/arm64, darwin/amd64,
darwin/arm64 and windows/amd64. Isolated terminal tests at 80x24 and 120x36 cover
returning to Recovery after new snapshots, retained reports, failed-only retry,
redacted restore, actual provider refresh, dotted-skill restore after deleting
the last library row and hidden-tab fallback. Native macOS/Windows execution
still belongs to PR CI; cross-compilation does not replace those jobs. The
batch was merged in [PR #13](https://github.com/rogeriojunior31/lazyagents/pull/13);
M24.1 delivery is tracked below.

**Recovery usability refinement (2026-10-17):** the screenshot exposed a
technical inventory of repeated unknown fields with no useful next action.
The opening view now separates ready-to-preview copies from manual-only
archives, explains absent/unverified origins once and offers `l` to inspect
all copies. Readable dates with milliseconds, file/agent/action columns,
highlighted rows and side/bottom details replace raw archive IDs as the main
list. Exact saved paths and verified destinations remain in details. Enter on
an unavailable copy opens an explanation instead of a restore action. Agent
and destination are never inferred from a legacy basename; CLI JSON contracts
and restore checks are unchanged.

Regression tests cover legacy-only and mixed inventories, scope/filter
cancellation, click selection, no writes during manual inspection, available
preview verification and bounded layouts. Isolated tmux checks reproduce the
20-copy legacy case and exercise both scopes, inspection, redacted preview
and cancellation at 40/80/120/220 columns. Format/vet/race/build/docs/language
checks pass; the guides and generated key reference are current in English
and Portuguese. This remains an M23 refinement before PR, not M24.1 work.

**Release gate:** M23.1–M23.4 checked; isolated regressions cover dangling
scripts, an empty library, corrupt backups, concurrent file edits and partial
writes; all common checks pass. **Verified for the M23 batch on 2026-10-17.**
Release review/tagging remains a separate action; the released baseline is 0.2.2.

## M24 Provider control

**Target:** 0.2.4. **Dependency:** M23.
Configure a connection knowing which agents can use it and what applying it
will change.

**Reference:** [provider setup and discovery](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/docs/user-manual/en/2-providers/2.1-add.md), [config editing and conflicts](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/docs/user-manual/en/2-providers/2.3-edit.md), [connectivity limits](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/docs/user-manual/en/4-proxy/4.5-model-test.md).

### M24.1 Validate the selected agents before applying

- [x] Choose explicit destination agents and validate endpoint, protocol,
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
- **Delivered (2026-10-17):** optional `agent.ProviderValidator` checks the
  four existing provider adapters without writes. Claude shares its env/state
  reader with apply; Codex shares its pure managed-block editor; Pi checks both
  native files and its ownership state; Crush checks its block and input fields.
  [validation.go](internal/modules/providers/validation.go) prepares every
  selected target before the service writes the first one. Invalid selections
  block the entire batch; no automatic compatible-subset fallback is used.
  [targets_ui.go](internal/modules/providers/targets_ui.go) lets users select
  destinations and read affected files, reasons, effective protocol/credential
  handling and new-session requirements before the existing confirmation.
  The form uses the same validation for a read-only draft check (`ctrl+v`).
  CLI adds `provider validate [--json]` and `--agents id,id` for validate/apply;
  existing list JSON and `--agent` selections remain available. Environment
  references are checked for syntax, never authenticated or resolved to values.
  Apply revalidates after review and retains M23 runtime reports/retries.
- **Verified:** endpoint and env-reference guards in direct adapter writes;
  Codex envKey/protocol failures before Claude writes; invalid native files and
  stale confirmation reviews; explicit subset selection, cancellation, draft
  checks, masked JSON/UI and runtime failure/retry regressions. Full format,
  vet, race, build, English and documentation checks pass. Isolated tmux
  exercises cover incompatible selections, scrolling, subset confirmation and
  Claude/Pi writes while Codex/Crush remain untouched at narrow/wide sizes.
  Template delivery is tracked in M24.2; field-level previews, network checks
  and drift are M24.3–M24.5.

### M24.2 Add a small maintained set of connection templates

- [x] Offer native agent defaults and verified direct-connection templates
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
- **Delivered (2026-10-17):** [templates.go](internal/modules/providers/templates.go)
  holds six small choices: custom, native/previous agent settings, OpenAI
  Responses (Codex/Pi), OpenAI Chat Completions (Pi/Crush), an existing
  Anthropic bearer-token gateway (Claude Code), and local Ollama Chat
  Completions (Pi/Crush). Each declares protocol, supported template targets,
  required inputs, credential handling, primary source and review date.
  [templates_ui.go](internal/modules/providers/templates_ui.go) opens from `n`
  or `providers new`, shows those requirements without I/O and prefills the
  existing editable form. Endpoint/model stay explicit; no model or secret
  is guessed. Guided gateway setup requires its saved bearer token; native
  x-api-key/helper/sign-in configuration stays with the agent.
  Focused-field guidance appears beside/below the form and scrolls independently
  without changing inputs. Editing existing profiles keeps their current values;
  no template identifier, credential store or catalog is persisted.
  Native defaults reuse the existing clear/report/retry path with no agents
  preselected, an explicit selection and confirmation. Provider file discovery
  remains in `internal/agent` and includes both Pi destinations in the review;
  native login, unrelated settings and the library are preserved. Previous or
  external providers may remain: this is not a forced official-provider reset.
- **Sources reviewed:** [Responses](https://developers.openai.com/api/reference/resources/responses/methods/create),
  [Chat Completions](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create),
  [Claude gateway setup](https://code.claude.com/docs/en/llm-gateway-connect),
  [Ollama compatibility](https://docs.ollama.com/api/openai-compatibility),
  plus the native clear behavior documented in the [provider guide](docs/guide/providers.md#connection-templates).
  These verify configuration guidance and documented protocols; no real account,
  gateway authentication or inference was exercised. Template targets describe
  the original setup; custom changes still use M24.1's native configuration
  checks and do not prove server protocol/model compatibility. No conversion,
  new network call, vendor catalog or native-format rewrite was introduced.
- **Verified:** each connection validates, applies and reads back through its
  declared native adapters in isolated homes; keyless Ollama remains keyless.
  Regressions cover required fields, custom edits, saved-token masking,
  unchanged profile storage, draft checks, cancellation and selected-only native
  clear with retained reports. Small/wide layouts keep inputs, help and controls
  reachable, including independent help scrolling. Full format/vet/race/build,
  docs/translation/English checks and isolated tmux flows pass. Adoption and
  field previews are next in M24.3; discovery/network checks and drift remain
  M24.4/M24.5.

### M24.3 Adopt existing settings and inspect configuration previews

- [x] Save an externally configured provider as a named profile without
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

- **Delivered (2026-10-17):** [adopt.go](internal/modules/providers/adopt.go)
  reads representable public fields through `ProviderHost.ReadProvider`, opens
  an editable draft with `i`, or saves a new library entry through
  `provider adopt <name> --agent id`. Native tokens are never copied: a literal
  credential requires explicit re-entry or an agent-supported reference.
  Existing library names are refused and saving never applies native settings.
  Native account sign-in, advanced options and credential helpers stay native.
- **Construction:** [provider_preview.go](internal/agent/provider_preview.go)
  adds the optional `ProviderPreviewer` capability. Claude/Pi share their pure
  settings/state edits with persistence; Codex/Crush share their existing block
  editors and profile builders. Previews expose file actions and value-free
  managed field changes, including both Pi files and previous-value restoration.
  Private in-memory revisions bind native bytes, ownership and the saved profile
  to confirmation; changed reviews block all selected writes. Existing backups,
  conditional atomic writes and per-target runtime results remain in use.
  [targets_ui.go](internal/modules/providers/targets_ui.go) handles apply, clear
  and failed-only retry previews; cancelling a retry retains the original report.
  CLI adds `apply/clear --preview [--json]`. Imports matching public fields stay
  labeled external until managed edits exist, so space applies the imported
  profile. Changed saved models show the separate live model and reapply action.
- **Native evidence and boundaries:** Pi reads explicitly selected custom
  providers, honoring model-level API/endpoint overrides from its
  [models reference](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/models-configuration.md);
  inherited/unsupported protocols are refused for import. Crush reads explicit
  global JSON `models.large`/`providers` connections for `openai-compat` and
  `anthropic`, checked against the
  [v0.96.1 schema](https://github.com/charmbracelet/crush/blob/v0.96.1/schema.json),
  only when no executable rc overrides can change the result. Arbitrary Bash is
  never evaluated. Import does not resolve project/environment precedence or
  copy custom headers/options. Claude's overwritten native token remains only
  in its existing backup; clear restores previous endpoint/model, not that token.
- **Verified:** isolated import → save → preview → apply → clear for all four
  adapters preserves unknown settings and previous provider/model selection,
  including Codex trust data. Regressions cover credential re-entry/references,
  duplicate names, unsupported sources/protocols, no script execution, unchanged
  native files during import/preview, matching preview/write actions, changed
  reviews, saved/live state, selected-only retries, cancellation and small/wide
  layouts. Format/vet/race/build, docs/translation/English checks and isolated
  tmux import/apply/clear with masked fields pass. No new network operation or
  release/version change. M24.4 owns discovery/connectivity; M24.5 owns complete
  protocol/credential-reference drift comparison and ambiguous matches.

### M24.4 Discover models and check endpoint reachability on request

- [x] Add bounded, cancellable connectivity checks and model discovery for
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

- **Delivered (2026-10-17):** [connection.go](internal/modules/providers/connection.go)
  separates a credential-free HEAD probe of the explicit base URL from one
  OpenAI-compatible GET to `<base>/models`. It preserves configured prefixes
  without guessing another `/v1`. Saved/typed bearer tokens take precedence
  over a named environment reference resolved only in the lazyagents process;
  keyless catalogs remain supported. Native login, helpers and custom headers
  are never read. The credential rule in [CLAUDE.md](CLAUDE.md) now explicitly
  allows this on-demand request-scoped use of profile credentials.
- **Construction and controls:** [connection_ui.go](internal/modules/providers/connection_ui.go)
  exposes `c` / form `ctrl+t` for address checks and `m` / form `ctrl+l` for
  catalogs, also available through the Providers palette. The UI captures and
  cancels its request context, ignores late replies and closes requests on
  module shutdown. Filter/select an exact ID into the existing edit form;
  save/apply remain separate and manual entry is retained. CLI adds
  `provider probe/models <profile> [--json]` with Ctrl+C cancellation through
  [connection_cli.go](internal/modules/providers/connection_cli.go).
- **Evidence and limits:** results distinguish an HTTP response, credentials
  configured/sent/rejected/forbidden, a parsed catalog and the selected ID's
  listed/not-listed/not-selected/not-checked state. A 401 never means accepted
  authentication, and a 404/405 does not mean a supported catalog. A successful
  credentialed catalog request is recorded without independently certifying a
  token that a public server might ignore. Catalog membership does not prove
  inference, tools or actual agent compatibility. Reviewed against the
  [OpenAI list-models API](https://developers.openai.com/api/reference/resources/models/methods/list)
  and [Ollama compatibility reference](https://docs.ollama.com/api/openai-compatibility).
  Anthropic/native-account/custom-auth catalogs stay manual-only until their
  schema and credential flow are explicitly supported; no protocol is guessed.
- **Safety:** Go `net/http` uses a 10-second deadline, normal TLS verification,
  64 KiB response-header and 1 MiB decompressed body limits. All redirects are
  refused, including same-origin redirects. Single-page catalogs are bounded
  to 10,000 entries; unsafe/oversized/credential-echoing IDs, explicit pagination
  and malformed bodies are refused. No raw server body, redirect destination or
  network error is rendered. URL queries/userinfo/fragments are refused for
  these requests. No inference, automatic startup/reload/validation/preview/apply
  requests, native writes, backups or persistent catalog cache were added.
- **Verified:** local HTTP fixtures cover prefixes/escaped paths, credential
  precedence, missing environment references, keyless catalogs, 401/403/404/405,
  other failures, malformed/hostile catalogs, limits including gzip expansion,
  TLS rejection, same/cross-origin redirects, header/body cancellation and
  timeout. TUI tests cover no network on activation, model filtering/selection,
  preserved drafts, late replies and compact/wide layouts. Full format/vet/race/
  build, docs/translation/English checks and isolated tmux flows pass, including
  actual built-CLI Ctrl+C cancellation and unchanged library/native config.
  Tests use no real account or external inference. M24.5 below completes the
  comparison flow; the 0.2.4 batch is still unreleased.

### M24.5 Show drift and resolve ambiguous profile matches

- [x] Compare saved profiles with the live provider, explain external
  edits and distinguish profiles sharing an endpoint/model. Support an
  explicit refresh and a choice to retain external settings or reapply.
- **Acceptance:** two profiles differing in protocol or credential reference
  are not silently shown as the same profile. Changes made outside
  lazyagents are visible and preserved until a chosen operation applies.
- **Cause:** endpoint/model matching previously returned the first candidate,
  including an endpoint-only fallback. It could hide a changed protocol/env
  reference or imply that one of several equivalent profiles was applied.
- **Construction:** the optional
  [ProviderFieldReader](internal/agent/provider_fields.go) projects each
  adapter's configured public endpoint/model, normalized protocol and native
  credential precedence from redacted profiles. Unspecified optional fields
  do not constrain recognition; their writes still use the existing native
  apply/clear rules. [comparison.go](internal/modules/providers/comparison.go)
  records per-profile differences and every public match; only one candidate
  fills Status.Profile. No endpoint fallback, native login or variable lookup.
  Ownership/library read errors remain visible instead of claiming a match.
- **Delivered:** the matrix/detail separates saved/live values, lists ambiguous
  candidates and explains differences for the selected agent. `r` refreshes
  without writes/network; leaving live settings unchanged or cancelling retains
  them. `p` always previews reapplying the named profile to the selected agent,
  including token rotation when public fields already match. `space` clears
  only a unique managed match; an ambiguous/external match previews application.
  CLI `provider list --json` adds `matches`/`comparisons`; `profile` is unique
  only, and `saved_changed` indicates no saved public-field match for an active
  snapshot. Existing review bindings, conditional writes, backups and partial
  result/retry handling are reused.
- **Limits:** token values are never compared. Profiles differing only in a
  literal secret, ignored fields or equivalent defaults can remain ambiguous
  after apply; choosing a named row resolves the operation, not historical
  identity. A public match proves neither authentication nor token equality.
  Differences do not identify which side changed; the snapshot covers global
  representable native fields, not project/process overrides. Recovery remains
  per file and does not reapply associated files or ownership state.
- **Verified:** native defaults, protocol/credential precedence, aliases and
  ambiguity across Claude Code/Codex/Pi/Crush; native endpoint/reference edits,
  no fallback match, read-only refresh and cancelled previews, explicit reapply,
  secret-only rotation, malformed library/ownership state and unknown Crush
  protocol. For all four adapters, clear and reviewed Recovery restore the
  exact backed-up drift file. TUI coverage includes 40/80/120/220-column layouts
  and no secrets in UI/JSON. Isolated tmux verifies ambiguity, saved/live env
  differences, preview/cancel/confirm, backups, clear/restoration, Recovery and
  preserved Codex trust. Full format/vet/race/build/docs/translation/English
  checks pass. Guides and generated references are synchronized in EN/PT-BR.

**Release gate:** create/adopt → validate → preview → apply → inspect drift
→ clear/recover works for each currently supported provider adapter. Model
discovery failures leave configuration untouched.

**M24 review and CI follow-through (2026-10-17):**

- Reviewed the complete M24 diff and the failed Native runs from the M23 PR
  ([PR run](https://github.com/rogeriojunior31/lazyagents/actions/runs/39048914973),
  [merge run](https://github.com/rogeriojunior31/lazyagents/actions/runs/39051521034)).
  Both macOS and Windows failed the same Recovery identity assertion: joining
  the filename to a longer native temporary root wrapped inside the identity.
  Details now show filename and folder separately; regression coverage varies
  the root length across 220 cases without skipping the native test.
- Provider CLI parsing now rejects extra positional arguments before any
  operation. Previously an extra argument could stop flag parsing before
  `--agent`, silently selecting every installed target for apply/clear.
  Regression checks preserve library, native files, ownership and backups.
- Confirmed writes capture input until their result arrives, preventing a
  second confirmation from submitting an overlapping operation. Codex's
  provider reader now consistently handles literal-quoted table names; import
  no longer loses those public fields. Both bugs have reproducing tests.
- Validate with the CI toolchain (Go 1.26.9), not only the workstation's Go 1.27:
  format/vet/race/build, shell syntax, English checks, standalone preview and
  fixture-recorder builds, and every published cross-build target. CI supports
  explicit workflow dispatch so the same checks can run on a branch before PR.
  Local Go 1.26.9 checks and all five cross-builds pass. Isolated tmux verifies
  quoted-name import, reviewed apply/completion and manual Recovery with a long
  folder; the built CLI rejects stray arguments without native changes. Keep
  checks enabled; native runners must confirm the actual branch revision.

M24.1–M24.5 satisfy the local adapter/fixture checks above; real credential and
inference validation are deliberately outside this gate. Release review,
PR/publishing and tagging remain separate actions; the released baseline stays
0.2.2. Continue with M25.1 on the same branch when requested.

## M25 Skills and sources

**Target:** 0.2.5. **Dependency:** M23 and M24's validation/result conventions.
Make hundreds of skills manageable while preserving ownership and local edits.

**Reference:** [skills inventory, discovery and destinations](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/docs/user-manual/en/3-extensions/3.3-skills.md).

### M25.1 Search and filter the whole skill inventory

- [x] Search name, description, directory and source. Filter by library/local,
  enabled agent, nowhere enabled, invalid metadata and update state.
- **Cause:** skillItem.FilterValue contained only the display name; source and
  purpose queries could not find an installed skill. Rebuilding rows also
  reread lint on the UI thread, and list-only filtering could reset selection
  before the detail caught up.
- **Construction:** [list.go](internal/modules/skills/list.go) indexes already
  loaded name/description, canonical folder/path and Origin type/source/subfolder.
  The scan command computes lint off the UI thread and caches it with the
  inventory. [filters_ui.go](internal/modules/skills/filters_ui.go) holds four
  fixed filter fields and projects rows from that cache, then uses Bubbles'
  synchronous in-memory SetFilterText. No per-keystroke filesystem/source scan,
  background request or search dependency. Late list matches cannot restore
  hidden rows. Selection and detail use canonical directory identity rather
  than a possibly duplicated display name.
- **Delivered:** `/` searches purpose, folder or repository; `f` combines text
  with library/local location, enabled-anywhere/nowhere/specific-agent state,
  invalid frontmatter/lint and cached update evidence. The panel supports
  draft/apply/cancel/reset; `F` clears text and state filters, while `esc` clears
  only text. Counts and active filters stay visible, empty searches remain
  editable with no actionable hidden selection, and details follow the visible
  row. Scan/reload retains filters; operations keep existing skill ownership,
  local-content protection and confirmations.
- **Update evidence:** only explicit `U` checks populate update states. Unknown
  and locally edited results are no longer described as all up to date; a
  no-Git check clears prior evidence, scans prune removed skills, and updates
  invalidate cached statuses. Filters never trigger a check. Unchecked/unknown
  includes entries without usable Git evidence; cached results can be stale.
- **Scope:** these are inventory-view filters, not new bulk operations. `space`,
  `a`, `x`, `u` and `d` use the selected visible skill. `A`, `U` and saved profile
  operations retain their explicit full-inventory scope, explained in the panel
  and guide. M25.2 adds reviewed selected-row operations; no version/release or
  persistent filter settings are introduced here.
- **Verified:** source/subfolder, path and purpose queries; text plus state
  filters; library/local and local/shared activation; invalid YAML/lint;
  available/edited/current/unknown evidence; duplicate-name selection, draft
  cancellation/reset, zero-result safety, cache-only lint after files disappear,
  reload and unchanged cross-tab counts. Isolated tmux exercises a 32-skill
  library/local inventory at 40/80/120 columns with combined agent/update and
  invalid-metadata filters, source search, no results and clearing; native skill,
  origin/config and backup bytes remain unchanged. Responsive tests also cover
  220 columns. Full format/vet/race/build/docs/English and five-target build
  checks pass with Go 1.26.9; guides and generated keys are current in EN/PT-BR.

### M25.2 Operate on selected rows with an explicit target scope

- [x] Select multiple skills and apply enable, disable, update or remove
  to that selection. Show selected/visible/total counts and explicit activation
  destinations before applying a frozen scope.
- **Cause:** the matrix only had a cursor and single-row operations. Filtering
  did not define a safe bulk set, and an implicit all-agent default would make
  it too easy to enable a large source in unintended destinations.
- **Construction:** [batch_ui.go](internal/modules/skills/batch_ui.go) keeps
  canonical folder marks in the existing Tab, prunes hidden/removed rows and
  captures the chosen rows, origins and agent paths in a reviewed plan. The
  existing scrollable Confirm defaults to No; a busy mode prevents submitting
  the same batch twice. [batch.go](internal/modules/skills/batch.go) rescans
  current state and calls existing ToggleResult, Update and Remove operations.
  M23.4 OperationReport identifies skill/agent or skill/library entries and
  narrows retries to failed entries without rebuilding the UI selection.
- **Delivered:** `m` or a checkbox click marks a folder, `v` replaces marks with
  every matching visible row (including off screen), and `V` clears them. `c`
  chooses enable/disable/update/remove and reviews every canonical folder.
  Activation initially targets the current agent column; the user can toggle
  individual agents, choose all or choose none, with an empty scope blocked.
  Update/remove explicitly affect the selected library copies and every agent
  reading them. Existing single-row keys stay single-row; `A`, `U` and profiles
  retain their documented full-inventory scopes. Clearing filters does not
  resurrect hidden marks. Panel counts remain complete at 40 columns.
- **Protection:** local/foreign rows and linked library entries are skipped.
  Bulk updates skip edited content, missing install hashes and non-Git skills;
  changed recorded Git sources require a new review. Existing archives precede
  replacement/removal. Remove now preserves the library copy if inspecting or
  cleaning managed links fails, allowing retry to finish cleanup. Activation
  keeps shared-link placement and native shared-reader constraints.
- **Verified:** canonical marks with duplicate display names, filtered/off-screen
  selection, empty scopes, hidden/deleted mark pruning, default cancellation,
  frozen plans across scans, duplicate-submit protection, explicit destinations,
  shared readers, local/foreign preservation, local Git updates and changed
  source rejection, backups/removal and pair-only retry without replacing
  successful links. Isolated tmux covers a 33-row inventory at 40/80/120 columns,
  cancelled writes, chosen-agent activation/deactivation, an injected permission
  failure and failed-only retry, edited-content preservation, removal, backup
  access and unchanged unrelated/native/config bytes. Format/vet/full race
  tests/build, docs/English/scripts and all five published-target build checks
  pass with Go 1.26.9; guide, safety and generated keys are current in EN/PT-BR.
- **Limits:** plans/marks are session-only. Existing operations run sequentially
  with existing per-skill clones; per-source progress, cancellation and file
  diffs remain M25.5. No new CLI commands, settings, dependencies or release
  publication are introduced.

### M25.3 Install and activate in one TUI workflow

- [x] Extend the source picker with explicit agent destinations and a
  library-only option. Remember the last confirmed target selection in the
  skills config. Show separate install/import and activation outcomes.
- **Cause:** source discovery installed a single entry immediately and the
  multi-entry picker only copied files. Activating a large source required
  another pass through the matrix; retrying a copy after an activation failure
  could collide with the files that were already installed.
- **Construction:** [picker.go](internal/modules/skills/picker.go) retains source
  selection and adds a destination step; [install_ui.go](internal/modules/skills/install_ui.go)
  handles review, busy states, source cleanup and the persistent report. Even
  single-entry and registry sources use the same review. Discovery and Install
  in [install.go](internal/modules/skills/install.go) keep their CLI contracts.
  [install_targets.go](internal/modules/skills/install_targets.go) calls Install
  once per chosen entry and passes only newly copied skills to M25.2 activation.
  OperationReport accepts an optional retry allowlist, so copy/import failures
  remain visible while only failed activation pairs or preference saves retry.
- **Delivered:** select entries, choose installed agents or Library only, review
  exact entries/destinations and confirm with No as the default. `tab` switches
  steps, `esc` goes back or cancels and cleans staging. Busy discovery/install
  states prevent duplicate submissions. `skills.installTargets` remembers the
  last confirmed explicit IDs; absent/empty means library only, never all.
  Saved unavailable IDs are warned about and omitted, not replaced by other
  agents. Confirming library only saves `[]`; hooks-only imports and cancellations
  do not change the preference. The module reads the current config for each
  save, using Config.Section/SetSectionValue/Save to retain foreign sections,
  sibling keys, comments and list style without adding global config fields.
- **Protection:** existing library collisions are neither overwritten nor
  activated by the install; other chosen entries continue. Failed activation
  retains files and retries links without cloning/copying/importing again.
  Preference errors appear separately and never roll back installed files or
  overwrite malformed config. Source hooks start individually unchecked;
  the all-skills key never checks them. Explicitly marked hooks are imported
  only and never activated by skill destinations. Native shared-reader and
  library-directory visibility constraints remain explicit in the review.
- **Verified:** selected subset, collisions, successful/failed activation pairs,
  no-copy retries after source removal and local edits, library-only and
  unavailable targets, preference round trips/errors/retry, unknown config keys
  and comments, single-skill review/default cancellation, staged-source cleanup,
  duplicate submission, hooks-only preference preservation and hook opt-in.
  Isolated tmux exercises a local Git source with 24 skills plus unchecked hooks,
  selecting two for Claude/Codex, injecting a blocked Codex path, retrying only
  its links while preserving edited files/origins, recalling destinations,
  cancelling clones, installing a single library-only skill and importing hooks
  without native settings writes. Destination/review panels fit 40/80/120 columns.
  Full format/vet/race/build, docs/English/scripts and all five published-target
  build checks pass with Go 1.26.9; guides, configuration, safety and generated
  keys are current in EN/PT-BR.
- **Limits:** copy/import failures need source/collision correction and a new
  explicit install. Source operations retain their existing time bounds;
  cancellation, per-source progress and update diffs remain M25.5. Remembered
  destinations affect only TUI installs, not CLI commands or batch actions.

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
