# Temas do lazyagents

Cada tema é um YAML com o mesmo formato do [SP Night](https://github.com/sp-night/sp-night):
uma `palette` de cores com nome e os papéis semânticos em cinco grupos. O código de UI nunca
usa hex: consome tokens (`theme.Primary`, `theme.SynString`, `theme.ANSI("red")`), que
resolvem contra o tema ativo na hora de desenhar. Por isso os estilos de pacote acompanham
o `theme:` do `config.yaml`, que `main` aplica antes da TUI.

```yaml
id: meu-tema          # opcional; se vier, igual ao nome do arquivo
label: Meu Tema
description: "…"
appearance: dark      # dark | light
extends: noite        # só temas do usuário; padrão noite
palette:
  laranja: "#f2984a"
ui:
  accent: laranja     # "#rrggbb" ou um nome da palette
```

## Origem dos temas

- **SP Night** (`noite`, `garoa`, `jaragua`): gerados por `generate/main.go`, que copia a
  paleta (`palette/sp_night.json`, 23 cores por flavor) e os papéis (`palette/roles.json`)
  sem alteração. Aviso MIT em [LICENSE-SP-Night](LICENSE-SP-Night).

  ```sh
  go run ./internal/tui/theme/generate -source ~/Projects/SP-Night/sp-night
  go run ./internal/tui/theme/generate -source ~/Projects/SP-Night/sp-night -check
  ```

- **Comunidade** (`tokyonight*`, `dracula`, `gruvbox-dark`, `nord`, `rose-pine*`, `kanagawa`,
  `everforest-dark`, `onedark`): escritos à mão a partir da paleta oficial de cada projeto
  (URL no cabeçalho de cada arquivo), com os papéis distribuídos como o próprio projeto faz
  no editor. Avisos de licença em [LICENSES-themes.md](LICENSES-themes.md).

- **Usuário**: `<ConfigDir>/themes/<id>.yaml`, carregados por `LoadUser` no boot. Papel
  ausente vem de `extends`, e as referências à `palette` são resolvidas depois da herança:
  trocar uma cor com nome no tema filho repinta todo papel herdado que a usa.

Todo tema embutido precisa definir todos os papéis sem `extends`; `load_test.go` garante isso.

## Papéis e tokens

| Grupo | Papéis | Tokens (o que pinta na TUI) |
| --- | --- | --- |
| `ui` | `bg`, `bg_deep`, `panel`, `float`, `line`, `selection` | `Bg` fundo, `Deep` texto da pílula da aba ativa, `Surface` cards, `Float`, `Line`, `Sel` linha selecionada |
| `ui` | `border`, `border_active` | `Border`, `BorderFocus` |
| `ui` | `fg_bright`, `fg`, `fg_dim`, `fg_muted` | `Bright` títulos, `Text`, `Subtle` dicas e rótulos, `Muted` |
| `ui` | `accent`, `accent_alt`, `cursor`, `link`, `match`, `on_accent` | `Primary` identidade (aba ativa, Claude Code), `Accent` secundário (Gemini CLI), `Cursor`, `Link`, `Match`, `OnAccent` |
| `diagnostic` | `error`, `warn`, `info`, `hint`, `ok` | `Err`, `Warn`, `Info`, `Hint`, `OK` (toasts, estados, OpenCode) |
| `git` | `added`, `modified`, `removed`, `renamed`, `staged`, `untracked`, `conflict` | `Added`/`Removed` no diff de perfil de skills, e os demais |
| `syntax` | `keyword`, `function`, `type`, `string`, `number`, `constant`, `comment`, `tag`, `punctuation` e mais 17 | `SynString` código no markdown, `SynPunct` cercas, `SynComment` citações e frontmatter, `SynKeyword`… |
| `ansi` | `black` … `bright_white` (16) | `ANSI(nome)`, cores de agentes sem cor própria |

Plugins recebem no `init` o mapa `theme.colors` com cada token e cada papel (`ui.accent`,
`ansi.red`…) em hex.

`Paint` reaplica as cores de uma superfície depois de cada reset ANSI dentro dela (`\e[m`,
`\e[39m`, `\e[49m`), para que spans aninhados não abram buracos no fundo de um card.
