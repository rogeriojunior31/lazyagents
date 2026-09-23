package usage

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/rogeriojunior31/lazyagents/internal/cli"
)

const usageLine = "usage [limits|daily|agents|projects|models] [--agent id] [--since 7d] [--limit n] [--refresh] [--json]"

const usageHelp = `visões:
  (nenhuma)  painel: limites da assinatura, bloco atual de 5h e resumo do período
  limits     janelas de limite (sessão, semana) com uso e horário de reset
  daily      tokens por dia
  agents     tokens por agente
  projects   tokens por projeto (diretório da sessão)
  models     tokens por modelo

opções:
  --agent id       só este agente
  --since período  início do período: 7d, 24h ou 2026-09-01 (padrão 7d)
  --limit n        linhas de agents/projects/models no texto (padrão 10; 0 = todas)
  --refresh        ignora o cache de limites (5 min) e consulta de novo
  --json           saída JSON (sem corte de --limit)

Tokens vêm dos transcripts locais; limites, da conta de cada agente.
O custo em USD só aparece para agente autenticado por API key: em assinatura
não se paga por token.`

// views são as visões de `usage <visão>`; a vazia é o painel.
var views = []string{"limits", "daily", "agents", "projects", "models"}

// commands são os subcomandos da CLI deste módulo.
func commands(svc *Service) []cli.Command {
	return []cli.Command{{
		Name:    "usage",
		Usage:   usageLine,
		Summary: "consumo dos agentes: limites da assinatura, tokens e custo por dia, agente, projeto e modelo",
		Help:    usageHelp,
		Run:     func(c cli.Context, a []string) int { return cmdUsage(a, c, svc) },
	}}
}

// usageOpts são os argumentos já validados.
type usageOpts struct {
	view, agentID string
	from          time.Time
	period        string // rótulo do período: "últimos 7 dias"
	limit         int
	refresh, json bool
}

func cmdUsage(args []string, c cli.Context, svc *Service) int {
	var o usageOpts
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		o.view, args = args[0], args[1:]
		if !slices.Contains(views, o.view) {
			fmt.Fprintf(c.Err, "lazyagents usage: visão desconhecida %q (%s)\n", o.view, strings.Join(views, ", "))
			return 1
		}
	}
	fs := cli.Flags("usage", c.Err)
	fs.Usage = func() { fmt.Fprintf(c.Err, "uso: lazyagents %s\n\n%s\n", usageLine, usageHelp) }
	fs.StringVar(&o.agentID, "agent", "", "só este agente")
	since := fs.String("since", "7d", "início do período")
	fs.IntVar(&o.limit, "limit", 10, "linhas no texto")
	fs.BoolVar(&o.refresh, "refresh", false, "ignora o cache de limites")
	fs.BoolVar(&o.json, "json", false, "saída JSON")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(c.Err, "lazyagents usage: argumento inesperado %q\n", fs.Arg(0))
		return 1
	}
	if !c.KnownAgent(o.agentID) {
		return 1
	}
	var err error
	if o.from, o.period, err = parseSince(*since, time.Now()); err != nil {
		fmt.Fprintln(c.Err, "lazyagents usage:", err)
		return 1
	}
	switch o.view {
	case "":
		return usagePanel(c, svc, o)
	case "limits":
		return usageLimits(c, svc, o)
	default:
		return usageTotals(c, svc, o)
	}
}

