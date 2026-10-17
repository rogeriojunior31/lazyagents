# Management capabilities and CC Switch comparison

Reviewed 2026-10-17 against lazyagents 0.2.2, with main at 1387fb5, and the
local CC Switch checkout at 797e0276ffc20e54cbdf55e3de84505d77231c91.
The [active roadmap](../../BACKLOG.md) owns task status and release targets;
this analysis owns the comparison, evidence and scope decisions.

## Product direction

lazyagents already provides a useful terminal control surface for skills,
sessions, providers, hooks and usage. Its next improvement is a complete
management lifecycle: inventory the actual state, validate a proposed change,
preview its effect, apply it, explain each result and recover the prior state.
CC Switch is a useful reference for this workflow and for discovering gaps.

The highest-priority work is recovery and predictable operations across the
resources already supported. MCP is the next distinct managed resource.
Instruction files and profiles that combine several modules come after those
operations have clear ownership and recovery semantics.

ECC is a skill source. It is a useful large-source discovery/install scenario,
but it is not the desktop application being compared here.

## Existing capabilities and their limits

| Area | Delivered in lazyagents | Important remaining work |
|---|---|---|
| Skills | Central library, agent matrix, local adoption, source discovery, selective CLI install, source updates, local-edit detection, profiles and archive restore | Search purpose/source, selected-row operations, source list, install destinations in the TUI, reviewed update diff and accessible recovery after deletion |
| Providers | Four adapters, library/form, live state, guarded edits, backups and clear/restore of previous values | Target-specific preflight, previews, templates, discovery/probes, external-state adoption and ambiguity/drift |
| Hooks | Three adapters, import of script packages, event compatibility, partial states and command/script editing | Active-script deletion guard, complete result reports and config recovery |
| Sessions | Unified listing, filtering/search, aliases, native resume, export, protected deletion and structured reader | Version evidence, Gemini timestamps and verified child-session links; Hermes history is conditional on its formats |
| Usage | On-demand Claude/Codex limits, cached failures, indexed Claude/Codex/Pi history, per-response cost and period/agent/project/model views | Better source/freshness explanation, evidence-backed extra adapters and user-defined rates for otherwise unpriced API models |
| Diagnostics | CLI doctor and read-only Agents tab, capability/directory views and broken-link checks | TUI doctor reports, precedence conflicts, refreshed detection and actionable next steps |
| Configuration | Comment-preserving YAML, module sections, layout/splash options and themes | Discoverable settings and guidance in empty states |
| Recovery | Skill archives, surgical config backups, guarded writes and per-adapter previous-provider records | Global inventory, deleted-item access, config restoration and inspectable operation outcomes |
| Distribution | Five release targets, checksums, versioned Go install, native CI and EN/PT-BR docs | Package managers, manual platform lifecycle evidence and truthful capability presentation |
| Extension points | Optional adapter capabilities and JSON Lines plugins with guarded process lifetime | Add MCP/instructions within these boundaries, after the core lifecycle is dependable |

Evidence: [module registry](../../internal/app/features.go),
[agent support](../reference/agents.md), [skills guide](../guide/skills.md),
[providers guide](../guide/providers.md), [hooks guide](../guide/hooks.md),
[sessions guide](../guide/sessions.md) and [usage guide](../guide/usage.md).

## What to adopt from CC Switch

These are product decisions for lazyagents, rather than a commitment to copy
the desktop application's implementation or every advertised feature.

