package skills

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
)

type installFlowDoneMsg struct{ out installResult }

func (m Tab) discoverCmd(source string) tea.Cmd {
	svc, op := m.svc, m.sourceOp
	return m.sourceCommand(func(ctx context.Context, notify func(string)) tea.Msg {
		progress(notify, "Discovering entries in "+source)
		found, origin, cleanup, err := svc.discoverProgress(ctx, source, notify)
		if err != nil && cleanup != "" {
			_ = os.RemoveAll(cleanup)
			cleanup = ""
		}
		ids, prefErr := svc.installTargets()
		if prefErr != nil {
			origin.Notes = append(origin.Notes, fmt.Sprintf("Could not load install destinations; using library only: %s", prefErr))
		}
		if op != nil && cleanup != "" {
			root := cleanup
			op.own(func() { _ = os.RemoveAll(root) })
		}
		return discoverMsg{found: found, origin: origin, cleanup: cleanup, err: err, targets: ids, op: op}
	})
}

func (m *Tab) openInstallPicker(msg discoverMsg) {
	m.picker = newPicker(msg.found, msg.origin, msg.cleanup)
	m.picker.fromSources = msg.repository != nil
	m.picker.agents = slices.Clone(m.agents)
	for i := range m.picker.agents {
		m.picker.agents[i].ReadDirs = slices.Clone(m.picker.agents[i].ReadDirs)
	}
	for _, id := range msg.targets {
		if slices.ContainsFunc(skillCapableAgents(m.picker.agents), func(ag agent.Agent) bool { return ag.ID == id }) {
			if !slices.Contains(m.picker.ids, id) {
				m.picker.ids = append(m.picker.ids, id)
			}
		} else {
			m.picker.origin.Notes = append(m.picker.origin.Notes, fmt.Sprintf("Saved destination %q is unavailable; it is not selected for this install", id))
		}
	}
	m.mode, m.toast = skModePick, ""
}

func (m Tab) updateInstall(msg tea.KeyPressMsg) (Tab, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = skModeList
		m.input.Blur()
		return m, nil
	case "enter":
		src := strings.TrimSpace(m.input.Value())
		if src == "" {
			return m, nil
		}
		m.input.Blur()
		m.mode = skModeInstallBusy
		spin := m.beginSource(fmt.Sprintf("Looking for skills in %s…", src), skModeInstallBusy, skModeList)
		return m, tea.Batch(spin, m.discoverCmd(src))
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Tab) updatePick(msg tea.KeyPressMsg) (Tab, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.picker.destinations {
			m.picker.destinations = false
			return m, nil
		}
		m.mode = skModeList
		if m.picker.fromSources {
			m.mode = skModeSources
		}
		cleanup := m.picker.cleanup
		m.picker.cleanup = ""
		return m, func() tea.Msg {
			if cleanup != "" {
				_ = os.RemoveAll(cleanup)
			}
			return nil
		}
	case "enter":
		chosen := m.picker.chosen()
		if len(chosen) == 0 {
			m.setToast("no skill or hook selected (space marks)", true)
			return m, nil
		}
		if hasSkillCollisions(chosen) {
			return m, nil
		}
		if !m.picker.destinations {
			m.picker.destinations = true
			m.picker.targetCursor = 0
			if m.picker.chosenSkills() == 0 {
				m.picker.ids = []string{}
			}
			return m, nil
		}
		lines := []string{fmt.Sprintf("Install %d skill(s) and import %d hook(s)?", m.picker.chosenSkills(), len(chosen)-m.picker.chosenSkills())}
		if len(m.picker.ids) == 0 {
			lines = append(lines, "Library only: create no activation links. Agents reading the library directly still see its skills.")
		} else {
			lines = append(lines, "Activate newly installed skills in: "+m.agentLabels(m.picker.ids), "Other activation states are preserved where native readers allow it; shared links may move. Local/foreign copies are preserved.")
		}
		lines = append(lines, "Existing library entries are not overwritten or activated by this install. Hooks are imported only; enable them separately from Hooks.")
		if m.picker.origin.ExactPath {
			lines = append(lines, fmt.Sprintf("Source: %s\nRef: %s\nObserved commit: %s", m.picker.origin.Source, m.picker.origin.Ref, m.picker.origin.Revision))
		}
		for _, f := range chosen {
			kind := "skill"
			if f.Hook != nil {
				kind = "hook"
			}
			label := f.Name
			if m.picker.origin.ExactPath {
				label = fmt.Sprintf("%s (%s)", f.Name, f.Rel)
			}
			lines = append(lines, fmt.Sprintf("- %s: %s", kind, label))
		}
		m.confirm = components.NewConfirm(strings.Join(lines, "\n"))
		m.ckind, m.mode = confirmKindInstall, skModeConfirm
		return m, nil
	case "tab":
		if m.picker.destinations {
			m.picker.destinations = false
		} else if len(m.picker.chosen()) > 0 && !hasSkillCollisions(m.picker.chosen()) {
			m.picker.destinations = true
			m.picker.targetCursor = 0
			if m.picker.chosenSkills() == 0 {
				m.picker.ids = []string{}
			}
		}
		return m, nil
	default:
		m.picker = m.picker.update(msg, m.width, m.height)
		return m, nil
	}
}

func (m Tab) updateRegistrySearch(msg tea.KeyPressMsg) (Tab, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = skModeList
		m.input.Blur()
		return m, nil
	case "enter":
		term := strings.TrimSpace(m.input.Value())
		if term == "" {
			return m, nil
		}
		m.input.Blur()
		m.mode = skModeInstallBusy
		spin := m.beginSource(fmt.Sprintf("Searching GitHub for %q…", term), skModeInstallBusy, skModeRegistry)
		svc, op := m.svc, m.sourceOp
		return m, tea.Batch(spin, op.command(func(ctx context.Context, notify func(string)) tea.Msg {
			results, err := svc.SearchRegistryContext(ctx, term)
			return registrySearchMsg{results: results, err: err, op: op}
		}))
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Tab) updateRegistryPick(msg tea.KeyPressMsg) (Tab, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = skModeList
		return m, nil
	case "enter":
		sel, ok := m.regPicker.selected()
		if !ok {
			return m, nil
		}
		m.mode = skModeInstallBusy
		spin := m.beginSource(fmt.Sprintf("Looking for skills in %s…", sel.Repo), skModeInstallBusy, skModeRegistryPick)
		return m, tea.Batch(spin, m.discoverCmd(sel.Repo))
	default:
		m.regPicker.update(msg)
		return m, nil
	}
}

func (m *Tab) finishInstall(out installResult) {
	m.inFlight, m.mode, m.toast = false, skModeList, ""
	svc := m.svc
	retryTargets := []string{}
	for _, sk := range out.activation.skills {
		for _, id := range out.activation.ids {
			retryTargets = append(retryTargets, batchTarget(sk.Dir, id))
		}
	}
	if out.remember {
		retryTargets = append(retryTargets, installSettingsTarget)
	}
	m.report = &components.OperationReport{
		Title: "Install and activation results", Result: out.result, RetryTargets: retryTargets,
		Recovery: "Install/import failures require fixing the source or collision and starting a new install. Retry repeats only failed activation pairs or preference saves; installed files are kept. B opens skill backups.",
		Recover:  func() tea.Msg { return openBackupsMsg{} },
		Retry:    func(ids []string) core.OperationResult { return svc.retryInstall(out, ids) },
	}
}
