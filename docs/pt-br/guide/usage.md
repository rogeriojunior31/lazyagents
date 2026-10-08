# Consumo
<!-- source: e66f99fd3c00 -->

A aba Usage responde a duas perguntas por agente: quanto resta da sua assinatura e onde seus tokens foram usados. Os limites vêm da conta de cada agente; os tokens vêm das conversas já no disco. A aba é somente leitura.

<a id="concepts"></a>

## Conceitos

- **Janelas de limite:** porcentagem consumida da assinatura em cada janela e horário de renovação. Há janela de sessão (5 horas) e semanal; algumas contas têm limites semanais por modelo. Scripts devem identificar pelo `kind` (`session`, `weekly`, `weekly_model`). `label` é texto de apresentação e pode mudar.
- **Autenticação:** `subscription`, `API key` ou `unknown`. O lazyagents lê o tipo nos arquivos do agente sem guardar segredos; isso determina se mostra custo em dólares.
- **Eventos de consumo:** um por resposta do modelo na conversa, com horário, modelo, pasta e tokens (entrada, saída, leitura e escrita de cache). As tabelas somam esses eventos.
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

A coluna em dólares só aparece se algum agente filtrado usa chave de API. Assinaturas não pagam por token; a coluna fica oculta. Para contas com chave:

- Cada resposta usa o preço do próprio modelo, e as linhas somam as respostas. Sessions usa a mesma regra: mudar de modelo no meio da sessão não faz tudo ser cobrado pelo último modelo.
- Há preços apenas para modelos Claude, conforme a [tabela da API Claude](https://platform.claude.com/docs/en/about-claude/pricing). O programa segue o registro de cobrança do Claude Code: escrita de cache de 1 hora custa 2× a entrada (5 minutos: 1,25×); modo rápido usa preço próprio no Opus 4.8, Opus 5 e Opus 5.5 (outros modelos rápidos ficam sem preço); Batch API custa metade; inferência exclusiva dos EUA (`inference_geo: "us"`) usa 1,1× no Claude 4.6 ou posterior (versões anteriores sem esse recurso ficam sem preço). Priority Tier e outras regiões sem preço publicado são tratados como sem preço.
- Se uma resposta não tiver preço, por exemplo de um modelo desconhecido do Codex, a linha mostra `—` em vez de soma parcial. Respostas sem tokens custam $0 independentemente do modelo; linhas sintéticas de erro de API ou interrupção (`<synthetic>`) não invalidam a soma.
- Pi registra o custo por resposta pelo próprio catálogo; o programa usa esse valor para qualquer modelo. Como Pi pode usar vários provedores, cada resposta considera o seu: OAuth por `/login` (assinatura ChatGPT ou Claude) custa `$0.00`; chave de API em `auth.json` ou `apiKey` de `models.json` mantém o custo do Pi. O valor é usado como registrado: modelos sem preço no Pi, como locais, mostram `$0.00`, sem estimativa da tabela. Pi conta como `API key` se qualquer provedor tiver chave armazenada; a chave fictícia que o lazyagents cria para um perfil sem chave não conta.

A [tabela oficial](https://platform.claude.com/docs/en/about-claude/pricing) é a referência. Trate os valores apresentados como estimativas.

<a id="from-the-cli"></a>

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

Valores desconhecidos mantêm o padrão. Só a TUI usa essa seção; a CLI usa suas flags. Veja [configuração](../configuration.md).

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
