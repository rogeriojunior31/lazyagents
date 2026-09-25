package skills

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// UpdateStatus is the update state of a git skill.
type UpdateStatus int

const (
	UpdateStatusUnknown       UpdateStatus = iota // missing hash or check error
	UpdateStatusUpToDate                          // same content as the remote
	UpdateStatusAvailable                         // newer version on the remote
	UpdateStatusLocallyEdited                     // local content differs from the saved hash
)

// UpdateCheck is the check result for one skill.
type UpdateCheck struct {
	Skill  Skill
	Status UpdateStatus
	Err    error
}

// hashDir is a stable SHA-256 of dir: files in lexicographic order, hidden
// entries skipped, each regular file as rel + "\x00" + content.
func hashDir(dir string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
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
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\x00", filepath.ToSlash(rel))
		h.Write(data)
		return nil
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// CheckUpdates checks git-sourced skills for updates, cloning each source URL
// once. Per-skill errors go in UpdateCheck.Err; the slice is always returned.
func (s *Service) CheckUpdates(skills []Skill) ([]UpdateCheck, error) {
	var gitSkills []Skill
	for _, sk := range skills {
		if sk.Origin != nil && sk.Origin.Type == "git" && sk.InLibrary {
			gitSkills = append(gitSkills, sk)
		}
	}
	if len(gitSkills) == 0 {
		return nil, nil
	}

	// group by source to clone each repo once
	bySource := make(map[string][]Skill)
	var order []string
	seen := make(map[string]bool)
	for _, sk := range gitSkills {
		src := sk.Origin.Source
		if !seen[src] {
			seen[src] = true
			order = append(order, src)
		}
		bySource[src] = append(bySource[src], sk)
	}

	var out []UpdateCheck
	for _, source := range order {
		tmp, cloneErr := cloneShallow(source)
		for _, sk := range bySource[source] {
			if cloneErr != nil {
				out = append(out, UpdateCheck{Skill: sk, Status: UpdateStatusUnknown, Err: cloneErr})
				continue
			}
			out = append(out, s.checkOne(tmp, sk))
		}
		if tmp != "" {
			_ = os.RemoveAll(tmp)
		}
	}
	return out, nil
}

func (s *Service) checkOne(cloneDir string, sk Skill) UpdateCheck {
	libPath := filepath.Join(s.paths.LibraryDir(), sk.Dir)
	localHash, err := hashDir(libPath)
	if err != nil {
		return UpdateCheck{Skill: sk, Status: UpdateStatusUnknown,
			Err: fmt.Errorf("local hash: %w", err)}
	}

	// locally edited: current hash differs from the one saved on install/update
	if sk.Origin.Hash != "" && localHash != sk.Origin.Hash {
		return UpdateCheck{Skill: sk, Status: UpdateStatusLocallyEdited}
	}

	srcDir, err := locateInClone(cloneDir, sk)
	if err != nil {
		return UpdateCheck{Skill: sk, Status: UpdateStatusUnknown,
			Err: fmt.Errorf("locating in the clone: %w", err)}
	}
	remoteHash, err := hashDir(srcDir)
	if err != nil {
		return UpdateCheck{Skill: sk, Status: UpdateStatusUnknown,
			Err: fmt.Errorf("remote hash: %w", err)}
	}

	if localHash == remoteHash {
		return UpdateCheck{Skill: sk, Status: UpdateStatusUpToDate}
	}
	return UpdateCheck{Skill: sk, Status: UpdateStatusAvailable}
}

// UpdateAll updates every skill with UpdateStatusAvailable. Locally edited
// skills are skipped (returned in skipped); one failure does not stop the rest.
func (s *Service) UpdateAll(checks []UpdateCheck) (updated int, skipped []string, errs []error) {
	for _, c := range checks {
		switch c.Status {
		case UpdateStatusAvailable:
			if err := s.Update(c.Skill); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", c.Skill.Dir, err))
			} else {
				updated++
			}
		case UpdateStatusLocallyEdited:
			skipped = append(skipped, c.Skill.Dir)
		}
	}
	return
}
