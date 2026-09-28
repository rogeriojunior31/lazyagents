<p align="center"><img src="docs/assets/logo.svg" width="180" alt="lazyagents logo: a sloth asleep in a hammock while three robots carry mini terminals"></p>

# lazyagents

[![CI](https://github.com/rogeriojunior31/lazyagents/actions/workflows/ci.yml/badge.svg)](https://github.com/rogeriojunior31/lazyagents/actions/workflows/ci.yml)

A terminal UI to manage, in one place, what your AI coding agents use: **skills**, **sessions**, **usage**, **providers** and **hooks**. Works with Claude Code, Codex, Gemini CLI, OpenCode and Hermes Agent; Claude Desktop is detected only ([what each agent supports](docs/reference/agents.md)). Inspired by [cc-switch](https://github.com/farion1231/cc-switch) and lazygit.

A **skill** is a folder with a `SKILL.md` of instructions that an agent loads when a task matches ([Agent Skills](https://agentskills.io)). Every agent keeps its own copy in its own dir; lazyagents keeps one library and links each skill into the agents you choose.

![demo](demo.gif)

> lazyagents is pre-1.0: it is used daily, but commands and file formats may still change between minor versions. Changes are recorded in the [CHANGELOG](CHANGELOG.md).

## What it does

- **[Skills](docs/guide/skills.md):** one library for every agent. Install from GitHub, a folder or a zip, then enable each skill per agent with a keypress. Enabling is a symlink, so nothing is copied or lost. Also covers adopting local skills, profiles, updates from the source and backups.
- **[Sessions](docs/guide/sessions.md):** one history across Claude Code, Codex, Gemini CLI and OpenCode. Resume any session in its own CLI and folder, read transcripts as chat, search across all of them, give them aliases, clean up with backups.
- **[Usage](docs/guide/usage.md):** subscription limits (session and weekly windows, reset times) and token consumption by day, agent, project and model.
- **[Providers](docs/guide/providers.md):** endpoint and model profiles applied to each agent's live config, cc-switch style, with a preview of every change and a backup before writing. Tokens are never shown.
- **[Hooks](docs/guide/hooks.md):** a hook library installed per agent, including hooks shipped by skill repositories. Hooks you wrote yourself are left alone.
- **[Plugins](docs/guide/plugins.md):** any executable in the plugins dir becomes a tab, a command and a `doctor` section. Plugins can be written in any language.
- **Headless CLI** with `--json` for scripts and for the agents themselves ([reference](docs/reference/cli.md)).

## Install

```sh
go install github.com/rogeriojunior31/lazyagents@latest
```

Or download a binary from [Releases](https://github.com/rogeriojunior31/lazyagents/releases) for Linux, macOS or Windows, with `SHA256SUMS` to verify.

The test suite runs on Linux, macOS and Windows in CI; day-to-day use is on Linux, so reports from the other systems are very welcome. On Windows, enabling skills needs permission to create symlinks (Developer Mode).

## Quick start

```sh
lazyagents doctor              # what agents and skills lazyagents sees
lazyagents                     # open the TUI: tab switches tabs, ? shows the keys, q quits
lazyagents install user/repo   # install skills from a GitHub repository
lazyagents enable my-skill --agent claude-code
lazyagents sessions --here     # sessions started in this folder
lazyagents usage               # limits and consumption
```

The [getting started guide](docs/getting-started.md) walks through it in five minutes.

## Documentation

| | |
|---|---|
| [Getting started](docs/getting-started.md) | Install, first skill, first resumed session |
| [Guides](docs/README.md#guides) | Skills, sessions, usage, providers, hooks, agents, plugins |
| [Configuration](docs/configuration.md) · [Themes](docs/themes.md) | `config.yaml`, tab layout, file locations, 14 built-in themes and your own |
| [Safety and data](docs/safety.md) | What lazyagents writes, backups and restore, secrets, network |
| [Troubleshooting](docs/troubleshooting.md) | Common problems and fixes |
| [Reference](docs/README.md#reference) | Every command, key, agent capability and theme, generated from the code |

## How it stays safe

lazyagents only changes an agent's files when you act. Config writes are confirmed in the TUI and backed up first, keys it does not manage are preserved, and a real skill folder is never deleted. Tokens are stored with mode 0600 and never displayed unless you ask with `provider list --reveal`. There is no network call at startup and no telemetry. Details: [Safety and data](docs/safety.md).

## Contributing

Issues and pull requests are welcome: see [CONTRIBUTING](CONTRIBUTING.md) and the [architecture overview](docs/architecture.md). Security issues go through [SECURITY](SECURITY.md). Planned work is in [BACKLOG.md](BACKLOG.md).

## License

[MIT](LICENSE). The themes come from [SP Night](https://sp-night.github.io/) and community palettes, under their own licenses ([notices](internal/tui/theme/LICENSES-themes.md)).
