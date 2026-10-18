//go:build ignore

package main

import (
	"os"
	"strings"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
)

func TestPreviewDoesNotInheritNativePathsOrCredentials(t *testing.T) {
	before := os.Environ()
	t.Cleanup(func() {
		os.Clearenv()
		for _, pair := range before {
			key, value, _ := strings.Cut(pair, "=")
			_ = os.Setenv(key, value)
		}
	})
	for _, override := range agent.ConfigOverrides {
		if err := os.Setenv(override.Var, t.TempDir()); err != nil {
			t.Fatal(err)
		}
	}
	for _, key := range []string{"ANTHROPIC_API_KEY", "GEMINI_API_KEY", "OPENAI_API_KEY", "LOCAL_KEY", "OPENCODE_CONFIG_CONTENT"} {
		if err := os.Setenv(key, "private-test-value"); err != nil {
			t.Fatal(err)
		}
	}
	home, path := t.TempDir(), os.Getenv("PATH")
	if err := isolatePreview(home); err != nil {
		t.Fatal(err)
	}
	for _, override := range agent.ConfigOverrides {
		if _, ok := os.LookupEnv(override.Var); ok {
			t.Errorf("preview inherited %s", override.Var)
		}
	}
	for _, key := range []string{"ANTHROPIC_API_KEY", "GEMINI_API_KEY", "OPENAI_API_KEY", "LOCAL_KEY", "OPENCODE_CONFIG_CONTENT"} {
		if _, ok := os.LookupEnv(key); ok {
			t.Errorf("preview inherited %s", key)
		}
	}
	if os.Getenv("HOME") != home || os.Getenv("USERPROFILE") != home || os.Getenv("PATH") != path {
		t.Fatal("preview lost isolated home or tool lookup path")
	}
}
