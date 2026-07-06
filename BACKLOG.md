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

## M1 — Sessões e edição no dia a dia

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

### M1.4 — Criar skill nova com `n`
- [ ] Criar uma skill do zero pela TUI.
- **Toca:** `internal/skill/ops.go` (`Create(name) (path, error)`: valida nome kebab-case, recusa existente, grava template com frontmatter via `fsutil.WriteAtomic`), `views/skills.go` (input de nome reusando o textinput do install).
- **Detalhes:** template mínimo: frontmatter `name`/`description` + seção de instruções. Depois de criar, abre o `$EDITOR` (reusa M1.3) e rescan ao voltar.
- **Aceite:** `n` + nome cria `~/.lazyskills/skills/<nome>/SKILL.md` válido, abre editor, aparece na lista ao voltar; nome inválido/duplicado vira toast de erro; testes unitários do `Create`.

---

## M2 — Ciclo de vida das skills

### M2.1 — Registrar origem da instalação
- [ ] Saber de onde cada skill da biblioteca veio, para poder atualizar.
- **Toca:** `internal/skill/install.go` (gravar `.origin.json` — `{type: git|zip|dir, url|path, installedAt}` — dentro da pasta da skill na biblioteca, via `fsutil.WriteAtomic`), `internal/skill/skill.go` (`Skill.Origin` no scan), `views/skills.go` (mostrar origem no card de detalhes).
- **Detalhes:** `.origin.json` começa com `.` → invisível para os agentes; descoberta ignora dirs ocultos, então não vira "skill". Adopt registra `{type: dir, path: origem}`.
- **Aceite:** instalar do GitHub grava a URL; scan expõe; card mostra "origem github.com/x/y"; round-trip testado.

### M2.2 — Atualizar skill do GitHub com `u`
- [ ] Re-instalar a versão mais nova de uma skill que veio de repositório.
- **Toca:** `internal/skill/ops.go` (`Update(sk)`: clone raso da origem, redescobre a skill pelo nome, backup `.tar.gz` da atual, substitui conteúdo preservando `.origin.json`), `views/skills.go` (tecla `u` + confirm).
- **Detalhes:** symlinks nos agentes não mudam (apontam pra pasta, que é substituída no lugar). Skill sem origem git → toast explicando.
- **Aceite:** update substitui o conteúdo, gera backup, mantém ativações; sem origem → erro amigável; teste com repo fixture local (`git init` em TempDir).

### M2.3 — CLI headless
- [ ] Usar tudo sem TUI, scriptável (inclusive por agentes de código).
- **Toca:** `main.go` (dispatch de subcomandos, stdlib `flag`; sem lib de CLI), talvez `internal/cli/` se `main.go` passar de ~200 linhas.
- **Comandos:** `lazyskills list [--json]` · `enable <skill> [--agent id|--all]` · `disable <skill> [--agent id|--all]` · `install <origem>` (não interativo: instala tudo que descobrir) · `remove <skill>` · `adopt <skill> --agent id` · `sessions [--json]` · `doctor` (agentes detectados + problemas: symlink quebrado, SKILL.md inválido).
- **Aceite:** cada comando com saída limpa e exit code correto; `--json` estável para script; `lazyskills` sem argumentos continua abrindo a TUI; testes dos comandos via services (não via exec).

---

## M3 — Perfis de skills

### M3.1 — Service de perfis
- [ ] Conjuntos nomeados de skills aplicáveis de uma vez ("trabalho", "pessoal").
- **Toca:** `internal/skill/profiles.go` (novo: `~/.lazyskills/profiles.json` — `{nome: [skills]}` — via `fsutil.WriteAtomic`; `SaveProfile(nome, skills)`, `ApplyProfile(nome, agents)`).
- **Detalhes:** semântica de aplicar = **garantir as listadas ativas em todos os agentes** e desativar as *gerenciadas* que ficaram de fora (skills locais nunca são tocadas — mesma regra do DisableAll). Perfil que referencia skill inexistente → erro listando as faltantes.
- **Aceite:** salvar/aplicar com testes table-driven; aplicar é idempotente; campos desconhecidos no JSON sobrevivem ao round-trip.

### M3.2 — Perfis na TUI
- [ ] Tecla `p` na aba Skills: lista perfis → `enter` aplica (com confirm mostrando o diff: o que ativa/desativa) · `s` salva o estado atual como perfil novo.
- **Toca:** `views/skills.go` (novo modo, reusa o padrão do picker).
- **Aceite:** ciclo completo no tmux: salvar perfil, bagunçar ativações, aplicar, matriz volta ao estado do perfil.

---

## M4 — Distribuição e higiene

### M4.1 — Release v0.1.0
- [ ] Binário instalável sem clonar o repo.
- **Toca:** `.goreleaser.yaml`, `main.go` (version via `-ldflags`), tag `v0.1.0`, repo no GitHub.
- **Aceite:** `goreleaser release --snapshot --clean` gera binários linux/amd64+arm64; `lazyskills --version` mostra a tag. (AUR/PKGBUILD: task separada se valer a pena.)

### M4.2 — Demo no README
- [ ] GIF mostrando matriz, toggle, install do GitHub e resume de sessão.
- **Toca:** `demo.tape` (vhs), README.
- **Aceite:** `vhs demo.tape` gera o GIF; README exibe.

### M4.3 — Deletar sessão pela TUI (com rede de segurança)
- [ ] Higiene do histórico sem `rm` manual. Última task por ser a mais destrutiva.
- **Toca:** `agent.Adapter` (método `DeleteSession(s) error`), `views/sessions.go` (tecla `d` + confirm destacando agente e título).
- **Detalhes:** claude/codex/gemini = mover o arquivo para `~/.lazyskills/backups/sessions/` (não apagar). opencode = delegar ao próprio CLI (`opencode session delete <id>`). Sessão com processo vivo (arquivo em `~/.claude/sessions/*.json` com o mesmo id) → recusar.
- **Aceite:** deletar move o arquivo pro backup e some da lista; sessão ativa é recusada; testes por adapter.

---

## Fora de escopo (decidido)

- Watch automático de filesystem (`r` recarrega; fsnotify é complexidade sem dor real)
- Sync em nuvem, system tray, auto-updater, i18n
- Gerenciar skills embutidas em plugins do Claude Code (são do marketplace do plugin)
