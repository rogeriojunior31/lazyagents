# Grava demo.gif com dados fictícios (scripts/demo-home.sh), nunca com o HOME real.
# Requisitos: vhs, ttyd e ffmpeg no PATH. Uso, na raiz do repo: vhs demo.tape
Output demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 1280
Set Height 720
Set Theme "TokyoNight"
Set Padding 10
Set PlaybackSpeed 0.8

# setup oculto: HOME de demonstração + binário no PATH
Hide
Type "scripts/demo-home.sh /tmp/lazyagents-demo >/dev/null && go build -o /tmp/lazyagents-demo/bin/lazyagents ."
Enter
Sleep 8s
Type "export HOME=/tmp/lazyagents-demo PATH=/tmp/lazyagents-demo/bin:$PATH; unset XDG_CONFIG_HOME XDG_DATA_HOME; cd ~ && clear"
Enter
Sleep 1s
Show

Type "lazyagents"
Sleep 400ms
Enter
Sleep 1.2s

# splash → aba Skills: matriz skill × agente
Enter
Sleep 2s

# code-review: ativa no Codex (tecla 2 cria o symlink) e desativa de novo
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

# transcript da sessão em cards de chat
Type "v"
Sleep 2.5s
Escape
Sleep 600ms

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

Type "q"
Sleep 500ms
