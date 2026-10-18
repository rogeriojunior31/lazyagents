# Configuration recovery

Recovery finds and restores snapshots of agent configuration files. [Skill archives](skills.md#backups-and-restore) have their own inventory in Skills.

## Find a change

Open **Recovery** or choose `: recovery`. `/` filters by agent ID, file, operation or local date (`YYYY-MM-DD`); all query words must match. `enter` applies the filter, then opens the selected preview. `esc` while editing clears the filter; `r` reloads. Returning to this tab also reloads the inventory, so snapshots created by another operation appear without restarting.

The opening view shows **Ready to preview**, with separate totals for previewable and **Manual only** copies. `l` switches to **All backups** and back. Rows show readable dates (including milliseconds), filenames, agent names and actions such as **Apply provider**. The backup filename, backup folder and restore destination belong to the selected copy's details, beside the list or below it in a narrow terminal. Filename and folder are shown separately so a long folder cannot split the backup identity. Click or use arrows to select; `shift+↑/↓` or `ctrl+u/d` scrolls the details.

Operations `provider.apply`, `provider.clear`, `hook.install` and `hook.uninstall` identify the attempted write the snapshot preceded. They describe **before an attempt**, including one that later failed. A file that did not exist has no previous file to archive.

New snapshots record agent, exact destination, operation, date, permissions and checksum. Filenames stay unchanged. Provenance lives in `backups/.config-backup-meta/<backup>.json` (`0600`, folder created with `0700`), without configuration values. Metadata is removed when its snapshot rotates out; retention remains 20 snapshots per filename prefix.

Copies without a verified origin are kept under **All backups**, marked **Manual**. If none are available for preview, the opening view explains why and offers `l` to inspect them. Selecting a manual-only copy explains the limitation and shows its saved file path; `enter` opens those details without offering a restore. A basename such as `settings.json` is used by several agents and does not prove its destination. No path is guessed or assigned to old backups. The CLI retains `unknown` agent/operation fields for compatibility.

## Review and restore

The preview names the destination and permissions and compares **current file → archived file**, including unrelated edits made since the snapshot. JSON changes identify field paths; TOML and Crush changes identify line positions and recognized assignment keys. All values, commands and comments are hidden, including unknown fields. A hidden-to-hidden change still indicates different underlying values.

Read the full preview with the scrolling keys, then press `enter` to confirm replacing the **whole file**, including unrelated differences shown there. `esc` returns without writing. The current file receives a safety snapshot with operation `config.restore`. Restoring keeps the stricter current/archived permissions.

The source checksum, metadata, destination contents, permissions and resolved parent directory are rechecked before writing. Changes since review require a fresh preview. Symlinked snapshot/destination files, damaged snapshots and paths not approved by the current adapter are refused. Changing a config-directory override can therefore make an old snapshot ineligible.

## Supported files and limits

| Agent | Adapter-approved files |
|---|---|
| Claude Code | `settings.json` (providers and hooks) |
| Codex | provider `config.toml`; `hooks.json` |
| Pi | `models.json` and `settings.json`, reviewed separately |
| Crush | global `crushrc` (providers and hooks) |

Paths follow [agent config overrides](../reference/agents.md#config-overrides). Checks cover integrity and the structure understood by existing adapters; configurations are never executed, and not every native setting is validated. A corrupt current JSON file can be replaced with a valid archived object. TOML uses the existing structural scanner and Crush uses managed-block checks.

Codex refuses changed `config.toml` files containing hook-related text, trust hashes or Unicode escapes that could conceal approval keys. This conservative check leaves native [hook trust](hooks.md#codex-hooks-switch-and-trust) under Codex's control. Such files need native/manual review.

Recovery is **per file**. Saved profiles, provider ownership records, libraries and other files keep their current state. Pi requires separate previews/restores for its two files. Providers and Hooks refresh their displayed state after recovery. A restored hook may reference a separately removed script: check the library and scripts before using it.

## From the CLI

```sh
lazyagents config-backups list
lazyagents config-backups list --query 'claude-code settings.json provider.clear'
lazyagents config-backups list --query '2026-10-17' --json
lazyagents config-backups preview settings.json.20261017T120000.000000000
```

Use an exact ID from the list. The preview prints the restore command and `--review` token; run it after reviewing the differences. `preview <id> --json` provides the same redacted changes and token for scripts. Restore requires that token and refuses a stale review. See the [CLI reference](../reference/cli.md#config-backups).

## After a partial operation

Provider/hook bulk reports link here through `b`. Their outcomes describe the attempt; a failure may include changes within that agent, and successful agents remain changed. Fix the cause and retry failed targets in the report, or inspect a snapshot before restoring a whole file. A restore does not retry the original operation. Skill activation uses symlinks: adjust visibility in Skills and use its separate archive inventory for library content.

## Recent operation history

Press `h` in Recovery or choose `: history` to inspect completed write attempts after restarting. Rows show module/action, local time and counts of succeeded, failed and skipped targets. `enter` opens retained target IDs and outcomes. The history includes provider apply/clear, hook enable/disable/command toggles, skill install/activation/profiles/batch/update/adopt/remove/restore, and configuration restores. It is not a journal of session content, local profile/form edits, preferences, plugin actions or operations performed directly in an agent.

In details, `r` navigates to the original module to inspect current state and review a new attempt. It never repeats the saved operation or selects every original target. Fix the cause and explicitly choose failed destinations; a successful target may have changed again since this record. Existing live operation reports retain their failed-target retry service; after restart an update always needs a fresh source review. If the module is hidden, enable its tab through `settings` or use its CLI.

Related snapshots are matched by time, action and target, with config provenance and checksum checked when recording. Overlapping attempts may reference the same snapshot; these are recovery candidates, not a transaction log. Use `[`/`]` to choose a reference and `b` to revalidate its origin, integrity and current destination through the existing redacted preview. Restore still needs explicit confirmation. A missing, rotated, damaged or no-longer-approved config backup cannot be restored from history. Skill references lead to the separate Skills archive inventory (`B`); inspect the exact filename there before restoring. Links/new files can have no prior snapshot.

History lives in `operation-history.json` in the data folder (`0600`), keeping the newest **200 attempts**, at most **1000 targets** and **32 related snapshots** per attempt, within **32 MiB**. Older entries rotate automatically; excess targets are counted explicitly. This retention is independent of the backup inventory. Only time, IDs, module/action, resource name, outcome and backup references are saved: no raw errors, result details/state, tokens, config bodies, source URLs, transcripts or imported command payloads. The file records completed service results; a process killed before result persistence may leave no entry. Always inspect actual files after an interruption.

`c` asks before clearing history. This keeps every backup, current setting and library file. Writes and clear use an exclusive local lock plus an atomic guarded replacement, so concurrent lazyagents processes cannot silently overwrite records. Persistence problems are reported separately from completed target outcomes; do not repeat successful targets to repair history. A crash during the short metadata update can leave `operation-history.json.lock`: remove that empty folder only after confirming no lazyagents process is writing. A malformed regular history file can be explicitly cleared; symlink/special/oversized files are refused and need local inspection.

```sh
lazyagents history list --query 'providers codex failed' --json
lazyagents history inspect <id> --json
lazyagents history clear --yes
```

`inspect` verifies config backup availability and prints review entry points; no history command executes retry or restore. See the [CLI reference](../reference/cli.md#history).

## Refresh state and selection

Loading, an initial inventory failure and an empty backup inventory are shown separately. Failed refreshes keep previous backups with a stale label. Successful reloads retain the selected backup by exact ID, even if a new snapshot changes list order. Preview/restore still revalidate actual files; returning from a completed report or refreshing never repeats a write.
