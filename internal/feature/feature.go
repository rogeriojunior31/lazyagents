// Package feature é o contrato entre um módulo e a raiz de composição.
//
// Um módulo vive num pacote só (internal/modules/<nome>): service de domínio,
// aba da TUI, comandos da CLI e a Feature que declara tudo isso. O pacote
// app não conhece nenhum módulo concreto além da linha que o registra, e
// nem cli nem tui conhecem módulo algum — os dois são framework.
package feature

import (
	"sync"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
)

// Deps é o que todo módulo recebe: o que é comum a todos e caro de montar
// duas vezes. O service do módulo é construído pelo próprio módulo, dentro da
// Feature — por isso Deps não cresce a cada módulo novo.
type Deps struct {
	Paths    core.Paths
	Adapters []agent.Adapter
	Version  string
	// Config é o config.yaml lido no boot (zero se ausente ou inválido). Cada
	// módulo lê a própria seção com Config.Section("<id>", &cfg).
	Config core.Config

	mu         sync.Mutex
	notices    []string
	reserved   map[string]bool
	hidden     map[string]bool // abas ocultas pela seção tui: do config
	skipped    map[string]bool // ocultas que alguma feature consultou (existem)
	detectOnce sync.Once
	agents     []agent.Agent
}

// Agents devolve a detecção dos agentes, memoizada (Detect roda `--version`
// de cada CLI; só paga quando alguém pede, e uma vez só).
func (d *Deps) Agents() []agent.Agent {
	d.detectOnce.Do(func() { d.agents = agent.DetectAll(d.Adapters) })
	return d.agents
}

// Notice registra um aviso de boot (migração, config inválida, plugin
// pulado). A TUI imprime ao fechar a tela alternativa; a CLI, na hora.
func (d *Deps) Notice(msgs ...string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.notices = append(d.notices, msgs...)
}

// Notices devolve os avisos acumulados.
func (d *Deps) Notices() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.notices...)
}

// Reserve marca nomes já ocupados por abas e comandos embutidos. Quem cria
// abas em runtime (plugins) consulta Reserved antes de registrar.
func (d *Deps) Reserve(names ...string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.reserved == nil {
		d.reserved = map[string]bool{}
	}
	for _, n := range names {
		d.reserved[n] = true
	}
}

// Reserved diz se o nome já pertence a uma aba ou comando embutido.
func (d *Deps) Reserved(name string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.reserved[name]
}

// HideTabs marca abas ocultas pela configuração. O app chama antes de
// instanciar as abas.
func (d *Deps) HideTabs(ids ...string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.hidden == nil {
		d.hidden = map[string]bool{}
	}
	for _, id := range ids {
		d.hidden[id] = true
	}
}

// TabHidden diz se a aba id está oculta. Aba oculta de módulo embutido é
// criada assim mesmo (continua alimentando as outras pelos eventos); quem
// sobe um processo por aba (plugins) consulta isto e nem cria a aba.
func (d *Deps) TabHidden(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.hidden[id] {
		return false
	}
	if d.skipped == nil {
		d.skipped = map[string]bool{}
	}
	d.skipped[id] = true
	return true
}

// SkippedTabs devolve as abas ocultas que alguma feature deixou de criar.
func (d *Deps) SkippedTabs() map[string]bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := map[string]bool{}
	for id := range d.skipped {
		out[id] = true
	}
	return out
}

// Feature é o que um módulo declara: abas, comandos, checks do doctor. Todos
// os campos são opcionais — um módulo pode ser só CLI, ou só aba.
type Feature struct {
	// Name é o id do módulo. Vira nome reservado para plugins.
	Name string
	// Tabs devolve as abas do módulo (normalmente uma; os plugins devolvem
	// uma por binário encontrado).
	Tabs func(d *Deps) []module.Module
	// Commands são os subcomandos da CLI.
	Commands func(d *Deps) []cli.Command
	// Checks são as seções do doctor.
	Checks func(d *Deps) []cli.Check
	// Close encerra recursos do módulo ao sair (processos de plugin).
	Close func()
	// Last empurra a aba para o fim, depois até das abas de plugin. É para
	// aba de consulta (Uso), que nunca deve disputar espaço com as de trabalho.
	Last bool
}
