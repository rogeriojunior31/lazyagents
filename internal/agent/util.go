package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// maxLineBuf enlarges the scanner buffer: one JSONL line with a long assistant
// reply can exceed 1 MB.
const maxLineBuf = 4 * 1024 * 1024

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// version is the version number `bin --version` prints: CLIs wrap it in
// their name ("codex-cli 0.159.2", "crush version v0.97.1").
func version(bin string, args ...string) string { return versionNumber(versionLine(bin, args...)) }

var versionRe = regexp.MustCompile(`\bv?(\d+(?:\.\d+)+(?:-[0-9A-Za-z.-]+)?)`)

// versionNumber is the first version number in line, without a leading v;
// a line with none is kept as it is.
func versionNumber(line string) string {
	if m := versionRe.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	return line
}

// versionLine runs `bin --version` with a short timeout and returns the first line.
func versionLine(bin string, args ...string) string {
	if bin == "" {
		return ""
	}
	if len(args) == 0 {
		args = []string{"--version"}
	}
	// Node/Bun CLIs (gemini, opencode) take >2s just to print their version
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, args...).Output()
	if err != nil {
		return ""
	}
	line, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	if len(line) > 40 {
		line = line[:40]
	}
	return line
}

// firstLines reads up to max lines of a JSONL file. Errors give an empty list:
// session listing is best-effort and never takes the TUI down.
func firstLines(path string, max int) [][]byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), maxLineBuf)
	var out [][]byte
	for len(out) < max && sc.Scan() {
		line := make([]byte, len(sc.Bytes()))
		copy(line, sc.Bytes())
		out = append(out, line)
	}
	return out
}

// looseUserText extracts a user message's text from a JSONL line of unknown
// shape: {"role":"user","content":...}, {"type":"user","message":{...}},
// {"role":"user","parts":[{"text":...}]} and nested Codex payloads.
func looseUserText(line []byte) string {
	var m map[string]any
	if err := json.Unmarshal(line, &m); err != nil {
		return ""
	}
	return userTextFromMap(m)
}

func userTextFromMap(m map[string]any) string {
	if meta, _ := m["isMeta"].(bool); meta {
		return ""
	}
	role, _ := m["role"].(string)
	typ, _ := m["type"].(string)
	if role == "user" || typ == "user" {
		for _, k := range []string{"content", "parts", "text", "display"} {
			if t := extractAnyText(m[k]); t != "" {
				return t
			}
		}
	}
	// unwrap common envelopes: message (Claude), payload (Codex)
	for _, k := range []string{"message", "payload"} {
		if inner, ok := m[k].(map[string]any); ok {
			if t := userTextFromMap(inner); t != "" {
				return t
			}
		}
	}
	return ""
}

// extractAnyText gets text from content in any shape: a string, a list of
// {"type":"text"/"input_text","text":...} blocks, or {"text":...}.
func extractAnyText(v any) string {
	switch c := v.(type) {
	case string:
		return c
	case []any:
		for _, b := range c {
			if t := extractAnyText(b); t != "" {
				return t
			}
		}
	case map[string]any:
		if t, ok := c["text"].(string); ok {
			return t
		}
	}
	return ""
}

// cleanTitle turns a prompt into a list title: collapses whitespace, drops
// harness tags ("<local-command...>") and cuts at max runes.
func cleanTitle(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "<") {
		return ""
	}
	s = strings.Join(strings.FieldsFunc(s, unicode.IsSpace), " ")
	r := []rune(s)
	if len(r) > max {
		return string(r[:max-1]) + "…"
	}
	return s
}
