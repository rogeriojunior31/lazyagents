# Primeiros passos
<!-- source: 191c1a027e98 -->

Esta página leva você da instalação a uma skill ativada em dois agentes e uma sessão retomada, em cerca de cinco minutos.

## Instalação

Com Go 1.26 ou mais recente:

```sh
go install github.com/rogeriojunior31/lazyagents@v0.2.3
```

Use a versão explícita durante a renumeração das releases: os espelhos de módulos
do Go ainda podem retornar uma versão antiga de número maior com `@latest`.

Ou baixe um binário em [Releases](https://github.com/rogeriojunior31/lazyagents/releases) (Linux, macOS Intel e Apple Silicon, Windows) e confira-o usando `SHA256SUMS`:

```sh
tar -xzf lazyagents_<version>_linux_amd64.tar.gz
./lazyagents --version
```

<a id="local-homebrew-and-arch-recipes"></a>

### Receitas locais para Homebrew e Arch

O repositório inclui receitas revisadas para os arquivos publicados da versão
**0.2.3**, com seus SHA256 da release. O tap público do Homebrew e a publicação
no AUR **ainda não estão disponíveis**; use Go/Releases acima ou as receitas
locais abaixo. O Homebrew cobre macOS/Linux amd64/arm64; o pacote Arch cobre
Linux x86_64/aarch64. Veja requisitos de atualização e publicação em
[manutenção de pacotes](../../CONTRIBUTING.md#package-recipes).

Em um checkout que contenha `packaging/`, com Homebrew instalado:

```sh
brew tap-new --no-git local/lazyagents
cp packaging/homebrew/Formula/lazyagents.rb "$(brew --repository local/lazyagents)/Formula/"
HOMEBREW_NO_AUTO_UPDATE=1 brew install local/lazyagents/lazyagents
brew test local/lazyagents/lazyagents
lazyagents --version
```

No Arch Linux, revise a receita e use as ferramentas nativas para compilar e
instalar o pacote. `base-devel` é necessário para montar o pacote, não para
executar o binário:

```sh
cd packaging/aur/lazyagents-bin
makepkg --verifysource
makepkg -si
lazyagents --version
```

As receitas verificam o checksum do arquivo escolhido e instalam programa,
README e avisos de licença incluídos na release. Não têm hooks de pós-instalação
e não criam nem reescrevem configurações, bibliotecas ou ajustes dos agentes.

Nenhuma outra ferramenta é obrigatória. Alguns recursos usam ferramentas que você talvez já tenha:

| Ferramenta | Uso |
|---|---|
| `git` | instalar skills do GitHub |
| `gh`, autenticado | buscar skills no GitHub (`S` na aba Skills) |
| `sqlite3` | ler sessões do OpenCode e Crush |
| `lsof` | fora do Linux, indicar quais sessões do Claude Code estão em execução (no Linux, usa `/proc`) |

<a id="upgrade"></a>

## Atualização

Para uma instalação local Homebrew, revise e copie a fórmula mais recente deste
repositório para o mesmo tap antes de executar:

```sh
cp packaging/homebrew/Formula/lazyagents.rb "$(brew --repository local/lazyagents)/Formula/"
HOMEBREW_NO_AUTO_UPDATE=1 brew upgrade local/lazyagents/lazyagents
brew test local/lazyagents/lazyagents
lazyagents --version
```

No Arch, atualize o checkout revisado e execute `makepkg -si` novamente em
`packaging/aur/lazyagents-bin`. A receita local não exige um gerenciador AUR.
Os gerenciadores substituem os arquivos instalados do programa e preservam os
dados do usuário. Testes Linux/amd64 cobrem instalação na 0.2.2, atualização
para 0.2.3 e remoção com checksums de configuração/biblioteca intactos.
Resultados nativos de macOS/ARM ficam separados no
[roadmap](../../BACKLOG.md#m281-package-installation-and-upgrade-paths).

Para instalações Go, repita `go install` com a versão publicada desejada. Para
binários baixados, verifique o novo `SHA256SUMS` antes de substituir o arquivo.
Evite várias cópias no `PATH`; `command -v lazyagents` mostra a usada pelo shell.

## Confira o que o lazyagents encontra

```sh
lazyagents doctor
```

O `doctor` lista os agentes encontrados, valida cada skill e informa links quebrados, hooks e plugins. Um agente é considerado instalado quando seu binário está no `PATH` ou seu diretório de configuração existe. A matriz completa dos recursos de cada agente está na [referência de suporte dos agentes](../reference/agents.md).

## Abra a TUI

```sh
lazyagents
```

As abas ficam no topo: `tab` e `shift+tab` alternam entre elas; `1`–`9` vão direto à aba correspondente (clicar também funciona). `?` lista as teclas da aba atual e `:` abre a paleta de comandos. `q` sai. O painel de detalhes fica ao lado da tabela em terminais largos e abaixo dela nos estreitos; a roda do mouse rola o conteúdo e um clique seleciona.

### Ative uma skill em dois agentes

1. Na aba **Skills**, pressione `i` e informe uma origem: um repositório do GitHub (`usuario/repo` ou sua URL), uma pasta local ou um `.zip`.
2. Revise as skills encontradas e escolha quais instalar. Selecione os agentes de destino, ou mantenha a opção de somente biblioteca, depois revise e confirme. Os arquivos são copiados para sua biblioteca, `~/.local/share/lazyagents/skills/`; o resultado informa instalação e ativação separadamente.
3. Selecione uma skill, vá até a coluna de um agente com `←/→` e pressione `space`. O lazyagents cria um link simbólico no diretório de skills desse agente. Pressionar `space` novamente remove o link.
4. `a` ativa a skill em todos os agentes de uma vez; `x` desativa em todos.

Skills que já estavam dentro de um agente também aparecem, como skills locais. `o` incorpora uma delas à biblioteca para que os outros agentes possam usá-la. Veja o [guia de skills](../guide/skills.md).

### Retome uma sessão

1. Abra a aba **Sessions**. Ela lista conversas do Claude Code, Codex, Gemini CLI, OpenCode, Pi e Crush, das mais recentes para as mais antigas.
2. Digite `/` para filtrar ou `f` para mostrar apenas um agente.
3. Pressione `enter`: o lazyagents suspende sua interface e retoma a sessão na CLI do próprio agente, na pasta correta. Ao sair do agente, você volta ao lazyagents.

`v` permite ler a conversa sem retomar a sessão. Veja o [guia de sessões](../guide/sessions.md).

### Consulte seu consumo

A aba **Usage** mostra os limites da assinatura (janelas de sessão e semanais, com os horários de renovação) e o consumo de tokens das conversas locais. `p` altera o período e `←/→` muda a visualização. Os limites do Claude Code são consultados pela rede somente quando você abre a aba ou executa `lazyagents usage` ou `doctor`, e ficam em cache por cinco minutos. Até você entrar em um agente ou usá-lo pela primeira vez, seus limites aparecem como ainda indisponíveis: isso é esperado. Veja o [guia de consumo](../guide/usage.md).

## Use em scripts

Cada aba tem uma alternativa sem interface, e os comandos de consulta aceitam `--json`:

```sh
lazyagents list --json                     # skills e onde estão ativadas
lazyagents enable my-skill --all           # ativar em todos os agentes
lazyagents sessions --here --limit 5       # sessões recentes nesta pasta
lazyagents usage daily --since 30d
```

`lazyagents help <comando>` explica cada comando; o mesmo texto está na [referência da CLI](../reference/cli.md).

## Desinstalação

Para remover apenas o programa instalado pelo gerenciador de pacotes:

```sh
brew uninstall --force local/lazyagents/lazyagents  # todas as versões Homebrew
# Ou, no Arch:
sudo pacman -R lazyagents-bin
```

Esses comandos preservam configurações, skills, backups e ajustes dos agentes.
Para remover todo o setup, primeiro desfaça as mudanças nos agentes e só depois
remova os dados do usuário explicitamente:

Desfaça o que o lazyagents colocou nos diretórios dos agentes antes de apagar seus dados. Caso contrário, os links das skills apontarão para uma biblioteca que não existe mais (`lazyagents doctor` lista links quebrados):

1. Execute `lazyagents disable <skill>` para cada skill ativada (`lazyagents list` mostra quais são). Para manter uma skill em um agente, primeiro copie sua pasta da biblioteca para o diretório de skills desse agente.
2. Execute `lazyagents provider clear` e `lazyagents hooks disable <nome>` para cada hook instalado.
3. Apague o binário, `~/.config/lazyagents` e `~/.local/share/lazyagents`. A pasta de backups fica dentro da última: preserve-a se houver algo que você possa querer recuperar.

## Próximos passos

- Alterar o tema, a ordem das abas ou a aba inicial: [Configuração](../configuration.md) e [Temas](../themes.md).
- Apontar um agente para outro endpoint de API: [Providers](../guide/providers.md).
- Executar um comando em eventos dos agentes: [Hooks](../guide/hooks.md).
- Entender o que o lazyagents altera no disco antes de usá-lo: [Segurança e dados](../safety.md).
