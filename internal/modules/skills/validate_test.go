package skills

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		dir       string
		md        string
		wantField string // "" = nenhum issue esperado
		wantMsg   string // substring esperada na Msg do issue de wantField
	}{
		{
			name: "skill válida sem issues",
			dir:  "minha",
			md:   "---\nname: minha\ndescription: uma skill de teste\n---\n\ncorpo com instruções\n",
		},
		{
			name:      "name ausente no frontmatter",
			dir:       "minha",
			md:        "---\ndescription: uma skill de teste\n---\n\ncorpo\n",
			wantField: "name",
			wantMsg:   "missing",
		},
		{
			name:      "name não é kebab-case",
			dir:       "minha",
			md:        "---\nname: Minha_Skill\ndescription: uma skill de teste\n---\n\ncorpo\n",
			wantField: "name",
			wantMsg:   "kebab-case",
		},
		{
			name:      "name difere da pasta",
			dir:       "minha",
			md:        "---\nname: outra-coisa\ndescription: uma skill de teste\n---\n\ncorpo\n",
			wantField: "name",
			wantMsg:   "differs from the folder",
		},
		{
			name:      "description vazia",
			dir:       "minha",
			md:        "---\nname: minha\ndescription: \n---\n\ncorpo\n",
			wantField: "description",
			wantMsg:   "empty",
		},
		{
			name:      "description longa demais",
			dir:       "minha",
			md:        "---\nname: minha\ndescription: " + strings.Repeat("x", maxDescriptionLen+1) + "\n---\n\ncorpo\n",
			wantField: "description",
			wantMsg:   "over",
		},
		{
			name:      "corpo vazio após o frontmatter",
			dir:       "minha",
			md:        "---\nname: minha\ndescription: uma skill de teste\n---\n",
			wantField: "body",
			wantMsg:   "no instructions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeSkill(t, t.TempDir(), tt.dir, tt.md)
			sk := parseSkill(path, tt.dir)
			if !sk.Valid {
				t.Fatalf("setup: skill deveria parsear válida, warning=%q", sk.Warning)
			}
			issues := Validate(sk)
			if tt.wantField == "" {
				if len(issues) != 0 {
					t.Fatalf("esperava 0 issues, veio %+v", issues)
				}
				return
			}
			var found *Issue
			for i := range issues {
				if issues[i].Field == tt.wantField {
					found = &issues[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("esperava issue no campo %q, veio %+v", tt.wantField, issues)
			}
			if !strings.Contains(found.Msg, tt.wantMsg) {
				t.Fatalf("Msg = %q, esperava conter %q", found.Msg, tt.wantMsg)
			}
		})
	}
}

func TestValidateSkillInvalida(t *testing.T) {
	// frontmatter ilegível: sk.Valid == false, Validate não deve duplicar o Warning.
	path := writeSkill(t, t.TempDir(), "quebrada", "sem frontmatter nenhum\n")
	sk := parseSkill(path, "quebrada")
	if sk.Valid {
		t.Fatal("setup: esperava sk.Valid == false")
	}
	if issues := Validate(sk); issues != nil {
		t.Fatalf("esperava nil para skill inválida, veio %+v", issues)
	}
}
