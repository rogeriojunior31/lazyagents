package sessions

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// Aliases belong to lazyagents, never to the CLI: they live in
// <DataDir>/session-aliases.json as {"aliases": {"<agent>:<id>": "alias"}}.
// Unknown top-level keys survive rewrites.

// aliasKey is agent-scoped: ids may collide across CLIs.
func aliasKey(s agent.Session) string { return s.AgentID + ":" + s.ID }

func (s *Service) readAliases() (map[string]json.RawMessage, map[string]string, error) {
	raw := map[string]json.RawMessage{}
	aliases := map[string]string{}
	data, err := os.ReadFile(s.aliasesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return raw, aliases, nil
		}
		return nil, nil, fmt.Errorf("reading aliases: %w", err)
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil, fmt.Errorf("invalid aliases in %s: %w", s.aliasesPath, err)
	}
	if v, ok := raw["aliases"]; ok {
		if err := json.Unmarshal(v, &aliases); err != nil {
			return nil, nil, fmt.Errorf("invalid aliases in %s: %w", s.aliasesPath, err)
		}
	}
	if raw == nil {
		raw = map[string]json.RawMessage{}
	}
	if aliases == nil {
		aliases = map[string]string{}
	}
	return raw, aliases, nil
}

// SetAlias saves the session alias; a blank alias removes it.
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
