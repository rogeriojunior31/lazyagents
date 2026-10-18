# Segurança e dados
<!-- source: 49292813bea7 -->

O lazyagents edita arquivos dos quais seus agentes dependem. Esta página informa exatamente o que muda, como desfazer e como segredos são tratados. Os guias de módulos detalham cada assunto.

<a id="principles"></a>

## Princípios

- **Nada muda sem uma ação sua.** Navegar, abrir a TUI ou consultar informações nunca escreve arquivos dos agentes.
- **Configurações são confirmadas.** Na TUI, aplicar provedor ou instalar hook mostra arquivo e mudanças antes de escrever. Skills só adicionam/removem links, por isso a ativação é imediata.
- **Backup antes de sobrescrever.** Configurações são copiadas antes de cada escrita; skills removidas e sessões excluídas são preservadas antes da remoção.
- **Só conteúdo gerenciado é alterado.** Chaves, hooks e seções TOML não escritos pelo lazyagents ficam intactos e na mesma ordem. No `env` do Claude Code, aplicação/limpeza só remove chaves próprias e restaura endpoint/modelo substituídos.
- **Escrita atômica.** Usa arquivo temporário e renomeação sobre o original, evitando arquivo parcial após falha.
- **Escritas concorrentes não são sobrescritas.** A configuração só é substituída se ainda corresponder à leitura. Se outro processo mudar, Codex (`config.toml`) e Pi (`models.json`, `settings.json`) são relidos e editados novamente, até três tentativas com um backup. Claude Code e hooks são deixados como o outro programa escreveu, com aviso. Remover arquivos criados pelo lazyagents segue a mesma regra.

<a id="what-lazyagents-writes"></a>

## O que o lazyagents escreve

