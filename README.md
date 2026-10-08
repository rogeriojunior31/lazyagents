<p align="center"><img src="docs/assets/logo.svg" width="160" alt="lazyagents logo: a sloth asleep in a hammock while three robots carry mini terminals"></p>

<h1 align="center">lazyagents</h1>

<p align="center"><strong>The lazygit for AI coding agents.</strong><br>Skills, sessions, usage, providers and hooks of every agent in one terminal.</p>

<p align="center">Claude Code · Codex · Gemini CLI · OpenCode · Pi · Crush · Hermes Agent</p>

<p align="center">
  <a href="https://github.com/rogeriojunior31/lazyagents/actions/workflows/ci.yml"><img src="https://github.com/rogeriojunior31/lazyagents/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/rogeriojunior31/lazyagents/releases"><img src="https://img.shields.io/github/v/release/rogeriojunior31/lazyagents" alt="Latest release"></a>
  <a href="https://rogeriojunior31.github.io/en/docs/lazyagents/"><img src="https://img.shields.io/badge/docs-read%20online-f2984a" alt="Documentation"></a>
</p>

<p align="center"><b>English</b> · <a href="README.pt-br.md">Português</a></p> <!-- check-english:allow (language name) -->

<p align="center"><img src="docs/assets/demos/hero.gif" width="880" alt="lazyagents: a skill enabled in every agent with one key, sessions from Claude Code and Codex in one list, a transcript read as a log, subscription limits"></p>

## Install

```sh
go install github.com/rogeriojunior31/lazyagents@latest
```

Or download a binary for Linux, macOS or Windows from [Releases](https://github.com/rogeriojunior31/lazyagents/releases). Then run:

```sh
lazyagents          # open the TUI: tab switches tabs, ? shows the keys, q quits
lazyagents doctor   # what agents and skills lazyagents sees
```

## 📖 Documentation

<p align="center"><a href="https://rogeriojunior31.github.io/en/docs/lazyagents/"><strong>rogeriojunior31.github.io/en/docs/lazyagents</strong></a></p>

Everything about using lazyagents lives in the docs, with search, updated with every release:

- **[Getting started](https://rogeriojunior31.github.io/en/docs/lazyagents/getting-started/)**: install, first skill and first resumed session in five minutes.
- **[Guides](https://rogeriojunior31.github.io/en/docs/lazyagents/#guides)**: skills, sessions, usage, providers, hooks, agents and plugins.
- **[Keys](https://rogeriojunior31.github.io/en/docs/lazyagents/reference/keys/)** and **[CLI](https://rogeriojunior31.github.io/en/docs/lazyagents/reference/cli/)**: every key and command.
- **[Safety and data](https://rogeriojunior31.github.io/en/docs/lazyagents/safety/)**: what lazyagents writes, backups, secrets, network.
- **[Troubleshooting](https://rogeriojunior31.github.io/en/docs/lazyagents/troubleshooting/)**: common problems and fixes.

## Tour

<table>
<tr>
<td width="50%"><a href="https://rogeriojunior31.github.io/en/docs/lazyagents/guide/skills/"><img src="docs/assets/demos/skills.gif" alt="Skills tab: one library, enabled per agent with a keypress"></a><br><b>Skills</b> · one library, enabled per agent with a keypress</td>
<td width="50%"><a href="https://rogeriojunior31.github.io/en/docs/lazyagents/guide/sessions/"><img src="docs/assets/demos/sessions.gif" alt="Sessions tab: every agent's history, searched and read as a log"></a><br><b>Sessions</b> · every agent's history, searched and read as a log</td>
</tr>
<tr>
<td width="50%"><a href="https://rogeriojunior31.github.io/en/docs/lazyagents/guide/providers/"><img src="docs/assets/demos/providers.gif" alt="Providers tab: API and local-model profiles"></a><br><b>Providers</b> · API and local-model profiles</td>
<td width="50%"><a href="https://rogeriojunior31.github.io/en/docs/lazyagents/guide/hooks/"><img src="docs/assets/demos/hooks.gif" alt="Hooks tab: one command, every agent"></a><br><b>Hooks</b> · one command, every agent</td>
</tr>
<tr>
<td width="50%"><a href="https://rogeriojunior31.github.io/en/docs/lazyagents/guide/usage/"><img src="docs/assets/demos/usage.gif" alt="Usage tab: limits and tokens per day, agent, project"></a><br><b>Usage</b> · limits and tokens per day, agent, project</td>
<td width="50%"><a href="https://rogeriojunior31.github.io/en/docs/lazyagents/guide/agents/"><img src="docs/assets/demos/agents.gif" alt="Agents tab: what was detected"></a><br><b>Agents</b> · what was detected</td>
</tr>
</table>

Each clip opens its guide.

> lazyagents is pre-1.0: it is used daily, but commands and file formats may still change between minor versions. See the [CHANGELOG](CHANGELOG.md).

## Contributing

Issues and pull requests are welcome: see [CONTRIBUTING](CONTRIBUTING.md). Security issues go through [SECURITY](SECURITY.md).

## License

[MIT](LICENSE). The themes come from [SP Night](https://sp-night.github.io/) and community palettes, under their own licenses ([notices](internal/tui/theme/LICENSES-themes.md)).
