package skills

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// allAgents é a sentinela usada na lista de agentes de uma skill para dizer
// "todos os agentes instalados com suporte a skills". Só é produzida pela
// migração do formato antigo (lista plana); SaveProfile grava sempre IDs
// concretos, então na primeira regravação o perfil migrado vira concreto.
const allAgents = "*"

// ProfileSpec mapeia cada skill (nome da pasta) para os agentes em que ela deve
// ficar ativa. É o corpo de um perfil no formato novo.
type ProfileSpec map[string][]string

// ProfileChange descreve, para uma skill, os agentes que aplicar o perfil vai
// ativar (Add) e desativar (Remove). Usado para mostrar o diff na TUI.
type ProfileChange struct {
	Skill  string
	Add    []string
	Remove []string
}

// readProfilesRaw lê o arquivo inteiro em map[string]json.RawMessage para
// preservar campos desconhecidos no round-trip. Arquivo ausente = ok.
// Cada perfil é decodificado de forma resiliente: formato novo
// ({skill: [agentIDs]}) ou legado ([skills], convertido para {skill: ["*"]}).
func (s *Service) readProfilesRaw() (map[string]json.RawMessage, map[string]ProfileSpec, error) {
	raw := make(map[string]json.RawMessage)
	data, err := os.ReadFile(s.paths.ProfilesPath())
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("reading profiles: %w", err)
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, nil, fmt.Errorf("parsing profiles: %w", err)
		}
	}
	if raw == nil {
		raw = make(map[string]json.RawMessage)
	}
	profiles := make(map[string]ProfileSpec)
	if pRaw, ok := raw["profiles"]; ok {
		perProfile := make(map[string]json.RawMessage)
		if err := json.Unmarshal(pRaw, &perProfile); err != nil {
			return nil, nil, fmt.Errorf("parsing profile list: %w", err)
		}
		for name, body := range perProfile {
			spec, err := decodeProfile(body)
			if err != nil {
				return nil, nil, fmt.Errorf("parsing profile %q: %w", name, err)
			}
			profiles[name] = spec
		}
	}
	return raw, profiles, nil
}

// decodeProfile aceita o formato novo (objeto) ou o legado (array de skills).
func decodeProfile(body json.RawMessage) (ProfileSpec, error) {
	var spec ProfileSpec
	if err := json.Unmarshal(body, &spec); err == nil {
		if spec == nil {
			spec = ProfileSpec{}
		}
		return spec, nil
	}
	var legacy []string
	if err := json.Unmarshal(body, &legacy); err != nil {
		return nil, err
	}
	spec = make(ProfileSpec, len(legacy))
	for _, sk := range legacy {
		if sk != "" {
			spec[sk] = []string{allAgents}
		}
	}
	return spec, nil
}

