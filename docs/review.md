# Revisão da versão atual — 22/09/2026

Base: `04c771d` (árvore limpa no início). Revisão transversal de código, testes,
contratos, documentação, exemplos, scripts e distribuição. As correções estão na
árvore de trabalho; nenhuma tag, publicação ou push faz parte desta rodada.

## Achados corrigidos

Severidade: alta = possibilidade de alteração indevida/perda de dados;
média = resultado incorreto, indisponibilidade ou quebra de integração;
baixa = documentação, diagnóstico ou manutenção. Cada linha identifica a
reprodução original, a correção e a evidência executável.

| ID | Severidade | Local e evidência original | Correção e validação |
| --- | --- | --- | --- |
| R01 | Alta | `skills.Install`: `name: ../../escaped` do frontmatter chegava ao destino sem validação | Recusa componentes inseguros antes de copiar; `TestInstallRejectsEscapingName` |
| R02 | Alta | `skills.Restore`: backup inválido era aberto depois de remover o estado atual | Extração em staging, validação do gzip, backup e troca com rollback; `TestRestoreCorruptBackupKeepsExistingSkill` |
| R03 | Alta | `skills.replaceDir`: removia conteúdo antes de confirmar a cópia remota | Prepara o novo estado em staging e mantém o anterior até a troca; `TestUpdateMissingSourcePreservesContents` |
| R04 | Alta | `skills.MigrateLibrary`: ignorava conflitos/erros de links e descartava configuração inválida | Preflight, recusa sobreposição, preservação das origens até salvar config e rollback das cópias/links; testes `TestMigrateLibrary_*` e `TestMigrationRejectsConflictAndInvalidConfig` |
| R05 | Alta | `hooks.Delete`: `Files` do JSON podia apontar fora da biblioteca | Exclusão de scripts restrita a filho direto da biblioteca; nomes carregados precisam corresponder ao arquivo; `TestDeleteRefusesExternalScripts` |
| R06 | Alta | Importação de hooks podia sobrescrever entrada existente e interpolava caminhos no código shell | Validação antes de copiar; recusa colisões e exportação da variável sem substituir texto dentro de aspas; `TestImportDoesNotOverwriteLibraryEntry`, `TestRewriteCommandPreservesShellQuoting` |
| R07 | Alta | `scripts/demo-home.sh`: `rm -rf` no argumento arbitrário | Recusa qualquer destino existente; demo usa pasta temporária única. Smoke test preservou arquivo sentinela |
| R08 | Alta | Provedor Codex duplicava `model_provider`; marcador sem fechamento consumia o resto da config | Preserva/restaura provedor anterior, valida delimitadores e recusa TOML multilinha que o editor não interpreta; regressões em `provider_test.go` |
| R09 | Média | Provedor Claude descartava valores não-string desconhecidos em `env`; token novo podia manter modo 0644 | Preserva os valores desconhecidos e restringe a escrita com token a 0600; `TestClaudeProviderKeepsUnknownEnvValues`, `TestClaudeTokenTightensPermissions` |
| R10 | Média | Cache, apelidos e perfis aceitavam JSON `null` e depois escreviam em mapa nil | Mapas inicializados; `TestNullCacheAndFutureTimestamp`, `TestNullAliases`, `TestNullProfiles` |
| R11 | Média | ZIP maior que 64 MiB era truncado sem erro; backups eram ordenados por nome de skill | Limite explícito com erro e ordenação por data; `TestZipRejectsOversizedEntry`, `TestBackupsSortedAcrossSkills` |
| R12 | Média | Exportação usava ID externo como caminho e transcript saía em 0644 | Recusa separadores/traversal, nomes com nanossegundos e modo 0600; `TestExportRefusesTraversal` e testes de exportação |
| R13 | Média | Metadados/erros de plugin podiam conter controles de terminal; exec não acompanhava encerramento do app | Sanitização, contexto de cancelamento e ambiente do plugin; `TestManifestStripsTerminalControls`, `TestExecStopsWhenServiceCloses` |
| R14 | Média | Prefixo de preço de Opus 4 aplicava tarifa de uma versão a outra; agregados misturavam modelos e projetos homônimos | Tarifas explícitas por versão, sem estimativa para modelos mistos/desconhecidos, projetos agrupados por caminho; `TestVersionSpecificPricing`, `TestAggregationSeparatesPathsAndMixedModels` |
| R15 | Média | Hermes era marcado como leitor automático de `~/.agents/skills`; ajuda oferecia `wire_api=chat` ao Codex | Hermes anuncia apenas seu diretório padrão; Codex recusa protocolo removido e ajuda usa `responses`; `TestCodexRejectsUnsupportedWireAPI` |
| R16 | Baixa | Preview descartava o comando que carrega as skills fictícias | Inicialização executa somente o scan de skills, preservando sessões fictícias; verificação via tmux mostrou 5 skills e 4 sessões |
| R17 | Baixa | GoReleaser residual, docs de arquitetura/suporte divergentes, pacotes sem avisos de licença | Workflow por shell é a única definição; docs alinhadas; licenças nos pacotes; CI/release incluem race e sintaxe dos scripts |

