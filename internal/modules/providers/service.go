// Package providers troca o endpoint/modelo que cada agente usa, aplicando
// perfis nomeados na config viva do CLI (estilo cc-switch).
//
// O módulo inteiro vive aqui: service (este arquivo), aba da TUI (tab.go,
// view.go, help.go), comandos da CLI (cli.go) e o registro (feature.go). Os
// perfis são do lazyagents e vivem em <ConfigDir>/providers.json (0600, pode
// conter token); quem sabe escrever no arquivo de cada agente é o adapter,
// via agent.ProviderHost.
package providers

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// maxNameLen limita o nome do perfil ao que cabe na matriz da TUI.
const maxNameLen = 40

// Service guarda a biblioteca de perfis e aplica um deles num agente.
type Service struct {
	adapters   []agent.Adapter
	path       string
	backupsDir string
	home       string // só para encurtar caminhos na tela
	// Detect devolve a detecção dos agentes. Quem monta o service passa a
	// versão memoizada (app.Deps.Agents) para não rodar `--version` de todos
	// os CLIs de novo; nil cai na detecção direta.
	Detect func() []agent.Agent
}

func New(adapters []agent.Adapter, paths core.Paths) *Service {
	return &Service{adapters: adapters, path: paths.ProvidersPath(), backupsDir: paths.BackupsDir(), home: paths.Home}
}

// detectAll roda a detecção uma vez por operação (nunca por adapter: cada
// Detect paga um `--version`).
func (s *Service) detectAll() []agent.Agent {
	if s.Detect != nil {
		return s.Detect()
	}
	return agent.DetectAll(s.adapters)
}

func findAgent(agents []agent.Agent, id string) agent.Agent {
	for _, a := range agents {
		if a.ID == id {
			return a
		}
	}
	return agent.Agent{ID: id}
}

// Path é o arquivo de perfis (exibição no doctor).
func (s *Service) Path() string { return s.path }

// library é o formato em disco. Objeto (e não lista) para caber campo novo
// depois sem quebrar quem já tem o arquivo.
type library struct {
	Profiles []agent.ProviderProfile `json:"profiles"`
}

// Profiles devolve os perfis salvos, em ordem alfabética. Token preenchido —
// quem exibe chama Redacted.
func (s *Service) Profiles() ([]agent.ProviderProfile, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("lendo %s: %w", s.path, err)
	}
	var lib library
	if err := json.Unmarshal(data, &lib); err != nil {
		return nil, fmt.Errorf("lendo %s: %w", s.path, err)
	}
	sort.Slice(lib.Profiles, func(i, j int) bool { return lib.Profiles[i].Name < lib.Profiles[j].Name })
	return lib.Profiles, nil
}

// Profile encontra um perfil pelo nome.
func (s *Service) Profile(name string) (agent.ProviderProfile, error) {
	profiles, err := s.Profiles()
	if err != nil {
		return agent.ProviderProfile{}, err
	}
	for _, p := range profiles {
		if p.Name == name {
			return p, nil
		}
	}
	return agent.ProviderProfile{}, fmt.Errorf("perfil %q não existe", name)
}

// Save cria ou substitui um perfil pelo nome.
func (s *Service) Save(p agent.ProviderProfile) error {
	p.Name = strings.TrimSpace(p.Name)
	switch {
	case p.Name == "":
		return fmt.Errorf("o perfil precisa de um nome")
	case len([]rune(p.Name)) > maxNameLen:
		return fmt.Errorf("nome do perfil: máximo de %d caracteres", maxNameLen)
	case p.BaseURL == "" && p.Model == "" && p.Token == "":
		return fmt.Errorf("perfil %q não muda nada: defina baseUrl, model ou token", p.Name)
	case p.BaseURL != "" && !validURL(p.BaseURL):
		return fmt.Errorf("endpoint %q: use uma URL http:// ou https://", p.BaseURL)
	}
	p.HasToken = false // derivado; nunca persistido

	profiles, err := s.Profiles()
	if err != nil {
		return err
	}
	replaced := false
	for i := range profiles {
		if profiles[i].Name == p.Name {
			profiles[i], replaced = p, true
			break
		}
	}
	if !replaced {
		profiles = append(profiles, p)
	}
	return s.write(profiles)
}

