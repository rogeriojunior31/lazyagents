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

// settings é um arquivo JSON vivo de um CLI (ex.: ~/.claude/settings.json)
// aberto para edição cirúrgica: só a chave de topo que vamos mexer é
// decodificada, todas as outras voltam ao disco como vieram e na ordem
// original. É o primitivo compartilhado pelos módulos que escrevem em config
// de agente (providers, hooks) — ninguém reescreve esses arquivos na mão.
type settings struct {
	path  string
	perm  os.FileMode
	pairs []settingPair // chaves de topo, na ordem do arquivo
}

type settingPair struct {
	key string
	raw json.RawMessage
}

// readSettings lê path. Arquivo ausente ou vazio vira um documento vazio, com
// permissão 0600 — esses arquivos costumam guardar token (regra 7), então o
// default é o restritivo; arquivo existente mantém a permissão que já tinha.
func readSettings(path string) (*settings, error) {
	s := &settings{path: path, perm: 0o600}
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
	if len(bytes.TrimSpace(data)) == 0 {
		return s, nil
	}
	// Decoder em vez de map: o token stream dá a ordem das chaves, que o
	// map perderia — o arquivo é do usuário e pode ser editado à mão.
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("lendo %s: %w", path, err)
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, fmt.Errorf("lendo %s: raiz não é um objeto JSON", path)
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("lendo %s: %w", path, err)
		}
		key, _ := keyTok.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, fmt.Errorf("lendo %s: chave %q: %w", path, key, err)
		}
		s.pairs = append(s.pairs, settingPair{key: key, raw: raw})
	}
	return s, nil
}

// get decodifica a chave de topo key em out. ok=false quando a chave não
// existe (out fica intacto).
func (s *settings) get(key string, out any) (bool, error) {
	for _, p := range s.pairs {
		if p.key != key {
			continue
		}
		if err := json.Unmarshal(p.raw, out); err != nil {
			return false, fmt.Errorf("lendo %s: chave %q: %w", s.path, key, err)
		}
		return true, nil
	}
	return false, nil
}

// set grava v na chave de topo key, no lugar que ela já ocupava (ou no fim,
// se for nova). v nil remove a chave.
func (s *settings) set(key string, v any) error {
	if v == nil {
		s.pairs = deleteKey(s.pairs, key)
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("codificando %q: %w", key, err)
	}
	for i := range s.pairs {
		if s.pairs[i].key == key {
			s.pairs[i].raw = raw
			return nil
		}
	}
	s.pairs = append(s.pairs, settingPair{key: key, raw: raw})
	return nil
}

func deleteKey(pairs []settingPair, key string) []settingPair {
	out := pairs[:0]
	for _, p := range pairs {
		if p.key != key {
			out = append(out, p)
		}
	}
	return out
}

// save faz backup do arquivo vivo em backupsDir (no-op se ele ainda não
// existe) e regrava tudo atomicamente, preservando a permissão original.
// backupsDir vazio pula o backup — só para quem já fez o seu.
func (s *settings) save(backupsDir string) error {
	data, err := s.encode()
	if err != nil {
		return err
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

// encode serializa o documento com indentação de 2 espaços, o formato que os
// CLIs escrevem. Cada valor é recompactado antes de indentar para que um
// arquivo formatado à mão saia consistente.
func (s *settings) encode() ([]byte, error) {
	if len(s.pairs) == 0 {
		return []byte("{}\n"), nil
	}
	var b bytes.Buffer
	b.WriteString("{\n")
	for i, p := range s.pairs {
		key, err := json.Marshal(p.key)
		if err != nil {
			return nil, fmt.Errorf("codificando %q: %w", p.key, err)
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, p.raw); err != nil {
			return nil, fmt.Errorf("codificando %q: %w", p.key, err)
		}
		b.WriteString("  ")
		b.Write(key)
		b.WriteString(": ")
		if err := json.Indent(&b, compact.Bytes(), "  ", "  "); err != nil {
			return nil, fmt.Errorf("codificando %q: %w", p.key, err)
		}
		if i < len(s.pairs)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteString("}\n")
	return b.Bytes(), nil
}
