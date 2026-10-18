# Agentes
<!-- source: 83b85ccd2079 -->

A aba Agents oferece uma visão somente leitura de todos os agentes conhecidos: se estão instalados, qual a versão e o que o lazyagents pode gerenciar. Use-a para entender por que um agente não aparece em outra aba ou onde ficam suas skills.

![A aba Agents: os agentes que o lazyagents conhece, quais estão instalados, onde cada um lê as skills e como foi detectado](../../assets/demos/agents.gif)

O suporte de cada agente (diretórios, provedores, hooks, limites e eventos) é gerado do código na [referência dos agentes](../reference/agents.md). Aqui você encontra a explicação da aba.

<a id="the-table"></a>

## A tabela

Uma linha por agente, com os instalados primeiro:

| Coluna na interface | Significado |
|---|---|
| agent | nome na cor do agente; `○` e texto discreto se não instalado |
| version | primeira linha de `<cli> --version` |
| skills | skills ativadas; `—` se não houver diretório local de skills |
| sessions | sessões do agente encontradas no disco |
| hooks · provider · usage | `✓` quando é possível gerenciar hooks, aplicar perfil de provedor ou ler consumo |

O contador da aba é o número de agentes instalados. Em terminais com menos de 75 colunas, as três colunas de recursos passam para uma linha `manages` no detalhe. Em terminais largos, o detalhe fica ao lado da tabela.

<a id="the-detail-pane"></a>

## Painel de detalhes

Para o agente selecionado:

- **skills in:** diretório onde o lazyagents cria os links de skills.
- **reads too:** outros diretórios lidos pelo agente. Uma skill ali aparece mesmo sem ativação pelo lazyagents.
- **detection:** caminho do binário, ou diretório de configuração se o binário não estiver no `PATH`.
- **(shared):** diretório compartilhado, como `~/.agents/skills`. O link fica ali só enquanto a skill estiver ativada para todos os agentes instalados que o leem; veja [diretórios compartilhados](skills.md#concepts).

Um agente não instalado mostra apenas a orientação de instalar sua CLI e reabrir o lazyagents.

<a id="how-detection-works"></a>

## Como funciona a detecção

- O agente conta como instalado se o binário estiver no `PATH` **ou** seu diretório de configuração existir, como `~/.claude` ou `~/.codex`. Com apenas o diretório, o detalhe avisa que o binário não está no `PATH`: skills e sessões ainda funcionam, mas retomar exige a CLI.
- A versão vem de `<cli> --version`, executado uma vez em segundo plano ao iniciar a TUI, com até 6 segundos por CLI. Na CLI do lazyagents, só comandos que precisam da versão, como `doctor`, fazem essa consulta.
- Claude Desktop é detectado apenas pelo diretório de configuração. Skills e conversas ficam na conta claude.ai; não há conteúdo local para gerenciar.
- Hermes Agent ativa skills em `~/.hermes/skills` (`%LOCALAPPDATA%\hermes\skills` no Windows), no perfil de `hermes profile use` ou em `HERMES_HOME`. Também lê `skills.create_dir` e `skills.external_dirs` de `config.yaml`, resolvendo `~`, `${VAR}` e caminhos relativos como o Hermes. Diretórios inexistentes ficam de fora. Incluir `~/.agents/skills` torna Hermes participante desse compartilhamento.
- Pi usa `~/.pi/agent` ou `PI_CODING_AGENT_DIR`. Como outras ferramentas também têm um binário `pi`, sem esse diretório o binário só conta se `pi --version` imprimir apenas uma versão.
- Agentes movidos por suas variáveis (`CLAUDE_CONFIG_DIR`, `CODEX_HOME`, `XDG_DATA_HOME` para OpenCode…) são encontrados ali se o lazyagents executar com o mesmo ambiente. Veja a [referência de variáveis](../reference/agents.md#config-overrides).
- Crush conta como instalado pelo binário, `~/.config/crush` ou `~/.local/share/crush`. Lê skills em `~/.config/crush/skills` (onde são ativadas), `~/.config/agents/skills`, `~/.claude/skills` e `~/.agents/skills`. Se houver `CRUSH_SKILLS_DIR`, usa somente esse diretório, onde o lazyagents também ativa as skills.

A detecção ocorre uma vez por execução. Reabra o lazyagents após instalar uma CLI.

Na CLI, `lazyagents doctor` mostra a mesma detecção na primeira seção; `--json` inclui a versão.

<a id="doctor-report"></a>

## Relatório do doctor

Pressione `d` em Agents ou escolha `agents doctor` na paleta. Abrir o relatório executa os mesmos checks da CLI para sintaxe da configuração, skills, links, provedores e hooks. Os arquivos atuais são lidos sem reparos nem mudanças de ativação. Abrir a aba Agents, por si só, não executa esses checks.

Cada problema identifica gravidade, recurso afetado, causa e próximo passo. A primeira seção com falha abre automaticamente. Use `←/→` para trocar de seção, `↑/↓` ou `pgup/pgdn` para rolar e `enter` para solicitar a aba relacionada. Se ela estiver oculta, use a ação indicada ou a CLI. Caminhos longos e próximos passos continuam acessíveis em terminal 40×16.

Usage e plugins começam como **não executados**. Pressione `n` para incluí-los: Usage pode consultar o endpoint de assinatura de um agente e plugins podem executar código próprio. Usage mantém seu cache normal e identifica resultados em cache ou obtidos na consulta; um resultado em cache não comprova disponibilidade remota atual. `r` sempre executa apenas checks locais, inclusive após uma execução externa. Fechar com `esc` cancela a execução; resultados atrasados não substituem um relatório mais novo. Não há reparos automáticos, checks na inicialização ou repetição remota em segundo plano. A detecção continua sendo a do início da execução; reabra o lazyagents após instalar uma CLI.

A CLI usa o mesmo relatório: `lazyagents doctor` inclui checks externos como antes, enquanto `lazyagents doctor --local` corresponde ao escopo inicial da TUI. `--json` preserva `ok`, `agents` e `title`, `report` e `problems` de cada seção. Campos adicionais incluem `scope`, `cancelled` e, por seção, `findings`, `external`, `skipped`, `next_step` e `tab` opcional. Cada finding contém `severity`, `resource`, `explanation`, `next_step` e `tab` opcional. Código 1 indica problema ou cancelamento; código 0 se refere apenas aos checks executados. Seções externas ignoradas nunca comprovam saúde remota. A configuração atual é relida para detectar erros de sintaxe, sem incluir a entrada bruta do parser nesse diagnóstico. Veja a [referência da CLI](../reference/cli.md#doctor).

<a id="keys"></a>

## Teclas

`↑/↓` seleciona o agente e `shift+↑/↓` rola os detalhes. Veja a [referência completa](../reference/keys.md#agents-tab).

<a id="asking-for-a-new-agent"></a>

## Solicitar um novo agente

O suporte usa um adaptador em `internal/agent`, único lugar que conhece caminhos e formatos dos agentes. Para solicitar um, abra uma issue com nome, locais de skills, sessões e configuração, e links da documentação. Para implementar, veja [Adicionar um agente](../architecture.md#adding-an-agent).
