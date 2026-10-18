# Provedores
<!-- source: eb08269ccd83 -->

Um perfil de provedor define endpoint, modelo e token com um nome. O lazyagents escreve esses valores na configuração do agente para alternar Claude Code ou Codex entre API oficial, proxy e modelo local sem editar JSON/TOML manualmente. Funciona como [cc-switch](https://github.com/farion1231/cc-switch): grava configurações nativas, pode consultar catálogos sob demanda e nunca executa um proxy. Agentes compatíveis e arquivos estão na [referência](../reference/agents.md#capabilities).

![A aba Providers: um perfil de modelo local e outro de API, a ajuda contextual e a confirmação mostrada antes de aplicar um perfil ao Claude Code](../../assets/demos/providers.gif)

<a id="concepts"></a>

## Conceitos

- **Perfil:** `name`, `baseUrl`, `model`, `token`, `envKey` e `wireApi`. Precisa definir pelo menos endpoint, modelo ou token. Nomes têm até 40 caracteres.
- **Biblioteca:** `~/.config/lazyagents/providers.json`, com modo `0600`, pois pode conter tokens.
- **Aplicado:** estado atual da configuração do agente, lido diretamente do arquivo. Uma configuração manual também aparece como provedor definido fora do lazyagents. A identificação compara os campos públicos usados pelo adaptador: endpoint, modelo explícito, protocolo normalizado e referência/presença de credencial. Uma correspondência única descreve esses campos, não a autenticação nem qual perfil foi aplicado historicamente.
- **Limpar:** remove apenas o que o lazyagents gerencia e restaura o estado anterior. O resto fica intacto.

<a id="in-the-tui"></a>

## Na TUI

A aba Providers é uma matriz perfil × agente: `●` correspondência pública única gerenciada, `◆` correspondência externa ou ambígua, `○` sem correspondência para a linha e `–` agente não instalado. O detalhe mostra endpoint, modelo, estado do token (`token ✓`, nunca o valor) e configuração aplicada por agente. Veja as [teclas](../reference/keys.md#providers-tab).

<a id="create-a-profile"></a>

### Criar um perfil

Sem perfis salvos, a aba aponta para `n` para template/perfil personalizado e `i` para importar os campos públicos do agente selecionado. A paleta também oferece `providers new`. Criar/importar salva primeiro na biblioteca; revise e aplique separadamente. Sem agente compatível, crie o perfil agora, instale a CLI e use `agents refresh` antes de aplicar.

`n` permite escolher um template de conexão, uma configuração personalizada ou os padrões do agente. `enter` em uma conexão abre o formulário editável com nome, endpoint, modelo, token, variável de ambiente e API. `tab` troca de campo e `enter` salva. `ctrl+v` verifica a compatibilidade do rascunho com os agentes instalados sem salvar ou aplicar; `esc` volta ao formulário. A verificação usa a mesma validação da aplicação, inclusive a presença do token salvo durante a edição. O token fica mascarado. `e` edita o perfil: deixar o token vazio mantém o salvo, que nunca é carregado no formulário. Renomear também renomeia na biblioteca. A ajuda do campo em foco aparece ao lado em terminais largos e abaixo nos menores; use `shift+↑/↓`, `ctrl+u/d` ou a roda do mouse para rolar. Conexões guiadas exigem endpoint e modelo explícito; perfis personalizados mantêm os campos opcionais conforme o agente. Salvar altera apenas a biblioteca; aplicar continua sendo uma ação separada.

<a id="import-an-existing-connection"></a>

### Importar uma conexão existente

Escolha a coluna do agente com `←/→` e pressione `i`, inclusive com a biblioteca vazia. O rascunho editável importa endpoint, modelo, protocolo e referências de ambiente compatíveis da configuração nativa. Escolha um nome novo e salve; nomes existentes são recusados. Salvar nunca grava arquivos do agente. Aplique o perfil separadamente quando estiver pronto.

Valores de tokens nativos nunca são copiados para o rascunho ou a biblioteca. Quando há credencial literal, digite-a novamente no campo mascarado ou forneça uma referência de ambiente compatível com o agente. Claude exige a digitação do token porque seu adaptador de perfis não resolve essas referências. Login de conta nativo, helpers de credencial e opções avançadas permanecem no agente e devem ser configurados nele. A importação captura campos públicos representáveis; não copia toda a autenticação nem resolve ajustes efetivos de projeto/ambiente.

Claude lê suas chaves globais explícitas de provedor; Codex lê a tabela do provedor personalizado selecionado, incluindo nomes simples entre aspas; Pi lê o provedor selecionado em `models.json` e os padrões de `settings.json`, respeitando endpoint/API explícitos por modelo. Uma API herdada ausente no arquivo não pode ser importada com segurança; os campos seguem a [referência de modelos do Pi](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/models-configuration.md). Crush importa a conexão explícita de `models.large`/`providers` no `crush.json` global somente quando não existe `crushrc` executável que possa sobrescrevê-la; aceita apenas os tipos `openai-compat` e `anthropic`. Um bloco gerenciado do Crush continua legível. Bash arbitrário nunca é executado para descobrir configurações. Protocolos não suportados, referências inválidas e ausência de endpoint explícito são recusados. Os campos JSON do Crush seguem seu [schema v0.96.1](https://github.com/charmbracelet/crush/blob/v0.96.1/schema.json).

<a id="preview-configuration-changes"></a>

### Revisar alterações de configuração

Aplicar e limpar abrem a revisão de destinos antes da confirmação. Cada arquivo nativo mostra `create`, `update`, `remove` ou `unchanged`, seguido dos campos gerenciados adicionados, alterados ou removidos. Todos os valores ficam ocultos, inclusive credenciais e dados nativos não relacionados. A prévia reutiliza a lógica de edição da gravação, inclui os dois arquivos do Pi e calcula a restauração dos ajustes anteriores de provedor/modelo. Inspecionar ou cancelar não cria backups nem registros de propriedade.

Antes da operação confirmada, o lazyagents verifica novamente os arquivos selecionados, os registros de propriedade e o perfil salvo atual. Uma mudança desde a revisão bloqueia a operação antes de gravar qualquer destino; reabra a prévia. As gravações mantêm substituição atômica condicional e backups. Falhas durante a execução ainda podem deixar alterações parciais entre arquivos/agentes, relatadas no painel normal de resultados. Após confirmar, ações de provedor ficam bloqueadas até a operação terminar, impedindo envios duplicados.

<a id="compare-saved-and-live-settings"></a>

### Comparar configurações salvas e atuais

O detalhe separa o perfil salvo do estado nativo atual. Selecione um perfil e a coluna do agente para ver diferenças de endpoint, modelo, protocolo ou credencial como **saved** (salvo) e **live** (atual). Editar a biblioteca ou a configuração nativa pode causar diferenças; a comparação não atribui quando nem onde surgiram. `r` relê ambos sem gravar, criar backups ou fazer requisições de rede.

Padrões de protocolo e prioridade de credencial seguem cada adaptador. Claude sempre usa Anthropic e ignora `wireApi`/`envKey` do perfil; Codex usa Responses por padrão e sua referência de ambiente; Pi normaliza aliases nativos de protocolo, enquanto Pi/Crush preferem token salvo à referência de ambiente. Campos opcionais não definidos não restringem a identificação. Aplicá-los ainda segue as regras nativas abaixo, inclusive remoção ou restauração de valores anteriormente gerenciados; revise a prévia antes de confirmar.

Todos os perfis com campos públicos correspondentes são listados. Múltiplas correspondências são **ambíguas**: o lazyagents nunca escolhe o primeiro perfil, e `space` revisa a aplicação da linha selecionada em vez de limpar uma correspondência arbitrária. Perfis que diferem apenas no valor literal do token, em campos ignorados ou em padrões equivalentes podem continuar ambíguos após aplicar. Tokens são representados apenas pela presença, e valores de variáveis nunca são lidos para comparar. Uma correspondência pública única não verifica se os tokens salvo e nativo são iguais ou válidos.

Para **manter a configuração atual**, deixe-a como está ou cancele uma prévia com `esc`. Para **reaplicar**, selecione o perfil e o agente desejados e pressione `p`, depois revise e confirme. Isso sempre abre a prévia de aplicação, inclusive para trocar token quando os campos públicos já coincidem. `space` limpa somente uma correspondência única gerenciada; uma correspondência externa continua indicada como externa e abre a aplicação. Atualizar, renderizar ou editar um perfil nunca reaplica automaticamente. A leitura cobre campos globais nativos representáveis, não ajustes efetivos de projeto/ambiente nem autenticação de conta nativa.

<a id="connection-templates"></a>

### Templates de conexão

O catálogo pequeno fornece valores iniciais no formato de perfil existente. Cada entrada informa agentes compatíveis com o template, protocolo, campos obrigatórios, credenciais e fonte/data da revisão. As fontes abaixo foram revisadas em **2026-10-17** e os perfis são verificados contra os adaptadores nativos. Isso verifica a orientação de configuração, não autenticação real, disponibilidade do modelo ou sucesso de inferência. Templates não acessam a rede nem leem o valor da variável de credencial.

| Opção inicial | Agentes do template | O que fornecer |
|---|---|---|
| Custom connection | Claude Code, Codex, Pi, Crush | Endpoint, protocolo e campos específicos; tudo continua editável |
| Agent defaults / previous settings | Claude Code, Codex, Pi, Crush | Selecionar agentes explicitamente, revisar e confirmar a remoção dos ajustes de provedor gerenciados |
| OpenAI Responses | Codex, Pi | Modelo e `OPENAI_API_KEY` exportada; preenche `https://api.openai.com/v1` e `responses` ([referência OpenAI](https://developers.openai.com/api/reference/resources/responses/methods/create)) |
| OpenAI Chat Completions | Pi, Crush | Modelo e `OPENAI_API_KEY` exportada; preenche `https://api.openai.com/v1` e `chat` ([referência OpenAI](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create)) |
| Anthropic gateway for Claude Code | Claude Code | URL base do gateway, modelo e token bearer salvo; o gateway existente deve aceitar Anthropic Messages e `Authorization: Bearer` ([configuração Claude Code](https://code.claude.com/docs/en/llm-gateway-connect)) |
| Ollama local · Chat Completions | Pi, Crush | Servidor local em execução e ID exato do modelo baixado; preenche `http://localhost:11434/v1` e `chat`, sem token ou variável ([referência Ollama](https://docs.ollama.com/api/openai-compatibility)) |

Templates não convertem protocolos nem iniciam proxy ou servidor. Codex exige Responses; escolher Chat Completions não torna o endpoint compatível com Codex. Mantenha o caminho base da API, sem acrescentar operações como `/responses` ou `/chat/completions`. Escolha um modelo com suporte às ferramentas do agente. A opção Ollama cobre o uso local sem chave; endpoints autenticados ou na nuvem exigem credenciais personalizadas. O gateway para Claude usa token bearer salvo; gateways que exigem `x-api-key`, helpers rotativos ou outro login continuam sob configuração nativa.

Todos os campos sugeridos podem ser alterados antes de salvar. Alterações de endpoint/protocolo exigem nova verificação; os agentes documentados pelo template se referem à configuração original. Editar um perfil salvo abre seus valores atuais, sem repor os do template. Escolher ou cancelar um template não sobrescreve perfis, aplica conexões nem descarta credenciais salvas.

**Agent defaults** usa a limpeza existente, sem salvar perfil vazio. Nenhum agente começa marcado: selecione os destinos explicitamente e confirme. Remove apenas configurações de provedor gerenciadas pelo lazyagents e restaura valores anteriores quando registrados; não força o provedor oficial. Configurações externas, do projeto ou do ambiente podem continuar prevalecendo. Faça login pelo próprio agente; seus arquivos de autenticação e configurações não relacionadas são preservados. A biblioteca não muda, e resultados e novas tentativas usam o relatório normal.

<a id="apply-or-clear-per-agent"></a>

### Aplicar ou limpar por agente

Escolha a coluna com `←/→` e pressione `space` para validar aquele destino. `a` abre a seleção com os agentes instalados que aceitam provedores marcados. Cada linha indica se o perfil está pronto ou incompatível; selecione a linha para ler o motivo, os arquivos afetados e os requisitos de ativação. `space` inclui ou exclui agentes e `enter` revisa a seleção na confirmação existente. Agentes incompatíveis marcados impedem a confirmação: escolher só os compatíveis é uma ação explícita. `shift+↑/↓` ou `ctrl+u/d` rolam os requisitos; `esc` cancela sem gravar.

A validação verifica a sintaxe do endpoint e da referência de credencial, requisitos de protocolo/modelo por agente e os arquivos e registros necessários à alteração. Todos os destinos selecionados são verificados antes de gravar configurações, backups ou registros de propriedade. Token sem `envKey` para Codex, protocolo incompatível, bloco gerenciado inválido ou configuração ilegível bloqueiam a aplicação inteira. Os arquivos são relidos na gravação. Autenticação e suporte real de endpoint/modelo não são testados: o nome válido de uma variável é apenas uma referência. A validação não lê nem exporta seu valor. Inicie uma nova sessão do agente após aplicar, para carregar a configuração e as variáveis necessárias.

Usar `space` em uma correspondência única gerenciada abre a prévia da limpeza. Use `p` para revisar a reaplicação. `x` abre essa prévia com os agentes instalados marcados; desmarque os que deseja preservar, revise e confirme. `d` remove da biblioteca, mas não desfaz configurações já aplicadas; use `x` ou limpe por agente.

<a id="environment-diagnostics"></a>

### Diagnósticos de ambiente

Os detalhes de Providers, `provider list` e o [doctor](agents.md#doctor-report) informam problemas de ambiente suportados com nomes de variáveis, arquivos de origem e próximos passos. `provider list --json` acrescenta diagnósticos `environment`; os findings do doctor recebem `code`, `variable`, `source` e `other_source` quando aplicáveis. Valores comparados e conteúdo de credenciais nunca entram nesses diagnósticos. Os diagnósticos ficam acessíveis mesmo sem perfis salvos. Terminais baixos priorizam os detalhes do agente selecionado: troque com `←/→` e role com `ctrl+u/d` ou `shift+↑/↓`. `r` relê os arquivos em Providers; após mudar o ambiente do shell, é necessário iniciar o lazyagents novamente a partir desse ambiente.

- **Claude Code:** compara `ANTHROPIC_BASE_URL` e `ANTHROPIC_MODEL` do processo atual com `settings.env` do usuário e verifica `ANTHROPIC_MODEL` contra a chave `model`. A configuração normalmente substitui variáveis herdadas; políticas de sessão ou gerenciadas podem preservar ou substituir esses valores. A variável de modelo aplicada precede `model`; flags e `/model` podem prevalecer. Roteamento hospedado (`CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST`) fica fora da análise. Valores na configuração são literais: `$NAME` em um token não é uma referência ausente. Veja a [precedência do Claude](https://code.claude.com/docs/en/env-vars#precedence).
- **Codex:** verifica se o `env_key` explícito do provedor selecionado na configuração do usuário está ausente, vazio ou contém apenas espaços neste processo. Não presume sobrescritas de endpoint nem resolve login nativo, helpers de autenticação ou perfis selecionados pela CLI. Veja a [configuração de provedores do Codex](https://learn.chatgpt.com/docs/config-file/config-reference).
- **Pi:** verifica referências explícitas `$NAME`/`${NAME}` no provedor selecionado em `models.json`. Uma entrada `api_key` ou `oauth` desse provedor em `auth.json` suprime o check da referência de menor prioridade em models; apenas metadados do tipo são decodificados. `--api-key` em tempo de execução precede credenciais salvas, depois `models.json` e, por fim, ambiente/autenticação de nuvem do provedor. Flags de execução e extensões ficam fora desse retrato, e uma entrada salva não comprova autenticação válida. Dólares escapados, literais e comandos não são referências de ambiente. Veja a [autenticação do Pi](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/models.md#authenticate) e os [valores de provedores personalizados](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/custom-provider.md).
- **Crush:** verifica a referência explícita de `--api-key "$NAME"` no bloco gerenciado pelo lazyagents. O Crush expande a variável ao carregar `crushrc`; scripts podem defini-la depois, então a ausência aqui não comprova ausência no Crush. Scripts arbitrários e JSON legado não são avaliados nesses diagnósticos. Veja a [configuração do Crush](https://github.com/charmbracelet/crush#configuration).

São observações da configuração de usuário suportada e do **processo lazyagents**, não um resolvedor da configuração efetiva de sessões abertas, camadas de projeto/gerenciadas ou todos os modos de autenticação. Uma referência ausente pede revisão, não remoção automática nem afirmação de falha de conexão. Nenhum arquivo de inicialização do shell ou comando de credencial é avaliado; variáveis e arquivos não são alterados, e nenhuma requisição é enviada. Metadados ilegíveis geram aviso explícito de diagnóstico indisponível, sem entrada bruta do parser. Outros agentes ainda não têm verificações de ambiente confirmadas.

<a id="endpoint-checks-and-model-discovery"></a>

### Verificação de endpoint e descoberta de modelos

Use `c` em um perfil salvo, ou `ctrl+t` no formulário, para enviar uma requisição `HEAD` sem credenciais à URL base. Uma resposta HTTP, inclusive 401, 404 ou 405, comprova que um endpoint HTTP respondeu; não comprova autenticação nem compatibilidade com a API. Não há requisição alternativa nem inferência.

Use `m` em um perfil salvo, ou `ctrl+l` no formulário, para consultar um catálogo **OpenAI compatível**. A paleta também oferece `providers probe` e `providers models`. A operação envia `GET <base>/models`, preservando o prefixo configurado: `https://host/proxy/v1/` vira `https://host/proxy/v1/models`. Inclua o prefixo exigido pela API; o lazyagents não adivinha nem acrescenta `/v1`, converte URLs de operação ou segue redirecionamentos. Queries na URL, credenciais embutidas e fragmentos são recusados nas ações de rede. Catálogos Anthropic, login nativo e autenticação personalizada não são suportados; digite esses modelos manualmente. Revisado em **2026-10-17** contra a [API de modelos OpenAI](https://developers.openai.com/api/reference/resources/models/methods/list) e a [referência de compatibilidade Ollama](https://docs.ollama.com/api/openai-compatibility).

A consulta usa primeiro o token bearer salvo ou digitado no perfil, depois `envKey` do **ambiente do processo lazyagents**, ou nenhuma credencial para endpoints sem chave. Ao editar com token vazio, a requisição usa a credencial salva sem carregá-la no campo. Variáveis exportadas só em outro processo do agente não estão disponíveis aqui. A consulta descreve a conexão do perfil, não a autenticação efetiva do agente selecionado; arquivos de login nativo, helpers e headers personalizados nunca são lidos ou executados.

O resultado distingue resposta do endpoint, credencial configurada/enviada/rejeitada/proibida, catálogo interpretado e estado do modelo selecionado: `listed`, `not_listed`, `not_selected` ou `not_checked`. HTTP 401 rejeita autenticação; 403 proíbe acesso; 404/405 indica ausência de catálogo compatível. Um catálogo retornado com credenciais comprova que aquela requisição recebeu dados; servidores públicos podem ignorar a credencial, portanto isso não valida o token de forma independente. Um ID listado é evidência do catálogo, não comprovação de inferência, ferramentas, compatibilidade de protocolo ou disponibilidade futura.

Digite para filtrar IDs, navegue com `↑/↓` e pressione `enter` para colocar o ID exato no formulário editável. Na matriz, isso abre a edição do perfil existente. Salvar e aplicar continuam separados. A entrada manual permanece disponível, inclusive depois de erro, catálogo vazio ou não suportado. Role os detalhes com `shift+↑/↓`, `ctrl+u/d` ou a roda do mouse. `esc` cancela uma requisição em andamento ou fecha o resultado preservando o rascunho; respostas atrasadas não podem reabri-lo.

Cada requisição tem prazo de 10 segundos, limite de 64 KiB nos headers de resposta e 1 MiB no corpo do catálogo após descompressão. Apenas uma página com até 10.000 entradas é aceita; paginação explícita é recusada. IDs devem ser ASCII visível, não vazios, sem espaços, com até 256 bytes e sem conter a credencial usada. Resultados inseguros ou inválidos são recusados. Corpos de erro, URLs de redirecionamento e erros brutos da rede não são exibidos. Consultas não gravam perfis, configurações nativas, propriedade, backups nem cache persistente. Nada é executado automaticamente ao iniciar, recarregar, validar, revisar ou aplicar.

<a id="what-is-written-to-claude-code"></a>

### O que é escrito no Claude Code

Três chaves no bloco `env` de `~/.claude/settings.json`:

```json
{
  "theme": "dark",
  "env": {
    "ANTHROPIC_AUTH_TOKEN": "sk-…",
    "ANTHROPIC_BASE_URL": "https://proxy.example.com",
    "ANTHROPIC_MODEL": "my-model"
  }
}
```

Só essas chaves mudam. Outras variáveis e chaves mantêm valor e ordem; novas chaves ficam no fim de `env`. O lazyagents lembra quais escreveu e só remove as próprias:

- Um campo vazio não muda uma chave definida manualmente. Se foi escrita por um perfil anterior, é removida: trocar para perfil sem modelo retira o modelo antigo.
- Se substituir endpoint ou modelo manual, limpar restaura o valor anterior. Tokens manuais não são guardados pelo lazyagents; permanecem no backup de `settings.json`.
- Chaves editadas por você após a aplicação passam a ser suas e não são removidas.
- Se o lazyagents criou `env` e ele ficar vazio, limpar remove também o bloco. Aplicar e limpar devolve o arquivo original.

O registro fica em `~/.local/share/lazyagents/claude-provider-state.json` (0600) e contém apenas a impressão digital dos valores escritos, nunca o token. Sem registro (nenhuma aplicação ou perfil de v0.2 ou anterior), nenhuma chave é considerada gerenciada: `x` as preserva, sem apagar configurações manuais de proxy ou Bedrock. Remova manualmente resíduos de versões antigas; os backups preservam o arquivo anterior.

Ao escrever um token, a permissão do arquivo passa a `0600`.

<a id="what-is-written-to-codex"></a>

### O que é escrito no Codex

Codex mantém estado próprio em `~/.codex/config.toml`, como confiança em `[projects.*]` e hashes de hooks. O lazyagents não interpreta nem reescreve o arquivo inteiro. Só edita blocos entre marcadores `# lazyagents — managed block`, preservando as outras linhas:

```toml
# lazyagents — managed block start (do not edit by hand)
model_provider = "lazyagents"
model = "qwen3"
# lazyagents: previous model = "gpt-5"
# lazyagents — managed block end

[projects."/home/me/app"]
trust_level = "trusted"

# lazyagents — managed block start (do not edit by hand)
[model_providers.lazyagents]
name = "local"
base_url = "http://localhost:11434/v1"
env_key = "OLLAMA_KEY"
wire_api = "responses"
# lazyagents — managed block end
```

Os comentários do exemplo são marcadores reais do programa e devem permanecer exatamente assim. Chaves globais ficam antes da primeira tabela; a tabela do provedor vai ao fim. Os valores anteriores de `model` e `model_provider` são guardados como comentários e restaurados ao limpar. Aplicar e limpar devolve o original.

Para Codex, o perfil exige endpoint. `wireApi` só aceita `responses`, também padrão do Codex.

### Pi

Pi guarda provedores personalizados em `~/.pi/agent/models.json`. O lazyagents escreve o provedor `lazyagents` e o define como padrão em `~/.pi/agent/settings.json` (`defaultProvider` e `defaultModel`). Os demais valores ficam intactos:

```json
{
  "providers": {
    "lazyagents": {
      "name": "lazyagents: local",
      "baseUrl": "http://localhost:11434/v1",
      "api": "openai-completions",
      "apiKey": "$OLLAMA_KEY",
      "models": [{ "id": "qwen3" }]
    }
  }
}
```

Pi só disponibiliza modelos declarados pelo provedor; o perfil precisa de endpoint e modelo. `wireApi` escolhe a API: vazio ou `chat` para chat compatível com OpenAI, `responses` para OpenAI Responses, `anthropic` para Anthropic Messages. Provedor e modelo padrão anteriores ficam nos dados do lazyagents e são restaurados ao limpar, salvo se você já tiver escolhido outro no Pi. Um `models.json` criado pelo perfil é removido na limpeza.

### Crush

`crushrc` é um script Bash com comandos como `provider add` e `model large`, executado de cima para baixo; linhas posteriores prevalecem. O lazyagents acrescenta um bloco ao fim de `~/.config/crush/crushrc` (ou diretório de `CRUSH_GLOBAL_CONFIG`) sem alterar suas linhas:

```bash
# lazyagents — managed block start: provider (do not edit by hand)
provider add lazyagents --name 'lazyagents: local' --type openai-compat --base-url 'http://localhost:11434/v1' --api-key "$OLLAMA_KEY"
model add 'lazyagents/qwen3' --name 'qwen3'
model large 'lazyagents/qwen3'
# lazyagents — managed block end: provider
```

Os comentários também são marcadores reais, não texto a traduzir na configuração. Como é o último, `model large` do bloco prevalece. Limpar remove o bloco e volta a usar suas linhas, sem restauração adicional. Valores usam aspas simples para impedir execução como Bash; variáveis usam `"$VAR"` para o Crush expandir. O perfil exige endpoint e modelo; `wireApi` aceita vazio, `chat` (compatível com OpenAI) ou `anthropic`. Um `crushrc` criado pelo lazyagents é removido junto com o bloco. Se houver `crush.json` na mesma pasta, Crush combina os dois, dá preferência ao `crushrc` e registra um aviso.

<a id="tokens-and---env-key"></a>

### Tokens e --env-key

Claude Code lê o token de `settings.json`, então ele é escrito ali. Claude sempre usa Anthropic Messages; `wireApi` e `envKey` do perfil não mudam seu protocolo nem fornecem seu token. Use um token salvo ou configure a autenticação nativa separadamente. Codex só lê a variável indicada por `env_key`, nunca recebe token em `config.toml`. Pi aceita os dois: token em `models.json` com modo 0600, ou `"apiKey": "$VAR"`, resolvido pelo próprio Pi. Pi esconde modelos sem credencial, então perfis sem chave, como Ollama local, recebem `"apiKey": "lazyagents-no-key"`, conforme a convenção do Pi. No Codex, defina a variável no perfil (`--env-key`) e exporte no shell:

```sh
export OLLAMA_KEY=…   # no perfil do seu shell
```

Aplicar no Codex um perfil com token, mas sem `envKey`, falha com explicação em vez de criar configuração sem autenticação.

<a id="bulk-results"></a>

## Resultados em lote

Após passar pela validação, aplicar/limpar provedores informa cada agente como `succeeded`, `failed` ou `skipped`, com motivo e estado lido após a tentativa. As alterações são por destino: uma falha não desfaz as gravações que funcionaram, e um destino com falha pode ter alterações parciais. A matriz relê o disco mesmo após falhas.

O resultado fica aberto até ser dispensado. Role relatórios longos, use `r` para revisar e confirmar uma nova tentativa apenas nos IDs com falha, `b` para abrir Recovery ou `esc`/`enter` para voltar à matriz atualizada. A repetição lê a entrada salva atual e mantém os resultados anteriores de sucesso ou omissão; não repete a seleção original de todos os agentes. Consulte a [referência de teclas](../reference/keys.md) e o [guia de recuperação](recovery.md). Se Recovery estiver oculta, use `lazyagents config-backups list`.

As alterações pela CLI imprimem diagnósticos por destino em stderr, incluindo sucessos quando outro destino falha. Códigos de saída, saída de sucesso e campos JSON das listagens permanecem iguais. Corrija a causa e repita a operação com `--agent <id-com-falha>` para cada falha; repetir o comando em lote incluiria novamente os agentes bem-sucedidos.

O estado observado no relatório descreve a última tentativa naquele destino. `b` abre Recovery e mantém o relatório disponível ao voltar. Se Recovery estiver oculta, o relatório permanece aberto; use `lazyagents config-backups list` como alternativa.

<a id="from-the-cli"></a>

## Pela CLI

```sh
# proxy para Claude Code; token pela entrada padrão, fora do histórico do shell
pass show work/proxy-token | lazyagents provider add proxy \
  --base-url https://proxy.example.com --model my-model --token -

# modelo local para Codex
lazyagents provider add local --base-url http://localhost:11434/v1 \
  --model qwen3 --env-key OLLAMA_KEY --wire-api responses

lazyagents provider probe local              # HEAD, sem credenciais
lazyagents provider models local --json       # consulta explícita; sem inferência
lazyagents provider validate proxy --json   # sem gravações ou rede
lazyagents provider validate local --agents codex,pi
lazyagents provider apply local --agents codex,pi
lazyagents provider adopt imported --agent codex  # só biblioteca; preserva referência de ambiente
lazyagents provider apply imported --agent codex --preview --json
lazyagents provider clear --agent codex --preview
lazyagents provider apply proxy --agent claude-code
lazyagents provider apply local --agent codex
lazyagents provider list            # tokens como •••• ou $VAR para envKey
lazyagents provider list --json     # sem tokens; usa "has_token": true
lazyagents provider clear           # todos os agentes instalados
lazyagents provider rm proxy
```

Argumentos posicionais inesperados são recusados antes de qualquer operação, inclusive quando aparecem antes das flags de destino.

Sem `--agent` ou `--agents`, `apply` valida todos os agentes instalados que aceitam provedores; um destino incompatível impede todas as gravações. Escolha explicitamente com `--agent id` ou `--agents id,id`. `clear` mantém `--agent` e o padrão de todos os agentes instalados. Falhas durante a gravação ainda podem ser parciais: os demais destinos são processados e os erros são reunidos. Veja os [comandos e flags](../reference/cli.md#provider).

Aplicar e limpar mostram a prévia mascarada antes de gravar; `--preview` encerra sem alterações e aceita `--json`. `list --json` mantém os campos existentes e acrescenta `matches` (todos os nomes candidatos pelos campos públicos) e `comparisons` (por perfil, `differences` públicos com `field`, `saved`, `live`, mais campos opcionais sem restrição em `inherited`). `profile` só aparece com correspondência única; fica vazio na ambiguidade. `external: true` indica ausência de ajustes de provedor gerenciados. `saved_changed: true` passa a indicar que o estado ativo não corresponde a nenhum perfil salvo; não determina propriedade histórica nem origem da mudança.

<a id="files"></a>

## Arquivos

| Caminho | Conteúdo |
|---|---|
| `~/.config/lazyagents/providers.json` | biblioteca de perfis, 0600 |
| `~/.local/share/lazyagents/claude-provider-state.json` | chaves `env` escritas no Claude Code, só impressões digitais, 0600 |
| `~/.claude/settings.json` | bloco `env` do Claude Code |
| `~/.codex/config.toml` | blocos gerenciados do Codex |
| `~/.local/share/lazyagents/backups/` | `settings.json.<timestamp>` e `config.toml.<timestamp>`, 20 mais recentes de cada |

<a id="safety"></a>

## Segurança

- Toda aplicação e limpeza copia o arquivo do agente ao backup com as mesmas permissões e grava atomicamente por arquivo temporário e renomeação.
- Tokens ficam mascarados: `token ✓` na TUI, `••••` em `provider list` e campo omitido em JSON. Só `provider list --reveal` imprime o valor.
- Toda escrita pela TUI pede confirmação com os arquivos afetados. A validação de provedores não grava nem autentica credenciais. URLs com credenciais embutidas ou fragmentos e nomes inválidos de variáveis são recusados.
- `doctor` informa configuração aplicada por agente e aponta provedores em agentes não instalados ou arquivos ilegíveis.

<a id="limits"></a>

## Limitações

- Apenas Claude Code, Codex, Pi e Crush suportam provedores. Outros só aparecem em `provider list` quando implementarem o recurso.
- Strings multilinha (`"""` ou `'''`) e arrays multilinha em TOML são preservados: linhas internas não são tratadas como chaves, tabelas ou marcadores. Strings/arrays não terminados, blocos quebrados ou aninhados e `[model_providers.lazyagents]` manual fora do bloco gerenciado são recusados, sem alterar o arquivo.
- A identificação compara somente campos públicos configurados. Padrões equivalentes, campos ignorados e valores literais diferentes de token podem manter múltiplos perfis ambíguos; escolha o perfil pelo nome para reaplicar. A correspondência não valida credenciais, resolve ajustes de processo/projeto nem reconstrói o histórico de aplicações.

Use a [aba Recovery ou o CLI config-backups](recovery.md) para localizar backups de configuração, revisar diferenças ocultadas e restaurar um arquivo verificado. A recuperação substitui um arquivo inteiro após confirmação, incluindo alterações não relacionadas mostradas na prévia.

Tentativas de gravação concluídas também aparecem no [histórico de Recovery](recovery.md#recent-operation-history), inclusive após reiniciar. Confira o estado atual antes de repetir; limpar o histórico mantém os backups.
