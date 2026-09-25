package skills

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
	"github.com/rogeriojunior31/lazyagents/internal/modules/hooks"
)

// Found is a skill discovered in a source (folder, zip or git repo), a
// candidate for the library.
type Found struct {
	SrcDir      string // absolute folder with the SKILL.md
	Rel         string // path relative to the source ("" = root)
	Name        string // proposed library name (basename or repo name)
	Description string
	Valid       bool
	Hidden      bool // found under a hidden dir (e.g. .openclaw): lower priority
	Depth       int
	Plugin      string // marketplace.json plugin declaring it ("" = generic discovery)
	// A non-nil Hook marks a hooks/hooks.json entry, not a skill: installed
	// into the hooks library by the same picker, since skill repos often ship
	// hooks too.
	Hook *hooks.Found
}

// originFile records where the skill came from, inside its library folder.
// The leading "." is deliberate: hidden dirs are skipped by discovery, so
// it never becomes a "skill" or gets reinstalled.
const originFile = ".origin.json"

// Origin is a library skill's provenance, which allows updating it later.
// Missing = created or copied by hand.
type Origin struct {
	Type        string    `json:"type"`          // git | zip | dir
	Source      string    `json:"source"`        // source URL or path
	Sub         string    `json:"sub,omitempty"` // skill subfolder inside the source
	InstalledAt time.Time `json:"installedAt"`
	Hash        string    `json:"hash,omitempty"` // content SHA-256; empty = unknown
	// Notes are discovery warnings for the user (e.g. marketplace plugins in
	// another repo); not persisted.
	Notes []string `json:"-"`
}

func readOrigin(skillDir string) *Origin {
	data, err := os.ReadFile(filepath.Join(skillDir, originFile))
	if err != nil {
		return nil
	}
	var o Origin
	if json.Unmarshal(data, &o) != nil || o.Type == "" {
		return nil
	}
	return &o
}

// Source is the source type detected from the user's input.
type Source int

const (
	SourceDir Source = iota
	SourceZip
	SourceGit
)

// DetectSource classifies the input: git URL/spec, .zip file or folder.
func DetectSource(input string) Source {
	in := strings.TrimSpace(input)
	switch {
	case strings.HasPrefix(in, "http://"), strings.HasPrefix(in, "https://"),
		strings.HasPrefix(in, "git@"), strings.HasPrefix(in, "file://"),
		strings.HasSuffix(in, ".git"):
		return SourceGit
	case strings.HasSuffix(strings.ToLower(in), ".zip"):
		return SourceZip
	default:
		// short "owner/repo" not on disk → GitHub
		if !strings.HasPrefix(in, "/") && !strings.HasPrefix(in, "~") &&
			strings.Count(in, "/") == 1 {
			if _, err := os.Stat(in); err != nil {
				return SourceGit
			}
		}
		return SourceDir
	}
}

// Discover finds skills in any source. Zip and git are materialized in a temp
// dir (cleanupDir, removed by the caller after Install); a local folder has
// cleanupDir "". Pass the returned Origin to Install so it gets recorded.
func (s *Service) Discover(input string) (found []Found, origin Origin, cleanupDir string, err error) {
	in := expandHome(strings.TrimSpace(input), s.paths.Home)
	switch DetectSource(in) {
	case SourceGit:
		tmp, err := cloneShallow(in)
		if err != nil {
			return nil, Origin{}, "", err
		}
		o := Origin{Type: "git", Source: normalizeGitURL(in)}
		f, err := discoverAt(tmp, repoName(in), &o)
		return f, o, tmp, err
	case SourceZip:
		tmp, err := extractZip(in)
		if err != nil {
			return nil, Origin{}, "", err
		}
		base := strings.TrimSuffix(filepath.Base(in), filepath.Ext(in))
		o := Origin{Type: "zip", Source: in}
		f, err := discoverAt(tmp, base, &o)
		return f, o, tmp, err
	default:
		o := Origin{Type: "dir", Source: in}
		f, err := discoverAt(in, filepath.Base(filepath.Clean(in)), &o)
		return f, o, "", err
	}
}

