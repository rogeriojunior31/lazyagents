# Plano — lazyagents 100% em inglês

> Documento de trabalho em PT-BR (o último). Ao final da Fase 6 ele é apagado; o registro
> fica no BACKLOG (M15, já em inglês) e no histórico do git.

## Objetivo

Inglês passa a ser o idioma **padrão e único** do projeto: TUI, CLI, mensagens de erro,
comentários, testes, docs, README, BACKLOG, CLAUDE.md, CI e **commits**. Tradução para
PT-BR vem depois, como *feature* (catálogo de mensagens), não como idioma do código.

## Retrato de hoje (25/09/2026)

| Onde | Tamanho |
|---|---|
| `.go` não-teste com PT | 130 arquivos |
| `_test.go` com PT | 61 arquivos (~460 linhas com strings acentuadas, muitas são asserções de texto da UI) |
| Linhas de comentário em PT | ~1.700 |
| Linhas com string literal em PT (não-teste) | ~390 — skills 233, hooks 133, sessions 116, agent 116, providers 78, usage 77, components 35, fsutil 34, plugins 33, resto < 20 |
| Docs | README (261), BACKLOG (242), CLAUDE.md (94), `docs/plugins.md`, `docs/review.md`, `docs/tui-design-review.md`, `internal/tui/theme/README.md`, `LICENSES-themes.md` |
| Outros | `.github/workflows/*.yml` (nomes de step e comentários), `scripts/*`, `demo.tape`, `examples/plugins/hello`, 14 temas YAML (`description`, cabeçalho) |
| Commits | 170, todos em PT — **não reescrever histórico** |

Já em inglês (não mexer): identificadores Go (quase todos), nomes de comando/flag da CLI,
tags `json`/`yaml`, ids de abas (`skills`, `sessions`…), ids de janela de rate limit.

## Decisões (fixar antes de começar)

1. **Não reescrever o histórico do git.** Só commits novos em inglês.
2. **Sem i18n agora.** Nada de biblioteca nem catálogo nesta migração (YAGNI). A única
   exigência para facilitar a tradução futura: frases inteiras em uma string
   (`fmt.Sprintf("%d skills enabled in %s", n, a)`), nunca frase montada por pedaços.
3. **Nomes próprios ficam:** temas SP Night (`noite`, `garoa`, `jaragua`) e as chaves da
   paleta (`sodio`, `laje`, `vao`…) são identidade do design system, não texto de UI.
   `label`/`description` dos temas são traduzidos (ver Fase 4).
4. **Estilo do inglês:** frases curtas, *sentence case* em títulos e ajuda
   (`Install skill`, não `Install Skill`), sem ponto final em toast/rodapé, voz ativa.
   Erros minúsculos e sem ponto (convenção Go): `fmt.Errorf("reading %s: %w", p, err)`.
5. **Commits:** Conventional Commits em inglês, imperativo:
   `feat(skills): install from zip`, `fix(theme): badge text uses on_accent`.

## Glossário (usar sempre o mesmo termo)

| PT | EN |
|---|---|
| sessão / sessões | session / sessions |
| uso | usage |
| provedor / perfil | provider / profile |
| agente | agent |
| biblioteca (de skills) | library |
| ativar / desativar | enable / disable |
| adotar / adoção | adopt / adoption |
| apelido | alias |
| aba | tab |
| paleta (de comandos) | command palette |
| janela (de limite) | window — `session`, `weekly`, `weekly · <model>` |
| bloco gerenciado | managed block |
| aviso / erro / dica | warning / error / hint |
| retomar (sessão) | resume |
| diagnóstico | diagnostics (`doctor`) |
| origem (de instalação) | source |
| atualizar da origem | update from source |
| confirmar / cancelar | confirm / cancel |
| hoje / 7 dias / tudo | today / 7d / all |

## ⚠️ Pontos de compatibilidade (tratar ANTES de traduzir)

Strings que já estão **gravadas no disco do usuário** ou são **contrato** — traduzir cego quebra.

1. **Marcadores do bloco gerenciado no `config.toml` do Codex** (`internal/agent/codex_provider.go:20-29`):
   `# lazyagents — início do bloco gerenciado (não editar à mão)`, `… fim do bloco gerenciado`,
   `# lazyagents: model anterior = `, `# lazyagents: provider anterior = `.
   O parser tem que **reconhecer PT e EN** na leitura e **escrever só EN**; na próxima
   escrita o bloco migra sozinho. Teste: config.toml com marcadores PT → aplicar/limpar
   perfil funciona e sai com marcadores EN, resto do arquivo byte a byte igual.
