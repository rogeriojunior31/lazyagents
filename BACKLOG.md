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
- [x] Exibir uma tela de boas-vindas ao abrir o lazykills antes de entrar na TUI principal.
- **Toca:** `internal/tui/views/splash.go` (novo: modelo Bubble Tea standalone com logo ASCII, versão, dica de teclas), `internal/tui/app.go` (estado inicial `stateSplash` → transita para `stateMain` ao pressionar qualquer tecla ou após timeout configurável).
- **Detalhes:** logo em ASCII art com o nome "lazyskills"; linha de versão (`ldflags`); lista das teclas principais (`?` ajuda, `tab` muda aba, `q` sai); timeout de 2s ou qualquer tecla avança. Respeita tamanho do terminal (`tea.WindowSizeMsg`).
- **Aceite:** splash aparece ao iniciar, some ao pressionar tecla ou após 2s, TUI principal abre normalmente; redimensionar o terminal durante o splash não quebra o layout.

### M3.4 — Abas e listas 100% clicáveis com mouse ✅
- [x] Todas as abas e itens de lista respondem a clique simples (seleciona) e duplo clique (ativa ação primária), além do teclado já existente.
- **Toca:** `internal/tui/views/skills.go`, `views/sessions.go`, `views/agents.go`, `internal/tui/app.go` (roteamento de `tea.MouseMsg` para a view ativa).
- **Detalhes:** clicar em aba (Skills / Sessions / Agents) troca a aba ativa; clicar em linha de lista move o cursor; duplo clique executa a ação primária da aba (skills: abre leitor; sessions: abre transcript; agents: sem ação extra). Roda do mouse já funciona — garantir que continue funcionando. Não usar coordenadas absolutas hardcoded: calcular offset a partir do layout renderizado.
- **Aceite:** todas as abas trocam ao clicar; itens de lista selecionam ao clicar; duplo clique abre detalhe; teclado continua funcionando em paralelo; teste manual no tmux com mouse habilitado.

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

- [ ] Hoje o `u` (M2.2) atualiza às cegas: o usuário não sabe *quando* há versão nova. Detectar por hash de conteúdo, como o cc-switch (SHA-256).
- **Toca:** `internal/skill/install.go` (campo `Hash string` em `Origin` + func `hashDir(dir) (string, error)`), `internal/skill/ops.go` (`CheckUpdates(skills []Skill) ([]UpdateCheck, error)` + `UpdateAll`), `internal/tui/views/skills.go` (tecla `U` + badge).
- **Detalhes:**
  - `hashDir`: SHA-256 estável do conteúdo — `filepath.WalkDir` com paths relativos ordenados, concatenando `rel + "\x00" + conteúdo` de cada arquivo regular; **ignora ocultos** (mesma regra do `replaceDir` em ops.go), assim `.origin.json` não entra no hash.
  - Gravar `Hash` no `.origin.json` em `Install` e `Update` (via `writeOrigin`). `.origin.json` antigo sem `hash` → tratar como desconhecido (oferece update).
  - `CheckUpdates`: só skills com `Origin.Type == "git"`. **Agrupar por `Origin.Source`** para clonar cada repo uma única vez (`cloneShallow`), localizar cada skill com `locateInClone` e comparar `hashDir(remoto)` vs `hashDir(local)`. Retornar por skill: `EmDia | UpdateDisponivel | EditadaLocalmente` (local ≠ hash gravado → editada localmente; nesse caso Update All pula e reporta, só `u` individual com confirm sobrescreve).
  - TUI: operação de rede → **só sob demanda** (tecla `U`), nunca no startup nem no `Scan`; roda em `tea.Cmd` com toast "verificando updates…". Resultado: badge `↑` na linha da lista e no card; confirm mostrando quais serão atualizadas antes de aplicar `Update` em cada uma (reusa M2.2, que já preserva symlinks e faz backup).
- **Aceite:** teste com repo fixture local (`git init` em `t.TempDir()`, mesmo padrão do teste do `Update` em ops_test.go): sem commit novo → 0 updates; commit novo → detecta e Update All resolve; skill editada localmente → reportada e pulada pelo Update All; `.origin.json` sem `hash` não quebra; duas skills do mesmo repo → um clone só.

