<!-- Generated from the code by `go test ./docs -update`. Do not edit by hand. -->

# CLI reference

Every command also prints its own page with `lazyagents help <command>`. Without arguments, `lazyagents` opens the TUI. Each external plugin adds one more command named after its id, described by `lazyagents help` once the plugin is installed ([plugins guide](../guide/plugins.md)).

Commands exit with `0` on success and non-zero on failure.

| Command | What it does |
|---|---|
| [`list`](#list) | list skills and how many agents have them enabled |
| [`enable`](#enable) | enable a library skill (symlink in the agent) |
| [`disable`](#disable) | disable a skill (removes the symlink) |
| [`install`](#install) | install skills from a GitHub repo, zip or directory |
| [`remove`](#remove) | remove a skill from the library |
| [`backups`](#backups) | list skill backups, including deleted skills |
| [`restore`](#restore) | restore a skill archive into the library |
| [`adopt`](#adopt) | move an agent's local skill into the library |
| [`migrate-library`](#migrate-library) | move the skills library to another directory |
| [`skills`](#skills) | manage skills, source scans, reviewed updates and activation profiles |
| [`sessions`](#sessions) | list agent sessions, newest first |
| [`provider`](#provider) | manage agent provider profiles (endpoint and token) |
| [`hooks`](#hooks) | list library hooks and install/uninstall them in agents |
| [`config-backups`](#config-backups) | inspect and recover agent configuration snapshots |
| [`history`](#history) | inspect recent local write attempts without replaying them |
| [`usage`](#usage) | agent consumption: subscription limits, tokens and cost per day, agent, project and model |
| [`doctor`](#doctor) | diagnose agents, skills, hooks, providers, usage and plugins |

## list

```text
lazyagents list [--json]
```

List skills and how many agents have them enabled.

```text
options:
  --json   JSON output: every skill with its source, lint state and per-agent state

Lists the library skills and the local skills found in the agents' skill
directories. ENABLED IN counts the agents that see the skill, LIBRARY says
whether lazyagents manages it.
```

## enable

```text
lazyagents enable <skill> [--agent id|--all]
```

Enable a library skill (symlink in the agent).

```text
options:
  --agent id   only this agent (claude-code, codex, gemini-cli, …)
  --all        every installed agent with a skills directory (the default)

<skill> is the folder name or the frontmatter name. Enabling creates a
symlink from the library into the agent's skills directory; an agent that
already sees the skill is left as is. Only library skills can be enabled:
adopt a local skill first.
```

## disable

```text
lazyagents disable <skill> [--agent id|--all]
```

Disable a skill (removes the symlink).

```text
options:
  --agent id   only this agent
  --all        every agent where lazyagents enabled it (the default)

Removes the skill from the agent only; it stays in the library and in the
other agents. A skill linked once in a shared directory (~/.agents/skills)
moves to the own directory of each agent that keeps it. A local skill (a real
folder or another tool's symlink) is never deleted: the command fails for it
instead.
```

## install

```text
lazyagents install <source> [skill...] [--all] [--hooks]
```

Install skills from a GitHub repo, zip or directory.

```text
sources:
  owner/repo                  GitHub repository (needs git in PATH)
  https://… · git@… · *.git   any git URL, cloned with --depth 1
  ./skills.zip                a zip file
  ~/some/folder               a local folder

options:
  --all     enable the installed skills in every installed agent with a
            skills directory, like lazyagents enable --all
  --hooks   also install the plugin hooks the source ships (hooks/hooks.json)

Every folder with a SKILL.md is installed into the library, found at any
depth; a Claude Code marketplace (.claude-plugin/marketplace.json) is read
through its manifest. Skill names after the source install only those; an
unknown name installs nothing and lists the skills the source has. A name
already in the library is skipped and reported. Without --all, installing
does not enable anything: use lazyagents enable next.
Without --hooks, hooks are only counted: a hook runs a third-party command
on every agent event.
```

## remove

```text
lazyagents remove <skill>
```

Remove a skill from the library.

```text
Removes the skill from the library and every lazyagents symlink that
points to it. A .tar.gz backup is written first to the backups directory,
and the Skills tab (B) or restore command recovers it. Local skills are not
in the library and cannot be removed here.
```

## backups

```text
lazyagents backups [--query text] [--json]
```

List skill backups, including deleted skills.

```text
options:
  --query text   filter by original folder name or local date (YYYY-MM-DD)
  --json         JSON array with skill, time and path

Lists every skill archive, newest first, even with an empty library.
Query words must all match the name/date. Restore an exact archive using
lazyagents restore <backup>; this command does not list agent config backups.
```

## restore

```text
lazyagents restore <backup>
```

Restore a skill archive into the library.

```text
Use an exact archive filename or path from lazyagents backups. Restores
the original skill folder into the current library, even when it is empty.
Existing library files are replaced after a safety backup. Extraction is
staged first, so a corrupt archive leaves current files intact.
Agent enablements are unchanged: removed links are not recreated. Use
lazyagents enable <skill> to enable the restored skill again.
```

## adopt

```text
lazyagents adopt <skill> --agent <id>
```

Move an agent's local skill into the library.

```text
options:
  --agent id   the agent whose skills directory holds the local copy (required)

Copies the local folder into the library, backs the original up, and
replaces it with a symlink, so the agent keeps seeing the skill and other
agents can now enable it. Symlinks created by other tools are refused:
manage them with the tool that made them.
```

## migrate-library

```text
lazyagents migrate-library <dir>
```

Move the skills library to another directory.

```text
Copies every library skill to <dir>, repoints the agents' lazyagents
symlinks, saves libraryDir in config.yaml and only then removes the old
copies. Each skill is backed up first. The move is refused when a name
already exists in <dir> or when one directory contains the other; on a
failure the links and config stay as they were.
Example: lazyagents migrate-library ~/.agents/skills
```

## skills

```text
lazyagents skills list|enable|disable|install|remove|backups|restore|adopt|migrate-library|sources|updates|profiles …
```

Manage skills, source scans, reviewed updates and activation profiles.

```text
lazyagents skills <sub> runs the same library command as lazyagents <sub>.
Without <sub> it runs list. Maintenance commands below are available only under
skills. Use lazyagents skills help <sub> for focused help.

sources list|scan <name> [--json]
subcommands:
  list            list configured names, URLs, refs and subdirectories (default; offline)
  scan <name>     clone just this saved source and list repository-relative entries

options:
  --json          stable JSON; scans include source, revision, entries and notes

Scanning uses the saved branch/tag and subdirectory. It never installs,
activates or edits config. Duplicate destination names retain distinct paths.
Use the TUI source picker to install a saved source with exact-path provenance.
Interrupting a scan stops Git and its children and cleans staging.

updates check [directory...]|apply <directory...> [--review token] [--replace-local] [--json]
subcommands:
  check [directory...]   review exact library folders; omitted selection checks all Git skills
  apply <directory...>   apply only these exact reviewed folders

options:
  --json                 check: review/items with stable status strings, origin, diff and hashes
                         apply: targets with id/outcome/detail/state and optional error
  --review token         apply: required identity from check for this exact selection
  --replace-local        apply: allow overwriting edits/missing baseline for exactly one folder

Selectors are canonical folder names from skills list, never display-name aliases.
Unknown/duplicate selections and extra arguments are refused before writes.
Check includes one token for the full scope and one per checked item, so a
script can select a single item from a full check. Apply stages and verifies
again; changed local files, origins or upstream commits require another check.
The same review/diff, backup, exact-path and replacement services serve the TUI.
Bulk updates skip local edits and missing baselines. --replace-local never
bypasses a stale review. Interruptions preserve completed items and their
backups; later items stay unchanged. A failure requires a new check.
Neither command changes activation links or install destination preferences.

profiles list|show <name>|save <name>|preview <name>|apply <name>|remove <name> [--review token] [--json]
subcommands:
  list             list profile names (default; offline)
  show <name>      inspect the saved canonical folder-to-agent specification
  save <name>      save/overwrite the current managed activation matrix
  preview <name>   compare the saved profile with the current matrix and destinations
  apply <name>     apply the reviewed profile to the full matrix
  remove <name>    remove just the saved profile; activation is unchanged

options:
  --json           stable JSON: names, profile specification, preview or operation result
  --review token   apply: required identity from preview; changed state is refused

Save uses the same managed-link snapshot as the TUI; local/foreign and shared
visibility that lazyagents cannot control are excluded. Apply has the same
full-inventory/agent scope as TUI profiles, including disabling managed skills
omitted from the saved profile. Preview lists canonical folders, display names,
agent additions/removals and the saved specification, including unavailable IDs.
Shared-reader constraints still apply; local/foreign copies are preserved.
Legacy profile arrays remain readable. Existing JSON metadata survives writes.
Removing a missing name is idempotent; neither save nor remove touches links.
```

## sessions

```text
lazyagents sessions [--agent id] [--here] [--limit n] [--json]
```

List agent sessions, newest first.

```text
options:
  --agent id   only this agent (an unknown id lists the valid ones)
  --here       only sessions started in the current directory
  --limit n    at most n sessions (default 0 = all)
  --json       JSON output

Text output has AGENT, TITLE, CWD and UPDATED columns; an alias shows as
"alias (title)". --json adds each session's id and, when the agent records it,
token usage (input, output, cache_read, cache_write, model) and, for agents
authenticated with an API key, cost_usd for known models: a subscription does
not pay per token. To resume a session, open the TUI.
```

## provider

```text
lazyagents provider list|validate <profile>|probe <profile>|models <profile>|apply <profile>|clear|adopt <profile>|add <profile>|rm <profile> [--agent id|--agents ids] [--preview] [--json] [--reveal]
```

Manage agent provider profiles (endpoint and token).

```text
subcommands:
  list                 profiles and what each agent has applied (default)
  validate <profile>   check selected destinations without writing or network requests
  probe <profile>      send one credential-free HEAD request to the saved base URL
  models <profile>     fetch an OpenAI-compatible catalog using profile credentials
  apply <profile>      validate all selected targets, then write the profile to the agents' config
  clear                remove what lazyagents applied, keeping the rest of the file
  adopt <profile>      import public settings from --agent into the library only
  add <profile>        create or replace a profile
  rm <profile>         delete a profile from the library (agents keep what was applied)

options:
  --agent id           apply/validate/clear: only this agent (default: every installed agent that supports it)
  --agents ids         apply/validate: explicit comma-separated agent IDs
  --preview            apply/clear: inspect masked field changes without writing
  --json               list/validate/preview/probe/models: JSON output
  --reveal             list: show tokens in plain text

adopt options:
  --agent id           source agent (required); save public fields to a new library name
  --token -            explicitly re-enter a credential via stdin
  --env-key VAR        replace the imported credential reference

add options:
  --base-url url       compatible endpoint (http or https)
  --model m            default model (empty = agent default)
  --token -            read the token from stdin; a literal value also works but lands in shell history
  --env-key VAR        Codex, OpenCode, Pi, Crush: environment variable holding the token
  --wire-api api       Codex: only "responses"; Pi: chat (default), responses or anthropic;
                       Crush: chat (default) or anthropic; OpenCode: chat only; Gemini: gemini only

Profiles live in providers.json with mode 0600. Tokens are masked in text and
JSON output unless --reveal is given. Every apply and clear backs the agent's
file up first. Codex never gets the token: set --env-key and export that variable.
Gemini, OpenCode, Pi and Crush need --base-url and --model. Validation checks configuration,
not authentication; a variable name does not prove a working credential.
Adopt copies public connection fields only. Re-enter native tokens with --token -
or supply --env-key; native account login and advanced options are not imported.
Apply/clear previews hide all values and never serialize private review revisions.
Any incompatible selected target blocks the entire apply before the first write.
Use --agent or --agents to explicitly select a compatible subset.
Probe/models are on-demand only, with a 10-second timeout and no redirects.
Models uses a saved bearer token before envKey; native login is never read.
A response proves reachability; a listed model does not prove inference works.
List compares public fields using native defaults and credential references/presence.
Multiple matching profiles stay ambiguous; token values are never compared.
List --json adds matches and per-profile comparisons; profile requires a unique match.
List and doctor include value-free environment diagnostics with variable names,
origins and next steps. Only the current lazyagents process and supported native
fields are inspected; running sessions, shell startup files and helpers are not.
Gemini uses GenerateContent, a saved API key or --env-key GEMINI_API_KEY, and
a managed home dotenv block plus security.auth.selectedType. Clear restores the
previous auth selection and removes only its block. Shell variables/nearer .env
files can override this global snapshot; restart Gemini in a trusted workspace.
Vertex AI, external auth, keyless gateways and OpenAI protocols are unsupported.
OpenCode manages one strict-JSON global config; JSONC comments/trailing commas and
multiple global files are refused. Project, inline and managed settings may override it.
Start a new agent session after applying; existing sessions may keep old settings.
```

## hooks

```text
lazyagents hooks list|enable <name>|disable <name>|add <name>|rm <name> [--agent id] [--json]
```

List library hooks and install/uninstall them in agents.

```text
subcommands:
  list                 library hooks and where they are installed (default)
  enable <name>        install the hook in the agents that fire its events
  disable <name>       uninstall the hook
  add <name>           create a library hook with one command
  rm <name>            delete a library hook (uninstall imported scripts first)

options:
  --agent id           enable/disable: only this agent (default: every installed agent that supports hooks)
  --json               list: JSON output

add options:
  --event name         event that fires the command: SessionStart, PreToolUse, Stop…
  --command text       shell command to run
  --matcher re         event filter, e.g. a tool name for PreToolUse (empty = all)
  --timeout n          seconds (0 = agent default)
  --desc text          description shown in the tab

A hook runs a command on every event, so every install backs the agent's file up
first, and hooks lazyagents did not install are never touched. Codex only runs a
new hook after you trust it in Codex itself; lazyagents never writes that trust.
To import a plugin's hooks from a repository: lazyagents install <source> --hooks.
Removing an inline library hook keeps its installed commands. Imported scripts
require uninstalling the hook from every agent first; unreadable configs block
deletion. Scripts shared with another library entry are kept.
```

## config-backups

```text
lazyagents config-backups list|preview <id>|restore <id> [--query text] [--review token] [--json]
```

Inspect and recover agent configuration snapshots.

```text
subcommands:
  list                 list config snapshots (default), including unknown legacy origins
  preview <id>         verify origin/integrity and show a redacted before/after preview
  restore <id>         restore this file using the --review token from its preview

options:
  --query text         list: match agent, destination/file, operation and local date
  --json               list/preview: JSON output; config values are never included
  --review token       restore: required token from the exact reviewed preview

Use an exact ID from list. New snapshots carry adapter-provided provenance;
legacy snapshots without it are unknown and cannot be restored automatically.
The preview lists changed fields/lines with all values hidden. Restore replaces
the whole file, including unrelated edits listed in the preview. Current files
receive a safety backup; changed source, destination or permissions require a
new preview. Existing stricter permissions stay restricted.
Restore is per file: profiles, provider ownership records and other files are
not rolled back. Pi's two files need separate reviews. Codex hook trust/feature
approvals remain under native control and can prevent a config.toml restore.
Skill archives use skills backups/restore instead.
```

## history

```text
lazyagents history list|inspect <id>|clear [--query text] [--json] [--yes]
```

Inspect recent local write attempts without replaying them.

```text
subcommands:
  list                 list recent attempts, newest first (default)
  inspect <id>         inspect target outcomes and verify related config backups
  clear --yes          clear history only; keep all backups and current settings

options:
  --query text         list: match module, action, resource, target, outcome or local date
  --json               list/inspect: metadata only; no errors, bodies or commands
  --yes                clear: confirm removing local history

Keep at most 200 attempts, 1000 targets and 32 related backups per attempt,
within 32 MiB. The owner-only history file is separate from backup retention.
Concurrent history updates are serialized; persistence failures are reported.
History is not an executable retry plan. Inspect current state and review the
operation again in its module. A config backup requires a fresh config-backups
preview and its review token; unavailable/rotated backups cannot be restored.
Related snapshots match the attempt time, action and target; overlapping
attempts may reference the same snapshot. No operation is replayed here.
```

## usage

```text
lazyagents usage [limits|daily|agents|projects|models] [--agent id] [--since 7d] [--limit n] [--refresh] [--json]
```

Agent consumption: subscription limits, tokens and cost per day, agent, project and model.

```text
views:
  (none)     dashboard: subscription limits, current 5h block and period summary
  limits     limit windows (session, week) with usage and reset time
  daily      tokens per day
  agents     tokens per agent
  projects   tokens per project (session directory)
  models     tokens per model

options:
  --agent id       only this agent
  --since period   start of the period: 7d, 24h or 2026-09-01 (default 7d)
  --limit n        agents/projects/models rows in text output (default 10; 0 = all)
  --refresh        ignore the limits cache (5 min) and fetch again
  --json           JSON output (not cut by --limit)

Tokens come from local transcripts; limits, from each agent's account.
USD cost only shows for agents authenticated with an API key: a subscription
does not pay per token.
```

## doctor

```text
lazyagents doctor [--json] [--local]
```

Diagnose agents, skills, hooks, providers, usage and plugins.

```text
sections:
  detected agents  each agent id and whether it is installed
  configuration    current lazyagents config syntax
  skills           library and agent skills with an invalid or incomplete SKILL.md
  symlinks         broken symlinks in any skill dir an agent reads
  providers        the provider applied to each agent; one applied to an
                   agent that is not installed is a problem
  hooks            invalid library files, hook commands missing from PATH or
                   not executable, and each agent's hooks note
  usage            authentication and subscription limits per agent (from the
                   cache when fresh, otherwise fetched; external)
  plugins          skipped plugin files, each plugin's handshake and, when its
                   manifest asks for it, its own doctor (external)

options:
  --json   JSON output: {ok, agents, sections[{title, report, problems,
           findings, external, skipped, next_step, tab}], scope, cancelled?}
  --local  skip usage/network and plugin execution; label their sections not run

Findings include severity, resource, explanation and next_step. Diagnostics
never repair configuration or activation links. Usage can refresh its cache.
Exits with 1 on a problem or cancellation, 0 when completed checks are OK.
Skipped external checks do not certify remote availability.
```
