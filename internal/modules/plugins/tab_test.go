package plugins

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui/events"
)

const fixture = `#!/bin/sh
read init
printf '{"type":"manifest","title":"Eco","help":[{"title":"Eco","keys":[["x","eco"]]}],"commands":[{"name":"ping","desc":"pinga"}]}\n'
printf '{"type":"frame","view":"\\u001b[1molá\\u001b[0m\\u001b[2J","count":3}\n'
while read line; do
  case "$line" in
    *'"type":"key"'*) k=$(printf '%s' "$line" | sed 's/.*"key":"\([^"]*\)".*/\1/'); printf '{"type":"frame","view":"tecla %s","capturing":true}\n' "$k" ;;
    *'"type":"command"'*) printf '{"type":"exec","execId":7,"argv":["sh","-c","echo out; exit 2"]}\n' ;;
    *'"type":"exec_result"'*) c=$(printf '%s' "$line" | sed 's/.*"code":\([0-9]*\).*/\1/'); printf '{"type":"frame","view":"exec code %s"}\n' "$c" ;;
    *'"type":"reload"'*) exit 0 ;;
  esac
done
`

func newModule(t *testing.T, script string) *Tab {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sem sh no PATH")
	}
	svc := New(core.PathsIn(t.TempDir()))
	svc.Handshake = 500 * time.Millisecond
	t.Cleanup(svc.Close)
	if err := os.MkdirAll(svc.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(svc.Dir, "eco")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return newTab(svc, Plugin{ID: "eco", Path: path}, Msg{})
}

// run executa o cmd devolvido pelo módulo e entrega cada msg ao Update
// conforme chega, como o runtime faz (filhos de um Batch em paralelo);
// devolve o último cmd não nulo.
func run(t *testing.T, m *Tab, cmd tea.Cmd) tea.Cmd {
	t.Helper()
	if cmd == nil {
		t.Fatal("cmd nulo: o módulo parou de escutar o plugin")
	}
	ch := make(chan tea.Msg, 16)
	spawn(cmd, ch)
	var next tea.Cmd
	for pending := 1; pending > 0; {
		select {
		case msg := <-ch:
			if sp, ok := msg.(spawned); ok {
				pending += sp.n - 1
				continue
			}
			pending--
			if c := m.Update(msg); c != nil {
				next = c
			}
		case <-time.After(3 * time.Second):
			t.Fatal("plugin não respondeu")
		}
	}
	return next
}

// spawned avisa o run que um Batch virou n cmds.
type spawned struct{ n int }

func spawn(cmd tea.Cmd, ch chan tea.Msg) {
	go func() {
		msg := cmd()
		if b, ok := msg.(tea.BatchMsg); ok {
			for _, c := range b {
				spawn(c, ch)
			}
			ch <- spawned{len(b)}
			return
		}
		ch <- msg
	}()
}

func TestModuleProxiesPlugin(t *testing.T) {
	m := newModule(t, fixture)
	if m.Title() != "Eco" || m.ID() != "eco" || len(m.Help()) != 1 || len(m.Commands()) != 1 {
		t.Fatalf("manifesto não aplicado: title=%q help=%d cmds=%d", m.Title(), len(m.Help()), len(m.Commands()))
	}
	cmd := run(t, m, m.Init())
	if !strings.Contains(m.View(), "\x1b[1molá") || strings.Contains(m.View(), "[2J") || m.Count() != 3 {
		t.Errorf("frame inicial: view=%q count=%d", m.View(), m.Count())
	}
	m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	cmd = run(t, m, cmd)
	if m.View() != "tecla x" || !m.Capturing() {
		t.Errorf("eco: view=%q capturing=%v", m.View(), m.Capturing())
	}
	m.Update(m.Commands()[0].Msg) // paleta → command → plugin pede exec
	cmd = run(t, m, cmd)          // frame(exec) → Update devolve Batch(wait, exec)
	cmd = run(t, m, cmd)          // exec roda → exec_result → frame final
	if m.View() != "exec code 2" {
		t.Errorf("exec: view=%q", m.View())
	}
	m.Update(events.Reload{}) // fixture sai com 0
	run(t, m, cmd)
	if m.proc != nil || !strings.Contains(m.View(), "plugin eco:") || m.Capturing() {
		t.Errorf("estado morto esperado: %q", m.View())
	}
	// r respawna
	cmd = m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	run(t, m, cmd)
	if m.proc == nil || m.Count() != 3 {
		t.Errorf("respawn falhou: %v", m.err)
	}
}

func TestModuleDeadOnStartFailure(t *testing.T) {
	m := newModule(t, "#!/bin/sh\necho falhei >&2\nexit 1\n")
	if m.proc != nil || m.Init() != nil || m.Title() != "eco" || m.Commands() != nil {
		t.Fatalf("deveria nascer morto: %+v", m)
	}
	if v := m.View(); !strings.Contains(v, "plugin eco:") || !strings.Contains(v, "falhei") {
		t.Errorf("view do estado morto: %q", v)
	}
	if cmd := m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"}); cmd != nil {
		t.Error("tecla em estado morto não deveria gerar cmd")
	}
}

func TestExecStopsWhenServiceCloses(t *testing.T) {
	m := newModule(t, fixture)
	cmd := m.execCmd(Msg{Argv: []string{"sh", "-c", "exec sleep 30"}})
	result := make(chan tea.Msg, 1)
	go func() { result <- cmd() }()
	m.svc.Close()
	select {
	case <-result:
	case <-time.After(4 * time.Second):
		t.Fatal("exec survived service close")
	}
}

func TestLongFailureCanBeReadAndRestarted(t *testing.T) {
	m := newModule(t, fixture)
	cmd := run(t, m, m.Init())
	m.Update(events.Reload{})
	run(t, m, cmd)
	m.stderr = strings.Repeat("diagnóstico extenso\n", 40) + "FIM_ERRO\x1b[2J"
	m.Update(tea.WindowSizeMsg{Width: 36, Height: 9})
	if !strings.Contains(m.View(), "restart") || m.Count() != -1 {
		t.Fatal("falha deve oferecer reinício e limpar contagem")
	}
	for i := 0; i < 50; i++ {
		m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	}
	if v := m.View(); !strings.Contains(v, "FIM_ERRO") || strings.Contains(v, "[2J") || !strings.Contains(v, "restart") {
		t.Fatalf("fim do erro inacessível ou controle não saneado: %q", v)
	}
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 14})
	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	run(t, m, m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"}))
	if m.proc == nil || m.scroll != 0 || m.Count() != 3 {
		t.Fatal("reinício não restaurou o plugin")
	}
}
