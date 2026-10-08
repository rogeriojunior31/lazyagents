<p align="center"><img src="docs/assets/logo.svg" width="160" alt="Logo do lazyagents: uma preguiça dormindo na rede enquanto três robôs carregam mini terminais"></p>

<h1 align="center">lazyagents</h1>

<p align="center"><strong>O lazygit dos agentes de código com IA.</strong><br>Skills, sessões, consumo, providers e hooks de todos os agentes num terminal só.</p>

<p align="center">Claude Code · Codex · Gemini CLI · OpenCode · Pi · Crush · Hermes Agent</p>

<p align="center">
  <a href="https://github.com/rogeriojunior31/lazyagents/actions/workflows/ci.yml"><img src="https://github.com/rogeriojunior31/lazyagents/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/rogeriojunior31/lazyagents/releases"><img src="https://img.shields.io/github/v/release/rogeriojunior31/lazyagents" alt="Última versão"></a>
  <a href="https://rogeriojunior31.github.io/docs/lazyagents/"><img src="https://img.shields.io/badge/docs-ler%20online-f2984a" alt="Documentação"></a>
</p>

<p align="center"><a href="README.md">English</a> · <b>Português</b></p>

<p align="center"><img src="docs/assets/demos/hero.gif" width="880" alt="lazyagents: uma skill ativada em todos os agentes com uma tecla, sessões do Claude Code e do Codex numa lista só, um transcript lido como log, limites da assinatura"></p>

## Instalação

```sh
go install github.com/rogeriojunior31/lazyagents@latest
```

Ou baixe um binário para Linux, macOS ou Windows em [Releases](https://github.com/rogeriojunior31/lazyagents/releases). Depois rode:

```sh
lazyagents          # abre a TUI: tab troca de aba, ? mostra as teclas, q sai
lazyagents doctor   # quais agentes e skills o lazyagents encontra
```

## 📖 Documentação

<p align="center"><a href="https://rogeriojunior31.github.io/docs/lazyagents/"><strong>rogeriojunior31.github.io/docs/lazyagents</strong></a></p>

Tudo sobre como usar o lazyagents está na documentação, com busca e atualizada a cada release:

- **[Primeiros passos](https://rogeriojunior31.github.io/docs/lazyagents/getting-started/)**: instalação, primeira skill e primeira sessão retomada em cinco minutos.
- **[Guias](https://rogeriojunior31.github.io/docs/lazyagents/#guias)**: skills, sessões, consumo, providers, hooks, agentes e plugins.
- **[Teclas](https://rogeriojunior31.github.io/docs/lazyagents/reference/keys/)** e **[CLI](https://rogeriojunior31.github.io/docs/lazyagents/reference/cli/)**: todas as teclas e comandos.
- **[Segurança e dados](https://rogeriojunior31.github.io/docs/lazyagents/safety/)**: o que o lazyagents grava, backups, segredos, rede.
- **[Solução de problemas](https://rogeriojunior31.github.io/docs/lazyagents/troubleshooting/)**: problemas comuns e como resolver.

## Tour

<table>
<tr>
<td width="50%"><a href="https://rogeriojunior31.github.io/docs/lazyagents/guide/skills/"><img src="docs/assets/demos/skills.gif" alt="Aba Skills: uma biblioteca, ativada por agente com uma tecla"></a><br><b>Skills</b> · uma biblioteca, ativada por agente com uma tecla</td>
<td width="50%"><a href="https://rogeriojunior31.github.io/docs/lazyagents/guide/sessions/"><img src="docs/assets/demos/sessions.gif" alt="Aba Sessions: o histórico de todos os agentes, com busca e leitura como log"></a><br><b>Sessions</b> · o histórico de todos os agentes, com busca e leitura como log</td>
</tr>
<tr>
<td width="50%"><a href="https://rogeriojunior31.github.io/docs/lazyagents/guide/providers/"><img src="docs/assets/demos/providers.gif" alt="Aba Providers: perfis de API e de modelo local"></a><br><b>Providers</b> · perfis de API e de modelo local</td>
<td width="50%"><a href="https://rogeriojunior31.github.io/docs/lazyagents/guide/hooks/"><img src="docs/assets/demos/hooks.gif" alt="Aba Hooks: um comando, todos os agentes"></a><br><b>Hooks</b> · um comando, todos os agentes</td>
</tr>
<tr>
<td width="50%"><a href="https://rogeriojunior31.github.io/docs/lazyagents/guide/usage/"><img src="docs/assets/demos/usage.gif" alt="Aba Usage: limites e tokens por dia, agente e projeto"></a><br><b>Usage</b> · limites e tokens por dia, agente e projeto</td>
<td width="50%"><a href="https://rogeriojunior31.github.io/docs/lazyagents/guide/agents/"><img src="docs/assets/demos/agents.gif" alt="Aba Agents: o que foi detectado"></a><br><b>Agents</b> · o que foi detectado</td>
</tr>
</table>

Cada clipe abre o seu guia.

> O lazyagents ainda é pré-1.0: é usado no dia a dia, mas comandos e formatos de arquivo ainda podem mudar entre versões menores. Veja o [CHANGELOG](CHANGELOG.md).

## Contribuindo

Issues e pull requests são bem-vindos: veja o [CONTRIBUTING](CONTRIBUTING.md). Questões de segurança vão pelo [SECURITY](SECURITY.md).

## Licença

[MIT](LICENSE). Os temas vêm do [SP Night](https://sp-night.github.io/) e de paletas da comunidade, sob as próprias licenças ([avisos](internal/tui/theme/LICENSES-themes.md)).
