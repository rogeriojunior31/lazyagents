package skills

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// Issue is a SKILL.md quality problem per the Agent Skills spec
// (agentskills.io) in a file that parsed; Skill.Warning covers parse failures.
type Issue struct {
	Field string
	Msg   string
}

// maxDescriptionLen is the description length limit recommended by the spec.
const maxDescriptionLen = 1024

// Validate lints sk's SKILL.md locally: name present, kebab-case and equal to
// the folder; description non-empty and within the limit; non-empty body.
// Returns nil when the frontmatter is unreadable: Warning already reports it.
func Validate(sk Skill) []Issue {
	if !sk.Valid {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(sk.Path, "SKILL.md"))
	if err != nil {
		return []Issue{{Field: "SKILL.md", Msg: fmt.Sprintf("reading file: %v", err)}}
	}
	meta, ok := ParseMeta(data)
	if !ok {
		return []Issue{{Field: "SKILL.md", Msg: "invalid YAML frontmatter"}}
	}

	var issues []Issue
	switch {
	case meta.Name == "":
		issues = append(issues, Issue{Field: "name", Msg: "missing from frontmatter"})
	case !skillNameRe.MatchString(meta.Name):
		issues = append(issues, Issue{Field: "name", Msg: "not kebab-case"})
	case meta.Name != sk.Dir:
		issues = append(issues, Issue{Field: "name", Msg: fmt.Sprintf("differs from the folder (%q)", sk.Dir)})
	}
	switch {
	case meta.Description == "":
		issues = append(issues, Issue{Field: "description", Msg: "empty"})
	case len(meta.Description) > maxDescriptionLen:
		issues = append(issues, Issue{Field: "description", Msg: fmt.Sprintf("over %d chars (has %d)", maxDescriptionLen, len(meta.Description))})
	}
	if bodyAfterFrontmatter(data) == "" {
		issues = append(issues, Issue{Field: "body", Msg: "no instructions after the frontmatter"})
	}
	return issues
}

// bodyAfterFrontmatter returns the trimmed content after the closing fence
// (--- or ...); without frontmatter, the whole file, like frontmatter().
func bodyAfterFrontmatter(data []byte) string {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	lines := bytes.Split(data, []byte("\n"))
	if len(lines) == 0 || string(bytes.TrimRight(lines[0], "\r ")) != "---" {
		return string(bytes.TrimSpace(data))
	}
	for i := 1; i < len(lines); i++ {
		t := string(bytes.TrimRight(lines[i], "\r "))
		if t == "---" || t == "..." {
			return string(bytes.TrimSpace(bytes.Join(lines[i+1:], []byte("\n"))))
		}
	}
	return ""
}
