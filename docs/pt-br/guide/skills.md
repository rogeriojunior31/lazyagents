# Skills
<!-- source: f257b4ec7d71 -->

Uma skill é uma pasta com um `SKILL.md`: instruções que um agente carrega quando a tarefa corresponde à descrição. Cada agente lê skills do próprio diretório, por isso a mesma skill costuma acabar copiada em vários lugares. O lazyagents mantém uma única cópia numa biblioteca central e a disponibiliza aos agentes por links simbólicos. Você instala e atualiza uma vez e escolhe em quais agentes ativar. A aba Skills mostra uma matriz de skill × agente; as mesmas operações estão disponíveis na CLI.

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

Percorra a lista e digite `/` para filtrar por nome. `enter` abre o `SKILL.md` renderizado como Markdown; no leitor, `e` abre o arquivo no editor e `esc` volta. `r` verifica novamente a biblioteca e os diretórios dos agentes. O título da aba conta as skills locais ainda fora da biblioteca.

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

O lazyagents procura pastas com `SKILL.md` em qualquer profundidade. Se a raiz tiver um, ela própria é a skill. `.git`, `node_modules` e `vendor` são ignorados. Para nomes repetidos, vence a pasta fora de diretórios ocultos e mais próxima da raiz. O seletor mostra as skills encontradas, inicialmente todas marcadas: `space` alterna uma, `a` alterna todas, `enter` instala e `esc` cancela.

A instalação só copia para a biblioteca e registra a origem em `.origin.json` na pasta da skill. Nada é ativado automaticamente. Um nome já existente não é sobrescrito: esse item falha, mas a instalação dos demais continua.

**Marketplaces.** Se a origem tiver um manifesto de marketplace do Claude Code (`.claude-plugin/marketplace.json`) declarando skills, o lazyagents usa o manifesto em vez da varredura. O seletor prefixa cada skill com o nome do plugin. Plugins vindos de outro repositório ou pacote não são buscados; aparecem como notas.

**Hooks na mesma origem.** Se houver `hooks/hooks.json` no formato de plugin do Claude Code, os hooks aparecem no mesmo seletor, desmarcados: um hook executa comandos de terceiros a cada evento do agente. Veja o [guia de hooks](hooks.md).

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

`b` lista os backups da skill selecionada, dos mais novos aos mais antigos. `enter` restaura após confirmação. A pasta atual, se existir, recebe backup antes da substituição, permitindo desfazer a restauração. O backup é extraído e verificado numa pasta temporária antes da troca; um backup corrompido mantém a skill intacta.

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
lazyagents list --json | jq '.[] | select(.valid | not) | .dir'
```

`lazyagents skills <comando>` é um alias dos mesmos comandos. Busca, criação, atualização, perfis e restauração ainda estão disponíveis só na TUI.

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