// parseSince lê o início do período: "Nd" (hoje e os N-1 dias anteriores,
// desde a meia-noite), "Nh" (últimas N horas) ou uma data AAAA-MM-DD local.
func parseSince(s string, now time.Time) (time.Time, string, error) {
	bad := fmt.Errorf("--since %q inválido (use 7d, 24h ou 2026-09-01)", s)
	if unit := s[max(0, len(s)-1):]; unit == "d" || unit == "h" {
		n, err := strconv.Atoi(s[:len(s)-1])
		if err != nil || n <= 0 {
			return time.Time{}, "", bad
		}
		switch unit {
		case "d":
			y, m, d := now.Date()
			from := time.Date(y, m, d, 0, 0, 0, 0, now.Location()).AddDate(0, 0, -(n - 1))
			if n == 1 {
				return from, "hoje", nil
			}
			return from, fmt.Sprintf("últimos %d dias", n), nil
		case "h":
			return now.Add(-time.Duration(n) * time.Hour), fmt.Sprintf("últimas %d horas", n), nil
		}
	}
	if t, err := time.ParseInLocation("2006-01-02", s, now.Location()); err == nil {
		return t, "desde " + t.Format("02/01/2006"), nil
	}
	return time.Time{}, "", bad
}

// statuses consulta os limites (cacheados, salvo refresh) do filtro.
func statuses(svc *Service, o usageOpts) []Status {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out := []Status{}
	for _, st := range svc.Status(ctx, o.refresh) {
		if o.agentID == "" || st.AgentID == o.agentID {
			out = append(out, st)
		}
	}
	return out
}

// costScope devolve o Pricer do filtro e se a coluna de custo faz sentido
// (algum agente do filtro é cobrado por token).
func costScope(svc *Service, agentID string) (Pricer, bool) {
	api := svc.apiKeyAgents()
	return pricerFor(api), len(api) > 0 && (agentID == "" || api[agentID])
}

func usageLimits(c cli.Context, svc *Service, o usageOpts) int {
	sts := statuses(svc, o)
	if o.json {
		writeJSON(c.Out, sts)
		return exitFor(sts)
	}
	if len(sts) == 0 {
		fmt.Fprintln(c.Err, "lazyagents: nenhum agente com informação de limites")
		return 1
	}
	_, _ = lipgloss.Fprint(c.Out, renderLimits(sts))
	return exitFor(sts)
}

func usageTotals(c cli.Context, svc *Service, o usageOpts) int {
	events := svc.RecentEvents(o.from, o.agentID)
	price, showCost := costScope(svc, o.agentID)
	var rows []Total
	switch o.view {
	case "daily":
		rows = fillDays(Daily(events, 0, price), o.from, time.Now())
	case "agents":
		rows = ByAgent(events, price)
	case "projects":
		rows = ByProject(events, price)
	case "models":
		rows = ByModel(events, price)
	}
	total := Sum(events, price)
	if o.json {
		out := jsonReport{View: o.view, Since: o.from, Rows: []jsonTotal{}, Total: toJSON(total, showCost)}
		for _, t := range rows {
			out.Rows = append(out.Rows, toJSON(t, showCost))
		}
		writeJSON(c.Out, out)
		return 0
	}
	if len(events) == 0 {
		fmt.Fprintln(c.Out, "sem uso registrado nos transcripts —", o.period)
		return 0
	}
	hidden := 0
	if o.view != "daily" && o.limit > 0 && len(rows) > o.limit {
		rows, hidden = rows[:o.limit], len(rows)-o.limit
	}
	_, _ = lipgloss.Fprint(c.Out, renderTotals(o, rows, hidden, total, showCost))
	return 0
}

func usagePanel(c cli.Context, svc *Service, o usageOpts) int {
	sts := statuses(svc, o)
	events := svc.RecentEvents(o.from, o.agentID)
	price, showCost := costScope(svc, o.agentID)
	now := time.Now()
	block, hasBlock := Current(Blocks(events, now))
	if o.json {
		out := jsonPanel{Since: o.from, Limits: sts, Total: toJSON(Sum(events, price), showCost), Agents: []jsonTotal{}, Daily: []jsonTotal{}}
		if hasBlock {
			out.Block = &jsonBlock{Start: block.Start, End: block.End, Tokens: Tokens(block.Usage), Events: block.Events}
		}
		for _, t := range ByAgent(events, price) {
			out.Agents = append(out.Agents, toJSON(t, showCost))
		}
		for _, t := range fillDays(Daily(events, 0, price), o.from, now) {
			out.Daily = append(out.Daily, toJSON(t, showCost))
		}
		writeJSON(c.Out, out)
		return 0
	}
	if len(sts) == 0 && len(events) == 0 {
		fmt.Fprintln(c.Err, "lazyagents: nenhum agente com informação de uso —", o.period)
		return 1
	}
	_, _ = lipgloss.Fprint(c.Out, renderPanel(o, sts, events, price, showCost, now))
	return 0
}

