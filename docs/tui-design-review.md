# Validação de design da TUI — 24/09/2026

A base visual é consistente: superfícies sem excesso de bordas, destaque de seleção, cores por agente e componentes compartilhados. O principal problema é a perda de informação em telas pequenas. Recomendo corrigir acesso ao conteúdo antes de mudar paleta ou adicionar decoração.

## Método e limites

- Revisão das seis abas embutidas, seus modos secundários e do contêiner de plugins.
- Execução do preview isolado em tmux: 100×32, 80×24, 64×24 e 40×16; navegação nas abas, ajuda e formulário de provedor. Capturas de texto em `/tmp/lazyagents-design-audit/` (temporárias).
- Em 80×24: leitura de SKILL.md, prompts de instalação/criação/busca, perfis vazios, ausência de backups, confirmação de remoção; em Sessões, apelido, pasta, busca, erro de transcript e confirmação de exclusão. Confirmações canceladas.
- `go test ./...` e `go vet ./...` passaram. O preview compilou. O teste `TestResponsiveLayout` cobre também 120×40, mas testa principalmente dimensões: recortar conteúdo pode satisfazer o teste e ainda prejudicar o uso.
- Preview com dados fictícios: não há transcript em disco, histórico de backups, hooks/perfis preenchidos ou consumo representativo. Esses estados foram analisados por código e testes, não aprovados visualmente com dados reais. Nenhuma consulta de conta foi necessária.
- Capturas de terminal permitem avaliar texto, dimensões e navegação; não constituem avaliação visual dos três temas em diferentes monitores. Não houve medição de contraste nem teste de leitor de tela.
- A análise abaixo registra o estado inicial. Os lotes implementados estão descritos ao final.

## Achados prioritários

| Prioridade | Evidência | Melhoria proposta | Critério de aceite |
|---|---|---|---|
| P1 | Em 80×24, a ajuda de Skills termina no título “Ativação”; os atalhos abaixo desaparecem. `renderHelp` não tem rolagem e o root recorta a altura. | Colocar o conteúdo da ajuda em viewport, mantendo título e instrução de fechamento visíveis. | Todos os atalhos acessíveis por teclado em 40×16 e 80×24; posição de rolagem indicada. |
| P1 | Em 64×24, Agentes mostra “SKILLS” e depois “…”; diretórios e capacidades ficam inacessíveis. Provedores usa o mesmo corte sem rolagem. | Reutilizar o padrão lista/detalhe com foco e viewport já usado em Skills/Sessões. | Caminhos, avisos e estados de todos os agentes podem ser lidos sem aumentar o terminal. |
| P1 | Em 40×16, o formulário de provedor perde os últimos campos e o rodapé. Inputs têm largura fixa de 44 e a view não recebe altura. | Dimensionar inputs pela largura útil; em telas estreitas, rótulo acima do campo; rolar acompanhando o foco. | O campo focado, seu erro e a ação de salvar permanecem visíveis até o último campo. |
| P1 | `Confirm.ViewIn` quebra a pergunta, mas não limita nem permite rolar o conteúdo. Perguntas extensas podem deslocar os botões para fora do corpo. Evidência por código. | Reservar rodapé fixo para a decisão e rolar apenas a descrição da alteração. | Confirmações com múltiplos agentes e caminhos longos preservam pergunta completa acessível e Sim/Não visíveis. |
| P1 | Backups usa janela fixa de 16 itens, independentemente da altura. Além disso, chama `Truncate(path, maxW-34)`, que pode receber valor não positivo e causar panic em largura muito pequena. Evidência por código. | Dimensionar a janela pela altura disponível e tornar truncamento seguro para largura zero/negativa. | Vinte backups navegáveis em 40×16; estreitar abaixo de 39 colunas externas não causa panic. |
| P2 | Em 40×16, a barra mostra “Skill Sessõ Prove … Agent”. A largura é dividida igualmente, mesmo entre títulos curtos e longos. | Distribuir pela largura do título; quando não couber, mostrar uma janela de abas com a ativa por inteiro e indicadores laterais. | Nome da aba ativa legível; clique e Tab seguem a mesma ordem. |
| P2 | `Hints` interrompe a lista ao faltar espaço. `?` geralmente é o último item; em 40×16 o cabeçalho também esconde os atalhos globais. | Reservar espaço primeiro para `?` e, nas telas secundárias, `esc`; depois incluir ações contextuais. | Ajuda ou saída da subtela sempre descobrível. |
| P2 | Em Hooks vazio, o comando de criação e a explicação de importação são cortados em 80×24; o rodapé prioriza ações sem item. | Mostrar instrução curta, quebrar texto explicativo e priorizar a ação que resolve o vazio. | Usuário consegue descobrir como adicionar o primeiro hook sem interpretar um comando incompleto. |
| P2 | Sessões confirma exclusão no rodapé com Enter; os demais módulos usam o diálogo com “Não” selecionado. | Usar o componente de confirmação compartilhado, com quantidade e destino do backup. | Mesmo comportamento de cancelamento e confirmação entre módulos. |
| P2 | Inputs simples de Skills têm largura fixa de 60; seletores truncam descrições por tamanho fixo, não pela sobra real. | Redimensionar inputs e linhas pela largura do painel. | Cursor e trecho em edição visíveis; nomes longos preservam acesso ao detalhe. |

