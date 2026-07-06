package skill

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"lazyskills/internal/fsutil"
)

// Found é uma skill descoberta numa origem (pasta, zip ou repositório git),
// candidata a instalação na biblioteca.
type Found struct {
	SrcDir      string // pasta absoluta com o SKILL.md
	Name        string // nome proposto para a biblioteca (basename ou nome do repo)
	Description string
	Valid       bool
	Hidden      bool // encontrada sob um dir oculto (ex.: .openclaw) — despriorizada
	Depth       int
}

// Source é o tipo de origem detectado a partir do texto digitado pelo usuário.
type Source int

const (
	SourceDir Source = iota
	SourceZip
	SourceGit
)

// DetectSource classifica a entrada: URL/spec git, arquivo .zip ou pasta.
func DetectSource(input string) Source {
	in := strings.TrimSpace(input)
	switch {
	case strings.HasPrefix(in, "http://"), strings.HasPrefix(in, "https://"),
		strings.HasPrefix(in, "git@"), strings.HasSuffix(in, ".git"):
		return SourceGit
	case strings.HasSuffix(strings.ToLower(in), ".zip"):
		return SourceZip
	default:
		// "usuario/repo" curto sem existir no disco → GitHub
		if !strings.HasPrefix(in, "/") && !strings.HasPrefix(in, "~") &&
			strings.Count(in, "/") == 1 {
			if _, err := os.Stat(in); err != nil {
				return SourceGit
			}
		}
		return SourceDir
	}
}

// Discover encontra skills numa origem qualquer. Para zip e git a origem é
// materializada num diretório temporário (retornado em cleanupDir para o
// chamador remover após Install). Para pasta local, cleanupDir é "".
func (s *Service) Discover(input string) (found []Found, cleanupDir string, err error) {
	in := expandHome(strings.TrimSpace(input), s.paths.Home)
	switch DetectSource(in) {
	case SourceGit:
		tmp, err := cloneShallow(in)
		if err != nil {
			return nil, "", err
		}
		f, err := discoverIn(tmp, repoName(in))
		return f, tmp, err
	case SourceZip:
		tmp, err := extractZip(in)
		if err != nil {
			return nil, "", err
		}
		base := strings.TrimSuffix(filepath.Base(in), filepath.Ext(in))
		f, err := discoverIn(tmp, base)
		return f, tmp, err
	default:
		f, err := discoverIn(in, filepath.Base(filepath.Clean(in)))
		return f, "", err
	}
}

// Install copia as skills escolhidas para a biblioteca. Retorna os nomes
// instalados; skills já existentes na biblioteca geram erro individual.
func (s *Service) Install(chosen []Found) (installed []string, err error) {
	var errs []string
	for _, f := range chosen {
		dst := filepath.Join(s.paths.LibraryDir(), f.Name)
		if _, statErr := os.Lstat(dst); statErr == nil {
			errs = append(errs, fmt.Sprintf("%s: já existe na biblioteca", f.Name))
			continue
		}
		if copyErr := copyDir(f.SrcDir, dst); copyErr != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", f.Name, copyErr))
			continue
		}
		installed = append(installed, f.Name)
	}
	if len(errs) > 0 {
		return installed, fmt.Errorf("instalação parcial: %s", strings.Join(errs, "; "))
	}
	return installed, nil
}

// discoverIn acha todas as skills sob root, aceitando qualquer layout de
// repositório: SKILL.md na raiz, skills/<nome>/SKILL.md, categorias aninhadas
// (skills/eng/foo/SKILL.md) e afins. Duplicatas por nome são resolvidas
// preferindo caminhos não-ocultos e mais rasos (ex.: skills/x vence .openclaw/skills/x).
func discoverIn(root, rootName string) ([]Found, error) {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("origem %s não é uma pasta acessível", root)
	}
	if _, err := os.Stat(filepath.Join(root, "SKILL.md")); err == nil {
		f := newFound(root, rootName, false, 0)
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
		all = append(all, newFound(path, name, hidden, strings.Count(rel, string(filepath.Separator))))
		return filepath.SkipDir // skill não contém outra skill
	})
	if walkErr != nil {
		return nil, walkErr
	}
	// dedupe por nome: não-oculto vence oculto; empate → mais raso
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
		return nil, fmt.Errorf("nenhum SKILL.md encontrado em %s", root)
	}
	return out, nil
}

func better(a, b Found) bool {
	if a.Hidden != b.Hidden {
		return !a.Hidden
	}
	return a.Depth < b.Depth
}

func newFound(dir, name string, hidden bool, depth int) Found {
	f := Found{SrcDir: dir, Name: name, Hidden: hidden, Depth: depth}
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

// cloneShallow clona o repositório com --depth 1 e hooks neutralizados (nunca
// executa nada do conteúdo clonado).
func cloneShallow(url string) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("instalação via GitHub requer git no PATH")
	}
	if !strings.Contains(url, "://") && !strings.HasPrefix(url, "git@") {
		url = "https://github.com/" + strings.TrimSuffix(url, "/")
	}
	tmp, err := os.MkdirTemp("", "lazyskills-git-*")
	if err != nil {
		return "", fmt.Errorf("criando temporário: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1",
		"-c", "core.hooksPath=/dev/null", url, tmp)
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

// extractZip descompacta com proteção contra zip-slip; symlinks são ignorados.
func extractZip(path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("abrindo zip %s: %w", path, err)
	}
	defer r.Close()
	tmp, err := os.MkdirTemp("", "lazyskills-zip-*")
	if err != nil {
		return "", fmt.Errorf("criando temporário: %w", err)
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
			return "", fmt.Errorf("extraindo %s: %w", f.Name, err)
		}
		data, err := io.ReadAll(io.LimitReader(rc, 64<<20)) // 64 MB por arquivo
		rc.Close()
		if err != nil {
			os.RemoveAll(tmp)
			return "", fmt.Errorf("extraindo %s: %w", f.Name, err)
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
