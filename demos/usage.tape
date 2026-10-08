# Usage: subscription limits and token consumption per agent.
# Fake data only (scripts/demo-home.sh). Usage: scripts/record-demo.sh usage
# title: lazyagents — usage

Set Width 2400
Set Height 1300
Source demos/setup.tape

Type "5"
Sleep 2.5s

# the same tokens by agent, then by project
Type "v"
Sleep 2s
Type "v"
Sleep 2s

# a longer period
Type "p"
Sleep 2.5s
