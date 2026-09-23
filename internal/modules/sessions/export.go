package sessions

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// ExportMarkdown grava o transcript de uma sessão em Markdown dentro de dir
// (tipicamente ExportsDir()), devolvendo o path gravado. Transcript vazio é
// um erro — o chamador decide como avisar (toast, sem gravar arquivo).
func ExportMarkdown(s agent.Session, entries []agent.Entry, dir string) (string, error) {
	if len(entries) == 0 {
		return "", fmt.Errorf("transcript vazio — nada para exportar")
	}
	for _, id := range []string{s.AgentID, s.ID} {
		if !filepath.IsLocal(id) || id == "." || strings.ContainsAny(id, `/\`) {
			return "", fmt.Errorf("identificador de sessão inseguro: %q", id)
		}
	}
	ts := time.Now().Format("20060102T150405.000000000")
	name := fmt.Sprintf("%s-%s-%s.md", s.AgentID, s.ID, ts)
	path := filepath.Join(dir, name)

	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", s.Title)
	fmt.Fprintf(&b, "- agente: %s\n", s.AgentName)
	fmt.Fprintf(&b, "- data: %s\n", s.MTime.Format("02/01/2006 15:04"))
	if s.CWD != "" {
		fmt.Fprintf(&b, "- pasta: %s\n", s.CWD)
	}
	b.WriteString("\n---\n\n")
	agentName := s.AgentName
	if agentName == "" {
		agentName = "agente"
	}
	for _, t := range turns(entries) {
		role := "◀ " + agentName
		if t.user {
			role = "▶ você"
		}
		fmt.Fprintf(&b, "## %s\n\n", role)
		inTools := false
		for _, e := range t.entries {
			if e.Role == agent.RoleTool {
				fmt.Fprintf(&b, "- ⚙ `%s`\n", strings.ReplaceAll(e.Text, "`", "'"))
				inTools = true
				continue
			}
			if inTools {
				b.WriteString("\n")
				inTools = false
			}
			fmt.Fprintf(&b, "%s\n\n", e.Text)
		}
		if inTools {
			b.WriteString("\n")
		}
	}

	if err := fsutil.WriteAtomic(path, []byte(b.String()), 0o600); err != nil {
		return "", fmt.Errorf("exportando transcript: %w", err)
	}
	return path, nil
}
