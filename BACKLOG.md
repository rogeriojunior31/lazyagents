# BACKLOG — lazyskills

Planejamento completo, em ordem de execução. **Uma task por vez**: não iniciar a próxima com a atual falhando.

## Como trabalhar (toda task)

1. Implementar seguindo as regras do `CLAUDE.md` (adapters isolados, escrita via `fsutil`, I/O só em `tea.Cmd`, testes com `t.TempDir()`).
2. Verificar: `gofmt -l . && go vet ./... && go test ./... && go build ./...` tudo verde.
3. Teste manual na TUI via tmux quando a task tocar UI (capturar a tela e conferir).
4. Marcar o checkbox aqui e commitar: `feat(escopo): descrição` em PT-BR.

Legenda: **toca** = arquivos/pacotes previstos · **aceite** = critérios verificáveis.

---

## M0 — Fundação ✅ (concluído)

- [x] Esqueleto Go + Bubble Tea v2 + fsutil (escrita atômica, backups)
- [x] `internal/agent`: adapters claude-code, codex, gemini-cli, opencode, claude-desktop, hermes-agent (detecção, skills dirs, sessões, resume)
- [x] `internal/skill`: biblioteca `~/.lazyskills/skills`, scan multi-dir honesto, enable/disable por symlink, adopt, remove com backup, install de pasta/zip/GitHub com descoberta recursiva e dedupe
- [x] `internal/session`: listagem unificada + resume
- [x] TUI: abas Skills (matriz + leitor de SKILL.md), Sessões (detalhe + resume via ExecProcess), Agentes (cards); mouse (wheel, clique, clique duplo); filtro corrigido; paste; títulos renomeados do Claude (`ai-title`)

---

## M1 — Sessões e edição no dia a dia ✅ (concluído)

### M1.1 — Ver transcript da sessão na TUI ✅
- [x] Ler a conversa (user/assistant) de uma sessão sem sair da TUI, antes de decidir retomar.
- **Toca:** `internal/agent/transcript.go` (novo: `Entry{Role, Text, Time}` + parser por agente), `agent.Adapter` (método `Transcript(s Session) ([]Entry, error)`), `views/sessions.go` (modo doc com viewport, igual ao leitor de SKILL.md).
- **Detalhes:** claude e gemini leem o `.jsonl` (parser resiliente: linha inválida vira warning, nunca crasha; buffer 4 MB). codex lê o rollout. opencode consulta `message.data` via `sqlite3 -json`. Renderizar `▶ você` / `◀ agente` com o markdown leve existente.
- **Teclas:** `v` abre o transcript · `esc` volta · scroll teclado + roda do mouse · clique fecha.
- **Aceite:** transcript da sessão atual do claude abre e rola no tmux; sessão gemini abre; linha JSON corrompida não derruba (teste unitário); `Height/Spacing` do clique continuam certos.

### M1.2 — Filtro melhor nas Sessões ✅
- [x] Fuzzy casa demais ("gemini" retorna 28 itens porque o cwd entra no filtro).
- **Toca:** `views/sessions.go`.
- **Detalhes:** `FilterValue = tag do agente + título` (sem cwd). Tecla `f` cicla `todas → claude → codex → gemini → opencode` (refaz `SetItems` com o subconjunto; contador no toast/status).
- **Aceite:** filtrar "gemini" retorna só sessões gemini; `f` cicla e o status mostra o agente ativo; `esc`/ciclo completo volta a "todas". Teste manual tmux.

### M1.3 — Editar skill com `e` ✅
- [x] Abrir o `SKILL.md` da skill selecionada no `$EDITOR`.
- **Toca:** `views/skills.go`.
- **Detalhes:** mesmo mecanismo do resume (`tea.ExecProcess` suspende a TUI). Editor: `$EDITOR`, fallback `vi`. Ao voltar: rescan + toast. Funciona também no modo leitura (`e` dentro do doc).
- **Aceite:** `e` abre o editor no arquivo certo, salvar+sair volta pra TUI com a descrição atualizada na lista.

### M1.4 — Criar skill nova com `n` ✅
- [x] Criar uma skill do zero pela TUI.
- **Toca:** `internal/skill/ops.go` (`Create(name) (path, error)`: valida nome kebab-case, recusa existente, grava template com frontmatter via `fsutil.WriteAtomic`), `views/skills.go` (input de nome reusando o textinput do install).
- **Detalhes:** template mínimo: frontmatter `name`/`description` + seção de instruções. Depois de criar, abre o `$EDITOR` (reusa M1.3) e rescan ao voltar.
- **Aceite:** `n` + nome cria `~/.lazyskills/skills/<nome>/SKILL.md` válido, abre editor, aparece na lista ao voltar; nome inválido/duplicado vira toast de erro; testes unitários do `Create`.

---

## M2 — Ciclo de vida das skills

### M2.1 — Registrar origem da instalação ✅
- [x] Saber de onde cada skill da biblioteca veio, para poder atualizar.
- **Toca:** `internal/skill/install.go` (gravar `.origin.json` — `{type: git|zip|dir, url|path, installedAt}` — dentro da pasta da skill na biblioteca, via `fsutil.WriteAtomic`), `internal/skill/skill.go` (`Skill.Origin` no scan), `views/skills.go` (mostrar origem no card de detalhes).
- **Detalhes:** `.origin.json` começa com `.` → invisível para os agentes; descoberta ignora dirs ocultos, então não vira "skill". Adopt registra `{type: dir, path: origem}`.
- **Aceite:** instalar do GitHub grava a URL; scan expõe; card mostra "origem github.com/x/y"; round-trip testado.

### M2.2 — Atualizar skill do GitHub com `u` ✅
- [x] Re-instalar a versão mais nova de uma skill que veio de repositório.
- **Toca:** `internal/skill/ops.go` (`Update(sk)`: clone raso da origem, redescobre a skill pelo nome, backup `.tar.gz` da atual, substitui conteúdo preservando `.origin.json`), `views/skills.go` (tecla `u` + confirm).
- **Detalhes:** symlinks nos agentes não mudam (apontam pra pasta, que é substituída no lugar). Skill sem origem git → toast explicando.
- **Aceite:** update substitui o conteúdo, gera backup, mantém ativações; sem origem → erro amigável; teste com repo fixture local (`git init` em TempDir).

