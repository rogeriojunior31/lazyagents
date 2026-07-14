package skill

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// Issue é um problema de qualidade no SKILL.md de uma skill, segundo a spec
// Agent Skills (agentskills.io). Distinto de Skill.Warning: Warning cobre
// falhas de leitura/parsing (frontmatter ilegível); Issue cobre convenções da
// spec num SKILL.md que já parseou com sucesso.
type Issue struct {
	Field string
	Msg   string
}

// maxDescriptionLen é o teto de tamanho da description recomendado pela spec.
const maxDescriptionLen = 1024

// Validate faz o lint local do SKILL.md de sk: name presente/kebab-case/igual
// à pasta, description não vazia e dentro do limite, corpo não vazio após o
// frontmatter. Skill com frontmatter ilegível (sk.Valid == false) já tem o
// problema reportado via Warning — Validate devolve nil nesse caso, sem
// duplicar o diagnóstico. Leitura local, sem rede.
func Validate(sk Skill) []Issue {
	if !sk.Valid {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(sk.Path, "SKILL.md"))
	if err != nil {
		return []Issue{{Field: "SKILL.md", Msg: fmt.Sprintf("lendo arquivo: %v", err)}}
	}
	meta, ok := ParseMeta(data)
	if !ok {
		return []Issue{{Field: "SKILL.md", Msg: "frontmatter YAML inválido"}}
	}

	var issues []Issue
	switch {
	case meta.Name == "":
		issues = append(issues, Issue{Field: "name", Msg: "ausente no frontmatter"})
	case !skillNameRe.MatchString(meta.Name):
		issues = append(issues, Issue{Field: "name", Msg: "não é kebab-case"})
	case meta.Name != sk.Dir:
		issues = append(issues, Issue{Field: "name", Msg: fmt.Sprintf("difere da pasta (%q)", sk.Dir)})
	}
	switch {
	case meta.Description == "":
		issues = append(issues, Issue{Field: "description", Msg: "vazia"})
	case len(meta.Description) > maxDescriptionLen:
		issues = append(issues, Issue{Field: "description", Msg: fmt.Sprintf("passa de %d chars (tem %d)", maxDescriptionLen, len(meta.Description))})
	}
	if bodyAfterFrontmatter(data) == "" {
		issues = append(issues, Issue{Field: "corpo", Msg: "sem instruções após o frontmatter"})
	}
	return issues
}

// bodyAfterFrontmatter devolve o conteúdo depois do fecho do frontmatter
// (segunda cerca --- ou ...), aparado. Sem frontmatter, devolve o arquivo
// inteiro aparado — mesma tolerância do parser em frontmatter().
func bodyAfterFrontmatter(data []byte) string {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	lines := bytes.Split(data, []byte("\n"))
	if len(lines) == 0 || string(bytes.TrimRight(lines[0], "\r ")) != "---" {
		return string(bytes.TrimSpace(data))
	}
	for i := 1; i < len(lines); i++ {
		t := string(bytes.TrimRight(lines[i], "\r "))
		if t == "---" || t == "..." {
			return string(bytes.TrimSpace(bytes.Join(lines[i+1:], []byte("\n"))))
		}
	}
	return ""
}
