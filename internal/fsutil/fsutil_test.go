package fsutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteAtomic(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(dir string) string // retorna o path destino
		data    []byte
		perm    os.FileMode
		preData []byte // conteúdo pré-existente (sobrescrita)
	}{
		{
			name:  "arquivo novo",
			setup: func(dir string) string { return filepath.Join(dir, "novo.txt") },
			data:  []byte("conteúdo novo"),
			perm:  0o644,
		},
		{
			name:    "sobrescrita",
			setup:   func(dir string) string { return filepath.Join(dir, "existe.txt") },
			data:    []byte("conteúdo final"),
			perm:    0o644,
			preData: []byte("conteúdo antigo bem maior que o novo"),
		},
		{
			name:  "perm 0600",
			setup: func(dir string) string { return filepath.Join(dir, "segredo.json") },
			data:  []byte(`{"apiKey":"sk-xxx"}`),
			perm:  0o600,
		},
		{
			name:  "cria diretorio pai inexistente",
			setup: func(dir string) string { return filepath.Join(dir, "sub", "dir", "f.txt") },
			data:  []byte("nested"),
			perm:  0o600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := tt.setup(dir)
			if tt.preData != nil {
				if err := os.WriteFile(path, tt.preData, 0o644); err != nil {
					t.Fatalf("setup pré-arquivo: %v", err)
				}
			}

			if err := WriteAtomic(path, tt.data, tt.perm); err != nil {
				t.Fatalf("WriteAtomic: %v", err)
			}

			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("lendo resultado: %v", err)
			}
			if string(got) != string(tt.data) {
				t.Errorf("conteúdo = %q, quer %q", got, tt.data)
			}

			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("stat: %v", err)
			}
			if info.Mode().Perm() != tt.perm {
				t.Errorf("perm = %o, quer %o", info.Mode().Perm(), tt.perm)
			}

			// Não deve sobrar arquivo temporário no diretório.
			entries, err := os.ReadDir(filepath.Dir(path))
			if err != nil {
				t.Fatalf("readdir: %v", err)
			}
			for _, e := range entries {
				if strings.Contains(e.Name(), ".tmp-") {
					t.Errorf("arquivo temporário não removido: %s", e.Name())
				}
			}
		})
	}
}

func TestWriteAtomicError(t *testing.T) {
	t.Run("falha ao criar diretório pai (ancestral é arquivo)", func(t *testing.T) {
		dir := t.TempDir()
		arquivo := filepath.Join(dir, "sou-arquivo")
		if err := os.WriteFile(arquivo, []byte("x"), 0o600); err != nil {
			t.Fatalf("setup: %v", err)
		}
		// path cujo pai (arquivo/sub) não pode existir: arquivo é um arquivo regular.
		alvo := filepath.Join(arquivo, "sub", "f.txt")
		if err := WriteAtomic(alvo, []byte("y"), 0o600); err == nil {
			t.Error("WriteAtomic deveria falhar quando o diretório pai não pode ser criado")
		}
	})
}

func TestBackup(t *testing.T) {
	t.Run("arquivo existente é copiado com timestamp e perm preservada", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "config.json")
		content := []byte(`{"apiKey":"sk-secret"}`)
		if err := os.WriteFile(src, content, 0o600); err != nil {
			t.Fatalf("setup: %v", err)
		}
		backupDir := filepath.Join(dir, "backups")

		got, err := Backup(src, backupDir)
		if err != nil {
			t.Fatalf("Backup: %v", err)
		}
		if got == "" {
			t.Fatal("Backup retornou path vazio para arquivo existente")
		}
		if filepath.Dir(got) != backupDir {
			t.Errorf("backup criado em %s, quer dentro de %s", filepath.Dir(got), backupDir)
		}
		if !strings.HasPrefix(filepath.Base(got), "config.json.") {
			t.Errorf("nome do backup = %q, quer prefixo %q", filepath.Base(got), "config.json.")
		}

		bdata, err := os.ReadFile(got)
		if err != nil {
			t.Fatalf("lendo backup: %v", err)
		}
		if string(bdata) != string(content) {
			t.Errorf("conteúdo do backup = %q, quer %q", bdata, content)
		}

		info, err := os.Stat(got)
		if err != nil {
			t.Fatalf("stat backup: %v", err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("perm do backup = %o, quer 0600", info.Mode().Perm())
		}
	})

	t.Run("arquivo inexistente é no-op sem erro", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "nao-existe.json")
		backupDir := filepath.Join(dir, "backups")

		got, err := Backup(src, backupDir)
		if err != nil {
			t.Fatalf("Backup de inexistente deveria ser no-op, retornou erro: %v", err)
		}
		if got != "" {
			t.Errorf("Backup de inexistente retornou path %q, quer vazio", got)
		}
		if _, err := os.Stat(backupDir); !os.IsNotExist(err) {
			t.Errorf("backupDir não deveria ser criado para arquivo inexistente")
		}
	})

	t.Run("backup de diretório retorna erro", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "umdir")
		if err := os.Mkdir(src, 0o700); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if _, err := Backup(src, filepath.Join(dir, "backups")); err == nil {
			t.Error("Backup de diretório deveria retornar erro")
		}
	})
}

