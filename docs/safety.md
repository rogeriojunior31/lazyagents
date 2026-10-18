# Safety and data

lazyagents edits files that your agents depend on. This page says exactly what it touches, how to undo each change, and how secrets are handled. The module guides go deeper on each subject.

## Principles

- **Nothing changes without an action from you.** Browsing, opening the TUI or running a query command never writes to an agent's files.
- **Config writes are confirmed.** In the TUI, applying a provider or installing a hook shows the file and what changes before writing. Enabling a skill only adds or removes a symlink, so it happens at once.
- **Backup before overwrite.** An agent config file is copied to the backups dir before each write; removed skills and deleted sessions are archived first.
- **Only what lazyagents manages is touched.** Keys, hooks and TOML sections it did not write are kept as they were, in the same order. In Claude Code's `env` block, applying or clearing a provider removes only the keys lazyagents wrote, and clearing puts back an endpoint or model a profile had replaced.
- **Atomic writes.** Files are written to a temp file and renamed over the original, so a crash never leaves half a file.
- **Concurrent writes are never overwritten.** An agent config is replaced only if it still holds what lazyagents read. If the agent itself, or another lazyagents, wrote it in between, Codex's `config.toml` and Pi's pair of files (`models.json`, `settings.json`) are read and edited again (up to three times, with one backup); Claude Code's `settings.json` and the hooks files are left as the other program wrote them, and lazyagents says so. Removing a file lazyagents created follows the same rule.

## What lazyagents writes

