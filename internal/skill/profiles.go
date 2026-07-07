package skill

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"lazyskills/internal/agent"
	"lazyskills/internal/fsutil"
)

// readProfilesRaw lê o arquivo inteiro em map[string]json.RawMessage para
// preservar campos desconhecidos no round-trip. Arquivo ausente = ok.
func (s *Service) readProfilesRaw() (map[string]json.RawMessage, map[string][]string, error) {
	raw := make(map[string]json.RawMessage)
	data, err := os.ReadFile(s.paths.ProfilesPath())
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("lendo perfis: %w", err)
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, nil, fmt.Errorf("parseando perfis: %w", err)
		}
	}
	profiles := make(map[string][]string)
	if pRaw, ok := raw["profiles"]; ok {
		if err := json.Unmarshal(pRaw, &profiles); err != nil {
			return nil, nil, fmt.Errorf("parseando lista de perfis: %w", err)
		}
	}
	return raw, profiles, nil
}

func (s *Service) writeProfiles(raw map[string]json.RawMessage, profiles map[string][]string) error {
	pData, err := json.Marshal(profiles)
	if err != nil {
		return err
	}
	raw["profiles"] = pData
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteAtomic(s.paths.ProfilesPath(), data, 0o644)
}

// ListProfiles devolve os nomes dos perfis em ordem alfabética.
func (s *Service) ListProfiles() ([]string, error) {
	_, profiles, err := s.readProfilesRaw()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// GetProfile devolve a lista de skills de um perfil.
func (s *Service) GetProfile(name string) ([]string, error) {
	_, profiles, err := s.readProfilesRaw()
	if err != nil {
		return nil, err
	}
	skills, ok := profiles[name]
	if !ok {
		return nil, fmt.Errorf("perfil %q não encontrado", name)
	}
	return skills, nil
}

// SaveProfile salva (ou sobrescreve) o perfil com a lista de skills fornecida.
// A lista é deduplicada e ordenada para estabilidade.
func (s *Service) SaveProfile(name string, skills []string) error {
	if name == "" {
		return fmt.Errorf("nome do perfil não pode ser vazio")
	}
	raw, profiles, err := s.readProfilesRaw()
	if err != nil {
		return err
	}
	seen := make(map[string]bool, len(skills))
	clean := make([]string, 0, len(skills))
	for _, sk := range skills {
		if sk != "" && !seen[sk] {
			seen[sk] = true
			clean = append(clean, sk)
		}
	}
	sort.Strings(clean)
	profiles[name] = clean
	return s.writeProfiles(raw, profiles)
}

// DeleteProfile remove o perfil. Não é erro deletar um perfil inexistente.
func (s *Service) DeleteProfile(name string) error {
	raw, profiles, err := s.readProfilesRaw()
	if err != nil {
		return err
	}
	delete(profiles, name)
	return s.writeProfiles(raw, profiles)
}

// ApplyProfile garante as skills do perfil ativas em todos os agentes instalados
// e desativa as skills gerenciadas que ficaram de fora. Skills locais nunca são
// tocadas (mesma regra do DisableAll). Idempotente.
// Retorna erro se alguma skill do perfil não existe na biblioteca.
func (s *Service) ApplyProfile(name string, agents []agent.Agent) error {
	_, profiles, err := s.readProfilesRaw()
	if err != nil {
		return err
	}
	wanted, ok := profiles[name]
	if !ok {
		return fmt.Errorf("perfil %q não encontrado", name)
	}

	skills, err := s.Scan(agents)
	if err != nil {
		return fmt.Errorf("aplicando perfil %q: %w", name, err)
	}

	byDir := make(map[string]Skill, len(skills))
	for _, sk := range skills {
		byDir[sk.Dir] = sk
	}

	var missing []string
	for _, w := range wanted {
		if sk, found := byDir[w]; !found || !sk.InLibrary {
			missing = append(missing, w)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("perfil %q referencia skills inexistentes: %s",
			name, strings.Join(missing, ", "))
	}

	wantedSet := make(map[string]bool, len(wanted))
	for _, w := range wanted {
		wantedSet[w] = true
	}

	var errs []error
	for _, sk := range skills {
		if !sk.InLibrary {
			continue
		}
		if wantedSet[sk.Dir] {
			if err := s.EnableAll(sk, agents); err != nil {
				errs = append(errs, err)
			}
		} else {
			if err := s.DisableAll(sk, agents); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}
