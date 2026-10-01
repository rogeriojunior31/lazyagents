package sessions

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// ExportMarkdown writes a session transcript as Markdown into dir and returns
// the path. An empty transcript is an error and writes nothing.
func ExportMarkdown(s agent.Session, entries []agent.Entry, dir string) (string, error) {
	if len(entries) == 0 {
		return "", fmt.Errorf("empty transcript, nothing to export")
	}
	for _, id := range []string{s.AgentID, s.ID} {
		if !filepath.IsLocal(id) || id == "." || strings.ContainsAny(id, `/\`) {
			return "", fmt.Errorf("unsafe session id: %q", id)
		}
	}
	ts := time.Now().Format("20060102T150405.000000000")
	name := fmt.Sprintf("%s-%s-%s.md", s.AgentID, s.ID, ts)
	path := filepath.Join(dir, name)

	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", s.Title)
	fmt.Fprintf(&b, "- agent: %s\n", s.AgentName)
	fmt.Fprintf(&b, "- date: %s\n", s.MTime.Format("2006-01-02 15:04"))
	if s.CWD != "" {
		fmt.Fprintf(&b, "- folder: %s\n", s.CWD)
	}
	b.WriteString("\n---\n\n")
	agentName := s.AgentName
	if agentName == "" {
		agentName = "agent"
	}
	for _, t := range turns(entries) {
		if t.event {
			for _, e := range t.entries {
				fmt.Fprintf(&b, "_— %s —_\n\n", e.Text)
			}
			continue
		}
		role := "◀ " + agentName
		if t.user {
			role = "▶ you"
		}
		fmt.Fprintf(&b, "## %s\n\n", role)
		inTools := false
		for _, e := range t.entries {
			if e.Role == agent.RoleTool {
				note := ""
				if e.Added+e.Removed > 0 {
					note = fmt.Sprintf(" (+%d −%d)", e.Added, e.Removed)
				}
				if e.Failed {
					note += " ✗"
				}
				fmt.Fprintf(&b, "- ❯ `%s`%s\n", strings.ReplaceAll(e.Text, "`", "'"), note)
				if e.Body != "" { // a plan or task list, indented under its call
					fmt.Fprintf(&b, "\n    %s\n\n", strings.ReplaceAll(e.Body, "\n", "\n    "))
				}
				inTools = true
				continue
			}
			if inTools {
				b.WriteString("\n")
				inTools = false
			}
			if e.Role == agent.RoleThinking { // quoted, apart from the reply
				fmt.Fprintf(&b, "> 💭 %s\n\n", strings.ReplaceAll(e.Text, "\n", "\n> "))
				continue
			}
			fmt.Fprintf(&b, "%s\n\n", e.Text)
		}
		if inTools {
			b.WriteString("\n")
		}
	}

	if err := fsutil.WriteAtomic(path, []byte(b.String()), 0o600); err != nil {
		return "", fmt.Errorf("exporting transcript: %w", err)
	}
	return path, nil
}
