<!-- Generated from the code by `go test ./docs -update`. Do not edit by hand. -->

# Key reference

The same lists show inside the TUI: `?` opens the help of the current tab. `:` opens the command palette, which runs any entry below by typing part of its name.

While a text field, filter or confirmation is open it owns the keyboard: `esc` closes it, `enter` confirms.

## Navigation

| Key | Action |
|---|---|
| `tab` | next tab |
| `shift+tab` | previous tab |
| `1-9` | go to tab N |
| `:` | command palette |
| `?` | open or close help |
| `q` | quit |

## Skills tab

### Source repositories

| Key | Action |
|---|---|
| `R` | open saved repositories; no automatic scans |
| `a · e` | in sources: add or edit URL, branch/tag and subfolder |
| `enter · s` | scan only the chosen source and pick entries to install |
| `d` | remove descriptor after confirmation; keep installed skills |
| `r` | reload descriptors without fetching repositories |
| `shift+↑/↓` | scroll source details, last scan and errors |
| `esc` | close sources or cancel the source form |

### Install picker

| Key | Action |
|---|---|
| `↑/↓ · space` | choose and mark entries or agent destinations |
| `a` | toggle all skills on the source step; all agents on destinations |
| `0 · x` | on destinations: library only, without activation links |
| `enter` | source: destinations; destinations: review before installing |
| `tab` | switch source entries and destinations |
| `pgup/pgdn` | scroll source warnings |
| `esc` | destinations: back to entries; entries: cancel and clean staging |

### Selected skill actions

| Key | Action |
|---|---|
| `m` | mark or unmark the current folder |
| `v` | replace selection with all visible rows (including off screen) |
| `V` | clear marked rows; hidden or removed rows are also unmarked |
| `c` | choose enable, disable, update or remove for marked rows |
| `↑/↓ · ←/→ · space` | in batch panel: choose action and agent destinations |
| `a · x` | in batch panel: select all agents or none |
| `enter · esc` | in batch panel: review frozen scope or cancel |

### Bulk results

| Key | Action |
|---|---|
| `↑/↓ · pgup/pgdn` | scroll results; kept until dismissed |
| `r` | retry only failed targets (confirm first) |
| `b` | open skill backups |
| `esc · enter` | dismiss the report and view refreshed state |

### Skills

| Key | Action |
|---|---|
| `enter` | read SKILL.md |
| `e` | edit in $EDITOR |
| `n` | new skill |
| `o` | adopt into library |
| `A` | adopt all local skills |
| `d` | remove (with backup) |
| `i` | install (GitHub/folder/zip) |
| `S` | search GitHub (registry) |

### Activation

| Key | Action |
|---|---|
| `←/→` | pick agent (column) |
| `space` | toggle in picked agent |
| `a` | enable in all |
| `x` | disable in all |

### Profiles & updates

| Key | Action |
|---|---|
| `p` | profiles: enter applies, s saves the matrix, d deletes |
| `u` | update this skill |
| `U` | check for updates |
| `b` | backups of selected skill (all when empty) |
| `B` | all skill backups; / filters by name or date |

### Inventory filters

| Key | Action |
|---|---|
| `↑/↓` | choose location, agent, metadata or updates |
| `←/→ · space` | cycle values; no files or network are read |
| `enter` | apply filters together with the text search |
| `r` | reset draft state filters; keep text search |
| `esc` | cancel without changing the active filters |

### List

| Key | Action |
|---|---|
| `shift+↑/↓` | scroll detail |
| `/` | search name, description, folder and source |
| `f` | filter location, enabled agent, metadata and cached update state |
| `F` | clear text and state filters; keep a visible selection |
| `esc` | clear text search; keep state filters |
| `r` | reload |

## Sessions tab

### Sessions

| Key | Action |
|---|---|
| `enter` | resume |
| `v` | read the transcript |
| `R` | resume in another folder |
| `c` | show the command |
| `m` | alias (empty removes it) |
| `d` | delete (with backup) |

### List

| Key | Action |
|---|---|
| `shift+↑/↓` | scroll the detail |
| `pgup/pgdn` | page through the list |
| `space` | select (batch/group) |
| `g` | group by project+agent |
| `f` | cycle the agent filter |
| `F` | search the transcripts |
| `/` | filter |
| `r` | reload |

### Transcript (v)

| Key | Action |
|---|---|
| `n · N` | your next · previous prompt |
| `g · G` | top · end |
| `m` | view: log / conversation / actions |
| `] · [` | pick the next · previous turn or subagent |
| `enter` | unfold the picked turn · open the subagent |
| `e` | steps (⋯): every step / only the answer |
| `t` | commands (❯): one per line / summarized |
| `r` | reasoning (💭): full / first line only |
| `x` | export to Markdown |
| `esc` | back to the list |

## Providers tab

### Providers

| Key | Action |
|---|---|
| `↑/↓ · j/k` | select profile |
| `←/→` | select agent (column) |
| `space` | preview apply; clear a unique managed match |
| `p` | preview/reapply the selected profile to the selected agent |
| `a` | preview changes and choose destination agents |
| `shift+↑/↓ · ctrl+u/d` | scroll the detail |
| `x` | clear the provider from all agents |
| `n` | choose a connection template, custom profile or agent defaults |
| `i` | import the selected agent into an editable library profile |
| `e` | edit the profile; an empty token keeps the saved one |
| `d` | delete the profile from the library |
| `c` | check the saved endpoint address without credentials |
| `m` | fetch an OpenAI-compatible catalog with profile credentials |
| `r` | refresh saved/live comparisons without writing |

