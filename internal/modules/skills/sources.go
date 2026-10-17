package skills

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/procutil"
)

type repository struct {
	Name   string `yaml:"name"`
	URL    string `yaml:"url"`
	Ref    string `yaml:"ref,omitempty"`
	Subdir string `yaml:"subdir,omitempty"`
}

func validateGitRef(ref string) error {
	return validateGitRefContext(context.Background(), ref)
}

func validateGitRefContext(parent context.Context, ref string) error {
	if ref == "" {
		return nil
	}
	if strings.HasPrefix(ref, "-") || strings.ContainsFunc(ref, unicode.IsControl) {
		return fmt.Errorf("invalid branch/tag %q", ref)
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	if _, err := procutil.Output(exec.CommandContext(ctx, "git", "check-ref-format", "refs/heads/"+ref)); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("invalid branch/tag %q (git must be available)", ref)
	}
	return nil
}

func normalizeRepository(r repository) (repository, error) {
	r.Name, r.URL, r.Ref, r.Subdir = strings.TrimSpace(r.Name), strings.TrimSpace(r.URL), strings.TrimSpace(r.Ref), strings.TrimSpace(r.Subdir)
	if r.Name == "" || len(r.Name) > 64 || strings.ContainsFunc(r.Name, unicode.IsControl) {
		return r, fmt.Errorf("source name must contain 1-64 printable characters")
	}
	if r.URL == "" || strings.ContainsFunc(r.URL, unicode.IsSpace) || strings.ContainsFunc(r.URL, unicode.IsControl) {
		return r, fmt.Errorf("enter a Git URL or owner/repo without whitespace")
	}
	if !strings.Contains(r.URL, ":") && strings.Count(r.URL, "/") == 1 && !strings.HasPrefix(r.URL, "/") && !strings.HasPrefix(r.URL, ".") {
		r.URL = normalizeGitURL(r.URL)
	}
	if strings.HasPrefix(r.URL, "git@") && strings.Contains(r.URL, ":") {
		// SCP-style SSH URLs are passed as one Git argument.
	} else {
		u, err := url.Parse(r.URL)
		if err != nil {
			return r, fmt.Errorf("invalid Git URL")
		}
		if _, hasPassword := u.User.Password(); hasPassword {
			return r, fmt.Errorf("use Git credential helpers instead of a password in the source URL")
		}
		switch u.Scheme {
		case "https", "http", "ssh", "git":
			if u.Host == "" || u.Path == "" {
				return r, fmt.Errorf("Git URL requires a host and repository path")
			}
		case "file":
			if u.Path == "" {
				return r, fmt.Errorf("file URL requires a repository path")
			}
		default:
			return r, fmt.Errorf("use an HTTP, SSH, Git or file Git URL")
		}
		if u.RawQuery != "" || u.Fragment != "" {
			return r, fmt.Errorf("put the branch/tag in Ref, not the Git URL")
		}
	}
	if strings.HasPrefix(r.Ref, "-") || strings.ContainsFunc(r.Ref, unicode.IsControl) {
		return r, fmt.Errorf("invalid branch/tag")
	}
	if r.Subdir == "." {
		r.Subdir = ""
	}
	if r.Subdir != "" {
		if strings.ContainsAny(r.Subdir, `\:`) || strings.ContainsFunc(r.Subdir, unicode.IsControl) || strings.HasPrefix(r.Subdir, "/") || slices.Contains(strings.Split(r.Subdir, "/"), "..") {
			return r, fmt.Errorf("source subfolder must stay inside the repository; use forward slashes")
		}
		r.Subdir = path.Clean(r.Subdir)
	}
	return r, nil
}

func (s *Service) repositories() ([]repository, error) {
	cfg, err := core.ReadConfig(s.paths.ConfigPath())
	if err != nil {
		return nil, err
	}
	return configRepositories(cfg)
}

func configRepositories(cfg core.Config) ([]repository, error) {
	var settings struct {
		Sources []repository `yaml:"sources"`
	}
	if err := cfg.Section("skills", &settings); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, r := range settings.Sources {
		if strings.TrimSpace(r.Name) == "" || seen[strings.ToLower(r.Name)] {
			return nil, fmt.Errorf("source names must be nonempty and unique")
		}
		seen[strings.ToLower(r.Name)] = true
	}
	return settings.Sources, nil
}

