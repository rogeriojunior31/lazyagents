# Providers: API and local-model profiles applied to an agent.
# Fake data only (scripts/demo-home.sh). Usage: scripts/record-demo.sh providers
# title: lazyagents — providers

Set Width 2400
Set Height 1300
Source demos/setup.tape

Type "3"
Sleep 2s

# contextual help
Type "?"
Sleep 1.8s
Escape
Sleep 500ms

# applying the first profile to Claude Code asks for confirmation showing from → to; cancel here
Space
Sleep 2.5s
Escape
Sleep 1s
