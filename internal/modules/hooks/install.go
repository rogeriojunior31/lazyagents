package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// Claude Code plugins declare hooks in <plugin>/hooks/hooks.json, and the
// commands reach their files via ${CLAUDE_PLUGIN_ROOT}, which only Claude Code
// expands, and only for plugins it installed. Importing therefore copies the
// folders the commands reference (keeping the layout: scripts locate themselves
// relative to the root), points the variable at the copy and exports it.
const (
	hooksDirName  = "hooks"
	hooksFile     = "hooks.json"
	pluginRootVar = "CLAUDE_PLUGIN_ROOT"
	pluginRoot    = "${" + pluginRootVar + "}"
	pluginRootSh  = "$" + pluginRootVar
	// maxDepth bounds the scan: plugin/hooks/hooks.json is the expected depth.
	maxDepth = 5
)

// Found is a hooks.json found in a source (clone, folder or zip), a candidate
// for import. Commands are still as the plugin wrote them.
type Found struct {
	Plugin      string // plugin name (the folder containing hooks/)
	Root        string // what ${CLAUDE_PLUGIN_ROOT} means
	Dir         string // hooks/ folder in the source
	Rel         string // path relative to the source root
	Description string // hooks.json description
	Hooks       []agent.Hook
}

// Events returns the file's distinct events.
func (f Found) Events() []string {
	seen := map[string]bool{}
	var out []string
	for _, h := range f.Hooks {
		if !seen[h.Event] {
			seen[h.Event] = true
			out = append(out, h.Event)
		}
	}
	return out
}

// DiscoverIn scans a source on disk for hooks/hooks.json. rootName names the
// plugin whose hooks/ is at the source root (a clone lives in a temp dir, whose
// name is useless). A plugin that cannot be read is skipped.
func DiscoverIn(root, rootName string) []Found {
	var out []Found
	rootClean := filepath.Clean(root)
	_ = filepath.WalkDir(rootClean, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(rootClean, path)
		if depth := len(strings.Split(filepath.ToSlash(rel), "/")); rel != "." && depth > maxDepth {
			return fs.SkipDir
		}
		if name := d.Name(); rel != "." && strings.HasPrefix(name, ".") && name != "." {
			return fs.SkipDir // .git, .github…
		}
		if d.Name() != hooksDirName {
			return nil
		}
		file := filepath.Join(path, hooksFile)
		hooks, err := agent.ReadHookFile(file)
		if err != nil || len(hooks) == 0 {
			return fs.SkipDir
		}
		pluginRootDir := filepath.Dir(path)
		plugin := filepath.Base(pluginRootDir)
		if pluginRootDir == rootClean && rootName != "" {
			plugin = rootName // hooks/ at the source root: the dir is temporary
		}
		relDir, _ := filepath.Rel(rootClean, path)
		out = append(out, Found{
			Plugin:      plugin,
			Root:        pluginRootDir,
			Dir:         path,
			Rel:         filepath.ToSlash(relDir),
			Description: hookFileDescription(file),
			Hooks:       hooks,
		})
		return fs.SkipDir
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Plugin < out[j].Plugin })
	return out
}

// hookFileDescription reads the hooks.json "description" field, if any.
func hookFileDescription(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var doc struct {
		Description string `json:"description"`
	}
	_ = json.Unmarshal(data, &doc)
	return doc.Description
}

// Import copies the referenced folders and writes one entry per plugin.
// Existing destinations are rejected before any file is copied.
func Import(paths core.Paths, f Found, source string) (names []string, err error) {
	if !nameRe.MatchString(f.Plugin) || len([]rune(f.Plugin)) > maxNameLen {
		return nil, fmt.Errorf("invalid plugin name: %q", f.Plugin)
	}
	if _, err := os.Lstat(filepath.Join(paths.HooksDir(), f.Plugin+".json")); !os.IsNotExist(err) {
		return nil, fmt.Errorf("hook %q already exists or cannot be accessed", f.Plugin)
	}
	dst := filepath.Join(paths.HooksDir(), f.Plugin)
	if _, err := os.Lstat(dst); !os.IsNotExist(err) {
		return nil, fmt.Errorf("hooks from %q are already in the library (remove them before reinstalling)", f.Plugin)
	}
	entry, err := libraryEntry(f, dst, source)
	if err != nil {
		return nil, err
	}
	for _, rel := range referencedPaths(f) {
		src := filepath.Join(f.Root, rel)
		if _, err := os.Stat(src); err != nil {
			_ = os.RemoveAll(dst)
			return nil, fmt.Errorf("%s: the commands reference %q, which does not exist in the source", f.Plugin, rel)
		}
		if err := copyTree(src, filepath.Join(dst, rel)); err != nil {
			_ = os.RemoveAll(dst)
			return nil, fmt.Errorf("copying %s from %q: %w", rel, f.Plugin, err)
		}
	}
	svc := &Service{dir: paths.HooksDir()}
	if err := svc.Save(entry); err != nil {
		_ = os.RemoveAll(dst) // import is all or nothing
		return nil, fmt.Errorf("importing %q: %w", f.Plugin, err)
	}
	return []string{entry.Name}, nil
}

// libraryEntry turns the plugin's hooks.json into one library entry, with
// commands pointing at the copied scripts.
func libraryEntry(f Found, dst, source string) (Hook, error) {
	entry := Hook{
		Name:        f.Plugin,
		Description: f.Description,
		Source:      strings.TrimSpace(source + " · " + f.Plugin),
		Files:       dst,
	}
	for _, h := range f.Hooks {
		command, err := rewriteCommand(h.Command, dst)
		if err != nil {
			return Hook{}, fmt.Errorf("%s (%s): %w", f.Plugin, h.Event, err)
		}
		h.Command = command
		// Real plugins repeat a command across groups (different matchers, same
		// identity); duplicates would skew the installed count, so keep one.
		duplicate := false
		for _, existing := range entry.Hooks {
			if existing.Same(h) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			entry.Hooks = append(entry.Hooks, h)
		}
	}
	return entry, nil
}