func (s *Service) writeProfiles(raw map[string]json.RawMessage, profiles map[string]ProfileSpec) error {
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

// GetProfile devolve a spec (skill → agentes) de um perfil.
func (s *Service) GetProfile(name string) (ProfileSpec, error) {
	_, profiles, err := s.readProfilesRaw()
	if err != nil {
		return nil, err
	}
	spec, ok := profiles[name]
	if !ok {
		return nil, fmt.Errorf("profile %q not found", name)
	}
	return spec, nil
}

// BuildProfileSpec fotografa a matriz atual: para cada skill da biblioteca,
// registra os agentes em que ela está ativa e sob controle do lazyagents
// (estado Managed = nosso symlink no dir gerenciado do próprio agente).
// Ignora ativações "local" (dir real ou symlink alheio) e "de eco" (skill que
// aparece num agente só porque ele lê um dir compartilhado de outro, ex.:
// opencode lendo ~/.claude/skills) — o Apply não consegue controlá-las por
// agente. Skills sem nenhum agente são omitidas; listas ordenadas e sem dupes.
func BuildProfileSpec(skills []Skill, agents []agent.Agent) ProfileSpec {
	spec := make(ProfileSpec)
	for _, sk := range skills {
		if !sk.InLibrary {
			continue
		}
		var on []string
		for _, ag := range agents {
			if !ag.Installed || !ag.SupportsSkills() {
				continue
			}
			if sk.States[ag.ID].Managed {
				on = append(on, ag.ID)
			}
		}
		if len(on) > 0 {
			sort.Strings(on)
			spec[sk.Dir] = on
		}
	}
	return spec
}

// SaveProfile salva (ou sobrescreve) o perfil com a spec fornecida. As listas de
// agentes são deduplicadas e ordenadas; entradas com lista vazia são descartadas.
func (s *Service) SaveProfile(name string, spec ProfileSpec) error {
	if name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}
	raw, profiles, err := s.readProfilesRaw()
	if err != nil {
		return err
	}
	clean := make(ProfileSpec, len(spec))
	for skill, agents := range spec {
		if skill == "" {
			continue
		}
		seen := make(map[string]bool, len(agents))
		list := make([]string, 0, len(agents))
		for _, a := range agents {
			if a != "" && !seen[a] {
				seen[a] = true
				list = append(list, a)
			}
		}
		if len(list) == 0 {
			continue
		}
		sort.Strings(list)
		clean[skill] = list
	}
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

// resolveTargets converte a spec do perfil no conjunto desejado de agentes por
// skill (expandindo a sentinela "*") e valida que toda skill referenciada existe
// na biblioteca. skillAgents mapeia agentes instalados com suporte a skills.
func resolveTargets(spec ProfileSpec, byDir map[string]Skill, skillAgents []agent.Agent) (map[string]map[string]bool, error) {
	var missing []string
	for w := range spec {
		if sk, found := byDir[w]; !found || !sk.InLibrary {
			missing = append(missing, w)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("profile refers to missing skills: %s", strings.Join(missing, ", "))
	}
	targets := make(map[string]map[string]bool, len(spec))
	for skill, agents := range spec {
		set := make(map[string]bool, len(agents))
		for _, a := range agents {
			if a == allAgents {
				for _, ag := range skillAgents {
					set[ag.ID] = true
				}
				continue
			}
			set[a] = true
		}
		targets[skill] = set
	}
	return targets, nil
}

// skillCapableAgents devolve apenas os agentes instalados com suporte a skills.
func skillCapableAgents(agents []agent.Agent) []agent.Agent {
	out := make([]agent.Agent, 0, len(agents))
	for _, ag := range agents {
		if ag.Installed && ag.SupportsSkills() {
			out = append(out, ag)
		}
	}
	return out
}

// ApplyProfile restaura a matriz do perfil: cada skill da biblioteca fica ativa
// exatamente nos agentes que o perfil pede e desativada nos demais. Skills locais
// nunca são tocadas (Disable as ignora). Idempotente. Retorna erro se o perfil
// referencia skills inexistentes.
func (s *Service) ApplyProfile(name string, agents []agent.Agent) error {
	_, profiles, err := s.readProfilesRaw()
	if err != nil {
		return err
	}
	spec, ok := profiles[name]
	if !ok {
		return fmt.Errorf("profile %q not found", name)
	}

	skills, err := s.Scan(agents)
	if err != nil {
		return fmt.Errorf("applying profile %q: %w", name, err)
	}
	byDir := make(map[string]Skill, len(skills))
	for _, sk := range skills {
		byDir[sk.Dir] = sk
	}

	capable := skillCapableAgents(agents)
	targets, err := resolveTargets(spec, byDir, capable)
	if err != nil {
		return fmt.Errorf("applying profile %q: %w", name, err)
	}

	var errs []error
	for _, sk := range skills {
		if !sk.InLibrary {
			continue
		}
		want := targets[sk.Dir] // nil se a skill não está no perfil → desativa em todos
		for _, ag := range capable {
			st := sk.States[ag.ID]
			switch {
			case want[ag.ID]:
				// Enable é no-op se a skill já está visível (inclusive por eco de
				// dir compartilhado); só cria symlink quando o agente não a vê.
				if err := s.Enable(sk, ag); err != nil {
					errs = append(errs, err)
				}
			case st.Managed:
				// só removemos o que controlamos: nosso symlink no dir do agente.
				// Ativações locais ou de eco (Via de outro agente) ficam intactas.
				if err := s.Disable(sk, ag); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}
	return errors.Join(errs...)
}

// DiffProfile compara o alvo do perfil com a matriz atual e devolve, por skill,
// os agentes a ativar (Add) e a desativar (Remove). Skills locais são ignoradas
// no Remove (nunca são desativadas). A lista sai ordenada por nome de skill.
func (s *Service) DiffProfile(name string, agents []agent.Agent) ([]ProfileChange, error) {
	_, profiles, err := s.readProfilesRaw()
	if err != nil {
		return nil, err
	}
	spec, ok := profiles[name]
	if !ok {
		return nil, fmt.Errorf("profile %q not found", name)
	}

	skills, err := s.Scan(agents)
	if err != nil {
		return nil, fmt.Errorf("comparing profile %q: %w", name, err)
	}
	byDir := make(map[string]Skill, len(skills))
	for _, sk := range skills {
		byDir[sk.Dir] = sk
	}

	capable := skillCapableAgents(agents)
	targets, err := resolveTargets(spec, byDir, capable)
	if err != nil {
		return nil, fmt.Errorf("comparing profile %q: %w", name, err)
	}

	var changes []ProfileChange
	for _, sk := range skills {
		if !sk.InLibrary {
			continue
		}
		want := targets[sk.Dir]
		var add, remove []string
		for _, ag := range capable {
			st := sk.States[ag.ID]
			if want[ag.ID] {
				if !st.On {
					add = append(add, ag.ID)
				}
			} else if st.Managed {
				// só entra no diff o que o Apply consegue desativar de fato.
				remove = append(remove, ag.ID)
			}
		}
		if len(add) > 0 || len(remove) > 0 {
			sort.Strings(add)
			sort.Strings(remove)
			changes = append(changes, ProfileChange{Skill: sk.Name, Add: add, Remove: remove})
		}
	}
	sort.Slice(changes, func(i, j int) bool {
		return strings.ToLower(changes[i].Skill) < strings.ToLower(changes[j].Skill)
	})
	return changes, nil
}
