package theme

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBuiltinThemesAreComplete(t *testing.T) {
	var ids []string
	for _, p := range Options() {
		ids = append(ids, p.ID)
		for _, g := range Schema {
			for _, r := range g.Roles {
				if !hexRe.MatchString(p.Roles[g.Group+"."+r]) {
					t.Errorf("%s: %s.%s sem cor", p.ID, g.Group, r)
				}
			}
		}
		for name, role := range tokenRoles {
			if p.Colors[name] == "" || p.Colors[name] != p.Roles[role] {
				t.Errorf("%s: token %s não resolve para %s", p.ID, name, role)
			}
		}
		if p.Appearance != "dark" && p.Appearance != "light" {
			t.Errorf("%s: appearance %q", p.ID, p.Appearance)
		}
	}
	want := []string{"noite", "garoa", "jaragua", "dracula", "everforest-dark", "gruvbox-dark", "kanagawa",
		"nord", "onedark", "rose-pine", "rose-pine-dawn", "rose-pine-moon", "tokyonight", "tokyonight-storm"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("ordem = %v", ids)
	}
}

// Os sabores SP Night carregam a paleta crua inteira e os papéis do upstream.
func TestSPNightKeepsOriginalPalette(t *testing.T) {
	p, _ := lookup("noite")
	if len(p.Named) != 23 || p.Named["sodio"] != "#f2984a" || p.Named["estaiada"] != "#38b59e" ||
		p.Named["temporal_vivo"] != "#c3a7f6" {
		t.Fatalf("paleta noite incompleta: %v", p.Named)
	}
	for role, want := range map[string]string{
		"ui.accent": "#f2984a", "syntax.macro": "#38b59e", "diagnostic.hint": "#5dbec4",
		"ansi.bright_magenta": "#c3a7f6", "git.untracked": "#707380",
	} {
		if p.Roles[role] != want {
			t.Errorf("%s = %s, quer %s", role, p.Roles[role], want)
		}
	}
}

func writeThemes(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLoadUser(t *testing.T) {
	defer func() { LoadUser(t.TempDir()); Apply(Default) }()
	dir := writeThemes(t, map[string]string{
		"meu.yaml":      "label: Meu\nui:\n  accent: \"#ff0000\"\n",
		"filho.yml":     "extends: meu\npalette:\n  sodio: \"#00ff00\"\n",
		"quente.yaml":   "extends: gruvbox-dark\nappearance: light\n",
		"ruim.yaml":     "ui:\n  bg: \"#12345\"\n",
		"semaspas.yaml": "ui:\n  bg: #123456\n",
		"orfao.yaml":    "extends: nao-existe\n",
		"a.yaml":        "extends: b\n",
		"b.yaml":        "extends: a\n",
		"noite.yaml":    "label: sequestro\n",
		"outro.yaml":    "id: diferente\n",
		"papel.yaml":    "ui:\n  acent: \"#ffffff\"\n",
		"ref.yaml":      "ui:\n  bg: inexistente\n",
		"chave.yaml":    "cores: {}\n",
		"leiame.txt":    "ignorado",
	})
	errs := LoadUser(dir)
	for _, bad := range []string{"ruim", "semaspas", "orfao", "a", "b", "noite", "outro", "papel", "ref", "chave"} {
		found := false
		for _, err := range errs {
			if strings.Contains(err.Error(), filepath.Join(dir, bad)+".yaml") {
				found = true
			}
		}
		if !found {
			t.Errorf("sem erro para %s: %v", bad, errs)
		}
	}
	if len(errs) != 10 {
		t.Errorf("erros = %d: %v", len(errs), errs)
	}

	var ids []string
	for _, p := range Options() {
		if p.User {
			ids = append(ids, p.ID)
		}
	}
	if !reflect.DeepEqual(ids, []string{"filho", "meu", "quente"}) {
		t.Fatalf("temas do usuário = %v", ids)
	}
	meu, _ := lookup("meu")
	noite, _ := lookup("noite")
	if meu.Label != "Meu" || meu.Roles["ui.accent"] != "#ff0000" || meu.Roles["ui.bg"] != noite.Roles["ui.bg"] {
		t.Fatalf("meu não herdou de noite: %v", meu.Roles)
	}
	// sobrescrever uma cor da paleta repinta os papéis herdados que a usam
	filho, _ := lookup("filho")
	if filho.Roles["ui.cursor"] != "#00ff00" || filho.Roles["ui.accent"] != "#ff0000" {
		t.Fatalf("filho: cursor=%s accent=%s", filho.Roles["ui.cursor"], filho.Roles["ui.accent"])
	}
	if q, _ := lookup("quente"); q.Appearance != "light" || q.Roles["ui.bg"] != "#282828" {
		t.Fatalf("quente: %v %s", q.Appearance, q.Roles["ui.bg"])
	}
	if noite.Label != "Noite Paulista" {
		t.Fatal("arquivo do usuário substituiu tema embutido")
	}
	if err := Apply("filho"); err != nil {
		t.Fatal(err)
	}

	// nova carga substitui a anterior
	LoadUser(writeThemes(t, map[string]string{"so.yaml": "label: Só\n"}))
	if _, ok := lookup("meu"); ok {
		t.Fatal("tema antigo do usuário continuou registrado")
	}
	if errs := LoadUser(filepath.Join(dir, "nao-existe")); len(errs) != 0 {
		t.Fatalf("dir ausente: %v", errs)
	}
}