// discoverAt prefers the source's .claude-plugin/marketplace.json when it has
// skills, else the generic scan; plugin hooks from the same source are always
// listed. Warnings go to o.Notes.
func discoverAt(root, rootName string, o *Origin) ([]Found, error) {
	found, notes, ok, err := discoverMarketplace(root)
	o.Notes = notes
	if err != nil {
		return nil, err
	}
	var scanErr error
	if !ok {
		found, scanErr = discoverIn(root, rootName)
	}
	// A hooks-only source is valid (a hook plugin has no SKILL.md): the skill
	// scan error only counts when there is no hook either.
	hooked := foundHooks(root, rootName)
	if scanErr != nil && len(hooked) == 0 {
		return nil, scanErr
	}
	return append(found, hooked...), nil
}

// foundHooks turns the source's hooks/hooks.json into picker entries.
func foundHooks(root, rootName string) []Found {
	var out []Found
	for _, h := range hooks.DiscoverIn(root, rootName) {
		desc := h.Description
		if desc == "" {
			desc = strings.Join(h.Events(), ", ")
		}
		out = append(out, Found{
			SrcDir:      h.Dir,
			Rel:         h.Rel,
			Name:        h.Plugin,
			Description: desc,
			Valid:       true,
			Plugin:      h.Plugin,
			Hook:        &h,
		})
	}
	return out
}

// Install copies the chosen skills into the library and records each source
// in .origin.json; hook entries go to the hooks library with their scripts
// copied and paths rewritten. An item already in the library fails alone.
func (s *Service) Install(chosen []Found, origin Origin) (installed []string, err error) {
	var errs []string
	for _, f := range chosen {
		if f.Hook != nil {
			names, hErr := hooks.Import(s.paths, *f.Hook, origin.Source)
			installed = append(installed, names...)
			if hErr != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", f.Name, hErr))
			}
			continue
		}
		if !safeSkillDir(f.Name) {
			errs = append(errs, fmt.Sprintf("unsafe skill name: %q", f.Name))
			continue
		}
		dst := filepath.Join(s.paths.LibraryDir(), f.Name)
		if _, statErr := os.Lstat(dst); !os.IsNotExist(statErr) {
			errs = append(errs, fmt.Sprintf("%s: already in the library", f.Name))
			continue
		}
		if copyErr := copyDir(f.SrcDir, dst); copyErr != nil {
			_ = os.RemoveAll(dst)
			errs = append(errs, fmt.Sprintf("%s: %v", f.Name, copyErr))
			continue
		}
		o := origin
		o.Sub = f.Rel
		o.InstalledAt = time.Now()
		if h, hErr := hashDir(dst); hErr == nil {
			o.Hash = h
		}
		if wErr := writeOrigin(dst, o); wErr != nil {
			errs = append(errs, fmt.Sprintf("%s (source): %v", f.Name, wErr))
		}
		installed = append(installed, f.Name)
	}
	if len(errs) > 0 {
		return installed, fmt.Errorf("partial install: %s", strings.Join(errs, "; "))
	}
	return installed, nil
}

func writeOrigin(skillDir string, o Origin) error {
	data, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteAtomic(filepath.Join(skillDir, originFile), data, 0o644)
}

// normalizeGitURL expands owner/repo to the full URL, like cloneShallow, so
// the recorded source can be cloned later.
func normalizeGitURL(url string) string {
	if !strings.Contains(url, "://") && !strings.HasPrefix(url, "git@") {
		return "https://github.com/" + strings.TrimSuffix(url, "/")
	}
	return url
}

