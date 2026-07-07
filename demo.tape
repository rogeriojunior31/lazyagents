Output demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "TokyoNight"
Set Padding 10
Set PlaybackSpeed 0.75

# garantir que o binário existe
Hide
Type "go build -o lazyskills . && clear"
Enter
Sleep 5s
Show

# iniciar a TUI
Type "./lazyskills"
Enter
Sleep 3s

# splash aparece — avança com enter
Enter
Sleep 1s

# aba Skills (default) — mostra a matriz
Sleep 1.5s

# toggle da primeira skill no Claude Code (tecla 1)
Type "1"
Sleep 1s

# desfaz o toggle
Type "1"
Sleep 800ms

# instalar uma skill do GitHub (abre dialog, cancela logo depois)
Type "i"
Sleep 500ms
Type "anthropics/ponytail"
Sleep 500ms
Escape
Sleep 500ms

# navegar para aba Sessões
Tab
Sleep 1.5s

# navegar para aba Agentes
Tab
Sleep 1.5s

# voltar para Skills
Tab
Sleep 1s

Type "q"