| Reference capability | Decision for lazyagents | Roadmap destination |
|---|---|---|
| Provider presets and guided fields | Adopt a small verified direct-connection set with per-agent requirements | M24.1–M24.2 |
| Import existing providers and view resulting config | Adopt read-first inventory/adoption and redacted previews | M24.3 |
| Connectivity and model catalog lookup | Adopt explicit bounded requests; keep reachability, authentication and model support distinct | M24.4 |
| Config edits from another program | Extend existing refusal/retry safeguards with visible drift and recovery choices | M23.3, M24.5 |
| Environment conflict notices | Adopt verified precedence diagnostics with variable names and masked values; no automatic shell-file cleanup | M26.2 |
| Skills Installed/Discover, source management and target selection | Adopt focused filters, repositories and an explicit install-to step | M25 |
| Public skill-registry search | Consider a documented API after repository discovery works well; keep provenance visible | F08 |
| Bulk matrix actions and target-specific failures | Adopt exact scope, complete outcomes and retry/recovery entry points | M23.4, M25.2 |
| Backup inventory, restore and import/export | Adopt accessible recovery first, portable non-secret manifests later | M23.2–M23.3, M30.4 |
| MCP library, import and per-app enablement | Adopt as a new module, initially Claude Code/Codex with verified transports/scopes | M29 |
| Prompt and instruction libraries | Adopt with declared file ownership and preservation of unrelated content | M30.1–M30.2 |
| Projects containing several resource types | Adopt explicit environment profiles composed from existing modules | M30.3–M30.5 |
| Usage source detail, pricing overrides and per-model totals | Extend current on-disk aggregation and make measurement/provenance clear | M27.4 |
| More account balances and subscription quotas | Consider individually, through verified read-only adapters and explicit opt-in | F02 |
| Apps page, install/update and duplicate-install diagnostics | Start with detection and native package-manager guidance; execution is separately scoped | M26.3, M28.1, F03 |
| Local routing, format conversion, failover and aggregation | Excluded; continue configuring agents' direct connections | Scope exclusions |
| Multi-account OAuth brokerage and subscription reuse across vendors | Excluded; native login remains with the agent | F04 permits native sign-in shortcuts only |
| WebDAV/S3 sync and desktop/tray/background operation | Excluded from the current product | Scope exclusions |
| Copy fallback when symlinks fail | Retain the symlink invariant and diagnose Windows prerequisites | M28.2 |
| Language and appearance settings | Keep existing themes; expose preferences; UI translation remains a separate candidate | M26.4, F01 |
| OpenClaw workspace and memory editing | Not an immediate coding-agent management requirement; no assigned implementation | Separate scope decision if requested |
| Desktop deep links and custom icon catalogs | Use existing CLI/import surfaces first; no dedicated URI scheme or asset system planned | M25.6, M30.4 |

The reference's connectivity guide explicitly distinguishes an HTTP response
from successful authentication or inference. Its prompt guide also documents
whole-file replacement/emptying. lazyagents should carry over the useful
controls while defining its own preservation and recovery behavior.

## Findings verified in the current code

### A1 Imported hook deletion can leave a broken live command

**Impact:** high. An installed hook can keep referencing a script removed
from disk, causing a later agent event to fail.

[Hook deletion](../../internal/modules/hooks/service.go) removes the library
entry and, if no other library entry shares it, its script directory. It does
not first check which agents still have the commands installed. The
[TUI confirmation](../../internal/modules/hooks/tab.go) says installed hooks
will remain, so the two lifecycle behaviors conflict for script-backed hooks.

Reproduced with a temporary home and cleared config overrides:

1. Save a hook package with a harmless local script.
2. Enable it in the isolated Claude Code settings.
3. Delete the library hook through the service used by CLI/TUI.
4. Observe that the live settings still reference the script and the script
   no longer exists.

**Required outcome:** M23.1 preserves active script dependencies and explains
how to uninstall them before deletion. Unreadable configs cannot justify cleanup.

### A2 A bulk provider failure does not describe successful writes

**Impact:** high for operator understanding. A failed command may already
have changed one or more agents.

The provider [service](../../internal/modules/providers/service.go) intentionally
continues after per-agent errors and joins those errors. The
[TUI result](../../internal/modules/providers/tab.go) refreshes live state but
does not provide a complete success/failure/skipped report for the operation.
This is an incomplete result contract, not evidence of cross-file atomicity.

Reproduced in an isolated home with Claude Code and Codex selected:

1. Save a profile with an endpoint, model and fake token, without envKey.
2. Apply it to all detected supported agents.
3. Claude Code is updated; Codex rejects the missing environment reference.
4. The returned error names Codex while Claude Code remains changed.

**Required outcome:** M23.4 describes every target; M24.1 catches known
incompatibilities before applying. Runtime I/O/concurrency failures still need
explicit partial results and guarded recovery.

### A3 Deleted skills have no direct global restore entry point

**Impact:** medium to high. A backup exists but is hard to reach at the
moment the user needs it.

The [backup shortcut](../../internal/modules/skills/list.go) requires a selected
skill. The [picker query](../../internal/modules/skills/backups.go) filters
backups to that selected directory. There is no global backup command in the
current CLI/palette. Removing a library-only skill removes its selectable row.

An isolated service scenario confirmed that removing the last skill leaves
an empty skill inventory and a retained backup. The UI control-flow inspection
establishes why the backup cannot then be reached directly through that row.

**Required outcome:** M23.2 opens all backups from an empty library and restores
a removed skill without recreating a placeholder first.

### A4 Discovery and management become harder as the library grows

**Impact:** daily friction. The skill item's
[FilterValue](../../internal/modules/skills/list.go) contains only its name.
There is no current selection of multiple skill rows, reusable source list or
TUI destination step equivalent to the CLI's selective install plus enable-all.

**Required outcome:** M25 searches metadata and source, scopes selected actions
and makes agent destinations explicit. A repository such as ECC is a useful
large input; installing all of it is not a default recommendation.

