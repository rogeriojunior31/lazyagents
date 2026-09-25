// Package module is the contract of a TUI tab. A new tab implements Module and
// is registered in internal/app; tui/app.go is never edited for it.
package module

import tea "charm.land/bubbletea/v2"

// Module is a tab. Pointer semantics: Update mutates the receiver and returns
// only the Cmd, so the root keeps []Module without the concrete type.
type Module interface {
	ID() string    // stable slug: "skills", "sessions"… (palette name)
	Title() string // tab label without the counter
	Count() int    // pill counter; -1 = none
	Init() tea.Cmd
	// Update always gets WindowSizeMsg and broadcasts (events.*); input (keys,
	// mouse, paste, events.Reload) only while active.
	Update(msg tea.Msg) tea.Cmd
	View() string    // body only; the root draws header, tabs and footer
	Capturing() bool // true while an input, filter or modal owns the keyboard
	ClearToast()     // called when switching tabs
	Help() []HelpGroup
}

// HelpGroup is a block of the help modal (?): title + {key, description}.
type HelpGroup struct {
	Title string
	Keys  [][2]string
}

// Commander is optional: the module's own palette commands. On choice the root
// activates the module and delivers Msg to its Update.
type Commander interface {
	Commands() []Command
}

// Command is a palette entry contributed by a module.
type Command struct {
	Name string
	Desc string
	Msg  tea.Msg
}
