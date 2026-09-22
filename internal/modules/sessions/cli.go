package sessions

import (
	"encoding/json"
	"flag"
	"fmt"
	"text/tabwriter"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
)

// SessionCommands são os subcomandos da feature sessions.
func commands(svc *Service) []cli.Command {
	return []cli.Command{
		{Name: "sessions", Usage: "sessions [--json]", Run: func(c cli.Context, a []string) int {
			return cmdSessions(a, c, svc)
		}},
	}
}

func cmdSessions(args []string, c cli.Context, sessionSvc *Service) int {
	out, errOut := c.Out, c.Err
	fs := flag.NewFlagSet("sessions", flag.ContinueOnError)
	fs.SetOutput(errOut)
	jsonOut := fs.Bool("json", false, "saída JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if sessionSvc == nil {
		fmt.Fprintln(errOut, "lazyagents: service de sessões não disponível")
		return 1
	}
	sessions, err := sessionSvc.List()
	if err != nil {
		fmt.Fprintln(errOut, "lazyagents:", err)
		return 1
	}
	if *jsonOut {
		items := make([]jsonSessionItem, 0, len(sessions))
		for _, s := range sessions {
			item := jsonSessionItem{
				ID:      s.ID,
				Agent:   s.AgentID,
				Title:   s.Title,
				Alias:   s.Alias,
				CWD:     c.Paths.Tilde(s.CWD),
				Updated: s.MTime.Format("2006-01-02 15:04"),
			}
			if u, ok := sessionSvc.SessionUsage(s); ok {
				ju := &jsonUsage{Input: u.Input, Output: u.Output, CacheRead: u.CacheRead, CacheWrite: u.CacheWrite, Model: u.Model}
				if cost, okCost := agent.EstimateCost(u); okCost {
					ju.CostUSD = &cost
				}
				item.Usage = ju
			}
			items = append(items, item)
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(items)
		return 0
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "AGENTE\tTÍTULO\tCWD\tATUALIZADO")
	for _, s := range sessions {
		title := s.Title
		if s.Alias != "" {
			title = s.Alias + " (" + s.Title + ")"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", s.AgentID, title, c.Paths.Tilde(s.CWD), s.MTime.Format("2006-01-02 15:04"))
	}
	_ = tw.Flush()
	return 0
}

// jsonSessionItem é a forma estável do `sessions --json`.
type jsonSessionItem struct {
	ID      string     `json:"id"`
	Agent   string     `json:"agent"`
	Title   string     `json:"title"`
	Alias   string     `json:"alias,omitempty"`
	CWD     string     `json:"cwd,omitempty"`
	Updated string     `json:"updated,omitempty"`
	Usage   *jsonUsage `json:"usage,omitempty"`
}

// jsonUsage só aparece quando o adapter da sessão sabe informar tokens
// (agent.UsageReader).
type jsonUsage struct {
	Input      int      `json:"input"`
	Output     int      `json:"output"`
	CacheRead  int      `json:"cache_read"`
	CacheWrite int      `json:"cache_write"`
	Model      string   `json:"model,omitempty"`
	CostUSD    *float64 `json:"cost_usd,omitempty"`
}
