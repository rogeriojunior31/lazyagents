# Configuração
<!-- source: 28830a2b0cda -->

Tudo é opcional: o lazyagents funciona sem configuração. Para personalizar, crie `config.yaml` no diretório de configuração.

<a id="where-files-live"></a>

## Onde ficam os arquivos

| Diretório | Linux | macOS | Windows |
|---|---|---|---|
| Configuração | `$XDG_CONFIG_HOME/lazyagents`, padrão `~/.config/lazyagents` | `~/Library/Application Support/lazyagents` | `%AppData%\lazyagents` |
| Dados | `$XDG_DATA_HOME/lazyagents`, padrão `~/.local/share/lazyagents` | igual ao Linux | igual ao Linux |

O restante da documentação usa os caminhos do Linux.

| Caminho | Conteúdo |
|---|---|
| `~/.config/lazyagents/config.yaml` | esta configuração |
| `~/.config/lazyagents/providers.json` | perfis de provedores, modo 0600, podem conter tokens |
| `~/.config/lazyagents/plugins/` | plugins externos, um executável por plugin ([guia](guide/plugins.md)) |
| `~/.config/lazyagents/themes/` | temas personalizados `<id>.yaml` ([temas](themes.md)) |
| `~/.local/share/lazyagents/skills/` | biblioteca de skills, movível por `libraryDir` |
| `~/.local/share/lazyagents/profiles.json` | perfis de ativação de skills |
| `~/.local/share/lazyagents/hooks/` | biblioteca de hooks, JSON por hook e scripts importados |
| `~/.local/share/lazyagents/session-aliases.json` | aliases de sessões |
| `~/.local/share/lazyagents/usage-cache.json` | cache de limites |
| `~/.local/share/lazyagents/claude-provider-state.json` | chaves `env` escritas no Claude Code; só impressões digitais, nunca os valores |
| `~/.local/share/lazyagents/pi-provider-state.json` | provedor/modelo padrão anterior do Pi, restaurado na limpeza; sem segredo |
| `~/.local/share/lazyagents/transcript-index.gob` | índice descartável; apagar só faz a próxima carga reler tudo |
| `~/.local/share/lazyagents/backups/` | backups de skills, sessões e configurações dos agentes |
| `~/.local/share/lazyagents/backups/.config-backup-meta/` | origem/checksum de configurações, um JSON 0600 por backup novo ([recuperação](guide/recovery.md)) |
| `~/.local/share/lazyagents/exports/` | conversas exportadas |

