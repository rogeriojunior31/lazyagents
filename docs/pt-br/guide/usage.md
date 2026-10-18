# Consumo
<!-- source: 5b3507c20967 -->

A aba Usage responde a duas perguntas por agente: quanto resta da sua assinatura e onde seus tokens foram usados. Os limites vêm da conta de cada agente; os tokens vêm das conversas já no disco. A aba é somente leitura.

![Limites e consumo de exemplo por agente, projeto e modelo, com origem dos custos e cobertura](../../assets/demos/usage.gif)

[Abrir vídeo na resolução original](../../assets/demos/usage.mp4).

<a id="concepts"></a>

## Conceitos

- **Janelas de limite:** porcentagem consumida da assinatura em cada janela e horário de renovação. Há janela de sessão (5 horas) e semanal; algumas contas têm limites semanais por modelo. Scripts devem identificar pelo `kind` (`session`, `weekly`, `weekly_model`). `label` é texto de apresentação e pode mudar.
- **Autenticação:** `subscription`, `API key` ou `unknown`. O lazyagents lê o tipo nos arquivos do agente sem guardar segredos; isso permite estimativas internas de API; custo nativo e declarações explícitas de cobrança para fallback são informados separadamente.
- **Eventos de consumo:** um por resposta do modelo na conversa, com horário, provider/modelo registrados, pasta e tokens (entrada, saída, leitura e escrita de cache). As tabelas somam esses eventos.
- **Bloco atual:** a atividade é dividida em blocos de 5 horas, como a janela de sessão do Claude Code. O bloco começa na primeira resposta após um intervalo e dura 5 horas. O atual contém o momento presente.

