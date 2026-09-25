package skills

// Skill search over GitHub repos. The skills.sh registry API (agentskills.io)
// needs a Vercel OIDC token, unusable from a local CLI, so this uses GitHub code
// search via `gh api` (auth already set up by the user, no new dependency).
// Installing reuses the Discover/Install flow.

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

// RegistryResult is a candidate repo found by the search.
type RegistryResult struct {
	Repo        string // owner/repo
	Path        string // path of the SKILL.md inside the repo
	Description string // repo description (code search has no per-skill description)
	URL         string
}

// searchCodeRunner runs the search and returns the raw JSON; a package var so
// tests avoid the network.
var searchCodeRunner = runGHSearchCode

// SearchRegistry finds repos with a SKILL.md containing term. Only on explicit
// user action — never on Scan or startup.
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

// parseSearchCodeResponse extracts the unique repos from a GitHub /search/code
// response, sorted by name.
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
