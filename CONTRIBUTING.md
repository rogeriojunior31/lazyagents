# Contributing to lazyagents

Thanks for helping. Bug reports, agent support, docs fixes and new modules are all welcome.

## Before you start

- **Bugs:** open an issue with the output of `lazyagents doctor` and `lazyagents --version`. The template asks for the rest.
- **Features and new agents:** open an issue first, so we agree on the shape before you write code. Planned work is in [BACKLOG.md](BACKLOG.md).
- **Security issues:** do not open a public issue; see [SECURITY.md](SECURITY.md).

[BACKLOG.md](BACKLOG.md) is the single roadmap: delivered work, the next task,
why each change matters, code entry points, construction guidance, references,
dependencies and acceptance/release criteria. Update it in place; keep new
findings with their implementation steps instead of creating companion plans.

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

To refresh the README and guide clips, use `scripts/record-demo.sh [name...]`.
See [recording product demos](docs/architecture.md#recording-product-demos) for
the isolated sample data, Linux dependencies, viewport and visual checks.

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
- Changed a public workflow? Review both READMEs, the task guide and its demo together. Keep the same overview/installation/quick-start/support/docs structure in English and Portuguese; put full behavior and limits in the guide. Regenerate affected clips instead of retaining an older flow under a new caption.
- Review each affected `docs/pt-br/` translation before updating its source hash. Required translations, generated references and links must pass `go test ./docs`; the website still follows published releases. Before tagging, align both installation pins and the checkout/release notice with the chosen revision.

The full table is in [docs/architecture.md](docs/architecture.md#documentation).

## Before opening a PR

```sh
test -z "$(gofmt -l .)" && go vet ./... && go test -race ./... && go build ./...
scripts/check-english.sh
```

CI runs the same steps. If you touched the TUI, try it in a real terminal at a narrow size (80×24) as well as a wide one.

## Releases

The current published baseline is **0.2.3**. Keep refinements, fixes and documentation work
on **0.2.x**; a merged PR does not need its own release. Group related changes
into a tested release.

The consolidated roadmap targets M27 at 0.2.4 and M28 at 0.2.5. Earlier
per-milestone version labels were planning batches, not published releases.
These are patch targets, not deadlines or a requirement to publish each task
separately. MCP management is the proposed 0.3.0 milestone; instructions
and environment profiles are the proposed 0.4.0 milestone. Neither minor is
ready until its own acceptance and release gates pass.

A minor bump needs an explicit milestone and acceptance criteria in BACKLOG.md,
including any compatibility changes. A new adapter or layout adjustment alone
is not a reason to advance the minor version. Version 1.0 requires a documented
compatibility commitment and validation of the supported platforms and core flows.

Release builds take their version from the tag; development builds keep `dev`.
Never move a published tag. The 2026-10-17 consolidation is a one-time exception:
the former 0.3.x releases became 0.2.1, and 0.4.x–0.5.x plus subsequent work became
0.2.2. Go module mirrors may retain those retired versions, so installation
examples pin the consolidated version rather than using `@latest`.

## Package recipes

The checked-in [Homebrew formula](packaging/homebrew/Formula/lazyagents.rb) and
[Arch binary recipe](packaging/aur/lazyagents-bin/PKGBUILD) pin published 0.2.3
archives, not the unreleased working tree. See [local installation and upgrade](docs/getting-started.md#local-homebrew-and-arch-recipes).
Neither a public tap nor an AUR package has been published. Publication status,
native results and missing dependencies live in [M28.1](BACKLOG.md#m281-package-installation-and-upgrade-paths).

After a new release exists, download its `SHA256SUMS`, review the archives and
update both recipes from the repository root:

```sh
gh release download v0.2.3 --pattern SHA256SUMS --dir /path/to/release
go run scripts/update-packages.go -version 0.2.3 -checksums /path/to/release/SHA256SUMS
(cd packaging/aur/lazyagents-bin && makepkg --printsrcinfo > .SRCINFO)
go test -race scripts/update-packages.go scripts/update-packages_test.go
```

Replace 0.2.3 with the published version being packaged. The generator requires
the complete five-archive manifest for exactly that version; missing, duplicate,
mixed-version or malformed checksums fail before either recipe is replaced.
It only writes local recipes: no network, tag creation, release publication or
package-manager execution. `.SRCINFO` comes from native `makepkg`, not a second
handwritten metadata format. Always review it together with `PKGBUILD`.

[Packaging CI](.github/workflows/packaging.yml) compares generated recipes to
published checksums, checks native `.SRCINFO`, builds the Arch package and
exercises install/upgrade/uninstall with version, license and user-data checks.
The Homebrew job runs on macOS; Arch runs inside the official
`archlinux:base-devel` container with non-root `makepkg`. `UPGRADE_FROM` is the
explicit previous-release baseline; review it when advancing the packaged
version. Linux checks can use a disposable Homebrew prefix outside `/tmp` and
`/var/tmp` (Homebrew's Linux sandbox keeps those temporary roots writable).
Do not run destructive package-manager checks against an existing installation.

To publish after native checks pass, review/copy `Formula/lazyagents.rb` into a
maintainer-owned Homebrew tap repository, then submit the reviewed recipe.
For AUR, verify the package name/account ownership, configure the maintainer's
AUR SSH key and push `PKGBUILD` plus the generated `.SRCINFO` to its package
repository. Neither this repository nor its CI stores publishing credentials
or pushes those repositories. Do not document remote install commands until
the tap/AUR listing is available and native installation from it is verified.
Primary references: [Homebrew taps](https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap),
[formula cookbook](https://docs.brew.sh/Formula-Cookbook),
[Arch package guidelines](https://wiki.archlinux.org/title/Arch_package_guidelines)
and [.SRCINFO](https://wiki.archlinux.org/title/.SRCINFO).

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
