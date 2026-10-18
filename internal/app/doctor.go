package app

import (
	"context"
	"fmt"
	"io"

	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// Assemble the same feature checks for both frontends; modules never import app.
func (a *App) doctorChecks() []cli.Check {
	checks := []cli.Check{{Title: "configuration", Tab: "agents", NextStep: "Check config.yaml syntax and file access, then rerun local checks. Restart lazyagents after changing startup preferences.", Run: func(c cli.Context, out io.Writer) []cli.Problem {
		if _, err := core.ReadConfig(c.Paths.ConfigPath()); err != nil {
			// Parser errors can quote configuration input. Report only the affected path.
			message := "Configuration could not be read or parsed; startup defaults may be in use."
			fmt.Fprintln(out, message)
			return []cli.Problem{{Severity: "error", Resource: c.Paths.ConfigPath(), Explanation: message}}
		}
		fmt.Fprintln(out, "Configuration syntax OK (a missing config uses defaults).")
		return nil
	}}}
	for _, f := range a.features {
		if f.Checks != nil {
			checks = append(checks, f.Checks(a.Deps)...)
		}
	}
	return checks
}

func (a *App) doctorReport(ctx context.Context, external bool) cli.DoctorReport {
	return cli.RunDoctor(ctx, cli.Context{Paths: a.Deps.Paths, Agents: a.Deps.Agents}, a.doctorChecks(), external)
}