### M2.3 — CLI headless ✅
- [x] Usar tudo sem TUI, scriptável (inclusive por agentes de código).
- **Toca:** `main.go` (dispatch de subcomandos, stdlib `flag`; sem lib de CLI), talvez `internal/cli/` se `main.go` passar de ~200 linhas.
- **Comandos:** `lazyskills list [--json]` · `enable <skill> [--agent id|--all]` · `disable <skill> [--agent id|--all]` · `install <origem>` (não interativo: instala tudo que descobrir) · `remove <skill>` · `adopt <skill> --agent id` · `sessions [--json]` · `doctor` (agentes detectados + problemas: symlink quebrado, SKILL.md inválido).
- **Aceite:** cada comando com saída limpa e exit code correto; `--json` estável para script; `lazyskills` sem argumentos continua abrindo a TUI; testes dos comandos via services (não via exec).

---

## M3 — Perfis de skills

### M3.1 — Service de perfis ✅
- [x] Conjuntos nomeados de skills aplicáveis de uma vez ("trabalho", "pessoal").
- **Toca:** `internal/skill/profiles.go` (novo: `~/.lazyskills/profiles.json` — `{nome: [skills]}` — via `fsutil.WriteAtomic`; `SaveProfile(nome, skills)`, `ApplyProfile(nome, agents)`).
- **Detalhes:** semântica de aplicar = **garantir as listadas ativas em todos os agentes** e desativar as *gerenciadas* que ficaram de fora (skills locais nunca são tocadas — mesma regra do DisableAll). Perfil que referencia skill inexistente → erro listando as faltantes.
- **Aceite:** salvar/aplicar com testes table-driven; aplicar é idempotente; campos desconhecidos no JSON sobrevivem ao round-trip.

### M3.2 — Perfis na TUI ✅
- [x] Tecla `p` na aba Skills: lista perfis → `enter` aplica (com confirm mostrando o diff: o que ativa/desativa) · `s` salva o estado atual como perfil novo.
- **Toca:** `views/skills.go` (novo modo, reusa o padrão do picker).
- **Aceite:** ciclo completo no tmux: salvar perfil, bagunçar ativações, aplicar, matriz volta ao estado do perfil.

### M3.3 — Tela inicial (splash/welcome) ✅
- [x] Exibir uma tela de boas-vindas ao abrir o lazyskills antes de entrar na TUI principal.
- **Toca:** `internal/tui/views/splash.go` (novo: modelo Bubble Tea standalone com logo ASCII, versão, dica de teclas), `internal/tui/app.go` (estado inicial `stateSplash` → transita para `stateMain` ao pressionar qualquer tecla ou após timeout configurável).
- **Detalhes:** logo em ASCII art com o nome "lazyskills"; linha de versão (`ldflags`); lista das teclas principais (`?` ajuda, `tab` muda aba, `q` sai); timeout de 2s ou qualquer tecla avança. Respeita tamanho do terminal (`tea.WindowSizeMsg`).
- **Aceite:** splash aparece ao iniciar, some ao pressionar tecla ou após 2s, TUI principal abre normalmente; redimensionar o terminal durante o splash não quebra o layout.

### M3.4 — Abas e listas 100% clicáveis com mouse ✅
- [x] Todas as abas e itens de lista respondem a clique simples (seleciona) e duplo clique (ativa ação primária), além do teclado já existente.
- **Toca:** `internal/tui/views/skills.go`, `views/sessions.go`, `views/agents.go`, `internal/tui/app.go` (roteamento de `tea.MouseMsg` para a view ativa).
- **Detalhes:** clicar em aba (Skills / Sessions / Agents) troca a aba ativa; clicar em linha de lista move o cursor; duplo clique executa a ação primária da aba (skills: abre leitor; sessions: abre transcript; agents: sem ação extra). Roda do mouse já funciona — garantir que continue funcionando. Não usar coordenadas absolutas hardcoded: calcular offset a partir do layout renderizado.
- **Aceite:** todas as abas trocam ao clicar; itens de lista selecionam ao clicar; duplo clique abre detalhe; teclado continua funcionando em paralelo; teste manual no tmux com mouse habilitado.

### M3.5 — Perfis por agente (snapshot da matriz) ✅
- [x] Perfil deixa de aplicar "tudo para todos" e passa a fotografar/restaurar a matriz por agente ("do jeito que eu deixei").
- **Toca:** `internal/skill/profiles.go` (novo formato `{nome: {skillDir: [agentIDs]}}`, `ProfileSpec`, `ProfileChange`, `BuildProfileSpec`, `DiffProfile`; `SaveProfile`/`GetProfile` por spec; `ApplyProfile` por agente), `internal/skill/profiles_test.go`, `internal/tui/views/skills.go` (salvar via snapshot, diff por agente, render por agente).
- **Detalhes:** salvar grava, por skill, os agentes onde ela está ativa e gerenciada (`On && !Local`); aplicar faz `Enable`/`Disable` por agente para bater exatamente com o perfil; skills locais nunca são tocadas. Perfis antigos (lista plana) migram automático: lidos como sentinela `"*"` (= todos os agentes instalados) e regravados com IDs concretos ao salvar. `DiffProfile` mostra no confirm o que ativa/desativa **por agente**.
- **Aceite:** salvar/aplicar restaura a matriz por agente e é idempotente (testes table-driven); migração legado testada; diff por agente no confirm; teste manual tmux.

---

## M4 — Distribuição e higiene

### M4.1 — Release v0.1.0 ✅
- [x] Binário instalável sem clonar o repo.
- **Toca:** `.goreleaser.yaml`, `main.go` (version via `-ldflags`), tag `v0.1.0`, repo no GitHub.
- **Aceite:** `goreleaser release --snapshot --clean` gera binários linux/amd64+arm64; `lazyskills --version` mostra a tag. (AUR/PKGBUILD: task separada se valer a pena.)

### M4.2 — Demo no README ✅
- [x] GIF mostrando matriz, toggle, install do GitHub e resume de sessão.
- **Toca:** `demo.tape` (vhs), README.
- **Aceite:** `vhs demo.tape` gera o GIF; README exibe.

### M4.3 — Deletar sessão pela TUI (com rede de segurança)
- [x] Higiene do histórico sem `rm` manual. Última task por ser a mais destrutiva.
- **Toca:** `agent.Adapter` (método `DeleteSession(s) error`), `views/sessions.go` (tecla `d` + confirm destacando agente e título; `space` para seleção múltipla).
- **Detalhes:** claude/codex/gemini = mover o arquivo para `~/.lazyskills/backups/sessions/` (não apagar). opencode = delegar ao próprio CLI (`opencode session delete <id>`). Sessão com processo vivo (arquivo em `~/.claude/sessions/*.json` com o mesmo id) → recusar. **Batch:** `space` marca/desmarca a sessão (indicador `✓` na linha); com seleção ativa, `d` deleta o lote com confirm mostrando a contagem por agente; ao final, toast com sucessos/falhas (falha em uma não aborta as demais — mesmo padrão de erros agregados do `session.Service.List`).
- **Aceite:** deletar move o arquivo pro backup e some da lista; sessão ativa é recusada; batch de 3 sessões com 1 falha deleta as outras 2 e reporta; testes por adapter.

