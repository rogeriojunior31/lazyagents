# Referência de teclas
<!-- source: e7aefa363f43 -->

As mesmas listas aparecem na TUI: `?` abre a ajuda da aba atual e `:` abre a paleta, que aceita parte do nome dos comandos abaixo. Esta referência traduz as ações; os nomes de comandos e teclas permanecem iguais aos da interface.

Enquanto um campo, filtro ou confirmação estiver aberto, ele recebe o teclado: `esc` fecha e `enter` confirma.

<a id="navigation"></a>

## Navegação

| Tecla | Ação |
|---|---|
| `tab` | próxima aba |
| `shift+tab` | aba anterior |
| `1-9` | ir à aba correspondente |
| `:` | paleta de comandos |
| `?` | abrir/fechar ajuda |
| `q` | sair |

<a id="skills-tab"></a>

## Aba Skills

<a id="bulk-results"></a>

### Resultados em lote

| Tecla | Ação |
|---|---|
| `↑/↓ · pgup/pgdn` | rolar resultados; mantidos até serem dispensados |
| `r` | repetir apenas nos destinos com falha, após confirmar |
| `b` | abrir backups de skills |
| `esc · enter` | dispensar relatório e ver o estado atualizado |

### Skills

| Tecla | Ação |
|---|---|
| `enter` | ler SKILL.md |
| `e` | editar em $EDITOR |
| `n` | criar skill |
| `o` | incorporar à biblioteca |
| `A` | incorporar todas as skills locais |
| `d` | remover com backup |
| `i` | instalar do GitHub/pasta/ZIP |
| `S` | buscar no GitHub |

<a id="activation"></a>

### Ativação

| Tecla | Ação |
|---|---|
| `←/→` | escolher coluna do agente |
| `space` | alternar no agente escolhido |
| `a` | ativar em todos |
| `x` | desativar em todos |

<a id="profiles--updates"></a>

### Perfis e atualizações

| Tecla | Ação |
|---|---|
| `p` | perfis: enter aplica, s salva matriz, d apaga |
| `u` | atualizar esta skill |
| `U` | verificar atualizações |
| `b` | backups da skill selecionada; todos quando não houver seleção |
| `B` | todos os backups de skills; `/` filtra por nome ou data |

<a id="list"></a>

### Lista

| Tecla | Ação |
|---|---|
| `shift+↑/↓` | rolar detalhes |
| `/` | filtrar |
| `r` | recarregar |

<a id="sessions-tab"></a>

## Aba Sessions

<a id="sessions"></a>

### Sessões

| Tecla | Ação |
|---|---|
| `enter` | retomar |
| `v` | ler conversa |
| `R` | retomar em outra pasta |
| `c` | mostrar comando |
| `m` | definir alias; vazio remove |
| `d` | excluir com backup |

<a id="list-1"></a>

### Lista

| Tecla | Ação |
|---|---|
| `shift+↑/↓` | rolar detalhes |
| `pgup/pgdn` | paginar lista |
| `space` | selecionar em lote/grupo |
| `g` | agrupar por projeto e agente |
| `f` | alternar filtro de agente |
| `F` | buscar nas conversas |
| `/` | filtrar |
| `r` | recarregar |

<a id="transcript-v"></a>

### Leitor de conversa (v)

| Tecla | Ação |
|---|---|
| `n · N` | sua próxima mensagem · anterior |
| `g · G` | início · fim |
| `m` | visualização: registro / conversa / ações |
| `] · [` | próximo turno/subagente · anterior |
| `enter` | expandir turno escolhido · abrir subagente |
| `e` | etapas (⋯): todas / apenas resposta |
| `t` | comandos (❯): um por linha / resumidos |
| `r` | raciocínio (💭): completo / primeira linha |
| `x` | exportar para Markdown |
| `esc` | voltar à lista |

<a id="providers-tab"></a>

## Aba Providers

<a id="providers"></a>

### Provedores

| Tecla | Ação |
|---|---|
| `↑/↓ · j/k` | selecionar perfil |
| `←/→` | selecionar coluna do agente |
| `space` | aplicar no agente; repetir limpa |
| `a` | revisar alterações e escolher agentes de destino |
| `shift+↑/↓ · ctrl+u/d` | rolar detalhes |
| `x` | limpar provedor de todos |
| `n` | escolher template de conexão, perfil personalizado ou padrões do agente |
| `i` | importar o agente selecionado para um perfil editável na biblioteca |
| `e` | editar perfil; token vazio mantém o salvo |
| `d` | apagar perfil da biblioteca |
| `r` | recarregar |

<a id="connection-templates-and-form"></a>

### Templates de conexão e formulário

| Tecla | Ação |
|---|---|
| `↑/↓ · j/k` | escolher configuração inicial; enter abre o formulário editável |
| `tab · shift+tab (form)` | selecionar campo e ler sua orientação |
| `shift+↑/↓ · ctrl+u/d` | rolar detalhes do template ou ajuda do campo em foco |
| `enter · ctrl+s (form)` | salvar na biblioteca; aplicar continua separado |
| `esc` | cancelar template ou formulário sem salvar |

<a id="provider-validation"></a>

### Validação de provedores

