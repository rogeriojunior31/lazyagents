# Hooks
<!-- source: 266b6180138b -->

Um hook é um comando de shell executado por um agente quando algo acontece: começa uma sessão, uma ferramenta está prestes a executar ou termina um turno. O lazyagents mantém hooks numa biblioteca e os instala nos agentes compatíveis. Você pode escrevê-los ou importar de repositórios com hooks de plugins do Claude Code. Agentes e eventos disponíveis estão na [referência](../reference/agents.md#hook-events).

<a id="concepts"></a>

## Conceitos

- **Biblioteca:** `~/.local/share/lazyagents/hooks/`, um JSON `<name>.json` por hook, editável manualmente.
- **Entrada:** contém nome e um ou mais comandos, cada um com `event`, `matcher` opcional e `command`. Um pacote importado pode reunir muitos comandos e eventos e ser instalado como unidade. Comandos individuais ainda podem ser desativados.
- **Identidade:** um hook instalado é reconhecido pela combinação de evento, filtro e comando. O lazyagents não adiciona marcador no arquivo do agente. Hooks sem correspondência na biblioteca são **externos**: apenas contados, nunca editados ou removidos.
- **Alcance:** cada comando é instalado apenas nos agentes que emitem seu evento. Em pacotes com vários eventos, cada agente recebe os comandos compatíveis.

Exemplo de entrada na biblioteca:

```json
{
  "name": "notify",
  "description": "notificação na área de trabalho",
  "hooks": [
    {
      "event": "Stop",
      "command": "notify-send \"Claude terminou\""
    }
  ]
}
```

Outros campos: `matcher` filtra, por exemplo, uma ferramenta em `PreToolUse`; `timeout` define segundos; `async: true` executa sem bloquear o turno; `off` lista índices dos comandos desativados. Entradas importadas incluem `source` e `files`, com origem e pasta de scripts. O nome deve coincidir com o arquivo e usar letras, dígitos, `-` ou `_`, até 40 caracteres.

<a id="in-the-tui"></a>

## Na TUI

A aba Hooks é uma matriz hook × agente: `●` instalado, `◐` parcial (faltam comandos), `○` não instalado, `–` agente sem nenhum dos eventos. As teclas estão na [referência](../reference/keys.md#hooks-tab).

<a id="install-or-uninstall-per-agent"></a>

### Instalar ou desinstalar por agente

Escolha a coluna com `←/→` e pressione `space`. A confirmação mostra `evento → comando`, arquivo a reescrever e avisos do agente. Pressionar novamente desinstala. `a` instala em todos os agentes compatíveis e `x` desinstala de todos. `d` apaga da biblioteca, mas mantém os hooks já instalados: desinstale primeiro para removê-los de todos os lugares. Scripts importados são apagados junto com o hook, salvo se outra entrada ainda os usar.

<a id="switch-single-commands-of-a-pack"></a>

### Alternar comandos individuais de um pacote

`enter` abre a lista da entrada selecionada. `space` ou `enter` ativa/desativa um comando; `esc` volta. Se instalada em agentes, a mudança é aplicada neles após confirmação. Caso contrário, só a biblioteca muda.

<a id="read-and-edit-commands-and-scripts"></a>

### Ler e editar comandos e scripts

`v` abre um leitor em tela cheia com o comando e os scripts referenciados (`.sh`, `.py`, `.js` e similares). `←/→` alterna e `e` edita em `$EDITOR`. Antes de salvar, a confirmação compara o texto antigo e novo. Salvar um comando atualiza todos os agentes onde está instalado e desfaz se algum falhar. Salvar um script faz backup e recusa se o arquivo mudou no disco desde a abertura.

<a id="import-hooks-from-a-repository"></a>

### Importar hooks de um repositório

Plugins do Claude Code declaram hooks em `<plugin>/hooks/hooks.json`. Na instalação com `i` em Skills, aparecem junto com as skills, inicialmente desmarcados, porque executam comandos de terceiros a cada evento.

Na importação:

- Pastas referenciadas por `${CLAUDE_PLUGIN_ROOT}` são copiadas com a estrutura original, junto com `hooks/`. Outras pastas do plugin não são copiadas.
- Cada comando recebe `export CLAUDE_PLUGIN_ROOT='<cópia>'; ` antes do original, e `${CLAUDE_PLUGIN_ROOT}` vira `$CLAUDE_PLUGIN_ROOT`, pois Claude Code recusa a forma com chaves fora dos próprios plugins. As aspas originais são mantidas.
- Uma entrada reúne todos os comandos de cada plugin. A importação é integral ou não acontece; nomes já existentes são recusados.

Comandos importados usam o protocolo do Claude Code. Outros agentes podem fornecer dados diferentes pela entrada padrão; a confirmação lembra a origem do hook.

<a id="codex-hooks-switch-and-trust"></a>

### Codex: ativação de hooks e confiança

Codex guarda hooks em `~/.codex/hooks.json` e mais dois itens em `config.toml`: `[features] hooks = true` e `trusted_hash` por comando. O hash registra seu consentimento. O lazyagents instala o hook, mas nunca escreve esses itens, pois isso aprovaria execução em seu nome. Em vez disso, avisa na confirmação, em `hooks list` e `doctor`:

- Hooks estão desligados no Codex: defina `hooks = true` em `[features]` de `config.toml`.
- Um novo hook só executa depois de você confirmar a confiança no próprio Codex.

Claude Code não tem essa etapa: o que estiver em `settings.json` executa.

### Crush

Crush executa apenas `PreToolUse` e lê respostas no formato do Claude Code. O lazyagents cria linhas `hook add` num bloco ao fim de `~/.config/crush/crushrc`, com nome `lazyagents-<id>` por hook, permitindo removê-los separadamente. O resto permanece intacto. `hooks list` também mostra hooks de `crush.json`, que nunca é editado; a desinstalação informa essa limitação. Hooks definidos fora do bloco no `crushrc` são Bash que só a execução revelaria, então não são listados. Comandos ou filtros multilinha não podem ser instalados: coloque-os num script e instale o script.

<a id="from-the-cli"></a>

## Pela CLI

```sh
lazyagents hooks add notify --event Stop --command 'notify-send "Claude terminou"' --desc "notificação na área de trabalho"
lazyagents hooks add guard --event PreToolUse --matcher Bash --command ~/bin/check-bash.sh --timeout 10
lazyagents hooks enable notify                  # todos os agentes que emitem Stop
lazyagents hooks enable guard --agent codex
lazyagents hooks list                           # instalações e contagem de hooks externos
lazyagents hooks list --json
lazyagents hooks disable guard
lazyagents hooks rm guard
lazyagents install owner/repo --hooks           # importar também hooks de plugins
```

`hooks add` avisa se o executável estiver ausente do `PATH` ou sem permissão, evitando falha silenciosa quando o evento ocorrer. Comandos e flags estão na [referência](../reference/cli.md#hooks). A importação faz parte de [`install`](../reference/cli.md#install).

<a id="files"></a>

## Arquivos

| Caminho | Conteúdo |
|---|---|
| `~/.local/share/lazyagents/hooks/<name>.json` | entradas da biblioteca, modo `0600` |
| `~/.local/share/lazyagents/hooks/<plugin>/` | scripts de um plugin importado |
| `~/.claude/settings.json` | chave `hooks` do Claude Code |
| `~/.codex/hooks.json` | hooks do Codex |
| `~/.local/share/lazyagents/backups/` | cópia antes de cada escrita; 20 mais recentes por arquivo |

<a id="safety"></a>

## Segurança

- Toda instalação, remoção e edição faz backup do arquivo do agente e grava atomicamente. Grupos não gerenciados são preservados byte a byte, inclusive campos desconhecidos. Exceção: se um grupo misturar comandos seus e da biblioteca, a desinstalação reescreve o grupo sem o comando da biblioteca.
- Escritas na TUI exigem confirmação mostrando comando e arquivo.
- Apagar um hook só remove scripts dentro da biblioteca de hooks, nunca arquivos externos.
- `doctor` informa executáveis ausentes, arquivos inválidos ou nomes inconsistentes, contagens por agente (gerenciados, externos e parciais) e avisos específicos.

<a id="limits"></a>

## Limitações

- Apenas Claude Code, Codex e Crush têm hooks; Codex emite menos eventos e Crush só `PreToolUse`. Veja a [tabela de eventos](../reference/agents.md#hook-events).
- Apenas hooks `command` são gerenciados; outros tipos permanecem intactos.
- Scripts importados podem exigir shell POSIX ou ferramentas do autor. `doctor` verifica o executável, não todas as dependências do script.
