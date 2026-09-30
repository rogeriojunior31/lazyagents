package skills

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

var (
	ErrNotInLibrary = errors.New("skill is not in the lazyagents library — adopt it first (key o)")
	ErrLocalSkill   = errors.New("local skill not managed by lazyagents")
	ErrNoSkillsDir  = errors.New("agent has no manageable skills directory")
	ErrSkillExists  = errors.New("a skill with this name already exists")
	ErrForeignDir   = errors.New("the agent sees it through another agent's skills directory — disable it there")
	ErrNoGitOrigin  = errors.New("skill has no git source — only skills installed from GitHub can be updated")
)

// skillNameRe validates skill names: kebab-case as agents expect (1-64 chars,
// lowercase/digits/hyphens, matching the folder name).
var skillNameRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// safeSkillDir accepts a single path component, legacy non-kebab names included.
func safeSkillDir(name string) bool {
	return filepath.IsLocal(name) && name != "." && !strings.ContainsAny(name, `/\`) && !strings.ContainsRune(name, 0)
}

// Create makes a new library skill from a SKILL.md template and returns its
// folder, ready for the editor.
func (s *Service) Create(name string) (string, error) {
	if !skillNameRe.MatchString(name) || len(name) > 64 {
		return "", fmt.Errorf("invalid name %q: use kebab-case (lowercase letters, digits and hyphens)", name)
	}
	dir := filepath.Join(s.paths.LibraryDir(), name)
	if _, err := os.Lstat(dir); err == nil {
		return "", fmt.Errorf("creating %s: %w", name, ErrSkillExists)
	}
	tmpl := fmt.Sprintf(`---
name: %s
description: TODO describe what the skill does and WHEN the agent should use it
---

# %s

Instructions for the agent to follow when the skill is enabled.
`, name, name)
	if err := fsutil.WriteAtomic(filepath.Join(dir, "SKILL.md"), []byte(tmpl), 0o644); err != nil {
		return "", fmt.Errorf("creating %s: %w", name, err)
	}
	return dir, nil
}

// Enable enables the skill in the agent, keeping it where it already is for
// the others (see place). agents is every detected agent.
func (s *Service) Enable(sk Skill, ag agent.Agent, agents []agent.Agent) error {
	if !ag.SupportsSkills() {
		return fmt.Errorf("%s: %w", ag.Name, ErrNoSkillsDir)
	}
	if !sk.InLibrary {
		return ErrNotInLibrary
	}
	if st := sk.States[ag.ID]; st.On {
		return nil // already visible (managed, shared or local)
	}
	want := managedIn(sk, agents)
	want[ag.ID] = true
	if err := s.place(sk, want, withAgent(agents, ag)); err != nil {
		return fmt.Errorf("enabling %s in %s: %w", sk.Dir, ag.Name, err)
	}
	return nil
}

// Disable removes the skill from the agent only. Local skills (real dir or
// third-party symlink) are never touched.
func (s *Service) Disable(sk Skill, ag agent.Agent, agents []agent.Agent) error {
	st := sk.States[ag.ID]
	switch {
	case !st.On:
		return nil
	case st.Local:
		return fmt.Errorf("disabling %s in %s: %w (in %s)", sk.Dir, ag.Name, ErrLocalSkill, st.Via)
	case !st.Managed:
		return fmt.Errorf("disabling %s in %s: %w", sk.Dir, ag.Name, ErrForeignDir)
	}
	want := managedIn(sk, agents)
	delete(want, ag.ID)
	if err := s.place(sk, want, withAgent(agents, ag)); err != nil {
		return fmt.Errorf("disabling %s in %s: %w", sk.Dir, ag.Name, err)
	}
	return nil
}

// EnableAll enables the skill in every installed agent with a skills dir.
func (s *Service) EnableAll(sk Skill, agents []agent.Agent) error {
	if !sk.InLibrary {
		return ErrNotInLibrary
	}
	want := map[string]bool{}
	for _, ag := range skillCapableAgents(agents) {
		want[ag.ID] = true
	}
	return s.place(sk, want, agents)
}

// DisableAll removes the skill's managed symlinks from every agent.
func (s *Service) DisableAll(sk Skill, agents []agent.Agent) error {
	return s.place(sk, map[string]bool{}, agents)
}

// managedIn is the set of agents that see the skill through a lazyagents link,
// their own or another agent's (OpenCode reading ~/.claude/skills).
func managedIn(sk Skill, agents []agent.Agent) map[string]bool {
	want := map[string]bool{}
	for _, ag := range agents {
		if st := sk.States[ag.ID]; st.On && !st.Local {
			want[ag.ID] = true
		}
	}
	return want
}

// withAgent is agents with ag counted as installed: an agent named explicitly
// is a target even before its CLI is installed.
func withAgent(agents []agent.Agent, ag agent.Agent) []agent.Agent {
	ag.Installed = true
	out := slices.DeleteFunc(slices.Clone(agents), func(a agent.Agent) bool { return a.ID == ag.ID })
	return append(out, ag)
}

// place makes the skill visible through lazyagents links to exactly the agents
// in want. A shared dir (~/.agents/skills) gets the link only when every
// installed agent reading it is in want; otherwise each agent gets its own, so
// enabling a skill for one agent never shows it to another. New links are
// created before old ones go, so a skill moving dirs is never missing.
func (s *Service) place(sk Skill, want map[string]bool, agents []agent.Agent) error {
	libDir := s.paths.LibraryDir()
	capable := skillCapableAgents(agents)
	readers := map[string][]string{}
	for _, ag := range capable {
		if ag.SharedDir != "" && ag.SharedDir != libDir {
			readers[ag.SharedDir] = append(readers[ag.SharedDir], ag.ID)
		}
	}
	shared := map[string]bool{}
	for dir, ids := range readers {
		path := filepath.Join(dir, sk.Dir)
		_, err := os.Lstat(path)
		free := os.IsNotExist(err) || s.isLibraryLink(path) // someone else's entry there is left alone
		shared[dir] = free && len(ids) > 1 && !slices.ContainsFunc(ids, func(id string) bool { return !want[id] })
	}

	desired := map[string]bool{}
	for dir, ok := range shared {
		if ok {
			desired[filepath.Join(dir, sk.Dir)] = true
		}
	}
	var ours []string
	type echo struct{ own, via string }
	var echoes []echo // wanted agents that see the skill through another agent's link
	for _, ag := range capable {
		for _, dir := range []string{ag.ManagedDir, ag.SharedDir} {
			if path := filepath.Join(dir, sk.Dir); dir != "" && !slices.Contains(ours, path) && s.isLibraryLink(path) {
				ours = append(ours, path)
			}
		}
		st := sk.States[ag.ID]
		switch {
		case !want[ag.ID] || shared[ag.SharedDir] || st.Local:
		case st.On && !st.Managed:
			echoes = append(echoes, echo{filepath.Join(ag.ManagedDir, sk.Dir), filepath.Join(st.Via, sk.Dir)})
		default:
			desired[filepath.Join(ag.ManagedDir, sk.Dir)] = true
		}
	}
	// An echo keeps working while the link it goes through stays; otherwise the
	// agent needs its own.
	for _, e := range echoes {
		if !desired[e.via] {
			desired[e.own] = true
		}
	}

	var errs []error
	src := filepath.Join(libDir, sk.Dir)
	for path := range desired {
		if slices.Contains(ours, path) {
			continue
		}
		if _, err := os.Lstat(path); err == nil {
			errs = append(errs, fmt.Errorf("%w at %s", ErrSkillExists, path))
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			errs = append(errs, err)
			continue
		}
		if err := os.Symlink(src, path); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...) // keep the old links: the skill stays where it was
	}
	for _, path := range ours {
		if !desired[path] {
			if err := os.Remove(path); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// isLibraryLink reports whether path is a symlink into the library, the only
// kind of entry lazyagents creates or removes in an agent's dir.
func (s *Service) isLibraryLink(path string) bool {
	target, err := os.Readlink(path)
	if err != nil {
		return false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	return insideDir(target, s.paths.LibraryDir())
}

// Adopt moves a local skill (real dir in the agent) into the library and
// replaces the original with a symlink, keeping it enabled. Backs up first.
func (s *Service) Adopt(sk Skill, ag agent.Agent) error {
	st := sk.States[ag.ID]
	if !st.Local || st.Via == "" {
		return fmt.Errorf("adopting %s: skill is not local in %s", sk.Dir, ag.Name)
	}
	if sk.InLibrary {
		return fmt.Errorf("adopting %s: %w in the library", sk.Dir, ErrSkillExists)
	}
	src := filepath.Join(st.Via, sk.Dir)
	if info, err := os.Lstat(src); err != nil || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("adopting %s: source is a third-party symlink — manage it with the tool that created it", sk.Dir)
	}
	dst := filepath.Join(s.paths.LibraryDir(), sk.Dir)
	if err := copyDir(src, dst); err != nil {
		return fmt.Errorf("adopting %s: %w", sk.Dir, err)
	}
	if err := writeOrigin(dst, Origin{Type: "dir", Source: src, InstalledAt: time.Now()}); err != nil {
		return fmt.Errorf("adopting %s (source): %w", sk.Dir, err)
	}
	if err := s.backupDir(src, sk.Dir); err != nil {
		return fmt.Errorf("adopting %s (backup): %w", sk.Dir, err)
	}
	if err := os.RemoveAll(src); err != nil {
		return fmt.Errorf("adopting %s: %w", sk.Dir, err)
	}
	if err := os.Symlink(dst, src); err != nil {
		return fmt.Errorf("adopting %s (symlink back): %w", sk.Dir, err)
	}
	return nil
}

// AdoptAll adopts every local skill (outside the library) from the scan, one
// by one; a failure does not stop the rest (errors aggregated).
func (s *Service) AdoptAll(skills []Skill, agents []agent.Agent) (adopted []string, errs []error) {
	for _, sk := range skills {
		if sk.InLibrary {
			continue
		}
		var ag agent.Agent
		var found bool
		for _, a := range agents {
			if st := sk.States[a.ID]; st.On && st.Local && st.Via != "" {
				ag, found = a, true
				break
			}
		}
		if !found {
			errs = append(errs, fmt.Errorf("adopting %s: no local copy to adopt", sk.Dir))
			continue
		}
		if err := s.Adopt(sk, ag); err != nil {
			errs = append(errs, err)
			continue
		}
		adopted = append(adopted, sk.Name)
	}
	return adopted, errs
}

// Remove deletes the skill from the library (with a .tar.gz backup) and the
// managed symlinks pointing to it in every agent.
func (s *Service) Remove(sk Skill, agents []agent.Agent) error {
	if !sk.InLibrary {
		return ErrNotInLibrary
	}
	libPath := filepath.Join(s.paths.LibraryDir(), sk.Dir)
	if err := s.backupDir(libPath, sk.Dir); err != nil {
		return fmt.Errorf("removing %s (backup): %w", sk.Dir, err)
	}
	var errs []error
	for _, ag := range agents {
		for _, dir := range ag.ReadDirs {
			target := filepath.Join(dir, sk.Dir)
			info, err := os.Lstat(target)
			if err != nil || info.Mode()&os.ModeSymlink == 0 {
				continue
			}
			resolved, err := os.Readlink(target)
			if err != nil {
				continue
			}
			if !filepath.IsAbs(resolved) {
				resolved = filepath.Join(dir, resolved)
			}
			if insideDir(resolved, s.paths.LibraryDir()) {
				if err := os.Remove(target); err != nil {
					errs = append(errs, fmt.Errorf("cleaning up link in %s: %w", dir, err))
				}
			}
		}
	}
	if err := os.RemoveAll(libPath); err != nil {
		errs = append(errs, fmt.Errorf("removing %s: %w", libPath, err))
	}
	return errors.Join(errs...)
}

// Update reinstalls the skill from its git source, replacing the content in
// place so agent activation symlinks keep working.
func (s *Service) Update(sk Skill) error {
	if sk.Origin == nil || sk.Origin.Type != "git" {
		return ErrNoGitOrigin
	}
	if !sk.InLibrary {
		return ErrNotInLibrary
	}
	tmp, err := cloneShallow(sk.Origin.Source)
	if err != nil {
		return fmt.Errorf("updating %s: %w", sk.Dir, err)
	}
	defer os.RemoveAll(tmp)

	srcDir, err := locateInClone(tmp, sk)
	if err != nil {
		return fmt.Errorf("updating %s: %w", sk.Dir, err)
	}
	libPath := filepath.Join(s.paths.LibraryDir(), sk.Dir)
	if err := s.backupDir(libPath, sk.Dir); err != nil {
		return fmt.Errorf("updating %s (backup): %w", sk.Dir, err)
	}
	if err := replaceDir(srcDir, libPath); err != nil {
		return fmt.Errorf("updating %s (copy): %w", sk.Dir, err)
	}
	o := *sk.Origin
	o.InstalledAt = time.Now()
	if h, hErr := hashDir(libPath); hErr == nil {
		o.Hash = h
	}
	if err := writeOrigin(libPath, o); err != nil {
		return fmt.Errorf("updating %s (source): %w", sk.Dir, err)
	}
	return nil
}

// locateInClone finds the skill folder in the temp clone: the recorded Sub if
// still valid, else rediscovered.
func locateInClone(tmp string, sk Skill) (string, error) {
	if sk.Origin.Sub != "" && filepath.IsLocal(sk.Origin.Sub) {
		candidate := filepath.Join(tmp, sk.Origin.Sub)
		if _, err := os.Stat(filepath.Join(candidate, "SKILL.md")); err == nil {
			return candidate, nil
		}
	}
	found, err := discoverIn(tmp, sk.Dir)
	if err != nil {
		return "", fmt.Errorf("skill not found in the repository: %w", err)
	}
	for _, f := range found {
		if f.Name == sk.Dir || f.Name == sk.Name {
			return f.SrcDir, nil
		}
	}
	if len(found) == 1 {
		return found[0].SrcDir, nil
	}
	return "", fmt.Errorf("skill %q not found in the remote repository", sk.Dir)
}

// replaceDir replaces dst's visible content with src's, keeping dst's hidden
// files (e.g. .origin.json). Hidden files in src are skipped so .env and the
// like are never imported.
func replaceDir(src, dst string) error {
	stage, err := os.MkdirTemp(filepath.Dir(dst), ".update-*")
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(stage)
		}
	}()
	ready, previous := filepath.Join(stage, "ready"), filepath.Join(stage, "previous")
	if err := copyDir(dst, ready); err != nil {
		return err
	}
	if err := replaceVisible(src, ready); err != nil {
		return err
	}
	if err := os.Rename(dst, previous); err != nil {
		return err
	}
	if err := os.Rename(ready, dst); err != nil {
		if rollbackErr := os.Rename(previous, dst); rollbackErr != nil {
			cleanup = false
			return fmt.Errorf("update: %v; previous state in %s: %w", err, previous, rollbackErr)
		}
		return err
	}
	return nil
}

func replaceVisible(src, dst string) error {
	entries, err := os.ReadDir(dst)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil || rel == "." {
			return relErr
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case info.IsDir():
			return os.MkdirAll(target, 0o755)
		case info.Mode().IsRegular():
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return fsutil.WriteAtomic(target, data, info.Mode().Perm())
		}
		return nil
	})
}

// backupDir archives a directory into <DataDir>/backups/<name>.<ts>.tar.gz.
func (s *Service) backupDir(dir, name string) error {
	data, err := tarGzDir(dir)
	if err != nil {
		return err
	}
	ts := time.Now().Format("20060102T150405.000000000")
	backupsDir := s.paths.BackupsDir()
	backup := filepath.Join(backupsDir, fmt.Sprintf("%s.%s.tar.gz", name, ts))
	if err := fsutil.WriteAtomic(backup, data, 0o600); err != nil {
		return err
	}
	_ = fsutil.RotateBackups(backupsDir, name+".", 20)
	return nil
}

func tarGzDir(dir string) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil || rel == "." {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() && !info.IsDir() {
			return nil // symlinks and special files are not backed up
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(tw, f); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Backup is a skill's .tar.gz backup file.
type Backup struct {
	SkillDir string
	Time     time.Time
	Path     string
}

// ListBackups lists the backups in BackupsDir(), newest first; an empty slice
// (no error) if the dir does not exist.
func (s *Service) ListBackups() ([]Backup, error) {
	entries, err := os.ReadDir(s.paths.BackupsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading backups directory: %w", err)
	}
	var out []Backup
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// format: <skillDir>.<ts>.tar.gz, ts = 20060102T150405 (15 chars)
		if !strings.HasSuffix(name, ".tar.gz") {
			continue
		}
		withoutExt := strings.TrimSuffix(name, ".tar.gz") // <skillDir>.<ts>
		// skillDir is kebab-case (no dots), so the first dot always separates it
		// from the timestamp.
		dot := strings.Index(withoutExt, ".")
		if dot < 0 {
			continue
		}
		skillDir := withoutExt[:dot]
		tsStr := withoutExt[dot+1:]
		// accepts seconds ("20060102T150405") and nanoseconds ("20060102T150405.000000000")
		t, err := time.ParseInLocation("20060102T150405.000000000", tsStr, time.Local)
		if err != nil {
			t, err = time.ParseInLocation("20060102T150405", tsStr, time.Local)
		}
		if err != nil {
			continue // not in the expected format
		}
		out = append(out, Backup{
			SkillDir: skillDir,
			Time:     t,
			Path:     filepath.Join(s.paths.BackupsDir(), name),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })
	return out, nil
}

// Restore restores a backup into the library, backing up the current state
// first if the skill exists. Agent symlinks point at the folder and keep
// working, so they are never touched.
func (s *Service) Restore(b Backup) error {
	if !safeSkillDir(b.SkillDir) {
		return fmt.Errorf("unsafe skill name: %q", b.SkillDir)
	}
	if err := os.MkdirAll(s.paths.LibraryDir(), 0o755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(s.paths.LibraryDir(), ".restore-*")
	if err != nil {
		return err
	}
	defer func() {
		if stage != "" {
			_ = os.RemoveAll(stage)
		}
	}()
	ready := filepath.Join(stage, "ready")
	if err := os.Mkdir(ready, 0o755); err != nil {
		return err
	}
	if err := extractTarGz(b.Path, ready); err != nil {
		return fmt.Errorf("restoring %s: %w", b.SkillDir, err)
	}
	libPath := filepath.Join(s.paths.LibraryDir(), b.SkillDir)
	previous := filepath.Join(stage, "previous")
	if _, err := os.Lstat(libPath); err == nil {
		if err := s.backupDir(libPath, b.SkillDir); err != nil {
			return fmt.Errorf("safety backup before restoring %s: %w", b.SkillDir, err)
		}
		if err := os.Rename(libPath, previous); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(ready, libPath); err != nil {
		if _, statErr := os.Lstat(previous); statErr == nil {
			if rollbackErr := os.Rename(previous, libPath); rollbackErr != nil {
				// Keep the previous state if even the rollback cannot finish.
				stage = ""
				return fmt.Errorf("restore: %v; previous state in %s: %w", err, previous, rollbackErr)
			}
		}
		return err
	}
	return nil
}

// extractTarGz extracts a .tar.gz into dst safely: clean paths, no ".." or
// absolute paths, no symlinks, 64 MB per entry (restoreBudget).
func extractTarGz(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening backup: %w", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("decompressing backup: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	budget := restoreBudget()
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}
		if err := budget.entry(hdr.Name); err != nil {
			return err
		}
		clean := filepath.Clean(hdr.Name)
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
			return fmt.Errorf("unsafe path in backup: %q", hdr.Name)
		}
		target := filepath.Join(dst, clean)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			data, err := budget.read(hdr.Name, tr)
			if err != nil {
				return err
			}
			perm := hdr.FileInfo().Mode().Perm()
			if err := fsutil.WriteAtomic(target, data, perm); err != nil {
				return err
			}
		}
	}
	if _, err := io.Copy(io.Discard, gr); err != nil {
		return fmt.Errorf("validating gzip: %w", err)
	}
	return nil
}

// MigrateLibrary copies and checks the targets before changing links/config;
// the old library is removed only after the new config is saved.
func (s *Service) MigrateLibrary(newDir string, agents []agent.Agent) error {
	oldDir, err := filepath.Abs(s.paths.LibraryDir())
	if err != nil {
		return err
	}
	newDir, err = filepath.Abs(newDir)
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(oldDir); err == nil {
		oldDir = resolved
	}
	if resolved, err := filepath.EvalSymlinks(newDir); err == nil {
		newDir = resolved
	}
	if oldDir == newDir {
		return nil
	}
	if insideDir(newDir, oldDir) || insideDir(oldDir, newDir) {
		return fmt.Errorf("libraries cannot contain each other")
	}
	cfg, err := core.ReadConfig(s.paths.ConfigPath())
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(oldDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	// Preflight: an existing name is never assumed to be already migrated.
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dst := filepath.Join(newDir, e.Name())
		if _, err := os.Lstat(dst); !os.IsNotExist(err) {
			return fmt.Errorf("destination already exists or is inaccessible: %s", dst)
		}
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(newDir); err == nil {
		newDir = resolved
	}
	if oldDir == newDir || insideDir(newDir, oldDir) || insideDir(oldDir, newDir) {
		return fmt.Errorf("libraries overlap after resolving symlinks")
	}
	var copied []string
	type linkChange struct{ path, target string }
	var links []linkChange
	committed := false
	defer func() {
		if committed {
			return
		}
		// If a link cannot be restored, keep the copy too so it stays valid.
		rollbackOK := true
		for i := len(links) - 1; i >= 0; i-- {
			l := links[i]
			if err := os.Remove(l.path); err != nil && !os.IsNotExist(err) {
				rollbackOK = false
				continue
			}
			if err := os.Symlink(l.target, l.path); err != nil {
				rollbackOK = false
			}
		}
		if rollbackOK {
			for _, name := range copied {
				_ = os.RemoveAll(filepath.Join(newDir, name))
			}
		}
	}()
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		src, dst := filepath.Join(oldDir, name), filepath.Join(newDir, name)
		if err := s.backupDir(src, name); err != nil {
			return err
		}
		copied = append(copied, name)
		if err := copyDir(src, dst); err != nil {
			return err
		}
	}
	var linkDirs []string // our links live in each agent's own dir and in shared dirs
	for _, ag := range agents {
		for _, dir := range []string{ag.ManagedDir, ag.SharedDir} {
			if dir != "" && !slices.Contains(linkDirs, dir) {
				linkDirs = append(linkDirs, dir)
			}
		}
	}
	for _, name := range copied {
		for _, dir := range linkDirs {
			path := filepath.Join(dir, name)
			target, err := os.Readlink(path)
			if err != nil {
				continue
			}
			resolved := target
			if !filepath.IsAbs(resolved) {
				resolved = filepath.Join(dir, target)
			}
			if physical, err := filepath.EvalSymlinks(resolved); err == nil {
				resolved = physical
			}
			if filepath.Clean(resolved) != filepath.Join(oldDir, name) {
				continue
			}
			if err := os.Remove(path); err != nil {
				return err
			}
			links = append(links, linkChange{path, target})
			if err := os.Symlink(filepath.Join(newDir, name), path); err != nil {
				return err
			}
		}
	}
	cfg.LibraryDir = newDir
	if err := cfg.Save(s.paths.ConfigPath()); err != nil {
		return err
	}
	committed = true
	s.paths.LibraryOverride = newDir
	for _, name := range copied {
		if err := os.RemoveAll(filepath.Join(oldDir, name)); err != nil {
			return fmt.Errorf("library migrated; removing old copy of %s: %w", name, err)
		}
	}
	return nil
}

// copyDir copies recursively, skipping symlinks (security: a malicious skill
// cannot pull in files from outside its folder).
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case info.IsDir():
			return os.MkdirAll(target, 0o755)
		case info.Mode().IsRegular():
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return fsutil.WriteAtomic(target, data, info.Mode().Perm())
		default:
			return nil // symlink/special file: skipped on purpose
		}
	})
}