In its own dirs ([paths](configuration.md#where-files-live)): the skill and hook libraries, profiles, aliases, caches, exports and backups.

In the agents' dirs, only on your action:

| Action | What changes | Undo |
|---|---|---|
| Enable a skill | A symlink in the agent's skills dir, pointing to the library | Disable it: the symlink is removed |
| Disable a skill | That symlink is removed. A real folder (a local skill, or another tool's content) is never deleted | Enable it again |
| Adopt a skill | The local folder moves into the library and a symlink takes its place | Copy the folder back from the library |
| Apply a provider | Claude Code: the `env` block of `~/.claude/settings.json`. Codex: `model_provider` and a delimited `[model_providers.lazyagents]` block in `~/.codex/config.toml`. Pi: the `lazyagents` provider in `~/.pi/agent/models.json` and `defaultProvider`/`defaultModel` in `~/.pi/agent/settings.json`. Crush: a delimited block at the end of `~/.config/crush/crushrc`, every value single-quoted | Clear the provider, or restore the backup |
| Install a hook | One entry in the agent's hooks file (`~/.claude/settings.json`, `~/.codex/hooks.json`), or a `hook add` line in a delimited block of Crush's `crushrc` | Uninstall it, or restore the backup |
| Delete a session | The transcript file is removed after a copy goes to the backups dir. OpenCode sessions are exported with `opencode export` first, then deleted by `opencode session delete`, and Crush sessions saved with `crush session show --json`, then deleted by `crush session delete`; nothing is deleted if the backup fails | Copy the backup back; for OpenCode, `opencode import <file>`; a Crush backup is a record only (Crush has no import) |
| `migrate-library` | Skills move to the new dir, symlinks are recreated, `libraryDir` is saved in `config.yaml` | Run `migrate-library` back to the old dir |

The exact files per agent are in the [agent support reference](reference/agents.md#capabilities).

Provider apply validates every selected agent before the first write. Invalid endpoint/credential references, incompatible protocol/model requirements, unreadable native files, invalid managed blocks or unsupported JSON shapes block the whole selection without creating backups or ownership records. Choose a compatible subset explicitly in the Providers selector or with `provider apply --agents id,id`. `provider validate` and the form's draft check are read-only and make no network requests; a named environment variable does not prove authentication. Each write still rereads the native file, so later I/O/concurrency failures may leave partial changes reported per target. See the [provider guide](guide/providers.md).

Connection templates only prefill editable library inputs. They do not fetch credentials, authenticate, discover models or apply agent settings. The agent-defaults entry explicitly selects agents and confirms the existing clear operation: only managed provider overrides are removed, previous/external providers may remain, and native login stays in the agent. See [connection templates](guide/providers.md#connection-templates).

Provider adoption reads only representable public connection fields and saves a new library entry separately from native writes. Native tokens require explicit re-entry or a supported environment reference; login files and credential helpers are not copied. Executable Crush configuration is never evaluated. Apply/clear previews show file actions and managed field names with every value hidden, using the same edits as persistence. Previewing creates no backups or ownership records. Confirmed operations recheck the saved profile, native files and ownership state against the review; changed reviews block all selected writes. Existing conditional writes, backups and per-target runtime failure reporting still apply. See [import and previews](guide/providers.md#import-an-existing-connection).

Provider drift checks compare public native fields, credential references and token presence only. Refresh never reads variable values, authenticates, writes native settings or chooses among ambiguous profile matches. Keep live settings by leaving them unchanged; explicit reapplication uses the same reviewed, backed-up write as apply. Matching does not compare token values or prove profile identity. See [saved/live comparisons](guide/providers.md#compare-saved-and-live-settings).

## Backups

Everything goes to `~/.local/share/lazyagents/backups/`, named `<file>.<timestamp>`:

| What | Kept |
|---|---|
| Agent config files (`settings.json`, `config.toml`, `hooks.json`, `models.json`) | the newest 20 per file |
| Skills removed or updated (`<skill>.<timestamp>.tar.gz`) | the newest 20 per skill |
| Deleted sessions (`<transcript file>.<timestamp>`, `opencode-<id>.<timestamp>.json`, `crush-<id>.<timestamp>.json`) | all |

Skill backups are available even with an empty library: `B` or `: skills backups` in the TUI, or `lazyagents skills backups` in the CLI. Filter by original name/date, then restore a selected archive with `enter` or `lazyagents skills restore <backup>` ([skills guide](guide/skills.md#backups-and-restore)). Current library files receive a safety backup before replacement; corrupt archives leave them intact. Restoring files preserves existing agent links and does not recreate removed enablements. Configuration snapshots are recovered in **Recovery** or through `config-backups`: inspect the redacted current-to-backup differences and explicitly confirm replacing the whole file. Current data is backed up first, stale reviews are refused and stricter permissions are preserved. Legacy snapshots without verified provenance remain unknown and cannot be restored automatically. See the [recovery guide](guide/recovery.md) for supported files and native approval limits.

```sh
lazyagents config-backups list --query 'claude-code settings.json'
```

## Secrets

- **Provider tokens** live in `~/.config/lazyagents/providers.json` with mode 0600, and backups keep the same mode. The TUI and `--json` show only `token ✓`; the plain value prints only with `lazyagents provider list --reveal`. Pass tokens with `--token -` to read them from stdin and keep them out of your shell history.
- For **Codex**, prefer `--env-key VAR`: `config.toml` then names the environment variable and never holds the token.
- For **Claude Code**, the token has to live in `settings.json`, as Claude Code reads it there. lazyagents makes that file 0600 when it writes a token.
- For **Pi**, `--env-key VAR` also keeps the token out of the file (`"apiKey": "$VAR"`); a token otherwise goes into `models.json`, which lazyagents makes 0600.
- **Agent credentials** (for example Claude Code's login) are read only to make the one call that needs them, the Claude Code limits request, and never stored, logged or shown. To tell the auth mode, lazyagents reads only whether a credential exists and its type (Pi's `auth.json`: `oauth` or `api_key`; whether Pi's `models.json` or Crush's `crush.json` has an API key), never its value.

## Diagnostics

The optional `settings` palette panel edits only the saved theme and `tui.startTab`/`tabs`/`hidden`. It keeps the current session unchanged until restart and never writes native agent settings or starts a plugin. Saving rereads config, preserves foreign fields/comments, refuses stale edited preferences and guards the final write against concurrent file changes. Errors do not quote parser input, and the panel receives no foreign configuration bodies or credentials. Escape discards an unsaved draft; an in-flight local save completes before closing. First-use guidance opens existing import/source/provider flows; it performs no automatic download, activation or onboarding writes.

Agent detection refresh (`r` in Agents or `agents refresh`) runs local version probes and refreshes local skill/session/provider/hook evidence. It does not request usage limits, execute plugin checks or import a different shell's environment. Existing usage-limit evidence stays intact while local usage aggregates update. Failed version probes retain prior detection for that agent with an explicit warning; other agents still update. Probes reuse bounded output, a six-second deadline and owned child-process cleanup; command errors are not quoted in diagnostics. Cache snapshots are copied and synchronized, and late results from older revisions are ignored. No automatic refresh timer or native configuration write is added.

Provider environment diagnostics inspect only supported native fields and the current lazyagents process. Reports carry variable names, origins and next steps, never compared endpoint/model values or credentials. Credential variables are checked only for presence/nonempty content; their contents are never compared or serialized. Pi stored-auth precedence uses type metadata only. No shell startup files, credential helpers or Crush scripts are executed, no variables are removed and no native settings are repaired. Hosted Claude routing and unverified agent/authentication modes are excluded; project/session/managed overrides may change effective behavior. Missing variables in this process do not prove failure in a separately launched agent. See [coverage and precedence](guide/providers.md#environment-diagnostics).

The Agents doctor report runs local checks only when opened with `d` or
`agents doctor`, and retries locally with `r`. It reports configuration syntax,
skill metadata, broken links, provider state and hook command availability
without repairing files or changing links. `n` explicitly includes usage and
plugin checks; `doctor --local` provides the same restricted scope in the CLI,
while full `doctor` retains its existing behavior. Skipped checks are labeled
not run. Usage can refresh its existing cache, and cached evidence does not
certify remote availability. Plugins execute with the user's permissions.
Closing the report cancels the request, including usage calls and plugin
handshake/doctor processes. Reports are session-only and no new configuration
or report file is written. Malformed lazyagents configuration is identified by
path without quoting parser input; rendered report data escapes terminal
controls. Native consent and credentials remain owned by the adapters.

## Network

lazyagents works offline. It goes to the network only when you ask for something that needs it:

| When | What |
|---|---|
| You open the Usage tab, run `lazyagents usage` / full `lazyagents doctor`, or explicitly include external checks with `n` in the Agents doctor report | Claude Code limits, from the same endpoint as its `/usage` command, cached for 5 minutes. Codex limits come from its local files |
| You request `provider probe` / `c` / `ctrl+t` in Providers | One credential-free HEAD request to the profile's base URL; any HTTP response proves only reachability |
| You request `provider models` / `m` / `ctrl+l` in Providers | One GET to its OpenAI-compatible `<base>/models`, using the saved/typed bearer token before the named environment reference, or no credential for keyless endpoints |
| You install from GitHub | `git clone --depth 1` of that repository |
| You search GitHub (`S`) | `gh api` with your `gh` login |
| You check skill updates | the skill's source repository |

Provider requests have a 10-second deadline and can be cancelled (`esc` in the TUI, Ctrl+C in the CLI). Redirects are never followed, even within the same origin. Catalog bodies are bounded to 1 MiB after decompression, headers to 64 KiB and results to 10,000 single-page IDs. Credentials remain local to request construction; native login and helpers are never read, errors do not expose raw bodies/URLs, and unsafe returned IDs are refused. A catalog returned with credentials does not independently validate a token on a public server, and a listed model does not prove inference works. Checks write no configuration or persistent cache. See [endpoint checks](guide/providers.md#endpoint-checks-and-model-discovery).

Nothing runs at startup, and there is no telemetry.

## Consent belongs to the agent

Some agents ask you to approve a command before running it. lazyagents never answers for you. Codex, for example, records your approval of a hook as a `trusted_hash` in `config.toml` and needs `[features] hooks` on: lazyagents installs the hook and warns that it is pending, but never writes the hash or turns the feature on. You approve it in Codex.

## Third-party content

- **Hooks run commands.** A hook from a repository runs its command on every matching event, with your permissions. Hooks found while installing skills come unchecked; read them first (`v` in the Hooks tab opens the command and its scripts).
- **Skills are instructions** your agents will follow. Read a `SKILL.md` (`enter`) before enabling a skill from a source you do not know.
- **Plugins are programs** you install yourself. lazyagents treats everything a plugin sends as untrusted, strips terminal control sequences from it and keeps a failing plugin from taking the TUI down, but the plugin itself runs with your permissions. See the [plugins guide](guide/plugins.md).

## Reporting a vulnerability

See [SECURITY.md](../SECURITY.md).

## Partial operations

Bulk provider, hook and skill activation/profile operations are per target, with no automatic rollback across agents. Results name successful, failed and skipped agents and include state read after the attempt; a failed target may still contain partial changes. The TUI retains its report until dismissed and confirms retries of failed IDs only. Matrices reload actual state after failure or retry. CLI details go to stderr, exit codes and list JSON remain compatible; retry with an explicit `--agent` to avoid repeating successful targets. Recover configuration files through [Recovery](guide/recovery.md) and library archives through [Skills](guide/skills.md#backups-and-restore); neither reruns an operation.

Selected-row skill actions freeze the reviewed folders and agent destinations before confirmation. Hidden/removed rows are unmarked, and selecting visible rows includes matching rows off screen. Activation results and retries identify exact skill/agent pairs; update/remove results identify skill/library entries because every reader uses that copy. Local/foreign entries are skipped. Bulk updates preserve local edits and missing hash baselines and refuse changed recorded sources. Removal keeps the library copy when managed-link cleanup fails, so retry can finish cleanup; successful removals still have an archive. See [selected skill actions](guide/skills.md#act-on-selected-rows).

TUI skill installs review selected source entries and explicit agent destinations before copying. The initial default is library only; later installs preselect the last confirmed available IDs from `skills.installTargets`, never an implicit all-agent scope. Only newly installed copies are activated; collisions leave existing copies and their activation unchanged. Failed activation keeps library files and retries links only. Copy/import failures are not retried through the activation report. Imported hooks remain a separate opt-in and are never enabled by skill destinations. A failed preference save is reported separately and does not undo installed files or overwrite malformed config.

Saved skill repository descriptors are config only. Adding/editing/removing them never fetches content or changes installed origins. Explicit scans validate branch/tag arguments, clone once with hooks disabled and enforce the chosen subfolder's physical containment. Repeated skill names remain distinct by source path and colliding destination selections are refused. Installed named-source updates retain URL/ref/exact path and never look up an edited descriptor or fall back to another path when the original disappears. The observed commit is provenance, not signature verification or an immutable upstream pin. No source scans run at startup, and scan status/errors remain session-only. Source discovery skips linked hook manifests before reading commands.

Skill update review retains one staged clone per installed URL/ref until dismissal/application. Local and upstream hashes are compared with the saved install baseline; bulk never replaces locally edited or baseline-less items. A single-item replacement requires its own visible diff and explicit confirmation. The local hash/origin and staged content are checked again before backup and folder swap; origin metadata is prepared in staging too. Cancellation cleans staging and stops Git/search process trees. An atomic swap already started finishes; previously completed items remain applied with backups. Update failures/cancellation require fresh review rather than automatic write retries. Diff output is bounded and escapes terminal controls; omitted binary/large content is labeled. Content fingerprints frame individual files to detect binary file-layout changes after review. Existing install baselines remain readable and are refreshed on successful install/update. Legacy origin files and named-source ref/revision/exact-path behavior remain supported.

TUI skill profile application verifies the same profile/inventory/destination fingerprint as the CLI, shows canonical folders in its confirmation and blocks input until the result is available. A changed preview requires a new review.

CLI skill maintenance reuses those services under `skills sources`, `skills updates` and `skills profiles`. Source listing/scanning never installs or edits descriptors. Update apply requires canonical folders and the matching check identity; profile apply requires its preview identity for the full managed matrix. Invalid arguments/selections and stale reviews are refused before writes. Review identities are content/scope fingerprints, not authorization credentials. Scans/checks/updates handle interrupts with process-tree and staging cleanup. New JSON results preserve stdout for structured output and use stderr for diagnostics; exit status must be checked, especially after a partial operation. Existing library command aliases and JSON remain compatible. No new config keys, persisted review cache or automatic remote activity is introduced.
