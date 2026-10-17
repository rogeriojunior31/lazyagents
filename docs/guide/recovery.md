# Configuration recovery

Recovery finds and restores snapshots of agent configuration files. [Skill archives](skills.md#backups-and-restore) have their own inventory in Skills.

## Find a change

Open **Recovery** or choose `: recovery`. `/` filters by agent ID, file, operation or local date (`YYYY-MM-DD`); all query words must match. `enter` applies the filter, then opens the selected preview. `esc` while editing clears the filter; `r` reloads. Returning to this tab also reloads the inventory, so snapshots created by another operation appear without restarting.

Operations `provider.apply`, `provider.clear`, `hook.install` and `hook.uninstall` identify the attempted write the snapshot preceded. They describe **before an attempt**, including one that later failed. A file that did not exist has no previous file to archive.

New snapshots record agent, exact destination, operation, date, permissions and checksum. Filenames stay unchanged. Provenance lives in `backups/.config-backup-meta/<backup>.json` (`0600`, folder created with `0700`), without configuration values. Metadata is removed when its snapshot rotates out; retention remains 20 snapshots per filename prefix.

Legacy snapshots without metadata show agent and operation as **unknown** and cannot be restored automatically. A basename such as `settings.json` does not prove its destination; no path is guessed or assigned to old backups.

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