---

## M5 — Paridade com cc-switch (gaps de skills e sessões)

Origem: análise comparativa com o [cc-switch](https://github.com/farion1231/cc-switch) (app desktop Tauri/Rust, ~114k stars, cobre os mesmos agentes). Só entraram gaps dentro do escopo do projeto (skills + sessões). Ordem por valor/custo.

### M5.1 — Detectar update disponível + Update All

- [x] Hoje o `u` (M2.2) atualiza às cegas: o usuário não sabe *quando* há versão nova. Detectar por hash de conteúdo, como o cc-switch (SHA-256).
- **Toca:** `internal/skill/install.go` (campo `Hash string` em `Origin` + func `hashDir(dir) (string, error)`), `internal/skill/ops.go` (`CheckUpdates(skills []Skill) ([]UpdateCheck, error)` + `UpdateAll`), `internal/tui/views/skills.go` (tecla `U` + badge).
- **Detalhes:**
  - `hashDir`: SHA-256 estável do conteúdo — `filepath.WalkDir` com paths relativos ordenados, concatenando `rel + "\x00" + conteúdo` de cada arquivo regular; **ignora ocultos** (mesma regra do `replaceDir` em ops.go), assim `.origin.json` não entra no hash.
  - Gravar `Hash` no `.origin.json` em `Install` e `Update` (via `writeOrigin`). `.origin.json` antigo sem `hash` → tratar como desconhecido (oferece update).
  - `CheckUpdates`: só skills com `Origin.Type == "git"`. **Agrupar por `Origin.Source`** para clonar cada repo uma única vez (`cloneShallow`), localizar cada skill com `locateInClone` e comparar `hashDir(remoto)` vs `hashDir(local)`. Retornar por skill: `EmDia | UpdateDisponivel | EditadaLocalmente` (local ≠ hash gravado → editada localmente; nesse caso Update All pula e reporta, só `u` individual com confirm sobrescreve).
  - TUI: operação de rede → **só sob demanda** (tecla `U`), nunca no startup nem no `Scan`; roda em `tea.Cmd` com toast "verificando updates…". Resultado: badge `↑` na linha da lista e no card; confirm mostrando quais serão atualizadas antes de aplicar `Update` em cada uma (reusa M2.2, que já preserva symlinks e faz backup).
- **Aceite:** teste com repo fixture local (`git init` em `t.TempDir()`, mesmo padrão do teste do `Update` em ops_test.go): sem commit novo → 0 updates; commit novo → detecta e Update All resolve; skill editada localmente → reportada e pulada pelo Update All; `.origin.json` sem `hash` não quebra; duas skills do mesmo repo → um clone só.

### M5.2 — Restore de backup pela TUI

- [x] `Remove`, `Adopt` e `Update` já geram `.tar.gz` em `~/.lazyskills/backups` (`backupDir`, ops.go), mas restaurar é manual. Fechar o ciclo.
- **Toca:** `internal/skill/ops.go` (`type Backup{Skill, Time, Path string}`, `ListBackups() ([]Backup, error)`, `Restore(b Backup) error`), `internal/tui/views/skills.go` (tecla `b` abre picker de backups reusando `picker.go`).
- **Detalhes:**
  - `ListBackups`: parsear nomes `<skill>.<ts>.tar.gz` do `BackupsDir()` (ts formato `20060102T150405`, gerado pelo `backupDir`) — atenção: o nome da skill pode conter pontos? Não (kebab-case, `skillNameRe`), mas parsear do fim (últimos 2 componentes são ts e extensão dupla). Ordenar mais recente primeiro.
  - `Restore`: extrair o tar.gz para `LibraryDir()/<skill>` com as **mesmas proteções do `extractZip`** (install.go): `filepath.Clean`, recusar `..` e paths absolutos, ignorar symlinks, `io.LimitReader` 64 MB, escrita via `fsutil.WriteAtomic`.
  - Se a skill **já existe** na biblioteca: safety backup do estado atual (`backupDir`) antes de sobrescrever — nunca perde nada (padrão cc-switch). Restaurar não mexe em symlinks de agentes (se existiam, voltam a funcionar sozinhos porque apontam pra pasta).
  - Rotação: ao gravar backup, manter no máximo 20 por skill (apagar os mais antigos; ver se `fsutil.RotateBackups` serve ou adaptar).
- **Aceite:** round-trip `Remove` → `Restore` devolve a árvore idêntica (teste comparando arquivos); restaurar sobre skill existente gera safety backup antes; tar.gz corrompido → erro amigável e biblioteca intacta; rotação mantém 20; picker na TUI mostra nome + data e restaura com confirm (tmux).

### M5.3 — Busca de sessões por projeto

- [x] O filtro `/` não encontra sessões pelo nome do projeto — o cwd ficou fora do `FilterValue` de propósito no M1.2 (fuzzy casava letras espalhadas pelo path inteiro).
- **Toca:** `internal/tui/views/sessions.go` (só o `FilterValue` de `sessionItem`, sessions.go:65).
- **Detalhes:** incluir o **basename** do CWD (`filepath.Base(s.CWD)`) no fim do `FilterValue` — não o path inteiro, que foi a causa do problema do M1.2. Ordem: `tag + título + basename` (fuzzy do bubbles ranqueia matches no início melhor).
- **Aceite:** filtrar pelo nome da pasta do projeto encontra a sessão; regressão do M1.2 coberta: "gemini" continua retornando só sessões gemini (teste manual tmux); sessão com CWD vazio não quebra.

### M5.4 — Corrigir pasta no resume (directory picker)

- [x] Retomar sessão de projeto que foi movido/renomeado falha ou cai em pasta errada. Hoje o adapter do opencode silenciosamente troca cwd inexistente pelo home (opencode.go:105-109); os demais nem verificam.
- **Toca:** `internal/tui/views/sessions.go` (novo modo `sessModeDir` com `textinput`, mesmo padrão do input de install em skills.go), `internal/agent/opencode.go` (remover o fallback silencioso — `ResumeCmd` devolve o CWD original; quem decide é a view).
- **Detalhes:**
  - No `enter`/duplo clique: se o `dir` retornado por `ResumeCmd` não existe → em vez de falhar, abrir input pré-preenchido com o path para o usuário corrigir; `enter` confirma (expande `~`), `esc` cancela.
  - Tecla `R` = "retomar em outra pasta": abre o mesmo input mesmo com dir válido.
  - Pasta digitada inexistente → toast de erro, permanece no input.
- **Aceite:** sessão com cwd inexistente abre o picker em vez de falhar; `R` retoma em pasta alternativa; `esc` volta à lista; opencode sem cwd válido não cai mais no home silenciosamente; teste manual tmux.

### M5.5 — OpenCode: sessões do storage JSON legado (dedupe)

- [x] Versões antigas do opencode guardavam sessões em JSON no filesystem; o adapter só lê o SQLite (opencode.go:61). O cc-switch lê ambos e deduplica.
- **Toca:** `internal/agent/opencode.go` (`ListSessions`).
- **Detalhes:** primeiro **verificar na máquina** o layout real do storage legado (`~/.local/share/opencode/storage/session/...` ou similar — inspecionar uma instalação antiga; se não houver evidência, fechar a task como "não se aplica"). Leitura best-effort: JSON inválido é pulado, nunca derruba. Dedupe por `Session.ID` — SQLite vence (mais atual). Bônus: com storage JSON presente e `sqlite3` fora do PATH, listar as do JSON em vez do erro atual (opencode.go:66-69).
- **Aceite:** fixture com a mesma sessão nas duas fontes lista uma vez; sessão só no JSON aparece; sem `sqlite3` mas com JSON → lista parcial em vez de erro; testes com `t.TempDir()`.

### M5.6 — Biblioteca em `~/.agents/skills` (interop)

- [x] `~/.agents/skills` é convenção comunitária emergente para skills compartilhadas entre ferramentas (o opencode já lê — ver `ReadDirs` em opencode.go:38-42; o cc-switch oferece como storage alternativo). Permitir usar esse dir como biblioteca torna o lazyskills interoperável sem symlink para essas ferramentas.
- **Toca:** `internal/skill/skill.go` (novo `LoadPaths`: lê `~/.lazyskills/config.json` — `{"libraryDir": "~/.agents/skills"}` — e `Paths.LibraryDir()` honra o override; config ausente = comportamento atual), `internal/skill/ops.go` (`MigrateLibrary(newDir string, agents []agent.Agent) error`), `internal/cli/cli.go` (subcomando `migrate-library <dir>` — migração fica só no CLI, fora da TUI).
- **Detalhes:**
  - Config via `fsutil.WriteAtomic`; campos desconhecidos do JSON sobrevivem ao round-trip (mesma regra dos perfis M3.1).
  - `MigrateLibrary`: para cada skill da biblioteca: `copyDir` para o novo dir → refazer os symlinks **gerenciados** nos agentes (detectar com a mesma lógica do `Remove`, ops.go:180-199: `Readlink` + `insideDir` na biblioteca antiga) → remover a origem. Backup `.tar.gz` de cada skill antes. Idempotente: rodar de novo não faz nada.
  - **Armadilha do Scan:** se o `libraryDir` coincidir com um `ReadDirs` de agente (caso do opencode com `~/.agents/skills`), o passo 2 do `Scan` (skill.go:104-146) marcaria as skills da biblioteca como `Local` (são dirs reais). Corrigir: dir de leitura igual ao `LibraryDir()` → estado compartilhado/on sem `Local` (a skill é nossa).
- **Aceite:** sem config, tudo como antes (zero regressão na suíte); migração move skills, refaz symlinks e preserva ativações; re-rodar é no-op; Scan com biblioteca em `~/.agents/skills` não duplica nem marca como local no opencode; testes com `t.TempDir()`.

### Ideias avaliadas e não planejadas (por ora)

- Registry/marketplace de skills (busca no skills.sh, repos pré-configurados): caro, depende de serviço externo — reavaliar depois do M5. **→ reavaliado: virou M9.**
- Agrupamento de sessões agente → projeto e TOC no transcript: melhorias de navegação, sem dor concreta ainda. **→ agrupamento virou M8.A4.**
- Symlink vs cópia configurável na ativação: symlink resolve; cópia criaria drift entre agentes.

---

## M6 — Refinamento visual (estilo yazi/lazygit)

Princípio: **moldura, foco e ritmo**. Toda mudança fica em `internal/tui/` — nenhum I/O novo, `tui/` continua sem tocar em disco (regra 4). Cada task exige **regressão funcional zero**: mesmas teclas, mesmo mouse, mesmos fluxos. Decisões travadas: **ícones só Unicode** (portabilidade do binário distribuído) e **re-layout emoldurado completo**.

### M6.1 — Tema centralizado (fundação)

- [x] Cores hoje duplicadas e hardcoded em 7 arquivos (`tui/styles.go`, `views/styles.go`, `agents.go`, `splash.go`, `components/confirm.go`, `delegate.go`, `markdown.go`). Impossível refinar com consistência assim.
- **Toca:** novo `internal/tui/theme/theme.go` (tokens semânticos: `Primary`, `Subtle`, `Border`, `BorderFocus`, `Bg`, `Text`, `OK/Warn/Err` + estilos derivados). Os 7 arquivos passam a importar daqui.
- **Aceite:** zero literais `lipgloss.Color("#...")` fora de `theme/`; `gofmt/vet/test/build` verdes; **visual idêntico** ao atual no tmux (refactor puro, regressão zero).

### M6.2 — Componente `Panel` emoldurado

- [x] Base do look yazi: painel com borda arredondada, **título embutido na borda superior** e cor de borda variável (focado = `BorderFocus`, inativo = `Border`).
- **Toca:** novo `internal/tui/components/panel.go` — `Panel{Title, Focused, Width, Height}.Render(content)`.
- **Aceite:** larguras/alturas corretas (teste unitário de dimensão via `lipgloss.Width/Height`); snapshot no tmux com título e foco alternando.

### M6.3 — Barra de título + status bar + abas pill

- [x] Substitui header/tagline/`─` solto por um chrome de verdade.
- **Toca:** `app.go` (`View()`), `theme`.
- **Detalhes:** nome do app como badge à esquerda; **contadores à direita** (skills ativas · nº sessões · `v{versão}`); abas em **pill** com ícone + contador (`● Skills 14`), ativa preenchida; remover os dois separadores de largura total.
- **Aceite:** contadores dinâmicos batem com as views; abas pill clicáveis (mouse do M3.4 preservado); tmux.

### M6.4 — Listas emolduradas: foco + seleção de linha cheia

- [x] Ganho visual maior. Envolver as listas de Skills e Sessões no `Panel` (título `Skills (14)` / `Sessões (61)`).
- **Toca:** `views/skills.go`, `views/sessions.go`, `delegate.go`.
- **Detalhes:** realce de **linha inteira** selecionada (background sutil, não só a barra `│`); esconder o "N items" e os dots de paginação crus da `list` default (footer/scrollbar próprio); borda do painel de lista em `BorderFocus`, detalhe em `Border` (indica foco tipo yazi).
- **Aceite:** painéis com título+contador; linha selecionada com fundo; sem sobras da list default; teclado/mouse/filtro idênticos; tmux nas duas abas.

### M6.5 — Painel de detalhe/preview alinhado

- [x] Padronizar o card de detalhe como `Panel` (título `Detalhe` / `Sessão`) **alinhado em altura** com a lista — hoje flutua e desalinha.
- **Toca:** `views/skills.go`, `views/sessions.go`.
- **Aceite:** molduras esquerda/direita com mesma altura e topo alinhado; key-chips consistentes; tmux.

### M6.6 — Agentes, rodapé e splash

- [x] Coerência do restante do chrome com o novo tema.
- **Toca:** `views/agents.go`, `app.go` (footer/help), `views/splash.go`, `demo.tape`.
- **Detalhes:** grid de Agentes usando o mesmo `Panel`/tema e ícones de status coerentes; rodapé de ajuda com **keycaps** (tecla em chip, ex. `⏎ retoma`); splash com moldura/alinhamento polidos; regravar o GIF.
- **Aceite:** agentes/rodapé/splash coerentes com o tema; `vhs demo.tape` gera GIF novo; tmux.

### M6.7 — Modais coerentes (confirm / picker / inputs)

- [x] Fechar as bordas: todos os diálogos passam pelo tema.
- **Toca:** `components/confirm.go`, `views/picker.go`, inputs de install/perfil/nome em `skills.go`/`sessions.go`.
- **Detalhes:** todos passam pelo `Panel` + tema + keycaps; overlay centralizado consistente.
- **Aceite:** diálogos com moldura/título/keycaps uniformes; fluxos idênticos; tmux.

### M6.8 — Transcript estilo chat (cards empilhados) ✅

- [x] Hoje o transcript (M1.1) é um viewport de largura cheia com rótulos `▶ você` / `◀ agente` e markdown corrido — lê como log, não como conversa. Deixar "tipo chat" (inspiração cc-switch): cada mensagem num card emoldurado, papel no título, cor por papel e ritmo entre turnos.
- **Decisões travadas (usuário):** **cards empilhados** reusando `components.Panel` (ambos à esquerda, não balões alinhados); **só texto** — `Entry{Role,Text}` fica intacto, sem metadados novos. Consequência: trabalho **100% em `internal/tui/`**, zero mudança em `internal/agent/` (regras 1 e 4 preservadas).
- **Toca:**
  - `internal/tui/components/panel.go`: campo opcional `Border lipgloss.Color` (valor zero = comportamento atual, cor por `Focused`). Backward-compat total — os callers de M6.2–M6.7 continuam idênticos.
  - `internal/tui/views/sessions.go`: reescrever `renderTranscript` para empilhar **um `Panel` por `Entry`**; largura de leitura limitada; cor + ícone por papel; 1 linha em branco entre cards. `bodyHeight`, o viewport e o header do `sessModeDoc` (esc/scroll/clique fecha) ficam inalterados.
  - possível: um par de tokens/estilos por papel em `views/styles.go` reusando `theme.Primary`/`theme.OK`/`theme.Subtle` (nada de literais de cor fora de `theme/`).
- **Detalhes:**
  - Largura do card = `min(vp.Width, ~96)`; markdown renderizado em `panel.ContentWidth()` (quebra dentro do card, sem estourar a borda).
  - Papel: `user` → título `▶ você`, borda `theme.Primary`; `assistant` → título `◀ agente`, borda `theme.Subtle` (ou `OK`). Distinção por **cor + ícone**, não por lado.
  - Transcript vazio mantém o hint atual (`(transcript vazio ou em formato desconhecido)`).
  - `ansi.Truncate` do `Panel` já protege linhas longas; conferir que blocos de código longos ficam legíveis dentro do card.
- **Aceite:**
  - transcript de sessão claude abre com cards emoldurados e rola no tmux; `user` e `agente` visualmente distintos (cor + ícone); markdown (código, listas, títulos) continua destacado **dentro** do card.
  - linha JSON corrompida continua não derrubando (parser de `internal/agent` intacto — teste do M1.1 verde).
  - `Panel` com `Border` custom: teste de dimensão (`lipgloss.Width/Height`) igual ao atual; callers sem `Border` com visual idêntico ao de hoje (refactor sem regressão).
  - teste unitário de `renderTranscript` com entries de exemplo: não quebra, produz um bloco por mensagem, respeita a largura.
  - `gofmt -l . && go vet ./... && go test ./... && go build ./...` verdes.

---

## M7 — Polimento de UX e consistência

Com M6 a TUI ficou "emoldurada". M7 fecha os gaps de **feedback, navegação e coerência** que ainda separam do yazi/lazygit. Tudo em `internal/tui/`, **nenhum I/O novo** (regra 4), **regressão funcional zero**.

### M7.1 — Modal de ajuda (`?`) por contexto ✅
- [x] `?` só mostrava 4 teclas globais; as ~15 teclas de cada aba viviam numa linha de hint densa que truncava em terminal estreito. Virou um `Panel` centralizado com todas as teclas agrupadas.
- **Toca:** `internal/tui/views/help.go` (novo: `HelpGroup` + métodos `Help()` de Skills/Sessions/Agents, refletindo as teclas já tratadas — nada inventado); `internal/tui/app.go` (campo `showHelp`, `?` abre / esc·q·? fecham, guarda de mouse, `renderHelp`/`helpColumns` em duas colunas com fallback de coluna única).
- **Aceite:** `?` abre modal com todas as teclas da aba ativa nas 3 abas; fecha sem perder seleção; sem truncar (duas colunas até ~64 cols, uma abaixo disso); `gofmt/vet/test/build` verdes; tmux.

### M7.2 — Spinner em operações assíncronas ✅
- [x] Operações lentas (check updates, install/discover do GitHub, update, transcript, reload) mostravam toast estático; não havia spinner.
- **Toca:** `charm.land/bubbles/v2/spinner` (MiniDot, `theme.Primary`); `skills.go`/`sessions.go` ganham `spin`+`inFlight`, helper `beginSpin(label)` (liga o tick e troca o rótulo), handler de `spinner.TickMsg`, clear de `inFlight` nas mensagens de conclusão e spinner no `toastLine`/toast do View.
- **Aceite:** discover do GitHub anima (`⠴ procurando skills…`) e some ao concluir com `✗`/`✓`; sem loops de tick duplicados; biblioteca intacta ao errar; `gofmt/vet/test/build` verdes; tmux.

### M7.3 — Toast unificado, estilizado e auto-dismiss ✅
- [x] Toast cru, persistente e duplicado em dois arquivos.
- **Toca:** novo `internal/tui/components/toast.go` (selo colorido por tipo `✓`/`✗` + texto na cor, usado pelas duas views); `skills.go`/`sessions.go` ganham `toastSeq` + wrapper `Update` que agenda `expireToastCmd`/`sessExpireToastCmd` (one-shot `tea.Tick` de 4s com guarda de seq por view — sem tocar os ~20 call sites de toast); `ClearToast()` chamado em `app.go` ao trocar de aba.
- **Aceite:** selo com fundo por tipo, some sozinho após ~4s (verificado no tmux), some ao trocar de aba; componente idêntico entre abas; API `m.toast/m.toastErr` preservada; `gofmt/vet/test/build` verdes.

### M7.4 — Foco navegável entre painéis (←/→) ✅
- [x] Detalhe nunca focável/rolável; borda acesa não andava.
- **Toca:** `skills.go`/`sessions.go` ganham `paneID` (lista|detalhe), `detailVP viewport.Model` + `refreshDetail()` (recomputa o conteúdo, reseta o scroll só ao trocar o item), `detailContent()` (descrição/dados **completos**, sem truncar — agora rolam). ←/→ movem o foco (← lista, → detalhe); a borda `BorderFocus` segue o foco; com detalhe focado, as teclas de rolagem (`detailScrollKeys`) e a roda vão pro viewport, as demais continuam agindo sobre o item; clique num painel o foca.
- **Detalhes:** ←/→ estavam livres (paginação da list desativada); as ações (1-9, e, d…) seguem funcionando com o detalhe focado — só a rolagem é desviada.
- **Aceite:** foco alterna e a borda acompanha; detalhe mostra o texto completo e rola quando focado; lista não move enquanto o detalhe está focado; mouse/teclas/filtro preservados; `gofmt/vet/test/build` verdes; tmux nas duas abas.

### M7.5 — Faxina de consistência (refactor puro) ✅
- [x] 3 definições de keycap chip duplicadas.
- **Toca:** `components/panel.go` exporta `KeycapStyle` (fonte única); `app.go` (rodapé de ajuda) e `skills.go` (removido o `keyChip` local; badges numéricos e hints do detalhe agora usam `components.Keycap`) passam a consumir daqui.
- **Decisões (escopo enxuto, sem churn/risco):** larguras `2/5` (skills) × `3/5` (sessões) **mantidas** — diferem de propósito (lista de sessões é mais larga). Splash **mantido** com `lipgloss.RoundedBorder` — já é borda arredondada + `theme.Primary`, visualmente idêntica ao `Panel`; converter só perderia o `Padding(1,4)` mais arejado sem ganho visível. Idiomas de seleção (`[x]` no picker, `✓` em sessões, barra no delegate) preservados — unificar mudaria comportamento visível.
- **Aceite:** zero mudança de comportamento; keycaps idênticos em modais, rodapé e detalhe; `gofmt/vet/test/build` verdes; tmux.

---

## M8 — Dados locais potentes (execução paralela em worktrees)

Origem: segunda análise comparativa (jul/2026) — cc-switch v3.17, ccmanager, ccusage, claude-history, claude-code-log. Nicho confirmado: nenhuma ferramenta combina skills + sessões numa TUI local; M8 reforça esse diferencial só com dados que já estão na máquina (zero rede). Supera duas rejeições antigas do M5 ("agrupamento de sessões" e "TOC/busca no transcript") — a dor chegou.

### ⚙️ Como trabalhar em paralelo (vale para M8 e M9)

As tasks estão divididas em **3 lanes com footprints de arquivos disjuntos**. Cada lane = 1 branch + 1 worktree + 1 agente. Dentro de cada lane vale a regra de sempre: **uma task por vez, na ordem**, verificações verdes + commit por task.

| Lane | Branch | Worktree | Footprint exclusivo |
|---|---|---|---|
| **A — Sessões** | `feat/m8-sessions` | `../lazyskills-sessions` | `internal/agent/`, `internal/session/`, `views/sessions.go`, `internal/cli/` |
| **B — Skills** | `feat/m8-skills` | `../lazyskills-skills` | `internal/skill/`, `views/skills.go`, `views/picker.go` |
| **C — Chrome** | `feat/m8-chrome` | `../lazyskills-chrome` | `internal/tui/app.go`, `internal/tui/components/` (arquivos novos), `.gitignore` |

Regras de convivência:

1. Setup: `git worktree add ../lazyskills-<lane> -b feat/m8-<lane> master`.
2. **Não tocar arquivo fora do footprint da lane.** Compartilhados de risco: `views/help.go` (cada lane edita SÓ a função `Help()` da sua view), `views/styles.go` e `theme/` (só adicionar, nunca renomear/mover), `go.mod` (nenhuma dependência nova sem alinhar). Precisou sair do footprint → **parar e reportar**, não invadir.
3. Marcar o checkbox no BACKLOG.md junto do commit da task (conflitos de checkbox são triviais no rebase).
4. Merge no master na ordem de conclusão; após cada merge, as lanes vivas fazem `git rebase master` antes de continuar.
5. M9 continua nas mesmas lanes **depois** do merge completo do M8 (o M9.1 toca `internal/cli/`, que no M8 pertence à lane A).

---

### Lane A — Sessões

#### M8.A1 — Tokens e custo estimado por sessão
- [x] Os JSONL que já parseamos trazem `usage` por mensagem; ninguém no nicho expõe isso numa TUI de sessões (ccusage prova a demanda). Mostrar tokens/custo no detalhe da sessão.
- **Toca:** `internal/agent/usage.go` (novo: `Usage{Input, Output, CacheRead, CacheWrite int; Model string}` + interface opcional `UsageReader{ SessionUsage(s Session) (Usage, bool) }` — **não** mudar a interface `Adapter`, usar type assertion na view/service), `internal/agent/pricing.go` (novo: tabela de preços embutida por modelo, best-effort — modelo desconhecido = só tokens, sem custo), `views/sessions.go` (linhas de tokens/custo no `detailContent()`; carregar em `tea.Cmd` ao selecionar, com cache por `Session.ID`), `internal/cli/cli.go` (`sessions --json` ganha os campos de usage quando disponíveis).
- **Detalhes:** claude soma `message.usage` das linhas assistant do JSONL (parser resiliente do M1.1 como base); codex/gemini/opencode best-effort — sem usage = `ok=false`, detalhe simplesmente omite as linhas. Nunca no startup: só ao focar a sessão (lazy, cacheado).
- **Aceite:** sessão claude mostra `tokens 12.3k in · 45.6k out · cache 200k` e custo estimado no detalhe; sessão sem usage não mostra nada e não quebra; fixture JSONL com usage testada; `--json` estável; verdes; tmux.

#### M8.A2 — Status vivo das sessões (● ativa)
- [x] A lista não distingue sessão em andamento de histórico morto. O M4.3 já detecta sessão viva para recusar delete — promover a detecção a feature visível (padrão ccmanager: Idle/Busy na lista).
  - Nota: o M4.3 nunca chegou a implementar a recusa (só backup+delete+batch existiam). Esta task cria `IsLive` do zero (via `lsof` em lote, um processo só por reload) e passa a usá-lo como fonte única tanto pro badge quanto pra recusa do delete.
- **Toca:** `internal/agent/` (interface opcional `LiveChecker{ IsLive(s Session) bool }`; claude reusa a checagem do M4.3; demais retornam false por ora), `views/sessions.go` (indicador `●` em `theme.OK` na linha + "ativa" no detalhe; refresh junto do reload `r`).
- **Aceite:** sessão claude em andamento aparece com `●`; delete continua recusando viva (mesma fonte de verdade, sem lógica duplicada); agentes sem suporte não mostram nada; verdes; tmux com uma sessão claude aberta em paralelo.

#### M8.A3 — Busca full-text nos transcripts
- [ ] `/` filtra só tag+título+basename; "onde foi aquela conversa sobre X?" não tem resposta hoje (inspiração: claude-history).
- **Toca:** `internal/session/search.go` (novo: `SearchTranscripts(ads []agent.Adapter, sessions []agent.Session, query string) ([]Match, error)` — usa `Transcript()` de cada adapter, **sem** conhecer paths/formatos, regra 1 intacta; `Match{Session, Excerpt}` com trecho ±40 runas), `views/sessions.go` (tecla `F` abre input de busca; roda em `tea.Cmd` com spinner do M7.2; resultado vira subconjunto da lista com toast `N sessões contêm "x"`; `esc` restaura).
- **Detalhes:** case-insensitive, matching simples (`strings.Contains` sobre texto normalizado); erros de transcript individuais não abortam a busca (mesmo padrão de erros agregados do `session.Service.List`).
- **Aceite:** buscar palavra presente numa sessão antiga a encontra; busca sem resultado avisa e mantém a lista; transcript corrompido não derruba; teste unitário do service com fixtures; verdes; tmux.

#### M8.A4 — Agrupamento agente → projeto na lista
- [ ] Com 60+ sessões a lista plana cansa (cc-switch v3.16.5 agrupou por provider → projeto). Toggle de vista agrupada.
- **Toca:** `views/sessions.go` (tecla `g` alterna flat ↔ agrupada; na agrupada, headers `▸ claude · lazyskills (12)` como itens não-selecionáveis; `space` num header marca/desmarca o grupo inteiro — integra com o batch delete do M4.3).
- **Detalhes:** agrupar por `AgentID` + `filepath.Base(CWD)` (CWD vazio → "sem projeto"); ordenação dentro do grupo por MTime; filtro `/` e ciclo `f` continuam operando sobre a vista ativa; preferência de vista não persiste (sempre abre flat).
- **Aceite:** `g` alterna as vistas; batch por grupo deleta o grupo com confirm mostrando contagem; teclado/mouse/filtro preservados nas duas vistas; verdes; tmux.

#### M8.A5 — Export de transcript para Markdown
- [ ] Fechar o ciclo do leitor: levar a conversa para fora da TUI (inspiração: claude-code-log).
- **Toca:** `internal/session/export.go` (novo: `ExportMarkdown(s agent.Session, entries []agent.Entry, dir string) (path string, err error)` — grava `<agente>-<id>-<ts>.md` via `fsutil.WriteAtomic`), `views/sessions.go` (tecla `x` no `sessModeDoc`; roda em `tea.Cmd`; toast com o path gravado).
- **Detalhes:** destino `<DataDir>/exports/` (padrão XDG: `~/.local/share/lazyskills/exports`, via `skill.Paths`); formato: header com agente/título/data/CWD + `## ▶ você` / `## ◀ agente` por entry.
- **Aceite:** `x` no transcript gera o .md com todas as mensagens; arquivo legível; export de transcript vazio vira toast de aviso sem arquivo; teste unitário do formato; verdes; tmux.

---

### Lane B — Skills

#### M8.B1 — Detector de skills não-gerenciadas
- [x] `adopt` existe, mas nada aponta proativamente o que há para adotar (cc-switch v3.16.4 pôs um indicador). O Scan já marca `Local` — falta dar visibilidade e ação em lote.
- **Toca:** `views/skills.go` (contador `N locais` no título do painel quando houver skill local adotável; tecla `A` abre confirm listando as locais e adota todas), `internal/skill/ops.go` (`AdoptAll(agents) (adopted []string, errs []error)` — loop de `Adopt` com erros agregados, falha em uma não aborta).
- **Aceite:** com skill local presente o título mostra o contador; `A` adota todas com confirm, backups gerados (comportamento do Adopt preservado); sem locais, `A` vira toast informativo; testes do `AdoptAll`; verdes; tmux.

#### M8.B2 — Install seletivo de repo multi-skill
- [x] O install do GitHub instala tudo que o discover encontra; repos como anthropics/skills têm dezenas (padrão do `skills add --skill` da Vercel: escolher antes).
- **Toca:** `internal/skill/install.go` (separar descoberta de instalação: `Discover(origem) ([]Found, cleanup, error)` + `InstallFound(sel []Found)` — o install atual vira `Discover`+todas), `views/skills.go` (após o discover com spinner, se >1 skill: picker multi-select com `space`, `a` marca todas, `enter` instala as marcadas; 1 skill = instala direto como hoje).
- **Aceite:** repo com N skills abre o picker e instala só as marcadas; repo com 1 skill não muda o fluxo; `esc` no picker cancela sem instalar nada (cleanup do clone); dedupe e `.origin.json` preservados; testes do `Discover`/`InstallFound`; verdes; tmux.

#### M8.B3 — Validação de SKILL.md (lint local)
- [x] A spec Agent Skills (agentskills.io) virou padrão aberto adotado por Codex/Cursor/Gemini/OpenCode. Nenhum concorrente faz lint local — diferencial barato.
- **Toca:** `internal/skill/validate.go` (novo: `Validate(sk Skill) []Issue` — frontmatter parseável, `name` presente/kebab-case/igual ao dir, `description` não vazia e ≤ 1024 chars, corpo não vazio), `views/skills.go` (badge `!` em `theme.Warn` na linha + issues listadas no detalhe).
- **Detalhes:** validação roda no Scan (é leitura local barata, sem rede); `Issue{Field, Msg}`.
- **Aceite:** skill sem description ganha badge e issue legível no detalhe; skill válida não mostra nada; testes table-driven do `Validate`; verdes; tmux.

---

### Lane C — Chrome

#### M8.C1 — Command palette (`:`) ✅
- [x] Padrão k9s: `:` abre um input com fuzzy sobre comandos nomeados — descobribilidade sem decorar tecla.
- **Toca:** `internal/tui/components/palette.go` (novo: `Command{Name,Desc}` + `Palette` com input + lista filtrada por substring case-insensitive), `internal/tui/app.go` (`keys.Palette` (`:`), `showPalette`/`palette`, `runPaletteCommand` — `skills`/`sessions`/`agents` trocam de aba, `help` abre o modal, `quit` sai, `reload` sintetiza a tecla `r` via `updateActive` sem tocar as views), `internal/tui/keys.go` (binding `Palette`).
- **Detalhes:** só comandos **globais**, gate por `!capturingInput()` igual às demais ações globais (doc mode/inputs das views não são interrompidos); guarda de mouse/wheel/paste igual ao modal de ajuda; entrada `:` somada ao grupo "Navegação" do `?`.
- **Aceite:** `:ses`+enter vai para Sessões (confirmado tmux); `:q`+enter sai; `:xyz` sem match mostra "nenhum comando" sem quebrar; esc fecha sem efeito; `:` não colide com teclas existentes; modal `?` lista a nova tecla; `gofmt/vet/test/build` verdes.

#### M8.C2 — Higiene do repo ✅
- [x] Binário `lazyskills` compilado na raiz aparece como untracked (`.gitignore` tinha o typo `lazkills`, sem a letra "y").
- **Toca:** `.gitignore` (corrigido para `/lazyskills`).
- **Aceite:** `git status` limpo após build local.

---

## M9 — Descoberta de skills (rede, sob demanda) — segunda onda

Reavaliação prometida no M5 ("registry/marketplace: reavaliar depois do M5"): o cenário mudou — spec aberta em agentskills.io e registry público skills.sh (API GA, ~600k skills). Regra de ouro herdada do M5.1: **rede só sob demanda**, nunca no startup nem no Scan. Executar nas mesmas lanes após o merge do M8 (rebase antes).

#### M9.1 — `doctor` valida skills (lane B — primeiro task pós-merge, toca `internal/cli/`)
- [ ] Levar o `Validate` do M8.B3 ao CLI.
- **Toca:** `internal/cli/cli.go` (`doctor` lista issues por skill; exit code 1 se houver issue).
- **Aceite:** `lazyskills doctor` reporta skill inválida com campo+mensagem; tudo válido = exit 0; teste via service.

#### M9.2 — Busca no registry skills.sh (lane B)
- [ ] Descobrir skills sem sair da TUI (cc-switch v3.13 integrou; a API pública é HTTP simples).
- **Toca:** `internal/skill/registry.go` (novo: client HTTP da API do skills.sh — busca por termo, retorna `{Name, Repo, Description}`; timeout 10s; sem dependência nova, `net/http` stdlib), `views/skills.go` (tecla `S` abre input de busca remota → spinner → picker de resultados → `enter` instala via fluxo GitHub existente, reusando o install seletivo do M8.B2).
- **Detalhes:** erro de rede = toast, biblioteca intacta; nenhum request fora da ação explícita do usuário.
- **Aceite:** buscar termo conhecido lista resultados e instala o escolhido; sem rede → toast de erro amigável; teste do client com `httptest.Server`; verdes; tmux.

#### M9.3 — Marketplaces no formato oficial (lane B)
- [ ] Ler `.claude-plugin/marketplace.json` de repos git (formato oficial dos plugins do Claude Code) como fonte adicional de skills — é git + JSON, casa com o `git clone --depth 1` existente.
- **Toca:** `internal/skill/marketplace.go` (novo: `LoadMarketplace(url) ([]Entry, error)` via clone raso + parse do JSON), `views/skills.go` (entrada no fluxo de install: origem que contenha `.claude-plugin/marketplace.json` lista as entries no picker).
- **Aceite:** repo fixture com marketplace.json lista e instala uma entry; JSON inválido = erro amigável; testes com `t.TempDir()`; verdes; tmux.

#### M9.4 — Rename/bookmark de sessão (lane A)
- [ ] Sessões com títulos crus são difíceis de reencontrar (padrão cc-sessions). ⚠️ Renomear o título **no arquivo do CLI** mexe em arquivo vivo — em vez disso, apelido próprio, não invasivo.
- **Toca:** `internal/session/alias.go` (novo: `<DataDir>/session-aliases.json` no padrão XDG — `{sessionID: apelido}` via `fsutil.WriteAtomic`, campos desconhecidos sobrevivem), `views/sessions.go` (tecla `m` renomeia com input; apelido aparece no lugar do título com marcador sutil; `FilterValue` inclui o apelido).
- **Aceite:** apelido sobrevive a restart; filtro encontra pelo apelido; remover apelido (input vazio) volta ao título original; round-trip testado; verdes; tmux.

---

## Fora de escopo (decidido)

- Watch automático de filesystem (`r` recarrega; fsnotify é complexidade sem dor real)
- Sync em nuvem, system tray, auto-updater, i18n
- Gerenciar skills embutidas em plugins do Claude Code (são do marketplace do plugin)
