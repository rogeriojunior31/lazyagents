# Records demo.gif with fake data (scripts/demo-home.sh), never with the real HOME.
# Requires vhs, ttyd and ffmpeg in PATH. Usage: scripts/record-demo.sh
Output demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 1280
Set Height 720
# terminal theme = SP Night (its ANSI colors)
Set Theme '{"name":"SP Night","background":"#151723","foreground":"#d3d7eb","cursor":"#f2984a","selection":"#272937","black":"#0f101a","red":"#ea5d5d","green":"#89d093","yellow":"#f5c66b","blue":"#6e92de","magenta":"#b094e2","cyan":"#5dbec4","white":"#868999","brightBlack":"#707380","brightRed":"#ff716f","brightGreen":"#9ce4a6","brightYellow":"#ffdc9c","brightBlue":"#80a5f2","brightMagenta":"#c3a7f6","brightCyan":"#71d1d7","brightWhite":"#e7ebff"}'
Set Padding 10
Set PlaybackSpeed 0.8

# hidden setup: demo HOME + binary in PATH
Hide
Type "demo_root=$(mktemp -d /tmp/lazyagents-demo.XXXXXX); scripts/demo-home.sh $demo_root/home >/dev/null && go build -o $demo_root/home/bin/lazyagents ."
Enter
Sleep 8s
Type "export HOME=$demo_root/home PATH=$demo_root/home/bin:/usr/bin:/bin; unset XDG_CONFIG_HOME XDG_DATA_HOME; cd ~ && clear"
Enter
Sleep 1s
Show

Type "lazyagents"
Sleep 400ms
Enter

# splash with the logo (the sloth in the hammock) → Skills tab: skill × agent matrix
Sleep 1.8s
Enter
Sleep 2s

# code-review (first row): pick the Codex column, enable (space creates the
# symlink) and disable again; digits switch tabs, so never type 1-9 here
Sleep 800ms
Right
Sleep 700ms
Space
Sleep 1.5s
Space
Sleep 1s

# read the SKILL.md
Enter
Sleep 1.8s
Escape
Sleep 600ms

# command palette: go to Sessions
Type ":"
Sleep 600ms
Type "ses"
Sleep 800ms
Enter
Sleep 1.5s

# full-text search in the transcripts
Type "F"
Sleep 600ms
Type "idempot"
Sleep 700ms
Enter
Sleep 1.5s

# transcript of the matching session: the answer, with the steps folded into one line
Type "v"
Sleep 2.8s

# every step: reasoning, lookups, edits (+lines), the failing test (✗) and its fix
Type "t"
Sleep 3s
Type "t"
Sleep 500ms

# actions view: only what the agent did to files and shell
Type "m"
Sleep 200ms
Type "m"
Sleep 2.5s
Type "m"
Sleep 400ms
Escape
Sleep 600ms

# clear the search
Escape
Sleep 800ms

# group by agent + project
Type "g"
Sleep 1.8s
Type "g"
Sleep 600ms

# Providers tab: fake profiles for an API and a local model
Tab
Sleep 2s

# contextual help
Type "?"
Sleep 1.8s
Escape
Sleep 500ms

# applying the first profile to Claude Code asks for confirmation showing from → to; cancel here
Space
Sleep 2.2s
Escape
Sleep 700ms

# Hooks tab: a fake command tied to edits; installing in Codex asks for
# confirmation and warns that Codex hooks are off
Tab
Sleep 2s
Right
Sleep 600ms
Space
Sleep 2.5s
Escape
Sleep 700ms

# Usage tab: fake limits read from the local cache
Tab
Sleep 2.5s

# Agents tab
Tab
Sleep 2s

Type "q"
Sleep 500ms
