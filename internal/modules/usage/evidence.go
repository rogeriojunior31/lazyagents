package usage

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
)

func costKind(t Total) string {
	if !t.Priced || t.Events == 0 {
		return "unavailable"
	}
	if t.Recorded > 0 && t.Estimated > 0 {
		return "mixed"
	}
	if t.Estimated > 0 {
		return "estimated"
	}
	return "recorded"
}
func costEvidenceText(t Total) string {
	text := fmt.Sprintf("Cost: %d recorded, %d estimated, %d unavailable responses; USD", t.Recorded, t.Estimated, t.Unpriced)
	if t.MissingUsage > 0 {
		text += fmt.Sprintf(" · %d responses have incomplete token counters", t.MissingUsage)
	}
	if len(t.Reasons) > 0 {
		text += "\nUnavailable: " + strings.Join(t.Reasons, "; ")
	}
	var sources []string
	for _, s := range t.Sources {
		label := s.Source
		if s.Date != "" {
			label += " (" + s.Date + ")"
		}
		if !slices.Contains(sources, label) {
			sources = append(sources, label)
		}
	}
	if len(sources) > 0 {
		shown := sources[:min(4, len(sources))]
		text += "\nSources: " + strings.Join(shown, "; ")
		if len(sources) > len(shown) {
			text += fmt.Sprintf("; +%d more (--json)", len(sources)-len(shown))
		}
	}
	return text
}

type HistoryEvidence struct {
	ReadAt       time.Time `json:"read_at"`
	LastResponse time.Time `json:"last_response,omitempty"`
	Sessions     int       `json:"sessions"`
	WithoutUsage int       `json:"sessions_without_usage"`
	Unavailable  int       `json:"unavailable_sessions"`
	Unsupported  int       `json:"unsupported_sessions"`
	PriceWarning string    `json:"price_warning,omitempty"`
}

func historyText(h HistoryEvidence) string {
	text := "Local history"
	if !h.ReadAt.IsZero() {
		text += " read " + h.ReadAt.Local().Format("2006-01-02 15:04")
	}
	if !h.LastResponse.IsZero() {
		text += " · latest response " + h.LastResponse.Local().Format("2006-01-02 15:04")
	}
	text += fmt.Sprintf(" · %d unavailable, %d without usage, %d unsupported sessions", h.Unavailable, h.WithoutUsage, h.Unsupported)
	if h.Unavailable+h.WithoutUsage+h.Unsupported > 0 {
		text += "\nPartial history: totals cover observed responses only"
	}
	if h.PriceWarning != "" {
		text += "\n" + h.PriceWarning
	}
	return text
}
func lastResponse(events []agent.UsageEvent) time.Time {
	var latest time.Time
	for _, e := range events {
		at := e.Time
		if e.LastTime.After(at) {
			at = e.LastTime
		}
		if at.After(latest) {
			latest = at
		}
	}
	return latest
}
