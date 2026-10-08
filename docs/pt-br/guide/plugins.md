# Plugins
<!-- source: d0e981c438e9 -->

Um plugin acrescenta sua própria aba, comandos e diagnósticos ao lazyagents sem alterar o código. É um executável em qualquer linguagem que usa JSON Lines pela entrada/saída padrão. Esta página explica instalação, uso e criação; o contrato completo está no [protocolo de plugins](../plugins.md).

<a id="installing-a-plugin"></a>

## Instalar um plugin

Coloque o arquivo no diretório de plugins e dê permissão de execução:

```sh
mkdir -p ~/.config/lazyagents/plugins
cp my-plugin ~/.config/lazyagents/plugins/
chmod +x ~/.config/lazyagents/plugins/my-plugin
```

Se `XDG_CONFIG_HOME` estiver definida, use `$XDG_CONFIG_HOME/lazyagents/plugins/`. Veja os caminhos de macOS/Windows em [configuração](../configuration.md). No Windows, arquivos `.exe` contam como executáveis.

O nome sem extensão é o **ID**. Deve:

- seguir `^[a-z0-9][a-z0-9_-]{0,31}$` (minúsculas, dígitos, `-` e `_`, até 32 caracteres);
- não repetir aba ou comando interno (`skills`, `sessions`, `list`, `doctor`, `help`…);
- ser único: `hello` e `hello.sh` na mesma pasta colidem.

Arquivos inválidos são ignorados com aviso (ao sair da TUI ou antes da saída da CLI) e aparecem em `lazyagents doctor`.

<a id="what-a-plugin-becomes"></a>

## O que o plugin acrescenta

| Local | Recurso |
|---|---|
| TUI | aba com título definido pelo plugin, após as abas de trabalho e antes de Usage/Agents |
| Paleta | comandos `:<id> <comando>` |
| Ajuda (`?`) | lista própria de teclas |
| CLI | `lazyagents <id> [args…]` executa o arquivo com argumentos, fluxos de entrada/saída e código de saída repassados |
| `doctor` | resultado da negociação inicial na seção `plugins` e, se solicitado, diagnóstico próprio |

O processo da aba inicia com a TUI e termina com ela, junto dos processos filhos.

<a id="configuring-a-plugin"></a>

## Configurar um plugin

A seção com o ID em `config.yaml` é entregue inteira ao plugin ao iniciar:

```yaml
hello:
  greeting: hi
```

Para mover ou ocultar, use `tui:` como nas outras abas ([configuração](../configuration.md)). Um plugin oculto nem inicia, mas `lazyagents <id>` continua funcionando:

```yaml
tui:
  hidden: [hello]
```

<a id="trust-and-safety"></a>

## Confiança e segurança

Plugins executam com as permissões do seu usuário, como qualquer programa instalado. O lazyagents não oferece sandbox; instale apenas plugins confiáveis.

A proteção oferecida é contra falhas na TUI:

- Mensagens são não confiáveis. O texto mantém cores, mas perde outros controles de terminal (cursor, limpeza, títulos) e é recortado ao tamanho da aba. Títulos, ajuda e nomes de comandos também são reduzidos e limpos.
- Falta de resposta em 3 segundos, saída inválida ou encerramento deixa a aba em erro, mostrando motivo e final de stderr. `r` ou `:reload` reinicia. O resto do programa continua.
- Se o plugin parar de ler a entrada, é encerrado.

<a id="writing-a-plugin"></a>

## Escrever um plugin

O exemplo mínimo funcional é o script [`examples/plugins/hello`](../../../examples/plugins/hello). Ele mostra os três modos:

1. **`<bin> serve`** executa a aba. Lê `init` (tamanho, caminhos, cores, configuração), responde `manifest` (título, ajuda, comandos, diagnóstico) e envia `frame` com o texto completo quando mudar. Teclas, redimensionamentos e comandos chegam por novas linhas na entrada. Ao fechar a entrada, termina.
2. **`<bin> doctor`** só executa se o manifesto tiver `"doctor": true`. Imprima os resultados; código diferente de zero indica problema. Não usa terminal; a saída aparece ao terminar. Após 30 segundos, falha e encerra plugin e filhos.
3. **`<bin> <qualquer outra coisa>`** recebe os argumentos de `lazyagents <id> …` diretamente.

Experimente:

```sh
cp examples/plugins/hello ~/.config/lazyagents/plugins/hello
chmod +x ~/.config/lazyagents/plugins/hello
lazyagents doctor       # negociação inicial e diagnóstico do hello
lazyagents hello a b    # argumentos repassados
lazyagents              # aba Hello repete cada tecla
```

Dicas:

- **stdout é exclusivo do protocolo.** Logs vão para stderr; o programa guarda os últimos 4 KiB para exibir em falhas.
- Toda execução recebe `LAZYAGENTS_HOME`, `LAZYAGENTS_CONFIG_DIR`, `LAZYAGENTS_DATA_DIR`, `LAZYAGENTS_LIBRARY_DIR` e `LAZYAGENTS_PROTOCOL`.
- Para abrir editor ou CLI de agente, peça ao hospedeiro com `exec` em vez de iniciar sozinho: ele suspende a TUI e entrega o terminal.
- Para depurar sem TUI, envie linhas manualmente: `printf '{"type":"init","protocol":1,"width":80,"height":24}\n' | ./my-plugin serve`.

Campos, limites e ciclo de vida estão no [protocolo](../plugins.md). Mudanças incompatíveis incrementam `protocol` em `init`.