// Edit regrava o perfil orig com os campos de p. Token vazio mantém o salvo
// — quem edita (a aba) nunca vê o token, então não tem como reenviá-lo.
// Nome diferente renomeia: o perfil novo entra e o antigo sai.
func (s *Service) Edit(orig string, p agent.ProviderProfile) error {
	old, err := s.Profile(orig)
	if err != nil {
		return err
	}
	if p.Token == "" {
		p.Token = old.Token
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name != orig {
		if _, err := s.Profile(p.Name); err == nil {
			return fmt.Errorf("já existe um perfil %q", p.Name)
		}
	}
	if err := s.Save(p); err != nil {
		return err
	}
	if p.Name != orig {
		return s.Delete(orig)
	}
	return nil
}

// Delete remove um perfil da biblioteca. Não mexe em agente onde ele já foi
// aplicado — para isso existe Clear.
func (s *Service) Delete(name string) error {
	profiles, err := s.Profiles()
	if err != nil {
		return err
	}
	kept := profiles[:0]
	for _, p := range profiles {
		if p.Name != name {
			kept = append(kept, p)
		}
	}
	if len(kept) == len(profiles) {
		return fmt.Errorf("perfil %q não existe", name)
	}
	return s.write(kept)
}

func (s *Service) write(profiles []agent.ProviderProfile) error {
	data, err := json.MarshalIndent(library{Profiles: profiles}, "", "  ")
	if err != nil {
		return fmt.Errorf("gravando perfis: %w", err)
	}
	// 0600: o arquivo pode conter token (regra 7).
	return fsutil.WriteAtomic(s.path, append(data, '\n'), 0o600)
}

// Status é o que está aplicado num agente agora.
type Status struct {
	AgentID   string `json:"agent"`
	AgentName string `json:"name"`
	File      string `json:"file"`
	Installed bool   `json:"installed"`
	// Applied é o provedor lido da config viva, sempre sem o token.
	Applied agent.ProviderProfile `json:"applied,omitempty"`
	Active  bool                  `json:"active"`
	// Profile é o nome do perfil da biblioteca que casa com o aplicado
	// (vazio quando foi configurado fora do lazyagents).
	Profile string `json:"profile,omitempty"`
	Err     string `json:"error,omitempty"`
}

// Status devolve, na ordem de registro, um Status por agente que suporta
// troca de provedor. Agente sem a capacidade fica de fora.
func (s *Service) Status() []Status {
	profiles, _ := s.Profiles()
	agents := s.detectAll()
	var out []Status
	for _, ad := range s.adapters {
		host, ok := ad.(agent.ProviderHost)
		if !ok {
			continue
		}
		a := findAgent(agents, ad.ID())
		st := Status{AgentID: ad.ID(), AgentName: a.Name, File: host.ProviderFile(), Installed: a.Installed}
		applied, active, err := host.ReadProvider()
		switch {
		case err != nil:
			st.Err = err.Error()
		case active:
			st.Applied, st.Active = applied.Redacted(), true
			st.Profile = matchProfile(applied, profiles)
		}
		out = append(out, st)
	}
	return out
}

// matchProfile identifica o perfil aplicado pelo endpoint (o que todo agente
// grava); modelo desempata quando dois perfis compartilham o endpoint.
func matchProfile(applied agent.ProviderProfile, profiles []agent.ProviderProfile) string {
	best := ""
	for _, p := range profiles {
		if p.BaseURL == "" || p.BaseURL != applied.BaseURL {
			continue
		}
		if p.Model == applied.Model {
			return p.Name
		}
		if best == "" {
			best = p.Name
		}
	}
	return best
}

// Apply grava o perfil no agente. agentID vazio aplica em todos os que
// suportam e estão instalados.
func (s *Service) Apply(name, agentID string) error {
	p, err := s.Profile(name)
	if err != nil {
		return err
	}
	return s.each(agentID, func(id string, host agent.ProviderHost) error {
		return host.ApplyProvider(p, s.backupsDir)
	})
}

// Clear desfaz o que o lazyagents aplicou, preservando o resto do arquivo.
func (s *Service) Clear(agentID string) error {
	return s.each(agentID, func(id string, host agent.ProviderHost) error {
		return host.ClearProvider(s.backupsDir)
	})
}

// each roda fn no agente pedido, ou em todos os instalados que suportam
// provedores. Falha de um agente não impede os outros: os erros são
// acumulados.
func (s *Service) each(agentID string, fn func(id string, host agent.ProviderHost) error) error {
	var errs []string
	var agents []agent.Agent
	if agentID == "" {
		agents = s.detectAll() // só o "aplicar em todos" precisa saber quem está instalado
	}
	found := false
	for _, ad := range s.adapters {
		if agentID != "" && ad.ID() != agentID {
			continue
		}
		host, ok := ad.(agent.ProviderHost)
		if !ok {
			if agentID != "" {
				return fmt.Errorf("%s não suporta troca de provedor", ad.ID())
			}
			continue
		}
		if agentID == "" && !findAgent(agents, ad.ID()).Installed {
			continue
		}
		found = true
		if err := fn(ad.ID(), host); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", ad.ID(), err))
		}
	}
	switch {
	case !found && agentID != "":
		return fmt.Errorf("agente %q não existe", agentID)
	case !found:
		return fmt.Errorf("nenhum agente instalado suporta troca de provedor")
	case len(errs) > 0:
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func validURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
