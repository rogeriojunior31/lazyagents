# Referência de teclas
<!-- source: 79c1ffff40ab -->

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

<a id="source-repositories"></a>

### Repositórios de origem

| Tecla | Ação |
|---|---|
| `R` | abrir repositórios salvos; sem varredura automática |
| `a · e` | nas origens: adicionar ou editar URL, branch/tag e subpasta |
| `enter · s` | verificar apenas a origem escolhida e selecionar itens para instalar |
| `d` | remover cadastro após confirmação; manter skills instaladas |
| `r` | recarregar cadastros sem buscar repositórios |
| `shift+↑/↓` | rolar detalhes, última tentativa e erros da origem |
| `esc` | fechar origens ou cancelar formulário |

<a id="install-picker"></a>

### Seletor de instalação

| Tecla | Ação |
|---|---|
| `↑/↓ · space` | escolher e marcar itens ou agentes de destino |
| `a` | alternar todas as skills na origem; todos os agentes nos destinos |
| `0 · x` | nos destinos: somente biblioteca, sem links de ativação |
| `enter` | origem: destinos; destinos: revisar antes de instalar |
| `tab` | alternar entre itens da origem e destinos |
| `pgup/pgdn` | rolar avisos da origem |
| `esc` | destinos: voltar aos itens; itens: cancelar e limpar arquivos temporários |

<a id="selected-skill-actions"></a>

### Ações nas skills marcadas

| Tecla | Ação |
|---|---|
| `m` | marcar ou desmarcar a pasta atual |
| `v` | substituir seleção por todas as linhas visíveis, inclusive fora da tela |
| `V` | limpar marcas; linhas ocultas ou removidas também são desmarcadas |
| `c` | escolher ativar, desativar, atualizar ou remover as linhas marcadas |
| `↑/↓ · ←/→ · space` | no painel de lote: escolher ação e agentes de destino |
| `a · x` | no painel de lote: selecionar todos os agentes ou nenhum |
| `enter · esc` | no painel de lote: revisar escopo fixado ou cancelar |

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

<a id="inventory-filters"></a>

### Filtros de inventário

| Tecla | Ação |
|---|---|
| `↑/↓` | escolher localização, agente, metadados ou atualizações |
| `←/→ · space` | alternar valores; sem ler arquivos ou acessar a rede |
| `enter` | aplicar filtros junto com o texto da busca |
| `r` | redefinir filtros de estado do rascunho; manter texto |
| `esc` | cancelar sem alterar os filtros ativos |

<a id="list"></a>

### Lista

| Tecla | Ação |
|---|---|
| `shift+↑/↓` | rolar detalhes |
| `/` | buscar nome, descrição, pasta e origem |
| `f` | filtrar localização, ativação por agente, metadados e estado de atualização armazenado |
| `F` | limpar texto e filtros de estado; manter seleção visível |
| `esc` | limpar busca textual; manter filtros de estado |
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
| `space` | revisar aplicação; limpar uma correspondência única gerenciada |
| `p` | revisar/reaplicar o perfil selecionado no agente selecionado |
| `a` | revisar alterações e escolher agentes de destino |
| `shift+↑/↓ · ctrl+u/d` | rolar detalhes |
| `x` | limpar provedor de todos |
| `n` | escolher template de conexão, perfil personalizado ou padrões do agente |
| `i` | importar o agente selecionado para um perfil editável na biblioteca |
| `e` | editar perfil; token vazio mantém o salvo |
| `d` | apagar perfil da biblioteca |
| `c` | verificar o endereço do endpoint salvo sem credenciais |
| `m` | consultar catálogo OpenAI compatível com credenciais do perfil |
| `r` | atualizar comparações entre salvo/atual sem gravar |

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

<a id="endpoint-and-model-checks"></a>

### Verificações de endpoint e modelo

| Tecla | Ação |
|---|---|
| `ctrl+t (form)` | consultar endpoint do rascunho sem credenciais |
| `ctrl+l (form)` | buscar catálogo OpenAI compatível do rascunho |
| `↑/↓ · type (catalog)` | selecionar ou filtrar um modelo retornado |
| `shift+↑/↓ · ctrl+u/d` | rolar evidências da requisição e erros |
| `enter (catalog)` | usar modelo no formulário; salvar/aplicar continuam separados |
| `esc` | cancelar a requisição ou voltar sem alterações |

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
| `skills sources` | gerenciar repositórios de skills salvos; verificar só quando solicitado |
| `providers new` | escolher template de conexão, perfil personalizado ou padrões do agente |
| `providers clear` | limpar provedor de todos |
| `providers probe` | verificar endereço do endpoint selecionado sem credenciais |
| `providers models` | consultar catálogo OpenAI compatível do perfil selecionado |
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
