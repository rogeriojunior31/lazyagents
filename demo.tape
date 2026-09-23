# Grava demo.gif com dados fictícios (scripts/demo-home.sh), nunca com o HOME real.
# Requisitos: vhs, ttyd e ffmpeg no PATH. Uso: scripts/record-demo.sh
Output demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 1280
Set Height 720
# tema do terminal = Noite Paulista (ANSI do SP Night)
Set Theme '{"name":"Noite Paulista","background":"#151723","foreground":"#d3d7eb","cursor":"#f2984a","selection":"#272937","black":"#0f101a","red":"#ea5d5d","green":"#89d093","yellow":"#f5c66b","blue":"#6e92de","magenta":"#b094e2","cyan":"#5dbec4","white":"#868999","brightBlack":"#707380","brightRed":"#ff716f","brightGreen":"#9ce4a6","brightYellow":"#ffdc9c","brightBlue":"#80a5f2","brightMagenta":"#c3a7f6","brightCyan":"#71d1d7","brightWhite":"#e7ebff"}'
Set Padding 10
Set PlaybackSpeed 0.8

# setup oculto: HOME de demonstração + binário no PATH
Hide
Type "demo_root=$(mktemp -d /tmp/lazyagents-demo.XXXXXX); scripts/demo-home.sh $demo_root/home >/dev/null && go build -o $demo_root/home/bin/lazyagents ."
Enter
Sleep 8s
Type "export HOME=$demo_root/home PATH=$demo_root/home/bin:$PATH; unset XDG_CONFIG_HOME XDG_DATA_HOME; cd ~ && clear"
Enter
Sleep 1s
Show

Type "lazyagents"
Sleep 400ms
Enter

# splash com o logo (a preguiça na rede) → aba Skills: matriz skill × agente
Sleep 1.8s
Enter
Sleep 2s

# code-review (a primeira da lista é local): ativa no Codex (tecla 2 cria o symlink) e desativa de novo
Down
Sleep 800ms
Type "2"
Sleep 1.5s
Type "2"
Sleep 1s

# foco no painel de detalhe e volta
Right
Sleep 1s
Left
Sleep 600ms

# lê o SKILL.md
Enter
Sleep 1.8s
Escape
Sleep 600ms

# paleta de comandos: vai para Sessões
Type ":"
Sleep 600ms
Type "ses"
Sleep 800ms
Enter
Sleep 1.5s

# busca full-text nos transcripts
Type "F"
Sleep 600ms
Type "idempot"
Sleep 700ms
Enter
Sleep 1.5s

# transcript da sessão encontrada em cards de chat
Type "v"
Sleep 2.5s
Escape
Sleep 600ms

# limpa a busca
Escape
Sleep 800ms

# agrupa por agente + projeto
Type "g"
Sleep 1.8s
Type "g"
Sleep 600ms

# aba Agentes
Tab
Sleep 2s

# ajuda contextual
Type "?"
Sleep 1.8s
Escape
Sleep 500ms

# aba Provedores: perfis fictícios para API e modelo local
Tab
Sleep 2s
Down
Sleep 900ms

# aplicar no Claude Code pede confirmação mostrando de → para; aqui cancela
Type "1"
Sleep 2.2s
Escape
Sleep 700ms

# aba Hooks: comando fictício associado a edições; instalar pede confirmação
Tab
Sleep 2s
Type "1"
Sleep 2s
Escape
Sleep 700ms

# aba Uso: limites fictícios lidos do cache local
Tab
Sleep 2.5s

Type "q"
Sleep 500ms