### M5.2 — Restore de backup pela TUI

- [ ] `Remove`, `Adopt` e `Update` já geram `.tar.gz` em `~/.lazyskills/backups` (`backupDir`, ops.go), mas restaurar é manual. Fechar o ciclo.
- **Toca:** `internal/skill/ops.go` (`type Backup{Skill, Time, Path string}`, `ListBackups() ([]Backup, error)`, `Restore(b Backup) error`), `internal/tui/views/skills.go` (tecla `b` abre picker de backups reusando `picker.go`).
- **Detalhes:**
  - `ListBackups`: parsear nomes `<skill>.<ts>.tar.gz` do `BackupsDir()` (ts formato `20060102T150405`, gerado pelo `backupDir`) — atenção: o nome da skill pode conter pontos? Não (kebab-case, `skillNameRe`), mas parsear do fim (últimos 2 componentes são ts e extensão dupla). Ordenar mais recente primeiro.
  - `Restore`: extrair o tar.gz para `LibraryDir()/<skill>` com as **mesmas proteções do `extractZip`** (install.go): `filepath.Clean`, recusar `..` e paths absolutos, ignorar symlinks, `io.LimitReader` 64 MB, escrita via `fsutil.WriteAtomic`.
  - Se a skill **já existe** na biblioteca: safety backup do estado atual (`backupDir`) antes de sobrescrever — nunca perde nada (padrão cc-switch). Restaurar não mexe em symlinks de agentes (se existiam, voltam a funcionar sozinhos porque apontam pra pasta).
  - Rotação: ao gravar backup, manter no máximo 20 por skill (apagar os mais antigos; ver se `fsutil.RotateBackups` serve ou adaptar).
- **Aceite:** round-trip `Remove` → `Restore` devolve a árvore idêntica (teste comparando arquivos); restaurar sobre skill existente gera safety backup antes; tar.gz corrompido → erro amigável e biblioteca intacta; rotação mantém 20; picker na TUI mostra nome + data e restaura com confirm (tmux).

### M5.3 — Busca de sessões por projeto

- [ ] O filtro `/` não encontra sessões pelo nome do projeto — o cwd ficou fora do `FilterValue` de propósito no M1.2 (fuzzy casava letras espalhadas pelo path inteiro).
- **Toca:** `internal/tui/views/sessions.go` (só o `FilterValue` de `sessionItem`, sessions.go:65).
- **Detalhes:** incluir o **basename** do CWD (`filepath.Base(s.CWD)`) no fim do `FilterValue` — não o path inteiro, que foi a causa do problema do M1.2. Ordem: `tag + título + basename` (fuzzy do bubbles ranqueia matches no início melhor).
- **Aceite:** filtrar pelo nome da pasta do projeto encontra a sessão; regressão do M1.2 coberta: "gemini" continua retornando só sessões gemini (teste manual tmux); sessão com CWD vazio não quebra.

### M5.4 — Corrigir pasta no resume (directory picker)

- [ ] Retomar sessão de projeto que foi movido/renomeado falha ou cai em pasta errada. Hoje o adapter do opencode silenciosamente troca cwd inexistente pelo home (opencode.go:105-109); os demais nem verificam.
- **Toca:** `internal/tui/views/sessions.go` (novo modo `sessModeDir` com `textinput`, mesmo padrão do input de install em skills.go), `internal/agent/opencode.go` (remover o fallback silencioso — `ResumeCmd` devolve o CWD original; quem decide é a view).
- **Detalhes:**
  - No `enter`/duplo clique: se o `dir` retornado por `ResumeCmd` não existe → em vez de falhar, abrir input pré-preenchido com o path para o usuário corrigir; `enter` confirma (expande `~`), `esc` cancela.
  - Tecla `R` = "retomar em outra pasta": abre o mesmo input mesmo com dir válido.
  - Pasta digitada inexistente → toast de erro, permanece no input.
