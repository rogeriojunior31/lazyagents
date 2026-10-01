# Documentação do lazyagents
<!-- source: 030fb9cd6651 -->

O lazyagents é o lazygit dos agentes de código com IA: uma TUI e uma CLI que gerenciam, num lugar só, o que os seus agentes usam: skills, sessões, consumo, providers e hooks. Primeira vez aqui? Comece pelo [Primeiros passos](../getting-started.md).

> Tradução em andamento: as páginas ainda não traduzidas abrem em inglês, que é a versão de referência.

## Guias

Uma página por módulo, organizada por tarefa.

| Guia | O que cobre |
|---|---|
| [Skills](../guide/skills.md) | A biblioteca de skills, ativar skills por agente, instalar do GitHub, de pastas e de zips, perfis, atualizações e backups |
| [Sessões](../guide/sessions.md) | Um histórico só para todos os agentes: retomar, ler e exportar transcripts, buscar, apelidos, limpeza |
| [Consumo](../guide/usage.md) | Limites da assinatura, consumo de tokens por dia, agente, projeto e modelo, estimativa de custo |
| [Providers](../guide/providers.md) | Perfis de endpoint e modelo aplicados na configuração de cada agente |
| [Hooks](../guide/hooks.md) | Uma biblioteca de hooks instalada por agente, e hooks importados de repositórios |
| [Agentes](../guide/agents.md) | Quais agentes são detectados e o que cada um suporta |
| [Plugins](../guide/plugins.md) | Novas abas e comandos com executáveis externos, em qualquer linguagem |

## Tópicos

- [Configuração](../configuration.md): `config.yaml`, layout das abas, onde cada arquivo fica.
- [Temas](../themes.md): temas embutidos e como criar o seu.
- [Segurança e dados](../safety.md): o que o lazyagents escreve, backups e restauração, segredos, consentimento.
- [Solução de problemas](../troubleshooting.md): problemas comuns e como resolver.

## Referência

Gerada a partir do código, então sempre bate com a versão desta árvore.

- [CLI](../reference/cli.md): todos os comandos e opções.
- [Teclas](../reference/keys.md): todas as teclas de todas as abas e a paleta de comandos.
- [Suporte por agente](../reference/agents.md): pastas de skills, recursos e eventos de hook por agente.
- [Temas embutidos](../reference/themes.md).
- [Protocolo de plugins](../plugins.md): o contrato JSON Lines que os plugins externos implementam.

## Contribuir

- [Arquitetura](../architecture.md): como o código é organizado e como adicionar um módulo, um agente ou um tema.
- [CONTRIBUTING](../../CONTRIBUTING.md), [SECURITY](../../SECURITY.md) e o [Código de Conduta](../../CODE_OF_CONDUCT.md).
