# Provedores
<!-- source: aa199db2592d -->

Um perfil de provedor define endpoint, modelo e token com um nome. O lazyagents escreve esses valores na configuração do agente para alternar Claude Code ou Codex entre API oficial, proxy e modelo local sem editar JSON/TOML manualmente. Funciona como [cc-switch](https://github.com/farion1231/cc-switch): só altera configuração, nunca executa um proxy. Agentes compatíveis e arquivos estão na [referência](../reference/agents.md#capabilities).

![A aba Providers: um perfil de modelo local e outro de API, a ajuda contextual e a confirmação mostrada antes de aplicar um perfil ao Claude Code](../../assets/demos/providers.gif)

<a id="concepts"></a>

## Conceitos

- **Perfil:** `name`, `baseUrl`, `model`, `token`, `envKey` e `wireApi`. Precisa definir pelo menos endpoint, modelo ou token. Nomes têm até 40 caracteres.
- **Biblioteca:** `~/.config/lazyagents/providers.json`, com modo `0600`, pois pode conter tokens.
- **Aplicado:** estado atual da configuração do agente, lido diretamente do arquivo. Uma configuração manual também aparece como provedor definido fora do lazyagents. A correspondência com a biblioteca usa endpoint e também modelo se dois perfis compartilharem o endpoint.
- **Limpar:** remove apenas o que o lazyagents gerencia e restaura o estado anterior. O resto fica intacto.

<a id="in-the-tui"></a>

## Na TUI

A aba Providers é uma matriz perfil × agente: `●` aplicado, `○` padrão do agente e `–` agente não instalado. O detalhe mostra endpoint, modelo, estado do token (`token ✓`, nunca o valor) e configuração aplicada por agente. Veja as [teclas](../reference/keys.md#providers-tab).

<a id="create-a-profile"></a>

### Criar um perfil

`n` abre o formulário com nome, endpoint, modelo, token, variável de ambiente e API. `tab` troca de campo e `enter` salva. `ctrl+v` verifica a compatibilidade do rascunho com os agentes instalados sem salvar ou aplicar; `esc` volta ao formulário. A verificação usa a mesma validação da aplicação, inclusive a presença do token salvo durante a edição. O token fica mascarado. `e` edita o perfil: deixar o token vazio mantém o salvo, que nunca é carregado no formulário. Renomear também renomeia na biblioteca.

<a id="apply-or-clear-per-agent"></a>

### Aplicar ou limpar por agente

Escolha a coluna com `←/→` e pressione `space` para validar aquele destino. `a` abre a seleção com os agentes instalados que aceitam provedores marcados. Cada linha indica se o perfil está pronto ou incompatível; selecione a linha para ler o motivo, os arquivos afetados e os requisitos de ativação. `space` inclui ou exclui agentes e `enter` revisa a seleção na confirmação existente. Agentes incompatíveis marcados impedem a confirmação: escolher só os compatíveis é uma ação explícita. `shift+↑/↓` ou `ctrl+u/d` rolam os requisitos; `esc` cancela sem gravar.

A validação verifica a sintaxe do endpoint e da referência de credencial, requisitos de protocolo/modelo por agente e os arquivos e registros necessários à alteração. Todos os destinos selecionados são verificados antes de gravar configurações, backups ou registros de propriedade. Token sem `envKey` para Codex, protocolo incompatível, bloco gerenciado inválido ou configuração ilegível bloqueiam a aplicação inteira. Os arquivos são relidos na gravação. Autenticação e suporte real de endpoint/modelo não são testados: o nome válido de uma variável é apenas uma referência. A validação não lê nem exporta seu valor. Inicie uma nova sessão do agente após aplicar, para carregar a configuração e as variáveis necessárias.

Repetir `space` no perfil já aplicado limpa. `x` limpa o provedor de todos os agentes instalados. `d` remove da biblioteca, mas não desfaz configurações já aplicadas; use `x` ou limpe por agente.

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

O resultado fica aberto até ser dispensado. Role relatórios longos, use `r` para confirmar uma nova tentativa apenas nos IDs com falha, `b` para abrir Recovery ou `esc`/`enter` para voltar à matriz atualizada. A repetição lê a entrada salva atual e mantém os resultados anteriores de sucesso ou omissão; não repete a seleção original de todos os agentes. Consulte a [referência de teclas](../reference/keys.md) e o [guia de recuperação](recovery.md). Se Recovery estiver oculta, use `lazyagents config-backups list`.

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

lazyagents provider validate proxy --json   # sem gravações ou rede
lazyagents provider validate local --agents codex,pi
lazyagents provider apply local --agents codex,pi
lazyagents provider apply proxy --agent claude-code
lazyagents provider apply local --agent codex
lazyagents provider list            # tokens como •••• ou $VAR para envKey
lazyagents provider list --json     # sem tokens; usa "has_token": true
lazyagents provider clear           # todos os agentes instalados
lazyagents provider rm proxy
```

Sem `--agent` ou `--agents`, `apply` valida todos os agentes instalados que aceitam provedores; um destino incompatível impede todas as gravações. Escolha explicitamente com `--agent id` ou `--agents id,id`. `clear` mantém `--agent` e o padrão de todos os agentes instalados. Falhas durante a gravação ainda podem ser parciais: os demais destinos são processados e os erros são reunidos. Veja os [comandos e flags](../reference/cli.md#provider).

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
- Perfis aplicados são identificados por endpoint e modelo. Dois com os mesmos valores são indistinguíveis.

Use a [aba Recovery ou o CLI config-backups](recovery.md) para localizar backups de configuração, revisar diferenças ocultadas e restaurar um arquivo verificado. A recuperação substitui um arquivo inteiro após confirmação, incluindo alterações não relacionadas mostradas na prévia.
