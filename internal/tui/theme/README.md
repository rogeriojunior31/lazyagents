# SP Night for lazyagents

The bundled Noite Paulista, Garoa and Pico do Jaraguá themes are generated from
the canonical SP Night palette and semantic roles. Noite Paulista is the default.
The upstream MIT notice is preserved in [LICENSE-SP-Night](LICENSE-SP-Night).

The mapping lives in `generate/main.go`. UI code only consumes semantic tokens:

| Tokens | SP Night roles |
| --- | --- |
| Primary / BorderFocus | ui.accent / ui.border_active |
| Accent | ui.accent_alt |
| Bg / Deep / Surface / Sel | ui.bg / ui.bg_deep / ui.panel / ui.selection |
| Text / Bright / Subtle / Muted | ui.fg / ui.fg_bright / ui.fg_dim / ui.fg_muted |
| Border | ui.border |
| OK / Warn / Err | diagnostic.ok / diagnostic.warn / diagnostic.error |

From the lazyagents repository root:

```sh
go run ./internal/tui/theme/generate -source ~/Projects/SP-Night/sp-night
go run ./internal/tui/theme/generate -source ~/Projects/SP-Night/sp-night -check
```

The generator resolves roles first, then palette values, and produces one JSON
file per flavor under `themes/`. All three files are embedded in the binary;
there is no runtime dependency on the source checkout or the network.

Color tokens implement `color.Color` and resolve against the active palette at
render time, so the package-level styles follow the theme chosen in
`config.json`, which `main` applies once before the TUI starts.

`Paint` re-applies a surface's colors after every ANSI reset inside it (`\e[m`,
`\e[39m`, `\e[49m`), so nested spans never punch holes in a card background.
