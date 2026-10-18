package tui

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rogeriojunior31/lazyagents/internal/tui/components"
	"github.com/rogeriojunior31/lazyagents/internal/tui/kit"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

// Preferences describes only editable presentation fields, never config bodies.
type Preferences struct {
	Theme, StartTab, Revision string
	Themes                    []string
	Tabs                      []PreferenceTab
}

type PreferenceTab struct {
	ID, Title   string
	Hidden      bool
	Unavailable bool
}

// PreferencesHelp is shared with the generated key reference.
var PreferencesHelp = module.HelpGroup{Title: "Preferences", Keys: [][2]string{
	{"↑/↓ · j/k", "select a preference or tab"},
	{"←/→", "cycle theme or visible initial tab"},
	{"space · enter", "cycle the selected preference or show/hide a tab"},
	{"s", "make the selected visible tab initial"},
	{"[ · ]", "move the selected tab earlier/later"},
	{"ctrl+s", "save for the next launch; keep the current session"},
	{"r", "reload saved preferences; confirm discarding a changed draft"},
	{"esc · q", "close; confirm discarding unsaved preferences"},
}}

type preferencesState struct {
	data            Preferences
	initial         Preferences
	discard         *components.Confirm
	reload          bool
	cursor          int
	loading, saving bool
	message         string
}

type preferencesResult struct {
	request *preferencesState
	data    Preferences
	err     error
	saved   bool
}

func (m *Model) openPreferences() tea.Cmd {
	p := &preferencesState{loading: true}
	m.preferences = p
	load := m.loadPreferences
	return func() tea.Msg {
		if load == nil {
			return preferencesResult{request: p, err: fmt.Errorf("Preferences are unavailable in this view.")}
		}
		data, err := load()
		return preferencesResult{request: p, data: data, err: err}
	}
}

func (m *Model) updatePreferences(msg tea.KeyPressMsg) tea.Cmd {
	p := m.preferences
	if p.saving {
		return nil
	}
	if p.discard != nil {
		confirm, answer := p.discard.Update(msg, bodyWidth(m.width), bodyHeight(m.height))
		p.discard = &confirm
		if answer == components.No {
			p.discard = nil
		}
		if answer == components.Yes {
			if p.reload {
				return m.openPreferences()
			}
			m.preferences = nil
		}
		return nil
	}
	if msg.String() == "esc" || msg.String() == "q" || msg.String() == "r" {
		reload := msg.String() == "r"
		if preferencesDirty(p) {
			c := components.NewConfirm("Discard unsaved preferences?\nChoose No to keep editing. Saved settings and this session stay unchanged.")
			p.discard = &c
			p.reload = reload
			return nil
		}
		if reload {
			return m.openPreferences()
		}
		m.preferences = nil
		return nil
	}
	if p.loading {
		return nil
	}

	if len(p.data.Themes) == 0 {
		return nil
	}
	p.message = ""
	switch msg.String() {
	case "up", "k":
		p.cursor = max(0, p.cursor-1)
	case "down", "j":
		p.cursor = min(len(p.data.Tabs)+1, p.cursor+1)
	case "ctrl+s":
		if m.savePreferences == nil {
			p.message = "Preferences cannot be saved in this view."
			return nil
		}
		p.saving = true
		draft := p.data
		draft.Tabs = slices.Clone(draft.Tabs)
		save := m.savePreferences
		return func() tea.Msg {
			data, err := save(draft)
			return preferencesResult{request: p, data: data, err: err, saved: true}
		}
	case "left", "right", "space", "enter":
		step := 1
		if msg.String() == "left" {
			step = -1
		}
		switch p.cursor {
		case 0:
			i := slices.Index(p.data.Themes, p.data.Theme)
			p.data.Theme = p.data.Themes[(i+step+len(p.data.Themes))%len(p.data.Themes)]
		case 1:
			var visible []string
			for _, tab := range p.data.Tabs {
				if !tab.Hidden && !tab.Unavailable {
					visible = append(visible, tab.ID)
				}
			}
			if len(visible) > 0 {
				i := slices.Index(visible, p.data.StartTab)
				p.data.StartTab = visible[(i+step+len(visible))%len(visible)]
			}
		default:
			if msg.String() != "enter" && msg.String() != "space" {
				break
			}
			tab := &p.data.Tabs[p.cursor-2]
			if !tab.Hidden {
				visible := 0
				for _, candidate := range p.data.Tabs {
					if !candidate.Hidden && !candidate.Unavailable {
						visible++
					}
				}
				if visible <= 1 && !tab.Unavailable {
					p.message = "Keep at least one available tab visible."
					return nil
				}
			}
			tab.Hidden = !tab.Hidden
			if tab.Hidden && tab.ID == p.data.StartTab {
				for _, candidate := range p.data.Tabs {
					if !candidate.Hidden && !candidate.Unavailable {
						p.data.StartTab = candidate.ID
						break
					}
				}
			}
		}
	case "s":
		if p.cursor >= 2 {
			tab := p.data.Tabs[p.cursor-2]
			if tab.Hidden || tab.Unavailable {
				p.message = "Choose a visible, available initial tab."
			} else {
				p.data.StartTab = tab.ID
			}
		}
	case "[", "]":
		if p.cursor >= 2 {
			i := p.cursor - 2
			next := i - 1
			if msg.String() == "]" {
				next = i + 1
			}
			if next >= 0 && next < len(p.data.Tabs) {
				p.data.Tabs[i], p.data.Tabs[next] = p.data.Tabs[next], p.data.Tabs[i]
				p.cursor = next + 2
			}
		}
	}
	return nil
}