func TestRotateBackups(t *testing.T) {
	const prefix = "config.json."

	// nomes ordenáveis lexicograficamente (= cronologicamente), do mais antigo ao mais novo.
	mkNames := func(n int) []string {
		stamps := []string{
			"20240101T000001.000000000",
			"20240101T000002.000000000",
			"20240101T000003.000000000",
			"20240101T000004.000000000",
			"20240101T000005.000000000",
		}
		names := make([]string, n)
		for i := 0; i < n; i++ {
			names[i] = prefix + stamps[i]
		}
		return names
	}

	tests := []struct {
		name      string
		nBackups  int
		keep      int
		extra     []string // arquivos que NÃO casam o prefixo, devem sobreviver
		wantKept  int      // quantos backups (com prefixo) devem restar
		keepNewer bool     // se true, valida que os mantidos são os de timestamp maior
	}{
		{name: "mantém os N mais novos", nBackups: 5, keep: 3, wantKept: 3, keepNewer: true},
		{name: "keep maior que total mantém todos", nBackups: 2, keep: 5, wantKept: 2},
		{name: "keep zero é no-op defensivo", nBackups: 3, keep: 0, wantKept: 3},
		{name: "keep negativo é no-op defensivo", nBackups: 3, keep: -1, wantKept: 3},
		{name: "não toca arquivos de outro prefixo", nBackups: 4, keep: 1, wantKept: 1,
			extra: []string{"settings.json.20240101T000009.000000000", "outro.txt"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			names := mkNames(tt.nBackups)
			for _, n := range names {
				if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600); err != nil {
					t.Fatalf("setup: %v", err)
				}
			}
			for _, n := range tt.extra {
				if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600); err != nil {
					t.Fatalf("setup extra: %v", err)
				}
			}

			if err := RotateBackups(dir, prefix, tt.keep); err != nil {
				t.Fatalf("RotateBackups: %v", err)
			}

			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("readdir: %v", err)
			}
			var kept []string
			survivors := map[string]bool{}
			for _, e := range entries {
				survivors[e.Name()] = true
				if strings.HasPrefix(e.Name(), prefix) {
					kept = append(kept, e.Name())
				}
			}
			if len(kept) != tt.wantKept {
				t.Errorf("restaram %d backups com prefixo, quer %d (%v)", len(kept), tt.wantKept, kept)
			}
			// arquivos de outro prefixo intactos
			for _, n := range tt.extra {
				if !survivors[n] {
					t.Errorf("arquivo %q foi removido indevidamente", n)
				}
			}
			// quando keep < total, os mantidos devem ser os mais novos
			if tt.keepNewer {
				want := names[tt.nBackups-tt.keep:] // sufixo = mais novos
				for _, w := range want {
					if !survivors[w] {
						t.Errorf("backup mais novo %q deveria ter sido mantido", w)
					}
				}
			}
		})
	}

	t.Run("diretório de backups inexistente é no-op sem erro", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "nao-existe")
		if err := RotateBackups(dir, prefix, 3); err != nil {
			t.Errorf("RotateBackups em dir inexistente deveria ser no-op, retornou: %v", err)
		}
	})
}
