# Temas
<!-- source: 9bdac9cf30fa -->

As cores da TUI vêm do tema escolhido em `theme:` de `config.yaml`, aplicado na inicialização. Os temas incluídos e seus IDs estão na [referência](reference/themes.md).

<a id="built-in-themes"></a>

## Temas incluídos

A família padrão é **[SP Night](https://sp-night.github.io/)**, inspirada em São Paulo. As variantes `sp-night`, `sp-night-garoa` e `sp-night-jaragua` incluem toda a paleta original e todos os papéis semânticos, sem adaptação. IDs usados até v0.2 (`noite`, `garoa`, `jaragua`) continuam funcionando em `theme:` e `extends:`.

Também há temas da comunidade com paletas oficiais: Tokyo Night, Dracula, Gruvbox, Nord, Rosé Pine, Kanagawa, Everforest e One Dark. Se uma cor original tornar texto ilegível (contraste abaixo de 3:1 contra fundo ou seleção), o papel usa outra da mesma paleta. Se nenhuma passar, o arquivo acrescenta uma cor derivada com comentário.

Para combinar terminal e editor, o SP Night tem versões para outras ferramentas em [sp-night.github.io](https://sp-night.github.io/).

<a id="your-own-theme"></a>

## Seu próprio tema

Crie `~/.config/lazyagents/themes/<id>.yaml` e use `theme: <id>`. Escreva só as mudanças: o restante vem de `extends`, cujo padrão é `sp-night`.

```yaml
# ~/.config/lazyagents/themes/my-dracula.yaml
label: Meu Dracula
extends: dracula
palette:
  purple: "#caa9fa"        # muda papéis herdados que usam purple
ui:
  accent: "#ff79c6"        # destaque: aba ativa e bordas focadas
  bg: "#1e1f29"
diagnostic:
  ok: "#50fa7b"
```

Formato:

| Campo | Significado |
|---|---|
| `id` | opcional; se presente, deve coincidir com o nome do arquivo |
| `label`, `description` | nome do tema e descrição curta |
| `appearance` | `dark` ou `light`; informativo, não muda a renderização |
| `extends` | tema herdado; padrão `sp-night` |
| `palette` | cores nomeadas, `name: "#rrggbb"` |
| `ui`, `diagnostic`, `git`, `syntax`, `ansi` | grupos de papéis; cada valor é hexadecimal entre aspas ou nome da paleta |

Os nomes são resolvidos após a herança: mudar uma cor nomeada altera todos os papéis herdados que a usam.

A lista completa de papéis e o que desenham está em [internal/tui/theme/README.md](../../internal/tui/theme/README.md). Os [arquivos incluídos](../../internal/tui/theme/themes/) são exemplos completos.

<a id="rules"></a>

### Regras

- Não reutilize o ID de um tema incluído; use `extends` para modificá-lo.
- Arquivos inválidos (hexadecimal incorreto, papel desconhecido, ciclo de herança) são ignorados com aviso; os demais carregam.
- Avisos aparecem na saída de erro ao sair da TUI ou antes da saída de um comando.

<a id="plugins-and-themes"></a>

## Plugins e temas

Plugins recebem o tema ativo ao iniciar, com tokens e papéis em hexadecimal, para combinar sua aba. Veja o [protocolo](plugins.md).

<a id="contributing-a-theme"></a>

## Contribuir um tema

Um novo tema incluído é um YAML completo em `internal/tui/theme/themes/`: paleta e todos os papéis, sem `extends`. Papéis de texto devem ter contraste de ao menos 3:1 sobre `ui.bg` e `ui.selection`; `go test ./internal/tui/theme/` verifica. Acrescente a licença da paleta em [LICENSES-themes.md](../../internal/tui/theme/LICENSES-themes.md) e execute `go test ./docs -update` para atualizar a [referência](reference/themes.md).

<a id="licenses"></a>

## Licenças

As paletas são MIT, exceto Tokyo Night (Apache-2.0). Avisos: [LICENSE-SP-Night](../../internal/tui/theme/LICENSE-SP-Night) e [LICENSES-themes.md](../../internal/tui/theme/LICENSES-themes.md).
