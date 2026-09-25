# Changelog

## Unreleased

### Changed

- English is now the project language. The TUI, CLI text, error messages and themes are in English; docs are next.
- SP Night themes are renamed: `sp-night` (default), `sp-night-garoa` and `sp-night-jaragua`. The old ids `noite`, `garoa` and `jaragua` still work in `theme:` and in a user theme's `extends:`. Plugins now receive `sp-night`… as `theme.id` in `init`.
- Dates are shown in ISO order (`2026-09-25`, `Tue 09-23`) and relative times in English (`3min ago`, `yesterday`).
- `usage --json`: rate-limit window `label` values are now in English (`session 5h`, `week`, `week · <model>`). `label` is display text and may change again; scripts should match on `kind` (`session`, `weekly`, `weekly_model`). Other human-readable text in `--json` output is English too: `auth` (`subscription`, `unknown`), placeholder session titles (`(no prompt)`, `(untitled)`), agent details and error messages.
- Codex `config.toml`: the comments that delimit the lazyagents managed blocks are now written in English. Blocks written by earlier versions are still recognized and are rewritten on the next provider apply or clear.
- The usage limits cache (`usage-cache.json`) has a new format; a cache from an earlier version is discarded once and fetched again.
