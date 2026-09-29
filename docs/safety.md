# Safety and data

lazyagents edits files that your agents depend on. This page says exactly what it touches, how to undo each change, and how secrets are handled. The module guides go deeper on each subject.

## Principles

- **Nothing changes without an action from you.** Browsing, opening the TUI or running a query command never writes to an agent's files.
- **Config writes are confirmed.** In the TUI, applying a provider or installing a hook shows the file and what changes before writing. Enabling a skill only adds or removes a symlink, so it happens at once.
- **Backup before overwrite.** An agent config file is copied to the backups dir before each write; removed skills and deleted sessions are archived first.
- **Only what lazyagents manages is touched.** Keys, hooks and TOML sections it did not write are kept as they were, in the same order. In Claude Code's `env` block, applying or clearing a provider removes only the keys lazyagents wrote, and clearing puts back an endpoint or model a profile had replaced.
- **Atomic writes.** Files are written to a temp file and renamed over the original, so a crash never leaves half a file.

## What lazyagents writes

In its own dirs ([paths](configuration.md#where-files-live)): the skill and hook libraries, profiles, aliases, caches, exports and backups.

In the agents' dirs, only on your action:

| Action | What changes | Undo |
|---|---|---|
| Enable a skill | A symlink in the agent's skills dir, pointing to the library | Disable it: the symlink is removed |
| Disable a skill | That symlink is removed. A real folder (a local skill, or another tool's content) is never deleted | Enable it again |
| Adopt a skill | The local folder moves into the library and a symlink takes its place | Copy the folder back from the library |
| Apply a provider | Claude Code: the `env` block of `~/.claude/settings.json`. Codex: `model_provider` and a delimited `[model_providers.lazyagents]` block in `~/.codex/config.toml`. Pi: the `lazyagents` provider in `~/.pi/agent/models.json` and `defaultProvider`/`defaultModel` in `~/.pi/agent/settings.json` | Clear the provider, or restore the backup |
| Install a hook | One entry in the agent's hooks file (`~/.claude/settings.json`, `~/.codex/hooks.json`) | Uninstall it, or restore the backup |
| Delete a session | The transcript file is removed after a copy goes to the backups dir. OpenCode sessions are exported with `opencode export` first, then deleted by `opencode session delete`; nothing is deleted if the export fails | Copy the backup back; for OpenCode, `opencode import <file>` |
| `migrate-library` | Skills move to the new dir, symlinks are recreated, `libraryDir` is saved in `config.yaml` | Run `migrate-library` back to the old dir |

The exact files per agent are in the [agent support reference](reference/agents.md#capabilities).

## Backups

Everything goes to `~/.local/share/lazyagents/backups/`, named `<file>.<timestamp>`:

| What | Kept |
|---|---|
| Agent config files (`settings.json`, `config.toml`, `hooks.json`, `models.json`) | the newest 20 per file |
| Skills removed or updated (`<skill>.<timestamp>.tar.gz`) | the newest 20 per skill |
| Deleted sessions (`<transcript file>.<timestamp>`, `opencode-<id>.<timestamp>.json`) | all |

Skill backups are restored from the TUI: `b` in the Skills tab lists them ([skills guide](guide/skills.md)). A config file backup is a plain copy: to restore, copy it over the live file.

```sh
ls -t ~/.local/share/lazyagents/backups/settings.json.* | head -1   # newest settings.json backup
```

## Secrets

- **Provider tokens** live in `~/.config/lazyagents/providers.json` with mode 0600, and backups keep the same mode. The TUI and `--json` show only `token ✓`; the plain value prints only with `lazyagents provider list --reveal`. Pass tokens with `--token -` to read them from stdin and keep them out of your shell history.
- For **Codex**, prefer `--env-key VAR`: `config.toml` then names the environment variable and never holds the token.
- For **Claude Code**, the token has to live in `settings.json`, as Claude Code reads it there. lazyagents makes that file 0600 when it writes a token.
- For **Pi**, `--env-key VAR` also keeps the token out of the file (`"apiKey": "$VAR"`); a token otherwise goes into `models.json`, which lazyagents makes 0600.
- **Agent credentials** (for example Claude Code's login) are read only to make the one call that needs them, the Claude Code limits request, and never stored, logged or shown. To tell the auth mode, lazyagents reads only whether a credential exists and its type (Pi's `auth.json`: `oauth` or `api_key`; whether Pi's `models.json` has an `apiKey`), never its value.

## Network

lazyagents works offline. It goes to the network only when you ask for something that needs it:

| When | What |
|---|---|
| You open the Usage tab, run `lazyagents usage` or `lazyagents doctor` | Claude Code limits, from the same endpoint as its `/usage` command, cached for 5 minutes. Codex limits come from its local files |
| You install from GitHub | `git clone --depth 1` of that repository |
| You search GitHub (`S`) | `gh api` with your `gh` login |
| You check skill updates | the skill's source repository |

Nothing runs at startup, and there is no telemetry.

## Consent belongs to the agent

Some agents ask you to approve a command before running it. lazyagents never answers for you. Codex, for example, records your approval of a hook as a `trusted_hash` in `config.toml` and needs `[features] hooks` on: lazyagents installs the hook and warns that it is pending, but never writes the hash or turns the feature on. You approve it in Codex.

## Third-party content

- **Hooks run commands.** A hook from a repository runs its command on every matching event, with your permissions. Hooks found while installing skills come unchecked; read them first (`v` in the Hooks tab opens the command and its scripts).
- **Skills are instructions** your agents will follow. Read a `SKILL.md` (`enter`) before enabling a skill from a source you do not know.
- **Plugins are programs** you install yourself. lazyagents treats everything a plugin sends as untrusted, strips terminal control sequences from it and keeps a failing plugin from taking the TUI down, but the plugin itself runs with your permissions. See the [plugins guide](guide/plugins.md).

## Reporting a vulnerability

See [SECURITY.md](../SECURITY.md).
