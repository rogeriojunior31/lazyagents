package providers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
)

const connectionTimeout = 10 * time.Second
const catalogBodyLimit = 1 << 20
const catalogModelLimit = 10000

// ConnectionResult contains evidence from one request, never credentials or raw server errors.
type ConnectionResult struct {
	Operation   string   `json:"operation"`
	Reachable   bool     `json:"reachable"`
	HTTPStatus  int      `json:"http_status,omitempty"`
	Credential  string   `json:"credential"`
	Catalog     bool     `json:"catalog"`
	Models      []string `json:"models,omitempty"`
	ModelStatus string   `json:"model_status"`
	Error       string   `json:"error,omitempty"`
}

func connectionResult(catalog bool) ConnectionResult {
	r := ConnectionResult{Operation: "probe", Credential: "not_sent", ModelStatus: "not_checked"}
	if catalog {
		r.Operation = "models"
	}
	return r
}

func (s *Service) CheckSaved(ctx context.Context, name string, catalog bool) ConnectionResult {
	p, err := s.Profile(name)
	if err != nil {
		r := connectionResult(catalog)
		r.Error = "The saved profile could not be read; reload the library before checking"
		return r
	}
	return checkConnection(ctx, p, catalog)
}

// CheckDraft resolves an existing library token only for an explicit catalog request.
// A draft's typed token takes precedence; native login and helpers are never read.
func (s *Service) CheckDraft(ctx context.Context, p agent.ProviderProfile, original string, catalog bool) ConnectionResult {
	if catalog && p.Token == "" && p.HasToken {
		saved, err := s.Profile(original)
		if err != nil || saved.Token == "" {
			r := connectionResult(catalog)
			r.Error = "The saved credential is unavailable; re-enter it explicitly or reload the profile"
			return r
		}
		p.Token = saved.Token
	}
	return checkConnection(ctx, p, catalog)
}

func checkConnection(ctx context.Context, p agent.ProviderProfile, catalog bool) ConnectionResult {
	result := connectionResult(catalog)
	if err := agent.ValidateProviderProfile(p); err != nil {
		result.Error = "Use a valid HTTP(S) endpoint and credential reference before checking"
		return result
	}
	u, err := url.Parse(p.BaseURL)
	if err != nil || p.BaseURL == "" || u.RawQuery != "" || u.ForceQuery {
		result.Error = "Use an explicit API base URL without query parameters or embedded credentials"
		return result
	}
	method := http.MethodHead
	credential := ""
	if catalog {
		switch p.WireAPI {
		case "", "chat", "responses", "openai-completions", "openai-responses":
		default:
			result.Error = "Model discovery supports OpenAI-compatible catalogs only; enter the model manually for this protocol"
			return result
		}
		path := strings.TrimRight(u.Path, "/")
		if strings.HasSuffix(path, "/models") || strings.HasSuffix(path, "/responses") || strings.HasSuffix(path, "/chat/completions") || strings.HasSuffix(path, "/messages") {
			result.Error = "Use the API base URL without an operation path; enter a model manually if no catalog is supported"
			return result
		}
		u = u.JoinPath("models")
		method = http.MethodGet
		credential = p.Token
		if credential == "" && p.EnvKey != "" {
			credential = os.Getenv(p.EnvKey)
			if credential == "" {
				result.Error = "The credential environment variable is empty or unset in lazyagents; export it or enter a token explicitly"
				return result
			}
		}
		for _, c := range credential {
			if c < 33 || c > 126 {
				result.Error = "The credential cannot be used as an HTTP bearer token; re-enter it explicitly"
				return result
			}
		}
	}
	ctx, cancel := context.WithTimeout(ctx, connectionTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		result.Error = "The endpoint cannot be requested; check its URL"
		return result
	}
	req.Header.Set("Accept", "application/json")
	if credential != "" {
		req.Header.Set("Authorization", "Bearer "+credential)
		result.Credential = "configured"
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxResponseHeaderBytes = 64 << 10
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: connectionTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		result.Error = connectionError(err)
		return result
	}
	defer response.Body.Close()
	result.Reachable, result.HTTPStatus = true, response.StatusCode
	if credential != "" {
		result.Credential = "sent"
	}
	if !catalog {
		return result
	}
	switch {
	case response.StatusCode == http.StatusUnauthorized:
		result.Credential, result.Error = "rejected", "The catalog returned HTTP 401; authentication was not accepted"
		return result
	case response.StatusCode == http.StatusForbidden:
		result.Credential, result.Error = "forbidden", "The catalog returned HTTP 403; access was forbidden"
		return result
	case response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusMethodNotAllowed:
		result.Error = "No supported model catalog at this base URL; enter the model manually"
		return result
	case response.StatusCode >= 300 && response.StatusCode < 400:
		result.Error = "The catalog redirected; redirects are not followed, so credentials stay at the configured destination"
		return result
	case response.StatusCode < 200 || response.StatusCode >= 300:
		result.Error = "The catalog request failed; check the HTTP status or enter the model manually"
		return result
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, catalogBodyLimit+1))
	if err != nil {
		result.Error = connectionError(err)
		return result
	}
	if len(data) > catalogBodyLimit {
		result.Error = "The catalog exceeds the 1 MiB limit; enter the model manually"
		return result
	}
	var catalogData struct {
		Data *[]struct {
			ID string `json:"id"`
		} `json:"data"`
		HasMore bool `json:"has_more"`
	}
	if json.Unmarshal(data, &catalogData) != nil || catalogData.Data == nil || len(*catalogData.Data) > catalogModelLimit || catalogData.HasMore {
		result.Error = "The catalog is malformed, paginated or too large; enter the model manually"
		return result
	}
	ids := make([]string, 0, len(*catalogData.Data))
	for _, model := range *catalogData.Data {
		if !safeModelID(model.ID) || credential != "" && strings.Contains(model.ID, credential) {
			result.Error = "The catalog contains unsafe model IDs; enter the model manually"
			return result
		}
		ids = append(ids, model.ID)
	}
	slices.Sort(ids)
	result.Catalog, result.Models = true, slices.Compact(ids)
	result.ModelStatus = "not_selected"
	if p.Model != "" {
		result.ModelStatus = "not_listed"
		if slices.Contains(result.Models, p.Model) {
			result.ModelStatus = "listed"
		}
	}
	return result
}

func safeModelID(id string) bool {
	if id == "" || len(id) > 256 {
		return false
	}
	for _, c := range id {
		if c < 33 || c > 126 {
			return false
		}
	}
	return true
}

func connectionError(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "Request cancelled; no settings were changed"
	case errors.Is(err, context.DeadlineExceeded):
		return "Request timed out; no settings were changed"
	default:
		return "Connection or response failed; check the endpoint, network and TLS configuration"
	}
}
