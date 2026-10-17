package skills

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
)

type inventoryFilters struct{ location, agent, metadata, updates string }

type filterChoice struct{ value, label string }

func (f *inventoryFilters) field(row int) *string {
	switch row {
	case 0:
		return &f.location
	case 1:
		return &f.agent
	case 2:
		return &f.metadata
	default:
		return &f.updates
	}
}

func (m Tab) filterChoices(row int) []filterChoice {
	switch row {
	case 0:
		return []filterChoice{{"", "Library and local"}, {"library", "Library only"}, {"local", "Local only"}}
	case 1:
		choices := []filterChoice{{"", "Any activation state"}, {"*", "Enabled anywhere"}, {"-", "Nowhere enabled"}}
		for _, ag := range m.targets {
			choices = append(choices, filterChoice{ag.ID, "Enabled in " + ag.Name})
		}
		return choices
	case 2:
		return []filterChoice{{"", "All metadata"}, {"invalid", "Invalid / lint"}}
	default:
		return []filterChoice{{"", "All update states"}, {"available", "Update available"}, {"edited", "Locally edited"}, {"current", "Up to date"}, {"unknown", "Unchecked / unknown"}}
	}
}

func (m *Tab) matchesFilters(it skillItem) bool {
	sk, f := it.s, m.filters
	if f.location == "library" && !sk.InLibrary || f.location == "local" && sk.InLibrary {
		return false
	}
	switch f.agent {
	case "":
	case "*":
		if sk.EnabledCount() == 0 {
			return false
		}
	case "-":
		if sk.EnabledCount() != 0 {
			return false
		}
	default:
		if !sk.States[f.agent].On {
			return false
		}
	}
	if f.metadata == "invalid" && sk.Valid && len(it.issues) == 0 {
		return false
	}
	status := m.updateStatus[sk.Dir]
	switch f.updates {
	case "available":
		return status == UpdateStatusAvailable
	case "edited":
		return status == UpdateStatusLocallyEdited
	case "current":
		return status == UpdateStatusUpToDate
	case "unknown":
		return status == UpdateStatusUnknown
	}
	return true
}

func (m Tab) selection() (string, int) {
	sk, _ := m.selected()
	return sk.Dir, m.list.Index()
}

// Filter the cached scan; lint and source checks never run here.
func (m *Tab) applyFilters(dir string, index int) {
	items := make([]list.Item, 0, len(m.inventory))
	for _, it := range m.inventory {
		if m.matchesFilters(it) {
			items = append(items, it)
		}
	}
	_ = m.list.SetItems(items)
	m.syncSearch(dir, index)
}

// SetFilterText matches synchronously in memory. No stale async result can
// replace the current projection or leave an actionable row with old detail.
func (m *Tab) syncSearch(dir string, index int) {
	state, input := m.list.FilterState(), m.list.FilterInput
	if state != list.Unfiltered {
		m.list.SetFilterText(input.Value())
		m.list.SetFilterState(state)
		m.list.FilterInput = input // keep the caret and editing focus
	}
	visible := m.list.VisibleItems()
	index = max(0, min(index, len(visible)-1))
	for i, it := range visible {
		if it.(skillItem).s.Dir == dir {
			index = i
			break
		}
	}
	m.list.Select(index)
	m.reconcileMarks()
	m.layout()
}

func (m Tab) updateSearch(msg tea.Msg) (Tab, tea.Cmd) {
	dir, index := m.selection()
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "esc":
			m.list.ResetFilter()
			m.syncSearch(dir, index)
			return m, nil
		case "enter":
			if m.list.FilterValue() == "" {
				m.list.ResetFilter()
			} else {
				m.list.SetFilterState(list.FilterApplied)
				m.list.FilterInput.Blur()
			}
			m.syncSearch(dir, index)
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.list.FilterInput, cmd = m.list.FilterInput.Update(msg)
	m.syncSearch(dir, index)
	return m, cmd
}

func (m Tab) updateFilters(msg tea.KeyPressMsg) (Tab, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = skModeList
	case "up", "k":
		m.filterRow = max(0, m.filterRow-1)
	case "down", "j":
		m.filterRow = min(3, m.filterRow+1)
	case "r":
		m.filterDraft = inventoryFilters{}
	case "enter":
		dir, index := m.selection()
		m.filters, m.mode = m.filterDraft, skModeList
		m.applyFilters(dir, index)
	case "left", "h", "right", "l", "space":
		choices := m.filterChoices(m.filterRow)
		value := m.filterDraft.field(m.filterRow)
		i := 0
		for n, c := range choices {
			if c.value == *value {
				i = n
				break
			}
		}
		delta := 1
		if msg.String() == "left" || msg.String() == "h" {
			delta = -1
		}
		*value = choices[(i+delta+len(choices))%len(choices)].value
	}
	return m, nil
}

func (m Tab) filterSummary() string {
	var labels []string
	for row := range 4 {
		value := *m.filters.field(row)
		if value == "" {
			continue
		}
		label := value
		for _, c := range m.filterChoices(row) {
			if c.value == value {
				label = c.label
				break
			}
		}
		labels = append(labels, label)
	}
	return strings.Join(labels, " · ")
}

func (m Tab) filtersView() string {
	panel := components.Panel{Title: "Filter skills", Focused: true, Width: min(m.width, 84)}
	inner := panel.ContentWidth()
	footer := kit.Wrap("↑↓ field · ←→ value\nenter apply · r reset states · esc cancel", inner, "")
	lines := []string{ansi.Truncate("Search: "+m.list.FilterValue(), inner, "…")}
	note := ""
	if m.height >= 18 {
		note = kit.Wrap(fmt.Sprintf("%d cached update checks; U checks updates separately. Filters only change visible rows. A, U and profiles keep their full inventory scope.", len(m.updateStatus)), inner, "")
	}
	rows := max(1, min(4, m.height-4-lipgloss.Height(footer)-lipgloss.Height(note)))
	start, end := kit.Window(m.filterRow, 4, rows)
	names := []string{"Location", "Agent", "Metadata", "Updates"}
	for row := start; row < end; row++ {
		value := *m.filterDraft.field(row)
		label := value
		for _, c := range m.filterChoices(row) {
			if c.value == value {
				label = c.label
				break
			}
		}
		prefix := "  "
		if row == m.filterRow {
			prefix = "▸ "
		}
		line := ansi.Truncate(fmt.Sprintf("%s%-9s %s", prefix, names[row], label), inner, "…")
		if row == m.filterRow {
			line = kit.StTitle.Render(line)
		}
		lines = append(lines, line)
	}
	if note != "" {
		lines = append(lines, note)
	}
	lines = append(lines, footer)
	return panel.Render(strings.Join(lines, "\n"))
}
