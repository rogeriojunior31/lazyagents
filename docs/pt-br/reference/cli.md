# Referência da CLI
<!-- source: ed969173637d -->

Cada comando imprime ajuda com `lazyagents help <comando>`. Sem argumentos, `lazyagents` abre a TUI. Plugins externos acrescentam comandos com seus IDs, descritos na ajuda após a instalação ([guia](../guide/plugins.md)). Esta página traduz a referência: comandos, opções, nomes de campos e colunas permanecem literais; a saída do programa continua em inglês.

Código de saída `0` indica sucesso; códigos diferentes de zero indicam falha.

| Comando | Função |
|---|---|
| [`list`](#list) | listar skills e quantos agentes as veem |
| [`enable`](#enable) | ativar skill da biblioteca por link simbólico |
| [`disable`](#disable) | desativar removendo o link |
| [`install`](#install) | instalar do GitHub, ZIP ou pasta |
| [`remove`](#remove) | remover da biblioteca |
| [`adopt`](#adopt) | incorporar skill local de agente |
| [`migrate-library`](#migrate-library) | mover biblioteca |
| [`skills`](#skills) | agrupar comandos de skills (`skills list` = `list`) |
| [`sessions`](#sessions) | listar sessões, recentes primeiro |
| [`provider`](#provider) | gerenciar perfis de endpoint e autenticação |
| [`hooks`](#hooks) | listar, instalar e desinstalar hooks |
| [`usage`](#usage) | limites de assinatura, tokens e custo por dia/agente/projeto/modelo |
| [`doctor`](#doctor) | diagnosticar agentes, skills, hooks, provedores, consumo e plugins |

## list

```text
lazyagents list [--json]
```

Lista skills da biblioteca e skills locais encontradas nos diretórios dos agentes, com a quantidade de agentes que as veem. `ENABLED IN` conta os agentes; `LIBRARY` indica gerenciamento pelo lazyagents.

| Opção | Efeito |
|---|---|
| `--json` | cada skill com origem, estado da validação e estado por agente |

## enable

```text
lazyagents enable <skill> [--agent id|--all]
```

Ativa uma skill da biblioteca criando link no diretório do agente. `<skill>` aceita nome da pasta ou do front matter. Agentes que já a veem ficam como estão. Skills locais precisam ser incorporadas antes.

| Opção | Efeito |
|---|---|
| `--agent id` | só esse agente (`claude-code`, `codex`, `gemini-cli`…) |
| `--all` | todos os instalados com diretório de skills; padrão |

## disable

```text
lazyagents disable <skill> [--agent id|--all]
```

Remove o link do agente, mantendo a skill na biblioteca e nos demais. Links únicos em `~/.agents/skills` são movidos aos diretórios próprios dos agentes que mantêm a ativação. Skills locais (pastas reais ou links de outra ferramenta) nunca são apagadas; o comando falha para elas.

| Opção | Efeito |
|---|---|
| `--agent id` | só esse agente |
| `--all` | todos onde o lazyagents ativou; padrão |

## install

```text
lazyagents install <source> [skill...] [--all] [--hooks]
```

Instala na biblioteca pastas com `SKILL.md` encontradas em qualquer profundidade. Marketplaces Claude Code (`.claude-plugin/marketplace.json`) são lidos pelo manifesto. Nomes após a origem restringem a instalação; nome desconhecido instala nada e lista as opções. Nomes já na biblioteca são ignorados com aviso. Sem `--all`, não ativa: use `enable`. Sem `--hooks`, apenas conta hooks, pois executam comandos de terceiros a cada evento.

| Origem | Significado |
|---|---|
| `owner/repo` | repositório GitHub, exige `git` no `PATH` |
| `https://…`, `git@…`, `*.git` | URL Git, clonada com `--depth 1` |
| `./skills.zip` | arquivo ZIP |
| `~/some/folder` | pasta local |

| Opção | Efeito |
|---|---|
| `--all` | ativar skills instaladas em todos os agentes com diretório, como `enable --all` |
| `--hooks` | instalar também hooks de plugins em `hooks/hooks.json` |

## remove

```text
lazyagents remove <skill>
```

Remove da biblioteca e apaga links criados pelo lazyagents que apontam para ela. Antes, faz backup `.tar.gz`; `b` em Skills restaura. Skills locais não estão na biblioteca e não podem ser removidas aqui.

## adopt

```text
lazyagents adopt <skill> --agent <id>
```

Copia a pasta local à biblioteca, faz backup da original e a substitui por link. O agente continua vendo a skill e outros podem ativá-la. Links de outras ferramentas são recusados; gerencie-os na ferramenta original.

| Opção | Efeito |
|---|---|
| `--agent id` | agente com a cópia local; obrigatória |

## migrate-library

```text
lazyagents migrate-library <dir>
```

Copia todas as skills para `<dir>`, redireciona links dos agentes, salva `libraryDir` em `config.yaml` e só depois remove cópias antigas, com backup prévio de cada skill. Recusa nomes já existentes no destino ou diretórios contidos um no outro. Em falha, links e configuração permanecem como antes.

Exemplo: `lazyagents migrate-library ~/.agents/skills`.

## skills

```text
lazyagents skills list|enable|disable|install|remove|adopt|migrate-library …
```

Agrupa os comandos acima. `lazyagents skills <sub>` equivale a `lazyagents <sub>`, com as mesmas opções. Sem subcomando, executa `list`. A organização acompanha `hooks` e `provider`.

## sessions

```text
lazyagents sessions [--agent id] [--here] [--limit n] [--json]
```

Lista sessões, recentes primeiro. Saída textual usa `AGENT`, `TITLE`, `CWD` e `UPDATED`; aliases aparecem como `alias (title)`. JSON acrescenta ID e, quando registrado, consumo (`input`, `output`, `cache_read`, `cache_write`, `model`). Para chave de API, inclui `cost_usd` em modelos conhecidos; assinaturas não pagam por token. Para retomar, abra a TUI.

| Opção | Efeito |
|---|---|
| `--agent id` | só esse agente; ID desconhecido lista os válidos |
| `--here` | sessões iniciadas no diretório atual |
| `--limit n` | no máximo n; padrão 0, todas |
| `--json` | saída JSON |

## provider

```text
lazyagents provider list|apply <profile>|clear|add <profile>|rm <profile> [--agent id] [--json] [--reveal]
```

Gerencia perfis de endpoint, modelo e autenticação.

| Subcomando | Efeito |
|---|---|
| `list` | perfis e configuração aplicada por agente; padrão |
| `apply <profile>` | escrever perfil na configuração dos agentes |
| `clear` | remover o aplicado, preservando o restante |
| `add <profile>` | criar ou substituir perfil |
| `rm <profile>` | apagar da biblioteca; agentes mantêm o aplicado |

| Opção | Efeito |
|---|---|
| `--agent id` | limitar aplicação/limpeza; padrão é todos os instalados compatíveis |
| `--json` | listagem JSON |
| `--reveal` | mostrar tokens em texto puro na listagem |

Opções de `add`:

| Opção | Efeito |
|---|---|
| `--base-url url` | endpoint compatível HTTP/HTTPS |
| `--model m` | modelo; vazio usa padrão do agente |
| `--token -` | ler da entrada padrão; valor literal funciona, mas fica no histórico do shell |
| `--env-key VAR` | variável com token para Codex, Pi e Crush |
| `--wire-api api` | Codex: só `responses`; Pi: `chat` padrão, `responses` ou `anthropic`; Crush: `chat` padrão ou `anthropic` |

Perfis ficam em `providers.json`, modo 0600. Tokens são mascarados em texto/JSON salvo `--reveal`. Aplicar/limpar faz backup do arquivo do agente. Codex nunca recebe token no arquivo: defina `--env-key` e exporte a variável. Pi/Crush exigem `--base-url` e `--model`.

## hooks

```text
lazyagents hooks list|enable <name>|disable <name>|add <name>|rm <name> [--agent id] [--json]
```

Lista hooks da biblioteca e instala/desinstala nos agentes.

| Subcomando | Efeito |
|---|---|
| `list` | biblioteca e locais de instalação; padrão |
| `enable <name>` | instalar em agentes com os eventos necessários |
| `disable <name>` | desinstalar |
| `add <name>` | criar entrada com um comando |
| `rm <name>` | apagar da biblioteca; instalações existentes permanecem |

| Opção | Efeito |
|---|---|
| `--agent id` | limitar ativação/desativação; padrão todos os instalados compatíveis |
| `--json` | listagem JSON |

Opções de `add`:

| Opção | Efeito |
|---|---|
| `--event name` | evento, como `SessionStart`, `PreToolUse`, `Stop` |
| `--command text` | comando shell |
| `--matcher re` | filtro de evento, como ferramenta em `PreToolUse`; vazio aceita todos |
| `--timeout n` | segundos; 0 usa padrão do agente |
| `--desc text` | descrição na aba |

Hooks executam a cada evento; instalar faz backup primeiro, e hooks externos não são tocados. Codex só executa novo hook após confiança aprovada no próprio Codex; o lazyagents não a escreve. Importe hooks com `lazyagents install <source> --hooks`.

## usage

```text
lazyagents usage [limits|daily|agents|projects|models] [--agent id] [--since 7d] [--limit n] [--refresh] [--json]
```

Mostra limites, tokens e custo por período/agente/projeto/modelo.

| Visualização | Conteúdo |
|---|---|
| sem subcomando | painel de limites, bloco atual de 5 horas e resumo do período |
| `limits` | janelas de sessão/semana com consumo e renovação |
| `daily` | tokens por dia |
| `agents` | tokens por agente |
| `projects` | tokens por projeto, usando pasta da sessão |
| `models` | tokens por modelo |

| Opção | Efeito |
|---|---|
| `--agent id` | só esse agente |
| `--since period` | início do período: `7d`, `24h` ou `2026-09-01`; padrão 7d |
| `--limit n` | linhas de agentes/projetos/modelos no texto; padrão 10, 0 todas |
| `--refresh` | ignorar cache de 5 minutos e consultar limites novamente |
| `--json` | JSON, não limitado por `--limit` |

Tokens vêm das conversas locais; limites vêm das contas dos agentes. Custos em USD só aparecem para chaves de API; assinaturas não pagam por token.

## doctor

```text
lazyagents doctor [--json]
```

Diagnostica agentes, skills, hooks, provedores, consumo e plugins.

| Seção na saída | Verificação |
|---|---|
| detected agents | ID e instalação de cada agente |
| skills | SKILL.md inválido/incompleto na biblioteca e agentes |
| symlinks | links quebrados nos diretórios lidos pelos agentes |
| providers | provedor aplicado; configuração em agente não instalado é problema |
| hooks | arquivos inválidos, executáveis ausentes/sem permissão e avisos por agente |
| usage | autenticação e limites; usa cache recente ou consulta novamente |
| plugins | arquivos ignorados, negociação inicial e diagnóstico próprio se declarado |

`--json` retorna `{ok, agents, sections[{title, report, problems}]}`. Sai com `1` se alguma seção reportar problema, `0` se tudo estiver correto.
