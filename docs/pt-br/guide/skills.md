# Skills
<!-- source: 3e7d9a8050e2 -->

Uma skill é uma pasta com um `SKILL.md`: instruções que um agente carrega quando a tarefa corresponde à descrição. Cada agente lê skills do próprio diretório, por isso a mesma skill costuma acabar copiada em vários lugares. O lazyagents mantém uma única cópia numa biblioteca central e a disponibiliza aos agentes por links simbólicos. Você instala e atualiza uma vez e escolhe em quais agentes ativar. A aba Skills mostra uma matriz de skill × agente; as mesmas operações estão disponíveis na CLI.

![A aba Skills: code-review ativada no Codex e desativada de novo, release-notes ativada em todos os agentes com uma tecla, o SKILL.md lido, a biblioteca filtrada](../../assets/demos/skills.gif)

<a id="concepts"></a>

## Conceitos

**Biblioteca.** Pasta das skills gerenciadas pelo lazyagents: `~/.local/share/lazyagents/skills/` por padrão, ou `libraryDir` em `config.yaml` ([Configuração](#configuration)). Cada skill ocupa uma subpasta com seu nome.

**Ativação por link simbólico.** Ativar cria `<diretório de skills do agente>/<skill> → <biblioteca>/<skill>`. Desativar remove apenas esse link. Como o agente lê a pasta da biblioteca, editar ou atualizar uma skill vale para todos os agentes ao mesmo tempo. Os diretórios lidos por cada agente estão na [referência dos agentes](../reference/agents.md#skills).

**Skills locais.** Uma pasta real no diretório de um agente, ou um link criado por outra ferramenta, é uma skill *local*. O lazyagents a mostra, mas nunca a apaga nem desativa. Para gerenciá-la, *incorpore-a* à biblioteca ([Incorporar skills locais](#adopt-local-skills)).

**Diretórios compartilhados.** Codex, Gemini CLI, OpenCode, Pi e Crush leem `~/.agents/skills`. Ao ativar uma skill para um só agente, o lazyagents cria o link no diretório próprio dele (`~/.codex/skills`, `~/.gemini/skills`…). Somente quando todos os agentes instalados que leem o diretório compartilhado têm a skill ativada é criado um único link ali. Desativar para um deles move os links de volta aos diretórios próprios dos demais. A exceção é `~/.claude/skills`, diretório do Claude Code também lido por OpenCode e Crush: uma skill ativada no Claude Code também aparece neles.

**Marcadores da matriz.** Cada célula indica o estado da skill em um agente:

| Marcador | Significado |
|---|---|
| `●` | ativada pelo lazyagents, com link no diretório próprio deste agente |
| `◆` | visível pelo diretório de outro agente, como OpenCode lendo `~/.claude/skills`; não pode ser alternada apenas neste agente |
| `▪` | local: conteúdo real não gerenciado pelo lazyagents |
| `○` | não visível para este agente |

**Perfis.** Um perfil salva um estado nomeado da matriz: quais skills da biblioteca estão ativadas em quais agentes. Aplicá-lo restaura esse estado, por exemplo para alternar entre conjuntos de trabalho e pessoais.

**Backups.** Antes de remover, substituir, restaurar sobre, incorporar ou migrar uma pasta de skill, o lazyagents salva um `.tar.gz` em `~/.local/share/lazyagents/backups/`. São mantidos os 20 backups mais recentes de cada skill.

<a id="in-the-tui"></a>

## Na TUI

A matriz tem uma linha por skill e uma coluna por agente instalado com diretório de skills. `←/→` escolhe a coluna. O detalhe da skill selecionada (descrição, origem, estado por agente e avisos de validação) aparece ao lado ou abaixo da tabela. A lista completa de teclas está na [referência de teclas](../reference/keys.md#skills-tab); `?` a mostra na TUI.

<a id="browse-and-read"></a>

### Navegar e ler

Percorra o inventário e digite `/` para buscar nome, descrição, pasta/caminho e origem registrada, incluindo repositório e subpasta de origem. A busca é aproximada e não distingue maiúsculas/minúsculas; por exemplo, procure uma finalidade como `database` ou um repositório como `everything-claude-code`. `enter` mantém a consulta, inclusive sem resultados; `esc` limpa o texto e preserva os filtros de estado. `enter` abre o `SKILL.md` renderizado como Markdown; no leitor, `e` abre o arquivo no editor e `esc` volta. `r` verifica novamente a biblioteca e os diretórios dos agentes. O contador da aba cobre todo o inventário; a matriz também conta as linhas locais visíveis fora da biblioteca.

<a id="combine-text-and-state-filters"></a>

### Combinar texto e filtros de estado

Pressione `f` para escolher um conjunto pequeno e fixo de filtros. `↑/↓` seleciona o campo e `←/→` ou `space` alterna o valor. Localização inclui todo o inventário, só biblioteca ou só skills locais. Agente inclui qualquer estado de ativação, ativada em algum agente, em nenhum ou em um agente instalado específico; visibilidade local e compartilhada também conta como ativada. Metadados podem incluir tudo ou apenas frontmatter inválido/problemas de validação. Atualizações podem incluir tudo, disponíveis, editadas localmente, em dia ou não verificadas/desconhecidas.

`enter` aplica esses filtros junto com o texto; `esc` cancela o rascunho. `r` no painel redefine os filtros de estado do rascunho, preservando o texto. `F` na matriz limpa texto e filtros de estado. O cabeçalho mostra a contagem visível/total e os filtros de estado ativos. A seleção acompanha a mesma pasta se ela continuar visível, inclusive quando nomes de exibição se repetem; caso contrário, vai para uma linha visível válida. Um resultado vazio não tem seleção acionável. Os detalhes sempre acompanham essa linha.

Os filtros usam apenas a varredura e a validação já carregadas: digitar ou alterar filtros não lê arquivos, clona repositórios nem faz requisições de rede. `r` na matriz atualiza metadados/ativação preservando os filtros. Filtros de atualização usam a última verificação explícita com `U`, que pode estar desatualizada; não verificada/desconhecida também inclui skills sem uma verificação Git utilizável. Atualizar uma skill invalida os estados de atualização armazenados; use `U` novamente quando necessário. Um filtro de atualizações vazio não comprova que o inventário está em dia.

Filtros alteram as linhas visíveis e descartam marcas ocultas. Use o [fluxo de linhas marcadas](#act-on-selected-rows) para ações em lote com filtros. `space`, `a`, `x`, `u` e `d` continuam atuando na skill visível selecionada. `A` (incorporar todas), `U` (verificar/atualizar todas de origem Git) e operações de perfis mantêm o escopo de todo o inventário, conforme suas confirmações existentes. Veja [perfis](#profiles) antes de salvar ou aplicar uma matriz.

<a id="act-on-selected-rows"></a>

### Ações nas linhas marcadas

Use `m` ou clique na caixa de uma linha para marcar sua pasta. `v` substitui a seleção por todas as linhas que correspondem ao texto e aos filtros de estado atuais, inclusive fora da tela; `V` limpa as marcas. O cabeçalho distingue contagens de marcadas, visíveis e totais. Alterar filtros ou recarregar desmarca automaticamente linhas ocultas ou removidas; limpar filtros não recupera essas marcas. Para selecionar todo o inventário, limpe os filtros com `F` e use `v`.

Pressione `c` para abrir **Selected skill actions**. Escolha ativar, desativar, atualizar ou remover. Na ativação, o destino inicial é a coluna de agente atual: alterne agentes individuais no painel, selecione todos com `a` ou nenhum com `x`. É necessário pelo menos um destino. `enter` abre uma confirmação listando todas as pastas canônicas, as contagens de marcadas/visíveis/totais e os agentes de destino. As pastas e os caminhos dos agentes revisados ficam fixos; uma varredura posterior não acrescenta linhas nem destinos. Cancelar volta ao rascunho sem gravar. Os atalhos de linha única continuam atuando em uma só linha mesmo com outras marcadas.

Ativar/desativar aplica apenas aos pares skill/agente listados, preservando os demais estados de ativação onde os leitores nativos permitem. Links compartilhados podem ser movidos para manter a visibilidade; um agente que também lê a pasta própria de outro ainda pode enxergar suas skills, como explicado em [diretórios compartilhados](#concepts). Pastas locais e links externos continuam protegidos.

Atualizar e remover atuam nas **cópias da biblioteca** selecionadas, afetando todos os agentes que leem essas cópias, em vez de um subconjunto de agentes. A confirmação explicita esse alcance. Atualizações buscam as origens Git revisadas e usam a operação existente de backup/substituição; edições locais, hashes de instalação ausentes, skills sem Git e links externos na biblioteca são ignorados. Uma origem registrada que mudou exige nova revisão. A remoção arquiva cada cópia antes de apagá-la e limpar seus links gerenciados; uma falha na limpeza dos links mantém a cópia na biblioteca para nova tentativa. Nenhuma dessas operações apaga cópias locais/externas dos agentes. O [relatório em lote](#bulk-results) persistente identifica cada par skill/agente, ou operação skill/biblioteca, e repete apenas as entradas com falha.

<a id="enable-and-disable"></a>

### Ativar e desativar

- `space` alterna a skill selecionada no agente sob o cursor (`←/→` escolhe a coluna).
- `a` ativa em todos os agentes instalados com diretório de skills; `x` desativa onde o lazyagents a gerencia.

Uma célula `▪` ou `◆` não pode ser alternada pela coluna: o aviso explica o motivo. Incorpore uma skill local com `o`, ou use `x` se ela chega ao agente pelo diretório de outro. Só skills da biblioteca podem ser ativadas.

<a id="install-from-github-a-folder-or-a-zip"></a>

### Instalar do GitHub, de uma pasta ou de um ZIP

Pressione `i` e informe a origem:

- `owner/repo` ou uma URL Git (`https://…`, `git@…`, `….git`): clonada com `git clone --depth 1` numa pasta temporária; exige `git` no `PATH`.
- Caminho de um arquivo `.zip`.
- Pasta local (`~` é expandido).

O lazyagents procura pastas com `SKILL.md` em qualquer profundidade. Se a raiz tiver um, ela própria é a skill. `.git`, `node_modules` e `vendor` são ignorados. Para nomes repetidos, vence a pasta fora de diretórios ocultos e mais próxima da raiz. O seletor mostra os itens encontrados mesmo quando há uma única skill. Skills começam marcadas e hooks desmarcados: `space` alterna um item, enquanto `a` alterna apenas skills e preserva a escolha individual de cada hook. `enter` avança para destinos; `esc` cancela e limpa o clone ou ZIP temporário.

A etapa de destinos oferece **Library only** (somente biblioteca) ou agentes instalados explícitos. Use `space` para alternar um destino, `a` para todos/nenhum e `0` para somente biblioteca. `tab` alterna entre itens e destinos; `esc` nos destinos volta aos itens. `enter` revisa as skills/hooks exatas e os nomes dos agentes, e pede confirmação com No selecionado por padrão. Cancelar a confirmação volta aos destinos sem gravar arquivos nem preferências. As etapas ocupadas de descoberta/instalação bloqueiam envios duplicados.

A primeira instalação usa somente biblioteca por padrão. Uma instalação de skills confirmada lembra os IDs explícitos em `skills.installTargets` no `config.yaml`; instalações seguintes pré-selecionam apenas os que continuam instalados e compatíveis. IDs salvos indisponíveis geram avisos na origem e não expandem a escolha para outros agentes. Escolher somente biblioteca salva explicitamente uma lista vazia. Importações só de hooks preservam a preferência de skills. Essa preferência não se aplica à CLI nem a ações em lote.

A instalação copia as skills escolhidas para a biblioteca e registra a origem em `.origin.json`, depois ativa apenas as recém-instaladas nos destinos revisados pela operação existente de distribuição de links. Somente biblioteca não cria links de ativação; agentes que leem a pasta da biblioteca diretamente continuam vendo seu conteúdo. Leitores nativos têm as mesmas [restrições de diretórios compartilhados](#concepts) da matriz. Um nome já presente na biblioteca falha sem sobrescrever nem ativar essa cópia existente; os demais itens selecionados continuam.

O relatório persistente separa resultados de `install`, `import hook` e ativação skill/agente. Uma falha de ativação mantém os arquivos instalados; `r` confirma repetir apenas pares de ativação ou gravações de preferência com falha, sem baixar/copiar a origem nem importar hooks novamente. Falhas de cópia/importação continuam visíveis e exigem corrigir a origem/colisão e iniciar outra instalação. Falhas ao salvar preferências não desfazem a instalação nem a ativação, e configurações malformadas são preservadas. Dispense o relatório para ajustar a ativação na matriz.

**Marketplaces.** Se a origem tiver um manifesto de marketplace do Claude Code (`.claude-plugin/marketplace.json`) declarando skills, o lazyagents usa o manifesto em vez da varredura. O seletor prefixa cada skill com o nome do plugin. Plugins vindos de outro repositório ou pacote não são buscados; aparecem como notas.

**Hooks na mesma origem.** Se houver `hooks/hooks.json` no formato de plugin do Claude Code, os hooks aparecem no mesmo seletor, desmarcados: um hook executa comandos de terceiros a cada evento do agente. Marque cada hook individualmente. Hooks selecionados são apenas importados para a biblioteca de hooks; os destinos de skills nunca os ativam nos agentes. Veja o [guia de hooks](hooks.md).

<a id="search-github"></a>

### Buscar no GitHub

`S` busca arquivos `SKILL.md` com seu termo no código do GitHub e lista os repositórios com descrições. `enter` inicia a instalação do repositório escolhido. A busca usa `gh api`, portanto exige a [CLI do GitHub](https://cli.github.com/) instalada e autenticada. Ela só ocorre quando você solicita.

<a id="create-and-edit"></a>

### Criar e editar

`n` pede um nome em kebab-case (letras minúsculas, dígitos e hífens, até 64 caracteres) e cria `<biblioteca>/<nome>/SKILL.md` a partir de um modelo com `name` e `description` preenchidos. `e` abre o arquivo da skill selecionada em `$EDITOR` (`vi` se não definido), suspendendo a TUI até o editor sair.

<a id="adopt-local-skills"></a>

### Incorporar skills locais

`o` incorpora a skill local selecionada: copia a pasta à biblioteca, faz backup da original e a substitui por um link simbólico para a cópia. O agente continua vendo a skill; outros agentes passam a poder ativá-la. `A` incorpora todas após uma confirmação que as lista.

Não é possível incorporar um link criado por outra ferramenta: gerencie-o com a ferramenta que o criou. Nomes já presentes na biblioteca também são recusados.

<a id="update-from-source"></a>

### Atualizar a partir da origem

Skills instaladas de origens Git podem ser atualizadas. Skills de ZIP, pasta ou criadas manualmente não podem.

- `u` atualiza a selecionada após confirmação: clona novamente o repositório, faz backup da pasta atual e substitui seu conteúdo no mesmo lugar, mantendo os links dos agentes.
- `U` verifica todas as skills de origem Git, clonando cada repositório uma vez, e marca a matriz: `↑` atualização disponível, `~` edição local. Se houver atualizações, pede para aplicar todas. Skills editadas localmente são ignoradas para preservar suas mudanças; `u` permite atualizar uma delas mesmo assim, com backup.

As edições locais são detectadas comparando o conteúdo com o hash salvo na instalação ou atualização.

<a id="profiles"></a>

### Perfis

`p` abre a lista de perfis:

- `s` salva a matriz atual. Apenas ativações controladas pelo lazyagents são registradas; skills locais e visíveis por diretórios de outros agentes ficam de fora.
- `enter` mostra as mudanças da aplicação (`+` ativar, `−` desativar, por skill e agente) e pede confirmação.
- `d` apaga o perfil após confirmação. As skills permanecem como estão.

Aplicar ativa as skills listadas nos agentes indicados e desativa as outras skills da biblioteca onde o lazyagents as ativou. Skills locais nunca são tocadas. Um perfil que cita uma skill ausente da biblioteca é recusado.

Os perfis ficam em `~/.local/share/lazyagents/profiles.json`.

<a id="backups-and-restore"></a>

### Backups e restauração

`B` ou `: skills backups` abre todos os backups de skills, dos mais novos aos mais antigos, incluindo skills removidas da biblioteca. `b` abre os backups da skill selecionada; sem seleção, também abre todos. Isso funciona após remover a última skill e reabrir uma biblioteca vazia.

Cada item mostra o nome original da pasta e a data local. Pressione `/` para filtrar por nome ou data (`YYYY-MM-DD`); todos os termos da busca devem corresponder, sem diferenciar maiúsculas e minúsculas. `enter` aplica o filtro; outro `enter` pede confirmação para restaurar o arquivo selecionado. `esc` durante a edição limpa o filtro; fora dela, volta a Skills. Cancelar uma restauração mantém a lista filtrada aberta.

A pasta atual, se existir, recebe backup antes da substituição, permitindo desfazer a restauração. O backup é extraído e verificado numa pasta temporária antes da troca; um backup corrompido mantém a skill intacta. Restaurar recupera apenas arquivos da biblioteca: links existentes nos agentes continuam funcionando, mas links removidos ao excluir uma skill não são recriados. Ative a skill restaurada explicitamente nos agentes desejados.

O CLI oferece o mesmo inventário em [`backups`](../reference/cli.md#backups), incluindo saída JSON, e restaura pelo nome exato de arquivo ou caminho listado em [`restore`](../reference/cli.md#restore).

<a id="skillmd-lint"></a>

### Validação de SKILL.md

Cada skill é validada contra o formato [Agent Skills](https://agentskills.io). Problemas aparecem nos detalhes e em `lazyagents doctor`:

- `name` deve existir, estar em kebab-case e coincidir com o nome da pasta;
- `description` deve ser não vazia e ter até 1024 caracteres;
- deve haver instruções depois do front matter.

Um front matter que não pode ser interpretado torna o `SKILL.md` inválido.

<a id="from-the-cli"></a>

## Pela CLI

A CLI cobre as operações diárias. Cada comando é descrito na [referência da CLI](../reference/cli.md#list); `lazyagents help <comando>` imprime a ajuda correspondente.

```sh
lazyagents install anthropics/skills             # instalar todas as skills do repositório
lazyagents install ./my-skills.zip --hooks       # instalar também os hooks incluídos
lazyagents install anthropics/skills pdf docx    # apenas estas skills
lazyagents install DietrichGebert/ponytail --all # instalar e ativar em todos os agentes
lazyagents list                                  # skills, número de agentes, biblioteca ou local
lazyagents enable pdf --agent claude-code        # ativar em um agente
lazyagents disable pdf --agent codex
lazyagents enable pdf                            # todos os agentes instalados, como a na TUI
lazyagents adopt my-notes --agent claude-code    # incorporar uma skill local
lazyagents remove pdf                            # remover da biblioteca, com backup
lazyagents skills backups --query pdf            # encontrar backups mesmo após remover
lazyagents skills backups --json
lazyagents skills restore pdf.20261017T120000.tar.gz  # usar o nome exato listado em backups
lazyagents list --json | jq '.[] | select(.valid | not) | .dir'
```

`lazyagents skills <comando>` é um alias dos mesmos comandos. Busca de novas skills, criação, atualização e perfis ainda estão disponíveis só na TUI.

<a id="configuration"></a>

## Configuração

```yaml
# ~/.config/lazyagents/config.yaml
libraryDir: ~/.agents/skills
```

`libraryDir` altera a localização da biblioteca. Defini-lo manualmente só muda onde o lazyagents procura; as skills existentes e os links continuam no lugar antigo. Para movê-los também, use [`lazyagents migrate-library <dir>`](../reference/cli.md#migrate-library): faz backup e copia cada skill, redireciona os links, salva `libraryDir` e só então remove as cópias antigas. O comando recusa nomes já existentes no destino (incluindo links) ou diretórios contidos um no outro. Resolva o conflito e execute novamente.

Usar `~/.agents/skills` como biblioteca torna todas as skills visíveis para Codex, Gemini CLI, OpenCode, Pi e Crush sem links, pois eles leem essa pasta. Em troca, perde-se a ativação por agente. Outras opções estão no [guia de configuração](../configuration.md).

<a id="files"></a>

## Arquivos

| Caminho | Quando é escrito |
|---|---|
| `~/.local/share/lazyagents/skills/<name>/` | instalação, criação, incorporação, atualização, restauração e remoção |
| `~/.local/share/lazyagents/skills/<name>/.origin.json` | instalação e atualização: tipo de origem, URL ou caminho, subpasta e hash |
| diretório de skills de cada agente | ativação e desativação (só links), incorporação (original vira link) e remoção (links à biblioteca) |
| `~/.local/share/lazyagents/backups/<name>.<timestamp>.tar.gz` | remoção, atualização, incorporação, restauração e migração; modo 0600, 20 por skill |
| `~/.local/share/lazyagents/profiles.json` | ao salvar um perfil |
| `~/.config/lazyagents/config.yaml` | `migrate-library`, no campo `libraryDir` |

Clones Git e ZIPs extraídos usam pastas temporárias removidas depois da instalação.

<a id="limits"></a>

## Limitações

- Links simbólicos dentro de uma skill não são copiados na instalação, incorporação ou migração; uma skill não pode importar arquivos externos à própria pasta. Entradas de ZIP que são links ou escapam da raiz são ignoradas. Um ZIP acima dos limites falha antes de chegar à biblioteca: entrada maior que 64 MB, mais de 512 MB descompactados no total ou mais de 10.000 entradas.
- A ativação usa links simbólicos. No Windows, pode exigir Modo de Desenvolvedor ou direitos de administrador.
- Os diretórios de skills por projeto do Hermes Agent (`<projeto>/.hermes/skills`, após `hermes skills trust`) não são lidos; `external_dirs` são.
- Skills incluídas em plugins do Claude Code não são gerenciadas.
- A TUI não monitora o sistema de arquivos: pressione `r` após mudanças manuais.

<a id="bulk-results"></a>

## Resultados em lote

Ativar/desativar uma skill em todos os agentes e aplicar um perfil de skills mostra resultados `succeeded`, `failed` e `skipped` por agente. Cópias locais ou externas são preservadas, agentes indisponíveis são ignorados e o estado é relido após cada tentativa. Alguns links podem já ter mudado quando outro caminho falha; links antigos permanecem até ser possível criar seus substitutos com segurança.

O relatório permanece até ser dispensado. Role o conteúdo, use `r` para confirmar uma nova tentativa apenas nos IDs com falha, `b` para abrir backups de skills ou `esc`/`enter` para voltar à matriz atualizada. A repetição relê o estado atual e o perfil salvo. A distribuição de links compartilhados continua conjunta para preservar a visibilidade dos outros agentes; pode mover links compartilhados mantendo o estado solicitado. Consulte a [referência de teclas](../reference/keys.md#skills-tab).

Ativar/desativar em lote e `install --all` pela CLI imprime o resultado de cada agente em stderr, preservando códigos de saída e campos JSON das listagens. Corrija a falha e use `enable <skill> --agent <id-com-falha>` ou `disable <skill> --agent <id-com-falha>` para repetir apenas nesse agente. Para `install --all`, repita a ativação com `enable`, sem reinstalar a skill. A ativação só altera links: [arquivos de backup](#backups-and-restore) restauram o conteúdo da biblioteca, enquanto `space` ou enable/disable ajusta a visibilidade.

O inventário também aceita pastas legadas com pontos no nome, preservando o nome original durante a restauração.
