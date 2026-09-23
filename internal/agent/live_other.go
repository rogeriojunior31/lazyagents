//go:build !linux

package agent

import (
	"os/exec"
	"strings"
)

// liveOpenFiles devolve, dentre paths, os que estão abertos por algum
// processo agora (via lsof, um único processo para todos de uma vez — nunca
// um lsof por sessão). Best-effort: sem lsof no PATH ou nenhum aberto, mapa
// vazio, nunca erro.
func liveOpenFiles(paths []string) map[string]bool {
	live := make(map[string]bool)
	if len(paths) == 0 {
		return live
	}
	// -F n: uma linha "n<caminho>" por arquivo aberto, sem colunas a parsear
	out, _ := exec.Command("lsof", append([]string{"-F", "n", "--"}, paths...)...).Output()
	want := make(map[string]bool, len(paths))
	for _, p := range paths {
		want[p] = true
	}
	for _, line := range strings.Split(string(out), "\n") {
		if p, ok := strings.CutPrefix(line, "n"); ok && want[p] {
			live[p] = true
		}
	}
	return live
}
