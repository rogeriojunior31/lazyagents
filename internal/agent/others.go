package agent

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ClaudeDesktop keeps skills and conversations in the claude.ai account, so
// there is nothing local to manage; the adapter exists for detection.
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
			a.Detail = fmt.Sprintf("%s · skills and chats live in the claude.ai account (not manageable locally)", dir)
			return a
		}
	}
	a.Detail = DetailNotInstalled
	return a
}

func (d *ClaudeDesktop) ListSessions() ([]Session, error) { return nil, nil }

func (d *ClaudeDesktop) ResumeCmd(Session) ([]string, string, bool) { return nil, "", false }

// Hermes adapts Hermes Agent (Nous Research): detected by binary or config dir;
// skills managed in <hermes home>/skills, plus the dirs its config.yaml adds
// (hermes_config.go).
type Hermes struct {
	Home       string
	HermesHome string // HERMES_HOME as set; ~ and $VAR are expanded like Hermes does
	Look       func(string) (string, error)
}

func NewHermes(home string) *Hermes {
	return &Hermes{Home: home, HermesHome: os.Getenv("HERMES_HOME"), Look: exec.LookPath}
}

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
		h.hermesSkills(&a)
		if bin != "" {
			a.Version = version(bin)
			a.Detail = bin
		} else {
			a.Detail = fmt.Sprintf("config in %s", h.configDir())
		}
	} else {
		a.Detail = DetailNotInstalled
	}
	return a
}

func (h *Hermes) ListSessions() ([]Session, error) { return nil, nil }

func (h *Hermes) ResumeCmd(Session) ([]string, string, bool) { return nil, "", false }

func (d *ClaudeDesktop) ID() string { return "claude-desktop" }

func (h *Hermes) ID() string { return "hermes-agent" }

// Transcript is unsupported: conversations live in the claude.ai account.
func (d *ClaudeDesktop) Transcript(Session) ([]Entry, error) {
	return nil, errors.New("Claude Desktop has no local transcript")
}

// Transcript is unsupported: no known local format.
func (h *Hermes) Transcript(Session) ([]Entry, error) {
	return nil, errors.New("Hermes Agent has no local transcript")
}

// DeleteSession is unsupported: conversations live in the claude.ai account.
func (d *ClaudeDesktop) DeleteSession(Session, string) error {
	return errors.New("Claude Desktop cannot delete sessions locally")
}

// DeleteSession is unsupported: no known local format.
func (h *Hermes) DeleteSession(Session, string) error {
	return errors.New("Hermes Agent cannot delete sessions locally")
}