Referências: `internal/tui/app.go` (`renderPill`, `renderHelp`, `View`), `internal/tui/kit/hints.go`, `internal/tui/kit/layout.go` (`Truncate`), `internal/tui/components/confirm.go`, `internal/modules/agents/tab.go` (`detailPanel`), `internal/modules/providers/view.go`, `internal/modules/providers/form_ui.go`, `internal/modules/skills/backups.go` e `internal/modules/sessions/view.go`.

## Revisão por página e subtela

| Página / fluxo | Validação e melhoria |
|---|---|
| Splash | Dimensões cobertas pelo teste de layout. Preservar a opção existente de pular; abertura rápida é mais útil que acrescentar animação. |
| Navegação global | Abas e ajuda exercitadas nas quatro dimensões. Corrigir títulos cortados e garantir descoberta de Tab, paleta e ajuda. |
| Paleta | Usada para navegar entre todas as abas. Código descarta `PasteMsg` enquanto está aberta: permitir colar comandos/filtros. Dimensionar também pela altura e manter seleção visível. |
| Skills — biblioteca e detalhe | Lista/detalhe funcionam no preview. O comportamento de foco é uma boa referência para os outros módulos. Acrescentar indicação de conteúdo abaixo no detalhe e legenda curta dos estados por agente. |
| Skills — leitura | SKILL.md aberto no preview. Cabeçalho mistura título e muitos atalhos numa linha; separar título dos controles responsivos e indicar posição da leitura. |
| Skills — instalar / nova / buscar no GitHub | Prompts abertos e cancelados. Usar contexto “Skills › Instalar”, largura de input adaptável e exemplos curtos. Manter mensagens de progresso e erro junto ao campo. |
| Skills — seleção de instalação / resultados GitHub | Código e testes de descoberta revisados; resultados não exercitados via rede. Exibir quantidade marcada, seleção forte na linha inteira e posição “item X de Y”. Reservar espaço para rodapé e avisos de marketplace. |
| Skills — perfis / salvar perfil | Estado vazio observado; código de salvar/aplicar revisado. No vazio, destacar `s salvar estado atual` em vez de `enter aplica`. Ao aplicar, resumo legível de ativações/desativações antes da confirmação. |
| Skills — backups / restaurar | Ausência de backups observada; lista preenchida revisada por código. Priorizar data e nome curto, mantendo caminho completo acessível; corrigir altura fixa e limite negativo de truncamento. |
| Skills — atualização / adoção / remoção | Confirmação de remoção observada; demais transições revisadas por código. Reutilizar diálogo com conteúdo rolável; apresentar resultado com quantidade alterada e erros que possam ser consultados após o toast. |
| Sessões — lista / detalhe / agrupamento / filtros / seleção | Lista e detalhe observados; código dos outros estados revisado. Em 80×24 a coluna Contexto corta metadados como a data. Quebrar campos por largura e dar mais espaço ao detalhe. Separar filtros ativos dos atalhos para que agrupamento não substitua ações essenciais. |
| Sessões — transcript / comandos / raciocínio / exportação | Preview falha ao abrir porque a sessão fictícia não tem arquivo; renderização com conteúdo está coberta por testes. O leitor já oferece posição e controles de expansão: preservar. Exibir estado aberto/fechado dos blocos e manter `esc` acessível em largura pequena. Exportação coberta por testes, não pelo ensaio manual. |
| Sessões — apelido / retomar em pasta / busca full-text | Prompts abertos e cancelados. Padronizar alinhamento e largura; usar a mesma convenção de título e retorno dos prompts de Skills. |
| Sessões — exclusão individual / lote | Confirmação observada e cancelada. Migrar a decisão do rodapé para diálogo compartilhado; manter visível a quantidade selecionada. |
| Provedores — lista / detalhe / vazio | Estado sem perfis observado; aplicar/limpar coberto por testes. Detalhe precisa de rolagem. Mostrar agente e estado antes dos caminhos técnicos, com legenda para os marcadores da lista. |
| Provedores — criar / editar | Formulário de criação observado; criação/edição testadas pela suíte. Separar dados principais de opções específicas do Codex; explicar campos junto ao foco e corrigir responsividade. Preservar mascaramento do token. |
| Provedores — aplicar / limpar / excluir | Fluxos revisados por código; aplicar e cancelar cobertos por testes. Organizar a confirmação em perfil, agentes e arquivos afetados, seguida dos controles fixos. |
| Hooks — biblioteca / detalhe / vazio | Estado vazio observado; conteúdo preenchido coberto parcialmente pelos testes. Direcionar para instalação em Skills e não gastar os primeiros atalhos com ações sem seleção. |
| Hooks — escolher comandos / ativar / remover / excluir | Modo de comandos e aplicação cobertos por testes, não pelo preview preenchido. Destacar explicitamente “selecionando comandos”, usar marcação textual e contador de ativos, preservando avisos e comando completo acessível. |
| Uso — limites / resumo / bloco atual | No preview, estado sem dados. Código de barras, cartões, resumo e rolagem revisado. Em telas pequenas, separar rótulo, barra e percentual; mostrar frescor do dado e erro sem truncar a explicação necessária. |
| Uso — dia / agente / projeto / modelo | Quatro visões e filtros analisados; testes cobrem agregação, mudanças de visão, período, agente e texto. Manter filtros visíveis durante rolagem e título explícito da visão. Separar visualmente limites de assinatura e consumo do período. |
| Uso — filtro / vazio / carregamento / erro | Estados revisados por código e testes. Distinguir ausência de consumo, filtro sem resultado e falha de atualização; oferecer ação correspondente e preservar último dado com indicação de cache. |
| Agentes — instalados / ausentes / capacidades / diretórios | Lista e ambos os tipos de agente disponíveis no preview. Evitar corte definitivo no detalhe. Agrupar versão/estado, contagens, capacidades e diretórios em ordem de importância. |
| Plugins — carregamento / conteúdo / falha | Contêiner e testes revisados; nenhum plugin externo foi exercitado manualmente. Padronizar falha e ação `r tentar novamente`; conteúdo de terceiros exige validação própria. |

