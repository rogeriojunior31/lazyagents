# lazyagents documentation

lazyagents is the lazygit for AI coding agents: a terminal UI and CLI that manages, in one place, what your coding agents use: skills, sessions, usage, providers and hooks. New here? Start with [Getting started](getting-started.md).

Also on the web at [rogeriojunior31.github.io/docs/lazyagents](https://rogeriojunior31.github.io/docs/lazyagents/), and fully translated into Brazilian Portuguese: [docs/pt-br](pt-br/README.md).

## Guides

One page per module, organized by task.

| Guide | What it covers |
|---|---|
| [Skills](guide/skills.md) | The skill library, enabling skills per agent, installing from GitHub, folders and zips, profiles, updates and backups |
| [Sessions](guide/sessions.md) | One history across agents: resume, read and export transcripts, search, aliases, cleanup |
| [Usage](guide/usage.md) | Subscription limits, token consumption by day, agent, project and model, cost estimates |
| [Providers](guide/providers.md) | Endpoint and model profiles applied to each agent's live config |
| [Hooks](guide/hooks.md) | A hook library installed per agent, and hooks imported from repositories |
| [Agents](guide/agents.md) | Which agents are detected and what each one supports |
| [Plugins](guide/plugins.md) | Adding tabs and commands with external executables, in any language |

## Topics

- [Configuration](configuration.md): `config.yaml`, tab layout, where every file lives.
- [Themes](themes.md): built-in themes and writing your own.
- [Safety and data](safety.md): what lazyagents writes, backups and restore, secrets, consent.
- [Troubleshooting](troubleshooting.md): common problems and how to fix them.

## Reference

Generated from the code, so they always match the version in this tree.

- [CLI](reference/cli.md): every command and option.
- [Keys](reference/keys.md): every key of every tab, and the command palette.
- [Agent support](reference/agents.md): skill dirs, capabilities and hook events per agent.
- [Built-in themes](reference/themes.md).
- [Plugin protocol](plugins.md): the JSON Lines contract external plugins implement.

## Contributing

- [Architecture](architecture.md): how the code is organized and how to add a module, an agent or a theme.
- [Writing docs](architecture.md#documentation): what is generated, what is written by hand and what the tests check.
- [CONTRIBUTING](../CONTRIBUTING.md), [SECURITY](../SECURITY.md) and the [Code of Conduct](../CODE_OF_CONDUCT.md).
