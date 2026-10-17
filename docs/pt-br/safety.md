# Segurança e dados
<!-- source: 0ae3758a80a2 -->

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

## Segredos

- **Tokens de provedores:** ficam em `~/.config/lazyagents/providers.json`, modo 0600, preservado nos backups. TUI e `--json` só indicam presença; texto puro exige `lazyagents provider list --reveal`. Use `--token -` para ler da entrada padrão, fora do histórico do shell.
- **Codex:** prefira `--env-key VAR`; `config.toml` guarda apenas o nome da variável.
- **Claude Code:** token precisa estar em `settings.json`, onde o agente o lê. O lazyagents usa modo 0600 ao escrevê-lo.
- **Pi:** `--env-key VAR` evita token no arquivo (`"apiKey": "$VAR"`); caso contrário fica em `models.json`, modo 0600.
- **Credenciais dos agentes:** por exemplo a autenticação do Claude Code, só são lidas para a consulta que exige isso (limites), nunca guardadas, registradas ou exibidas. Para identificar o tipo, o programa lê apenas existência e tipo (`oauth`/`api_key` no `auth.json` do Pi, presença de chave em `models.json` do Pi ou `crush.json`), nunca o valor.

<a id="network"></a>

## Rede

O lazyagents funciona offline. Só usa a rede quando você pede um recurso que depende dela:

| Quando | Consulta |
|---|---|
| abre Usage, executa `lazyagents usage` ou `doctor` | limites Claude Code, no endpoint de `/usage`, cache de 5 minutos; Codex usa arquivos locais |
| instala do GitHub | `git clone --depth 1` do repositório |
| busca no GitHub (`S`) | `gh api` com sua autenticação do `gh` |
| verifica atualizações de skills | repositório de origem |

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
