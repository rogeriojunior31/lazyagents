package skills

import (
	"fmt"
	"path/filepath"
	"slices"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

type installConfig struct {
	Targets []string `yaml:"installTargets"`
}

func (s *Service) installTargets() ([]string, error) {
	cfg, err := core.ReadConfig(s.paths.ConfigPath())
	if err != nil {
		return nil, err
	}
	var settings installConfig
	if err := cfg.Section("skills", &settings); err != nil {
		return nil, err
	}
	return settings.Targets, nil
}

func (s *Service) saveInstallTargets(ids []string) error {
	cfg, err := core.ReadConfig(s.paths.ConfigPath())
	if err != nil {
		return err
	}
	if ids == nil {
		ids = []string{} // library-only is an explicit empty list, never all agents
	}
	if err := cfg.SetSectionValue("skills", "installTargets", ids); err != nil {
		return err
	}
	return cfg.Save(s.paths.ConfigPath())
}

type installResult struct {
	result     core.OperationResult
	activation skillBatch
	remember   bool
}

const installSettingsTarget = "Install destinations / settings"

// Install each chosen entry once, then activate only newly copied skills.
// The retry plan contains no source files, hooks or copy operations.
func (s *Service) installWithTargets(chosen []Found, origin Origin, agents []agent.Agent, ids []string) installResult {
	out := installResult{activation: skillBatch{action: batchEnable, agents: agents, ids: slices.Clone(ids)}}
	for _, f := range chosen {
		stage := "install"
		if f.Hook != nil {
			stage = "import hook"
		} else {
			out.remember = true
		}
		names, err := s.Install([]Found{f}, origin)
		t := core.TargetResult{ID: batchTarget(f.Name, stage), Outcome: core.Succeeded, State: "installed in library"}
		if f.Hook != nil {
			t.State = "imported in hook library; not enabled in agents"
		}
		if err != nil {
			t.Outcome, t.Detail = core.Failed, err.Error()
			t.State = "not installed by this attempt"
			if len(names) > 0 {
				t.State = "library files copied; source metadata may be incomplete"
			}
		}
		out.result.Targets = append(out.result.Targets, t)
		if f.Hook != nil {
			continue // importing a hook never installs it in any agent
		}
		if !slices.Contains(names, f.Name) {
			out.result.Targets = append(out.result.Targets, core.TargetResult{ID: batchTarget(f.Name, "activation"), Outcome: core.Skipped, Detail: "installation failed; existing copies were not activated"})
			continue
		}
		if len(ids) == 0 {
			out.result.Targets = append(out.result.Targets, core.TargetResult{ID: batchTarget(f.Name, "activation"), Outcome: core.Skipped, Detail: "library only; no activation links requested"})
			continue
		}
		sk := parseSkill(filepath.Join(s.paths.LibraryDir(), f.Name), f.Name)
		sk.InLibrary = true
		out.activation.skills = append(out.activation.skills, sk)
	}
	activation := s.runBatch(out.activation, nil)
	out.result.Targets = append(out.result.Targets, activation.Targets...)
	out.result.Err = activation.Err
	if out.remember {
		out.result.Targets = append(out.result.Targets, s.rememberInstallTargets(ids))
	}
	return out
}

func (s *Service) rememberInstallTargets(ids []string) core.TargetResult {
	t := core.TargetResult{ID: installSettingsTarget, Outcome: core.Succeeded, State: "saved"}
	if err := s.saveInstallTargets(ids); err != nil {
		t.Outcome, t.Detail, t.State = core.Failed, fmt.Sprintf("Could not remember install destinations: %s", err), "not saved; installed skills are kept"
	}
	return t
}

func (s *Service) retryInstall(out installResult, ids []string) core.OperationResult {
	if len(ids) == 0 {
		return core.OperationResult{}
	}
	r := s.runBatch(out.activation, ids)
	if out.remember && slices.Contains(ids, installSettingsTarget) {
		r.Targets = append(r.Targets, s.rememberInstallTargets(out.activation.ids))
	}
	return r
}
