package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// settingsBackups é quantos backups de um mesmo arquivo de settings ficam
// guardados; os mais antigos são rotacionados fora.
const settingsBackups = 20

// object é um objeto JSON com as chaves na ordem original e os valores
// crus: só a chave que vamos mexer é decodificada, todas as outras voltam ao
// disco como vieram. É o primitivo de edição cirúrgica de config viva de
// CLI, usado tanto no arquivo inteiro (settings) quanto num objeto aninhado
// (o mapa de hooks).
type object struct {
	pairs []objectPair
}

type objectPair struct {
	key string
	raw json.RawMessage
}

// decodeObject lê um objeto JSON preservando a ordem das chaves. Um map
// perderia essa ordem, e o arquivo é do usuário — pode ter sido editado à
// mão. Entrada vazia vira objeto vazio.
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
		return nil, fmt.Errorf("esperava um objeto JSON")
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := keyTok.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, fmt.Errorf("chave %q: %w", key, err)
		}
		o.pairs = append(o.pairs, objectPair{key: key, raw: raw})
	}
	return o, nil
}

// get decodifica a chave key em out. ok=false quando não existe (out fica
// intacto).
func (o *object) get(key string, out any) (bool, error) {
	raw, ok := o.raw(key)
	if !ok {
		return false, nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return false, fmt.Errorf("chave %q: %w", key, err)
	}
	return true, nil
}

// raw devolve o valor cru de uma chave.
func (o *object) raw(key string) (json.RawMessage, bool) {
	for _, p := range o.pairs {
		if p.key == key {
			return p.raw, true
		}
	}
	return nil, false
}

// keys devolve as chaves na ordem do arquivo.
func (o *object) keys() []string {
	out := make([]string, len(o.pairs))
	for i, p := range o.pairs {
		out[i] = p.key
	}
	return out
}

// set grava v na chave key, no lugar que ela já ocupava (ou no fim, se for
// nova). v nil remove a chave.
func (o *object) set(key string, v any) error {
	if v == nil {
		o.delete(key)
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("codificando %q: %w", key, err)
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

// MarshalJSON emite o objeto compacto, na ordem original — é o que permite
// aninhar um object como valor de outro.
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
			return nil, fmt.Errorf("codificando %q: %w", p.key, err)
		}
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// indented serializa o objeto como arquivo: indentação de 2 espaços, que é o
// que os CLIs escrevem. Cada valor é recompactado antes de indentar para que
// um arquivo formatado à mão saia consistente.
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

// settings é um arquivo JSON vivo de um CLI (ex.: ~/.claude/settings.json)
// aberto para edição cirúrgica. É o primitivo compartilhado pelos módulos que
// escrevem em config de agente (providers, hooks) — ninguém reescreve esses
// arquivos na mão.
type settings struct {
	*object
	path string
	perm os.FileMode
}

// readSettings lê path. Arquivo ausente ou vazio vira um documento vazio, com
// permissão 0600 — esses arquivos costumam guardar token (regra 7), então o
// default é o restritivo; arquivo existente mantém a permissão que já tinha.
func readSettings(path string) (*settings, error) {
	s := &settings{object: &object{}, path: path, perm: 0o600}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("lendo %s: %w", path, err)
	}
	if info, err := os.Stat(path); err == nil {
		s.perm = info.Mode().Perm()
	}
	o, err := decodeObject(data)
	if err != nil {
		return nil, fmt.Errorf("lendo %s: %w", path, err)
	}
	s.object = o
	return s, nil
}

// save faz backup do arquivo vivo em backupsDir (no-op se ele ainda não
// existe) e regrava tudo atomicamente, preservando a permissão original.
// backupsDir vazio pula o backup — só para quem já fez o seu.
func (s *settings) save(backupsDir string) error {
	data, err := s.indented()
	if err != nil {
		return fmt.Errorf("gravando %s: %w", s.path, err)
	}
	if backupsDir != "" {
		if _, err := fsutil.Backup(s.path, backupsDir); err != nil {
			return err
		}
		_ = fsutil.RotateBackups(backupsDir, filepath.Base(s.path)+".", settingsBackups)
	}
	if err := fsutil.WriteAtomic(s.path, data, s.perm); err != nil {
		return fmt.Errorf("gravando %s: %w", s.path, err)
	}
	return nil
}
