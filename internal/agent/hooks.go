package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Eventos de hook. Os dois CLIs que suportam hooks hoje (Claude Code e
// Codex) usam o mesmo vocabulário em CamelCase no arquivo de configuração;
// cada adapter declara em HookEvents quais realmente dispara.
const (
	HookSessionStart     = "SessionStart"
	HookSessionEnd       = "SessionEnd"
	HookUserPromptSubmit = "UserPromptSubmit"
	HookPreToolUse       = "PreToolUse"
	HookPostToolUse      = "PostToolUse"
	HookPreCompact       = "PreCompact"
	HookNotification     = "Notification"
	HookStop             = "Stop"
	HookSubagentStop     = "SubagentStop"
	hookCommandType      = "command"
	hookGroupsKey        = "hooks"
)

// Hook é um comando disparado por um evento do agente. A identidade de um
// hook é a tripla (Event, Matcher, Command): é por ela que o lazyagents sabe
// se um hook seu já está instalado, sem nunca tocar nos hooks alheios.
type Hook struct {
	Event   string `json:"event"`
	Matcher string `json:"matcher,omitempty"` // filtro do evento ("" = todos)
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"` // segundos; 0 = default do agente
}

// Same diz se dois hooks são o mesmo para efeito de instalação.
func (h Hook) Same(o Hook) bool {
	return sameEvent(h.Event, o.Event) && h.Matcher == o.Matcher && h.Command == o.Command
}

// sameEvent compara nomes de evento ignorando caixa e separador: o Codex
// grava o nome em CamelCase no arquivo mas usa snake_case internamente, e um
// arquivo escrito à mão pode ter qualquer uma das duas formas.
func sameEvent(a, b string) bool { return normEvent(a) == normEvent(b) }

func normEvent(s string) string {
	return strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(s))
}

// HooksHost é implementado pelos adapters que sabem ler e escrever hooks.
// Opcional, fora da interface Adapter: quem consome faz type assertion
// (padrão de UsageReader).
type HooksHost interface {
	// HookEvents lista os eventos que este agente dispara.
	HookEvents() []string
	// HooksFile é o arquivo que AddHook/RemoveHook escrevem.
	HooksFile() string
	// ReadHooks devolve TODOS os hooks configurados, inclusive os que não
	// vieram do lazyagents.
	ReadHooks() ([]Hook, error)
	// AddHook instala o hook (idempotente), com backup do arquivo vivo.
	AddHook(h Hook, backupsDir string) error
	// RemoveHook desinstala o hook pela identidade; hook ausente é no-op.
	RemoveHook(h Hook, backupsDir string) error
	// HooksNote é um aviso curto sobre o estado do agente ("" = nada a
	// dizer), como hooks desligados na config ou confirmação pendente.
	HooksNote() string
}

// hookGroup é um grupo de hooks de um evento, na forma que os dois CLIs
// gravam: um matcher opcional e a lista de comandos.
type hookGroup struct {
	Matcher string      `json:"matcher,omitempty"`
	Hooks   []hookEntry `json:"hooks"`
}

type hookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

// hookDoc é um arquivo JSON com um mapa de hooks na chave "hooks" — o
// formato do settings.json do Claude Code e do hooks.json do Codex.
//
// Os grupos ficam crus (json.RawMessage) e só são reescritos quando o hook
// que estamos mexendo está dentro deles: grupo alheio volta ao disco byte a
// byte, com os campos que o lazyagents nem conhece.
type hookDoc struct {
	file  *settings
	byEv  *object // mapa evento → lista de grupos
	order []string
}

func openHookDoc(path string) (*hookDoc, error) {
	s, err := readSettings(path)
	if err != nil {
		return nil, err
	}
	d := &hookDoc{file: s, byEv: &object{}}
	if raw, ok := s.raw(hookGroupsKey); ok {
		o, err := decodeObject(raw)
		if err != nil {
			return nil, fmt.Errorf("lendo %s: chave %q: %w", path, hookGroupsKey, err)
		}
		d.byEv = o
	}
	d.order = d.byEv.keys()
	return d, nil
}

// eventKey encontra a chave do evento como ela está no arquivo (o Codex
// aceita o nome em CamelCase e normaliza internamente; casar sem diferenciar
// caixa evita criar uma chave duplicada).
func (d *hookDoc) eventKey(event string) string {
	for _, k := range d.order {
		if sameEvent(k, event) {
			return k
		}
	}
	return event
}