As escritas fora desses diretórios estão em [Segurança e dados](safety.md#what-lazyagents-writes).

Verificações de endpoint/catálogo usam os campos de perfil existentes e não criam arquivos nem cache persistente. A consulta usa primeiro o token salvo/digitado, depois a variável indicada em `envKey` no ambiente do processo lazyagents; nunca usa login nativo. Requisições têm limites fixos e só ocorrem por ação explícita, conforme [verificações de endpoint](guide/providers.md#endpoint-checks-and-model-discovery) e [Segurança](safety.md#network).

## config.yaml

```yaml
theme: sp-night-garoa           # tema incluído ou personalizado
libraryDir: ~/.agents/skills     # localização da biblioteca

tui:
  splash: true                   # false pula a tela inicial
  splashSeconds: 2               # duração; 0 pula, máximo 10
  startTab: sessions             # aba inicial
  tabs: [sessions, skills, usage]    # ordem; não listadas vêm depois
  hidden: [hooks, providers]         # ocultas da barra e da paleta

skills:
  installTargets: [codex]         # últimos destinos explícitos na TUI; [] = só biblioteca
  sources:                       # opcionais, mantidas pelo usuário; sem varredura automática
    - name: ECC
      url: https://github.com/affaan-m/everything-claude-code
      # ref: main                # branch/tag opcional; vazio acompanha padrão remoto
      # subdir: skills           # escopo opcional relativo ao repositório

usage:
  period: 30d                    # today | 7d | 30d | 90d | all
  view: projects                 # daily | agents | projects | models

hello:                           # seção de plugin, identificada por seu id
  greeting: hi
```

O lazyagents reescreve o arquivo ao alterar configurações que gerencia: `migrate-library` define `libraryDir`, uma instalação de skills confirmada na TUI lembra `skills.installTargets`, e o gerenciamento de origens altera `skills.sources`. Comentários, ordem das chaves e seções desconhecidas são preservados.

<a id="top-level-keys"></a>

### Chaves globais

| Chave | Padrão | Efeito |
|---|---|---|
| `theme` | `sp-night` | tema da TUI: [ID incluído](reference/themes.md) ou arquivo em `themes/`; lido na inicialização. ID desconhecido usa `sp-night` com aviso. IDs anteriores à v0.3 (`noite`, `garoa`, `jaragua`) ainda funcionam |
| `libraryDir` | `~/.local/share/lazyagents/skills` | biblioteca. Prefira `lazyagents migrate-library <dir>`, que move skills e recria links ([guia](guide/skills.md)) |

Apenas essas duas chaves são globais. As outras são seções de módulos/plugins, nomeadas pelo ID.

<a id="tui-tab-layout"></a>

### tui: disposição das abas

| Chave | Efeito |
|---|---|
| `splash` | `false` pula a tela inicial |
| `splashSeconds` | duração da tela, até 10 segundos; `0` pula |
| `startTab` | aba inicial |
| `tabs` | ordem das abas; omitidas vêm depois na ordem padrão |
| `hidden` | remove abas da barra e da paleta |

Os IDs são os da paleta: `skills`, `sessions`, `providers`, `hooks`, `recovery`, `usage`, `agents` e os plugins. Abas internas ocultas continuam carregando, pois Usage e Agents dependem dos demais módulos. Plugins ocultos não iniciam, mas seu comando de CLI continua disponível.

### usage

Filtros iniciais: `period` e `view`. Veja o [guia de consumo](guide/usage.md).

<a id="skills"></a>

### skills

`installTargets` é a lista de IDs de agentes pré-selecionados na próxima instalação de skills pela TUI. Ausente ou `[]` significa somente biblioteca; nunca significa todos os agentes. Apenas IDs salvos instalados e compatíveis com skills aparecem selecionados; IDs indisponíveis geram avisos. A confirmação final mostra os destinos efetivos. Avisos de destinos aparecem no seletor da origem durante a instalação, em vez da inicialização. Somente biblioteca ainda permite visibilidade aos agentes que leem a biblioteca diretamente. Importações só de hooks, instalações canceladas, CLI e ações em lote não alteram essa preferência. A gravação lê a configuração atual, preserva outras chaves/comentários e informa erros sem desfazer arquivos instalados. Veja o [fluxo de skills](guide/skills.md#install-from-github-a-folder-or-a-zip).

`sources` é uma lista de cadastros de repositório: `name` único, `url` Git, branch/tag `ref` opcional e `subdir` opcional com barras normais. O padrão é lista vazia. `R` em Skills abre a lista sem buscar; a varredura explícita clona apenas o cadastro escolhido. Última tentativa/erros ficam só na sessão e nunca são gravados na configuração. Mudanças de URL/ref afetam apenas novas varreduras; cada skill instalada guarda sua própria proveniência. Veja [origens salvas](guide/skills.md#saved-source-repositories). Remover `sources` ou uma entrada nunca apaga skills instaladas. Outras chaves da seção, inclusive `installTargets`, sobrevivem às alterações de origens. Uma lista malformada ou um cadastro alterado desde a revisão é preservado e informado, em vez de sobrescrito.

Revisões de atualização mantêm clones temporários até aplicar/fechar, sem adicionar chaves de configuração. Progresso, cancelamento e revisão existem só na sessão; o encerramento normal limpa o staging. Varreduras de origem e verificação/aplicação de atualizações na CLI limpam tudo antes de retornar, sem persistir revisões ou estado de varredura; preferências de origem/destinos permanecem iguais. Salvar/remover perfis usa o `profiles.json` existente. Veja a [revisão de atualizações](guide/skills.md#update-from-source).

<a id="plugin-sections"></a>

### Seções de plugins

Ao iniciar, o plugin recebe a seção inteira com seu ID. O conteúdo é definido por ele.

<a id="invalid-values-never-block-startup"></a>

## Valores inválidos não impedem a inicialização

- Um `config.yaml` ilegível é ignorado: o programa usa padrões e emite aviso.
- IDs desconhecidos de aba, tema ou filtro recebem aviso e usam o padrão.
- Avisos aparecem na saída de erro ao sair da TUI. Configuração, temas e plugins também avisam antes da saída de comandos. Avisos de `tui:` só aparecem na TUI, pois a CLI não tem abas.

<a id="upgrading-from-configjson"></a>

## Migrar de config.json

Versões de desenvolvimento anteriores à v0.1.0 usavam `config.json`. Na primeira execução, ele é migrado para `config.yaml` e preservado como `config.json.migrated`.
