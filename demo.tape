Output demo.gif

Require lazykills

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "Tokyo Night"
Set Padding 10
Set PlaybackSpeed 0.75

# iniciar a TUI
Type "lazykills"
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

# instalar uma skill do GitHub
Type "i"
Sleep 500ms
Type "anthropics/claude-code-config"
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

Escape
