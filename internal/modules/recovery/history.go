package recovery

import (
	"fmt"
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/core"
)

func (s *Service) historyEntries() ([]core.HistoryEntry, error) { return core.ReadHistory(s.paths) }
func (s *Service) clearHistory() error                          { return core.ClearHistory(s.paths) }

// HistoryInspection provides navigation hints, never a saved executable plan.
type HistoryInspection struct {
	core.HistoryEntry
	Next    string                `json:"next"`
	Backups []HistoryBackupStatus `json:"backup_status"`
}
type HistoryBackupStatus struct {
	core.HistoryBackup
	Available bool `json:"available"`
}

func (s *Service) inspectHistory(id string) (HistoryInspection, error) {
	entries, err := s.historyEntries()
	if err != nil {
		return HistoryInspection{}, err
	}
	for _, e := range entries {
		if e.ID != id {
			continue
		}
		p := HistoryInspection{HistoryEntry: e, Next: historyNext(e), Backups: []HistoryBackupStatus{}}
		for _, b := range e.Backups {
			status := HistoryBackupStatus{HistoryBackup: b}
			if b.Kind == "config" {
				preview, err := s.Preview(b.ID)
				status.Available = err == nil && preview.Agent == b.Target
			}
			p.Backups = append(p.Backups, status)
		}
		return p, nil
	}
	return HistoryInspection{}, fmt.Errorf("operation is no longer retained; reload history")
}

func historyNext(e core.HistoryEntry) string {
	switch e.Module {
	case "providers":
		return "Open Providers or run provider list; inspect current state and review apply/clear again. Select only failed targets after fixing the cause."
	case "hooks":
		return "Open Hooks or run hooks list; inspect current commands and targets, then review the intended change again. Never replay a command from history."
	case "skills":
		return "Open Skills or run skills list; inspect current library and visibility. Review only failed targets again. Updates need a fresh source check; skills backups lists archives."
	}
	return "Open Recovery or run config-backups list; preview an available snapshot against the current file before confirming a restore."
}

func historySummary(e core.HistoryEntry) string {
	good, bad, skipped := 0, 0, 0
	for _, t := range e.Targets {
		switch t.Outcome {
		case core.Succeeded:
			good++
		case core.Failed:
			bad++
		case core.Skipped:
			skipped++
		}
	}
	return fmt.Sprintf("%d succeeded · %d failed · %d skipped", good, bad, skipped)
}

func historyText(p HistoryInspection) string {
	e := p.HistoryEntry
	lines := []string{e.Module + " / " + e.Action, e.Time.Local().Format("2006-01-02 15:04:05")}
	if e.Subject != "" {
		lines = append(lines, "Resource: "+e.Subject)
	}
	lines = append(lines, historySummary(e))
	for _, t := range e.Targets {
		lines = append(lines, t.ID+": "+string(t.Outcome))
	}
	if e.Failed {
		lines = append(lines, "A failure may include partial writes; inspect current state.")
	}
	lines = append(lines, "", "Attempt: "+e.ID)
	if e.Omitted > 0 {
		lines = append(lines, fmt.Sprintf("%d additional targets omitted by the metadata limit", e.Omitted))
	}
	lines = append(lines, "", "Related backups (time, action and target; preview required):")
	if len(p.Backups) == 0 {
		lines = append(lines, "No matching backup recorded. A new file or activation link has no previous config snapshot.")
	}
	for i, b := range p.Backups {
		state := "unavailable for guided config restore"
		if b.Available {
			state = "ready for a fresh preview"
		}
		if b.Kind == "skill" {
			state = "inspect in Skills (B) or skills backups"
		}
		lines = append(lines, fmt.Sprintf("%d. %s: %s", i+1, b.Target, state), b.ID)
	}
	lines = append(lines, "", p.Next, "", "History records outcomes, not current state or permission to repeat a write. Existing operation reports can retry failed targets; after restart always inspect and review again. Clearing history keeps every backup.")
	return strings.Join(lines, "\n")
}