// fillDays devolve um Total por dia de from até now, em ordem cronológica,
// com zero nos dias sem uso: a série fica contínua para tabela e sparkline.
func fillDays(days []Total, from, now time.Time) []Total {
	byDay := map[string]Total{}
	for _, d := range days {
		byDay[d.Label] = d
	}
	var out []Total
	y, m, d := from.Date()
	for day := time.Date(y, m, d, 0, 0, 0, 0, now.Location()); !day.After(now); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		t, ok := byDay[key]
		if !ok {
			t = Total{Label: key, Priced: true}
		}
		out = append(out, t)
	}
	return out
}

// exitFor devolve 1 quando nenhum agente conseguiu informar limites.
func exitFor(sts []Status) int {
	for _, st := range sts {
		if len(st.Limits.Windows) > 0 {
			return 0
		}
	}
	return 1
}

// --- JSON: formas estáveis do `usage --json` ---

type jsonTotal struct {
	Label      string   `json:"label"`
	Tokens     int      `json:"tokens"`
	Input      int      `json:"input"`
	Output     int      `json:"output"`
	CacheRead  int      `json:"cache_read"`
	CacheWrite int      `json:"cache_write"`
	Events     int      `json:"events"`
	CostUSD    *float64 `json:"cost_usd,omitempty"` // só com API key e todo evento com preço
}

type jsonReport struct {
	View  string      `json:"view"`
	Since time.Time   `json:"since"`
	Rows  []jsonTotal `json:"rows"`
	Total jsonTotal   `json:"total"`
}

type jsonBlock struct {
	Start  time.Time `json:"start"`
	End    time.Time `json:"end"`
	Tokens int       `json:"tokens"`
	Events int       `json:"events"`
}

type jsonPanel struct {
	Since  time.Time   `json:"since"`
	Limits []Status    `json:"limits"`
	Block  *jsonBlock  `json:"current_block,omitempty"`
	Total  jsonTotal   `json:"total"`
	Agents []jsonTotal `json:"agents"`
	Daily  []jsonTotal `json:"daily"`
}

func toJSON(t Total, showCost bool) jsonTotal {
	j := jsonTotal{Label: t.Label, Tokens: t.Tokens, Input: t.Usage.Input, Output: t.Usage.Output,
		CacheRead: t.Usage.CacheRead, CacheWrite: t.Usage.CacheWrite, Events: t.Events}
	if showCost && t.Priced {
		cost := t.Cost
		j.CostUSD = &cost
	}
	return j
}

func writeJSON(w io.Writer, v any) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// checks reporta no doctor como cada agente está autenticado e se os limites
// foram obtidos (usa o cache; não força rede).
func checks(svc *Service) []cli.Check {
	return []cli.Check{{Title: "uso", Run: func(c cli.Context, out io.Writer) []string {
		var problems []string
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		for _, st := range svc.Status(ctx, false) {
			switch {
			case st.Err != "":
				fmt.Fprintf(out, "  ✗ %-16s %s\n", st.AgentID, st.Err)
				problems = append(problems, "uso de "+st.AgentID+": "+st.Err)
			default:
				fmt.Fprintf(out, "  ✓ %-16s %s · %d janela(s)\n", st.AgentID, st.AuthLabel, len(st.Limits.Windows))
			}
		}
		return problems
	}}}
}
