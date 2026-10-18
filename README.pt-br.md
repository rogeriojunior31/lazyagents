<p align="center"><img src="docs/assets/logo.svg" width="160" alt="Logo do lazyagents: uma preguiça dormindo na rede enquanto três robôs carregam mini terminais"></p>

<h1 align="center">lazyagents</h1>

<p align="center"><strong>O lazygit dos agentes de código com IA.</strong><br>Gerencie seus agentes de código em um só terminal.</p>

<p align="center">Claude Code · Codex · Gemini CLI · OpenCode · Pi · Crush · Hermes Agent</p>

<p align="center">
  <a href="https://github.com/rogeriojunior31/lazyagents/actions/workflows/ci.yml"><img src="https://github.com/rogeriojunior31/lazyagents/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/rogeriojunior31/lazyagents/releases"><img src="https://img.shields.io/github/v/release/rogeriojunior31/lazyagents" alt="Última versão"></a>
  <a href="https://rogeriojunior31.github.io/docs/lazyagents/"><img src="https://img.shields.io/badge/docs-ler%20online-f2984a" alt="Documentação"></a>
</p>

<p align="center"><a href="README.md">English</a> · <b>Português</b></p>

Agentes diferentes guardam skills, conversas e configurações em lugares diferentes. O lazyagents reúne esse trabalho em uma biblioteca e um histórico navegável, mantendo os formatos nativos de cada ferramenta.