### Connection templates and form

| Key | Action |
|---|---|
| `↑/↓ · j/k` | choose a starting point; enter opens the editable form |
| `tab · shift+tab (form)` | select a field and read its guidance |
| `shift+↑/↓ · ctrl+u/d` | scroll template details or focused-field guidance |
| `enter · ctrl+s (form)` | save to the library; apply remains a separate action |
| `esc` | cancel the template/form without saving |

### Provider validation

| Key | Action |
|---|---|
| `↑/↓ · j/k` | select a destination and read its requirements |
| `space` | include/exclude an agent; incompatible selections block apply |
| `shift+↑/↓ · ctrl+u/d` | scroll destination details |
| `enter` | review masked field changes before confirmation |
| `esc` | cancel without writing |
| `ctrl+v (form)` | check the draft without saving or applying |

### Endpoint and model checks

| Key | Action |
|---|---|
| `ctrl+t (form)` | probe the draft endpoint without credentials |
| `ctrl+l (form)` | fetch the draft's OpenAI-compatible model catalog |
| `↑/↓ · type (catalog)` | select or filter a returned model |
| `shift+↑/↓ · ctrl+u/d` | scroll request evidence and errors |
| `enter (catalog)` | use the selected model in the form; save/apply stay separate |
| `esc` | cancel the request or return without changes |

### Bulk results

| Key | Action |
|---|---|
| `↑/↓ · pgup/pgdn` | scroll results; kept until dismissed |
| `r` | preview and confirm a retry of failed targets only |
| `b` | open Recovery |
| `esc · enter` | dismiss the report and view refreshed state |

### Create a profile

| Key | Action |
|---|---|
| `lazyagents provider add` | create a profile from the CLI |
| `--token -` | read the token from stdin |

## Hooks tab

### Hooks

| Key | Action |
|---|---|
| `↑/↓ · j/k` | select hook |
| `←/→` | select agent (column) |
| `space` | install in the selected agent (again uninstalls) |
| `a` | install in every installed agent that fires one of its events |
| `enter` | pick the commands of the pack (space toggles, esc goes back) |
| `x` | uninstall from every agent |
| `d` | delete the hook from the library |
| `pgup/pgdn · ctrl+u/d` | read the detail and long commands, also while selecting |
| `shift+↑/↓` | scroll the detail one line |
| `v` | full-screen reader: ←→ switches command/scripts, e edits |
| `r` | reload |

### Bulk results

| Key | Action |
|---|---|
| `↑/↓ · pgup/pgdn` | scroll results; kept until dismissed |
| `r` | retry only failed targets (confirm first) |
| `b` | open Recovery |
| `esc · enter` | dismiss the report and view refreshed state |

### Create a hook

| Key | Action |
|---|---|
| `lazyagents hooks add` | create a hook from the CLI |
| `~/.local/share/lazyagents/hooks` | one JSON per hook, editable by hand |

## Recovery tab

### Config backups

| Key | Action |
|---|---|
| `↑/↓ · j/k` | select config backup |
| `/` | filter by agent, file, operation or local date |
| `enter` | preview an available backup or inspect why recovery is unavailable |
| `l` | switch between ready-to-preview and all backups |
| `r` | reload backups |
| `shift+↑/↓ · ctrl+u/d` | scroll backup details |

### Restore preview

| Key | Action |
|---|---|
| `↑/↓ · pgup/pgdn` | read the full redacted diff |
| `enter` | confirm replacement of this whole file |
| `esc` | back without restoring |

## Usage tab

### Filters

| Key | Action |
|---|---|
| `p / P` | period: today · 7 · 30 · 90 days · all |
| `a / A` | agent: all or just one |
| `←/→ · v` | view: day · agent · project · model |
| `/` | filter table rows by name |
| `esc` | clear the text; again, back to all agents |

### Usage

| Key | Action |
|---|---|
| `r` | refresh limits (queries the API) |
| `↑/↓ · j/k` | scroll |
| `pgup/pgdn · space` | scroll one page |
| `g / home` | back to the top |

## Agents tab

### Agents

| Key | Action |
|---|---|
| `↑/↓ · j/k` | select agent |
| `shift+↑/↓ · ctrl+u/d` | scroll the detail |
| `g · G` | first · last |

## Command palette

Type `:` and part of a name. Tab entries run inside their tab.

| Entry | What it does |
|---|---|
| `skills` | open the Skills tab |
| `sessions` | open the Sessions tab |
| `providers` | open the Providers tab |
| `hooks` | open the Hooks tab |
| `recovery` | open the Recovery tab |
| `usage` | open the Usage tab |
| `agents` | open the Agents tab |
| `skills backups` | browse all skill backups, including deleted skills |
| `skills sources` | manage saved skill repositories; scan only on request |
| `providers new` | choose a connection template, custom profile or agent defaults |
| `providers clear` | clear the provider from all agents |
| `providers probe` | check the selected endpoint address without credentials |
| `providers models` | fetch the selected profile's OpenAI-compatible catalog |
| `usage refresh` | refresh usage limits |
| `usage period today` | usage: period today |
| `usage period 7d` | usage: period 7 days |
| `usage period 30d` | usage: period 30 days |
| `usage period 90d` | usage: period 90 days |
| `usage period all` | usage: period all |
| `usage view daily` | usage by day |
| `usage view agents` | usage by agent |
| `usage view projects` | usage by project |
| `usage view models` | usage by model |
| `usage clear` | usage: clear agent and text |
| `help` | open help for the current tab |
| `reload` | reload the current tab |
| `quit` | quit lazyagents |
