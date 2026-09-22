package app

import (
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// O registro precisa gerar abas e comandos sem nome duplicado.
func TestFeaturesRegistry(t *testing.T) {
	d, err := LoadWith(core.PathsIn(t.TempDir()), "test")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, m := range d.Modules() {
		if seen["tab:"+m.ID()] {
			t.Errorf("aba duplicada %q", m.ID())
		}
		seen["tab:"+m.ID()] = true
	}
	for _, c := range d.Commands() {
		if seen[c.Name] {
			t.Errorf("comando duplicado %q", c.Name)
		}
		seen[c.Name] = true
	}
	for _, want := range []string{"tab:skills", "tab:sessions", "tab:agents", "list", "sessions", "doctor"} {
		if !seen[want] {
			t.Errorf("faltou %q no registro", want)
		}
	}
}
