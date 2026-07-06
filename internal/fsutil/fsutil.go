// Package fsutil centraliza toda escrita em disco: WriteAtomic (tmp + rename),
// Backup e RotateBackups. Nenhum outro pacote escreve arquivo diretamente.
package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// backupTimeLayout é o formato do timestamp no nome do backup. Largura fixa para
// que a ordenação lexicográfica dos nomes coincida com a ordem cronológica.
const backupTimeLayout = "20060102T150405.000000000"

// WriteAtomic escreve data em path de forma atômica: grava num arquivo temporário
// no MESMO diretório de path (rename atômico não cruza filesystem) e em seguida faz
// os.Rename por cima do destino. Cria o diretório pai (0700) se ele não existir.
// Em caso de erro, o arquivo vivo permanece intacto e o temporário é removido.
func WriteAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("criando diretório %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("criando temporário em %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	// Em qualquer caminho de erro, garante a remoção do temporário.
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("escrevendo temporário: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("ajustando permissão do temporário: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sincronizando temporário: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("fechando temporário: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("renomeando %s -> %s: %w", tmpName, path, err)
	}
	return nil
}

// Backup copia o arquivo em path para backupDir, nomeando-o como
// "<basename>.<timestamp>" e preservando a permissão do original (que pode conter
// segredos). Retorna o caminho do backup criado. Se path não existir, é no-op e
// retorna ("", nil) — não cria backupDir.
func Backup(path, backupDir string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("stat de %s: %w", path, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("backup de %s: é um diretório", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("lendo %s: %w", path, err)
	}

	name := fmt.Sprintf("%s.%s", filepath.Base(path), time.Now().Format(backupTimeLayout))
	dst := filepath.Join(backupDir, name)
	if err := WriteAtomic(dst, data, info.Mode().Perm()); err != nil {
		return "", fmt.Errorf("gravando backup %s: %w", dst, err)
	}
	return dst, nil
}

// RotateBackups mantém apenas os keep backups mais recentes em backupDir cujo nome
// começa com prefix, removendo os mais antigos. A idade é inferida pela ordenação
// lexicográfica do nome (ver backupTimeLayout). keep <= 0 é no-op defensivo: nunca
// remove tudo por um valor zerado acidental.
func RotateBackups(backupDir, prefix string, keep int) error {
	if keep <= 0 {
		return nil
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("lendo diretório de backups %s: %w", backupDir, err)
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), prefix) {
			names = append(names, e.Name())
		}
	}
	if len(names) <= keep {
		return nil
	}

	sort.Strings(names) // ascendente: mais antigos primeiro
	toRemove := names[:len(names)-keep]
	for _, n := range toRemove {
		if err := os.Remove(filepath.Join(backupDir, n)); err != nil {
			return fmt.Errorf("removendo backup antigo %s: %w", n, err)
		}
	}
	return nil
}
