package skills

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/core"
)

// UpdateStatus is the update state of a git skill.
type UpdateStatus int

const (
	UpdateStatusUnknown       UpdateStatus = iota // missing hash or check error
	UpdateStatusUpToDate                          // same content as the remote
	UpdateStatusAvailable                         // newer version on the remote
	UpdateStatusLocallyEdited                     // local content differs from the saved hash
)

var errLibraryLink = errors.New("library entry is a link; external content preserved")

func updateIsSkipped(err error) bool {
	return errors.Is(err, ErrNoGitOrigin) || errors.Is(err, ErrNotInLibrary) || errors.Is(err, errLibraryLink)
}

// UpdateCheck is the check result for one skill.
type UpdateCheck struct {
	Skill                           Skill
	Status                          UpdateStatus
	Err                             error
	Files                           []string
	Diff                            string
	LocalHash, RemoteHash, Revision string
	localChanged, upstreamChanged   *bool
	source                          string
}

// hashDir is a stable SHA-256 of dir: files in lexicographic order, hidden
// entries skipped, each regular file framed as its path and content digest.
func hashDir(dir string) (string, error) {
	return hashDirContext(context.Background(), dir)
}

func hashDirContext(ctx context.Context, dir string) (string, error) {
	return treeHashContext(ctx, dir, false)
}

// Old installs used unframed contents. Accept their baseline until the next
// install/update, while every reviewed snapshot uses the unambiguous digest.
func changedSinceInstall(ctx context.Context, dir, current, saved string) (*bool, error) {
	if saved == "" {
		return nil, nil
	}
	changed := false
	if current != saved {
		legacy, err := treeHashContext(ctx, dir, true)
		if err != nil {
			return nil, err
		}
		changed = legacy != saved
	}
	return &changed, nil
}

func treeHashContext(ctx context.Context, dir string, legacy bool) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return err
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\x00", filepath.ToSlash(rel))
		if legacy {
			_, err = io.Copy(h, contextReader{ctx, f})
		} else {
			content := sha256.New()
			_, err = io.Copy(content, contextReader{ctx, f})
			fmt.Fprintf(h, "%x\x00", content.Sum(nil))
		}
		f.Close()
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// CheckUpdates checks git-sourced skills for updates, cloning each source URL
// once per URL/ref. Per-skill errors go in UpdateCheck.Err.
func (s *Service) CheckUpdates(skills []Skill) ([]UpdateCheck, error) {
	var gitSkills []Skill
	for _, sk := range skills {
		if sk.InLibrary && sk.Origin != nil && sk.Origin.Type == "git" {
			gitSkills = append(gitSkills, sk)
		}
	}
	review, err := s.ReviewUpdates(context.Background(), gitSkills, nil)
	if review != nil {
		defer review.Close()
		return review.Checks, err
	}
	return nil, err
}

// UpdateReview owns the exact staged sources shown in a review. Close after
// applying or dismissing; application never downloads another revision.
type UpdateReview struct {
	Checks []UpdateCheck
	roots  []string
}

func (r *UpdateReview) Close() {
	for _, root := range r.roots {
		_ = os.RemoveAll(root)
	}
}

func progress(fn func(string), label string) {
	if fn != nil {
		fn(label)
	}
}

// ReviewUpdates groups clones by URL/ref and compares local and upstream
// content against the recorded install hash. Missing baselines stay explicit.
func (s *Service) ReviewUpdates(ctx context.Context, skills []Skill, notify func(string)) (*UpdateReview, error) {
	r := &UpdateReview{}
	type sourceRef struct{ url, ref string }
	type cloned struct {
		root, revision string
		err            error
	}
	sources := map[sourceRef]cloned{}
	for i, sk := range skills {
		if err := ctx.Err(); err != nil {
			r.Close()
			return nil, err
		}
		if sk.Origin != nil {
			o := *sk.Origin
			sk.Origin = &o
		}
		c := UpdateCheck{Skill: sk}
		if !sk.InLibrary {
			c.Err = ErrNotInLibrary
		} else if sk.Origin == nil || sk.Origin.Type != "git" {
			c.Err = ErrNoGitOrigin
		} else {
			key := sourceRef{sk.Origin.Source, sk.Origin.Ref}
			src, ok := sources[key]
			if !ok {
				progress(notify, fmt.Sprintf("Cloning source %s (ref: %s)…", key.url, key.ref))
				src.root, src.revision, src.err = cloneSourceContext(ctx, key.url, key.ref)
				sources[key] = src
				if src.root != "" {
					r.roots = append(r.roots, src.root)
				}
			}
			progress(notify, fmt.Sprintf("Reviewing skill %d/%d: %s", i+1, len(skills), sk.Dir))
			if src.err != nil {
				c.Err = src.err
			} else {
				c = s.checkOne(ctx, src.root, sk)
				c.Revision = src.revision
			}
		}
		r.Checks = append(r.Checks, c)
	}
	if err := ctx.Err(); err != nil {
		r.Close()
		return nil, err
	}
	return r, nil
}

