# Plugins externos
<!-- source: 3cff707e1a1d -->

Um plugin é um **executável** em `~/.config/lazyagents/plugins/` ou `$XDG_CONFIG_HOME/lazyagents/plugins/`. O nome sem extensão é o **ID**: vira aba da TUI, subcomando `lazyagents <id>` e seção `<id>:` de `config.yaml`. IDs seguem `^[a-z0-9][a-z0-9_-]{0,31}$` e não podem repetir abas internas (`skills`, `sessions`, `agents`, `providers`, `hooks`, `usage`, `plugins`) nem comandos da CLI.

Qualquer linguagem serve: o contrato usa JSON Lines na entrada/saída padrão. Há um exemplo shell completo em [`examples/plugins/hello`](../../examples/plugins/hello).

<a id="invocation-modes"></a>

## Modos de execução

| Comando | Quando | Contrato |
|---|---|---|
| `<bin> serve` | abre a TUI | protocolo abaixo, processo vivo até fechar a TUI |
| `<bin> <args…>` | `lazyagents <id> <args…>` | repassa stdin/stdout/stderr e código de saída |
| `<bin> doctor` | `doctor`, se o manifesto tiver `doctor: true` | problemas em stdout; código diferente de zero indica problema; sem terminal, mostra saída ao terminar; após 30 s falha e encerra árvore de processos |

Comandos solicitados por `exec` recebem o ambiente do plugin e são cancelados ao fechar o aplicativo. No Windows, aceita executáveis `.exe`; o exemplo shell exige ambiente POSIX.

Todos os modos recebem `LAZYAGENTS_HOME`, `LAZYAGENTS_CONFIG_DIR`, `LAZYAGENTS_DATA_DIR`, `LAZYAGENTS_LIBRARY_DIR` e `LAZYAGENTS_PROTOCOL`.

<a id="protocol-v1-serve"></a>

## Protocolo v1 (serve)

Uma mensagem JSON por linha, UTF-8, até 1 MiB. `type` identifica a mensagem. O hospedeiro é o lazyagents; o plugin lê stdin e escreve stdout. **stdout é exclusivo do protocolo**: logs vão para stderr, capturado pelo hospedeiro; os últimos 4 KiB aparecem na aba se houver falha.

<a id="host--plugin"></a>

### Hospedeiro → plugin

| `type` | Campos | Quando |
|---|---|---|
| `init` | `protocol` (1), `id`, `home`, `configDir`, `dataDir`, `libraryDir`, `theme{id, colors{role: "#hex"}}` (tokens `Primary`/`Bg`/`Info` e papéis `ui.accent`/`ansi.red`; veja `internal/tui/theme/README.md`), `config` (seção `<id>:` de `config.yaml` em JSON, omitida se ausente), `width`, `height` | primeira linha após iniciar |
| `resize` | `width`, `height` | área útil mudou, já descontando cabeçalho, abas e margens |
| `key` | `key`, `text` | tecla pressionada com aba ativa; nomes Bubble Tea: `a`, `enter`, `space`, `esc`, `ctrl+x`, `shift+tab` |
| `paste` | `text` | texto colado |
| `mouse` | `mouse{kind: "wheel"\|"click", x, y, button}` | coordenadas relativas ao corpo da aba |
| `command` | `name` | escolha de `<id> <name>` na paleta (`:`) |
| `reload` | — | `:reload` ou tecla de recarga |
| `agents` | `agents[{id, name, installed, version, managedDir, readDirs}]` | após detecção dos agentes e novamente após reiniciar o plugin |
| `exec_result` | `execId`, `code`, `stdout`, `stderr`, `error` | `exec` terminou; stdout/stderr só em execução não interativa, até 1 MiB |

Teclas globais (`q`, `?`, `:`, `tab`, `shift+tab`, `1`–`9`) **não chegam** ao plugin, salvo se o último quadro tiver `capturing: true`, usado enquanto um campo tem foco.

<a id="plugin--host"></a>

### Plugin → hospedeiro

| `type` | Campos | Regras |
|---|---|---|
| `manifest` | `title`, `help[{title, keys[[key, description]]}]`, `commands[{name, desc}]`, `doctor` | **primeira linha, em até 3 s** de `init`; título até 40 caracteres; nomes de comandos seguem `^[a-z0-9][a-z0-9_-]*$`, mostrados como `<id> <name>` |
| `frame` | `view`, `count`, `capturing` | estado completo da aba, a qualquer momento; último prevalece; `count` é o contador da aba, omitido para ocultar |
| `exec` | `execId`, `argv`, `dir`, `interactive` | pede execução ao hospedeiro; interativa suspende a TUI e entrega o terminal (editor/CLI), não interativa executa em segundo plano e retorna saída em `exec_result` |

`view` é texto com `\n`. Caracteres de controle usam escapes JSON (`\u001b[1m` para ESC). Cores SGR (`ESC[…m`) são mantidas; outros controles (cursor, limpeza, OSC) são removidos. O hospedeiro recorta ao tamanho da aba e repinta o fundo após cada `ESC[0m`.

<a id="lifecycle-and-failures"></a>

### Ciclo de vida e falhas

- Ao fechar stdin (EOF), o plugin deve sair. O hospedeiro envia `SIGTERM` e, 2 s depois, `SIGKILL`. No Linux/macOS, há um grupo próprio e os sinais atingem todo o grupo; processos restantes são encerrados após a saída. No Windows, o plugin inicia suspenso, entra num Job Object com encerramento ao fechar e só então executa: nada escapa. A parada encerra a árvore (`taskkill /T` durante execução); restos do job são encerrados após sair, inclusive se o lazyagents morrer. O mesmo vale para `exec` não interativo; o interativo mantém o terminal e executa como filho comum.
- Sem manifesto, linha não JSON, `type` vazio, linha maior que 1 MiB ou processo morto: a aba mostra erro e stderr; `:reload` reinicia. A TUI continua funcionando.
- Se o plugin parar de ler stdin e a fila de 256 mensagens encher, o hospedeiro o encerra.

<a id="configuration"></a>

## Configuração

Crie uma seção com o ID em `config.yaml`; ela chega inteira em `init.config`:

```yaml
hello:
  greeting: hi
```

<a id="testing"></a>

## Testes

```sh
mkdir -p ~/.config/lazyagents/plugins
cp examples/plugins/hello ~/.config/lazyagents/plugins/hello
chmod +x ~/.config/lazyagents/plugins/hello
lazyagents doctor          # negociação inicial e `hello doctor`
lazyagents hello a b       # argumentos repassados
lazyagents                 # aba Hello
```

Para depurar manualmente: `printf '{"type":"init","protocol":1,"width":80,"height":24}\n{"type":"key","key":"x"}\n' | ./hello serve`.
