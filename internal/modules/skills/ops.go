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
	ErrNoGitOrigin  = errors.New("skill has no git source — only skills installed from GitHub can be updated")
)

// skillNameRe valida nomes de skill: kebab-case, como os agentes esperam
// (1-64 chars, minúsculas/números/hífens, casando com o nome da pasta).
var skillNameRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// safeSkillDir aceita um único componente, inclusive nomes legados fora do kebab-case.
func safeSkillDir(name string) bool {
	return filepath.IsLocal(name) && name != "." && !strings.ContainsAny(name, `/\`) && !strings.ContainsRune(name, 0)
}

// Create cria uma skill nova na biblioteca com um SKILL.md de template e
// devolve a pasta criada, pronta para abrir no editor.
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
		return fmt.Errorf("enabling %s in %s: %w at %s", sk.Dir, ag.Name, ErrSkillExists, target)
	}
	if err := os.MkdirAll(ag.ManagedDir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", ag.ManagedDir, err)
	}
	src := filepath.Join(s.paths.LibraryDir(), sk.Dir)
	if err := os.Symlink(src, target); err != nil {
		return fmt.Errorf("enabling %s in %s: %w", sk.Dir, ag.Name, err)
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
		return fmt.Errorf("disabling %s in %s: %w (in %s)", sk.Dir, ag.Name, ErrLocalSkill, st.Via)
	}
	target := filepath.Join(st.Via, sk.Dir)
	info, err := os.Lstat(target)
	if err != nil {
		return fmt.Errorf("disabling %s in %s: %w", sk.Dir, ag.Name, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("disabling %s in %s: %w", sk.Dir, ag.Name, ErrLocalSkill)
	}
	if err := os.Remove(target); err != nil {
		return fmt.Errorf("disabling %s in %s: %w", sk.Dir, ag.Name, err)
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

// AdoptAll adota todas as skills locais (fora da biblioteca) do scan, uma a
// uma. Falha em uma não aborta as demais — erros agregados, mesmo padrão do
// EnableAll/DisableAll.
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

// Remove apaga a skill da biblioteca (com backup .tar.gz) e limpa os symlinks
// gerenciados que apontavam para ela em todos os agentes.
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

// locateInClone localiza a pasta da skill dentro do clone tmp.
// Usa o Sub registrado na origem se válido; caso contrário redescobre.
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

// replaceDir substitui o conteúdo visível de dst com o de src, preservando
// arquivos ocultos em dst (ex.: .origin.json). Arquivos ocultos do src são
// ignorados para não importar .env ou similares.
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

// backupDir compacta um diretório em <DataDir>/backups/<nome>.<ts>.tar.gz.
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

// Backup descreve um arquivo de backup (.tar.gz) de uma skill na biblioteca.
type Backup struct {
	SkillDir string
	Time     time.Time
	Path     string
}

// ListBackups lista todos os backups disponíveis em BackupsDir(), ordenados do
// mais recente ao mais antigo. Retorna slice vazio (sem erro) se o dir não existe.
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
		// formato: <skillDir>.<ts>.tar.gz  ts = 20060102T150405 (15 chars)
		if !strings.HasSuffix(name, ".tar.gz") {
			continue
		}
		withoutExt := strings.TrimSuffix(name, ".tar.gz") // <skillDir>.<ts>
		// skillDir é kebab-case (sem pontos), portanto o primeiro ponto é sempre o
		// separador entre skillDir e timestamp — parse do início é mais robusto.
		dot := strings.Index(withoutExt, ".")
		if dot < 0 {
			continue
		}
		skillDir := withoutExt[:dot]
		tsStr := withoutExt[dot+1:]
		// aceita tanto segundo ("20060102T150405") quanto nanossegundo ("20060102T150405.000000000")
		t, err := time.ParseInLocation("20060102T150405.000000000", tsStr, time.Local)
		if err != nil {
			t, err = time.ParseInLocation("20060102T150405", tsStr, time.Local)
		}
		if err != nil {
			continue // arquivo não segue o formato esperado
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

// Restore restaura um backup para a biblioteca. Se a skill já existir, faz
// safety backup do estado atual antes de sobrescrever. Nunca mexe em symlinks
// de agentes — eles apontam para a pasta e continuam funcionando depois.
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
				// Preserve o estado anterior se nem o rollback puder ser concluído.
				stage = ""
				return fmt.Errorf("restore: %v; previous state in %s: %w", err, previous, rollbackErr)
			}
		}
		return err
	}
	return nil
}

// extractTarGz extrai um arquivo .tar.gz para dst com proteções de segurança:
// paths limpos, sem ".." nem absolutos, sem symlinks, limite de 64 MB por entrada.
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
	const maxEntry = 64 << 20 // 64 MB
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
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
			data, err := io.ReadAll(io.LimitReader(tr, maxEntry+1))
			if err != nil {
				return fmt.Errorf("reading entry %s: %w", hdr.Name, err)
			}
			if int64(len(data)) > maxEntry {
				return fmt.Errorf("entry %s exceeds 64 MB", hdr.Name)
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

// MigrateLibrary copia e valida os destinos antes de mudar links/configuração.
// A origem só é removida depois de a nova configuração estar salva.
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
	// Preflight: um nome existente nunca é considerado implicitamente migrado.
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
		// Se um link não puder voltar, preserve também a cópia para ele continuar válido.
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
	for _, name := range copied {
		for _, ag := range agents {
			if ag.ManagedDir == "" {
				continue
			}
			path := filepath.Join(ag.ManagedDir, name)
			target, err := os.Readlink(path)
			if err != nil {
				continue
			}
			resolved := target
			if !filepath.IsAbs(resolved) {
				resolved = filepath.Join(ag.ManagedDir, target)
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
