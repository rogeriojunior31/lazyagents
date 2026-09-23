package usage

import (
	"slices"
	"strings"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
)

// Filtros da aba Uso: período, agente, visão e texto. Tudo em memória sobre
// os eventos já carregados — trocar de filtro nunca relê transcript.

// period é uma opção do filtro de período; since segue o --since da CLI.
type period struct{ id, label, since string }

var periods = []period{
	{"today", "hoje", "1d"},
	{"7d", "7 dias", "7d"},
	{"30d", "30 dias", "30d"},
	{"90d", "90 dias", "90d"},
	{"all", "tudo", ""},
}

// tabViews são as visões da tabela, na ordem do ←/→; os ids são os da CLI.
var tabViews = []struct{ id, label string }{
	{"daily", "dia"},
	{"agents", "agente"},
	{"projects", "projeto"},
	{"models", "modelo"},
}

// config é a seção `usage:` do config.yaml: os filtros com que a aba abre.
type config struct {
	Period string `yaml:"period"` // today | 7d | 30d | 90d | all
	View   string `yaml:"view"`   // daily | agents | projects | models
}

// filters é o estado dos filtros da aba.
type filters struct {
	period int    // índice em periods
	view   int    // índice em tabViews
	agent  string // "" = todos
	text   string // filtro nas linhas da tabela; "" = sem filtro
}

// newFilters aplica a config; valor desconhecido fica no padrão (7 dias, dia).
func newFilters(cfg config) filters {
	f := filters{period: 1}
	if i := slices.IndexFunc(periods, func(p period) bool { return p.id == cfg.Period }); i >= 0 {
		f.period = i
	}
	if i := slices.IndexFunc(tabViews, func(v struct{ id, label string }) bool { return v.id == cfg.View }); i >= 0 {
		f.view = i
	}
	return f
}

// from é o início do período; "tudo" começa no primeiro evento.
func (f filters) from(events []agent.UsageEvent, now time.Time) time.Time {
	if p := periods[f.period]; p.since != "" {
		t, _, _ := parseSince(p.since, now)
		return t
	}
	if len(events) == 0 {
		return now
	}
	first := events[0].Time // eventos chegam em ordem cronológica
	y, m, d := first.Local().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

// apply devolve os eventos do período e do agente escolhidos. Os eventos
// estão em ordem cronológica: o período é uma busca binária e, sem filtro
// de agente, só um recorte (o histórico pode ter centenas de milhares).
func (f filters) apply(events []agent.UsageEvent, now time.Time) []agent.UsageEvent {
	from := f.from(events, now)
	i, _ := slices.BinarySearchFunc(events, from, func(e agent.UsageEvent, t time.Time) int { return e.Time.Compare(t) })
	events = events[i:]
	if f.agent == "" {
		return events
	}
	var out []agent.UsageEvent
	for _, e := range events {
		if e.AgentID == f.agent {
			out = append(out, e)
		}
	}
	return out
}

// rows monta as linhas da visão; o filtro de texto casa com o rótulo, sem
// diferenciar maiúsculas. Dias saem do mais recente para o mais antigo.
func (f filters) rows(events []agent.UsageEvent, price Pricer, from, now time.Time) []Total {
	var rows []Total
	switch tabViews[f.view].id {
	case "daily":
		rows = fillDays(Daily(events, 0, price), from, now)
		slices.Reverse(rows)
	case "agents":
		rows = ByAgent(events, price)
	case "projects":
		rows = ByProject(events, price)
	case "models":
		rows = ByModel(events, price)
	}
	if f.text == "" {
		return rows
	}
	q := strings.ToLower(f.text)
	return slices.DeleteFunc(rows, func(t Total) bool {
		label := t.Label
		if tabViews[f.view].id == "daily" {
			label = dayText(t.Label) // "ter 23/09" também casa
		}
		return !strings.Contains(strings.ToLower(label), q)
	})
}

// agentsIn lista os agentes com evento ou limite, na ordem em que aparecem
// nos limites (a do registro) e depois nos eventos.
func agentsIn(sts []Status, events []agent.UsageEvent) []string {
	var ids []string
	for _, st := range sts {
		if !slices.Contains(ids, st.AgentID) {
			ids = append(ids, st.AgentID)
		}
	}
	for _, e := range events {
		if !slices.Contains(ids, e.AgentID) {
			ids = append(ids, e.AgentID)
		}
	}
	return ids
}

// cycle devolve o próximo (step 1) ou anterior (-1) de "" + ids.
func cycle(cur string, ids []string, step int) string {
	opts := append([]string{""}, ids...)
	i := max(0, slices.Index(opts, cur))
	return opts[(i+step+len(opts))%len(opts)]
}