| Tecla | Ação |
|---|---|
| `↑/↓ · j/k` | selecionar destino e ler requisitos |
| `space` | incluir/excluir agente; seleções incompatíveis bloqueiam a aplicação |
| `shift+↑/↓ · ctrl+u/d` | rolar detalhes do destino |
| `enter` | revisar alterações de campos mascarados antes da confirmação |
| `esc` | cancelar sem gravar |
| `ctrl+v (form)` | verificar o rascunho sem salvar ou aplicar |

<a id="bulk-results-1"></a>

### Resultados em lote

| Tecla | Ação |
|---|---|
| `↑/↓ · pgup/pgdn` | rolar resultados; mantidos até serem dispensados |
| `r` | revisar e confirmar nova tentativa apenas nos destinos com falha |
| `b` | abrir Recovery |
| `esc · enter` | dispensar relatório e ver o estado atualizado |

<a id="create-a-profile"></a>

### Criar perfil

| Comando/opção | Ação |
|---|---|
| `lazyagents provider add` | criar pela CLI |
| `--token -` | ler token da entrada padrão |

<a id="hooks-tab"></a>

## Aba Hooks

### Hooks

| Tecla | Ação |
|---|---|
| `↑/↓ · j/k` | selecionar hook |
| `←/→` | selecionar coluna do agente |
| `space` | instalar no selecionado; repetir desinstala |
| `a` | instalar em todos os agentes compatíveis com seus eventos |
| `enter` | escolher comandos do pacote; space alterna, esc volta |
| `x` | desinstalar de todos |
| `d` | apagar da biblioteca |
| `pgup/pgdn · ctrl+u/d` | ler detalhes e comandos longos, inclusive durante seleção |
| `shift+↑/↓` | rolar detalhes uma linha |
| `v` | leitor completo: ←→ alterna comando/scripts, e edita |
| `r` | recarregar |

<a id="bulk-results-2"></a>

### Resultados em lote

| Tecla | Ação |
|---|---|
| `↑/↓ · pgup/pgdn` | rolar resultados; mantidos até serem dispensados |
| `r` | repetir apenas nos destinos com falha, após confirmar |
| `b` | abrir Recovery |
| `esc · enter` | dispensar relatório e ver o estado atualizado |

<a id="create-a-hook"></a>

### Criar hook

| Comando/caminho | Ação |
|---|---|
| `lazyagents hooks add` | criar pela CLI |
| `~/.local/share/lazyagents/hooks` | JSON por hook, editável manualmente |

<a id="recovery-tab"></a>

## Aba Recovery

<a id="config-backups"></a>

### Backups de configuração

| Tecla | Ação |
|---|---|
| `↑/↓ · j/k` | selecionar backup de configuração |
| `/` | filtrar por agente, arquivo, operação ou data local |
| `enter` | abrir a prévia disponível ou inspecionar por que a recuperação está indisponível |
| `l` | alternar entre cópias disponíveis para prévia e todos os backups |
| `r` | recarregar backups |
| `shift+↑/↓ · ctrl+u/d` | rolar detalhes do backup |

<a id="restore-preview"></a>

### Prévia de restauração

| Tecla | Ação |
|---|---|
| `↑/↓ · pgup/pgdn` | ler todas as diferenças ocultadas |
| `enter` | confirmar a substituição do arquivo inteiro |
| `esc` | voltar sem restaurar |

<a id="usage-tab"></a>

## Aba Usage

<a id="filters"></a>

### Filtros

| Tecla | Ação |
|---|---|
| `p / P` | período: hoje · 7 · 30 · 90 dias · tudo |
| `a / A` | todos os agentes ou só um |
| `←/→ · v` | visualização por dia · agente · projeto · modelo |
| `/` | filtrar linhas por nome |
| `esc` | limpar texto; repetir volta a todos os agentes |

<a id="usage"></a>

### Consumo

| Tecla | Ação |
|---|---|
| `r` | atualizar limites, consultando a API |
| `↑/↓ · j/k` | rolar |
| `pgup/pgdn · space` | rolar uma página |
| `g / home` | voltar ao início |

<a id="agents-tab"></a>

## Aba Agents

<a id="agents"></a>

### Agentes

| Tecla | Ação |
|---|---|
| `↑/↓ · j/k` | selecionar agente |
| `shift+↑/↓ · ctrl+u/d` | rolar detalhes |
| `g · G` | primeiro · último |

<a id="command-palette"></a>

## Paleta de comandos

Digite `:` e parte de um nome. Comandos de abas executam dentro delas.

| Entrada | Ação |
|---|---|
| `skills` | abrir Skills |
| `sessions` | abrir Sessions |
| `providers` | abrir Providers |
| `hooks` | abrir Hooks |
| `recovery` | abrir Recovery |
| `usage` | abrir Usage |
| `agents` | abrir Agents |
| `skills backups` | consultar todos os backups de skills, incluindo removidas |
| `providers new` | escolher template de conexão, perfil personalizado ou padrões do agente |
| `providers clear` | limpar provedor de todos |
| `usage refresh` | atualizar limites |
| `usage period today` | período de hoje |
| `usage period 7d` | últimos 7 dias |
| `usage period 30d` | últimos 30 dias |
| `usage period 90d` | últimos 90 dias |
| `usage period all` | todo o histórico |
| `usage view daily` | consumo por dia |
| `usage view agents` | consumo por agente |
| `usage view projects` | consumo por projeto |
| `usage view models` | consumo por modelo |
| `usage clear` | limpar filtros de agente e texto |
| `help` | ajuda da aba atual |
| `reload` | recarregar aba |
| `quit` | sair do lazyagents |