### A5 Existing detection and checks are not a complete operator workflow

The [detection cache](../../internal/feature/feature.go) uses sync.Once;
the [Agents guide](../guide/agents.md) currently instructs users to reopen
after installing a CLI. Doctor remains a separate CLI report. Provider checks
do not currently diagnose environment/config precedence, discover models or
test connectivity.

**Required outcome:** M26 reuses the existing diagnostic sources and makes
refresh, next actions and current results visible inside the TUI.

## Technical constraints that shape the implementation

- **Ownership before writing.** Files, directories and values outside
  lazyagents' declared scope stay foreign. Inventory/import does not imply
  permission to enable or execute.
- **One source of truth for live state.** Read agent files to verify the
  effect of writes; a library profile is desired state, not proof that a
  connection or credential works.
- **Guarded recovery.** A backup can contain later-unrelated settings from
  another time. Show the differences, back up the current file and refuse a
  stale restore rather than treating a whole-file copy as a safe merge.
- **Secrets stay local.** Mask previews, errors and JSON; portable manifests
  contain references/placeholders. Native authentication stores and trust
  approvals remain with their agents.
- **Explicit I/O.** Network probes, catalog scans and executable checks are
  requested, bounded and cancellable. Browsing a screen never performs paid
  inference or starts imported executable code.
- **Small additions to the existing architecture.** Adapters own CLI paths
  and formats. Existing module services own their writes. A new capability
  is optional; registration stays in app.features. Do not add a general
  transaction engine, background scheduler or service container to implement
  a few concrete flows.
- **Composition needs its own design step.** MCP shares config files with
  providers/hooks; environment profiles combine several modules. Their
  read/write ownership and ordering need integration tests. A multi-file
  switch can fail midway and must not be advertised as atomic.
- **Evidence per capability and version.** CC Switch's adapters are useful
  references, not a substitute for native CLI fixtures. Missing Hermes or
  Gemini data must remain an explicit limitation until verified.
- **Preserve the terminal deployment model.** Reuse native package managers,
  the user's editor and explicit refresh. Keep symlinks as the placement
  rule until a separately reviewed change says otherwise.

## Priority and validation

M23 addresses data/lifecycle correctness. M24 makes provider operations
predictable. M25 reduces friction with larger libraries. M26 connects those
operations to diagnostics and useful guidance. M27 adds verified coverage
and measurement detail. M28 proves the complete product on supported systems.
These form the 0.2.x consolidation.

M29 adds MCP because it is a distinct resource missing from the current
management surface. M30 adds instruction variants and coherent environment
profiles once each participating resource can be safely inspected and recovered.
This sequence is based on dependencies and user outcomes, not the desktop
application's feature count.

The [lifecycle scenarios](../../BACKLOG.md#required-lifecycle-scenarios)
cover success, partial failure, concurrent edits and recovery. Current native
CI is already delivered; manual workflow verification is still separate.
Performance work records comparable cold/warm baselines and realistic large
fixtures before claiming improvement. UI checks cover full/error/empty states,
keyboard and mouse, small and wide terminals and the SP Night variants.

This comparison uses source/manual inspection and isolated reproductions of
lazyagents behavior. It does not establish end-to-end reliability of the
CC Switch desktop application or current compatibility of every external CLI.

## Reference material

CC Switch links below are pinned to the checkout used for the comparison:

- [Product coverage and feature overview](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/README.md#features).
- [Provider creation and model discovery](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/docs/user-manual/en/2-providers/2.1-add.md).
- [Config editing and concurrent changes](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/docs/user-manual/en/2-providers/2.3-edit.md).
- [Skills management](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/docs/user-manual/en/3-extensions/3.3-skills.md).
- [MCP management](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/docs/user-manual/en/3-extensions/3.1-mcp.md).
- [Instruction and prompt management](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/docs/user-manual/en/3-extensions/3.2-prompts.md).
- [Usage sources and pricing](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/docs/user-manual/en/4-proxy/4.4-usage.md).
- [Connectivity-check meaning and limits](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/docs/user-manual/en/4-proxy/4.5-model-test.md).
- [Environment conflict diagnostics](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/docs/user-manual/en/5-faq/5.4-env-conflict.md).
- [Settings and backup surfaces](https://github.com/farion1231/cc-switch/blob/797e0276ffc20e54cbdf55e3de84505d77231c91/docs/user-manual/en/1-getting-started/1.5-settings.md).

lazyagents references: [architecture](../architecture.md),
[safety](../safety.md), [fixture matrix](../../internal/agent/testdata/fixtures/README.md),
[release history](../../CHANGELOG.md) and [archived milestones](backlog-history.md).