Nos próprios diretórios ([caminhos](configuration.md#where-files-live)): bibliotecas de skills/hooks, perfis, aliases, caches, exportações e backups.

Nos diretórios dos agentes, apenas por sua ação:

| Ação | Mudança | Como desfazer |
|---|---|---|
| Ativar skill | link simbólico no diretório do agente para a biblioteca | desativar remove o link |
| Desativar skill | remove o link, nunca uma pasta real local ou de outra ferramenta | ativar novamente |
| Incorporar skill | pasta local vai à biblioteca; link toma seu lugar | copiar a pasta da biblioteca de volta |
| Aplicar provedor | Claude Code: `env` em `~/.claude/settings.json`. Codex: `model_provider` e bloco `[model_providers.lazyagents]` em `~/.codex/config.toml`. Pi: provedor `lazyagents` em `~/.pi/agent/models.json` e `defaultProvider`/`defaultModel` em `settings.json`. Crush: bloco delimitado ao fim de `~/.config/crush/crushrc`, com valores entre aspas simples | limpar o provedor ou restaurar backup |
| Instalar hook | entrada em `~/.claude/settings.json` ou `~/.codex/hooks.json`, ou linha `hook add` no bloco do `crushrc` | desinstalar ou restaurar backup |
| Excluir sessão | arquivo removido após cópia ao backup. OpenCode exporta com `opencode export` antes de `opencode session delete`; Crush salva com `crush session show --json` antes de `crush session delete`. Falha no backup impede exclusão | copiar backup de volta; OpenCode usa `opencode import <file>`; Crush não tem importação, backup é só registro |
| `migrate-library` | move skills, recria links e salva `libraryDir` em `config.yaml` | executar de volta ao diretório antigo |

Arquivos por agente estão na [referência](reference/agents.md#capabilities).

A aplicação de provedores valida todos os agentes selecionados antes da primeira gravação. Endpoint ou referência de credencial inválidos, protocolo/modelo incompatível, arquivos nativos ilegíveis, blocos gerenciados inválidos ou estruturas JSON não suportadas bloqueiam a seleção inteira sem criar backups ou registros de propriedade. Escolha explicitamente só os destinos compatíveis no seletor de Providers ou com `provider apply --agents id,id`. `provider validate` e a verificação do formulário são somente leitura e não acessam a rede; o nome de uma variável não comprova autenticação. Cada gravação relê o arquivo nativo; falhas posteriores de E/S ou concorrência ainda podem gerar alterações parciais relatadas por destino. Veja o [guia de provedores](guide/providers.md).

Templates de conexão apenas preenchem campos editáveis da biblioteca. Não buscam credenciais, autenticam, descobrem modelos nem aplicam configurações dos agentes. A opção de padrões exige selecionar agentes explicitamente e confirmar a limpeza existente: só ajustes de provedor gerenciados são removidos, provedores anteriores/externos podem permanecer e o login nativo continua no agente. Veja os [templates de conexão](guide/providers.md#connection-templates).

A importação de provedores lê apenas campos públicos representáveis da conexão e salva uma entrada nova na biblioteca, separadamente das gravações nativas. Tokens nativos exigem digitação explícita ou referência de ambiente compatível; arquivos de login e helpers não são copiados. Configuração executável do Crush nunca é avaliada. Prévias de aplicação/limpeza mostram ações por arquivo e nomes dos campos gerenciados, com todos os valores ocultos, usando a mesma edição da persistência. Consultar a prévia não cria backups nem registros de propriedade. Operações confirmadas comparam o perfil salvo, arquivos nativos e estado de propriedade com a revisão; mudanças bloqueiam todas as gravações selecionadas. Permanecem as escritas condicionais, os backups e o relato de falhas em execução por destino. Veja [importação e prévias](guide/providers.md#import-an-existing-connection).

A comparação de provedores usa apenas campos públicos nativos, referências de credencial e presença de token. Atualizar nunca lê valores de variáveis, autentica, grava configurações nativas nem escolhe entre perfis ambíguos. Mantenha a configuração atual deixando-a como está; a reaplicação explícita usa a mesma gravação revisada e com backup da aplicação. Correspondência não compara valores de tokens nem comprova identidade do perfil. Veja [comparação salvo/atual](guide/providers.md#compare-saved-and-live-settings).

## Backups

Tudo fica em `~/.local/share/lazyagents/backups/`, como `<file>.<timestamp>`:

| Conteúdo | Retenção |
|---|---|
| configurações (`settings.json`, `config.toml`, `hooks.json`, `models.json`) | 20 mais recentes por arquivo |
| skills removidas/atualizadas (`<skill>.<timestamp>.tar.gz`) | 20 mais recentes por skill |
| sessões excluídas (`<transcript file>.<timestamp>`, `opencode-<id>.<timestamp>.json`, `crush-<id>.<timestamp>.json`) | todas |

Backups de skills ficam disponíveis mesmo com a biblioteca vazia: `B` ou `: skills backups` na TUI, ou `lazyagents skills backups` no CLI. Filtre pelo nome original/data e restaure o arquivo selecionado com `enter` ou `lazyagents skills restore <backup>` ([guia](guide/skills.md#backups-and-restore)). Os arquivos atuais da biblioteca recebem backup de segurança antes da substituição; arquivos corrompidos os mantêm intactos. Restaurar arquivos preserva links existentes nos agentes e não recria ativações removidas. Backups de configuração são recuperados em **Recovery** ou por `config-backups`: revise as diferenças ocultadas entre estado atual e backup e confirme a substituição do arquivo inteiro. Os dados atuais recebem backup antes, revisões desatualizadas são recusadas e permissões mais restritas são preservadas. Cópias antigas sem origem comprovada permanecem desconhecidas e não podem ser restauradas automaticamente. Veja os arquivos compatíveis e limites de aprovação nativa no [guia de recuperação](guide/recovery.md).

```sh
lazyagents config-backups list --query 'claude-code settings.json'
```

<a id="secrets"></a>

<a id="operation-metadata"></a>

## Metadados de operações

Resultados concluídos de operações de provedores, hooks e skills e restaurações de configuração gravam `operation-history.json` com acesso exclusivo ao dono. Os metadados mantêm IDs/resultados, horário, nomes dos recursos e backups relacionados, sem erros brutos, corpos de configuração, detalhes/estado dos resultados, tokens, transcrições, URLs de origem ou comandos importados. A retenção é de 200 tentativas, 1000 destinos/32 referências por tentativa e 32 MiB; destinos omitidos são contabilizados. Atualizações/limpeza simultâneas usam bloqueio e gravação atômica condicional. Falha ao persistir o histórico não transforma destinos concluídos em alvos de repetição. Limpar o histórico com confirmação mantém backups e ajustes. Um resultado pode estar ausente se o processo parar antes da persistência; use o estado atual como evidência. Navegar no histórico não executa repetição; recuperar configuração exige validação atual do adaptador/integridade/destino, nova prévia com valores ocultos e confirmação. Veja [histórico e recuperação do bloqueio](guide/recovery.md#recent-operation-history).

## Segredos

- **Tokens de provedores:** ficam em `~/.config/lazyagents/providers.json`, modo 0600, preservado nos backups. TUI e `--json` só indicam presença; texto puro exige `lazyagents provider list --reveal`. Use `--token -` para ler da entrada padrão, fora do histórico do shell.
- **Codex:** prefira `--env-key VAR`; `config.toml` guarda apenas o nome da variável.
- **Claude Code:** token precisa estar em `settings.json`, onde o agente o lê. O lazyagents usa modo 0600 ao escrevê-lo.
- **Pi:** `--env-key VAR` evita token no arquivo (`"apiKey": "$VAR"`); caso contrário fica em `models.json`, modo 0600.
- **Credenciais dos agentes:** por exemplo a autenticação do Claude Code, só são lidas para a consulta que exige isso (limites), nunca guardadas, registradas ou exibidas. Para identificar o tipo, o programa lê apenas existência e tipo (`oauth`/`api_key` no `auth.json` do Pi, presença de chave em `models.json` do Pi ou `crush.json`), nunca o valor.

<a id="diagnostics"></a>

## Diagnósticos

O painel opcional `settings` da paleta edita somente o tema salvo e `tui.startTab`/`tabs`/`hidden`. Mantém a sessão atual até reiniciar e nunca grava configurações nativas dos agentes ou inicia plugins. O salvamento relê a configuração, preserva campos/comentários alheios, recusa preferências editadas sobre uma versão antiga e protege a gravação final contra mudanças concorrentes. Erros não citam a entrada do parser, e o painel não recebe corpos de configurações alheias nem credenciais. Escape descarta o rascunho não salvo; uma gravação local em andamento termina antes de fechar. Orientações de primeiro uso abrem os fluxos existentes de importação/origem/provedor, sem download, ativação ou gravações de onboarding automáticos.

Atualizar a detecção (`r` em Agents ou `agents refresh`) executa consultas locais de versão e atualiza evidências locais de skills/sessões/provedores/hooks. Não consulta limites de uso, executa checks de plugins nem importa o ambiente de outro shell. As evidências anteriores de limites são preservadas enquanto os agregados locais de uso são atualizados. Consultas de versão que falham preservam a detecção anterior daquele agente com um aviso explícito; outros agentes continuam sendo atualizados. As consultas reutilizam saída limitada, prazo de seis segundos e limpeza dos processos filhos; erros dos comandos não são citados nos diagnósticos. Retratos do cache são copiados e sincronizados, e resultados atrasados de revisões antigas são ignorados. Não há novo temporizador de atualização automática nem gravação de configurações nativas.

Os diagnósticos de ambiente de provedores inspecionam somente campos nativos suportados e o processo atual do lazyagents. Os relatórios levam nomes de variáveis, origens e próximos passos, nunca valores de endpoint/modelo comparados ou credenciais. Variáveis de credencial são verificadas apenas quanto à presença e conteúdo não vazio; seu conteúdo nunca é comparado nem serializado. A precedência de autenticação salva do Pi usa apenas metadados do tipo. Não são executados arquivos de inicialização do shell, helpers de credenciais ou scripts do Crush; variáveis não são removidas e configurações nativas não são reparadas. Roteamento hospedado do Claude e modos de agente/autenticação sem regras verificadas ficam fora da análise; ajustes de projeto/sessão/gerenciados podem mudar o comportamento efetivo. Variáveis ausentes neste processo não comprovam falha em um agente iniciado separadamente. Veja [cobertura e precedência](guide/providers.md#environment-diagnostics).

O relatório doctor de Agents executa checks locais somente quando aberto com `d` ou `agents doctor`, e repete localmente com `r`. Informa sintaxe da configuração, metadados de skills, links quebrados, estado de provedores e disponibilidade de comandos de hooks, sem reparar arquivos nem alterar links. `n` inclui explicitamente Usage e plugins; `doctor --local` oferece o mesmo escopo restrito na CLI, enquanto `doctor` completo mantém seu comportamento anterior. Checks ignorados aparecem como não executados. Usage pode atualizar seu cache existente, e evidência em cache não comprova disponibilidade remota. Plugins executam com as permissões do usuário. Fechar o relatório cancela a execução, inclusive consultas de uso e processos de handshake/doctor de plugins. Os relatórios duram apenas a sessão, sem nova configuração ou arquivo de relatório. Configuração inválida do lazyagents é identificada pelo caminho, sem citar a entrada do parser; o relatório renderizado neutraliza controles de terminal. Consentimento nativo e credenciais continuam sob controle dos adaptadores.

<a id="network"></a>

## Rede

O lazyagents funciona offline. Só usa a rede quando você pede um recurso que depende dela:

| Quando | Consulta |
|---|---|
| abre Usage, executa `lazyagents usage` / `doctor` completo, ou inclui checks externos com `n` no doctor de Agents | limites Claude Code, no endpoint de `/usage`, cache de 5 minutos; Codex usa arquivos locais |
| solicita `provider probe` / `c` / `ctrl+t` em Providers | uma requisição HEAD sem credenciais à URL base; qualquer resposta HTTP comprova só que o endpoint respondeu |
| solicita `provider models` / `m` / `ctrl+l` em Providers | um GET ao `<base>/models` OpenAI compatível, usando primeiro o bearer salvo/digitado, depois a referência de ambiente, ou nenhuma credencial para endpoints sem chave |
| instala do GitHub | `git clone --depth 1` do repositório |
| busca no GitHub (`S`) | `gh api` com sua autenticação do `gh` |
| verifica atualizações de skills | repositório de origem |

Requisições de provedor têm prazo de 10 segundos e cancelamento (`esc` na TUI, Ctrl+C na CLI). Redirecionamentos nunca são seguidos, inclusive na mesma origem. Catálogos têm limite de 1 MiB após descompressão, headers de 64 KiB e até 10.000 IDs em uma página. Credenciais ficam restritas à construção da requisição; login nativo e helpers nunca são lidos, erros não exibem corpos/URLs brutos e IDs inseguros são recusados. Catálogo retornado com credenciais não valida um token de forma independente em servidores públicos, e modelo listado não comprova inferência. Consultas não gravam configurações nem cache persistente. Veja [verificações de endpoint](guide/providers.md#endpoint-checks-and-model-discovery).

Nada disso executa na inicialização, e não há telemetria.

<a id="consent-belongs-to-the-agent"></a>

## O consentimento pertence ao agente

Alguns agentes exigem sua aprovação para executar comandos. O lazyagents nunca responde em seu nome. Codex registra a aprovação de hooks em `trusted_hash` e exige `[features] hooks` ativado: o lazyagents instala e avisa da pendência, mas não grava o hash nem ativa a função. Você aprova no Codex.

<a id="third-party-content"></a>

## Conteúdo de terceiros

- **Hooks executam comandos.** Um hook de repositório executa a cada evento correspondente com suas permissões. Hooks encontrados na instalação vêm desmarcados; leia primeiro com `v` em Hooks.
- **Skills são instruções** que seus agentes seguirão. Leia o `SKILL.md` (`enter`) antes de ativar de uma origem desconhecida.
- **Plugins são programas** instalados por você. O lazyagents trata mensagens como não confiáveis, remove controles de terminal e isola falhas da aba, mas o plugin executa com suas permissões. Veja o [guia](guide/plugins.md).

<a id="reporting-a-vulnerability"></a>

## Relatar uma vulnerabilidade

Veja [SECURITY.md](../../SECURITY.md).

<a id="partial-operations"></a>

## Operações parciais

Operações em lote de provedores, hooks e ativação/perfis de skills são por destino, sem reversão automática entre agentes. Os resultados identificam agentes bem-sucedidos, com falha e ignorados e incluem o estado lido após a tentativa; um destino com falha ainda pode conter alterações parciais. A TUI mantém o relatório até ser dispensado e confirma novas tentativas apenas nos IDs com falha. As matrizes releem o estado real após falha ou repetição. Na CLI, os detalhes vão para stderr; códigos de saída e JSON das listagens continuam compatíveis. Repita com `--agent` explícito para não atingir novamente destinos bem-sucedidos. Recupere configurações em [Recovery](guide/recovery.md) e arquivos da biblioteca em [Skills](guide/skills.md#backups-and-restore); nenhum deles repete uma operação.

Ações em linhas marcadas de skills fixam as pastas e os agentes de destino revisados antes da confirmação. Linhas ocultas/removidas são desmarcadas, e selecionar linhas visíveis inclui as correspondentes fora da tela. Resultados e repetições de ativação identificam pares exatos skill/agente; atualizar/remover identifica entradas skill/biblioteca, pois todos os leitores usam essa cópia. Entradas locais/externas são ignoradas. Atualizações em lote preservam edições locais e casos sem hash de referência, e recusam origens registradas que mudaram. A remoção mantém a cópia na biblioteca quando a limpeza de links gerenciados falha, permitindo concluir a limpeza em nova tentativa; remoções bem-sucedidas continuam com arquivo de backup. Veja [ações nas skills marcadas](guide/skills.md#act-on-selected-rows).

Instalações de skills na TUI revisam os itens da origem e os agentes de destino explícitos antes de copiar. O padrão inicial é somente biblioteca; depois, os últimos IDs confirmados e disponíveis de `skills.installTargets` vêm pré-selecionados, sem um escopo implícito de todos os agentes. Apenas cópias recém-instaladas são ativadas; colisões preservam as cópias existentes e sua ativação. Falhas de ativação mantêm os arquivos na biblioteca e repetem apenas os links. Falhas de cópia/importação não são repetidas pelo relatório de ativação. Hooks importados continuam dependendo de escolha individual e nunca são ativados pelos destinos de skills. Uma falha ao salvar a preferência aparece separadamente, sem desfazer arquivos instalados nem sobrescrever configuração malformada.

Cadastros de repositórios de skills são apenas configuração. Adicionar/editar/remover nunca busca conteúdo nem altera origens instaladas. Varreduras explícitas validam argumentos de branch/tag, clonam uma vez com hooks desabilitados e exigem que a subpasta resolvida permaneça fisicamente no repositório. Nomes repetidos continuam distintos pelo caminho e seleções que colidem no destino são recusadas. Atualizações de origens cadastradas conservam URL/ref/caminho exato e nunca consultam um cadastro editado nem substituem um caminho ausente por outro. O commit observado registra proveniência, sem verificar assinaturas ou fixar a origem de forma imutável. Nenhuma origem é verificada na inicialização, e estados/erros permanecem só na sessão. A descoberta ignora manifests de hooks que são links antes de ler os comandos.

A revisão de atualização de skills mantém um clone temporário por URL/ref instalada até fechar/aplicar. Hashes locais e da origem são comparados com a base salva; o lote nunca substitui itens editados localmente ou sem base. Uma substituição individual exige seu próprio diff visível e confirmação explícita. Hash/origem locais e conteúdo em staging são verificados novamente antes do backup e da troca de pasta; os metadados de origem também são preparados em staging. Cancelar limpa temporários e encerra as árvores de processos Git/busca. Uma troca atômica já iniciada termina; itens concluídos anteriormente permanecem aplicados com backups. Falhas/cancelamentos exigem nova revisão, sem repetição automática de escritas. O diff tem limites e escapa controles de terminal; conteúdo binário/grande omitido é identificado. Impressões de conteúdo delimitam cada arquivo para detectar mudanças na distribuição de arquivos binários após a revisão. Bases de instalações antigas continuam legíveis e são atualizadas em instalações/atualizações bem-sucedidas. Arquivos de origem antigos e o comportamento de ref/revisão/caminho exato de origens cadastradas continuam compatíveis.

Aplicar um perfil de skills na TUI verifica a mesma impressão de perfil/inventário/destinos da CLI, mostra as pastas canônicas na confirmação e bloqueia a entrada até o resultado. Uma prévia alterada exige outra revisão.

A manutenção de skills pela CLI reaproveita esses serviços em `skills sources`, `skills updates` e `skills profiles`. Listar/verificar origens nunca instala nem altera cadastros. Aplicar atualizações exige pastas canônicas e a identidade da verificação correspondente; aplicar um perfil exige a identidade da prévia da matriz gerenciada inteira. Argumentos/seleções inválidos e revisões antigas são recusados antes de gravar. Identidades de revisão são impressões de conteúdo/escopo, não credenciais de autorização. Varreduras/verificações/atualizações tratam interrupções com limpeza de árvores de processos e staging. O JSON novo mantém stdout estruturado e diagnósticos em stderr; confira o código de saída, principalmente após operação parcial. Aliases e JSON dos comandos de biblioteca existentes continuam compatíveis. Não há novas chaves de configuração, cache persistente de revisão nem atividade remota automática.