Também removido um bloco de imports vazio. Sem dependências novas no módulo,
sem mudança de versão do protocolo de plugins ou migração de formatos persistidos.
Hooks importados anteriormente mantêm seu comando gravado: a correção de quoting
vale para novas importações. Para substituí-los, desative/remova/reimporte a entrada.

## Cobertura

| Área | O que foi examinado |
| --- | --- |
| Boot, core, fsutil, composição | Paths, configuração/migração YAML, escrita atômica, backups, registro e encerramento |
| Skills | Descoberta local/git/ZIP/marketplace, instalação, ativação, adoção, perfis, hash/update, remoção, backup/restore e migração |
| Sessões e adapters | Listagem, parsing, resume, transcript, aliases, exportação, exclusão com backup e proteção de sessões ativas |
| Provedores e hooks | Bibliotecas, edição JSON/TOML, chaves alheias, tokens, consentimento, importação e scripts |
| Uso | Agregação, estimativa, cache, autenticação/limites sob demanda; rede autenticada real não foi exercitada |
| Plugins | Descoberta, protocolo, manifesto, processo, falhas, saída, CLI/doctor e exemplo shell |
| TUI | Roteamento, eventos, ajuda, paleta, estados vazios e layout; testes de layout de 40 a 120 colunas e preview em tmux |
| Distribuição/documentação | README, CLAUDE, backlog, plugins, temas/licenças, exemplos, demo, scripts, CI/release e dependências |

A cobertura combina leitura dirigida dos fluxos, buscas transversais, testes
existentes, regressões novas e smoke tests. Não equivale a certificação formal de
segurança nem a executar todas as combinações de agentes e sistemas operacionais.

## Referências de compatibilidade

Consulta em 22/09/2026. As páginas oficiais são móveis; os testes locais usam
fixtures e não estabelecem compatibilidade com toda versão futura dos CLIs.

- [Claude Code — hooks](https://code.claude.com/docs/en/hooks): configuração por evento e comandos; preservar a revisão de confiança do CLI.
- [Codex — configuração](https://learn.chatgpt.com/docs/config-file/config-reference): `model_providers`, `env_key` e `wire_api=responses`.
- [Codex — skills](https://learn.chatgpt.com/docs/build-skills): skills locais de usuário em `.agents/skills` e suporte a symlinks.
- [Plugins Claude no Codex](https://developers.openai.com/plugins/guides/submit-claude-plugin): hooks exigem adaptação e confiança; compartilhar nome de evento não prova equivalência de payload.
- [Gemini — skills](https://geminicli.com/docs/cli/using-agent-skills/): diretórios pessoais `.gemini/skills` e `.agents/skills`.
- [OpenCode — skills](https://opencode.ai/docs/skills/): descoberta e permissões de skills.
- [Hermes — skills](https://hermes-agent.nousresearch.com/docs/user-guide/features/skills/): diretórios externos exigem configuração explícita.
- [Claude API — preços](https://platform.claude.com/docs/en/about-claude/pricing): tarifas padrão por versão; a estimativa local usa escrita de cache de 5 minutos.

## Verificações e limites

Ambiente: Linux amd64, Go 1.27.1. O módulo continua declarando Go 1.26 e a CI usa
essa versão; esta rodada não altera automaticamente toolchain ou dependências.

- Baseline: `go test ./...` passou. A primeira tentativa no sandbox falhou por cache somente leitura e bloqueio do servidor HTTP de teste; a repetição autorizada fora dele passou.
- `gofmt -l .` sem arquivos, `git diff --check` limpo, testes, detector de corridas, `go vet`, build do projeto e build separado do preview passaram.
- Tmux: Garoa em 120×40, paleta em 64×24, Noite em 80×24 e Jaraguá em 100×40; conferidas skills, sessões, ajuda, agentes e estado vazio de hooks.
- `go mod verify`: módulos íntegros. `govulncheck` v1.8.0: nenhuma vulnerabilidade encontrada.
- CLI em home/XDG fictícios: versão, ajuda e JSON de skills, sessões, provedores e hooks válidos.
- Compilação de linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 e windows/amd64; o bloco exato de build/empacotamento do workflow de release foi executado em cópia temporária, sem publicação, e todos os SHA256SUMS conferiram. Somente o binário Linux amd64 foi executado.
- Links locais da documentação e sintaxe dos scripts conferidos.
- Nenhum dado real de agente foi alterado; não foram usadas credenciais para chamadas reais de limites.

Pendências não bloqueantes estão em M7 do backlog: testes nativos nas demais
plataformas, matriz de versões dos CLIs, variantes avançadas de configuração,
limites agregados de arquivos e processos e precisão adicional das estimativas.