Os agentes que informam limites e histórico estão na [referência](../reference/agents.md#capabilities).

<a id="in-the-tui"></a>

## Na TUI

Nada é carregado até você abrir a aba pela primeira vez. As teclas estão na [referência](../reference/keys.md#usage-tab). Os limites de cada agente são blocos lado a lado conforme a largura disponível. Quando sobra espaço à direita (cerca de 180 colunas), os principais agentes, modelos e projetos aparecem ao lado da tabela. As barras crescem até 40 colunas.

<a id="check-your-limits"></a>

### Consultar limites

O topo mostra uma barra por janela, com porcentagem usada, renovação e plano ou tipo de autenticação. A leitura varia por agente:

- **Claude Code:** consulta o mesmo endpoint usado por `/usage`, com a autenticação da CLI. Só acontece ao abrir ou atualizar a aba, nunca na inicialização.
- **Codex:** lê os limites registrados nos rollouts das últimas sessões. Sem consulta à rede; a atualização depende da última sessão Codex.

Os resultados ficam em cache por 5 minutos. `r` consulta novamente ignorando o cache. Se falhar, mantém os limites anteriores com aviso. Agentes sem limites a informar (não instalados, não autenticados, com chave de API ou nunca usados) não aparecem nos limites nem nessa parte do `doctor`.

<a id="filter-by-period-agent-and-view"></a>

### Filtrar por período, agente e visualização

- `p/P` avançam e voltam no período: hoje, 7, 30 ou 90 dias, ou todo o histórico.
- `a/A` alternam entre todos os agentes e cada agente individualmente.
- `←/→` ou `v` alternam dias, agentes, projetos e modelos. Projetos usam o caminho completo; pastas com nomes iguais não se misturam.
- `/` filtra linhas pelo nome. Na visualização diária também aceita o dia da semana (`Tue`). `esc` limpa o texto; um segundo `esc` volta a todos os agentes.

Mudar filtros não relê as conversas. A visualização diária inclui dias sem consumo, mantendo o gráfico contínuo.

Os filtros também estão na paleta (`:`): `:usage period 30d`, `:usage view projects`, `:usage clear` e `:usage refresh`.

<a id="read-the-current-block"></a>

### Ler o bloco atual

Abaixo dos limites, o resumo mostra o bloco atual de 5 horas, se houver: início, tempo restante e tokens já usados. Use-o para acompanhar a proximidade do fim da janela.

<a id="cost"></a>

### Custo

O custo é resolvido **por resposta**, antes de agrupar. A visão `models` separa providers registrados; identidade ausente permanece desconhecida e não herda a conexão atual. USD registrado pelo agente, inclusive zero, prevalece sobre estimativas. Esse valor pode vir do catálogo do próprio agente e **não é uma fatura verificada**.

- **Registrado:** OpenCode 2 e Pi mantêm o custo nativo. O tratamento OAuth já existente do Pi continua marcando chamadas cobertas como zero; credenciais não são importadas para eventos de uso.
- **Estimado:** a tabela Claude mantém origem e data de revisão (2026-09-30), com multiplicadores de tier/cache somente onde suportados. Nos demais casos, `usage.prices` pode fornecer uma tarifa de fallback validada para identidade exata. Valores que contêm estimativas recebem `~`.
- **Indisponível:** `—` indica falta de tarifa aplicável, tarifa obrigatória de cache, contadores incompletos ou cobrança não API. Uma resposta indisponível torna o custo agregado indisponível; componentes conhecidos não viram um total completo. Respostas Claude `<synthetic>` de erro/interrupção sem tokens continuam com estimativa zero.

O resumo mostra quantidades de respostas com custo registrado, estimado e indisponível, origem/data/moeda e contadores incompletos. O JSON acrescenta `cost_kind`, `cost_sources`, `recorded_responses`, `estimated_responses`, `unpriced_responses`, `missing_usage_responses`, `cost_unavailable_reasons` e `tokens_complete`; `cost_usd` só aparece para um agregado observado inteiramente precificado. Custos positivos pequenos usam seis casas (ou limiar positivo de exibição), sem aparentar gratuidade. Tudo é USD, sem conversão de moeda. O JSON mantém todas as fontes quando o resumo textual excede quatro. A data informa revisão da tarifa, não verificação automática de atualidade nem período histórico de cobrança.

<a id="recorded-history-and-freshness"></a>

### Histórico registrado e atualidade

O JSONL do Gemini CLI **0.64.0** repete snapshots do mesmo ID durável. O índice substitui o registro anterior de cada resposta, preservando modelo e horário registrados; anexar, reiniciar e reescrever não multiplica tokens. Cache sai da entrada total de prompt e pensamentos entram na saída gerada. Contadores ausentes/inacabados e tokens de ferramentas sem mapeamento verificado permanecem explicitamente incompletos. O registro não fornece provider/autenticação; a conexão ou login atual não é projetado sobre o histórico.

O OpenCode **2.0.32** guarda uma linha SQLite mutável de assistente por resposta. Consultas somente de metadados preservam provider/modelo registrados, entrada/saída/raciocínio/cache e custo USD opcional. Raciocínio entra na saída gerada e cache já vem separado da entrada. Respostas em andamento ficam com custo indisponível; ausência de tokens/custo não vira zero. Esquemas antigos de uso OpenCode aguardam evidência independente. Não há novas consultas de limites Gemini/OpenCode.

Ambos são comprovados por [goldens de fixtures nativas](../../../internal/agent/testdata/fixtures/README.md), com valores sintéticos de modelo local. Referências: [gravação Gemini](https://github.com/google-gemini/gemini-cli/blob/4a0749aceac5643e6f85a36d7420314d85e13b52/packages/core/src/services/chatRecordingService.ts), [contadores GenerateContent](https://ai.google.dev/api/generate-content#UsageMetadata), [categorias OpenCode](https://github.com/anomalyco/opencode/blob/843a6c514e606b436a7fe713422ace0a93ad3d61/packages/schema/src/token-usage.ts) e [schema/custo de assistente](https://github.com/anomalyco/opencode/blob/843a6c514e606b436a7fe713422ace0a93ad3d61/packages/schema/src/session-message.ts).

Os relatórios informam horário da leitura local, resposta registrada mais recente e quantidades de sessões indisponíveis, sem uso e sem suporte. Períodos CLI filtram pelo **horário da resposta**, incluindo transcrições copiadas com mtime antigo. A cobertura é o inventário de sessões disponível, não toda a conta: OpenCode lista até 500 sessões raiz. Conversas filhas verificadas podem ser abertas no [leitor de Sessions](sessions.md#read-a-transcript), mas seu uso não é somado recursivamente aos totais das sessões raiz. Falha de leitura não vira uso zero; avisos de histórico parcial explicam que os totais cobrem somente respostas observadas. Limites mantêm seus próprios horários de consulta/cache.

## Pela CLI

```sh
lazyagents usage                       # painel: limites, bloco atual e resumo do período
lazyagents usage limits                # apenas janelas de limite
lazyagents usage daily --since 30d     # tokens por dia nos últimos 30 dias
lazyagents usage projects --limit 0    # todos os projetos, não só os 10 primeiros
lazyagents usage models --agent claude-code --since 2026-09-01
lazyagents usage limits --refresh --json
```

`--since` aceita dias (`7d`, desde meia-noite), horas (`24h`) ou data (`2026-09-01`). `usage limits` retorna código `1` se nenhum agente informar limites, útil em scripts. As visualizações e opções estão na [referência da CLI](../reference/cli.md#usage).

<a id="configuration"></a>

## Configuração

A seção `usage:` de `config.yaml` define os filtros iniciais da aba:

```yaml
usage:
  period: 30d      # today | 7d | 30d | 90d | all (padrão: 7d)
  view: projects   # daily | agents | projects | models (padrão: daily)
```

Valores desconhecidos mantêm o padrão. Filtros iniciais só afetam a TUI; os preços de fallback também valem nos relatórios Usage da CLI. Veja [configuração](../configuration.md).

<a id="files"></a>

## Arquivos

| Caminho | Conteúdo |
|---|---|
| `~/.local/share/lazyagents/usage-cache.json` | limites consultados nos últimos 5 minutos; pode ser apagado |
| `~/.local/share/lazyagents/transcript-index.gob` | índice compartilhado com Sessions: tokens por intervalos de 15 minutos; pode ser apagado |

Esta aba não escreve arquivos dos agentes.

<a id="limits"></a>

## Limitações

- Só contam conversas locais: consumo em outra máquina ou em sessões excluídas não entra.
- O endpoint de limites do Claude Code não é uma API pública e pode mudar. Nesse caso, a aba mostra erro e mantém o último cache.
- Os limites do Codex dependem da última sessão nesta máquina. Janelas já vencidas mostram 0% sem horário de renovação, pois o uso registrado era anterior à renovação.
- Crush registra custo por sessão, mas só tokens da última requisição; não tem histórico aqui. Seu custo aparece em Sessions.
- Pi não mostra limites de assinatura, apenas tokens e custo. Chaves de provedor vindas só de variáveis de ambiente não são detectadas; essas chamadas mantêm o custo do Pi apenas se outro provedor Pi usar chave armazenada.

<a id="exact-fallback-api-prices"></a>

### Tarifas API de fallback exatas

A lista `usage.prices` vale para relatórios Usage da CLI e TUI. Filtros iniciais de período/visão continuam exclusivos da TUI. Exemplo **sintético**, não uma cotação Gemini:

```yaml
usage:
  prices:
    - agent: gemini-cli
      provider: ""              # identidade ausente exata, não um curinga
      model: private-model      # ID registrado exato
      tier: ""
      billing: api              # declaração do usuário sobre este histórico
      currency: USD
      source: https://pricing.example.test/rates
      date: "2026-10-18"
      input: 2                  # USD por milhão de tokens de entrada nova
      output: 8                 # inclui raciocínio/pensamentos gerados
      cacheRead: 0.5
      cacheWrite: 0
      cacheWrite1h: 0
```

O agente deve ser Claude Code, Codex, Gemini CLI, OpenCode ou Pi (use seu ID). Agente/provider/modelo/tier devem coincidir exatamente, sem aliases/curingas. Entrada/saída, `billing: api`, USD, data ISO não futura e fonte HTTPS sem credenciais/query/fragment são obrigatórios. Tarifas de cache só podem faltar quando a categoria não é usada; omissão nunca significa grátis. Zero explícito é permitido. Tarifas devem ser finitas, não negativas e no máximo um bilhão de USD por milhão de tokens. São aceitas até 256 identidades únicas. Duplicatas, campos desconhecidos e conjuntos inválidos desativam todos os fallbacks configurados com aviso sem valores; tarifas internas verificadas e custos nativos continuam disponíveis.

Se a autenticação histórica Gemini/OpenCode não está registrada, `billing: api` é sua declaração explícita para aquela identidade; o lazyagents não autentica nem comprova a cobrança passada. Contas reconhecidas como assinatura não recebem estimativa. Tarifas internas já aplicáveis não são substituídas e custo registrado sempre prevalece. Provider ausente só coincide com `provider: ""`; não use o perfil ativo para preencher o histórico. A tarifa vale para todas as respostas observadas correspondentes; a data é proveniência, não início de vigência nem inferência de tier.

Reinicie após editar config. Alterações recalculam agregados sem reescrever transcrições ou invalidar o índice de tokens. Não há consulta de preços, catálogo ou inferência. Sessions mantém seu comportamento de custo/preço existente; os fallbacks pertencem aos **relatórios Usage**. `≥` marca linhas com contadores parciais; confira as evidências antes de interpretar uma soma como completa.