## Direção visual proposta

1. **Uma navegação previsível:** lista e detalhe lado a lado quando couber; em largura pequena, uma área por vez com `←/→`. Usar esse comportamento em Skills, Sessões, Provedores, Hooks e Agentes, preservando o modo específico de seleção de comandos dos Hooks.
2. **Hierarquia constante:** nome/estado primeiro, informação útil em seguida, caminhos técnicos por último. Título de subtela com contexto; filtros em faixa própria; ações essenciais no rodapé.
3. **Rolagem explícita:** indicar posição ou “mais abaixo”; nunca usar apenas “…” quando não existe como alcançar o restante.
4. **Menos ruído:** três a cinco ações principais por tela, com ajuda sempre visível. Mostrar ações apropriadas ao estado vazio. Reutilizar `Panel`, `Hints` e viewports existentes.
5. **Cor com redundância:** manter a identidade dos temas, mas acompanhar cor com texto/símbolo de foco, erro e estado. Medir contraste dos três temas numa etapa visual específica antes de alterar tokens.

## Ordem de execução sugerida

- Primeiro: ajuda, confirmações, backups e formulário de provedor acessíveis em baixa altura/largura.
- Depois: rolagem de detalhes, navegação compacta, ações essenciais e estados vazios.
- Por último: refinamento de hierarquia, filtros persistentes em Uso e harmonização dos prompts.

