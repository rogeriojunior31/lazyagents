package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

// Problem carries actionable, value-safe evidence from a feature check.
type Problem struct {
	Severity    string `json:"severity"`
	Resource    string `json:"resource"`
	Explanation string `json:"explanation"`
	NextStep    string `json:"next_step"`
	Tab         string `json:"tab,omitempty"`
	Code        string `json:"code,omitempty"`
	Variable    string `json:"variable,omitempty"`
	Source      string `json:"source,omitempty"`
	OtherSource string `json:"other_source,omitempty"`
}

// External checks can query a remote service or execute plugin-owned code.
// They run only when the caller explicitly includes them.
type Check struct {
	Title         string
	External      bool
	NextStep, Tab string
	Run           func(c Context, out io.Writer) []Problem
}

type DoctorAgent struct {
	ID        string `json:"id"`
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
}

type DoctorSection struct {
	Title    string    `json:"title"`
	Report   string    `json:"report"`
	Problems []string  `json:"problems"`
	Findings []Problem `json:"findings"`
	External bool      `json:"external"`
	Skipped  bool      `json:"skipped"`
	NextStep string    `json:"next_step"`
	Tab      string    `json:"tab,omitempty"`
}

// DoctorReport preserves the original JSON fields; structured evidence and
// execution scope are additive. Skipped external checks never imply success.
type DoctorReport struct {
	OK        bool            `json:"ok"`
	Agents    []DoctorAgent   `json:"agents"`
	Sections  []DoctorSection `json:"sections"`
	Scope     string          `json:"scope"`
	Cancelled bool            `json:"cancelled,omitempty"`
}

type DoctorRunner func(context.Context, bool) DoctorReport

// RunDoctor is the single report evaluator used by the CLI and the TUI.
func RunDoctor(ctx context.Context, c Context, checks []Check, external bool) DoctorReport {
	c.Ctx = ctx
	report := DoctorReport{OK: true, Agents: []DoctorAgent{}, Sections: []DoctorSection{}, Scope: "local"}
	if external {
		report.Scope = "all"
	}
	if c.Agents != nil && ctx.Err() == nil {
		for _, ag := range c.Agents() {
			report.Agents = append(report.Agents, DoctorAgent{ag.ID, ag.Installed, ag.Version})
		}
	}
	for _, check := range checks {
		section := DoctorSection{Title: check.Title, Problems: []string{}, Findings: []Problem{}, External: check.External, NextStep: check.NextStep, Tab: check.Tab}
		switch {
		case ctx.Err() != nil:
			section.Skipped = true
			section.Report = "Not run: diagnostics cancelled. Run the checks again when ready."
		case check.External && !external:
			section.Skipped = true
			section.Report = "Not run: this check may use the network or execute plugin code. Include external checks explicitly."
		default:
			var out bytes.Buffer
			local := c
			local.Out, local.Err = &out, &out
			findings := check.Run(local, &out)
			section.Report = strings.TrimRight(out.String(), "\n")
			for _, p := range findings {
				if p.Severity == "" {
					p.Severity = "warning"
				}
				if p.Resource == "" {
					p.Resource = check.Title
				}
				if p.NextStep == "" {
					p.NextStep = check.NextStep
				}
				if p.Tab == "" {
					p.Tab = check.Tab
				}
				section.Findings = append(section.Findings, p)
				section.Problems = append(section.Problems, p.Explanation)
			}
		}
		report.OK = report.OK && len(section.Problems) == 0
		report.Sections = append(report.Sections, section)
	}
	if ctx.Err() != nil {
		report.Cancelled, report.OK = true, false
	}
	return report
}

// DoctorCommand keeps full diagnostics as the CLI default. --local provides
// the same restricted scope the TUI uses on opening its doctor report.
func DoctorCommand(checks []Check) Command {
	return Command{
		Name: "doctor", Usage: "doctor [--json] [--local]",
		Summary: "diagnose agents, skills, hooks, providers, usage and plugins",
		Help: `sections:
  detected agents  each agent id and whether it is installed
  configuration    current lazyagents config syntax
  skills           library and agent skills with an invalid or incomplete SKILL.md
  symlinks         broken symlinks in any skill dir an agent reads
  providers        the provider applied to each agent; one applied to an
                   agent that is not installed is a problem
  hooks            invalid library files, hook commands missing from PATH or
                   not executable, and each agent's hooks note
  usage            authentication and subscription limits per agent (from the
                   cache when fresh, otherwise fetched; external)
  plugins          skipped plugin files, each plugin's handshake and, when its
                   manifest asks for it, its own doctor (external)

options:
  --json   JSON output: {ok, agents, sections[{title, report, problems,
           findings, external, skipped, next_step, tab}], scope, cancelled?}
  --local  skip usage/network and plugin execution; label their sections not run

Findings include severity, resource, explanation and next_step. Diagnostics
never repair configuration or activation links. Usage can refresh its cache.
Exits with 1 on a problem or cancellation, 0 when completed checks are OK.
Skipped external checks do not certify remote availability.`,
		Run: func(c Context, args []string) int {
			fs := Flags("doctor", c.Err)
			jsonOut := fs.Bool("json", false, "JSON output")
			local := fs.Bool("local", false, "skip external checks")
			if err := fs.Parse(args); err != nil {
				return 1
			}
			if fs.NArg() != 0 {
				fmt.Fprintln(c.Err, "lazyagents: doctor accepts no positional arguments")
				return 1
			}
			ctx, stop := signal.NotifyContext(c.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			report := RunDoctor(ctx, c, checks, !*local)
			if *jsonOut {
				enc := json.NewEncoder(c.Out)
				enc.SetIndent("", "  ")
				if err := enc.Encode(report); err != nil {
					fmt.Fprintln(c.Err, "lazyagents: writing diagnostics:", err)
					return 1
				}
			} else {
				fmt.Fprintln(c.Out, "=== detected agents ===")
				for _, ag := range report.Agents {
					status := "not installed"
					if ag.Installed {
						status = "installed"
					}
					fmt.Fprintf(c.Out, "  %-20s %s\n", ag.ID, status)
				}
				count := 0
				for _, section := range report.Sections {
					fmt.Fprintf(c.Out, "\n=== %s ===\n%s\n", section.Title, section.Report)
					for _, p := range section.Findings {
						fmt.Fprintf(c.Out, "  Next (%s): %s\n", p.Resource, p.NextStep)
					}
					count += len(section.Problems)
				}
				switch {
				case report.Cancelled:
					fmt.Fprintln(c.Out, "\ndiagnostics cancelled")
				case count > 0:
					fmt.Fprintf(c.Out, "\n%d problem(s) found\n", count)
				case *local:
					fmt.Fprintln(c.Out, "\nall OK (local checks only; external checks not run)")
				default:
					fmt.Fprintln(c.Out, "\nall OK")
				}
			}
			if !report.OK {
				return 1
			}
			return 0
		},
	}
}
