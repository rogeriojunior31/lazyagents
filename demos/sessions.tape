# Sessions: full-text search, the transcript reader and its views.
# Fake data only (scripts/demo-home.sh). Usage: scripts/record-demo.sh sessions
# title: lazyagents — sessions

Set Width 2400
Set Height 1300
Source demos/setup.tape

Type "2"
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

# clear the search, then group by agent + project
Escape
Sleep 800ms
Type "g"
Sleep 1.8s
Type "g"
Sleep 1s
