package skills

// Busca de skills em repositórios do GitHub. A API pública do
// registry skills.sh (agentskills.io) exige um token OIDC da Vercel — sem uso
// possível a partir de um CLI local (ver decisão da task). Em vez disso,
// reaproveitamos a busca de código do GitHub via `gh api`, que já tem
// autenticação resolvida pelo usuário (gh auth login) e não exige nenhuma
// dependência nova. O resultado é uma lista de repositórios candidatos; a
// instalação em si reusa o fluxo Discover/Install existente.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// RegistryResult é um repositório candidato encontrado na busca.
type RegistryResult struct {
	Repo        string // owner/repo
	Path        string // caminho do SKILL.md encontrado dentro do repo
	Description string // descrição do repositório (GitHub não expõe descrição por skill na busca de código)
	URL         string
}

// searchCodeRunner executa a busca e devolve o JSON bruto da resposta.
// Variável de pacote para ser substituída nos testes (sem rede real).
var searchCodeRunner = runGHSearchCode

// SearchRegistry busca repositórios com SKILL.md contendo o termo pedido.
// Só roda sob ação explícita do usuário — nunca no Scan nem no startup.
func (s *Service) SearchRegistry(term string) ([]RegistryResult, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return nil, fmt.Errorf("empty search term")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	data, err := searchCodeRunner(ctx, term)
	if err != nil {
		return nil, err
	}
	results, err := parseSearchCodeResponse(data)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no results for %q", term)
	}
	return results, nil
}

func runGHSearchCode(ctx context.Context, term string) ([]byte, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, fmt.Errorf("searching requires the GitHub CLI (gh) in PATH, logged in")
	}
	q := "filename:SKILL.md " + term
	cmd := exec.CommandContext(ctx, "gh", "api", "-X", "GET", "search/code",
		"-f", "q="+q, "-f", "per_page=30")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("GitHub search: %s", msg)
	}
	return stdout.Bytes(), nil
}

// parseSearchCodeResponse extrai os repositórios únicos da resposta da busca
// de código do GitHub (schema de /search/code), ordenados por nome.
func parseSearchCodeResponse(data []byte) ([]RegistryResult, error) {
	var resp struct {
		Items []struct {
			Path       string `json:"path"`
			HTMLURL    string `json:"html_url"`
			Repository struct {
				FullName    string `json:"full_name"`
				Description string `json:"description"`
			} `json:"repository"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unexpected GitHub response: %w", err)
	}
	seen := make(map[string]bool, len(resp.Items))
	out := make([]RegistryResult, 0, len(resp.Items))
	for _, it := range resp.Items {
		repo := it.Repository.FullName
		if repo == "" || seen[repo] {
			continue
		}
		seen[repo] = true
		out = append(out, RegistryResult{
			Repo:        repo,
			Path:        it.Path,
			Description: it.Repository.Description,
			URL:         it.HTMLURL,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Repo < out[j].Repo })
	return out, nil
}
