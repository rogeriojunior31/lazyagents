package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// Crush's preferred config is crushrc, a Bash script of builtins (provider,
// model, hook…) run top to bottom, later statements winning. lazyagents never
// reserializes it: it owns only blocks delimited by the markers below, placed
// at the end of the global crushrc so they win, and copies every other line
// as it is. Values are single-quoted, so nothing lazyagents writes is shell
// code, and removing a block gives the user's own earlier settings back.

const (
	crushBlockStart = "# lazyagents — managed block start: %s (do not edit by hand)"
	crushBlockEnd   = "# lazyagents — managed block end: %s"
)

// crushrcFile is the global crushrc; CRUSH_GLOBAL_CONFIG moves it with crush.json.
func (c *Crush) crushrcFile() string {
	if c.GlobalConfig != "" {
		return filepath.Join(c.GlobalConfig, "crushrc")
	}
	return filepath.Join(c.configDir(), "crushrc")
}

// crushBlock returns the lines of the managed block kind in lines ("provider",
// "hooks"), and lines without it. A block without its end is an error: the
// file's structure is then unknown.
func crushBlock(lines []string, kind string) (block, rest []string, err error) {
	start, end := fmt.Sprintf(crushBlockStart, kind), fmt.Sprintf(crushBlockEnd, kind)
	in := false
	for _, l := range lines {
		switch strings.TrimSpace(l) {
		case start:
			if in {
				return nil, nil, fmt.Errorf("crushrc has a nested lazyagents %s block; file left untouched", kind)
			}
			in = true
			continue
		case end:
			if !in {
				return nil, nil, fmt.Errorf("crushrc has a lazyagents %s block end without a start; file left untouched", kind)
			}
			in = false
			continue
		}
		if in {
			block = append(block, l)
		} else {
			rest = append(rest, l)
		}
	}
	if in {
		return nil, nil, fmt.Errorf("crushrc has a lazyagents %s block without an end; file left untouched", kind)
	}
	return block, rest, nil
}

// readCrushBlock returns the body of a managed block in the global crushrc.
func (c *Crush) readCrushBlock(kind string) ([]string, error) {
	data, err := os.ReadFile(c.crushrcFile())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", c.crushrcFile(), err)
	}
	block, _, err := crushBlock(splitLines(string(data)), kind)
	return block, err
}

// crushBeforeWrite runs between reading and writing crushrc; tests use it to
// write the file concurrently.
var crushBeforeWrite = func() {}

// writeCrushBlock replaces the managed block kind with body (none: removed)
// at the end of the global crushrc. A file that ends up empty is removed: an
// empty crushrc is the same as none. Like the Codex editor, the edit is
// redone when Crush or another lazyagents wrote the file meanwhile.
func (c *Crush) writeCrushBlock(kind string, body []string, secret bool, backupsDir string) error {
	var err error
	for i := range 3 {
		if i > 0 {
			backupsDir = "" // one backup per edit
		}
		if err = c.writeCrushBlockOnce(kind, body, secret, backupsDir); !errors.Is(err, fsutil.ErrChanged) {
			return err
		}
	}
	return fmt.Errorf("%w; nothing was written, try again", err)
}

func (c *Crush) writeCrushBlockOnce(kind string, body []string, secret bool, backupsDir string) error {
	path := c.crushrcFile()
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	existed := err == nil
	_, lines, err := crushBlock(splitLines(string(data)), kind)
	if err != nil {
		return err
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(body) > 0 {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, fmt.Sprintf(crushBlockStart, kind))
		lines = append(lines, body...)
		lines = append(lines, fmt.Sprintf(crushBlockEnd, kind))
	}
	out := strings.Join(lines, "\n")
	if out != "" {
		out += "\n"
	}
	perm := os.FileMode(0o600)
	if info, err := os.Stat(path); err == nil && !secret {
		perm = info.Mode().Perm()
	}
	if backupsDir != "" && existed {
		if _, err := fsutil.Backup(path, backupsDir); err != nil {
			return err
		}
		_ = fsutil.RotateBackups(backupsDir, filepath.Base(path)+".", settingsBackups)
	}
	crushBeforeWrite()
	if strings.TrimSpace(out) == "" {
		if !existed {
			return nil
		}
		return fsutil.RemoveIfUnchanged(path, data)
	}
	if err := fsutil.WriteAtomicIfUnchanged(path, data, existed, []byte(out), perm); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// shQuote single-quotes s for Bash: nothing inside is expanded or run.
func shQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

var envNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// shWords splits one line of the Bash lazyagents writes into words: single
// and double quotes, backslash escapes. Enough for its own blocks, not for
// arbitrary Bash.
func shWords(line string) []string {
	var words []string
	var cur strings.Builder
	inWord := false
	for i := 0; i < len(line); i++ {
		switch ch := line[i]; {
		case ch == '\'':
			inWord = true
			j := strings.IndexByte(line[i+1:], '\'')
			if j < 0 {
				j = len(line) - i - 1
			}
			cur.WriteString(line[i+1 : i+1+j])
			i += j + 1
		case ch == '"':
			inWord = true
			for i++; i < len(line) && line[i] != '"'; i++ {
				if line[i] == '\\' && i+1 < len(line) {
					i++
				}
				cur.WriteByte(line[i])
			}
		case ch == '\\' && i+1 < len(line):
			inWord = true
			i++
			cur.WriteByte(line[i])
		case ch == ' ' || ch == '\t':
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			inWord = true
			cur.WriteByte(ch)
		}
	}
	if inWord {
		words = append(words, cur.String())
	}
	return words
}

// shFlag returns the value after --name in words.
func shFlag(words []string, name string) (string, bool) {
	for i := 0; i+1 < len(words); i++ {
		if words[i] == name {
			return words[i+1], true
		}
	}
	return "", false
}