// discoverIn finds every skill under root in any repo layout (root SKILL.md,
// skills/<name>/SKILL.md, nested categories…). Duplicate names prefer
// non-hidden, shallower paths (skills/x beats .openclaw/skills/x).
func discoverIn(root, rootName string) ([]Found, error) {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("source %s is not an accessible folder", root)
	}
	if _, err := os.Stat(filepath.Join(root, "SKILL.md")); err == nil {
		f := newFound(root, "", rootName, false, 0)
		return []Found{f}, nil
	}
	var all []Found
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // best-effort
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		if path != root && (name == ".git" || name == "node_modules" || name == "vendor") {
			return filepath.SkipDir
		}
		if _, err := os.Stat(filepath.Join(path, "SKILL.md")); err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		hidden := false
		for part := range strings.SplitSeq(rel, string(filepath.Separator)) {
			if strings.HasPrefix(part, ".") {
				hidden = true
				break
			}
		}
		all = append(all, newFound(path, rel, name, hidden, strings.Count(rel, string(filepath.Separator))))
		return filepath.SkipDir // a skill does not contain another skill
	})
	if walkErr != nil {
		return nil, walkErr
	}
	// dedupe by name: non-hidden beats hidden; tie → shallower
	best := make(map[string]Found)
	for _, f := range all {
		cur, ok := best[f.Name]
		if !ok || better(f, cur) {
			best[f.Name] = f
		}
	}
	out := make([]Found, 0, len(best))
	for _, f := range best {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if len(out) == 0 {
		return nil, fmt.Errorf("no SKILL.md found in %s", root)
	}
	return out, nil
}

func better(a, b Found) bool {
	if a.Hidden != b.Hidden {
		return !a.Hidden
	}
	return a.Depth < b.Depth
}

func newFound(dir, rel, name string, hidden bool, depth int) Found {
	f := Found{SrcDir: dir, Rel: rel, Name: name, Hidden: hidden, Depth: depth}
	if data, err := os.ReadFile(filepath.Join(dir, "SKILL.md")); err == nil {
		if meta, ok := ParseMeta(data); ok {
			f.Valid = true
			f.Description = meta.Description
			if meta.Name != "" {
				f.Name = meta.Name
			}
		}
	}
	return f
}

// cloneShallow clones with --depth 1 and hooks disabled (never runs anything
// from the cloned content).
func cloneShallow(url string) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("installing from GitHub requires git in PATH")
	}
	url = normalizeGitURL(url)
	tmp, err := os.MkdirTemp("", "lazyagents-git-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1",
		"-c", "core.hooksPath="+os.DevNull, "--", url, tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(tmp)
		return "", fmt.Errorf("git clone %s: %s", url, strings.TrimSpace(string(out)))
	}
	os.RemoveAll(filepath.Join(tmp, ".git"))
	return tmp, nil
}

func repoName(url string) string {
	base := filepath.Base(strings.TrimSuffix(strings.TrimSuffix(url, "/"), ".git"))
	if base == "" || base == "." {
		return "skill"
	}
	return base
}

// extractZip unzips with zip-slip protection; symlinks are skipped.
func extractZip(path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("opening zip %s: %w", path, err)
	}
	defer r.Close()
	tmp, err := os.MkdirTemp("", "lazyagents-zip-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}
	for _, f := range r.File {
		mode := f.Mode()
		if mode&os.ModeSymlink != 0 || (!mode.IsRegular() && !f.FileInfo().IsDir()) {
			continue
		}
		clean := filepath.Clean(f.Name)
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			continue // zip-slip
		}
		target := filepath.Join(tmp, clean)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				os.RemoveAll(tmp)
				return "", err
			}
			continue
		}
		rc, err := f.Open()
		if err != nil {
			os.RemoveAll(tmp)
			return "", fmt.Errorf("extracting %s: %w", f.Name, err)
		}
		data, err := io.ReadAll(io.LimitReader(rc, (64<<20)+1)) // 64 MB per file
		rc.Close()
		if err != nil {
			os.RemoveAll(tmp)
			return "", fmt.Errorf("extracting %s: %w", f.Name, err)
		}
		if len(data) > 64<<20 {
			os.RemoveAll(tmp)
			return "", fmt.Errorf("entry %s exceeds 64 MB", f.Name)
		}
		perm := mode.Perm()
		if perm == 0 {
			perm = 0o644
		}
		if err := fsutil.WriteAtomic(target, data, perm); err != nil {
			os.RemoveAll(tmp)
			return "", err
		}
	}
	return tmp, nil
}

func expandHome(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}
