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
	"regexp"
	"sort"
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// nameRe limita o nome do hook ao que também é um nome de arquivo seguro.
const maxNameLen = 40

var nameRe = regexp.MustCompile(`^[\p{L}\p{N}_-]+$`)

// Hook é uma entrada da biblioteca: o hook em si mais os campos do
// lazyagents (nome, descrição, proveniência), que nunca vão para o arquivo do
// agente.
type Hook struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Source é de onde o hook foi importado ("usuario/repo · plugin"). Vazio
	// = criado à mão.
	Source string `json:"source,omitempty"`
	// Files é a pasta com os scripts que os comandos usam, copiada na
	// importação.
	Files string `json:"files,omitempty"`
	// Hooks são os comandos da entrada. Quase sempre um só, mas um plugin
	// importado é um pacote: o security-guidance do marketplace oficial tem
	// 12 comandos em 5 eventos, e quebrá-lo em 12 entradas tornaria a
	// biblioteca e a matriz inúteis. A entrada liga e desliga inteira.
	Hooks []agent.Hook `json:"hooks"`
}

// Imported diz se o hook veio de um plugin de outro agente. Vale um aviso:
// esses hooks são escritos para o protocolo do Claude Code, e o payload que
// cada CLI manda no stdin pode não ser o mesmo.
func (h Hook) Imported() bool { return h.Source != "" }

// Events devolve os eventos distintos da entrada, na ordem em que aparecem.
func (h Hook) Events() []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range h.Hooks {
		if !seen[e.Event] {
			seen[e.Event] = true
			out = append(out, e.Event)
		}
	}
	return out
}

// Summary resume a entrada para a lista: o comando quando é um só, a
// contagem quando é um pacote.
func (h Hook) Summary() string {
	if len(h.Hooks) == 1 {
		return h.Hooks[0].Command
	}
	return fmt.Sprintf("%d comandos em %d eventos", len(h.Hooks), len(h.Events()))
}

// Service é a biblioteca de hooks mais a instalação nos agentes.
type Service struct {
	adapters   []agent.Adapter
	dir        string
	backupsDir string
	home       string // só para encurtar caminhos na tela
	// Detect devolve a detecção dos agentes; quem monta o service passa a
	// versão memoizada (feature.Deps.Agents). nil cai na detecção direta.
	Detect func() []agent.Agent
}

func New(adapters []agent.Adapter, paths core.Paths) *Service {
	return &Service{adapters: adapters, dir: paths.HooksDir(), backupsDir: paths.BackupsDir(), home: paths.Home}
}

// Dir é a biblioteca (exibição no doctor e na aba).
func (s *Service) Dir() string { return s.dir }

// Library lê a biblioteca, em ordem alfabética. Arquivo inválido não derruba
// a listagem: vira erro só dele.
func (s *Service) Library() ([]Hook, []string) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []string{fmt.Sprintf("lendo biblioteca: %v", err)}
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
		if h.Name+".json" != e.Name() || !nameRe.MatchString(h.Name) {
			problems = append(problems, fmt.Sprintf("%s: nome inválido ou diferente do arquivo", e.Name()))
			continue
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
	case !nameRe.MatchString(h.Name):
		return fmt.Errorf("nome do hook: use só letras, números, - e _")
	case len([]rune(h.Name)) > maxNameLen:
		return fmt.Errorf("nome do hook: máximo de %d caracteres", maxNameLen)
	case len(h.Hooks) == 0:
		return fmt.Errorf("o hook %q precisa de pelo menos um comando", h.Name)
	}
	for _, e := range h.Hooks {
		switch {
		case strings.TrimSpace(e.Command) == "":
			return fmt.Errorf("o hook %q precisa de um comando", h.Name)
		case strings.TrimSpace(e.Event) == "":
			return fmt.Errorf("o hook %q precisa de um evento", h.Name)
		}
	}
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return fmt.Errorf("gravando hook %q: %w", h.Name, err)
	}
	return fsutil.WriteAtomic(filepath.Join(s.dir, h.Name+".json"), append(data, '\n'), 0o600)
}

// Delete tira o hook da biblioteca. Não desinstala dos agentes — para isso
// existe Disable. Os scripts importados só são apagados quando nenhuma outra
// entrada aponta para eles.
func (s *Service) Delete(name string) error {
	h, err := s.Get(name)
	if err != nil {
		return err
	}
	if h.Files != "" {
		root, err := filepath.Abs(s.dir)
		if err != nil {
			return err
		}
		files, err := filepath.Abs(h.Files)
		if err != nil {
			return err
		}
		if filepath.Dir(files) != root {
			return fmt.Errorf("scripts de %q fora da biblioteca de hooks: %s", name, h.Files)
		}
	}
	if err := os.Remove(filepath.Join(s.dir, name+".json")); err != nil {
		return err
	}
	if h.Files == "" {
		return nil
	}
	lib, _ := s.Library()
	for _, other := range lib {
		if other.Files == h.Files {
			return nil // outra entrada ainda usa os mesmos scripts
		}
	}
	return os.RemoveAll(h.Files)
}

