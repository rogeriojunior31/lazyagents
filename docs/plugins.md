# Plugins externos

Um plugin é um **executável** em `~/.config/lazyagents/plugins/` (ou `$XDG_CONFIG_HOME/lazyagents/plugins/`). O nome do arquivo, sem extensão, é o **id** do plugin: vira a aba na TUI, o subcomando `lazyagents <id>` e a seção `<id>:` do `config.yaml`. Ids casam `^[a-z0-9][a-z0-9_-]{0,31}$` e não podem repetir uma aba embutida (`skills`, `sessions`, `agents`, `providers`, `hooks`, `usage`, `plugins`) nem um comando da CLI.

Qualquer linguagem serve: o contrato é JSON Lines por stdin/stdout. O exemplo completo em shell está em [`examples/plugins/hello`](../examples/plugins/hello).

## Modos de invocação

| Comando | Quando | Contrato |
|---|---|---|
| `<bin> serve` | TUI abre | protocolo abaixo; processo fica vivo até a TUI fechar |
| `<bin> <args…>` | `lazyagents <id> <args…>` | pass-through: stdin/stdout/stderr herdados, exit code repassado |
| `<bin> doctor` | `lazyagents doctor`, só se o manifesto tiver `doctor: true` | escreva os problemas em stdout; exit ≠ 0 = problema |

Os comandos solicitados por `exec` recebem o mesmo ambiente do plugin e são cancelados ao fechar o aplicativo. No Windows, a descoberta aceita executáveis `.exe`; o exemplo em shell requer um ambiente POSIX.

Em todos os modos o ambiente tem `LAZYAGENTS_HOME`, `LAZYAGENTS_CONFIG_DIR`, `LAZYAGENTS_DATA_DIR`, `LAZYAGENTS_LIBRARY_DIR` e `LAZYAGENTS_PROTOCOL`.

## Protocolo v1 (`serve`)

Uma mensagem JSON por linha, UTF-8, no máximo 1 MiB por linha. O campo `type` discrimina. O host é o lazyagents; o plugin lê stdin e escreve em stdout. **stdout é só protocolo**: logs vão para stderr, que o host captura (últimos 4 KiB aparecem na aba quando o plugin falha).

### Host → plugin

| `type` | campos | quando |
|---|---|---|
| `init` | `protocol` (1), `id`, `home`, `configDir`, `dataDir`, `libraryDir`, `theme{id, colors{papel: "#hex"}}` (chaves: tokens como `Primary`/`Bg`/`Info` e papéis como `ui.accent`/`ansi.red`; ver `internal/tui/theme/README.md`), `config` (a seção `<id>:` do `config.yaml`, como JSON; ausente se não houver), `width`, `height` | primeira linha após o spawn |
| `resize` | `width`, `height` | a área útil da aba mudou (já desconta header, abas e margens) |
| `key` | `key`, `text` | tecla, só com a aba ativa. `key` é o nome Bubble Tea: `a`, `enter`, `space`, `esc`, `ctrl+x`, `shift+tab` |
| `paste` | `text` | texto colado |
| `mouse` | `mouse{kind: "wheel"\|"click", x, y, button}` | coordenadas relativas ao corpo da aba |
| `command` | `name` | o usuário escolheu `<id> <name>` na paleta (`:`) |
| `reload` | — | `:reload` ou tecla de recarga |
| `agents` | `agents[{id, name, installed, version, managedDir, readDirs}]` | após a detecção dos agentes (e de novo após um respawn) |
| `exec_result` | `execId`, `code`, `stdout`, `stderr`, `error` | um `exec` terminou (`stdout`/`stderr` só nos não interativos, até 1 MiB) |

Teclas globais do lazyagents (`q`, `?`, `:`, `tab`, `shift+tab`) **não chegam** ao plugin, a menos que o último frame tenha `capturing: true` (use enquanto um input seu estiver com o foco).

### Plugin → host

| `type` | campos | regras |
|---|---|---|
| `manifest` | `title`, `help[{title, keys[[tecla, descrição]]}]`, `commands[{name, desc}]`, `doctor` | **primeira linha, em até 3 s** do `init`. `title` até 40 caracteres; `commands[].name` casa `^[a-z0-9][a-z0-9_-]*$` (aparece na paleta como `<id> <name>`) |
| `frame` | `view`, `count`, `capturing` | o estado completo da aba, a qualquer momento e quantas vezes quiser; o último vence. `count` é o número na aba (omita para não mostrar) |
| `exec` | `execId`, `argv`, `dir`, `interactive` | pede ao host que rode um comando. `interactive: true` suspende a TUI e entrega o terminal ao processo (editor, CLI de agente); senão roda em background e a saída volta em `exec_result` |

`view` é texto com `\n`. Como em todo JSON, caracteres de controle vão escapados (`\u001b[1m` para ESC). Cores via SGR (`ESC[…m`) são mantidas; qualquer outra sequência de escape (mover cursor, limpar tela, OSC) e caracteres de controle são removidos, e o host recorta ao tamanho da aba. O fundo do tema é repintado após cada `ESC[0m`.

### Ciclo de vida e falhas

- O plugin termina quando o stdin fecha (EOF) — trate isso e saia; o host manda `SIGTERM` e, 2 s depois, `SIGKILL`.
- Sem manifesto, linha que não é JSON, `type` vazio, linha > 1 MiB ou processo morto: a aba mostra o erro e o stderr capturado; `:reload` reinicia o plugin. Nada disso derruba a TUI.
- Se o plugin parar de ler stdin (fila de 256 mensagens cheia), o host o encerra.

## Configuração

Coloque a seção com o id do plugin no `config.yaml`; ela chega inteira no `init.config`:

```yaml
hello:
  greeting: olá
```

## Testar

```sh
mkdir -p ~/.config/lazyagents/plugins
cp examples/plugins/hello ~/.config/lazyagents/plugins/hello
chmod +x ~/.config/lazyagents/plugins/hello
lazyagents doctor          # seção "plugins": handshake e `hello doctor`
lazyagents hello a b       # pass-through
lazyagents                 # aba Hello
```

Para depurar o protocolo na mão: `printf '{"type":"init","protocol":1,"width":80,"height":24}\n{"type":"key","key":"x"}\n' | ./hello serve`.