Para validar a implementação, usar dados sintéticos com nomes/caminhos longos, 20 backups, vários agentes, hooks com múltiplos comandos, transcript extenso e tabelas de Uso. Verificar conteúdo alcançável, foco, teclado, mouse e redimensionamento; conferir só a dimensão final do frame é insuficiente.

## Primeiro lote implementado

- Ajuda rolável por teclado e mouse, com percentual e fechamento fixos; reabrir volta ao topo e redimensionar mantém a posição válida.
- Backups dimensionados pela altura, com posição no título; truncamento seguro com largura zero ou negativa.
- Formulário de provedor com inputs dimensionados pelo painel, janela de campos acompanhando o foco e ações fixas. O fim do texto continua visível e o token permanece mascarado.
- Testes de regressão para acesso ao último atalho, mouse, resize, 20 backups em telas pequenas e todos os campos com texto longo.
- Checks locais equivalentes à CI: gofmt, vet, testes com race detector, scripts, build, preview e compilação para Linux amd64/arm64, macOS amd64/arm64 e Windows amd64.
- Ensaio no tmux em 40×16: fim da ajuda e último campo do provedor acessíveis. A CI remota ainda depende de publicação do commit.

## Segundo lote implementado

- Confirmações com pergunta rolável por teclado/mouse, posição no título e controles fixos. O padrão continua sendo “Não”; rolar não autoriza a ação.
- Agentes e Provedores com foco por `←/→`, detalhe rolável, quebra de linhas e percentual. Em largura pequena, apenas o painel focado é exibido; em largura maior, continuam lado a lado.
- Teclas de navegação e mouse preservam a seleção ao rolar o detalhe. Mudar de item reinicia a posição. Cliques/roda em confirmação não alteram itens atrás do diálogo.
- Testes de perguntas extensas, decisão padrão, mouse, redimensionamento, caminhos longos e acesso ao fim dos detalhes. Testes com race detector e demais checks locais da CI passaram, incluindo os cinco alvos de compilação.
- Tmux em 40×16: capacidades de Agentes e arquivos de configuração de Provedores acessíveis até o final; confirmação cancelada sem alteração de configurações.

## Terceiro lote implementado

- Barra de abas dimensionada pelos títulos; remove contadores quando necessário e mostra uma janela em torno da aba ativa, com setas clicáveis para as abas ocultas. Renderização e cliques usam o mesmo layout.
- Cabeçalho compacto mantém `tab abas` e `? ajuda` em 40 colunas. Rodapés reservam ajuda/retorno antes das ações secundárias, sem duplicar esses atalhos.
- Hooks vazio usa a largura disponível para orientar a importação pela aba Skills, sem sugerir ativação/remoção sem seleção. Perfis de skills vazio destaca salvar o estado atual e voltar.
- Testes de navegação em 40/64/80/120/200 colunas, Unicode, plugins com títulos longos, clique nas abas/setas, Tab/Shift+Tab, ausência de abas e reserva dos atalhos essenciais.
- Ensaio em tmux 40×16 confirmou títulos legíveis e instruções completas nos estados vazios. Todos os checks locais equivalentes à CI passaram, incluindo race detector e os cinco alvos de compilação; CI remota ainda não executada.

## Quarto lote implementado

- Paleta dimensionada pela altura, seleção sempre visível, contador de posição, instruções fixas e suporte a texto colado. O cursor permanece acessível em consultas longas.
- Exclusão de sessões usa a confirmação compartilhada, com “Não” selecionado inicialmente. Mostra quantidade, backup e sessões afetadas em conteúdo rolável. Os alvos ficam fixados ao abrir: uma recarga posterior não altera o conjunto confirmado. Mouse não muda a seleção por trás do diálogo.
- Filtros de Uso permanecem no topo ao rolar, com layout compacto em duas linhas. A barra é memorizada junto ao corpo para não recalcular agregações na rolagem. O input aceita colar e mantém instruções em linha separada.
- Sessões separa o resumo de filtros/agrupamento dos atalhos; ativar filtros não substitui nem oculta ajuda e ações de navegação.
- Testes verificam paleta com 30 comandos em telas pequenas, paste isolado, cursor longo, exclusão cancelada por padrão, alvos preservados após recarga, filtros fixos e preservação do cache ao rolar.
- Tmux 40×16: último comando e fechamento da paleta visíveis; Enter cancelou a exclusão; filtros e input de Uso mantiveram instruções acessíveis. Consumo preenchido continua validado por dados sintéticos em testes.
- Todos os checks locais equivalentes à CI passaram, incluindo race detector e os cinco alvos de compilação. CI remota ainda não executada.

