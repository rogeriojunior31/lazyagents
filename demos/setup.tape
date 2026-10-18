# Shared by every demo tape (Source demos/setup.tape): 190x50 terminal, SP Night
# colors, and a hidden fake HOME (scripts/demo-home.sh), never the real one.
Set Shell "bash"
Set FontFamily "MesloLGM Nerd Font Mono"
Set FontSize 28
Set Columns 190
Set Rows 50
Set LineHeight 1.0
Set Padding 28
Set Framerate 20
Set Theme '{"name":"SP Night","background":"#151723","foreground":"#d3d7eb","cursor":"#f2984a","selection":"#272937","black":"#0f101a","red":"#ea5d5d","green":"#89d093","yellow":"#f5c66b","blue":"#6e92de","magenta":"#b094e2","cyan":"#5dbec4","white":"#868999","brightBlack":"#707380","brightRed":"#ff716f","brightGreen":"#9ce4a6","brightYellow":"#ffdc9c","brightBlue":"#80a5f2","brightMagenta":"#c3a7f6","brightCyan":"#71d1d7","brightWhite":"#e7ebff"}'

Hide
Type `exec env -i HOME="$HOME" PATH="$HOME/bin" TERM=xterm-256color COLORTERM=truecolor LANG=C.UTF-8 TZ=UTC "$LAZYAGENTS_DEMO_BINARY"`
Enter
Sleep 1.5s
Show
