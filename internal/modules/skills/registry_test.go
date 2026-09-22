package skills

import (
	"context"
	"errors"
	"testing"
)

func TestParseSearchCodeResponse(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    []RegistryResult
		wantErr bool
	}{
		{
			name: "dedupe por repo e ordena",
			json: `{"items":[
				{"path":"skills/b/SKILL.md","html_url":"https://github.com/z/b/blob/main/skills/b/SKILL.md","repository":{"full_name":"z/repo-b","description":"skill b"}},
				{"path":"skills/a/SKILL.md","html_url":"https://github.com/a/repo-a/blob/main/skills/a/SKILL.md","repository":{"full_name":"a/repo-a","description":"skill a"}},
				{"path":"skills/b2/SKILL.md","html_url":"https://github.com/z/b/blob/main/skills/b2/SKILL.md","repository":{"full_name":"z/repo-b","description":"skill b"}}
			]}`,
			want: []RegistryResult{
				{Repo: "a/repo-a", Path: "skills/a/SKILL.md", Description: "skill a", URL: "https://github.com/a/repo-a/blob/main/skills/a/SKILL.md"},
				{Repo: "z/repo-b", Path: "skills/b/SKILL.md", Description: "skill b", URL: "https://github.com/z/b/blob/main/skills/b/SKILL.md"},
			},
		},
		{
			name: "sem itens",
			json: `{"items":[]}`,
			want: nil,
		},
		{
			name:    "json inválido",
			json:    `não é json`,
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseSearchCodeResponse([]byte(tc.json))
			if tc.wantErr {
				if err == nil {
					t.Fatal("esperava erro, não teve")
				}
				return
			}
			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %d resultados, quer %d: %+v", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("resultado[%d] = %+v, quer %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestSearchRegistry(t *testing.T) {
	svc := New(testPaths(t))

	t.Run("termo vazio não chama a rede", func(t *testing.T) {
		called := false
		restore := stubSearchCodeRunner(t, func(context.Context, string) ([]byte, error) {
			called = true
			return nil, nil
		})
		defer restore()
		if _, err := svc.SearchRegistry("   "); err == nil {
			t.Fatal("esperava erro para termo vazio")
		}
		if called {
			t.Error("não deveria ter chamado o runner com termo vazio")
		}
	})

	t.Run("propaga erro de rede sem tocar a biblioteca", func(t *testing.T) {
		restore := stubSearchCodeRunner(t, func(context.Context, string) ([]byte, error) {
			return nil, errors.New("sem rede")
		})
		defer restore()
		if _, err := svc.SearchRegistry("react"); err == nil {
			t.Fatal("esperava erro de rede")
		}
	})

	t.Run("sem resultados vira erro amigável", func(t *testing.T) {
		restore := stubSearchCodeRunner(t, func(context.Context, string) ([]byte, error) {
			return []byte(`{"items":[]}`), nil
		})
		defer restore()
		if _, err := svc.SearchRegistry("termo-inexistente"); err == nil {
			t.Fatal("esperava erro para busca sem resultado")
		}
	})

	t.Run("resultado ok repassa o termo pro runner", func(t *testing.T) {
		var gotTerm string
		restore := stubSearchCodeRunner(t, func(_ context.Context, term string) ([]byte, error) {
			gotTerm = term
			return []byte(`{"items":[{"path":"SKILL.md","html_url":"https://github.com/x/y","repository":{"full_name":"x/y","description":"desc"}}]}`), nil
		})
		defer restore()
		got, err := svc.SearchRegistry("react native")
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if gotTerm != "react native" {
			t.Errorf("termo repassado = %q, quer %q", gotTerm, "react native")
		}
		if len(got) != 1 || got[0].Repo != "x/y" {
			t.Errorf("resultado inesperado: %+v", got)
		}
	})
}

func stubSearchCodeRunner(t *testing.T, fn func(context.Context, string) ([]byte, error)) func() {
	t.Helper()
	prev := searchCodeRunner
	searchCodeRunner = fn
	return func() { searchCodeRunner = prev }
}
