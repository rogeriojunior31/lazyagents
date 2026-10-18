# Sessions: full-text search, the transcript reader and its views.
# Fake data only (scripts/demo-home.sh). Usage: scripts/record-demo.sh sessions
# title: lazyagents — sessions · sample data

Source demos/setup.tape

Type ":"
Type "sessions"
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

# Clear the full-text search, then filter native fixture sessions.
Escape
Sleep 800ms
Type "/"
Type "fixture"
Enter
Sleep 1s
Type "v"
Sleep 2s
Type "e"
Sleep 2s
Escape
Sleep 800ms

# OpenCode's native parent/call link opens the child's own transcript.
Down
Type "v"
Sleep 1s
Type "e"
Type "]"
Type "]"
Sleep 1s
Enter
Sleep 2.5s
Escape
Sleep 1s
