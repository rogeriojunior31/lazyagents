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

// Total é um agregado rotulado (um dia, um projeto, um agente).
type Total struct {
	Label  string      `json:"label"`
	Usage  agent.Usage `json:"-"`
	Events int         `json:"events"`
	Tokens int         `json:"tokens"`
}

// Daily soma por dia local, do mais recente para o mais antigo, no máximo n dias.
func Daily(events []agent.UsageEvent, n int) []Total {
	byDay := map[string]*Total{}
	var order []string
	for _, e := range events {
		key := e.Time.Local().Format("2006-01-02")
		t, ok := byDay[key]
		if !ok {
			t = &Total{Label: key}
			byDay[key] = t
			order = append(order, key)
		}
		add(&t.Usage, e.Usage)
		t.Events++
	}
	sort.Sort(sort.Reverse(sort.StringSlice(order)))
	if n > 0 && len(order) > n {
		order = order[:n]
	}
	return collect(order, byDay)
}

// ByProject soma por basename do CWD, do maior para o menor.
func ByProject(events []agent.UsageEvent) []Total {
	byProj := map[string]*Total{}
	var order []string
	for _, e := range events {
		key := "sem projeto"
		if e.CWD != "" {
			if base := filepath.Base(e.CWD); base != "." && base != "/" && base != "" {
				key = base
			}
		}
		t, ok := byProj[key]
		if !ok {
			t = &Total{Label: key}
			byProj[key] = t
			order = append(order, key)
		}
		add(&t.Usage, e.Usage)
		t.Events++
	}
	out := collect(order, byProj)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Tokens > out[j].Tokens })
	return out
}

func collect(order []string, m map[string]*Total) []Total {
	out := make([]Total, 0, len(order))
	for _, k := range order {
		t := *m[k]
		t.Tokens = Tokens(t.Usage)
		out = append(out, t)
	}
	return out
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
	dst.Input += src.Input
	dst.Output += src.Output
	dst.CacheRead += src.CacheRead
	dst.CacheWrite += src.CacheWrite
	if src.Model != "" {
		dst.Model = src.Model
	}
}
