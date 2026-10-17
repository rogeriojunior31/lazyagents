package providers

import (
	"fmt"
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
)

// Adoption is an editable public-field snapshot; it never contains a native token.
type Adoption struct {
	Source          string                `json:"agent"`
	Profile         agent.ProviderProfile `json:"profile"`
	NeedsCredential bool                  `json:"needs_credential"`
	Notes           []string              `json:"notes"`
}

func (s *Service) AdoptDraft(id string) (Adoption, error) {
	ad := agent.ByID(s.adapters, id)
	host, ok := ad.(agent.ProviderHost)
	if !ok {
		return Adoption{}, fmt.Errorf("agent %q does not support provider adoption", id)
	}
	p, active, err := host.ReadProvider()
	if err != nil {
		return Adoption{}, err
	}
	p = p.Redacted()
	if !active || p.BaseURL == "" {
		return Adoption{}, fmt.Errorf("%s has no safely readable explicit endpoint to adopt; native account sign-in stays in the agent", id)
	}
	if validator, ok := host.(agent.ProviderValidator); ok {
		if _, err := validator.ValidateProvider(p); err != nil {
			return Adoption{}, err
		}
	} else if err := agent.ValidateProviderProfile(p); err != nil {
		return Adoption{}, err
	}
	needed := p.HasToken && p.EnvKey == ""
	p.Token, p.HasToken = "", false
	p.Name = id + "-imported"
	return Adoption{Source: id, Profile: p, NeedsCredential: needed, Notes: []string{
		"Only endpoint, model, protocol and credential references are imported. Native credentials, advanced options and login stay in the agent.",
		"A native token was not copied. Re-enter it explicitly or supply an agent-supported environment reference when needed.",
		"Saving changes only the profile library; applying is a separate reviewed action.",
	}}, nil
}

func (s *Service) SaveAdoption(a Adoption, p agent.ProviderProfile) error {
	if a.NeedsCredential && p.Token == "" && (p.EnvKey == "" || a.Source == "claude-code") {
		return fmt.Errorf("re-enter the credential explicitly; no token was imported from the native agent")
	}
	p.HasToken = false
	return s.saveNew(p)
}

func (s *Service) saveNew(p agent.ProviderProfile) error {
	profiles, err := s.Profiles()
	if err != nil {
		return err
	}
	for _, old := range profiles {
		if old.Name == strings.TrimSpace(p.Name) {
			return fmt.Errorf("profile %q already exists; choose another name or edit it explicitly", old.Name)
		}
	}
	return s.Save(p)
}
