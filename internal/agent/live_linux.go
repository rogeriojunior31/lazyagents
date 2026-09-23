package agent

import (
	"os"
	"path/filepath"
)

// liveOpenFiles devolve, dentre paths, os que estão abertos por algum
// processo agora. No Linux lê /proc/<pid>/fd direto: é o que o lsof faz, sem
// os ~130 ms fixos de subir o lsof e varrer tudo o mais. Processo de outro
// usuário não é legível, o que é certo: o agente roda como o próprio usuário.
// Best-effort: nada legível, mapa vazio, nunca erro.
func liveOpenFiles(paths []string) map[string]bool {
	live := make(map[string]bool)
	if len(paths) == 0 {
		return live
	}
	want := make(map[string][]string, len(paths)) // caminho real → como foi pedido
	for _, p := range paths {
		real, err := filepath.EvalSymlinks(p)
		if err != nil {
			real = p
		}
		want[real] = append(want[real], p)
	}
	procs, _ := os.ReadDir("/proc")
	for _, pr := range procs {
		if pr.Name()[0] < '0' || pr.Name()[0] > '9' {
			continue
		}
		dir := filepath.Join("/proc", pr.Name(), "fd")
		fds, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			if target, err := os.Readlink(filepath.Join(dir, fd.Name())); err == nil {
				for _, p := range want[target] {
					live[p] = true
				}
			}
		}
	}
	return live
}
