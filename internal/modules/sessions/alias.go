package sessions

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// Apelidos são do lazyagents, nunca do CLI: ficam em
// <DataDir>/session-aliases.json como {"aliases": {"<agente>:<id>": "apelido"}}.
// Chaves de topo desconhecidas sobrevivem à reescrita.

// aliasKey identifica a sessão entre agentes (ids podem colidir entre CLIs).
func aliasKey(s agent.Session) string { return s.AgentID + ":" + s.ID }

func (s *Service) readAliases() (map[string]json.RawMessage, map[string]string, error) {
	raw := map[string]json.RawMessage{}
	aliases := map[string]string{}
	data, err := os.ReadFile(s.aliasesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return raw, aliases, nil
		}
		return nil, nil, fmt.Errorf("lendo apelidos: %w", err)
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil, fmt.Errorf("apelidos em %s inválidos: %w", s.aliasesPath, err)
	}
	if v, ok := raw["aliases"]; ok {
		if err := json.Unmarshal(v, &aliases); err != nil {
			return nil, nil, fmt.Errorf("apelidos em %s inválidos: %w", s.aliasesPath, err)
		}
	}
	return raw, aliases, nil
}

// SetAlias grava o apelido da sessão; vazio (ou só espaços) remove.
func (s *Service) SetAlias(sess agent.Session, alias string) error {
	raw, aliases, err := s.readAliases()
	if err != nil {
		return err
	}
	if alias = strings.TrimSpace(alias); alias == "" {
		delete(aliases, aliasKey(sess))
	} else {
		aliases[aliasKey(sess)] = alias
	}
	if raw["aliases"], err = json.Marshal(aliases); err != nil {
		return err
	}
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteAtomic(s.aliasesPath, data, 0o644)
}
