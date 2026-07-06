package agent

import (
	"errors"
	"os/exec"
	"path/filepath"
)

// ClaudeDesktop adapta o Claude Desktop. As skills e conversas dele vivem na
// conta claude.ai (nuvem), então não há dir de skills nem sessões locais para
// gerenciar — o adapter existe para a aba de detecção de agentes.
type ClaudeDesktop struct {
	Home string
	Look func(string) (string, error)
}

func NewClaudeDesktop(home string) *ClaudeDesktop {
	return &ClaudeDesktop{Home: home, Look: exec.LookPath}
}

func (d *ClaudeDesktop) Detect() Agent {
	a := Agent{ID: "claude-desktop", Name: "Claude Desktop", Short: "D"}
	candidates := []string{
		filepath.Join(d.Home, ".config", "Claude"),
		filepath.Join(d.Home, ".config", "claude-desktop"),
		filepath.Join(d.Home, ".var", "app", "com.anthropic.ClaudeDesktop"),
	}
	for _, dir := range candidates {
		if dirExists(dir) {
			a.Installed = true
			a.Detail = dir + " · skills e conversas ficam na conta claude.ai (não gerenciável localmente)"
			return a
		}
	}
	a.Detail = "não instalado"
	return a
}

func (d *ClaudeDesktop) ListSessions() ([]Session, error) { return nil, nil }

func (d *ClaudeDesktop) ResumeCmd(Session) ([]string, string, bool) { return nil, "", false }

// Hermes adapta o Hermes Agent (Nous Research). Detecção por binário/dir de
// config; skills gerenciadas em ~/.hermes/skills quando o dir existir.
type Hermes struct {
	Home string
	Look func(string) (string, error)
}

func NewHermes(home string) *Hermes { return &Hermes{Home: home, Look: exec.LookPath} }

func (h *Hermes) configDir() string { return filepath.Join(h.Home, ".hermes") }

func (h *Hermes) Detect() Agent {
	a := Agent{ID: "hermes-agent", Name: "Hermes Agent", Short: "H"}
	bin := ""
	for _, name := range []string{"hermes", "hermes-agent"} {
		if b, err := h.Look(name); err == nil {
			bin = b
			break
		}
	}
	a.Installed = bin != "" || dirExists(h.configDir())
	if a.Installed {
		a.ManagedDir = filepath.Join(h.configDir(), "skills")
		a.ReadDirs = []string{a.ManagedDir, filepath.Join(h.Home, ".agents", "skills")}
		if bin != "" {
			a.Version = version(bin)
			a.Detail = bin
		} else {
			a.Detail = "config em " + h.configDir()
		}
	} else {
		a.Detail = "não instalado"
	}
	return a
}

func (h *Hermes) ListSessions() ([]Session, error) { return nil, nil }

func (h *Hermes) ResumeCmd(Session) ([]string, string, bool) { return nil, "", false }

// ID implementa Adapter sem I/O.
func (d *ClaudeDesktop) ID() string { return "claude-desktop" }

// ID implementa Adapter sem I/O.
func (h *Hermes) ID() string { return "hermes-agent" }

// Transcript não é suportado: as conversas vivem na conta claude.ai.
func (d *ClaudeDesktop) Transcript(Session) ([]Entry, error) {
	return nil, errors.New("Claude Desktop não expõe transcript local")
}

// Transcript não é suportado: sem formato local conhecido.
func (h *Hermes) Transcript(Session) ([]Entry, error) {
	return nil, errors.New("Hermes Agent não expõe transcript local")
}
