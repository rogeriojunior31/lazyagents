package skill

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
	"strings"
	"time"

	"lazyskills/internal/agent"
	"lazyskills/internal/fsutil"
)

var (
	ErrNotInLibrary = errors.New("skill não está na biblioteca do lazyskills — adote-a primeiro (tecla o)")
	ErrLocalSkill   = errors.New("skill local não gerenciada pelo lazyskills")
	ErrNoSkillsDir  = errors.New("agente não tem diretório de skills gerenciável")
	ErrSkillExists  = errors.New("já existe uma skill com esse nome")
	ErrNoGitOrigin  = errors.New("skill não tem origem git — só skills instaladas do GitHub podem ser atualizadas")
)

// skillNameRe valida nomes de skill: kebab-case, como os agentes esperam
// (1-64 chars, minúsculas/números/hífens, casando com o nome da pasta).
var skillNameRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Create cria uma skill nova na biblioteca com um SKILL.md de template e
// devolve a pasta criada, pronta para abrir no editor.
func (s *Service) Create(name string) (string, error) {
	if !skillNameRe.MatchString(name) || len(name) > 64 {
		return "", fmt.Errorf("nome inválido %q: use kebab-case (minúsculas, números e hífens)", name)
	}
	dir := filepath.Join(s.paths.LibraryDir(), name)
	if _, err := os.Lstat(dir); err == nil {
		return "", fmt.Errorf("criando %s: %w", name, ErrSkillExists)
	}
	tmpl := fmt.Sprintf(`---
name: %s
description: TODO descreva o que a skill faz e QUANDO o agente deve usá-la
---

# %s

Instruções para o agente seguir quando a skill for ativada.
`, name, name)
	if err := fsutil.WriteAtomic(filepath.Join(dir, "SKILL.md"), []byte(tmpl), 0o644); err != nil {
		return "", fmt.Errorf("criando %s: %w", name, err)
	}
	return dir, nil
}

// Enable ativa a skill no agente: symlink biblioteca → ManagedDir do agente.
func (s *Service) Enable(sk Skill, ag agent.Agent) error {
	if !ag.SupportsSkills() {
		return fmt.Errorf("%s: %w", ag.Name, ErrNoSkillsDir)
	}
	if !sk.InLibrary {
		return ErrNotInLibrary
	}
	if st := sk.States[ag.ID]; st.On {
		return nil // já visível (gerenciada, compartilhada ou local)
	}
	target := filepath.Join(ag.ManagedDir, sk.Dir)
	if _, err := os.Lstat(target); err == nil {
		return fmt.Errorf("ativando %s em %s: %w em %s", sk.Dir, ag.Name, ErrSkillExists, target)
	}
	if err := os.MkdirAll(ag.ManagedDir, 0o755); err != nil {
		return fmt.Errorf("criando %s: %w", ag.ManagedDir, err)
	}
	src := filepath.Join(s.paths.LibraryDir(), sk.Dir)
	if err := os.Symlink(src, target); err != nil {
		return fmt.Errorf("ativando %s em %s: %w", sk.Dir, ag.Name, err)
	}
	return nil
}

