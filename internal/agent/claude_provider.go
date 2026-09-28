package agent

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// Claude Code env keys a provider profile controls, read from settings.json "env".
const (
	claudeEnvBaseURL = "ANTHROPIC_BASE_URL"
	claudeEnvToken   = "ANTHROPIC_AUTH_TOKEN"
	claudeEnvModel   = "ANTHROPIC_MODEL"
)

var claudeProviderKeys = []string{claudeEnvBaseURL, claudeEnvToken, claudeEnvModel}

// claudeProviderState is what lazyagents wrote into settings.json "env", kept
// in its own data dir so the user's file carries no marker. Written holds a
// digest per key, never the value: the token must not reach this file.
type claudeProviderState struct {
	Written map[string]string `json:"written"`
	// Original is the user's value a profile replaced, restored when lazyagents
	// lets go of the key. Never the token (a secret): that one lives only in the backup.
	Original map[string]string `json:"original,omitempty"`
	// CreatedEnv says "env" did not exist before lazyagents added it.
	CreatedEnv bool `json:"createdEnv,omitempty"`
	// CreatedFile says settings.json itself did not exist before.
	CreatedFile bool `json:"createdFile,omitempty"`
}

func (c *Claude) ProviderFile() string { return filepath.Join(c.configDir(), "settings.json") }

// claudeEnv reads settings.json "env" as a string map. Non-string values (the
// file is the user's) are ignored, not errors.
func claudeEnv(s *settings) (map[string]string, error) {
	env := map[string]string{}
	var raw map[string]any
	ok, err := s.get("env", &raw)
	if err != nil || !ok {
		return env, err
	}
	for k, v := range raw {
		if str, ok := v.(string); ok {
			env[k] = str
		}
	}
	return env, nil
}

func (c *Claude) ReadProvider() (ProviderProfile, bool, error) {
	s, err := readSettings(c.ProviderFile())
	if err != nil {
		return ProviderProfile{}, false, err
	}
	env, err := claudeEnv(s)
	if err != nil {
		return ProviderProfile{}, false, err
	}
	p := ProviderProfile{
		BaseURL:  env[claudeEnvBaseURL],
		Model:    env[claudeEnvModel],
		HasToken: env[claudeEnvToken] != "",
	}
	if p.BaseURL == "" && p.Model == "" && !p.HasToken {
		return ProviderProfile{}, false, nil
	}
	return p, true, nil
}

func (c *Claude) ApplyProvider(p ProviderProfile, backupsDir string) error {
	return c.writeEnv(backupsDir, map[string]string{
		claudeEnvBaseURL: p.BaseURL,
		claudeEnvToken:   p.Token,
		claudeEnvModel:   p.Model,
	})
}

func (c *Claude) ClearProvider(backupsDir string) error {
	return c.writeEnv(backupsDir, map[string]string{})
}

// writeEnv sets the provider keys in "env" and lets go of the ones lazyagents
// wrote that values no longer sets. A key the user wrote, or changed after
// lazyagents wrote it, is never removed. The order of "env" is kept.
func (c *Claude) writeEnv(backupsDir string, values map[string]string) error {
	path := c.ProviderFile()
	s, err := readSettings(path)
	if err != nil {
		return err
	}
	env := &object{}
	raw, hadEnv := s.raw("env")
	if hadEnv && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		if env, err = decodeObject(raw); err != nil {
			return fmt.Errorf("reading %s: env: %w", path, err)
		}
	} else {
		hadEnv = false
	}
	st, err := c.loadProviderState()
	if err != nil {
		return err
	}
	// Without a record nothing is ours: a key may be the user's own proxy or
	// Bedrock setup, so neither apply nor clear removes an unrecorded key.
	before := maps.Clone(st.Written)

	for _, k := range claudeProviderKeys {
		cur, has := envString(env, k)
		ours := has && st.Written[k] == digest(cur)
		if v := values[k]; v != "" {
			if has && !ours && k != claudeEnvToken {
				if _, saved := st.Original[k]; !saved {
					st.Original[k] = cur
				}
			}
			if err := env.set(k, v); err != nil {
				return err
			}
			st.Written[k] = digest(v)
			continue
		}
		if ours {
			if orig, ok := st.Original[k]; ok {
				if err := env.set(k, orig); err != nil {
					return err
				}
			} else {
				env.delete(k)
			}
		}
		delete(st.Written, k)
		delete(st.Original, k)
	}

	if len(st.Written) > 0 && !hadEnv {
		st.CreatedEnv = true
	}
	if len(st.Written) > 0 && s.missing {
		st.CreatedFile = true
	}
	var set any = env
	if env.empty() && (st.CreatedEnv || !hadEnv) {
		set = nil // lazyagents created "env": leave no empty object behind
	}
	createdFile := st.CreatedFile
	if len(st.Written) == 0 {
		st.CreatedEnv, st.CreatedFile = false, false
	}
	if err := s.set("env", set); err != nil {
		return err
	}
	if values[claudeEnvToken] != "" {
		s.perm = 0o600
	}
	// Record old and new digests before touching settings.json: if the save
	// below fails, whichever values are on disk are still known as ours.
	pending := *st
	pending.Written = maps.Clone(st.Written)
	maps.Copy(pending.Written, before)
	if err := c.saveProviderState(&pending); err != nil {
		return err
	}
	// Only lazyagents ever put something in a file it created: remove it
	// instead of leaving "{}" behind. Anything else in it (hooks) keeps it.
	if len(st.Written) == 0 && createdFile && s.object.empty() {
		if backupsDir != "" {
			if _, err := fsutil.Backup(path, backupsDir); err != nil {
				return err
			}
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("removing %s: %w", path, err)
		}
	} else if err := s.save(backupsDir); err != nil {
		return err
	}
	return c.saveProviderState(st)
}

// envString returns env[k] when it is a string.
func envString(env *object, k string) (string, bool) {
	var v string
	ok, err := env.get(k, &v)
	return v, ok && err == nil
}

func digest(v string) string {
	sum := sha256.Sum256([]byte(v))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func (c *Claude) loadProviderState() (*claudeProviderState, error) {
	st := &claudeProviderState{}
	switch {
	case c.ProviderState == "":
		if c.providerMem != nil {
			*st = *c.providerMem
			st.Written, st.Original = maps.Clone(st.Written), maps.Clone(st.Original) // a failed write must not leak into memory
		}
	default:
		data, err := os.ReadFile(c.ProviderState)
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", c.ProviderState, err)
		}
		if err := json.Unmarshal(data, st); err != nil {
			return nil, fmt.Errorf("reading %s: %w", c.ProviderState, err)
		}
	}
	if st.Written == nil {
		st.Written = map[string]string{}
	}
	if st.Original == nil {
		st.Original = map[string]string{}
	}
	return st, nil
}

func (c *Claude) saveProviderState(st *claudeProviderState) error {
	if c.ProviderState == "" {
		cp := *st
		cp.Written, cp.Original = maps.Clone(st.Written), maps.Clone(st.Original)
		c.providerMem = &cp
		return nil
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("writing %s: %w", c.ProviderState, err)
	}
	return fsutil.WriteAtomic(c.ProviderState, append(data, '\n'), 0o600)
}
