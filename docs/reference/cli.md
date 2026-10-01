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
| [`adopt`](#adopt) | move an agent's local skill into the library |
| [`migrate-library`](#migrate-library) | move the skills library to another directory |
| [`skills`](#skills) | the skill commands above, grouped (skills list = list) |
| [`sessions`](#sessions) | list agent sessions, newest first |
| [`provider`](#provider) | manage agent provider profiles (endpoint and token) |
| [`hooks`](#hooks) | list library hooks and install/uninstall them in agents |
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
and the Skills tab restores it (b). Local skills are not in the library and
cannot be removed here.
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
lazyagents skills list|enable|disable|install|remove|adopt|migrate-library …
```

The skill commands above, grouped (skills list = list).

```text
lazyagents skills <sub> runs the same command as lazyagents <sub>, with
the same options; without <sub> it runs list. It exists so skills reads
like lazyagents hooks and lazyagents provider.
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
lazyagents provider list|apply <profile>|clear|add <profile>|rm <profile> [--agent id] [--json] [--reveal]
```

Manage agent provider profiles (endpoint and token).

```text
subcommands:
  list                 profiles and what each agent has applied (default)
  apply <profile>      write the profile to the agents' config
  clear                remove what lazyagents applied, keeping the rest of the file
  add <profile>        create or replace a profile
  rm <profile>         delete a profile from the library (agents keep what was applied)

options:
  --agent id           apply/clear: only this agent (default: every installed agent that supports it)
  --json               list: JSON output
  --reveal             list: show tokens in plain text

add options:
  --base-url url       compatible endpoint (http or https)
  --model m            default model (empty = agent default)
  --token -            read the token from stdin; a literal value also works but lands in shell history
  --env-key VAR        Codex, Pi, Crush: environment variable holding the token
  --wire-api api       Codex: only "responses"; Pi: chat (default), responses or anthropic;
                       Crush: chat (default) or anthropic

Profiles live in providers.json with mode 0600. Tokens are masked in text and
JSON output unless --reveal is given. Every apply and clear backs the agent's
file up first. Codex never gets the token: set --env-key and export that variable.
Pi and Crush need --base-url and --model.
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
  rm <name>            delete a hook from the library (agents where it is installed keep it)

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
lazyagents doctor [--json]
```

Diagnose agents, skills, hooks, providers, usage and plugins.

```text
sections:
  detected agents  each agent id and whether it is installed
  skills           library and agent skills with an invalid or incomplete SKILL.md
  symlinks         broken symlinks in any skill dir an agent reads
  providers        the provider applied to each agent; one applied to an
                   agent that is not installed is a problem
  hooks            invalid library files, hook commands missing from PATH or
                   not executable, and each agent's hooks note
  usage            authentication and subscription limits per agent (from the
                   cache when fresh, otherwise fetched)
  plugins          skipped plugin files, each plugin's handshake and, when its
                   manifest asks for it, its own doctor

options:
  --json   JSON output: {ok, agents, sections[{title, report, problems}]}

Exits with 1 when any section reports a problem, 0 when all is OK.
```