func preferencesText(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return '�'
		}
		return r
	}, value)
}

func (m Model) renderPreferences() string {
	p := m.preferences
	w, h := min(bodyWidth(m.width), 86), min(bodyHeight(m.height), 24)
	if p.discard != nil {
		return p.discard.ViewIn(w, h)
	}
	panel := components.Panel{Title: "Preferences", Focused: true, Width: w, Height: h}
	inner := panel.ContentWidth()
	if p.loading {
		return panel.Render("Reading preferences…\n\nesc closes")
	}
	if len(p.data.Themes) == 0 {
		return panel.Render(kit.Wrap(preferencesText(p.message), inner, "") + "\n\nesc closes; r reloads")
	}
	rows := []string{fmt.Sprintf("Theme: %s", preferencesText(p.data.Theme)), fmt.Sprintf("Initial tab: %s", preferencesText(p.data.StartTab))}
	for _, tab := range p.data.Tabs {
		mark := "shown"
		if tab.Hidden {
			mark = "hidden"
		}
		initial := ""
		if tab.ID == p.data.StartTab {
			initial = "; initial"
		}
		rows = append(rows, fmt.Sprintf("%s (%s%s)", preferencesText(tab.Title), mark, initial))
	}
	help := fmt.Sprintf("←→ change; running theme: %s", theme.Current())
	if p.cursor == 1 {
		help = "←→ initial tab; ↑↓ select"
	} else if p.cursor >= 2 {
		help = "space show/hide; s initial; [ ] order"
	}
	footer := kit.Wrap(help, inner, "") + "\n" + kit.Wrap("ctrl+s save; r reload; esc discard", inner, "")
	if p.message != "" {
		message := strings.Split(kit.Wrap(preferencesText(p.message), inner, ""), "\n")
		if len(message) > 3 {
			message = message[:3]
			message[2] = ansi.Truncate(message[2], max(1, inner-1), "") + "…"
		}
		footer = strings.Join(message, "\n") + "\n" + footer
	}
	if p.saving {
		footer = "Saving preferences…"
	}
	head := "Applies next launch"
	room := max(1, panel.ContentHeight()-1-len(strings.Split(footer, "\n")))
	start, end := kit.Window(p.cursor, len(rows), room)
	lines := []string{kit.StHint.Render(head)}
	for i := start; i < end; i++ {
		line := "  " + rows[i]
		if i == p.cursor {
			line = "› " + rows[i]
		}
		line = ansi.Truncate(line, inner, "…")
		if i == p.cursor {
			line = kit.StTitle.Render(line)
		}
		lines = append(lines, line)
	}
	return panel.Render(kit.Frame(strings.Join(lines, "\n"), footer, panel.ContentHeight()))
}

func clonePreferences(p Preferences) Preferences { p.Tabs = slices.Clone(p.Tabs); return p }
func preferencesDirty(p *preferencesState) bool {
	return p.data.Theme != p.initial.Theme || p.data.StartTab != p.initial.StartTab || !slices.Equal(p.data.Tabs, p.initial.Tabs)
}
