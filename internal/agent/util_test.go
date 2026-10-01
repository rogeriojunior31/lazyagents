package agent

import "testing"

// The Agents tab shows only the number, whatever the CLI wraps it in.
func TestVersionNumber(t *testing.T) {
	for in, want := range map[string]string{
		"2.1.285 (Claude Code)":   "2.1.285",
		"codex-cli 0.159.2":       "0.159.2",
		"crush version v0.97.1":   "0.97.1",
		"0.99.1":                  "0.99.1",
		"gemini 0.9.0-preview.2":  "0.9.0-preview.2",
		"Hermes Agent v1.4 (abc)": "1.4",
		"no version here":         "no version here",
		"":                        "",
	} {
		if got := versionNumber(in); got != want {
			t.Errorf("versionNumber(%q) = %q, want %q", in, got, want)
		}
	}
}