func (s *Service) checkOne(ctx context.Context, cloneDir string, sk Skill) UpdateCheck {
	c := UpdateCheck{Skill: sk}
	if !safeSkillDir(sk.Dir) {
		c.Err = fmt.Errorf("unsafe library folder %q", sk.Dir)
		return c
	}
	lib := filepath.Join(s.paths.LibraryDir(), sk.Dir)
	info, err := os.Lstat(lib)
	if err == nil && !info.IsDir() {
		err = errLibraryLink
	}
	if err == nil {
		c.LocalHash, err = hashDirContext(ctx, lib)
	}
	if err == nil {
		c.localChanged, err = changedSinceInstall(ctx, lib, c.LocalHash, sk.Origin.Hash)
	}
	if err != nil {
		c.Err = fmt.Errorf("local content: %w", err)
		return c
	}
	c.source, err = locateInCloneContext(ctx, cloneDir, sk)
	if err == nil {
		c.RemoteHash, err = hashDirContext(ctx, c.source)
	}
	if err == nil {
		c.upstreamChanged, err = changedSinceInstall(ctx, c.source, c.RemoteHash, sk.Origin.Hash)
	}
	if err == nil {
		c.Files, c.Diff, err = skillDiff(ctx, lib, c.source)
	}
	if err != nil {
		c.Err = err
		return c
	}
	switch {
	case c.localChanged != nil && *c.localChanged:
		c.Status = UpdateStatusLocallyEdited
	case c.LocalHash == c.RemoteHash:
		c.Status = UpdateStatusUpToDate
	default:
		c.Status = UpdateStatusAvailable
	}
	return c
}

func (s *Service) applyReviewed(ctx context.Context, c UpdateCheck, force bool) error {
	if c.Err != nil {
		return c.Err
	}
	if c.source == "" {
		return fmt.Errorf("update staging is missing; review again")
	}
	if !force && (c.Skill.Origin.Hash == "" || c.Status == UpdateStatusLocallyEdited) {
		return fmt.Errorf("local edits or missing baseline; use single-skill review")
	}
	lib := filepath.Join(s.paths.LibraryDir(), c.Skill.Dir)
	unchanged := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := os.Lstat(lib)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("library entry changed to a link")
		}
		if !reflect.DeepEqual(readOrigin(lib), c.Skill.Origin) {
			return fmt.Errorf("recorded origin changed since review; review again")
		}
		h, err := hashDirContext(ctx, lib)
		if err != nil {
			return err
		}
		if h != c.LocalHash {
			return fmt.Errorf("local content changed since review; review again")
		}
		return nil
	}
	if err := unchanged(); err != nil {
		return err
	}
	h, err := hashDirContext(ctx, c.source)
	if err != nil {
		return err
	}
	if h != c.RemoteHash {
		return fmt.Errorf("staged source changed since review; review again")
	}
	if err := s.backupDirContext(ctx, lib, c.Skill.Dir); err != nil {
		return fmt.Errorf("backup: %w", err)
	}
	o := *c.Skill.Origin
	o.Revision, o.Hash, o.InstalledAt = c.Revision, c.RemoteHash, time.Now()
	return replaceDirContext(ctx, c.source, lib, &o, unchanged)
}

// ApplyUpdates reports every reviewed item. Cancellation finishes an atomic
// swap already started, then leaves subsequent items unchanged.
func (s *Service) ApplyUpdates(ctx context.Context, r *UpdateReview, force bool, notify func(string)) core.OperationResult {
	result := core.OperationResult{}
	if force && len(r.Checks) != 1 {
		result.Err = fmt.Errorf("explicit replacement requires exactly one reviewed skill")
		return result
	}
	for i, c := range r.Checks {
		target := core.TargetResult{ID: batchTarget(c.Skill.Dir, "library"), State: "unchanged"}
		switch {
		case ctx.Err() != nil:
			target.Outcome, target.Detail = core.Skipped, "cancelled before replacement"
		case updateIsSkipped(c.Err):
			target.Outcome, target.Detail = core.Skipped, c.Err.Error()
		case c.Err != nil:
			target.Outcome, target.Detail = core.Failed, c.Err.Error()
		case !force && (c.Skill.Origin.Hash == "" || c.Status == UpdateStatusLocallyEdited):
			target.Outcome, target.Detail = core.Skipped, "local edits or missing baseline; use single-skill review"
		case c.LocalHash == c.RemoteHash:
			target.Outcome, target.Detail = core.Skipped, "already matches upstream"
		default:
			progress(notify, fmt.Sprintf("Updating skill %d/%d: %s", i+1, len(r.Checks), c.Skill.Dir))
			err := s.applyReviewed(ctx, c, force)
			if err != nil {
				target.Outcome, target.Detail = core.Failed, err.Error()
			} else {
				target.Outcome, target.State = core.Succeeded, "reviewed revision installed; backup saved"
			}
		}
		result.Targets = append(result.Targets, target)
	}
	result.Err = ctx.Err()
	return result
}

// UpdateAll keeps the legacy service API while rechecking local edits before
// replacement. The TUI uses retained UpdateReview staging instead.
func (s *Service) UpdateAll(checks []UpdateCheck) (updated int, skipped []string, errs []error) {
	for _, c := range checks {
		switch c.Status {
		case UpdateStatusLocallyEdited:
			skipped = append(skipped, c.Skill.Dir)
		case UpdateStatusAvailable:
			r, err := s.ReviewUpdates(context.Background(), []Skill{c.Skill}, nil)
			if err == nil {
				next := r.Checks[0]
				if next.Status == UpdateStatusLocallyEdited || next.Skill.Origin.Hash == "" || (c.LocalHash != "" && c.LocalHash != next.LocalHash) {
					skipped = append(skipped, c.Skill.Dir)
					r.Close()
					continue
				}
				err = s.applyReviewed(context.Background(), next, false)
				r.Close()
			}
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", c.Skill.Dir, err))
			} else {
				updated++
			}
		case UpdateStatusUnknown:
			if c.Err != nil {
				errs = append(errs, c.Err)
			}
		}
	}
	return
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
