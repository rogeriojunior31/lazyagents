# Hooks: one command installed across agents.
# Fake data only (scripts/demo-home.sh). Usage: scripts/record-demo.sh hooks
# title: lazyagents — hooks

Set Width 2400
Set Height 1300
Source demos/setup.tape

Type "4"
Sleep 2s

# installing in Codex asks for confirmation and warns that Codex hooks are off
Right
Sleep 600ms
Space
Sleep 3s
Escape
Sleep 1s
