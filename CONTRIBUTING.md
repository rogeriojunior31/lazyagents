# Contributing to lazyagents

Thanks for helping. Bug reports, agent support, docs fixes and new modules are all welcome.

## Before you start

- **Bugs:** open an issue with the output of `lazyagents doctor` and `lazyagents --version`. The template asks for the rest.
- **Features and new agents:** open an issue first, so we agree on the shape before you write code. Planned work is in [BACKLOG.md](BACKLOG.md).
- **Security issues:** do not open a public issue; see [SECURITY.md](SECURITY.md).

Everyone taking part follows the [Code of Conduct](CODE_OF_CONDUCT.md).

## Setup

You need Go 1.26 or newer.

```sh
git clone https://github.com/rogeriojunior31/lazyagents
cd lazyagents
go build -o lazyagents . && ./lazyagents
```

To work on the TUI without touching your real agents, run the preview. It uses fake data and an isolated config:

```sh
go run scripts/preview.go -theme sp-night-garoa -page 2
```

## Where things go

Read [docs/architecture.md](docs/architecture.md) first. In short: a feature is a module in `internal/modules/<name>/`, only `internal/agent/` knows agent paths and formats, and colors live only in `internal/tui/theme/`.

## Rules

- **English** everywhere: code, comments, UI and CLI text, docs, commit messages. `scripts/check-english.sh` runs in CI.
- **Comments say why**, briefly: an invariant, a gotcha, a compatibility or security reason. No comment is better than one that restates the code.
- **User-facing sentences are whole strings** (`fmt.Sprintf("%d skills enabled in %s", n, agent)`), never built from fragments, so they can be translated later.
- **Errors** wrap with context: `fmt.Errorf("reading %s: %w", path, err)`. In the TUI they become toasts, never panics.
- **Tests never touch your real home.** Use `core.PathsIn(t.TempDir())`.
- **User data first.** Writes are atomic, live agent files are backed up first, unknown keys are preserved, secrets stay masked. The full list is in [docs/architecture.md](docs/architecture.md#rules-that-protect-user-data).

## Documentation is part of the change

The docs are checked by `go test ./docs`, so they cannot drift quietly:

- Changed a command, flag, key, palette entry, agent or theme? Update its `Help` or `Help()` in the code, then regenerate the reference with `go test ./docs -update` and commit the result.
- Changed behavior a user would notice? Update the module's guide in `docs/guide/` and add a line to [CHANGELOG.md](CHANGELOG.md) under `Unreleased`.
- Added a module? Add `docs/guide/<name>.md` and link it from [docs/README.md](docs/README.md).

The full table is in [docs/architecture.md](docs/architecture.md#documentation).

## Before opening a PR

```sh
test -z "$(gofmt -l .)" && go vet ./... && go test -race ./... && go build ./...
scripts/check-english.sh
```

CI runs the same steps. If you touched the TUI, try it in a real terminal at a narrow size (80×24) as well as a wide one.

## Commits

[Conventional Commits](https://www.conventionalcommits.org/), imperative, in English:

```
feat(skills): install from zip
fix(sessions): keep the alias after a rename
docs(hooks): explain the Codex trust step
```

One logical change per PR is easier to review than a large one.

## License

By contributing you agree that your contributions are licensed under the [MIT License](LICENSE).
