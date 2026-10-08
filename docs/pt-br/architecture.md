# Arquitetura
<!-- source: d3da37379fe3 -->

Como o código é organizado e onde cada mudança entra. A versão curta com regras para assistentes de IA é [CLAUDE.md](../../CLAUDE.md); esta página é para pessoas.

<a id="the-big-picture"></a>

## Visão geral

O lazyagents é um único binário Go. Sem argumentos, abre uma TUI [Bubble Tea v2](https://github.com/charmbracelet/bubbletea); com subcomando, executa sem interface. Ambos usam os mesmos **módulos**: cada módulo reúne domínio, aba, comandos CLI e verificações de `doctor` em um pacote.

```text
main.go                 apenas despacho: --version, CLI ou TUI
internal/
├── app/                composição: inicialização, migrações e registro (features.go)
├── feature/            contrato entre módulo e raiz: Feature e Deps
├── core/               caminhos XDG e config.yaml; base da pilha
├── fsutil/             escritas atômicas, backups e rotação
├── agent/              adaptador por agente e interfaces opcionais de recursos
├── cli/                estrutura sem interface: despacho, ajuda, flags e doctor
├── tui/                estrutura TUI: raiz, abas, ajuda, paleta, componentes e temas
└── modules/            pacote por módulo
    ├── skills/  sessions/  providers/  hooks/  usage/  agents/
    └── plugins/        plugins externos: processo, protocolo e aba intermediária
```

Dependências apontam em uma direção:

```text
fsutil ← core ← agent ← {cli, tui/*} ← feature ← modules/* ← app ← main
```

`cli` e `tui` não conhecem módulos. O módulo os importa, nunca o contrário.

<a id="modules"></a>

## Módulos

Um módulo ocupa `internal/modules/<name>/`:

| Arquivo | Conteúdo |
|---|---|
| `service.go` e um por assunto | domínio: `Service`, tipos e entrada/saída; nunca importa `tui/` |
| `tab.go`, `view.go`, `help.go`, `msgs.go` | aba; modelo `Tab` construído por `newTab` |
| `cli.go` | `commands(svc)` e `checks(svc)`, não exportados |
| `feature.go` | `Feature() feature.Feature`, único ponto exportado |

Quando aba e domínio tratam do mesmo assunto, o arquivo da aba recebe `_ui` (`install.go`/`install_ui.go`).

A aba não realiza entrada/saída diretamente: chama o serviço em `tea.Cmd` e recebe uma mensagem. Erros viram avisos, nunca panics.

Abas se comunicam por `internal/tui/events`, com dados agregados (`map[agent]int`), nunca tipos de um módulo.

<a id="adding-a-module"></a>

### Adicionar um módulo

1. Crie `internal/modules/<name>/` com os arquivos acima.
2. Registre em uma linha de `features()` em `internal/app/features.go`. A ordem define as abas; `Last: true` coloca uma aba somente leitura no fim.
3. Leia configurações na seção própria por `d.Config.Section("<name>", &cfg)`. Não acrescente chave global.
4. Para arquivos de agentes, acrescente um recurso opcional, conforme a próxima seção.
5. Documente em `docs/guide/<name>.md` e ligue no [índice](README.md). Um teste exige o guia ([Documentação](#documentation)).

Não edite `app/app.go`, `tui/app.go` ou `cli/cli.go` para registrar um módulo.

<a id="agents"></a>

## Agentes

Só `internal/agent` conhece caminhos e formatos dos agentes. Todos implementam `Adapter` (detecção, sessões, retomada, leitura e exclusão). Recursos adicionais são interfaces opcionais verificadas por asserção de tipo:

| Interface | Usada por | Recurso |
|---|---|---|
| `ProviderHost` | provedores | aplicar/limpar perfil na configuração atual |
| `HooksHost` | hooks | ler, acrescentar e remover hooks e eventos |
| `RateLimitReader` | consumo | janelas de limite de assinatura |
| `UsageEventReader`, `UsageReader`, `AuthModeReader` | consumo e sessões | tokens por resposta/sessão e autenticação por token |
| `LiveChecker` | sessões | detectar execução atual |
| `TranscriptProber` | sessões | pré-verificação rápida de busca textual |

`Adapter` não cresce: novo recurso usa nova interface, implementada apenas onde houver suporte.

Conversas chegam como `[]agent.Entry`, com o mesmo formato por agente: papel (mensagem, resposta, raciocínio, ferramenta, evento), horário, tipo de chamada (`agent/toolkind.go`: shell, edição, leitura, agente, plano, tarefa), linhas adicionadas/removidas, falha e `Sub` para conversa de subagente ou outro ramo, lida pelo mesmo adaptador como `Session.Path`. Cada adaptador preenche o que seu formato registra e deixa o restante vazio; o leitor exibe o disponível.

<a id="adding-an-agent"></a>

### Adicionar um agente

1. Implemente `internal/agent/<agent>.go`: `Detect` (binário no `PATH` ou configuração; `--version` só aqui), sessões e conversas. Confirme formatos na documentação/código da CLI; não adivinhe.
2. Registre em `AllWithIndex` de `internal/agent/registry.go`. A ordem define colunas.
3. Implemente recursos compatíveis. Extração de tokens/prévias deve usar `lineScanner` e índice compartilhado (`internal/agent/index.go`) para ler cada arquivo uma vez.
4. Dê uma cor por `theme.AgentColor`.
5. Testes usam casas temporárias (`core.PathsIn(t.TempDir())`), nunca seu `~`. Variáveis que movem arquivos entram em `agent.ConfigOverrides` após confirmação na CLI. Pacotes que constroem adaptadores chamam `agenttest.ClearOverrides()` no `TestMain`. Registre formatos reais: acrescente um gravador em `scripts/record-fixtures.go` e execute `go test ./internal/agent -run TestRecordedFixtures -update`. Veja [Fixtures gravadas](#recorded-cli-fixtures).
6. Execute `go test ./docs -update` para atualizar a [referência](reference/agents.md). Se houver sessões, mencione no [guia](guide/sessions.md#where-sessions-come-from).

<a id="recorded-cli-fixtures"></a>

### Fixtures gravadas das CLIs

Agentes mudam formatos privados entre versões. `internal/agent/testdata/fixtures/<agent>/<version>/` guarda sessões produzidas pelas CLIs reais, por versão; `TestRecordedFixtures` compara cada adaptador com `golden.json`. A [matriz](../../internal/agent/testdata/fixtures/README.md) lista versões e origens.

- `go run scripts/record-fixtures.go` grava CLIs instaladas: cada uma usa casa temporária e fakellm, servidor local determinístico dentro do script (OpenAI Chat Completions/Responses e Anthropic Messages), sem conta nem rede. Caminhos viram `/work/proj` e `/home/user`; prompts de sistema são removidos. Dados identificando a máquina (usuário, hostname, kernel) fazem a gravação falhar.
- Nova versão ganha novo diretório; preserve os anteriores como matriz. `go test ./internal/agent -run TestRecordedFixtures -update` gera os resultados esperados; revise diferenças, pois revelam mudanças de formato.
- Limites Codex não vêm de modelo falso. `-codex-limits-from ~/.codex/sessions` copia o formato de `rate_limits` do rollout mais recente substituindo valores que descrevam a conta.
- Limites Claude Code vêm da rede. `-claude-limits` faz a mesma consulta da aba Usage, pela autenticação do Claude Code e `Claude.RateLimits`; preserva só campos lidos (`limits`, `five_hour`, `seven_day`), substitui valores e nunca grava a autenticação. O teste serve `limits.json` com credencial fictícia.

<a id="rules-that-protect-user-data"></a>

## Regras de proteção dos dados

São válidas em todo o código e verificadas na revisão:

- **Configuração usa `fsutil.WriteAtomic`.** Arquivo do agente recebe `fsutil.Backup` antes; chaves desconhecidas são preservadas.
- **JSON dos agentes** usa a primitiva `settings` (`internal/agent/settings.go`), alterando apenas a chave alvo e preservando ordem.
- **TOML do Codex nunca é reserializado.** Só blocos entre comentários `# lazyagents — …` são editados; o resto é copiado literalmente.
- **Ativação de skills usa links.** Desativar remove o link, nunca pasta real.
- **Segredos** ficam em arquivos 0600, mascarados na TUI/JSON, fora de logs, erros e estruturas exportadas.
- **Sem rede na inicialização.** Consultas apenas sob demanda.
- **Consentimento nunca é forjado.** Aprovações do agente, como `trusted_hash` do Codex, ficam com o usuário.
- **Plugins não são confiáveis.** Mensagens são sanitizadas; falha deixa apenas a aba em erro.

A versão para usuários está em [Segurança e dados](safety.md).

<a id="themes"></a>

## Temas

Cores existem só em `internal/tui/theme`. A UI usa tokens (`theme.Primary`, `theme.AgentColor(id)`) resolvidos durante renderização; não renderize estilos na inicialização do pacote. Veja [internal/tui/theme/README.md](../../internal/tui/theme/README.md) e [Contribuir um tema](themes.md#contributing-a-theme).

<a id="documentation"></a>

## Documentação

Listas existentes no código são geradas dele; o restante é escrito e verificado por testes.

| Tipo | Local | Fonte | Como manter |
|---|---|---|---|
| Referência | `docs/reference/` | `cli.Command` Usage/Summary/Help, `Help()` e `Commands()` das abas, adaptadores e temas | `go test ./docs` detecta diferenças; `go test ./docs -update` regenera |
| Guias | `docs/guide/<module>.md` | escrita manual, por módulo | teste exige guia e link no índice |
| Tópicos | `docs/*.md` | escrita manual | teste de links |
| Protocolo | `docs/plugins.md` | escrita manual | muda com versão do protocolo |
| Traduções | `docs/pt-br/<mesmo caminho>` | tradução da página inglesa | workflow compara marca do original |

`go test ./docs` verifica links relativos e âncoras de todos os Markdown do repositório. Renomear título/arquivo pode quebrar o teste antes de quebrar os docs.

O site [rogeriojunior31.github.io/docs/lazyagents](https://rogeriojunior31.github.io/docs/lazyagents/) publica os docs. `.github/workflows/docs.yml` chama o workflow compartilhado: em PR, valida título `# H1` e links; em tag `v*`, avisa o site para publicar a release. Use Markdown comum de GitHub: primeiro H1 vira título, `docs/README.md` é o índice e a ordem dos links define o menu; links relativos são convertidos para páginas do site. `docs/dev/` não é público.

Traduções ficam em `docs/pt-br/`, com o mesmo caminho relativo do original. Cada uma registra a marca da revisão traduzida abaixo do título:

```markdown
# Skills
<!-- source: 1a2b3c4d5e6f -->
```

Obtenha a marca com `sha256sum docs/guide/skills.md | cut -c1-12`. Se o original mudar, o workflow detecta e o site indica desatualização. Só atualize a marca após revisar a tradução. O verificador de inglês ignora `docs/pt-br/`, o README em português (`README.pt-br.md`) e os metadados localizados de `docs/site.json`.

<a id="website-integration"></a>

### Integração com o site

`docs/site.json` define nome e descrições por idioma, com `schema: 1`, `name` e `summary` contendo `en` e `pt-br`. O site lê a mesma revisão dos docs e metadados. Novos repositórios ainda precisam de cadastro inicial e montagem de módulo no site; não são descobertos arbitrariamente.

`requiredTranslations: ["pt-br"]` torna obrigatória a documentação completa e atualizada em português. Cada Markdown público em inglês deve ter tradução correspondente com a marca atual do original. Traduções ausentes ou antigas fazem os testes, workflow compartilhado e build do site falharem. Revise e traduza mudanças antes de atualizar a marca. Comandos, identificadores e mensagens literais permanecem iguais. Âncoras HTML `id` preservam os links de seções no GitHub e no site.

Originais ficam em `docs/`; traduções em `docs/pt-br/<mesmo caminho>`. Use links relativos para as traduções correspondentes. Metadados descrevem o projeto, não produzem traduções. O [estado das traduções](https://rogeriojunior31.github.io/docs/traducoes/) lista cobertura e marcas atuais.

Para repetir em outro projeto, copie a estrutura de `docs/site.json`, escreva `docs/README.md`, crie as traduções correspondentes com marcas revisadas e reutilize `.github/workflows/docs.yml`. `go test ./docs` valida referências, links e metadados; o workflow compartilhado valida metadados, títulos, links e atualização das traduções. Sem `SITE_DISPATCH_TOKEN`, o site procura docs publicados no build diário.

O que atualizar por mudança:

| Mudança | Atualização |
|---|---|
| comando ou flag | `Help` em `cli.go`, depois `go test ./docs -update` |
| tecla ou paleta | `Help()`/`Commands()` da aba e regeneração |
| agente, recurso ou evento de hook | adaptador e regeneração |
| tema | YAML e regeneração |
| módulo | `docs/guide/<name>.md` e link no índice |
| comportamento visível | guia do módulo e `CHANGELOG.md` |
| arquivo em disco, consulta de rede ou escrita em agente | [Configuração](configuration.md#where-files-live) e [Segurança](safety.md) |

Guias explicam tarefas/conceitos e citam teclas/flags necessárias, ligando à referência completa sem copiar tabelas que ficariam desatualizadas.

<a id="checks"></a>

## Verificações

```sh
test -z "$(gofmt -l .)" && go vet ./... && go test -race ./... && go build ./...
scripts/check-english.sh
```

A CI executa os mesmos passos; `go test ./...` inclui os testes dos docs.
