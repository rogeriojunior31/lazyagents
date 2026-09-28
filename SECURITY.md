# Security policy

lazyagents edits your AI agents' configuration, stores provider tokens, runs hooks and loads plugins, so we take security reports seriously.

## Supported versions

lazyagents is pre-1.0. Fixes land on `main` and in the next release; only the latest release is supported.

## Reporting a vulnerability

Please **do not open a public issue**. Report it privately through GitHub: on the repository page, open **Security → Report a vulnerability**.

Include what you can:

- the version (`lazyagents --version`) and your OS;
- the steps to reproduce, or a proof of concept;
- what an attacker gains (read a token, write outside the expected files, run a command…).

You can expect an acknowledgement within a week. Once a fix is released, the advisory is published with credit to you, unless you prefer otherwise.

## Scope

In scope, for example:

- a secret (provider token, agent credential) reaching a log, an error, `--json` output, the screen or a file with a permission wider than 0600;
- a write outside the files lazyagents documents in [docs/safety.md](docs/safety.md), or a write that loses content it should preserve;
- path traversal from a skill, a zip, a repository, a hook or a backup;
- a plugin, a skill or a repository escaping the sanitization lazyagents applies to it (terminal escape sequences, manifest fields).

Out of scope:

- what a hook, skill or plugin does once **you** chose to install and run it: they run with your permissions by design ([docs/safety.md](docs/safety.md#third-party-content));
- vulnerabilities in the agents themselves (Claude Code, Codex, Gemini CLI, OpenCode…), which should go to their maintainers.
