package skill

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// UpdateStatus descreve o estado de atualização de uma skill git.
type UpdateStatus int

const (
	UpdateStatusUnknown       UpdateStatus = iota // hash ausente ou erro ao verificar
	UpdateStatusUpToDate                          // conteúdo idêntico ao remoto
	UpdateStatusAvailable                         // nova versão disponível no remoto
	UpdateStatusLocallyEdited                     // conteúdo local diverge do hash gravado
)

// UpdateCheck é o resultado da verificação de uma skill individual.
type UpdateCheck struct {
	Skill  Skill
	Status UpdateStatus
	Err    error
}

// hashDir calcula um SHA-256 estável do conteúdo de dir: percorre em ordem
// lexicográfica, ignora arquivos e diretórios ocultos (ponto inicial) e
// concatena rel + "\x00" + conteúdo de cada arquivo regular.
func hashDir(dir string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\x00", filepath.ToSlash(rel))
		h.Write(data)
		return nil
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// CheckUpdates verifica se há atualizações disponíveis para skills com origem
// git. Agrupa por URL de origem para clonar cada repositório uma única vez.
// Erros por skill são retornados em UpdateCheck.Err; o slice sempre é retornado.
func (s *Service) CheckUpdates(skills []Skill) ([]UpdateCheck, error) {
	var gitSkills []Skill
	for _, sk := range skills {
		if sk.Origin != nil && sk.Origin.Type == "git" && sk.InLibrary {
			gitSkills = append(gitSkills, sk)
		}
	}
	if len(gitSkills) == 0 {
		return nil, nil
	}

	// agrupa por source para clonar cada repo uma vez só
	bySource := make(map[string][]Skill)
	var order []string
	seen := make(map[string]bool)
	for _, sk := range gitSkills {
		src := sk.Origin.Source
		if !seen[src] {
			seen[src] = true
			order = append(order, src)
		}
		bySource[src] = append(bySource[src], sk)
	}

	var out []UpdateCheck
	for _, source := range order {
		tmp, cloneErr := cloneShallow(source)
		for _, sk := range bySource[source] {
			if cloneErr != nil {
				out = append(out, UpdateCheck{Skill: sk, Status: UpdateStatusUnknown, Err: cloneErr})
				continue
			}
			out = append(out, s.checkOne(tmp, sk))
		}
		if tmp != "" {
			_ = os.RemoveAll(tmp)
		}
	}
	return out, nil
}

func (s *Service) checkOne(cloneDir string, sk Skill) UpdateCheck {
	libPath := filepath.Join(s.paths.LibraryDir(), sk.Dir)
	localHash, err := hashDir(libPath)
	if err != nil {
		return UpdateCheck{Skill: sk, Status: UpdateStatusUnknown,
			Err: fmt.Errorf("hash local: %w", err)}
	}

	// editada localmente: hash atual difere do gravado no install/update
	if sk.Origin.Hash != "" && localHash != sk.Origin.Hash {
		return UpdateCheck{Skill: sk, Status: UpdateStatusLocallyEdited}
	}

	srcDir, err := locateInClone(cloneDir, sk)
	if err != nil {
		return UpdateCheck{Skill: sk, Status: UpdateStatusUnknown,
			Err: fmt.Errorf("localizando no clone: %w", err)}
	}
	remoteHash, err := hashDir(srcDir)
	if err != nil {
		return UpdateCheck{Skill: sk, Status: UpdateStatusUnknown,
			Err: fmt.Errorf("hash remoto: %w", err)}
	}

	if localHash == remoteHash {
		return UpdateCheck{Skill: sk, Status: UpdateStatusUpToDate}
	}
	return UpdateCheck{Skill: sk, Status: UpdateStatusAvailable}
}

// UpdateAll atualiza todas as skills com UpdateStatusAvailable.
// Skills editadas localmente são puladas (retornadas em skipped).
// Falha em uma não aborta as demais — mesmo padrão do Service.List.
func (s *Service) UpdateAll(checks []UpdateCheck) (updated int, skipped []string, errs []error) {
	for _, c := range checks {
		switch c.Status {
		case UpdateStatusAvailable:
			if err := s.Update(c.Skill); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", c.Skill.Dir, err))
			} else {
				updated++
			}
		case UpdateStatusLocallyEdited:
			skipped = append(skipped, c.Skill.Dir)
		}
	}
	return
}
