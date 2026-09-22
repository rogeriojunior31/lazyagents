// Package hooks mantém uma biblioteca de hooks do usuário e os instala nos
// agentes que suportam (agent.HooksHost).
//
// A regra é a mesma das skills: o lazyagents gerencia o que está na
// biblioteca e nunca toca no que é do agente. Um hook é identificado pela
// tripla (evento, matcher, comando) — é assim que o módulo sabe se o seu
// hook já está instalado sem precisar marcar o arquivo do CLI.
package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// nameRe limita o nome do hook ao que também é um nome de arquivo seguro.
const maxNameLen = 40

// Hook é uma entrada da biblioteca: o hook em si mais nome e descrição, que
// são do lazyagents e não vão para o arquivo do agente.
type Hook struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	agent.Hook
}

// Service é a biblioteca de hooks mais a instalação nos agentes.
type Service struct {
	adapters   []agent.Adapter
	dir        string
	backupsDir string
	// Detect devolve a detecção dos agentes; quem monta o service passa a
	// versão memoizada (feature.Deps.Agents). nil cai na detecção direta.
	Detect func() []agent.Agent
}

func New(adapters []agent.Adapter, paths core.Paths) *Service {
	return &Service{adapters: adapters, dir: paths.HooksDir(), backupsDir: paths.BackupsDir()}
}

// Dir é a biblioteca (exibição no doctor e na aba).
func (s *Service) Dir() string { return s.dir }

// Library lê a biblioteca, em ordem alfabética. Arquivo inválido não derruba
// a listagem: vira erro só dele.
func (s *Service) Library() ([]Hook, []string) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, nil // biblioteca ainda não existe
	}
	var out []Hook
	var problems []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(s.dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", e.Name(), err))
			continue
		}
		var h Hook
		if err := json.Unmarshal(data, &h); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", e.Name(), err))
			continue
		}
		if h.Name == "" {
			h.Name = strings.TrimSuffix(e.Name(), ".json")
		}
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, problems
}

// Get encontra um hook da biblioteca pelo nome.
func (s *Service) Get(name string) (Hook, error) {
	lib, _ := s.Library()
	for _, h := range lib {
		if h.Name == name {
			return h, nil
		}
	}
	return Hook{}, fmt.Errorf("hook %q não existe na biblioteca", name)
}

// Save cria ou substitui um hook da biblioteca.
func (s *Service) Save(h Hook) error {
	h.Name = strings.TrimSpace(h.Name)
	switch {
	case h.Name == "" || strings.ContainsAny(h.Name, `/\.`):
		return fmt.Errorf("nome do hook: use só letras, números, - e _")
	case len([]rune(h.Name)) > maxNameLen:
		return fmt.Errorf("nome do hook: máximo de %d caracteres", maxNameLen)
	case strings.TrimSpace(h.Command) == "":
		return fmt.Errorf("o hook %q precisa de um comando", h.Name)
	case strings.TrimSpace(h.Event) == "":
		return fmt.Errorf("o hook %q precisa de um evento", h.Name)
	}
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return fmt.Errorf("gravando hook %q: %w", h.Name, err)
	}
	return fsutil.WriteAtomic(filepath.Join(s.dir, h.Name+".json"), append(data, '\n'), 0o600)
}

// Delete tira o hook da biblioteca. Não desinstala dos agentes — para isso
// existe Disable.
func (s *Service) Delete(name string) error {
	if _, err := s.Get(name); err != nil {
		return err
	}
	return os.Remove(filepath.Join(s.dir, name+".json"))
}

// Status é a situação dos hooks num agente.
type Status struct {
	AgentID   string   `json:"agent"`
	AgentName string   `json:"name"`
	File      string   `json:"file"`
	Installed bool     `json:"installed"`
	Events    []string `json:"events"`
	// Enabled são os nomes dos hooks da biblioteca já instalados no agente.
	Enabled []string `json:"enabled,omitempty"`
	// Foreign conta os hooks que estão no agente e não vieram da biblioteca:
	// o lazyagents nunca mexe neles.
	Foreign int    `json:"foreign"`
	Note    string `json:"note,omitempty"`
	Err     string `json:"error,omitempty"`
}