## Quinto lote implementado

- Seletores de instalação e resultados do GitHub dimensionados pela altura real, com posição/total no título e controles visíveis. A instalação mostra quantos itens estão marcados; hooks continuam desmarcados inicialmente.
- Avisos da descoberta em área rolável com PgUp/PgDn e percentual, sem deslocar os controles para fora da tela.
- Cliques nos seletores de instalação, GitHub, perfis e backups consideram a janela visível. Cliques no rodapé não selecionam entradas fora da lista.
- Inputs simples de Skills e Sessões dimensionados pela largura útil, com cursor preservado em textos longos. Prompts quebram linhas e os controles de confirmar/voltar usam espaço reservado. A instrução de remover apelido vazio ganhou linha própria.
- Testes com 30 entradas, hooks desmarcados, descrições/avisos longos, clique após rolagem, rodapé e cursor no fim de caminhos longos.
- Tmux 40×16: descoberta de pasta fictícia com 30 skills, navegação até a última entrada, contador e controles visíveis. Operação cancelada antes de instalar. Resultados GitHub e avisos extensos validados por dados sintéticos, sem consulta de rede.
- Todos os checks locais equivalentes à CI passaram, incluindo race detector e os cinco alvos de compilação. CI remota ainda não executada.

## Sexto lote implementado

- Uso mostra erros completos em cards roláveis, com ação de tentar novamente. Falhas continuam visíveis quando há limites anteriores, acompanhadas da indicação de dados preservados.
- Barras em telas estreitas separam rótulo, percentual e renovação; rótulos extensos permanecem completos. Resumo e instruções de estados vazios quebram linhas.
- Carregamento usa indicação neutra e aguarda ambas as consultas, de limites e consumo. O rodapé resume os avisos e direciona aos cards; uma resposta bem-sucedida limpa o aviso anterior.
- Testes verificam erros longos, limites em cache, acesso ao fim das mensagens em três larguras e conclusão das consultas nas duas ordens possíveis.
- Preview corrigido para carregar a aba inicial escolhida e documentar a ordem atual das páginas. Tmux em 40×16 confirmou leitura do erro até o fim e orientação para atualizar. Capturas dos temas Noite, Garoa e Jaraguá em 720×480 verificaram o layout do estado de erro de Uso; dados preenchidos continuam cobertos por testes sintéticos.
- Contraste calculado dos tokens: texto/superfície entre 9,25:1 e 12,38:1; texto secundário/superfície entre 4,71:1 e 5,08:1; texto/seleção entre 7,84:1 e 10,84:1. Nenhuma alteração de paleta nesta etapa. A medição não equivale à validação de todos os estados em cada terminal.
- Todos os checks locais equivalentes à CI passaram, incluindo gofmt, vet, testes com race detector, scripts, build, preview e os cinco alvos de compilação. CI remota ainda não executada.

Permanecem para os próximos lotes os refinamentos restantes do diagnóstico inicial, incluindo estados de erro/progresso dos demais módulos e validação visual dos fluxos preenchidos nos três temas.

## Sétimo lote implementado

- Hooks usa toda a área útil para escolher comandos em telas estreitas; o título mostra posição/total e Esc restaura a biblioteca.
- Limite de rolagem calculado pela altura real do detalhe, permitindo alcançar a última linha também no layout empilhado.
- Roda do mouse respeita o painel: no detalhe rola o conteúdo; durante a seleção navega comandos sem trocar o hook. Redimensionamento mantém o comando selecionado visível.
- Preview inclui um pacote fictício com três comandos, um desligado, para ensaiar o fluxo sem instalar ou executar hooks.
- Teste cobre fim do detalhe, mouse, seleção, resize e retorno à biblioteca em 36 e 100 colunas. Ensaio tmux 40×16 confirmou acesso ao terceiro comando e contador 3/3.
- Todos os checks locais da CI passaram: gofmt, vet, testes com race detector, scripts, build, preview e compilação dos cinco alvos. CI remota ainda não executada.

