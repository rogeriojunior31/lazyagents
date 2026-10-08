# Segurança e dados
<!-- source: 709908bbb022 -->

O lazyagents edita arquivos dos quais seus agentes dependem. Esta página informa exatamente o que muda, como desfazer e como segredos são tratados. Os guias de módulos detalham cada assunto.

<a id="principles"></a>

## Princípios

- **Nada muda sem uma ação sua.** Navegar, abrir a TUI ou consultar informações nunca escreve arquivos dos agentes.
- **Configurações são confirmadas.** Na TUI, aplicar provedor ou instalar hook mostra arquivo e mudanças antes de escrever. Skills só adicionam/removem links, por isso a ativação é imediata.
- **Backup antes de sobrescrever.** Configurações são copiadas antes de cada escrita; skills removidas e sessões excluídas são preservadas antes da remoção.
- **Só conteúdo gerenciado é alterado.** Chaves, hooks e seções TOML não escritos pelo lazyagents ficam intactos e na mesma ordem. No `env` do Claude Code, aplicação/limpeza só remove chaves próprias e restaura endpoint/modelo substituídos.
- **Escrita atômica.** Usa arquivo temporário e renomeação sobre o original, evitando arquivo parcial após falha.
- **Escritas concorrentes não são sobrescritas.** A configuração só é substituída se ainda corresponder à leitura. Se outro processo mudar, Codex (`config.toml`) e Pi (`models.json`, `settings.json`) são relidos e editados novamente, até três tentativas com um backup. Claude Code e hooks são deixados como o outro programa escreveu, com aviso. Remover arquivos criados pelo lazyagents segue a mesma regra.

<a id="what-lazyagents-writes"></a>

## O que o lazyagents escreve

Nos próprios diretórios ([caminhos](configuration.md#where-files-live)): bibliotecas de skills/hooks, perfis, aliases, caches, exportações e backups.

Nos diretórios dos agentes, apenas por sua ação:

| Ação | Mudança | Como desfazer |
|---|---|---|
| Ativar skill | link simbólico no diretório do agente para a biblioteca | desativar remove o link |
| Desativar skill | remove o link, nunca uma pasta real local ou de outra ferramenta | ativar novamente |
| Incorporar skill | pasta local vai à biblioteca; link toma seu lugar | copiar a pasta da biblioteca de volta |
| Aplicar provedor | Claude Code: `env` em `~/.claude/settings.json`. Codex: `model_provider` e bloco `[model_providers.lazyagents]` em `~/.codex/config.toml`. Pi: provedor `lazyagents` em `~/.pi/agent/models.json` e `defaultProvider`/`defaultModel` em `settings.json`. Crush: bloco delimitado ao fim de `~/.config/crush/crushrc`, com valores entre aspas simples | limpar o provedor ou restaurar backup |
| Instalar hook | entrada em `~/.claude/settings.json` ou `~/.codex/hooks.json`, ou linha `hook add` no bloco do `crushrc` | desinstalar ou restaurar backup |
| Excluir sessão | arquivo removido após cópia ao backup. OpenCode exporta com `opencode export` antes de `opencode session delete`; Crush salva com `crush session show --json` antes de `crush session delete`. Falha no backup impede exclusão | copiar backup de volta; OpenCode usa `opencode import <file>`; Crush não tem importação, backup é só registro |
| `migrate-library` | move skills, recria links e salva `libraryDir` em `config.yaml` | executar de volta ao diretório antigo |

Arquivos por agente estão na [referência](reference/agents.md#capabilities).

## Backups

Tudo fica em `~/.local/share/lazyagents/backups/`, como `<file>.<timestamp>`:

| Conteúdo | Retenção |
|---|---|
| configurações (`settings.json`, `config.toml`, `hooks.json`, `models.json`) | 20 mais recentes por arquivo |
| skills removidas/atualizadas (`<skill>.<timestamp>.tar.gz`) | 20 mais recentes por skill |
| sessões excluídas (`<transcript file>.<timestamp>`, `opencode-<id>.<timestamp>.json`, `crush-<id>.<timestamp>.json`) | todas |

Skills são restauradas pela TUI: `b` na aba Skills ([guia](guide/skills.md)). Backups de configuração são cópias comuns: para restaurar, copie sobre o arquivo atual.

```sh
ls -t ~/.local/share/lazyagents/backups/settings.json.* | head -1   # backup mais recente
```

<a id="secrets"></a>

## Segredos

- **Tokens de provedores:** ficam em `~/.config/lazyagents/providers.json`, modo 0600, preservado nos backups. TUI e `--json` só indicam presença; texto puro exige `lazyagents provider list --reveal`. Use `--token -` para ler da entrada padrão, fora do histórico do shell.
- **Codex:** prefira `--env-key VAR`; `config.toml` guarda apenas o nome da variável.
- **Claude Code:** token precisa estar em `settings.json`, onde o agente o lê. O lazyagents usa modo 0600 ao escrevê-lo.
- **Pi:** `--env-key VAR` evita token no arquivo (`"apiKey": "$VAR"`); caso contrário fica em `models.json`, modo 0600.
- **Credenciais dos agentes:** por exemplo a autenticação do Claude Code, só são lidas para a consulta que exige isso (limites), nunca guardadas, registradas ou exibidas. Para identificar o tipo, o programa lê apenas existência e tipo (`oauth`/`api_key` no `auth.json` do Pi, presença de chave em `models.json` do Pi ou `crush.json`), nunca o valor.

<a id="network"></a>

## Rede

O lazyagents funciona offline. Só usa a rede quando você pede um recurso que depende dela:

| Quando | Consulta |
|---|---|
| abre Usage, executa `lazyagents usage` ou `doctor` | limites Claude Code, no endpoint de `/usage`, cache de 5 minutos; Codex usa arquivos locais |
| instala do GitHub | `git clone --depth 1` do repositório |
| busca no GitHub (`S`) | `gh api` com sua autenticação do `gh` |
| verifica atualizações de skills | repositório de origem |

Nada disso executa na inicialização, e não há telemetria.

<a id="consent-belongs-to-the-agent"></a>

## O consentimento pertence ao agente

Alguns agentes exigem sua aprovação para executar comandos. O lazyagents nunca responde em seu nome. Codex registra a aprovação de hooks em `trusted_hash` e exige `[features] hooks` ativado: o lazyagents instala e avisa da pendência, mas não grava o hash nem ativa a função. Você aprova no Codex.

<a id="third-party-content"></a>

## Conteúdo de terceiros

- **Hooks executam comandos.** Um hook de repositório executa a cada evento correspondente com suas permissões. Hooks encontrados na instalação vêm desmarcados; leia primeiro com `v` em Hooks.
- **Skills são instruções** que seus agentes seguirão. Leia o `SKILL.md` (`enter`) antes de ativar de uma origem desconhecida.
- **Plugins são programas** instalados por você. O lazyagents trata mensagens como não confiáveis, remove controles de terminal e isola falhas da aba, mas o plugin executa com suas permissões. Veja o [guia](guide/plugins.md).

<a id="reporting-a-vulnerability"></a>

## Relatar uma vulnerabilidade

Veja [SECURITY.md](../../SECURITY.md).
