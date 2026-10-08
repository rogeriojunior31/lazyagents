# Solução de problemas
<!-- source: e9e97beaa3e4 -->

<a id="start-with-lazyagents-doctor"></a>

## Comece com lazyagents doctor

```sh
lazyagents doctor          # relatório legível; código 1 se houver problema
lazyagents doctor --json   # mesmas informações para scripts e relatos de bugs
```

O comando lista agentes e verifica skills, links quebrados, provedores, hooks, limites e plugins. A maioria dos problemas abaixo corresponde ao relatório. Inclua-o ao abrir uma issue ([referência](reference/cli.md#doctor)).

Avisos de configuração, temas e plugins aparecem em stderr, antes da saída da CLI ou depois de sair da TUI. As mensagens entre crases abaixo são textos reais do programa, cuja interface continua em inglês; as explicações e soluções estão traduzidas.

<a id="agents"></a>

## Agentes

<a id="an-agent-shows-as-not-installed"></a>

### Agente aparece como não instalado

**Causa:** não foi encontrado binário no `PATH` nem diretório de configuração (`~/.claude`, `~/.codex`, `~/.gemini`, `~/.config/opencode`, `~/.hermes` ou `%LOCALAPPDATA%\hermes` no Windows, `~/.pi/agent`, `~/.config/crush`), nem a localização da [variável do agente](reference/agents.md#config-overrides), como `CODEX_HOME`. O ambiente usado é o da inicialização: execute no mesmo shell do agente.

**Solução:** instale a CLI ou inclua o binário no `PATH` e reabra o lazyagents. A detecção ocorre uma vez por execução; veja o [guia](guide/agents.md#how-detection-works).

<a id="the-agent-is-detected-but-has-no-version-or-enter-says-the-cli-is-not-in-path"></a>

### Agente detectado sem versão ou retomada informa que a CLI não está no PATH

**Causa:** a configuração existe, mas o binário não está no `PATH`. Skills/sessões funcionam, porém retomar exige a CLI.

**Solução:** inclua o diretório do binário no `PATH`. O detalhe de Agents mostra `config in … (binary not in PATH)`.

## Skills

<a id="installing-from-github-requires-git-in-path"></a>

### Falta Git para instalar do GitHub

Mensagem: `installing from GitHub requires git in PATH`. Instale `git`. Pasta local e ZIP funcionam sem ele.

<a id="git-clone--fails"></a>

### git clone falha

A mensagem contém o erro do próprio Git. Confira nome/URL do repositório, rede e credenciais para repositórios privados.

<a id="no-skillmd-found-in-"></a>

### Nenhum SKILL.md encontrado

Mensagem: `no SKILL.md found in …`. A origem não contém `SKILL.md` em nenhuma profundidade. Confira o repositório ou pasta: cada skill deve conter esse arquivo.

<a id="searching-requires-the-github-cli-gh-in-path-logged-in"></a>

### Falta autenticação ou CLI do GitHub para buscar

Mensagem: `searching requires the GitHub CLI (gh) in PATH, logged in`. A busca `S` usa `gh api`. Instale a [CLI](https://cli.github.com/) e execute `gh auth login`. Outros erros aparecem como `GitHub search: …`.

<a id="-is-local-unmanaged--o-to-adopt"></a>

### Skill local não gerenciada

Mensagem: `… is local (unmanaged) — o to adopt`.

**Causa:** pasta real ou link de outra ferramenta no diretório do agente, não link da biblioteca. O lazyagents não apaga conteúdo real.

**Solução:** incorpore com `o`, `A` para todas ou `lazyagents adopt <skill> --agent <id>`. A pasta vai à biblioteca e recebe um link em seu lugar. Links de terceiros não são incorporáveis: use a ferramenta original.

<a id="a-skill-with-this-name-already-exists-when-enabling"></a>

### Nome já existente ao ativar

Mensagem: `a skill with this name already exists`. O diretório do agente já contém esse nome; a mensagem informa o caminho. Remova, renomeie ou incorpore se for a mesma skill.

<a id="a-skill-shows--or-turns-on-in-agents-you-did-not-pick"></a>

### Skill mostra ◆ ou aparece em agentes não selecionados

OpenCode e Crush também leem `~/.claude/skills`, então skills ativadas no Claude Code aparecem neles com `◆`. Uma skill colocada manualmente em `~/.agents/skills`, ou por versão antiga, é visível a Codex, Gemini CLI, OpenCode, Pi e Crush. Desative nos agentes que não devem vê-la; o lazyagents move para os diretórios próprios dos demais. Veja a [referência](reference/agents.md#skills).

<a id="doctor-reports-broken-symlink"></a>

### doctor informa link quebrado

Mensagem: `broken symlink`. O link aponta para pasta inexistente, geralmente após mover/apagar a biblioteca manualmente. Remova o link ou reinstale e ative a skill. Remoções pelo lazyagents têm backup `.tar.gz`, restaurável por `b` em Skills.

<a id="migrate-library-refuses-to-run"></a>

### migrate-library recusa a execução

| Mensagem | Significado e solução |
|---|---|
| `destination already exists or is inaccessible: <path>` | skill ou link com mesmo nome já existe no destino; nada é presumido migrado. Mova/remova o conflito e tente novamente |
| `libraries cannot contain each other` / `libraries overlap after resolving symlinks` | um diretório contém o outro; escolha locais separados |

Se a migração falhar parcialmente, as cópias são desfeitas e a origem permanece.

<a id="sessions"></a>

## Sessões

### Sessões OpenCode não aparecem

Mensagem: `opencode sessions live in SQLite: install sqlite3 to list them`. As sessões ficam em `opencode.db`; instale a ferramenta `sqlite3`.

<a id="providers"></a>

## Provedores

### String ou array TOML sem fechamento

Mensagem: `TOML config has an unterminated string or array; file left untouched`. O lazyagents edita Codex linha a linha, acompanhando strings/arrays multilinha. Sem fechamento, não conhece a estrutura e não altera o arquivo. Codex também não conseguiria carregá-lo: feche a string/array e aplique novamente.

### Tabela de provedor fora do bloco gerenciado

Mensagem: `table model_providers.lazyagents already exists outside the managed block`. Há uma tabela `[model_providers.lazyagents]` manual fora do bloco. Remova ou renomeie e aplique novamente.

### Arquivo mudou durante a edição

Mensagem: `it changed while lazyagents was editing it … try again`. O agente ou outro lazyagents escreveu no mesmo arquivo (`settings.json`, hooks, `config.toml`, `models.json`/`settings.json` do Pi). O lazyagents preserva a escrita concorrente; sua mudança não é aplicada. Codex/Pi já tentam automaticamente antes do aviso. Repita o comando; se persistir, feche o agente. Veja [segurança](safety.md#principles).

### Provedor aplicado em agente não instalado

Mensagem no `doctor`: `provider applied to an agent that is not installed`. A configuração continua, mas o agente foi removido. Limpe com `x` em Providers ou `lazyagents provider clear --agent <id>`, ou reinstale o agente.

## Hooks

<a id="a-hook-installed-in-codex-never-runs"></a>

### Hook do Codex não executa

Codex exige duas aprovações que o lazyagents deliberadamente não escreve:

- `hooks are off in Codex: set hooks = true under [features] in config.toml`: acrescente em `~/.codex/config.toml`.
- `a new hook only runs after you confirm trust in Codex itself`: aprove cada novo comando no próprio Codex.

Só você pode dar esse consentimento. Veja [segurança](safety.md).

### Comando de hook ausente ou sem permissão

Mensagens: `<command>: not in PATH`, `not found`, `not executable`. Corrija caminho/permissões do script ou edite o hook em `~/.local/share/lazyagents/hooks/`.

<a id="usage"></a>

## Consumo

<a id="subscription-limits-do-not-show"></a>

### Limites de assinatura não aparecem

Agentes sem dados ficam de fora dos limites e do `doctor`, sem aviso:

- **Claude Code:** não instalado, não autenticado (execute `/login`) ou com chave de API, que não tem limite de assinatura; mostra custo em vez disso.
- **Codex:** nunca usado nesta máquina ou ainda sem limites registrados. Use uma vez e atualize: os limites vêm das sessões.

| Mensagem | Solução |
|---|---|
| `Claude Code session expired: open the CLI to renew it` | abrir Claude Code uma vez |

Cache dura 5 minutos. Use `r` em Usage ou `lazyagents usage limits --refresh`. Claude Code exige rede; Codex lê arquivos locais.

<a id="numbers-look-stale-or-wrong-after-an-update"></a>

### Números antigos ou incorretos após atualizar

O cache fica em `~/.local/share/lazyagents/transcript-index.gob`. Índices de outra versão ou danificados são reconstruídos automaticamente. Pode apagar: a próxima execução relê todas as conversas, o que pode demorar.

<a id="configuration-and-themes"></a>

## Configuração e temas

### Configuração YAML inválida

Mensagem: `parsing config: …; using defaults`. O programa usa padrões e preserva o arquivo. Corrija a sintaxe; veja as [chaves](configuration.md).

### Aviso de configuração da TUI

Mensagem: `config tui: …`. ID ou valor desconhecido em `tui:`; é só aviso, o restante da disposição se aplica.

### Tema desconhecido

Mensagem: `unknown theme: <id> in <config path>; using "sp-night"`. `theme:` não corresponde a tema incluído nem arquivo em `~/.config/lazyagents/themes/`. Confira os [IDs](reference/themes.md). Arquivo personalizado inválido informa a causa, como papel ausente ou cor inválida.

## Plugins

### Plugin ignorado

Mensagem: `plugin <file> ignored: …`.

| Motivo literal | Solução |
|---|---|
| `not executable` | executar `chmod +x` no arquivo |
| `name must match …` | renomear com minúsculas, dígitos, `-` e `_`, até 32 caracteres |
| `duplicate id` | dois arquivos com mesmo nome sem extensão; manter um |
| `id reserved by a built-in tab or command` | renomear para não colidir com `skills`, `list`, `doctor`… |

<a id="a-plugin-tab-shows-an-error"></a>

### Aba de plugin em erro

O plugin não respondeu em 3 s, escreveu fora do protocolo ou terminou. A aba mostra causa e final de stderr. Corrija e pressione `r` ou `:reload`. `lazyagents doctor` mostra a negociação inicial fora da TUI. Veja as [dicas](guide/plugins.md).

<a id="still-stuck"></a>

## Ainda precisa de ajuda?

Abra uma issue com `lazyagents doctor --json`, sistema operacional, agentes e versões. Remova dados privados como caminhos e URLs de provedores antes de publicar.
