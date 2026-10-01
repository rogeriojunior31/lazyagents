package agent

import (
	"regexp"
	"strconv"
	"strings"
)

// Tool kinds: what a call did, so the reader can show each the way it reads
// best. Names differ per agent; the kind does not.
const (
	ToolShell = "shell" // ran a command
	ToolEdit  = "edit"  // changed files (Added/Removed lines)
	ToolRead  = "read"  // looked something up: file, search, listing, web
	ToolAgent = "agent" // started a subagent
	ToolPlan  = "plan"  // proposed a plan (Body)
	ToolTodo  = "todo"  // updated a task list (Body, one "☑ step" per line)
)

// toolKinds maps lowercased tool names to their kind. Claude Code, Codex,
// Gemini CLI, OpenCode, Crush and Pi; an unknown name has no kind.
var toolKinds = map[string]string{
	"bash": ToolShell, "shell": ToolShell, "exec_command": ToolShell, "local_shell_call": ToolShell,
	"run_shell_command": ToolShell,
	"edit":              ToolEdit, "multiedit": ToolEdit, "write": ToolEdit, "apply_patch": ToolEdit,
	"notebookedit": ToolEdit, "replace": ToolEdit, "write_file": ToolEdit, "multi_edit": ToolEdit,
	"read": ToolRead, "view": ToolRead, "read_file": ToolRead, "read_many_files": ToolRead,
	"ls": ToolRead, "list_directory": ToolRead, "glob": ToolRead, "grep": ToolRead,
	"search_file_content": ToolRead, "view_image": ToolRead, "webfetch": ToolRead,
	"websearch": ToolRead, "web_fetch": ToolRead, "google_web_search": ToolRead, "fetch": ToolRead,
	"agent": ToolAgent, "task": ToolAgent,
	"exitplanmode": ToolPlan,
	"todowrite":    ToolTodo, "update_plan": ToolTodo, "write_todos": ToolTodo,
}

// patchRe finds a patch passed to apply_patch inside codex exec code.
var patchRe = regexp.MustCompile(`apply_patch\(\s*"((?:[^"\\]|\\.)*)"`)

// classify fills the kind and the kind's detail of a tool call. arg is the
// one-line argument toolEntry chose; it returns the argument to show instead
// (the files of a patch) or arg unchanged.
func classify(e *Entry, name string, args any, arg string) string {
	e.Kind = toolKinds[strings.ToLower(name)]
	a, _ := args.(map[string]any)
	switch e.Kind {
	case ToolEdit:
		if p, ok := args.(string); ok { // codex apply_patch: the patch is the input
			return patchStats(e, p, arg)
		}
		edits := []any{a}
		if list, ok := a["edits"].([]any); ok { // MultiEdit
			edits = list
		}
		for _, x := range edits {
			m, _ := x.(map[string]any)
			old := firstString(m, "old_string", "oldString", "oldText")
			repl := firstString(m, "new_string", "newString", "newText", "content")
			add, del := lineDelta(old, repl)
			e.Added += add
			e.Removed += del
		}
	case ToolPlan:
		e.Body, _ = a["plan"].(string)
	case ToolTodo:
		e.Body = todoList(a)
	case "":
		// codex exec: JS code calling tools.exec_command or tools.apply_patch
		if code, ok := args.(string); ok && name == "exec" {
			if sub := patchRe.FindStringSubmatch(code); sub != nil {
				if p, err := strconv.Unquote(`"` + sub[1] + `"`); err == nil {
					e.Kind = ToolEdit
					return patchStats(e, p, arg)
				}
			}
			if execCmdRe.MatchString(code) {
				e.Kind = ToolShell
			}
		}
	}
	return arg
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok {
			return s
		}
	}
	return ""
}

// lineDelta counts the lines an edit adds and removes, leaving out the lines
// both sides share at the start and the end (the context Edit tools repeat).
func lineDelta(old, repl string) (added, removed int) {
	a, b := splitLines(old), splitLines(repl)
	for len(a) > 0 && len(b) > 0 && a[0] == b[0] {
		a, b = a[1:], b[1:]
	}
	for len(a) > 0 && len(b) > 0 && a[len(a)-1] == b[len(b)-1] {
		a, b = a[:len(a)-1], b[:len(b)-1]
	}
	return len(b), len(a)
}

// patchStats reads a codex patch ("*** Begin Patch", "*** Update File: p",
// +/- lines): the files become the argument, the lines the counts.
func patchStats(e *Entry, patch, arg string) string {
	var files []string
	for _, ln := range strings.Split(patch, "\n") {
		switch {
		case strings.HasPrefix(ln, "*** Add File: "), strings.HasPrefix(ln, "*** Update File: "),
			strings.HasPrefix(ln, "*** Delete File: "):
			_, f, _ := strings.Cut(ln, ": ")
			files = append(files, strings.TrimSpace(f))
		case strings.HasPrefix(ln, "***"):
		case strings.HasPrefix(ln, "+"):
			e.Added++
		case strings.HasPrefix(ln, "-"):
			e.Removed++
		}
	}
	if len(files) == 0 {
		return arg
	}
	return strings.Join(files, ", ")
}

// todoList renders a task list (Claude TodoWrite "todos", Codex update_plan
// "plan", Gemini write_todos) one "☑ step" per line.
func todoList(a map[string]any) string {
	var items []any
	for _, k := range []string{"todos", "plan"} {
		if list, ok := a[k].([]any); ok {
			items = list
			break
		}
	}
	var lines []string
	for _, x := range items {
		m, _ := x.(map[string]any)
		text := firstString(m, "content", "step", "description")
		if text == "" {
			continue
		}
		mark := "☐"
		switch m["status"] {
		case "completed":
			mark = "☑"
		case "in_progress":
			mark = "◐"
		case "cancelled":
			mark = "☒"
		}
		lines = append(lines, mark+" "+oneLine(text))
	}
	return strings.Join(lines, "\n")
}
