package hooks

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

type hookDocument struct{ Path, Text string }

// Matches literal paths only; never evaluates the shell or substitutions.
var commandWords = regexp.MustCompile(`"[^"]*"|'[^']*'|[^\s;|&]+`)

func readScript(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !st.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file", path)
	}
	data, err := io.ReadAll(io.LimitReader(f, 1024*1024+1))
	if err != nil {
		return "", err
	}
	if len(data) > 1024*1024 {
		return "", fmt.Errorf("%s is larger than 1 MiB", path)
	}
	if bytes.ContainsRune(data, 0) {
		return "", fmt.Errorf("%s is not text", path)
	}
	return string(data), nil
}

func (s *Service) documents(h Hook, index int) ([]hookDocument, error) {
	if index < 0 || index >= len(h.Hooks) {
		return nil, fmt.Errorf("command not found")
	}
	raw := h.Hooks[index].Command
	docs := []hookDocument{{Text: raw}}
	command := stripRootExport(raw)
	if h.Files != "" {
		command = strings.NewReplacer(pluginRoot, h.Files, pluginRootSh, h.Files).Replace(command)
	}
	seen := map[string]bool{}
	for _, word := range commandWords.FindAllString(command, -1) {
		path := strings.Trim(word, `"'`)
		switch strings.ToLower(filepath.Ext(path)) {
		case ".sh", ".bash", ".zsh", ".py", ".js", ".mjs", ".cjs", ".ts", ".rb", ".pl", ".lua", ".ps1":
		default:
			continue
		}
		if !filepath.IsAbs(path) {
			if h.Files == "" {
				continue
			}
			path = filepath.Join(h.Files, path)
		}
		path = filepath.Clean(path)
		if seen[path] {
			continue
		}
		seen[path] = true
		body, err := readScript(path)
		if err != nil {
			return docs, fmt.Errorf("reading script %s: %w", path, err)
		}
		docs = append(docs, hookDocument{Path: path, Text: body})
	}
	return docs, nil
}

func editCopy(doc hookDocument) (string, error) {
	ext := filepath.Ext(doc.Path)
	if ext == "" {
		ext = ".sh"
	}
	f, err := os.CreateTemp("", "lazyagents-hook-*"+ext)
	if err != nil {
		return "", err
	}
	_, err = f.WriteString(doc.Text)
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// saveDocument checks the version it read so external changes are not overwritten.
func (s *Service) saveDocument(h Hook, index int, doc hookDocument, text string) error {
	if text == doc.Text {
		return nil
	}
	if doc.Path != "" {
		current, err := readScript(doc.Path)
		if err != nil {
			return err
		}
		if current != doc.Text {
			return fmt.Errorf("script changed since it was read; reopen it before editing")
		}
		path, err := filepath.EvalSymlinks(doc.Path)
		if err != nil {
			return err
		}
		st, err := os.Stat(path)
		if err != nil {
			return err
		}
		if _, err = fsutil.Backup(path, s.backupsDir); err != nil {
			return err
		}
		return fsutil.WriteAtomic(path, []byte(text), st.Mode().Perm())
	}
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("the command cannot be empty")
	}
	current, err := s.Get(h.Name)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(current.Hooks) || current.Hooks[index] != h.Hooks[index] {
		return fmt.Errorf("command changed since it was read; reopen it before editing")
	}
	old := current.Hooks[index]
	next := old
	next.Command = text
	for i, c := range current.Hooks {
		if i != index && c.Same(next) {
			return fmt.Errorf("this hook already has an identical command")
		}
	}
	if _, err = fsutil.Backup(filepath.Join(s.dir, h.Name+".json"), s.backupsDir); err != nil {
		return err
	}
	type target struct {
		host   agent.HooksHost
		hadNew bool
	}
	var targets, changed []target
	for _, ad := range s.adapters {
		if host, ok := ad.(agent.HooksHost); ok {
			installed, err := host.ReadHooks()
			if err != nil {
				return fmt.Errorf("reading %s hooks before the edit: %w", ad.ID(), err)
			}
			if containsHook(installed, old) {
				targets = append(targets, target{host, containsHook(installed, next)})
			}
		}
	}
	rollback := func(cause error) error {
		for i := len(changed) - 1; i >= 0; i-- {
			t := changed[i]
			if err := t.host.AddHook(old, s.backupsDir); err != nil {
				cause = errors.Join(cause, err)
				continue
			}
			if !t.hadNew {
				cause = errors.Join(cause, t.host.RemoveHook(next, s.backupsDir))
			}
		}
		return cause
	}
	for _, t := range targets {
		changed = append(changed, t)
		if !t.hadNew {
			if err := t.host.AddHook(next, s.backupsDir); err != nil {
				return rollback(err)
			}
		}
		if err := t.host.RemoveHook(old, s.backupsDir); err != nil {
			return rollback(err)
		}
	}
	current.Hooks[index] = next
	if err := s.Save(current); err != nil {
		return rollback(err)
	}
	return nil
}