[Recursos](#features) · [Instalação](#installation) · [Primeiros passos](#quick-start) · [Suporte](#support) · [Documentação](#documentation)

<p align="center"><a href="docs/assets/demos/hero.mp4"><img src="docs/assets/demos/hero.gif" width="1200" alt="Visão geral com dados de exemplo: skills, conversas e consumo"></a></p>

Gravação com dados de exemplo em **190×50 células**. Clique para abrir o vídeo na resolução original. Este checkout inclui mudanças **Unreleased**; o site e a instalação abaixo seguem a release publicada **0.2.3**.

<a id="features"></a>

## Recursos

| O que você pode fazer | Guia | Demo |
|---|---|---|
| Manter uma biblioteca de skills, ativar por agente e revisar ações em lote | [Skills](docs/pt-br/guide/skills.md) | [Vídeo](docs/assets/demos/skills.mp4) |
| Buscar, retomar, ler e exportar conversas; abrir sessões filhas verificadas | [Sessões](docs/pt-br/guide/sessions.md) | [Vídeo](docs/assets/demos/sessions.mp4) |
| Salvar conexões de API/modelo local e revisar mudanças antes de aplicar | [Provedores](docs/pt-br/guide/providers.md) | [Vídeo](docs/assets/demos/providers.mp4) |
| Gerenciar hooks e revisar os comandos instalados em cada agente | [Hooks](docs/pt-br/guide/hooks.md) | [Vídeo](docs/assets/demos/hooks.mp4) |
| Inspecionar backups, restaurar configurações e consultar operações anteriores | [Recuperação](docs/pt-br/guide/recovery.md) | [Vídeo](docs/assets/demos/recovery.mp4) |
| Acompanhar limites, tokens e custos com origem e cobertura explícitas | [Consumo](docs/pt-br/guide/usage.md) | [Vídeo](docs/assets/demos/usage.mp4) |
| Conferir detecção, recursos e problemas locais com o doctor | [Agentes](docs/pt-br/guide/agents.md) | [Vídeo](docs/assets/demos/agents.mp4) |

<details>
<summary>Ver as demos por módulo em largura inteira</summary>

### Skills

Ative skills por agente e revise destinos em lote.

[![Skills: Ative skills por agente e revise destinos em lote](docs/assets/demos/skills.gif)](docs/assets/demos/skills.mp4)

[Guia](docs/pt-br/guide/skills.md) · [Abrir GIF](docs/assets/demos/skills.gif)

### Sessions

Busca e leitura de conversas, incluindo sessões filhas verificadas.

[![Sessions: Busca e leitura de conversas, incluindo sessões filhas verificadas](docs/assets/demos/sessions.gif)](docs/assets/demos/sessions.mp4)

[Guia](docs/pt-br/guide/sessions.md) · [Abrir GIF](docs/assets/demos/sessions.gif)

### Providers

Revise os dois arquivos Gemini e os modelos de conexão.

[![Providers: Revise os dois arquivos Gemini e os modelos de conexão](docs/assets/demos/providers.gif)](docs/assets/demos/providers.mp4)

[Guia](docs/pt-br/guide/providers.md) · [Abrir GIF](docs/assets/demos/providers.gif)

### Hooks

Revise a instalação antes de ativar um comando.

[![Hooks: Revise a instalação antes de ativar um comando](docs/assets/demos/hooks.gif)](docs/assets/demos/hooks.mp4)

[Guia](docs/pt-br/guide/hooks.md) · [Abrir GIF](docs/assets/demos/hooks.gif)

### Recovery

Inspecione backups e o histórico de operações.

[![Recovery: Inspecione backups e o histórico de operações](docs/assets/demos/recovery.gif)](docs/assets/demos/recovery.mp4)

[Guia](docs/pt-br/guide/recovery.md) · [Abrir GIF](docs/assets/demos/recovery.gif)

### Usage

Compare agentes, projetos e modelos com a origem dos custos.

[![Usage: Compare agentes, projetos e modelos com a origem dos custos](docs/assets/demos/usage.gif)](docs/assets/demos/usage.mp4)

[Guia](docs/pt-br/guide/usage.md) · [Abrir GIF](docs/assets/demos/usage.gif)

### Agents

Confira a detecção e execute diagnósticos locais.

[![Agents: Confira a detecção e execute diagnósticos locais](docs/assets/demos/agents.gif)](docs/assets/demos/agents.mp4)

[Guia](docs/pt-br/guide/agents.md) · [Abrir GIF](docs/assets/demos/agents.gif)

</details>

<a id="installation"></a>

## Instalação

Com **Go 1.26 ou mais recente**:

```sh
go install github.com/rogeriojunior31/lazyagents@v0.2.3
```

Ou baixe um binário para **Linux, macOS ou Windows** em [Releases](https://github.com/rogeriojunior31/lazyagents/releases) e confira o `SHA256SUMS`. O [guia de instalação e atualização](docs/pt-br/getting-started.md#instalação) detalha as plataformas, ferramentas opcionais e receitas locais de Homebrew/Arch. O tap público e o pacote no AUR ainda não foram publicados.

Use a versão explícita acima: durante a renumeração, `@latest` pode resolver uma versão maior retirada. O lazyagents gerencia agentes já existentes; instale suas CLIs separadamente.

<a id="quick-start"></a>

## Primeiros passos

```sh
lazyagents --version
lazyagents doctor --local   # confere detecção e arquivos, sem consultas externas
lazyagents                  # abre a interface
```

1. Em **Skills**, importe uma pasta ou repositório com `i` e revise os destinos de ativação.
2. Em **Sessions**, escolha uma conversa e pressione `v` para ler ou `enter` para retomar na CLI do agente.
3. Use `:` para abrir comandos como `providers`, `recovery` e `agents doctor`.

| Tecla | Ação |
|---|---|
| `tab` / `shift+tab` | trocar de aba |
| `↑` / `↓` | selecionar |
| `?` | ajuda da tela atual |
| `:` | paleta de comandos |
| `esc` | voltar ou cancelar |
| `q` | sair da tela principal |

A [referência de teclas](docs/pt-br/reference/keys.md) cobre as ações por contexto. A [CLI](docs/pt-br/reference/cli.md) oferece listagens e operações sem interface, incluindo saída JSON.

<a id="support"></a>

## Suporte e segurança

Claude Code, Codex, Gemini CLI, OpenCode, Pi, Crush e Hermes Agent têm recursos distintos. Consulte a [matriz gerada e as versões verificadas](docs/pt-br/reference/agents.md) antes de depender de um recurso específico. Hermes oferece skills; Claude Desktop aparece apenas na detecção.

Ativar skills usa links simbólicos; Windows exige a permissão correspondente. Escritas na configuração dos agentes têm backup e revisão; segredos ficam ocultos nas prévias. Limites de assinatura podem consultar a rede quando solicitados. Detalhes em [Segurança e dados](docs/pt-br/safety.md) e [Recuperação](docs/pt-br/guide/recovery.md).

O projeto é pré-1.0. Comandos e formatos podem mudar entre versões menores; veja o [CHANGELOG](CHANGELOG.md).

<a id="documentation"></a>

## Documentação

**[Ler online](https://rogeriojunior31.github.io/docs/lazyagents/)** · **[Índice desta revisão](docs/pt-br/README.md)** · [English](docs/README.md)

| Você quer… | Referência |
|---|---|
| Instalar, atualizar e usar pela primeira vez | [Primeiros passos](docs/pt-br/getting-started.md) |
| Seguir um fluxo de um módulo | [Guias por tarefa](docs/pt-br/README.md#guias) |
| Encontrar um comando, tecla ou recurso por agente | [CLI](docs/pt-br/reference/cli.md) · [Teclas](docs/pt-br/reference/keys.md) · [Agentes](docs/pt-br/reference/agents.md) |
| Configurar caminhos, abas e aparência | [Configuração](docs/pt-br/configuration.md) · [Temas](docs/pt-br/themes.md) |
| Entender escritas, backups ou um problema | [Segurança](docs/pt-br/safety.md) · [Solução de problemas](docs/pt-br/troubleshooting.md) |
| Acrescentar abas e comandos externos | [Plugins](docs/pt-br/guide/plugins.md) |

O README apresenta o produto; os guias detalham os fluxos e limites. Referências de comandos e recursos são geradas do código. Testes verificam links e traduções revisadas; o site publica a documentação da release.

## Contribuindo

Veja [CONTRIBUTING](CONTRIBUTING.md) e a [arquitetura](docs/pt-br/architecture.md). O [BACKLOG](BACKLOG.md) concentra o roadmap, o que foi entregue, as próximas tarefas e seus critérios de aceite. Questões de segurança seguem [SECURITY](SECURITY.md).

## Licença

[MIT](LICENSE). Os temas vêm do [SP Night](https://sp-night.github.io/) e de paletas da comunidade, sob as próprias licenças ([avisos](internal/tui/theme/LICENSES-themes.md)).
