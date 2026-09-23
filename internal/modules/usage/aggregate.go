package usage

import (
	"path/filepath"
	"sort"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
)

// BlockWindow é a janela de sessão usada nos blocos (o mesmo 5h do Claude Code).
const BlockWindow = 5 * time.Hour

// Block é uma janela de atividade: começa no primeiro evento e vai até
// BlockWindow depois dele; um intervalo maior que a janela abre outro bloco.
type Block struct {
	Start  time.Time   `json:"start"`
	End    time.Time   `json:"end"` // fim da janela (Start + BlockWindow)
	Last   time.Time   `json:"last"`
	Usage  agent.Usage `json:"-"`
	Events int         `json:"events"`
	Active bool        `json:"active"` // a janela ainda contém agora
}

// Blocks agrupa eventos (já ordenados) em janelas de BlockWindow.
func Blocks(events []agent.UsageEvent, now time.Time) []Block {
	var out []Block
	for _, e := range events {
		if n := len(out); n > 0 && e.Time.Before(out[n-1].End) {
			b := &out[n-1]
			add(&b.Usage, e.Usage)
			b.Events++
			b.Last = e.Time
			continue
		}
		out = append(out, Block{Start: e.Time, End: e.Time.Add(BlockWindow), Last: e.Time, Usage: e.Usage, Events: 1})
	}
	for i := range out {
		out[i].Active = now.Before(out[i].End) && !now.Before(out[i].Start)
	}
	return out
}

// Current devolve o bloco que contém agora, se houver.
func Current(blocks []Block) (Block, bool) {
	for i := len(blocks) - 1; i >= 0; i-- {
		if blocks[i].Active {
			return blocks[i], true
		}
	}
	return Block{}, false
}

// Total é um agregado rotulado (um dia, um projeto, um agente, um modelo).
type Total struct {
	Label  string      `json:"label"`
	Usage  agent.Usage `json:"-"`
	Events int         `json:"events"`
	Tokens int         `json:"tokens"`
	Cost   float64     `json:"-"` // soma do custo de cada evento
	Priced bool        `json:"-"` // Cost vale: todo evento do agregado teve preço
}

// Pricer estima o custo de um evento; ok=false = sem preço (conta por
// assinatura ou modelo fora da tabela). nil = não estima custo.
type Pricer func(agent.UsageEvent) (float64, bool)

// Daily soma por dia local, do mais recente para o mais antigo, no máximo n dias.
func Daily(events []agent.UsageEvent, n int, price Pricer) []Total {
	out := group(events, price, func(e agent.UsageEvent) (string, string) {
		day := e.Time.Local().Format("2006-01-02")
		return day, day
	})
	sort.SliceStable(out, func(i, j int) bool { return out[i].Label > out[j].Label })
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

// ByProject soma por CWD (rotulado pelo basename), do maior para o menor.
func ByProject(events []agent.UsageEvent, price Pricer) []Total {
	return byTokens(group(events, price, func(e agent.UsageEvent) (string, string) {
		if e.CWD != "" {
			if base := filepath.Base(e.CWD); base != "." && base != "/" && base != "" {
				return filepath.Clean(e.CWD), base
			}
		}
		return "", "sem projeto"
	}))
}

// ByAgent soma por agente, do maior para o menor.
func ByAgent(events []agent.UsageEvent, price Pricer) []Total {
	return byTokens(group(events, price, func(e agent.UsageEvent) (string, string) { return e.AgentID, e.AgentID }))
}

// ByModel soma por modelo, do maior para o menor.
func ByModel(events []agent.UsageEvent, price Pricer) []Total {
	return byTokens(group(events, price, func(e agent.UsageEvent) (string, string) {
		m := eventModel(e)
		if m == "" {
			m = "desconhecido"
		}
		return m, m
	}))
}

// Sum é o agregado de todos os eventos.
func Sum(events []agent.UsageEvent, price Pricer) Total {
	all := group(events, price, func(agent.UsageEvent) (string, string) { return "", "total" })
	if len(all) == 0 {
		return Total{Label: "total", Priced: price != nil}
	}
	return all[0]
}

// group soma os eventos por chave, na ordem da primeira aparição. O custo é
// somado evento a evento: um agregado de modelos diferentes nunca recebe a
// tarifa de um só, e basta um evento sem preço para o total ficar sem preço.
func group(events []agent.UsageEvent, price Pricer, key func(agent.UsageEvent) (id, label string)) []Total {
	byKey := map[string]*Total{}
	var order []string
	for _, e := range events {
		id, label := key(e)
		t, ok := byKey[id]
		if !ok {
			t = &Total{Label: label, Priced: price != nil}
			byKey[id] = t
			order = append(order, id)
		}
		add(&t.Usage, e.Usage)
		t.Events++
		if price != nil {
			c, ok := price(e)
			t.Cost += c
			t.Priced = t.Priced && ok
		}
	}
	out := make([]Total, 0, len(order))
	for _, k := range order {
		t := *byKey[k]
		t.Tokens = Tokens(t.Usage)
		out = append(out, t)
	}
	return out
}

func byTokens(out []Total) []Total {
	sort.SliceStable(out, func(i, j int) bool { return out[i].Tokens > out[j].Tokens })
	return out
}

// eventModel é o modelo do evento, com o do uso como reserva.
func eventModel(e agent.UsageEvent) string {
	if e.Model != "" {
		return e.Model
	}
	return e.Usage.Model
}

// Tokens é o total de tokens de um uso (entrada fresca, saída e cache).
func Tokens(u agent.Usage) int { return u.Input + u.Output + u.CacheRead + u.CacheWrite }

// Cost estima o custo em USD. ok=false em conta por assinatura (onde não se
// paga por token) ou com modelo fora da tabela de preços.
func Cost(u agent.Usage, mode agent.AuthMode) (float64, bool) {
	if mode != agent.AuthAPIKey {
		return 0, false
	}
	return agent.EstimateCost(u)
}

func add(dst *agent.Usage, src agent.Usage) {
	if Tokens(*dst) == 0 {
		dst.Model = src.Model
	} else if dst.Model != src.Model {
		dst.Model = "mixed" // tarifa única não representa um agregado de modelos diferentes
	}
	dst.Input += src.Input
	dst.Output += src.Output
	dst.CacheRead += src.CacheRead
	dst.CacheWrite += src.CacheWrite

}
