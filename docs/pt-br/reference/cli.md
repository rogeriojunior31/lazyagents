# Referência da CLI
<!-- source: 03e424dbfb86 -->

Cada comando imprime ajuda com `lazyagents help <comando>`. Sem argumentos, `lazyagents` abre a TUI. Plugins externos acrescentam comandos com seus IDs, descritos na ajuda após a instalação ([guia](../guide/plugins.md)). Esta página traduz a referência: comandos, opções, nomes de campos e colunas permanecem literais; a saída do programa continua em inglês.

Código de saída `0` indica sucesso; códigos diferentes de zero indicam falha.

| Comando | Função |
|---|---|
| [`list`](#list) | listar skills e quantos agentes as veem |
| [`enable`](#enable) | ativar skill da biblioteca por link simbólico |
| [`disable`](#disable) | desativar removendo o link |
| [`install`](#install) | instalar do GitHub, ZIP ou pasta |
| [`remove`](#remove) | remover da biblioteca |
| [`backups`](#backups) | listar backups de skills, incluindo removidas |
| [`restore`](#restore) | restaurar um backup de skill na biblioteca |
| [`adopt`](#adopt) | incorporar skill local de agente |
| [`migrate-library`](#migrate-library) | mover biblioteca |
| [`skills`](#skills) | gerenciar skills, varreduras de origem, atualizações revisadas e perfis |
| [`sessions`](#sessions) | listar sessões, recentes primeiro |
| [`provider`](#provider) | gerenciar perfis de endpoint e autenticação |
| [`hooks`](#hooks) | listar, instalar e desinstalar hooks |
| [`history`](#history) | inspecionar tentativas locais recentes sem repeti-las |
| [`config-backups`](#config-backups) | revisar e recuperar backups de configurações dos agentes |
| [`usage`](#usage) | limites de assinatura, tokens e custo por dia/agente/projeto/modelo |
| [`doctor`](#doctor) | diagnosticar agentes, skills, hooks, provedores, consumo e plugins |

## list

```text
lazyagents list [--json]
```

Lista skills da biblioteca e skills locais encontradas nos diretórios dos agentes, com a quantidade de agentes que as veem. `ENABLED IN` conta os agentes; `LIBRARY` indica gerenciamento pelo lazyagents.

| Opção | Efeito |
|---|---|
| `--json` | cada skill com origem, estado da validação e estado por agente |

## enable

```text
lazyagents enable <skill> [--agent id|--all]
```

Ativa uma skill da biblioteca criando link no diretório do agente. `<skill>` aceita nome da pasta ou do front matter. Agentes que já a veem ficam como estão. Skills locais precisam ser incorporadas antes.

| Opção | Efeito |
|---|---|
| `--agent id` | só esse agente (`claude-code`, `codex`, `gemini-cli`…) |
| `--all` | todos os instalados com diretório de skills; padrão |

## disable

```text
lazyagents disable <skill> [--agent id|--all]
```

Remove o link do agente, mantendo a skill na biblioteca e nos demais. Links únicos em `~/.agents/skills` são movidos aos diretórios próprios dos agentes que mantêm a ativação. Skills locais (pastas reais ou links de outra ferramenta) nunca são apagadas; o comando falha para elas.

| Opção | Efeito |
|---|---|
| `--agent id` | só esse agente |
| `--all` | todos onde o lazyagents ativou; padrão |

## install

```text
lazyagents install <source> [skill...] [--all] [--hooks]
```

Instala na biblioteca pastas com `SKILL.md` encontradas em qualquer profundidade. Marketplaces Claude Code (`.claude-plugin/marketplace.json`) são lidos pelo manifesto. Nomes após a origem restringem a instalação; nome desconhecido instala nada e lista as opções. Nomes já na biblioteca são ignorados com aviso. Sem `--all`, não ativa: use `enable`. Sem `--hooks`, apenas conta hooks, pois executam comandos de terceiros a cada evento.

| Origem | Significado |
|---|---|
| `owner/repo` | repositório GitHub, exige `git` no `PATH` |
| `https://…`, `git@…`, `*.git` | URL Git, clonada com `--depth 1` |
| `./skills.zip` | arquivo ZIP |
| `~/some/folder` | pasta local |

| Opção | Efeito |
|---|---|
| `--all` | ativar skills instaladas em todos os agentes com diretório, como `enable --all` |
| `--hooks` | instalar também hooks de plugins em `hooks/hooks.json` |

## remove

```text
lazyagents remove <skill>
```

Remove da biblioteca e apaga links criados pelo lazyagents que apontam para ela. Antes, faz backup `.tar.gz`; `B` em Skills ou o comando `restore` recupera os arquivos. Skills locais não estão na biblioteca e não podem ser removidas aqui.

## backups

```text
lazyagents backups [--query text] [--json]
```

Lista todos os arquivos de backup de skills, dos mais novos aos mais antigos, mesmo com a biblioteca vazia. A busca usa o nome original da pasta e a data local; todos os termos precisam corresponder, sem diferenciar maiúsculas e minúsculas. Para restaurar um arquivo exato, use `lazyagents restore <backup>`. Este comando não lista backups de configurações de agentes.

| Opção | Efeito |
|---|---|
| `--query text` | filtrar pelo nome original da pasta ou data local (`YYYY-MM-DD`) |
| `--json` | array JSON com `skill`, `time` e `path` |

## restore

```text
lazyagents restore <backup>
```

Aceita o nome exato do arquivo ou caminho listado por `lazyagents backups`. Restaura a pasta original na biblioteca atual, mesmo quando vazia. Arquivos existentes são substituídos após um backup de segurança. A extração ocorre numa pasta temporária antes da troca, então um arquivo corrompido preserva os dados atuais.

As ativações nos agentes permanecem como estão: links removidos não são recriados. Use `lazyagents enable <skill>` para reativar a skill restaurada.

## adopt

```text
lazyagents adopt <skill> --agent <id>
```

Copia a pasta local à biblioteca, faz backup da original e a substitui por link. O agente continua vendo a skill e outros podem ativá-la. Links de outras ferramentas são recusados; gerencie-os na ferramenta original.

| Opção | Efeito |
|---|---|
| `--agent id` | agente com a cópia local; obrigatória |

## migrate-library

```text
lazyagents migrate-library <dir>
```

Copia todas as skills para `<dir>`, redireciona links dos agentes, salva `libraryDir` em `config.yaml` e só depois remove cópias antigas, com backup prévio de cada skill. Recusa nomes já existentes no destino ou diretórios contidos um no outro. Em falha, links e configuração permanecem como antes.

Exemplo: `lazyagents migrate-library ~/.agents/skills`.

## skills

```text
lazyagents skills list|enable|disable|install|remove|backups|restore|adopt|migrate-library|sources|updates|profiles …
```

Gerenciar skills, varreduras de origem, atualizações revisadas e perfis de ativação. Os comandos de biblioteca mantêm os aliases existentes; sem subcomando, executa list. Os comandos de manutenção abaixo existem apenas dentro de skills. Use `lazyagents skills help <subcomando>` para ajuda específica.

```text
sources list|scan <name> [--json]
```

Listar cadastros salvos ou verificar um por solicitação. `list` é o padrão, sem rede: retorna nomes, URLs, refs e subpastas. `scan <nome>` clona apenas a origem cadastrada e lista entradas relativas ao repositório. `--json` retorna formato estável; varreduras incluem source, revision, entries e notes. Usa branch/tag e subpasta salvas, sem instalar, ativar ou editar configuração. Nomes de destino repetidos mantêm caminhos distintos. O seletor da TUI instala a origem com proveniência de caminho exato. Interromper encerra Git/filhos e limpa o staging.

```text
updates check [directory...]|apply <directory...> [--review token] [--replace-local] [--json]
```

Verificar e aplicar atualizações Git com proteção contra revisões antigas. `check` revisa pastas exatas; sem seleção, verifica todas as skills Git da biblioteca. `apply` exige seleção explícita. Seletores são pastas canônicas de `skills list`, não aliases de exibição; nomes desconhecidos/repetidos e argumentos extras são recusados antes de gravar. O check retorna um identificador para o escopo inteiro e outro para cada item verificado, permitindo selecionar um item de uma verificação global. Aplicar prepara novamente a origem e verifica a identidade; mudanças locais, de origem ou de commit remoto exigem novo check. Compartilha revisão/diff, backup, caminho exato e substituição com a TUI.

- `--json`: check retorna review/items com estados estáveis, origem, diff e hashes; apply retorna targets com id/outcome/detail/state e error opcional.
- `--review token`: apply exige a identidade do check para essa seleção exata.
- `--replace-local`: apply permite sobrescrever edições/base ausente de exatamente uma pasta; não ignora revisão antiga.

O lote preserva edições locais e bases ausentes. Interrupções mantêm itens concluídos e seus backups; itens posteriores ficam intactos. Falhas exigem nova verificação. Nenhum desses comandos altera links de ativação ou preferências de destinos da instalação.

```text
profiles list|show <name>|save <name>|preview <name>|apply <name>|remove <name> [--review token] [--json]
```

Salvar, inspecionar e aplicar perfis de ativação de skills. `list` é o padrão offline. `show` inspeciona a especificação de pasta canônica para agentes. `save` salva/sobrescreve a matriz gerenciada atual. `preview` compara o perfil salvo com a matriz e os destinos atuais. `apply` aplica o perfil revisado à matriz inteira. `remove` apaga só o perfil salvo, sem alterar ativação.

`--json` retorna nomes, especificação, prévia ou resultado em formato estável. `--review token` é obrigatório ao aplicar; mudanças após a prévia são recusadas. Salvar usa o mesmo snapshot dos links gerenciados da TUI, excluindo cópias locais/externas e visibilidade compartilhada não controlável. Aplicar mantém o escopo completo de inventário/agentes dos perfis da TUI, inclusive desativando skills gerenciadas omitidas. A prévia lista pastas canônicas, nomes de exibição, ativações/desativações e a especificação salva, inclusive IDs indisponíveis. Restrições de leitores compartilhados e preservação local/externa continuam valendo. Listas de perfil legadas permanecem legíveis; metadados JSON existentes sobrevivem a escritas. Remover nome ausente é idempotente; salvar/remover nunca altera links.

## sessions

```text
lazyagents sessions [--agent id] [--here] [--limit n] [--json]
```

Lista sessões, recentes primeiro. Saída textual usa `AGENT`, `TITLE`, `CWD` e `UPDATED`; aliases aparecem como `alias (title)`. JSON acrescenta ID e, quando registrado, consumo (`input`, `output`, `cache_read`, `cache_write`, `model`). Para chave de API, inclui `cost_usd` em modelos conhecidos; assinaturas não pagam por token. Para retomar, abra a TUI.

| Opção | Efeito |
|---|---|
| `--agent id` | só esse agente; ID desconhecido lista os válidos |
| `--here` | sessões iniciadas no diretório atual |
| `--limit n` | no máximo n; padrão 0, todas |
| `--json` | saída JSON |

## provider

```text
lazyagents provider list|validate <profile>|probe <profile>|models <profile>|apply <profile>|clear|adopt <profile>|add <profile>|rm <profile> [--agent id|--agents ids] [--preview] [--json] [--reveal]
```

Gerencia perfis de endpoint, modelo e autenticação.

| Subcomando | Efeito |
|---|---|
| `list` | perfis e configuração aplicada por agente; padrão |
| `validate <profile>` | verificar os destinos selecionados sem gravar ou acessar a rede |
| `probe <profile>` | enviar uma requisição HEAD sem credenciais à URL base salva |
| `models <profile>` | consultar catálogo OpenAI compatível usando credenciais do perfil |
| `apply <profile>` | validar todos os destinos selecionados e escrever o perfil |
| `clear` | remover o aplicado, preservando o restante |
| `adopt <profile>` | importar campos públicos do agente indicado por `--agent` para uma entrada nova na biblioteca |
| `add <profile>` | criar ou substituir perfil |
| `rm <profile>` | apagar da biblioteca; agentes mantêm o aplicado |

| Opção | Efeito |
|---|---|
| `--agent id` | limitar aplicação/validação/limpeza; padrão é todos os instalados compatíveis |
| `--agents ids` | aplicação/validação: seleção explícita de IDs separados por vírgula |
| `--preview` | aplicação/limpeza: revisar campos mascarados sem gravar |
| `--json` | listagem, validação, prévia, probe ou models em JSON |
| `--reveal` | mostrar tokens em texto puro na listagem |

Opções de `adopt`:

| Opção | Efeito |
|---|---|
| `--agent id` | agente de origem obrigatório; salva os campos públicos em um nome novo na biblioteca |
| `--token -` | digitar a credencial explicitamente via entrada padrão |
| `--env-key VAR` | substituir a referência de ambiente importada |

Opções de `add`:

| Opção | Efeito |
|---|---|
| `--base-url url` | endpoint compatível HTTP/HTTPS |
| `--model m` | modelo; vazio usa padrão do agente |
| `--token -` | ler da entrada padrão; valor literal funciona, mas fica no histórico do shell |
| `--env-key VAR` | variável com token para Codex, OpenCode, Pi e Crush |
| `--wire-api api` | Codex: só `responses`; Pi: `chat` padrão, `responses` ou `anthropic`; Crush: `chat` padrão ou `anthropic`; OpenCode: apenas `chat` |

Perfis ficam em `providers.json`, modo 0600. Tokens são mascarados em texto/JSON salvo `--reveal`. Aplicar/limpar faz backup do arquivo do agente. Codex nunca recebe token no arquivo: defina `--env-key` e exporte a variável. OpenCode/Pi/Crush exigem `--base-url` e `--model`. A validação verifica a configuração, não a autenticação: um nome de variável não comprova uma credencial funcional. A importação copia apenas campos públicos da conexão. Digite tokens nativos novamente com `--token -` ou forneça `--env-key`; login nativo e opções avançadas não são importados. Prévias de aplicação/limpeza ocultam todos os valores e nunca serializam as revisões privadas. Qualquer destino selecionado incompatível impede toda a aplicação antes de gravar. Use `--agent` ou `--agents` para escolher explicitamente só os compatíveis. Probe/models só executam sob demanda, com prazo de 10 segundos e sem redirecionamentos. Models usa primeiro o bearer salvo, depois envKey; nunca lê login nativo. Uma resposta comprova que o endpoint respondeu; um modelo listado não comprova inferência. List compara campos públicos conforme os padrões nativos e a referência/presença de credencial. Múltiplas correspondências continuam ambíguas; valores de tokens nunca são comparados. List --json acrescenta matches e comparações por perfil; profile exige correspondência única. List e doctor incluem diagnósticos de ambiente sem valores, com nomes de variáveis, origens e próximos passos. Apenas o processo atual do lazyagents e campos nativos suportados são inspecionados; sessões em execução, arquivos de inicialização do shell e helpers ficam fora da análise. OpenCode gerencia um único arquivo global com JSON estrito; comentários/vírgulas finais JSONC e múltiplos arquivos globais são recusados. Configurações de projeto, inline e gerenciadas podem sobrepor esse arquivo. Inicie uma nova sessão do agente para carregar as mudanças; sessões existentes podem manter a configuração anterior.

## hooks

```text
lazyagents hooks list|enable <name>|disable <name>|add <name>|rm <name> [--agent id] [--json]
```

Lista hooks da biblioteca e instala/desinstala nos agentes.

| Subcomando | Efeito |
|---|---|
| `list` | biblioteca e locais de instalação; padrão |
| `enable <name>` | instalar em agentes com os eventos necessários |
| `disable <name>` | desinstalar |
| `add <name>` | criar entrada com um comando |
| `rm <name>` | apagar da biblioteca; desinstale scripts importados primeiro |

| Opção | Efeito |
|---|---|
| `--agent id` | limitar ativação/desativação; padrão todos os instalados compatíveis |
| `--json` | listagem JSON |

Opções de `adopt`:

| Opção | Efeito |
|---|---|
| `--agent id` | agente de origem obrigatório; salva os campos públicos em um nome novo na biblioteca |
| `--token -` | digitar a credencial explicitamente via entrada padrão |
| `--env-key VAR` | substituir a referência de ambiente importada |

Opções de `add`:

| Opção | Efeito |
|---|---|
| `--event name` | evento, como `SessionStart`, `PreToolUse`, `Stop` |
| `--command text` | comando shell |
| `--matcher re` | filtro de evento, como ferramenta em `PreToolUse`; vazio aceita todos |
| `--timeout n` | segundos; 0 usa padrão do agente |
| `--desc text` | descrição na aba |

Hooks executam a cada evento; instalar faz backup primeiro, e hooks externos não são tocados. Codex só executa novo hook após confiança aprovada no próprio Codex; o lazyagents não a escreve. Importe hooks com `lazyagents install <source> --hooks`.

Apagar um hook inline da biblioteca mantém seus comandos instalados. Scripts importados exigem desinstalar o hook de todos os agentes primeiro; configurações que não puderem ser lidas bloqueiam a exclusão. Scripts compartilhados com outra entrada da biblioteca são preservados.

## config-backups

```text
lazyagents config-backups list|preview <id>|restore <id> [--query text] [--review token] [--json]
```

Inspeciona e recupera cópias de configurações dos agentes.

| Subcomando | Efeito |
|---|---|
| `list` | listar cópias de configuração, incluindo origens antigas desconhecidas; padrão |
| `preview <id>` | verificar origem/integridade e mostrar diferenças ocultadas entre estado atual e backup |
| `restore <id>` | restaurar este arquivo usando o token `--review` da prévia |

| Opção | Efeito |
|---|---|
| `--query text` | listagem: filtrar por agente, destino/arquivo, operação e data local |
| `--json` | listagem/prévia: saída JSON sem valores da configuração |
| `--review token` | restauração: token obrigatório da prévia exata revisada |

Use um ID exato da lista. Novos backups trazem origem registrada pelos adaptadores; cópias antigas sem metadados permanecem desconhecidas e não podem ser restauradas automaticamente. A prévia mostra campos/linhas alterados com todos os valores ocultos. Restaurar substitui o arquivo inteiro, incluindo alterações não relacionadas exibidas na revisão.

O estado atual recebe backup de segurança. Mudanças na origem, destino ou permissões exigem nova prévia; permissões mais restritas são preservadas. A recuperação é por arquivo: perfis, registros de propriedade de provedores e outros arquivos não são revertidos. Os dois arquivos do Pi exigem revisões separadas. Aprovações nativas de confiança/ativação de hooks do Codex podem impedir a restauração de `config.toml`. Arquivos de skills usam `skills backups/restore`.

## history

```text
lazyagents history list|inspect <id>|clear [--query text] [--json] [--yes]
```

Inspecionar tentativas locais recentes de gravação sem repeti-las.

```text
subcomandos:
  list                 listar tentativas recentes, novas primeiro (padrão)
  inspect <id>         inspecionar resultados e verificar backups de configuração relacionados
  clear --yes          limpar só o histórico; manter backups e ajustes atuais

opções:
  --query text         list: filtrar módulo, ação, recurso, destino, resultado ou data local
  --json               list/inspect: só metadados; sem erros brutos, corpos ou comandos
  --yes                clear: confirmar a remoção do histórico local

Mantém até 200 tentativas, 1000 destinos e 32 backups relacionados por tentativa,
dentro de 32 MiB. O arquivo acessível só pelo dono tem retenção própria.
Atualizações simultâneas são serializadas; falhas de persistência são relatadas.
O histórico não é um plano executável de repetição. Inspecione o estado atual e
revise novamente a operação no módulo. Backup de configuração exige nova prévia
config-backups e seu token de revisão; cópias ausentes/fora da retenção não podem
ser restauradas. Referências combinam horário, ação e destino; tentativas que
se sobrepõem podem apontar à mesma cópia. Nenhuma operação é repetida aqui.
```

## usage

```text
lazyagents usage [limits|daily|agents|projects|models] [--agent id] [--since 7d] [--limit n] [--refresh] [--json]
```

Mostra limites, tokens e custo por período/agente/projeto/modelo.

| Visualização | Conteúdo |
|---|---|
| sem subcomando | painel de limites, bloco atual de 5 horas e resumo do período |
| `limits` | janelas de sessão/semana com consumo e renovação |
| `daily` | tokens por dia |
| `agents` | tokens por agente |
| `projects` | tokens por projeto, usando pasta da sessão |
| `models` | tokens por modelo |

| Opção | Efeito |
|---|---|
| `--agent id` | só esse agente |
| `--since period` | início do período: `7d`, `24h` ou `2026-09-01`; padrão 7d |
| `--limit n` | linhas de agentes/projetos/modelos no texto; padrão 10, 0 todas |
| `--refresh` | ignorar cache de 5 minutos e consultar limites novamente |
| `--json` | JSON, não limitado por `--limit` |

Tokens vêm das conversas locais; limites vêm das contas dos agentes. Custos em USD só aparecem para chaves de API; assinaturas não pagam por token.

## doctor

```text
lazyagents doctor [--json] [--local]
```

Diagnostica agentes, skills, hooks, provedores, consumo e plugins.

| Seção na saída | Verificação |
|---|---|
| detected agents | ID e instalação de cada agente |
| configuration | sintaxe atual da configuração do lazyagents |
| skills | SKILL.md inválido/incompleto na biblioteca e agentes |
| symlinks | links quebrados nos diretórios lidos pelos agentes |
| providers | provedor aplicado; configuração em agente não instalado é problema |
| hooks | arquivos inválidos, executáveis ausentes/sem permissão e avisos por agente |
| usage | autenticação e limites; usa cache recente ou consulta novamente |
| plugins | arquivos ignorados, negociação inicial e diagnóstico próprio se declarado |

`--json` preserva `{ok, agents, sections[{title, report, problems}]}` e acrescenta `scope`, `cancelled` quando necessário e `findings`, `external`, `skipped`, `next_step` e `tab` opcional por seção. Findings identificam `severity`, `resource`, `explanation`, `next_step` e `tab` opcional. `--local` ignora Usage/rede e execução de plugins, identificando suas seções como não executadas. Sem a opção, Usage pode usar o cache ou buscar dados e plugins podem executar seu próprio doctor.

Diagnósticos nunca reparam configuração ou links de ativação; Usage pode atualizar o cache. Sai com `1` em caso de problema ou cancelamento, `0` quando os checks concluídos estão corretos. Checks externos ignorados não comprovam disponibilidade remota.
