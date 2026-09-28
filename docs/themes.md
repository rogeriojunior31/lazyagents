# Themes

The TUI colors come from a theme, picked with `theme:` in `config.yaml` and applied at startup. The list of built-in themes, with their ids, is in the [built-in themes reference](reference/themes.md).

## Built-in themes

The default family is **[SP Night](https://sp-night.github.io/)**, a palette with São Paulo as its reference. Its three flavors (`sp-night`, `sp-night-garoa`, `sp-night-jaragua`) ship with the full original palette and every semantic role, unadapted. The ids used up to v0.2 (`noite`, `garoa`, `jaragua`) still work in `theme:` and `extends:`.

Well-known community themes ship too, with their official palettes: Tokyo Night, Dracula, Gruvbox, Nord, Rosé Pine, Kanagawa, Everforest and One Dark. Where the original color of a role would leave text unreadable (contrast below 3:1 against the background or the selection), the role uses another color of the same palette. When no color of the palette passes, the file adds a derived one, marked with a comment.

To give your terminal and editor the same look, SP Night has ports for other tools at [sp-night.github.io](https://sp-night.github.io/).

## Your own theme

Create `~/.config/lazyagents/themes/<id>.yaml` and set `theme: <id>`. You only write what changes: anything missing comes from the theme in `extends` (default `sp-night`).

```yaml
# ~/.config/lazyagents/themes/my-dracula.yaml
label: My Dracula
extends: dracula
palette:
  purple: "#caa9fa"        # repaints every dracula role that uses purple
ui:
  accent: "#ff79c6"        # main highlight: active tab, focused borders
  bg: "#1e1f29"
diagnostic:
  ok: "#50fa7b"
```

The format:

| Field | Meaning |
|---|---|
| `id` | Optional. If present it must match the file name |
| `label`, `description` | What the theme is called and a line about it |
| `appearance` | `dark` or `light`. Informative only: it does not change the drawing |
| `extends` | The theme to inherit from. Default `sp-night` |
| `palette` | Named colors, `name: "#rrggbb"` |
| `ui`, `diagnostic`, `git`, `syntax`, `ansi` | Role groups. Each role takes `"#rrggbb"` (quoted) or a `palette` name |

Palette names are resolved after inheritance, so changing a named color in your theme repaints every inherited role that uses it.

The full list of roles, and what each one paints in the TUI, is in [internal/tui/theme/README.md](../internal/tui/theme/README.md). The built-in files in [internal/tui/theme/themes/](../internal/tui/theme/themes/) are complete examples.

### Rules

- A file cannot reuse a built-in theme's id. To change a built-in theme, `extends` it.
- An invalid file (bad hex, unknown role, an `extends` cycle) is skipped with a warning; the other themes still load.
- Warnings print on stderr when the TUI exits, or before the output of a CLI command.

## Plugins and themes

Plugins receive the active theme when they start, with every token and role as hex, so their tabs can match. See the [plugin protocol](plugins.md).

## Contributing a theme

A new built-in theme is a complete YAML file in `internal/tui/theme/themes/`: palette plus every role, with no `extends`, and text roles at a contrast of at least 3:1 over `ui.bg` and `ui.selection`. `go test ./internal/tui/theme/` enforces both. Add the license notice of the original palette to [LICENSES-themes.md](../internal/tui/theme/LICENSES-themes.md), then run `go test ./docs -update` so the [reference](reference/themes.md) lists it.

## Licenses

The palettes are MIT, except Tokyo Night (Apache-2.0). Notices: [LICENSE-SP-Night](../internal/tui/theme/LICENSE-SP-Night) and [LICENSES-themes.md](../internal/tui/theme/LICENSES-themes.md).