2. **Qualquer outro marcador/comentário que o lazyagents escreve em arquivo alheio** —
   levantar com `grep -rn '"# lazyagents\|lazyagents:' internal/agent` e aplicar a mesma regra.
3. **`RateWindow.Label`** sai no `usage --json` (`"semana"`, `"janela"`, `"semana (7d)"`).
   Mudar é *breaking* na saída JSON: registrar no changelog/release notes. Scripts devem
   usar `kind`, não `label` — documentar isso no README.
4. **Protocolo de plugins** (`docs/plugins.md`, `plugins.Protocol`): conferir se algum
   valor de campo é PT. Se só a prosa for PT, traduzir sem bump; se algum valor mudar,
   aceitar o antigo ou bumpar a versão (regra 10 do CLAUDE.md).
5. **`config.yaml`**: chaves e valores já são EN (`period`, `startTab`…). Conferir se
   algum *valor* aceito é PT (ex.: período `hoje`); se for, aceitar os dois na leitura.
6. **Templates gravados em disco** (scaffold do `SKILL.md` em `n`, export de sessão para
   Markdown em `x`, notas de backup): arquivos novos saem em EN; os antigos continuam
   legíveis (nada deve depender do texto deles).

## Fases

Cada fase é uma task do BACKLOG, um PR/commit próprio, e fecha com o check completo:
`test -z "$(gofmt -l .)" && go vet ./... && go test -race ./... && go build ./...`
mais tmux manual nas fases que tocam UI.

### Fase 0 — Regras e trilho (1 commit) ✅

- **CLAUDE.md** em inglês, com a seção *Language* (inglês em tudo, frase inteira numa
  string só) e commits em inglês no Workflow.
- `scripts/check-english.sh`: falha se houver `[áàâãéêíóôõúçÁÉÍÓÚÇ]` em arquivo
  rastreado. Exceções: arquivos em `ALLOW_FILES` (só este plano),
  nomes próprios (`Rosé`, `Jaraguá`) e linhas com o marcador `check-english:allow`
  (para as strings legadas da Fase 1). Roda no CI com `continue-on-error` até a Fase 5.
  Acento pega a maior parte; o que sobra (`para`, `com`… sem acento) cai na revisão.
- M15 no BACKLOG (em inglês); "commitar em PT-BR" e "i18n fora de escopo" saíram do BACKLOG.

### Fase 1 — Compatibilidade (1 commit) ✅

- **Codex:** marcadores novos em EN; os antigos em PT ficam como `codexLegacy*` e são
  reconhecidos na leitura (início, fim, `model`/`provider` anteriores). A próxima escrita
  troca tudo por EN. `TestCodexMigratesLegacyMarkers` cobre ler, aplicar e limpar a
  partir de um arquivo antigo (e falha se o reconhecimento for desligado).
- **Cache de limites** (achado nesta fase): o `usage-cache.json` guardava o `label` já
  em PT e o usa como fallback "stale" quando a rede falha, então traduzir sem mais nada
  deixaria "semana" na tela indefinidamente. O arquivo ganhou `{"version": 2, "agents": …}`;
  o formato antigo é descartado, inclusive como fallback (`TestStatusDropsOutdatedCache`).
  O `scripts/demo-home.sh` gravava o cache no formato antigo e passou a gravar o v2.
  Os rótulos de janela já foram traduzidos: `session 5h`, `week`, `week · <modelo>`,
  `window`, `session Nh`, `week (Nd)`.
- **`label` do `--json`:** documentado no código como texto de exibição (usar `kind`) e
  registrado no `CHANGELOG.md` novo, seção *Unreleased*. O release usa
  `--generate-notes`, que só lista PRs; o CHANGELOG é o registro para commits diretos.
- **Sem mudança necessária:** nenhum outro marcador em arquivo alheio; protocolo de
  plugins e valores do `config.yaml`/paleta já são EN (sem bump); o índice de transcripts
  guarda só dados crus (os textos "(sem prompt)" entram na leitura); o template do
  `SKILL.md` e o export de sessão não são relidos.
- **Fica para as Fases 2/3** (texto em `--json`, já avisado no CHANGELOG): `AuthMode`
  (`assinatura`/`desconhecido`), títulos provisórios de sessão, `Detail`/`SharedNote` dos
  agentes e os rótulos de dia (`ter 23/09`) da agregação de uso.

### Fase 2 — Superfície do usuário: TUI e CLI (1 commit por módulo)

Ordem pelo que o usuário mais vê; cada commit traduz **strings + os testes que as
afirmam** juntos (senão a suíte fica vermelha no meio).

