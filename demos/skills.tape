# Skills: the skill x agent matrix, a toggle and the SKILL.md reader.
# Fake data only (scripts/demo-home.sh). Usage: scripts/record-demo.sh skills
# title: lazyagents — skills

Set Width 2400
Set Height 1300
Source demos/setup.tape

Sleep 1.5s

# code-review (first row): pick the Codex column, enable (space creates the
# symlink) and disable again; digits switch tabs, so never type 1-9 here
Right
Sleep 700ms
Space
Sleep 1.5s
Space
Sleep 1s

# release-notes is off everywhere: a enables it in every agent
Down
Sleep 150ms
Down
Sleep 150ms
Down
Sleep 150ms
Down
Sleep 800ms
Type "a"
Sleep 2s

# read the SKILL.md
Enter
Sleep 2s
Escape
Sleep 600ms

# filter the library
Type "/"
Sleep 400ms
Type "dock"
Sleep 1.8s
Escape
Sleep 1s