## Oitavo lote implementado

- Comandos longos de Hooks deixam de ser abreviados no detalhe: conteúdo completo, matcher, estado e modificadores passam para linhas próprias quando não cabem. Comandos curtos mantêm o formato compacto.
- PgUp/PgDn e Ctrl+U/D funcionam também durante a seleção, sem mudar o comando marcado ou executar ações. O passo da página considera a linha reservada ao indicador para não saltar conteúdo.
- Navegar entre comandos posiciona o início do comando no painel; ajuda atualizada e preview com comando longo, async e timeout.
- Teste em 36/100 colunas verifica acesso ao começo e fim de um comando maior que a tela, matcher completo, estado desligado, modificadores, retorno por PgUp e preservação da seleção.
- Tmux 40×16 confirmou a leitura do comando fictício até FIM_COMANDO e async/120s. Nenhum hook instalado ou executado no ensaio.
- Todos os checks locais equivalentes à CI passaram, incluindo vet, race detector, build do preview e cinco alvos de compilação. CI remota ainda não executada.

## Nono lote — revisão após feedback visual

- A expansão de cada comando de Hooks em vários blocos foi rejeitada pelo usuário por dificultar a leitura. A revisão substitui esse formato por uma lista compacta com caixas de seleção; o texto integral do comando escolhido fica em uma área separada abaixo.
- A lista permanece visível durante a leitura por páginas. Setas mudam o comando e reiniciam sua leitura; espaço alterna a marcação, preservando a confirmação quando já instalado em agentes.
- Resumo da biblioteca volta a ser compacto; Enter abre a seleção/leitura completa. Metadados comuns foram reduzidos e os controles do rodapé encurtados.
- Capturas reais com VHS e ensaio em tmux usados para revisar a organização, além dos testes de acesso ao texto. Aparência ainda sujeita à avaliação do usuário; os testes de dimensões não substituem essa avaliação.
- Plugins: erro e stderr em área rolável com reinício fixo no rodapé; contagem antiga limpa ao encerrar. Teste com processo fictício cobre falha, leitura até o fim, sanitização e reinício. Tmux 40×16 confirmou o fim do diagnóstico e o atalho de reiniciar visíveis.
- Checks locais equivalentes à CI passaram: gofmt, vet, race detector, scripts, build, preview e cinco alvos de compilação. CI remota não executada.

## Décimo lote — leitura e edição solicitadas pelo usuário

- `v` abre comando/scripts em tela cheia, com linhas numeradas, barra vertical ocupando a altura da leitura e percentual sempre no começo do título. `←/→` alterna documentos; teclado, páginas e mouse rolam o texto.
- Scripts são identificados por caminhos literais com extensões de script conhecidas, incluindo caminhos entre aspas e a raiz de plugins importados. Comandos dinâmicos que dependem de avaliação do shell não são resolvidos nem executados. Arquivos binários e maiores que 1 MiB geram aviso.
- `e` abre uma cópia temporária no `$EDITOR` (fallback vi). Ao retornar, a confirmação mostra antes/depois e mantém “Não” como padrão. Cancelamento e falha do editor preservam o original.
- Salvamento de scripts verifica mudanças externas, cria backup e preserva permissões. Salvamento do comando atualiza a biblioteca e apenas os agentes onde o comando anterior estava instalado; tenta restaurar os estados anteriores em caso de falha, preservando comandos novos que já existiam.
- Testes cobrem leitura de script com espaços no caminho, paginação em 36×11, cancelamento padrão, salvar/atualizar leitor, permissões, backups, edição obsoleta, comando instalado, comando vazio e rollback após falha de escrita. Inspeção não executa substituições do shell.
- Ensaio tmux 40×16 percorreu leitura → editor fictício → confirmação → salvamento → conteúdo atualizado com backup, usando exclusivamente dados descartáveis. Capturas VHS em duas larguras foram inspecionadas visualmente.
- Todos os checks locais equivalentes à CI passaram após a ampliação: gofmt, vet, race detector, scripts, build, preview e cinco alvos de compilação. CI remota não executada.