func (d *hookDoc) groups(event string) []json.RawMessage {
	var groups []json.RawMessage
	if _, err := d.byEv.get(d.eventKey(event), &groups); err != nil {
		return nil
	}
	return groups
}

// list devolve todos os hooks do arquivo. Entrada que não é do tipo
// "command" é ignorada na leitura (e preservada na escrita).
func (d *hookDoc) list() []Hook {
	var out []Hook
	for _, event := range d.byEv.keys() {
		for _, raw := range d.groups(event) {
			var g hookGroup
			if err := json.Unmarshal(raw, &g); err != nil {
				continue
			}
			for _, e := range g.Hooks {
				if e.Type != "" && e.Type != hookCommandType {
					continue
				}
				out = append(out, Hook{Event: event, Matcher: g.Matcher, Command: e.Command, Timeout: e.Timeout})
			}
		}
	}
	return out
}

// add instala o hook como um grupo novo no fim do evento. Grupo existente
// nunca é editado — assim nenhum campo desconhecido se perde. Devolve false
// quando o hook já estava lá.
func (d *hookDoc) add(h Hook) (bool, error) {
	for _, existing := range d.list() {
		if existing.Same(h) {
			return false, nil
		}
	}
	group, err := json.Marshal(hookGroup{
		Matcher: h.Matcher,
		Hooks:   []hookEntry{{Type: hookCommandType, Command: h.Command, Timeout: h.Timeout}},
	})
	if err != nil {
		return false, err
	}
	key := d.eventKey(h.Event)
	groups := append(d.groups(h.Event), group)
	if err := d.byEv.set(key, groups); err != nil {
		return false, err
	}
	d.order = d.byEv.keys()
	return true, nil
}

// remove tira o hook do arquivo. Grupo que só tinha esse hook sai inteiro;
// grupo compartilhado com outros comandos é reescrito sem ele (único caso em
// que um grupo alheio é regravado). Devolve false quando não havia o que tirar.
func (d *hookDoc) remove(h Hook) (bool, error) {
	key := d.eventKey(h.Event)
	groups := d.groups(h.Event)
	kept := make([]json.RawMessage, 0, len(groups))
	changed := false
	for _, raw := range groups {
		var g hookGroup
		if err := json.Unmarshal(raw, &g); err != nil || g.Matcher != h.Matcher {
			kept = append(kept, raw)
			continue
		}
		rest := make([]hookEntry, 0, len(g.Hooks))
		for _, e := range g.Hooks {
			if e.Command == h.Command && (e.Type == "" || e.Type == hookCommandType) {
				changed = true
				continue
			}
			rest = append(rest, e)
		}
		switch {
		case len(rest) == len(g.Hooks):
			kept = append(kept, raw)
		case len(rest) > 0:
			g.Hooks = rest
			redone, err := json.Marshal(g)
			if err != nil {
				return false, err
			}
			kept = append(kept, redone)
		}
	}
	if !changed {
		return false, nil
	}
	if len(kept) == 0 {
		d.byEv.delete(key)
	} else if err := d.byEv.set(key, kept); err != nil {
		return false, err
	}
	d.order = d.byEv.keys()
	return true, nil
}

// save grava o arquivo com backup. Mapa de hooks vazio some do arquivo, para
// não deixar lixo onde não havia nada.
func (d *hookDoc) save(backupsDir string) error {
	if d.byEv.empty() {
		d.file.delete(hookGroupsKey)
	} else if err := d.file.set(hookGroupsKey, d.byEv); err != nil {
		return err
	}
	return d.file.save(backupsDir)
}

// hookAdd e hookRemove são o corpo compartilhado pelos adapters cujo arquivo
// tem o mapa de hooks na chave "hooks".
func hookAdd(path string, h Hook, backupsDir string) error {
	d, err := openHookDoc(path)
	if err != nil {
		return err
	}
	changed, err := d.add(h)
	if err != nil || !changed {
		return err
	}
	return d.save(backupsDir)
}

func hookRemove(path string, h Hook, backupsDir string) error {
	d, err := openHookDoc(path)
	if err != nil {
		return err
	}
	changed, err := d.remove(h)
	if err != nil || !changed {
		return err
	}
	return d.save(backupsDir)
}

func hookList(path string) ([]Hook, error) {
	d, err := openHookDoc(path)
	if err != nil {
		return nil, err
	}
	return d.list(), nil
}