// expected is a descriptor snapshot: an external edit must not be overwritten.
func (s *Service) saveRepository(r repository, expected *repository, remove bool) error {
	if remove && expected == nil {
		return fmt.Errorf("removing a source requires its reviewed descriptor")
	}
	var err error
	if !remove {
		r, err = normalizeRepository(r)
		if err != nil {
			return err
		}
		if err = validateGitRef(r.Ref); err != nil {
			return err
		}
	}
	cfg, err := core.ReadConfig(s.paths.ConfigPath())
	if err != nil {
		return err
	}
	list, err := configRepositories(cfg)
	if err != nil {
		return err
	}
	index := slices.IndexFunc(list, func(item repository) bool { return strings.EqualFold(item.Name, r.Name) })
	if expected == nil {
		if index >= 0 {
			return fmt.Errorf("source %q already exists", r.Name)
		}
		list = append(list, r)
	} else {
		if index < 0 || list[index] != *expected {
			return fmt.Errorf("source changed since review; reload sources")
		}
		if remove {
			list = slices.Delete(list, index, index+1)
		} else {
			list[index] = r
		}
	}
	if list == nil {
		list = []repository{}
	}
	if err = cfg.SetSectionValue("skills", "sources", list); err != nil {
		return err
	}
	return cfg.Save(s.paths.ConfigPath())
}

// Resolve physical paths on both sides (/var is a symlink on macOS).
func repositoryPath(root, rel string) (string, error) {
	physicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	dir, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil || !insideDir(dir, physicalRoot) {
		return "", fmt.Errorf("source path %q is missing or outside the repository", rel)
	}
	return dir, nil
}

func (s *Service) scanRepository(r repository) (found []Found, origin Origin, cleanup string, err error) {
	return s.scanRepositoryContext(context.Background(), r)
}

func (s *Service) scanRepositoryContext(ctx context.Context, r repository) (found []Found, origin Origin, cleanup string, err error) {
	return s.scanRepositoryProgress(ctx, r, nil)
}

func (s *Service) scanRepositoryProgress(ctx context.Context, r repository, notify func(string)) (found []Found, origin Origin, cleanup string, err error) {
	r, err = normalizeRepository(r)
	if err != nil {
		return
	}
	progress(notify, "Cloning source "+r.Name)
	cleanup, origin.Revision, err = cloneSourceContext(ctx, r.URL, r.Ref)
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(cleanup)
			cleanup = ""
		}
	}()
	origin.Type, origin.Source, origin.Ref, origin.ExactPath = "git", r.URL, r.Ref, true
	root, rootErr := repositoryPath(cleanup, "")
	if rootErr != nil {
		err = rootErr
		return
	}
	sub, subErr := repositoryPath(root, r.Subdir)
	if subErr != nil {
		err = subErr
		return
	}
	progress(notify, "Discovering source "+r.Name)
	found, err = discoverSourceAtContext(ctx, sub, repoName(r.URL), &origin, true)
	if err != nil {
		return
	}
	counts := map[string]int{}
	for i := range found {
		if ctx.Err() != nil {
			err = ctx.Err()
			return
		}
		progress(notify, fmt.Sprintf("Checking source %s: entry %d/%d", r.Name, i+1, len(found)))
		f := &found[i]
		resolved, e := repositoryPath(root, repositoryRelative(root, f.SrcDir))
		if e != nil {
			err = e
			return
		}
		if f.Hook == nil {
			if _, e = repositoryPath(root, repositoryRelative(root, filepath.Join(resolved, "SKILL.md"))); e != nil {
				err = e
				return
			}
			counts[strings.ToLower(f.Name)]++
		}
		f.Rel = filepath.ToSlash(repositoryRelative(root, resolved))
	}
	for i := range found {
		found[i].Collision = found[i].Hook == nil && counts[strings.ToLower(found[i].Name)] > 1
	}
	return
}

func repositoryRelative(root, dir string) string {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return "../outside"
	}
	return rel
}
