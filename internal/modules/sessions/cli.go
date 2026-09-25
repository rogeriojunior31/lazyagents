package sessions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
)

func commands(svc *Service) []cli.Command {
	return []cli.Command{
		{Name: "sessions", Usage: "sessions [--agent id] [--here] [--limit n] [--json]",
			Summary: "list agent sessions, newest first", Run: func(c cli.Context, a []string) int {
				return cmdSessions(a, c, svc)
			}},
	}
}

func cmdSessions(args []string, c cli.Context, sessionSvc *Service) int {
	out, errOut := c.Out, c.Err
	fs := cli.Flags("sessions", errOut)
	jsonOut := fs.Bool("json", false, "JSON output")
	agentID := fs.String("agent", "", "only this agent")
	here := fs.Bool("here", false, "only sessions from the current directory")
	limit := fs.Int("limit", 0, "at most n sessions (0 = all)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if !c.KnownAgent(*agentID) {
		return 1
	}
	if sessionSvc == nil {
		fmt.Fprintln(errOut, "lazyagents: sessions service unavailable")
		return 1
	}
	sessions, err := sessionSvc.List()
	if err != nil {
		fmt.Fprintln(errOut, "lazyagents:", err)
		return 1
	}
	cwd := ""
	if *here {
		if cwd, err = os.Getwd(); err != nil {
			fmt.Fprintln(errOut, "lazyagents:", err)
			return 1
		}
	}
	sessions = filter(sessions, *agentID, cwd, *limit)
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
	fmt.Fprintln(tw, "AGENT\tTITLE\tCWD\tUPDATED")
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

// jsonSessionItem is the stable shape of `sessions --json`.
type jsonSessionItem struct {
	ID      string     `json:"id"`
	Agent   string     `json:"agent"`
	Title   string     `json:"title"`
	Alias   string     `json:"alias,omitempty"`
	CWD     string     `json:"cwd,omitempty"`
	Updated string     `json:"updated,omitempty"`
	Usage   *jsonUsage `json:"usage,omitempty"`
}

// jsonUsage is present only when the adapter implements agent.UsageReader.
type jsonUsage struct {
	Input      int      `json:"input"`
	Output     int      `json:"output"`
	CacheRead  int      `json:"cache_read"`
	CacheWrite int      `json:"cache_write"`
	Model      string   `json:"model,omitempty"`
	CostUSD    *float64 `json:"cost_usd,omitempty"`
}

// filter keeps the service order; empty cwd or agentID do not filter and
// limit <= 0 does not cut.
func filter(sessions []agent.Session, agentID, cwd string, limit int) []agent.Session {
	var out []agent.Session
	for _, s := range sessions {
		if agentID != "" && s.AgentID != agentID {
			continue
		}
		if cwd != "" && filepath.Clean(s.CWD) != filepath.Clean(cwd) {
			continue
		}
		out = append(out, s)
		if limit > 0 && len(out) == limit {
			break
		}
	}
	return out
}
