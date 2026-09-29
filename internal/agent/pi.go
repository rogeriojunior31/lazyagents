package agent

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Pi adapts the pi coding agent (earendil-works/pi). Everything lives in one
// agent dir, ~/.pi/agent unless PI_CODING_AGENT_DIR points elsewhere.
type Pi struct {
	Home string
	Dir  string // PI_CODING_AGENT_DIR; empty means ~/.pi/agent
	Look func(string) (string, error)
}

func NewPi(home string) *Pi {
	return &Pi{Home: home, Dir: os.Getenv("PI_CODING_AGENT_DIR"), Look: exec.LookPath}
}

func (p *Pi) ID() string { return "pi" }

func (p *Pi) agentDir() string {
	switch {
	case p.Dir == "":
		return filepath.Join(p.Home, ".pi", "agent")
	case p.Dir == "~" || strings.HasPrefix(p.Dir, "~/"):
		return filepath.Join(p.Home, p.Dir[1:])
	}
	return p.Dir
}

// semver tells pi apart from other tools that ship a binary named `pi`.
var semver = regexp.MustCompile(`^v?\d+\.\d+\.\d+\S*$`)

func (p *Pi) Detect() Agent {
	a := Agent{ID: "pi", Name: "Pi", Short: "P"}
	dir := p.agentDir()
	hasDir := dirExists(dir)
	bin, _ := p.Look("pi")
	if bin != "" {
		a.Version = version(bin)
		if !hasDir && !semver.MatchString(a.Version) {
			bin, a.Version = "", ""
		}
	}
	a.Installed = bin != "" || hasDir
	if !a.Installed {
		a.Detail = DetailNotInstalled
		return a
	}
	a.ManagedDir = filepath.Join(dir, "skills")
	a.ReadDirs = []string{a.ManagedDir, filepath.Join(p.Home, ".agents", "skills")}
	if bin != "" {
		a.Detail = bin
	} else {
		a.Detail = fmt.Sprintf("config in %s (binary not in PATH)", dir)
	}
	return a
}

func (p *Pi) ListSessions() ([]Session, error) { return nil, nil }

func (p *Pi) ResumeCmd(Session) ([]string, string, bool) { return nil, "", false }

func (p *Pi) Transcript(Session) ([]Entry, error) {
	return nil, errors.New("Pi transcripts are not supported yet")
}

func (p *Pi) DeleteSession(Session, string) error {
	return errors.New("Pi cannot delete sessions yet")
}
