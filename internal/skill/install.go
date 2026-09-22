package skill

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
)

// Found é uma skill descoberta numa origem (pasta, zip ou repositório git),
// candidata a instalação na biblioteca.
type Found struct {
	SrcDir      string // pasta absoluta com o SKILL.md
	Rel         string // caminho relativo dentro da origem ("" = raiz)
	Name        string // nome proposto para a biblioteca (basename ou nome do repo)
	Description string
	Valid       bool
	Hidden      bool // encontrada sob um dir oculto (ex.: .openclaw) — despriorizada
	Depth       int
}

// originFile registra de onde a skill veio, dentro da própria pasta na
// biblioteca. Começa com "." de propósito: dirs ocultos ficam fora da
// descoberta, então o arquivo nunca vira "skill" nem é reinstalado.
const originFile = ".origin.json"

// Origin é a proveniência de uma skill da biblioteca — o que permite
// atualizá-la depois (M2.2). Ausente = criada/copiada manualmente.
type Origin struct {
	Type        string    `json:"type"`          // git | zip | dir
	Source      string    `json:"source"`        // URL ou caminho de origem
	Sub         string    `json:"sub,omitempty"` // subpasta da skill dentro da origem
	InstalledAt time.Time `json:"installedAt"`
	Hash        string    `json:"hash,omitempty"` // SHA-256 do conteúdo; vazio = desconhecido
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
		strings.HasPrefix(in, "git@"), strings.HasPrefix(in, "file://"),
		strings.HasSuffix(in, ".git"):
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
// A Origin retornada deve ser repassada ao Install para ficar registrada.
func (s *Service) Discover(input string) (found []Found, origin Origin, cleanupDir string, err error) {
	in := expandHome(strings.TrimSpace(input), s.paths.Home)
	switch DetectSource(in) {
	case SourceGit:
		tmp, err := cloneShallow(in)
		if err != nil {
			return nil, Origin{}, "", err
		}
		f, err := discoverIn(tmp, repoName(in))
		return f, Origin{Type: "git", Source: normalizeGitURL(in)}, tmp, err
	case SourceZip:
		tmp, err := extractZip(in)
		if err != nil {
			return nil, Origin{}, "", err
		}
		base := strings.TrimSuffix(filepath.Base(in), filepath.Ext(in))
		f, err := discoverIn(tmp, base)
		return f, Origin{Type: "zip", Source: in}, tmp, err
	default:
		f, err := discoverIn(in, filepath.Base(filepath.Clean(in)))
		return f, Origin{Type: "dir", Source: in}, "", err
	}
}

// Install copia as skills escolhidas para a biblioteca e registra a origem
// de cada uma em .origin.json. Retorna os nomes instalados; skills já
// existentes na biblioteca geram erro individual.
func (s *Service) Install(chosen []Found, origin Origin) (installed []string, err error) {
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
		o := origin
		o.Sub = f.Rel
		o.InstalledAt = time.Now()
		if h, hErr := hashDir(dst); hErr == nil {
			o.Hash = h
		}
		if wErr := writeOrigin(dst, o); wErr != nil {
			errs = append(errs, fmt.Sprintf("%s (origem): %v", f.Name, wErr))
		}
		installed = append(installed, f.Name)
	}
	if len(errs) > 0 {
		return installed, fmt.Errorf("instalação parcial: %s", strings.Join(errs, "; "))
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

// normalizeGitURL expande a forma curta usuario/repo para a URL completa,
// igual ao cloneShallow, para a origem registrada ser clonável depois.
func normalizeGitURL(url string) string {
	if !strings.Contains(url, "://") && !strings.HasPrefix(url, "git@") {
		return "https://github.com/" + strings.TrimSuffix(url, "/")
	}
	return url
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

// cloneShallow clona o repositório com --depth 1 e hooks neutralizados (nunca
// executa nada do conteúdo clonado).
func cloneShallow(url string) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("instalação via GitHub requer git no PATH")
	}
	url = normalizeGitURL(url)
	tmp, err := os.MkdirTemp("", "lazyagents-git-*")
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
	tmp, err := os.MkdirTemp("", "lazyagents-zip-*")
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