// referencedPaths returns the distinct top-level folders of the plugin root
// that the commands reference, always including hooks/. Only these are copied:
// the rest (docs, tests, skills) does not belong in the hook library.
func referencedPaths(f Found) []string {
	seen := map[string]bool{hooksDirName: true}
	out := []string{hooksDirName}
	for _, h := range f.Hooks {
		for _, rel := range rootRefs(h.Command) {
			if !seen[rel] {
				seen[rel] = true
				out = append(out, rel)
			}
		}
	}
	sort.Strings(out)
	return out
}

// rootRefs extracts the first segment of each ${CLAUDE_PLUGIN_ROOT}/<seg>/…
func rootRefs(command string) []string {
	var out []string
	for _, ref := range []string{pluginRoot, pluginRootSh} {
		rest := command
		for {
			i := strings.Index(rest, ref+"/")
			if i < 0 {
				break
			}
			rest = rest[i+len(ref)+1:]
			seg := rest
			if j := strings.IndexAny(seg, `/"' 	`); j >= 0 {
				seg = seg[:j]
			}
			if seg != "" && seg != "." && seg != ".." {
				out = append(out, seg)
			}
		}
	}
	return out
}

// rewriteCommand sets the root before the shell expands the original command,
// keeping the author's quoting and never inserting paths into shell code.
// ${CLAUDE_PLUGIN_ROOT} becomes $CLAUDE_PLUGIN_ROOT: Claude Code rejects any
// settings.json command containing the literal "${CLAUDE_PLUGIN_ROOT}" ("the hook
// is not associated with a plugin"), even if the command exports it.
func rewriteCommand(command, dst string) (string, error) {
	if !strings.Contains(command, pluginRootVar) {
		return command, nil
	}
	command, err := unbracePluginRoot(command)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("export %s=%s; %s", pluginRootVar, shellQuote(dst), command), nil
}

// stripRootExport removes the "export CLAUDE_PLUGIN_ROOT='…'; " prefix added by
// rewriteCommand, returning the plugin's command.
func stripRootExport(command string) string {
	if !strings.HasPrefix(command, "export "+pluginRootVar+"=") {
		return command
	}
	if i := strings.Index(command, "; "); i >= 0 {
		return command[i+2:]
	}
	return command
}

// unbracePluginRoot replaces ${CLAUDE_PLUGIN_ROOT} with $CLAUDE_PLUGIN_ROOT,
// refusing when the next character would extend the name (${CLAUDE_PLUGIN_ROOT}x).
func unbracePluginRoot(command string) (string, error) {
	var b strings.Builder
	rest := command
	for {
		i := strings.Index(rest, pluginRoot)
		if i < 0 {
			b.WriteString(rest)
			return b.String(), nil
		}
		after := rest[i+len(pluginRoot):]
		if after != "" && isNameByte(after[0]) {
			return "", fmt.Errorf("command uses %s glued to a name (%q): Claude Code does not accept that form outside a plugin", pluginRoot, command)
		}
		b.WriteString(rest[:i] + pluginRootSh)
		rest = after
	}
}

func isNameByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// shellQuote quotes a path for a shell command line.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// copyTree copies a tree keeping the exec bit (hook scripts need it).
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil // symlinks and the like stay out of the library
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}

// RepairImported fixes entries imported before rewriteCommand switched to
// $CLAUDE_PLUGIN_ROOT (Claude Code rejected the old form): swaps each old command
// wherever installed and rewrites the entry. Returns the repaired names; touches
// no file when there is nothing to repair.
func (s *Service) RepairImported() ([]string, error) {
	lib, _ := s.Library()
	var repaired, errs []string
	for _, entry := range lib {
		var olds, news []agent.Hook
		for i, h := range entry.Hooks {
			prefix := pluginRootVar + "="
			if !strings.HasPrefix(h.Command, "export "+prefix) || !strings.Contains(h.Command, pluginRoot) {
				continue
			}
			fixed, err := unbracePluginRoot(h.Command)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", entry.Name, err))
				continue
			}
			olds = append(olds, h)
			h.Command = fixed
			news = append(news, h)
			entry.Hooks[i] = h
		}
		if len(olds) == 0 {
			continue
		}
		if err := s.swapInstalled(olds, news); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", entry.Name, err))
			continue // the old entry still matches what is installed
		}
		if err := s.Save(entry); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", entry.Name, err))
			continue
		}
		repaired = append(repaired, entry.Name)
	}
	if len(errs) > 0 {
		return repaired, fmt.Errorf("reparando hooks: %s", strings.Join(errs, "; "))
	}
	return repaired, nil
}

// swapInstalled replaces the old command with the new one in every agent that has it.
func (s *Service) swapInstalled(olds, news []agent.Hook) error {
	for _, ad := range s.adapters {
		host, ok := ad.(agent.HooksHost)
		if !ok {
			continue
		}
		installed, err := host.ReadHooks()
		if err != nil {
			continue // unreadable file: Status already shows the error
		}
		for i, old := range olds {
			if !containsHook(installed, old) {
				continue
			}
			if err := host.AddHook(news[i], s.backupsDir); err != nil {
				return fmt.Errorf("%s: %w", ad.ID(), err)
			}
			if err := host.RemoveHook(old, s.backupsDir); err != nil {
				return fmt.Errorf("%s: %w", ad.ID(), err)
			}
		}
	}
	return nil
}