1. `tui/` framework: `app.go` (header, splash, ajuda global, paleta), `components/`
   (Confirm, Toast, Palette), `kit/`. Títulos de aba: `Sessions`, `Usage`, `Providers`,
   `Agents` (`internal/tui/app_test.go:132` muda junto).
2. `modules/skills` (233 linhas — maior; pode partir em tab/help e service/cli)
3. `modules/sessions`
4. `modules/hooks`
5. `modules/providers`
6. `modules/usage`
7. `modules/agents`, `modules/plugins`
8. `cli/` (Usage/Summary/Help de todos os comandos, `doctor`), `app/`, `main.go`

Por módulo, a lista de arquivos: `help.go`, `view.go`, `tab.go`, `*_ui.go`, `msgs.go`
(toasts), `cli.go` (`Summary`, mensagens), `feature.go` (títulos de check).
Verificação manual: abrir cada aba no tmux, `?` (ajuda), confirm, toast de erro, e
`lazyagents help`, `lazyagents <cmd> --help`, `doctor`. Conferir **largura**: inglês
costuma ser mais curto, mas rótulos de coluna e rodapés têm tamanho fixo — ver truncamento.

### Fase 3 — Mensagens de erro e domínio (1 commit por pacote)

`agent/` (116), `fsutil/` (34), `core/`, services dos módulos. Só texto de
`fmt.Errorf`/`errors.New` e strings exibidas. Testes que comparam mensagem de erro mudam
junto (preferir `errors.Is`/`strings.Contains` de um trecho estável quando for tocar).

### Fase 4 — Temas

- `internal/tui/theme/README.md` e `LICENSES-themes.md` em EN.
- Temas da comunidade (dracula, nord…): `description` em EN direto no YAML.
- SP Night (`noite`, `garoa`, `jaragua`): os YAML são **gerados** — traduzir no upstream
  (`~/Projects/SP-Night`, `palette/roles.json`/metadata) e regenerar, ou fazer o gerador
  aceitar `label`/`description` em EN. Cabeçalho gerado (`# Gerado a partir…`) muda no
  `generate/main.go`. `-check` precisa continuar verde.

### Fase 5 — Comentários e testes (em lote, por pacote)

~1.700 linhas de comentário + nomes de subteste (`t.Run`) + mensagens de `t.Errorf`.
Não muda comportamento, então pode ir em commits grandes por diretório. Manter o
**conteúdo** do comentário (o *porquê*), não resumir. Nomes de teste já em inglês ficam;
os raros em PT (`TestWriteAtomicErro`) viram EN.
Ao final: `check-english.sh` passa a **quebrar** o CI.

### Fase 6 — Docs, README e repositório

- **README.md** reescrito em EN (não traduzido linha a linha: ler como quem chega do
  GitHub). Alt do logo, seções, exemplos de CLI. `demo.gif` regravado (`demo.tape` e
  `scripts/demo-home.sh` com dados/rótulos em EN; `scripts/record-demo.sh`).
- `docs/plugins.md` em EN; `examples/plugins/hello` com saídas em EN.
- **BACKLOG.md** em EN (histórico M0–M14 inclusive — é o que o contribuidor lê).
- `docs/review.md` e `docs/tui-design-review.md`: são registros de revisão já
  concluídos → traduzir **ou** apagar (o git guarda). Recomendo apagar se nada neles
  estiver pendente.
- `.github/workflows/*.yml`: nomes de step e comentários.
- Opcional e barato para open source: `CONTRIBUTING.md` curto (check local, estilo de
  commit, "English only") e template de PR.
- Apagar este arquivo.

## Depois: tradução PT-BR (fora deste plano)

Esboço, só para não fechar portas agora:
- Catálogo por chave = string em inglês (estilo gettext): `i18n.T("Install skill")`,
  com `pt-BR.yaml` embutido; string ausente cai no inglês.
- Idioma por `language:` no `config.yaml` (seção global nova — exige decisão, pois hoje
  só `theme` e `libraryDir` são globais) com fallback para `LANG`/`LC_ALL`.
- CLI `--json` **nunca** traduz (é contrato); só texto humano.
- Por isso a regra da Decisão 2: frase inteira numa string só.

## Checklist de pronto

- [ ] `scripts/check-english.sh` obrigatório no CI e verde
- [ ] Config do Codex com marcadores PT antigos é lida e migrada (teste)
- [ ] Todas as abas, ajuda, paleta, confirms e toasts em EN (tmux)
- [ ] `lazyagents help`, `--help` de cada comando e `doctor` em EN
- [ ] README, docs, BACKLOG, CLAUDE.md, temas e CI em EN; `demo.gif` novo
- [ ] Release notes citam o `label` do `usage --json`
- [ ] Commits desta migração em inglês
