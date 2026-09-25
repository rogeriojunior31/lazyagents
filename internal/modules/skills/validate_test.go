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
		wantField string // "" = no issue expected
		wantMsg   string // substring expected in the Msg of wantField's issue
	}{
		{
			name: "valid skill without issues",
			dir:  "mine",
			md:   "---\nname: mine\ndescription: a test skill\n---\n\nbody with instructions\n",
		},
		{
			name:      "name missing from frontmatter",
			dir:       "mine",
			md:        "---\ndescription: a test skill\n---\n\nbody\n",
			wantField: "name",
			wantMsg:   "missing",
		},
		{
			name:      "name is not kebab-case",
			dir:       "mine",
			md:        "---\nname: Minha_Skill\ndescription: a test skill\n---\n\nbody\n",
			wantField: "name",
			wantMsg:   "kebab-case",
		},
		{
			name:      "name differs from the folder",
			dir:       "mine",
			md:        "---\nname: outra-coisa\ndescription: a test skill\n---\n\nbody\n",
			wantField: "name",
			wantMsg:   "differs from the folder",
		},
		{
			name:      "empty description",
			dir:       "mine",
			md:        "---\nname: mine\ndescription: \n---\n\nbody\n",
			wantField: "description",
			wantMsg:   "empty",
		},
		{
			name:      "description too long",
			dir:       "mine",
			md:        "---\nname: mine\ndescription: " + strings.Repeat("x", maxDescriptionLen+1) + "\n---\n\nbody\n",
			wantField: "description",
			wantMsg:   "over",
		},
		{
			name:      "empty body after the frontmatter",
			dir:       "mine",
			md:        "---\nname: mine\ndescription: a test skill\n---\n",
			wantField: "body",
			wantMsg:   "no instructions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeSkill(t, t.TempDir(), tt.dir, tt.md)
			sk := parseSkill(path, tt.dir)
			if !sk.Valid {
				t.Fatalf("setup: skill should parse as valid, warning=%q", sk.Warning)
			}
			issues := Validate(sk)
			if tt.wantField == "" {
				if len(issues) != 0 {
					t.Fatalf("want 0 issues, got %+v", issues)
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
				t.Fatalf("want an issue on field %q, got %+v", tt.wantField, issues)
			}
			if !strings.Contains(found.Msg, tt.wantMsg) {
				t.Fatalf("Msg = %q, want it to contain %q", found.Msg, tt.wantMsg)
			}
		})
	}
}

func TestValidateInvalidSkill(t *testing.T) {
	// Unreadable frontmatter: sk.Valid == false; Validate must not repeat the Warning.
	path := writeSkill(t, t.TempDir(), "broken", "no frontmatter at all\n")
	sk := parseSkill(path, "broken")
	if sk.Valid {
		t.Fatal("setup: want sk.Valid == false")
	}
	if issues := Validate(sk); issues != nil {
		t.Fatalf("want nil for an invalid skill, got %+v", issues)
	}
}
