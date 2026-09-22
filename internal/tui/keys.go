package tui

import "charm.land/bubbles/v2/key"

type keyMap struct {
	NextTab key.Binding
	PrevTab key.Binding
	Help    key.Binding
	Palette key.Binding
	Quit    key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		NextTab: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "próxima aba")),
		PrevTab: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "aba anterior")),
		Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "ajuda")),
		Palette: key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "comandos")),
		Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "sair")),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Palette, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.NextTab, k.PrevTab}, {k.Help, k.Palette, k.Quit}}
}
