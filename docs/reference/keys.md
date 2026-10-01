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
| `b` | backups |

### List

| Key | Action |
|---|---|
| `shift+↑/↓` | scroll detail |
| `/` | filter |
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
| `] · [` | pick the next · previous turn with steps |
| `enter` | unfold · fold the picked turn |
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
| `space` | apply to the selected agent (again to clear) |
| `a` | apply to all installed agents |
| `shift+↑/↓ · ctrl+u/d` | scroll the detail |
| `x` | clear the provider from all agents |
| `n` | new profile (form) |
| `e` | edit the profile; an empty token keeps the saved one |
| `d` | delete the profile from the library |
| `r` | reload |

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

### Create a hook

| Key | Action |
|---|---|
| `lazyagents hooks add` | create a hook from the CLI |
| `~/.local/share/lazyagents/hooks` | one JSON per hook, editable by hand |

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
| `usage` | open the Usage tab |
| `agents` | open the Agents tab |
| `providers new` | create a provider profile |
| `providers clear` | clear the provider from all agents |
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