// Status devolve, na ordem de registro, um Status por agente que suporta
// hooks.
func (s *Service) Status() []Status {
	lib, _ := s.Library()
	agents := s.detectAll()
	var out []Status
	for _, ad := range s.adapters {
		host, ok := ad.(agent.HooksHost)
		if !ok {
			continue
		}
		a := findAgent(agents, ad.ID())
		st := Status{
			AgentID: ad.ID(), AgentName: a.Name, File: host.HooksFile(),
			Installed: a.Installed, Events: host.HookEvents(), Note: host.HooksNote(),
		}
		installed, err := host.ReadHooks()
		if err != nil {
			st.Err = err.Error()
			out = append(out, st)
			continue
		}
		for _, h := range installed {
			if name, ok := matchLibrary(h, lib); ok {
				st.Enabled = append(st.Enabled, name)
			} else {
				st.Foreign++
			}
		}
		out = append(out, st)
	}
	return out
}

// matchLibrary diz se um hook instalado veio da biblioteca.
func matchLibrary(h agent.Hook, lib []Hook) (string, bool) {
	for _, l := range lib {
		if l.Hook.Same(h) {
			return l.Name, true
		}
	}
	return "", false
}

// Enable instala o hook no agente. agentID vazio instala em todos os que
// suportam, estão instalados e disparam o evento do hook.
func (s *Service) Enable(name, agentID string) error {
	h, err := s.Get(name)
	if err != nil {
		return err
	}
	return s.each(agentID, h.Event, func(host agent.HooksHost) error {
		return host.AddHook(h.Hook, s.backupsDir)
	})
}

// Disable desinstala o hook do agente.
func (s *Service) Disable(name, agentID string) error {
	h, err := s.Get(name)
	if err != nil {
		return err
	}
	return s.each(agentID, "", func(host agent.HooksHost) error {
		return host.RemoveHook(h.Hook, s.backupsDir)
	})
}

// each roda fn no agente pedido, ou em todos os instalados que suportam
// hooks. event não vazio exige que o agente dispare aquele evento — instalar
// um hook que o CLI nunca dispara seria silenciosamente inútil.
func (s *Service) each(agentID, event string, fn func(host agent.HooksHost) error) error {
	agents := s.detectAll()
	var errs []string
	found := false
	for _, ad := range s.adapters {
		if agentID != "" && ad.ID() != agentID {
			continue
		}
		host, ok := ad.(agent.HooksHost)
		if !ok {
			if agentID != "" {
				return fmt.Errorf("%s não suporta hooks", ad.ID())
			}
			continue
		}
		if agentID == "" && !findAgent(agents, ad.ID()).Installed {
			continue
		}
		if event != "" && !supportsEvent(host, event) {
			if agentID != "" {
				return fmt.Errorf("%s não dispara o evento %s", ad.ID(), event)
			}
			continue
		}
		found = true
		if err := fn(host); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", ad.ID(), err))
		}
	}
	switch {
	case !found && agentID != "":
		return fmt.Errorf("agente %q não existe", agentID)
	case !found:
		return fmt.Errorf("nenhum agente instalado suporta esse hook")
	case len(errs) > 0:
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func supportsEvent(host agent.HooksHost, event string) bool {
	for _, e := range host.HookEvents() {
		if strings.EqualFold(e, event) {
			return true
		}
	}
	return false
}

// CommandProblem devolve um aviso quando o executável do comando do hook não
// está no PATH (o hook falharia em silêncio na hora do evento).
func CommandProblem(h Hook) string {
	fields := strings.Fields(h.Command)
	if len(fields) == 0 {
		return "comando vazio"
	}
	bin := fields[0]
	if strings.ContainsAny(bin, "/\\") {
		if info, err := os.Stat(bin); err != nil {
			return bin + ": não encontrado"
		} else if info.Mode()&0o111 == 0 {
			return bin + ": sem permissão de execução"
		}
		return ""
	}
	if _, err := exec.LookPath(bin); err != nil {
		return bin + ": não está no PATH"
	}
	return ""
}

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
