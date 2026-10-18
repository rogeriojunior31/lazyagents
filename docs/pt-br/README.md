# Documentação do lazyagents
<!-- source: e7b680f5108b -->

O lazyagents é o lazygit dos agentes de código com IA: uma TUI e uma CLI que gerenciam, num lugar só, o que os seus agentes usam: skills, sessões, consumo, providers, hooks e recuperação. Primeira vez aqui? Comece pelo [Primeiros passos](getting-started.md).

Todos os guias, tópicos e referências deste índice têm tradução para português. Comandos, opções, identificadores e mensagens literais do programa mantêm sua forma original para que os exemplos funcionem e os erros possam ser reconhecidos.

## Guias

Uma página por módulo, organizada por tarefa.

| Guia | O que cobre |
|---|---|
| [Skills](guide/skills.md) | A biblioteca de skills, ativar skills por agente, instalar do GitHub, de pastas e de zips, perfis, atualizações e backups |
| [Sessões](guide/sessions.md) | Um histórico só para todos os agentes: retomar, ler e exportar conversas, buscar, apelidos, limpeza |
| [Provedores](guide/providers.md) | Perfis de endpoint e modelo aplicados na configuração de cada agente |
| [Hooks](guide/hooks.md) | Uma biblioteca de hooks instalada por agente, e hooks importados de repositórios |
| [Recuperação](guide/recovery.md) | Encontrar backups de configurações, revisar mudanças ocultadas e restaurar um arquivo verificado; histórico retido de operações |
| [Consumo](guide/usage.md) | Limites da assinatura, consumo de tokens por dia, agente, projeto e modelo, custos registrados, estimativas e cobertura |
| [Agentes](guide/agents.md) | Quais agentes são detectados e o que cada um suporta |
| [Plugins](guide/plugins.md) | Novas abas e comandos com executáveis externos, em qualquer linguagem |

## Tópicos

- [Configuração](configuration.md): `config.yaml`, disposição das abas, onde cada arquivo fica.
- [Temas](themes.md): temas incluídos e como criar o seu.
- [Segurança e dados](safety.md): o que o lazyagents escreve, backups e restauração, segredos, consentimento.
- [Solução de problemas](troubleshooting.md): problemas comuns e como resolver.

## Referência

Traduzida da referência gerada a partir do código. As marcas de revisão são verificadas para impedir a publicação quando o original muda sem revisão da tradução.

- [CLI](reference/cli.md): todos os comandos e opções.
- [Teclas](reference/keys.md): todas as teclas de todas as abas e a paleta de comandos.
- [Suporte por agente](reference/agents.md): pastas de skills, recursos e eventos de hook por agente.
- [Temas incluídos](reference/themes.md).
- [Protocolo de plugins](plugins.md): o contrato JSON Lines que os plugins externos implementam.

## Contribuir

- [Arquitetura](architecture.md): como o código é organizado e como adicionar um módulo, um agente ou um tema.
- [Escrever documentação](architecture.md#documentation): referências geradas, guias manuais e verificações de atualização.
- [Gravar demos](architecture.md#recording-product-demos): dados de exemplo, dimensões, ferramentas e revisão visual.
- [CONTRIBUTING](../../CONTRIBUTING.md), [SECURITY](../../SECURITY.md) e o [Código de Conduta](../../CODE_OF_CONDUCT.md).
