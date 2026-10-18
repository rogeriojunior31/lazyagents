# Recuperação de configurações
<!-- source: a99b9d157392 -->

Recovery localiza e restaura backups de configurações dos agentes. [Arquivos de skills](skills.md#backups-and-restore) têm inventário próprio em Skills.

<a id="find-a-change"></a>

## Encontrar uma alteração

Abra **Recovery** ou escolha `: recovery`. `/` filtra por ID do agente, arquivo, operação ou data local (`YYYY-MM-DD`); todos os termos devem corresponder. `enter` aplica o filtro e depois abre a prévia selecionada. `esc` durante a edição limpa o filtro; `r` recarrega.

A tela inicial mostra **Ready to preview**, com totais separados das cópias disponíveis para prévia e **Manual only**. `l` alterna para **All backups** e volta. As linhas mostram datas legíveis com milissegundos, nomes de arquivo, agentes e ações como **Apply provider**. O nome do arquivo, a pasta do backup e o destino da restauração ficam nos detalhes da cópia selecionada, ao lado da lista ou abaixo dela em terminais estreitos. Nome e pasta aparecem separadamente para que uma pasta longa não quebre a identificação do backup. Clique ou use as setas para selecionar; `shift+↑/↓` ou `ctrl+u/d` rola os detalhes.

Operações `provider.apply`, `provider.clear`, `hook.install` e `hook.uninstall` identificam a tentativa de escrita que o backup antecedeu. Descrevem o estado **antes da tentativa**, inclusive quando ela falhou depois. Arquivos que não existiam não têm versão anterior para arquivar.

Novas cópias registram agente, destino exato, operação, data, permissões e checksum. Os nomes dos arquivos continuam iguais. A origem fica em `backups/.config-backup-meta/<backup>.json` (`0600`, pasta criada com `0700`), sem valores da configuração. Metadados saem da retenção junto com o backup; o limite permanece em 20 cópias por prefixo de nome de arquivo.

Cópias sem origem verificada ficam em **All backups**, marcadas como **Manual**. Quando nenhuma está disponível para prévia, a tela inicial explica o motivo e oferece `l` para inspecioná-las. Selecionar uma cópia manual explica a limitação e mostra o caminho do arquivo salvo; `enter` abre esses detalhes sem oferecer restauração. Vários agentes usam `settings.json`, então esse nome não comprova o destino. Nenhum caminho é deduzido ou atribuído às cópias antigas. A CLI mantém campos de agente/operação `unknown` por compatibilidade.


Ao voltar para esta aba, o inventário é relido automaticamente; cópias criadas por outras operações aparecem sem reiniciar.

<a id="review-and-restore"></a>

## Revisar e restaurar

A prévia identifica destino e permissões e compara **arquivo atual → arquivo arquivado**, incluindo alterações não relacionadas feitas desde o backup. JSON mostra caminhos dos campos; TOML e Crush mostram posições de linhas e chaves de atribuição reconhecidas. Todos os valores, comandos e comentários ficam ocultos, inclusive campos desconhecidos. Uma mudança de oculto para oculto ainda indica valores diferentes.

Leia toda a prévia com as teclas de rolagem e pressione `enter` para confirmar a substituição do **arquivo inteiro**, incluindo diferenças não relacionadas mostradas ali. `esc` volta sem escrever. O arquivo atual recebe um backup de segurança com operação `config.restore`. Prevalecem as permissões mais restritas entre arquivo atual e arquivado.

Checksum, metadados, conteúdo do destino, permissões e diretório pai resolvido são verificados novamente antes da escrita. Mudanças desde a revisão exigem nova prévia. Links simbólicos no backup/destino, cópias danificadas e caminhos não aprovados pelo adaptador atual são recusados. Alterar a variável de pasta de configuração pode tornar uma cópia antiga inelegível.

<a id="supported-files-and-limits"></a>

## Arquivos compatíveis e limites

| Agente | Arquivos aprovados pelo adaptador |
|---|---|
| Claude Code | `settings.json` (provedores e hooks) |
| Codex | `config.toml` de provedor; `hooks.json` |
| OpenCode | único arquivo global com JSON estrito `opencode.json` ou `opencode.jsonc`; metadados de propriedade não são restaurados |
| Pi | `models.json` e `settings.json`, revisados separadamente |
| Crush | `crushrc` global (provedores e hooks) |

Os caminhos respeitam as [variáveis de configuração dos agentes](../reference/agents.md#config-overrides). As verificações cobrem integridade e a estrutura compreendida pelos adaptadores existentes; configurações nunca são executadas e nem todas as opções nativas são validadas. Um JSON atual corrompido pode ser substituído por um objeto arquivado válido. TOML usa o leitor estrutural existente; Crush verifica blocos gerenciados.

Codex recusa mudanças em `config.toml` com texto relacionado a hooks, hashes de confiança ou escapes Unicode que possam ocultar chaves de aprovação. Essa verificação conservadora mantém a [confiança de hooks](hooks.md#codex-hooks-switch-and-trust) sob controle do Codex. Esses arquivos exigem revisão nativa/manual.

A recuperação é **por arquivo**. Perfis salvos, registros de propriedade de provedores, bibliotecas e outros arquivos mantêm o estado atual. Pi exige prévias/restaurações separadas para seus dois arquivos. Providers e Hooks atualizam o estado exibido após a recuperação. Um hook restaurado pode apontar para um script removido separadamente: confira biblioteca e scripts antes de usá-lo.

<a id="from-the-cli"></a>

## Pelo CLI

```sh
lazyagents config-backups list
lazyagents config-backups list --query 'claude-code settings.json provider.clear'
lazyagents config-backups list --query '2026-10-17' --json
lazyagents config-backups preview settings.json.20261017T120000.000000000
```

Use um ID exato da lista. A prévia imprime o comando de restauração e o token `--review`; execute após revisar as diferenças. `preview <id> --json` oferece as mesmas mudanças ocultadas e token para scripts. Restaurar exige esse token e recusa revisão desatualizada. Veja a [referência do CLI](../reference/cli.md#config-backups).

<a id="after-a-partial-operation"></a>

## Após uma operação parcial

Relatórios em lote de provedores/hooks abrem esta aba com `b`. Os resultados descrevem a tentativa; uma falha pode incluir alterações dentro do agente, e os agentes bem-sucedidos continuam alterados. Corrija a causa e repita nos destinos com falha pelo relatório, ou revise uma cópia antes de restaurar um arquivo inteiro. Restaurar não repete a operação original. A ativação de skills usa links simbólicos: ajuste a visibilidade em Skills e use seu inventário próprio de arquivos para conteúdo da biblioteca.

<a id="recent-operation-history"></a>

## Histórico recente de operações

Pressione `h` em Recovery ou escolha `: history` para inspecionar tentativas de gravação concluídas, inclusive após reiniciar. As linhas mostram módulo/ação, horário local e totais de destinos concluídos, com falha e ignorados. `enter` abre os IDs e resultados retidos. O histórico inclui aplicação/limpeza de provedores, ativação/desativação/alternância de comandos de hooks, instalação/ativação/perfis/lotes/atualização/incorporação/remoção/restauração de skills e restauração de configurações. Não registra conteúdo de sessões, edição local de perfis/formulários, preferências, ações de plugins ou operações realizadas diretamente nos agentes.

Nos detalhes, `r` navega ao módulo original para inspecionar o estado atual e revisar uma nova tentativa. Não repete a operação salva nem seleciona todos os destinos originais. Corrija a causa e escolha explicitamente os destinos que falharam; um destino concluído pode ter mudado novamente desde o registro. Relatórios ainda abertos mantêm o serviço de repetição apenas dos destinos com falha; após reiniciar, atualizações exigem nova revisão da origem. Se o módulo estiver oculto, habilite sua aba em `settings` ou use o CLI.

Backups relacionados são associados por horário, ação e destino, com origem e checksum das configurações verificados ao registrar. Tentativas simultâneas podem referenciar o mesmo backup; são candidatos à recuperação, não um registro transacional. Use `[`/`]` para escolher uma referência e `b` para revalidar origem, integridade e destino atual pela prévia existente com valores ocultos. Restaurar ainda exige confirmação explícita. Backups ausentes, removidos pela retenção, danificados ou sem destino aprovado não podem ser restaurados pelo histórico. Referências de skills levam ao inventário separado em Skills (`B`); confira o nome exato antes de restaurar. Links e arquivos novos podem não ter cópia anterior.

O histórico fica em `operation-history.json` no diretório de dados (`0600`), mantendo as **200 tentativas** mais recentes, até **1000 destinos** e **32 backups relacionados** por tentativa, dentro de **32 MiB**. Entradas antigas saem automaticamente; destinos excedentes são contabilizados explicitamente. Essa retenção é independente dos backups. São salvos apenas horário, IDs, módulo/ação, nome do recurso, resultado e referências: sem erros brutos, detalhes/estado dos resultados, tokens, corpos de configuração, URLs de origem, transcrições ou comandos importados. O arquivo registra resultados concluídos dos serviços; um processo encerrado antes da persistência pode não deixar entrada. Após interrupções, confira os arquivos reais.

`c` pede confirmação antes de limpar o histórico. Todos os backups, ajustes atuais e arquivos da biblioteca permanecem. Gravação e limpeza usam bloqueio local exclusivo e substituição atômica condicional, impedindo que processos simultâneos substituam registros silenciosamente. Problemas de persistência são relatados separadamente dos resultados concluídos; não repita destinos já concluídos para reparar o histórico. Uma queda durante a curta atualização pode deixar `operation-history.json.lock`: remova essa pasta vazia somente após confirmar que nenhum processo lazyagents está gravando. Um arquivo regular de histórico inválido pode ser limpo explicitamente; links, arquivos especiais ou grandes demais são recusados e precisam de inspeção local.

```sh
lazyagents history list --query 'providers codex failed' --json
lazyagents history inspect <id> --json
lazyagents history clear --yes
```

`inspect` verifica a disponibilidade dos backups de configuração e mostra caminhos para revisão; nenhum comando de histórico executa repetição ou restauração. Veja a [referência do CLI](../reference/cli.md#history).

<a id="refresh-state-and-selection"></a>

## Estado da atualização e seleção

Carregamento, falha na primeira leitura e inventário vazio são mostrados separadamente. Falhas ao atualizar mantêm backups anteriores com indicação de desatualização. Atualizações bem-sucedidas preservam o backup selecionado pelo ID exato, mesmo quando uma cópia nova muda a ordem da lista. Prévia/restauração continuam revalidando os arquivos reais; retornar de um relatório concluído ou atualizar nunca repete uma gravação.