- **Aceite:** sessão com cwd inexistente abre o picker em vez de falhar; `R` retoma em pasta alternativa; `esc` volta à lista; opencode sem cwd válido não cai mais no home silenciosamente; teste manual tmux.

### M5.5 — OpenCode: sessões do storage JSON legado (dedupe)

- [ ] Versões antigas do opencode guardavam sessões em JSON no filesystem; o adapter só lê o SQLite (opencode.go:61). O cc-switch lê ambos e deduplica.
- **Toca:** `internal/agent/opencode.go` (`ListSessions`).
- **Detalhes:** primeiro **verificar na máquina** o layout real do storage legado (`~/.local/share/opencode/storage/session/...` ou similar — inspecionar uma instalação antiga; se não houver evidência, fechar a task como "não se aplica"). Leitura best-effort: JSON inválido é pulado, nunca derruba. Dedupe por `Session.ID` — SQLite vence (mais atual). Bônus: com storage JSON presente e `sqlite3` fora do PATH, listar as do JSON em vez do erro atual (opencode.go:66-69).
- **Aceite:** fixture com a mesma sessão nas duas fontes lista uma vez; sessão só no JSON aparece; sem `sqlite3` mas com JSON → lista parcial em vez de erro; testes com `t.TempDir()`.

### M5.6 — Biblioteca em `~/.agents/skills` (interop)

- [ ] `~/.agents/skills` é convenção comunitária emergente para skills compartilhadas entre ferramentas (o opencode já lê — ver `ReadDirs` em opencode.go:38-42; o cc-switch oferece como storage alternativo). Permitir usar esse dir como biblioteca torna o lazyskills interoperável sem symlink para essas ferramentas.
- **Toca:** `internal/skill/skill.go` (novo `LoadPaths`: lê `~/.lazyskills/config.json` — `{"libraryDir": "~/.agents/skills"}` — e `Paths.LibraryDir()` honra o override; config ausente = comportamento atual), `internal/skill/ops.go` (`MigrateLibrary(newDir string, agents []agent.Agent) error`), `internal/cli/cli.go` (subcomando `migrate-library <dir>` — migração fica só no CLI, fora da TUI).
- **Detalhes:**
  - Config via `fsutil.WriteAtomic`; campos desconhecidos do JSON sobrevivem ao round-trip (mesma regra dos perfis M3.1).
  - `MigrateLibrary`: para cada skill da biblioteca: `copyDir` para o novo dir → refazer os symlinks **gerenciados** nos agentes (detectar com a mesma lógica do `Remove`, ops.go:180-199: `Readlink` + `insideDir` na biblioteca antiga) → remover a origem. Backup `.tar.gz` de cada skill antes. Idempotente: rodar de novo não faz nada.
  - **Armadilha do Scan:** se o `libraryDir` coincidir com um `ReadDirs` de agente (caso do opencode com `~/.agents/skills`), o passo 2 do `Scan` (skill.go:104-146) marcaria as skills da biblioteca como `Local` (são dirs reais). Corrigir: dir de leitura igual ao `LibraryDir()` → estado compartilhado/on sem `Local` (a skill é nossa).
- **Aceite:** sem config, tudo como antes (zero regressão na suíte); migração move skills, refaz symlinks e preserva ativações; re-rodar é no-op; Scan com biblioteca em `~/.agents/skills` não duplica nem marca como local no opencode; testes com `t.TempDir()`.

### Ideias avaliadas e não planejadas (por ora)

- Registry/marketplace de skills (busca no skills.sh, repos pré-configurados): caro, depende de serviço externo — reavaliar depois do M5.
- Agrupamento de sessões agente → projeto e TOC no transcript: melhorias de navegação, sem dor concreta ainda.
- Symlink vs cópia configurável na ativação: symlink resolve; cópia criaria drift entre agentes.

---

## Fora de escopo (decidido)

- Watch automático de filesystem (`r` recarrega; fsnotify é complexidade sem dor real)
- Sync em nuvem, system tray, auto-updater, i18n
- Gerenciar skills embutidas em plugins do Claude Code (são do marketplace do plugin)
