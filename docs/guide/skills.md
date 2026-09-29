# Skills

A skill is a folder with a `SKILL.md`: instructions an agent loads when the task matches its description. Each agent reads skills from its own directory, so the same skill usually ends up copied in several places. lazyagents keeps one copy in a central library and makes it visible to each agent through a symlink, so you install and update a skill once and choose per agent where it is enabled. The Skills tab shows this as a skill × agent matrix, and the same operations are available from the CLI.

## Concepts

**Library.** The folder where lazyagents keeps the skills it manages: `~/.local/share/lazyagents/skills/` by default, or `libraryDir` in `config.yaml` (see [Configuration](#configuration)). Each skill is one subfolder, named after the skill.

**Activation is a symlink.** Enabling a skill in an agent creates a symlink `<agent skills dir>/<skill> → <library>/<skill>`. Disabling removes that symlink and nothing else. Since the agent reads the library folder through the link, editing or updating a skill applies to every agent at once. Where each agent reads its skills is in the [agent reference](../reference/agents.md#skills).

**Local skills.** A skill that lives in an agent's directory as a real folder, or as a symlink created by another tool, is *local*: lazyagents shows it, but never deletes or disables it. To manage it, *adopt* it into the library (see [Adopt local skills](#adopt-local-skills)).

**Shared directories.** `~/.agents/skills` is read by Codex, Gemini CLI, OpenCode and Pi. Enabling a skill for one agent only shows it to that agent: lazyagents links it in the agent's own directory (`~/.codex/skills`, `~/.gemini/skills`…). Only when the skill is enabled for every installed agent that reads `~/.agents/skills` does it become a single link there; disabling it for one of them moves it back to the own directory of each agent that keeps it. The exception is `~/.claude/skills`, Claude Code's own directory, which OpenCode also reads: a skill enabled for Claude Code shows up in OpenCode too.

**Matrix markers.** Each cell of the matrix shows the skill's state in one agent:

| Marker | Meaning |
|---|---|
| `●` | enabled by lazyagents (a symlink in this agent's own directory) |
| `◆` | visible through another agent's directory (OpenCode reading `~/.claude/skills`); it cannot be toggled for this agent alone |
| `▪` | local: real content lazyagents does not manage |
| `○` | not visible to this agent |

**Profiles.** A profile is a named snapshot of the matrix: which library skills are enabled in which agents. Applying one brings the matrix back to that state, for example to switch between a "work" and a "personal" set.

**Backups.** Before a skill folder is removed, replaced, restored over, adopted or migrated, lazyagents writes a `.tar.gz` of it to `~/.local/share/lazyagents/backups/`. The 20 newest backups of each skill are kept.

## In the TUI

The matrix has one row per skill and one column per installed agent that has a skills directory. `←`/`→` pick the agent column; the selected skill's detail (description, source, per-agent state and lint warnings) shows beside or below the table. The complete list of keys is in the [key reference](../reference/keys.md#skills-tab), and `?` shows it inside the TUI.

### Browse and read

Move through the list and type `/` to filter by name. `enter` opens the `SKILL.md` rendered as Markdown; inside the reader, `e` opens it in your editor and `esc` goes back. `r` rescans the library and the agents' directories. The tab title counts the local skills that are not in the library yet.

### Enable and disable

- `space` toggles the selected skill in the agent under the cursor; `1`–`9` toggle it in agent N, counting columns from the left.
- `a` enables the skill in every installed agent that has a skills directory; `x` disables it everywhere lazyagents manages it.

A cell marked `▪` or `◆` cannot be toggled from its column: the toast explains why (adopt a local skill with `o`, or use `x` for a skill that reaches the agent through another agent's directory). Only library skills can be enabled.

### Install from GitHub, a folder or a zip

Press `i` and type a source:

- `owner/repo` or any git URL (`https://…`, `git@…`, `….git`): cloned with `git clone --depth 1` into a temporary folder, so `git` must be in `PATH`.
- A path to a `.zip` file.
- A local folder (`~` is expanded).

lazyagents looks for every folder with a `SKILL.md`, at any depth. If the source root has one, the source itself is the skill. `.git`, `node_modules` and `vendor` are skipped; when two folders have the same name, the one outside hidden directories and closest to the root wins. A picker then lists what was found, all skills checked: `space` toggles one, `a` toggles all, `enter` installs, `esc` cancels.

Installing only copies into the library and records the source in `.origin.json` inside the skill folder. Nothing is enabled until you enable it. A name that already exists in the library is not overwritten: that item fails and the rest of the install goes on.

**Marketplaces.** When the source has a Claude Code marketplace manifest (`.claude-plugin/marketplace.json`) that declares skills, lazyagents reads the manifest instead of scanning, and the picker prefixes each skill with the name of its plugin. Plugins whose source is another repository or a package are not fetched; they show up as notes in the picker.

**Hooks in the same source.** If the source ships `hooks/hooks.json` (the Claude Code plugin format), its hooks appear in the same picker, unchecked, because a hook runs a third-party command on every agent event. See the [hooks guide](hooks.md).

### Search GitHub

`S` searches GitHub code for `SKILL.md` files that contain your term and lists the repositories found, with their descriptions. `enter` on a result starts the install flow above for that repository. The search goes through `gh api`, so it needs the [GitHub CLI](https://cli.github.com/) installed and logged in. It only runs when you ask for it.

### Create and edit

`n` asks for a name in kebab-case (lowercase letters, digits and hyphens, at most 64 characters) and creates `<library>/<name>/SKILL.md` from a template with `name` and `description` filled in. `e` opens the selected skill's `SKILL.md` in `$EDITOR` (`vi` if unset), suspending the TUI until the editor exits.

### Adopt local skills

`o` adopts the selected local skill: its folder is copied into the library, the original is backed up, and the original is replaced by a symlink to the library copy. The agent keeps seeing the skill, and the other agents can now enable it. `A` adopts every local skill at once, after a confirmation that lists them.

A skill that is a symlink created by another tool cannot be adopted: manage it with the tool that created it. A name that already exists in the library is refused too.

### Update from source

Skills installed from a git source can be updated; skills from a zip, a folder or created by hand cannot.

- `u` updates the selected skill after a confirmation: the repository is cloned again, the current folder is backed up and its content is replaced in place, so the agents' symlinks keep working.
- `U` checks every git-sourced skill, cloning each repository once, and marks the matrix: `↑` update available, `~` edited locally. If updates are available, it asks to apply them all. Skills edited locally are skipped, so your changes are not overwritten; `u` updates one of them anyway, with a backup first.

lazyagents detects local edits by comparing the folder's content with the hash it saved at install or update time.

### Profiles

`p` opens the profile list:

- `s` saves the current matrix as a profile. Only the enablements lazyagents controls are recorded: local skills and skills seen through another agent's directory are left out.
- `enter` on a profile shows what applying it would change (`+` enable, `−` disable, per skill and agent) and asks for confirmation.
- `d` deletes the selected profile after a confirmation. Skills stay enabled as they are.

Applying a profile enables each listed library skill in the agents the profile names and disables the other library skills where lazyagents enabled them. Local skills are never touched. A profile that names a skill no longer in the library is refused.

Profiles are stored in `~/.local/share/lazyagents/profiles.json`.

### Backups and restore

`b` lists the backups of the selected skill, newest first. `enter` restores the selected one after a confirmation. The current folder, if any, is backed up before being replaced, so a restore can itself be undone. The backup is unpacked to a temporary folder and checked before the current folder is swapped out, so a corrupt backup leaves the skill as it was.

### SKILL.md lint

Every skill is checked against the [Agent Skills](https://agentskills.io) format, and problems show in the detail panel and in `lazyagents doctor`:

- `name` is present, in kebab-case and equal to the folder name;
- `description` is not empty and has at most 1024 characters;
- there are instructions after the frontmatter.

A `SKILL.md` whose frontmatter cannot be parsed is flagged as invalid instead.

## From the CLI

The CLI covers the everyday operations. Every command is described in the [CLI reference](../reference/cli.md#list), and `lazyagents help <command>` prints the same text.

```sh
lazyagents install anthropics/skills            # install every skill in a repository
lazyagents install ./my-skills.zip --hooks      # also install the hooks it ships
lazyagents list                                 # skills, agent count, library or local
lazyagents enable pdf --agent claude-code       # enable in one agent
lazyagents disable pdf --agent codex
lazyagents enable pdf                           # every installed agent, like a in the TUI
lazyagents adopt my-notes --agent claude-code   # move a local skill into the library
lazyagents remove pdf                           # remove from the library, with backup
lazyagents list --json | jq '.[] | select(.valid | not) | .dir'
```

`lazyagents skills <command>` is an alias for the same commands. Search, create, update, profiles and restore are only in the TUI for now.

## Configuration

```yaml
# ~/.config/lazyagents/config.yaml
libraryDir: ~/.agents/skills
```

`libraryDir` moves the library. Setting it by hand only changes where lazyagents looks; the existing skills and the symlinks that point to them stay where they were. To move them too, use [`lazyagents migrate-library <dir>`](../reference/cli.md#migrate-library): it backs up and copies every skill, repoints the agents' symlinks, saves `libraryDir`, and only then removes the old copies. It refuses when a skill name already exists in the destination (symlinks included) or when one directory contains the other; resolve the conflict and run it again.

Pointing the library at `~/.agents/skills` makes every skill visible to Codex, Gemini CLI, OpenCode and Pi without any symlink, since they read that directory; the price is that they can no longer be enabled per agent. The other options are in the [configuration guide](../configuration.md).

## Files

| Path | Written when |
|---|---|
| `~/.local/share/lazyagents/skills/<name>/` | install, create, adopt, update, restore, remove |
| `~/.local/share/lazyagents/skills/<name>/.origin.json` | install and update: source type, URL or path, subfolder, content hash |
| each agent's skills directory | enable and disable (symlinks only), adopt (the original becomes a symlink), remove (links into the library) |
| `~/.local/share/lazyagents/backups/<name>.<timestamp>.tar.gz` | remove, update, adopt, restore, migrate-library (mode 0600, 20 per skill) |
| `~/.local/share/lazyagents/profiles.json` | saving a profile |
| `~/.config/lazyagents/config.yaml` | `migrate-library` (`libraryDir`) |

Git clones and extracted zips go to temporary folders that are removed after the install.

## Limits

- Symlinks inside a skill folder are not copied on install, adopt or migrate, so a skill cannot pull in files from outside its folder. Zip entries that are symlinks or escape the archive root are skipped, and an entry over 64 MB fails the install.
- Enabling uses symlinks. On Windows, creating them may need Developer Mode or administrator rights.
- Hermes Agent's extra skill directories (`external_dirs`) are not detected yet.
- Skills bundled inside Claude Code plugins are not managed.
- The TUI does not watch the filesystem: press `r` after changing skill folders by hand.
