# Short loop for the top of the README (~12 s): one skill enabled in every
# agent, a session read as a log, the limits. Fake data only (scripts/demo-home.sh).
# title: lazyagents — skills · sessions · usage
# Usage: scripts/record-demo.sh hero

Set Width 2000
Set Height 1100
Source demos/setup.tape

# Skills: release-notes is off everywhere; a enables it in every agent
Sleep 1.2s
Down
Sleep 150ms
Down
Sleep 150ms
Down
Sleep 150ms
Down
Sleep 900ms
Type "a"
Sleep 2s

# Sessions from every agent, one read as a log
Tab
Sleep 1.6s
Type "v"
Sleep 2.6s
Escape
Sleep 300ms

# Usage: subscription limits
Type ":"
Sleep 300ms
Type "usa"
Sleep 300ms
Enter
Sleep 2.5s
