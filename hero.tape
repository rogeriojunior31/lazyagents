# Short loop for the top of the README (~12 s): one skill enabled in every
# agent, a session read as a log, the limits. Fake data only, like demo.tape.
# Usage: scripts/record-demo.sh hero.tape
Output hero.gif

Set Shell "bash"
Set FontSize 14
Set Width 1000
Set Height 600
# terminal theme = SP Night (its ANSI colors)
Set Theme '{"name":"SP Night","background":"#151723","foreground":"#d3d7eb","cursor":"#f2984a","selection":"#272937","black":"#0f101a","red":"#ea5d5d","green":"#89d093","yellow":"#f5c66b","blue":"#6e92de","magenta":"#b094e2","cyan":"#5dbec4","white":"#868999","brightBlack":"#707380","brightRed":"#ff716f","brightGreen":"#9ce4a6","brightYellow":"#ffdc9c","brightBlue":"#80a5f2","brightMagenta":"#c3a7f6","brightCyan":"#71d1d7","brightWhite":"#e7ebff"}'
Set Padding 10
Set PlaybackSpeed 1

# hidden setup: demo HOME + binary in PATH; the splash is off so no key
# depends on how long it takes to go away
Hide
Type "demo_root=$(mktemp -d /tmp/lazyagents-demo.XXXXXX); scripts/demo-home.sh $demo_root/home >/dev/null && go build -o $demo_root/home/bin/lazyagents ."
Enter
Sleep 8s
Type "export HOME=$demo_root/home PATH=$demo_root/home/bin:/usr/bin:/bin; unset XDG_CONFIG_HOME XDG_DATA_HOME; mkdir -p ~/.config/lazyagents && printf 'tui:\n  splash: false\n' > ~/.config/lazyagents/config.yaml; cd ~ && clear && lazyagents"
Enter
Sleep 1.5s
Show

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
