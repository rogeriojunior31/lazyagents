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
			name: "dedupes by repo and sorts",
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
			name: "no items",
			json: `{"items":[]}`,
			want: nil,
		},
		{
			name:    "invalid json",
			json:    `not json`,
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseSearchCodeResponse([]byte(tc.json))
			if tc.wantErr {
				if err == nil {
					t.Fatal("want an error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %d results, want %d: %+v", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("result[%d] = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestSearchRegistry(t *testing.T) {
	svc := New(testPaths(t))

	t.Run("empty term skips the network", func(t *testing.T) {
		called := false
		restore := stubSearchCodeRunner(t, func(context.Context, string) ([]byte, error) {
			called = true
			return nil, nil
		})
		defer restore()
		if _, err := svc.SearchRegistry("   "); err == nil {
			t.Fatal("want an error for an empty term")
		}
		if called {
			t.Error("runner called with an empty term")
		}
	})

	t.Run("network error propagates without touching the library", func(t *testing.T) {
		restore := stubSearchCodeRunner(t, func(context.Context, string) ([]byte, error) {
			return nil, errors.New("no network")
		})
		defer restore()
		if _, err := svc.SearchRegistry("react"); err == nil {
			t.Fatal("want a network error")
		}
	})

	t.Run("no results is a friendly error", func(t *testing.T) {
		restore := stubSearchCodeRunner(t, func(context.Context, string) ([]byte, error) {
			return []byte(`{"items":[]}`), nil
		})
		defer restore()
		if _, err := svc.SearchRegistry("missing-term"); err == nil {
			t.Fatal("want an error for a search without results")
		}
	})

	t.Run("ok result passes the term to the runner", func(t *testing.T) {
		var gotTerm string
		restore := stubSearchCodeRunner(t, func(_ context.Context, term string) ([]byte, error) {
			gotTerm = term
			return []byte(`{"items":[{"path":"SKILL.md","html_url":"https://github.com/x/y","repository":{"full_name":"x/y","description":"desc"}}]}`), nil
		})
		defer restore()
		got, err := svc.SearchRegistry("react native")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotTerm != "react native" {
			t.Errorf("term passed = %q, want %q", gotTerm, "react native")
		}
		if len(got) != 1 || got[0].Repo != "x/y" {
			t.Errorf("unexpected result: %+v", got)
		}
	})
}

func stubSearchCodeRunner(t *testing.T, fn func(context.Context, string) ([]byte, error)) func() {
	t.Helper()
	prev := searchCodeRunner
	searchCodeRunner = fn
	return func() { searchCodeRunner = prev }
}