// Disable desativa a skill no agente removendo o symlink gerenciado. Skills
// locais (dir real ou symlink de terceiros) não são tocadas.
func (s *Service) Disable(sk Skill, ag agent.Agent) error {
	st := sk.States[ag.ID]
	if !st.On {
		return nil
	}
	if st.Local {
		return fmt.Errorf("desativando %s em %s: %w (em %s)", sk.Dir, ag.Name, ErrLocalSkill, st.Via)
	}
	target := filepath.Join(st.Via, sk.Dir)
	info, err := os.Lstat(target)
	if err != nil {
		return fmt.Errorf("desativando %s em %s: %w", sk.Dir, ag.Name, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("desativando %s em %s: %w", sk.Dir, ag.Name, ErrLocalSkill)
	}
	if err := os.Remove(target); err != nil {
		return fmt.Errorf("desativando %s em %s: %w", sk.Dir, ag.Name, err)
	}
	return nil
}

// EnableAll ativa a skill em todos os agentes instalados com dir de skills.
func (s *Service) EnableAll(sk Skill, agents []agent.Agent) error {
	var errs []error
	for _, ag := range agents {
		if !ag.Installed || !ag.SupportsSkills() {
			continue
		}
		if err := s.Enable(sk, ag); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// DisableAll remove os symlinks gerenciados da skill em todos os agentes.
func (s *Service) DisableAll(sk Skill, agents []agent.Agent) error {
	var errs []error
	for _, ag := range agents {
		st := sk.States[ag.ID]
		if !st.On || st.Local {
			continue // skills locais ficam — nunca deletar conteúdo real
		}
		if err := s.Disable(sk, ag); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Adopt traz uma skill local (dir real no agente) para a biblioteca e troca o
// original por um symlink, sem perder a ativação. Faz backup .tar.gz antes.
func (s *Service) Adopt(sk Skill, ag agent.Agent) error {
	st := sk.States[ag.ID]
	if !st.Local || st.Via == "" {
		return fmt.Errorf("adotando %s: skill não é local em %s", sk.Dir, ag.Name)
	}
	if sk.InLibrary {
		return fmt.Errorf("adotando %s: %w na biblioteca", sk.Dir, ErrSkillExists)
	}
	src := filepath.Join(st.Via, sk.Dir)
	if info, err := os.Lstat(src); err != nil || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("adotando %s: origem é symlink de terceiros — gerencie pela ferramenta que o criou", sk.Dir)
	}
	dst := filepath.Join(s.paths.LibraryDir(), sk.Dir)
	if err := copyDir(src, dst); err != nil {
		return fmt.Errorf("adotando %s: %w", sk.Dir, err)
	}
	if err := writeOrigin(dst, Origin{Type: "dir", Source: src, InstalledAt: time.Now()}); err != nil {
		return fmt.Errorf("adotando %s (origem): %w", sk.Dir, err)
	}
	if err := s.backupDir(src, sk.Dir); err != nil {
		return fmt.Errorf("adotando %s (backup): %w", sk.Dir, err)
	}
	if err := os.RemoveAll(src); err != nil {
		return fmt.Errorf("adotando %s: %w", sk.Dir, err)
	}
	if err := os.Symlink(dst, src); err != nil {
		return fmt.Errorf("adotando %s (symlink de volta): %w", sk.Dir, err)
	}
	return nil
}

// Remove apaga a skill da biblioteca (com backup .tar.gz) e limpa os symlinks
// gerenciados que apontavam para ela em todos os agentes.
func (s *Service) Remove(sk Skill, agents []agent.Agent) error {
	if !sk.InLibrary {
		return ErrNotInLibrary
	}
	libPath := filepath.Join(s.paths.LibraryDir(), sk.Dir)
	if err := s.backupDir(libPath, sk.Dir); err != nil {
		return fmt.Errorf("removendo %s (backup): %w", sk.Dir, err)
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
					errs = append(errs, fmt.Errorf("limpando link em %s: %w", dir, err))
				}
			}
		}
	}
	if err := os.RemoveAll(libPath); err != nil {
		errs = append(errs, fmt.Errorf("removendo %s: %w", libPath, err))
	}
	return errors.Join(errs...)
}

// Update re-instala a skill da origem git, substituindo o conteúdo in-place
// para não quebrar os symlinks de ativação existentes nos agentes.
func (s *Service) Update(sk Skill) error {
	if sk.Origin == nil || sk.Origin.Type != "git" {
		return ErrNoGitOrigin
	}
	if !sk.InLibrary {
		return ErrNotInLibrary
	}
	tmp, err := cloneShallow(sk.Origin.Source)
	if err != nil {
		return fmt.Errorf("atualizando %s: %w", sk.Dir, err)
	}
	defer os.RemoveAll(tmp)

	srcDir, err := locateInClone(tmp, sk)
	if err != nil {
		return fmt.Errorf("atualizando %s: %w", sk.Dir, err)
	}
	libPath := filepath.Join(s.paths.LibraryDir(), sk.Dir)
	if err := s.backupDir(libPath, sk.Dir); err != nil {
		return fmt.Errorf("atualizando %s (backup): %w", sk.Dir, err)
	}
	if err := replaceDir(srcDir, libPath); err != nil {
		return fmt.Errorf("atualizando %s (cópia): %w", sk.Dir, err)
	}
	o := *sk.Origin
	o.InstalledAt = time.Now()
	if err := writeOrigin(libPath, o); err != nil {
		return fmt.Errorf("atualizando %s (origem): %w", sk.Dir, err)
	}
	return nil
}

// locateInClone localiza a pasta da skill dentro do clone tmp.
// Usa o Sub registrado na origem se válido; caso contrário redescobre.
func locateInClone(tmp string, sk Skill) (string, error) {
	if sk.Origin.Sub != "" {
		candidate := filepath.Join(tmp, sk.Origin.Sub)
		if _, err := os.Stat(filepath.Join(candidate, "SKILL.md")); err == nil {
			return candidate, nil
		}
	}
	found, err := discoverIn(tmp, sk.Dir)
	if err != nil {
		return "", fmt.Errorf("skill não encontrada no repositório: %w", err)
	}
	for _, f := range found {
		if f.Name == sk.Dir || f.Name == sk.Name {
			return f.SrcDir, nil
		}
	}
	if len(found) == 1 {
		return found[0].SrcDir, nil
	}
	return "", fmt.Errorf("skill %q não encontrada no repositório remoto", sk.Dir)
}

// replaceDir substitui o conteúdo visível de dst com o de src, preservando
// arquivos ocultos em dst (ex.: .origin.json). Arquivos ocultos do src são
// ignorados para não importar .env ou similares.
func replaceDir(src, dst string) error {
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

// backupDir compacta um diretório em ~/.lazyskills/backups/<nome>.<ts>.tar.gz.
func (s *Service) backupDir(dir, name string) error {
	data, err := tarGzDir(dir)
	if err != nil {
		return err
	}
	ts := time.Now().Format("20060102T150405")
	backup := filepath.Join(s.paths.BackupsDir(), fmt.Sprintf("%s.%s.tar.gz", name, ts))
	return fsutil.WriteAtomic(backup, data, 0o600)
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
			return nil // symlinks e especiais ficam fora do backup
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

// copyDir copia recursivamente ignorando symlinks (segurança: skill maliciosa
// não vaza arquivos de fora da própria pasta).
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
			return nil // symlink/especial: ignorado de propósito
		}
	})
}
