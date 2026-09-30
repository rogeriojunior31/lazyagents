package agent

import (
	"errors"
	"strings"
)

// errTOMLUnterminated is a config whose string or array never closes: its
// structure is unknown, so it is never edited.
var errTOMLUnterminated = errors.New("TOML config has an unterminated string or array; file left untouched")

// tomlValueLines reports, per line, whether the line starts inside a value
// that spans lines: a multiline string ("""…""" or ”'…”'), or an array or
// inline table left open. Such a line is value content, never a key, a
// [table] header or a lazyagents marker, and nothing may be inserted before
// it. This is only enough TOML to find those boundaries; everything else is
// copied as written.
func tomlValueLines(lines []string) ([]bool, error) {
	out := make([]bool, len(lines))
	var ml string // "" or the open multiline delimiter
	depth := 0    // open [ and { of values
	for i, line := range lines {
		out[i] = ml != "" || depth > 0
		for j := 0; j < len(line); {
			if ml != "" {
				end := tomlMultilineEnd(line[j:], ml)
				if end < 0 {
					break
				}
				j += end
				ml = ""
				continue
			}
			switch c := line[j]; {
			case c == '#':
				j = len(line)
			case strings.HasPrefix(line[j:], `"""`), strings.HasPrefix(line[j:], "'''"):
				ml = line[j : j+3]
				j += 3
			case c == '"' || c == '\'':
				j += tomlStringLen(line[j:])
			case c == '[' && depth == 0 && strings.TrimSpace(line[:j]) == "":
				j = len(line) // a [table] header: nothing after it is a value
			case c == '[' || c == '{':
				depth++
				j++
			case c == ']' || c == '}':
				depth = max(0, depth-1)
				j++
			default:
				j++
			}
		}
	}
	if ml != "" || depth > 0 {
		return nil, errTOMLUnterminated
	}
	return out, nil
}

// tomlMultilineEnd returns where the text after the closing delim ends, or -1.
// A basic string (""") may escape a quote with a backslash; a literal (”') may
// not. Up to two extra quotes before the delimiter belong to the string.
func tomlMultilineEnd(s, delim string) int {
	for j := 0; j < len(s); j++ {
		if delim == `"""` && s[j] == '\\' {
			j++
			continue
		}
		if strings.HasPrefix(s[j:], delim) {
			end := j + 3
			for k := 0; k < 2 && end < len(s) && s[end] == delim[0]; k++ {
				end++
			}
			return end
		}
	}
	return -1
}

// tomlStringLen is the length of the one-line string at the start of s,
// quotes included; an unclosed one takes the rest of the line.
func tomlStringLen(s string) int {
	q := s[0]
	for j := 1; j < len(s); j++ {
		if q == '"' && s[j] == '\\' {
			j++
			continue
		}
		if s[j] == q {
			return j + 1
		}
	}
	return len(s)
}