// Status é a situação dos hooks num agente.
type Status struct {
	AgentID   string   `json:"agent"`
	AgentName string   `json:"name"`
	File      string   `json:"file"`
	Installed bool     `json:"installed"`
	Events    []string `json:"events"`
	// Enabled são os nomes das entradas da biblioteca com TODOS os comandos
	// suportados já instalados no agente.
	Enabled []string `json:"enabled,omitempty"`
	// Partial são as entradas instaladas pela metade (pacote importado cujo
	// enable falhou no meio, ou comando removido à mão no arquivo).
	Partial []string `json:"partial,omitempty"`
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
		for _, ins := range installed {
			if !claimedBy(lib, ins) {
				st.Foreign++ // hook do próprio usuário: nunca tocamos nele
			}
		}
		for _, entry := range lib {
			want := supportedHooks(host, entry)
			if len(want) == 0 {
				continue // o agente não dispara nenhum evento da entrada
			}
			have := 0
			for _, w := range want {
				if containsHook(installed, w) {
					have++
				}
			}
			switch {
			case have == len(want):
				st.Enabled = append(st.Enabled, entry.Name)
			case have > 0:
				st.Partial = append(st.Partial, entry.Name)
			}
		}
		out = append(out, st)
	}
	return out
}

// claimedBy diz se um hook instalado pertence a alguma entrada da biblioteca.
func claimedBy(lib []Hook, h agent.Hook) bool {
	for _, entry := range lib {
		if containsHook(entry.Hooks, h) {
			return true
		}
	}
	return false
}

// containsHook diz se a lista tem um hook com a mesma identidade.
func containsHook(list []agent.Hook, h agent.Hook) bool {
	for _, item := range list {
		if item.Same(h) {
			return true
		}
	}
	return false
}

// supportedHooks filtra os comandos da entrada que o agente pode rodar. Um
// pacote importado costuma ter evento que só um dos CLIs dispara; instalar o
// que dá e dizer o que ficou de fora é melhor que recusar o pacote inteiro.
func supportedHooks(host agent.HooksHost, entry Hook) []agent.Hook {
	var out []agent.Hook
	for _, h := range entry.Hooks {
		if supportsEvent(host, h.Event) {
			out = append(out, h)
		}
	}
	return out
}

// Enable instala a entrada no agente (só os comandos cujos eventos ele
// dispara). agentID vazio instala em todos os instalados que suportam.
func (s *Service) Enable(name, agentID string) error {
	h, err := s.Get(name)
	if err != nil {
		return err
	}
	return s.each(agentID, h, func(host agent.HooksHost) error {
		want := supportedHooks(host, h)
		if len(want) == 0 {
			return fmt.Errorf("não dispara nenhum evento de %q (%s)", h.Name, strings.Join(h.Events(), ", "))
		}
		for _, one := range want {
			if err := host.AddHook(one, s.backupsDir); err != nil {
				return err
			}
		}
		return nil
	})
}

// Disable desinstala todos os comandos da entrada.
func (s *Service) Disable(name, agentID string) error {
	h, err := s.Get(name)
	if err != nil {
		return err
	}
	return s.each(agentID, Hook{}, func(host agent.HooksHost) error {
		for _, one := range h.Hooks {
			if err := host.RemoveHook(one, s.backupsDir); err != nil {
				return err
			}
		}
		return nil
	})
}

// each roda fn no agente pedido, ou em todos os instalados que suportam
// hooks. entry com comandos exige que o agente dispare ao menos um dos
// eventos dela — instalar um hook que o CLI nunca dispara seria
// silenciosamente inútil.
func (s *Service) each(agentID string, entry Hook, fn func(host agent.HooksHost) error) error {
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
		if len(entry.Hooks) > 0 && len(supportedHooks(host, entry)) == 0 {
			if agentID != "" {
				return fmt.Errorf("%s não dispara nenhum evento de %q (%s)", ad.ID(), entry.Name, strings.Join(entry.Events(), ", "))
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

// CommandProblem devolve um aviso quando o executável de algum comando da
// entrada não está no PATH (o hook falharia em silêncio na hora do evento).
func CommandProblem(h Hook) string {
	for _, one := range h.Hooks {
		if p := commandProblem(one.Command); p != "" {
			return p
		}
	}
	return ""
}

func commandProblem(command string) string {
	fields := strings.Fields(stripRootExport(command)) // o executável vem depois do export
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
