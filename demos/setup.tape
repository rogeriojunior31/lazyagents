# Shared by every demo tape (Source demos/setup.tape): 2x terminal, SP Night
# colors, and a hidden fake HOME (scripts/demo-home.sh), never the real one.
Set Shell "bash"
Set FontFamily "MesloLGM Nerd Font Mono"
Set FontSize 28
Set LineHeight 1.0
Set Padding 28
Set Framerate 30
Set Theme '{"name":"SP Night","background":"#151723","foreground":"#d3d7eb","cursor":"#f2984a","selection":"#272937","black":"#0f101a","red":"#ea5d5d","green":"#89d093","yellow":"#f5c66b","blue":"#6e92de","magenta":"#b094e2","cyan":"#5dbec4","white":"#868999","brightBlack":"#707380","brightRed":"#ff716f","brightGreen":"#9ce4a6","brightYellow":"#ffdc9c","brightBlue":"#80a5f2","brightMagenta":"#c3a7f6","brightCyan":"#71d1d7","brightWhite":"#e7ebff"}'

Hide
Type "demo_root=$(mktemp -d /tmp/lazyagents-demo.XXXXXX); scripts/demo-home.sh $demo_root/home >/dev/null && go build -o $demo_root/home/bin/lazyagents ."
Enter
Sleep 8s
Type "export HOME=$demo_root/home PATH=$demo_root/home/bin:/usr/bin:/bin; unset XDG_CONFIG_HOME XDG_DATA_HOME; mkdir -p ~/.config/lazyagents && printf 'tui:\n  splash: false\n' > ~/.config/lazyagents/config.yaml; cd ~ && clear && lazyagents"
Enter
Sleep 1.5s
Show
