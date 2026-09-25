package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// settingsBackups is how many backups of one settings file are kept.
const settingsBackups = 20

// object is a JSON object with the original key order and raw values: only
// the key being changed is decoded; the rest goes back to disk as it came.
// The surgical-edit primitive for live CLI config, used for whole files
// (settings) and nested objects (the hooks map).
type object struct {
	pairs []objectPair
}

type objectPair struct {
	key string
	raw json.RawMessage
}

// decodeObject reads a JSON object keeping key order (a map would lose it, and
// the file may be hand-edited). Empty input is an empty object.
func decodeObject(data []byte) (*object, error) {
	o := &object{}
	if len(bytes.TrimSpace(data)) == 0 {
		return o, nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, fmt.Errorf("expected a JSON object")
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := keyTok.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, fmt.Errorf("key %q: %w", key, err)
		}
		o.pairs = append(o.pairs, objectPair{key: key, raw: raw})
	}
	return o, nil
}

// get decodes key into out. ok=false when absent (out untouched).
func (o *object) get(key string, out any) (bool, error) {
	raw, ok := o.raw(key)
	if !ok {
		return false, nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return false, fmt.Errorf("key %q: %w", key, err)
	}
	return true, nil
}

func (o *object) raw(key string) (json.RawMessage, bool) {
	for _, p := range o.pairs {
		if p.key == key {
			return p.raw, true
		}
	}
	return nil, false
}

func (o *object) keys() []string {
	out := make([]string, len(o.pairs))
	for i, p := range o.pairs {
		out[i] = p.key
	}
	return out
}

// set writes v at key, in place (or at the end if new). nil v removes the key.
func (o *object) set(key string, v any) error {
	if v == nil {
		o.delete(key)
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encoding %q: %w", key, err)
	}
	for i := range o.pairs {
		if o.pairs[i].key == key {
			o.pairs[i].raw = raw
			return nil
		}
	}
	o.pairs = append(o.pairs, objectPair{key: key, raw: raw})
	return nil
}

func (o *object) delete(key string) {
	out := o.pairs[:0]
	for _, p := range o.pairs {
		if p.key != key {
			out = append(out, p)
		}
	}
	o.pairs = out
}

func (o *object) empty() bool { return len(o.pairs) == 0 }

// MarshalJSON emits the object compactly in the original order, so an object
// can nest inside another.
func (o *object) MarshalJSON() ([]byte, error) {
	if o == nil || len(o.pairs) == 0 {
		return []byte("{}"), nil
	}
	var b bytes.Buffer
	b.WriteByte('{')
	for i, p := range o.pairs {
		if i > 0 {
			b.WriteByte(',')
		}
		key, err := json.Marshal(p.key)
		if err != nil {
			return nil, err
		}
		b.Write(key)
		b.WriteByte(':')
		if err := json.Compact(&b, p.raw); err != nil {
			return nil, fmt.Errorf("encoding %q: %w", p.key, err)
		}
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// indented serializes the object as a file with 2-space indent, as the CLIs
// write it. Values are recompacted first so hand-formatted files come out
// consistent.
func (o *object) indented() ([]byte, error) {
	if len(o.pairs) == 0 {
		return []byte("{}\n"), nil
	}
	compact, err := o.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	if err := json.Indent(&b, compact, "", "  "); err != nil {
		return nil, err
	}
	b.WriteByte('\n')
	return b.Bytes(), nil
}

// settings is a live CLI JSON file (e.g. ~/.claude/settings.json) open for
// surgical edits, shared by every module that writes agent config.
type settings struct {
	*object
	path string
	perm os.FileMode
}

// readSettings reads path. A missing or empty file is an empty document with
// mode 0600 (these files often hold tokens); an existing file keeps its mode.
func readSettings(path string) (*settings, error) {
	s := &settings{object: &object{}, path: path, perm: 0o600}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if info, err := os.Stat(path); err == nil {
		s.perm = info.Mode().Perm()
	}
	o, err := decodeObject(data)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	s.object = o
	return s, nil
}

// save backs the live file up into backupsDir (no-op if it does not exist yet)
// and rewrites it atomically with its original mode. An empty backupsDir skips
// the backup, for callers that already made one.
func (s *settings) save(backupsDir string) error {
	data, err := s.indented()
	if err != nil {
		return fmt.Errorf("writing %s: %w", s.path, err)
	}
	if backupsDir != "" {
		if _, err := fsutil.Backup(s.path, backupsDir); err != nil {
			return err
		}
		_ = fsutil.RotateBackups(backupsDir, filepath.Base(s.path)+".", settingsBackups)
	}
	if err := fsutil.WriteAtomic(s.path, data, s.perm); err != nil {
		return fmt.Errorf("writing %s: %w", s.path, err)
	}
	return nil
}
