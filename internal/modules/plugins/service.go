package plugins

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"

	"github.com/rogeriojunior31/lazyagents/internal/core"
)

var (
	idRe   = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)
	nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
)

// Plugin é um binário descoberto em <ConfigDir>/plugins; ID é o nome do
// arquivo sem extensão e vira id da aba, subcomando da CLI e seção da config.
type Plugin struct {
	ID   string
	Path string
}

// Service descobre e executa plugins. Guarda os processos vivos para o Close
// na saída da TUI.
type Service struct {
	Dir       string        // <ConfigDir>/plugins
	Handshake time.Duration // espera máxima pelo manifesto (3 s; testes encurtam)

	ctx    context.Context
	cancel context.CancelFunc
	paths  core.Paths
	mu     sync.Mutex
	procs  []*Proc
}

// New monta o service sobre os paths do app.
func New(p core.Paths) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{Dir: p.PluginsDir(), Handshake: 3 * time.Second, paths: p, ctx: ctx, cancel: cancel}
}

// List devolve os plugins válidos em ordem alfabética e um aviso por entrada
// pulada (sem bit de execução, nome inválido, id duplicado). Dir ausente = nada.
func (s *Service) List() (pls []Plugin, warnings []string) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []string{fmt.Sprintf("lendo plugins: %v", err)}
	}
	seen := map[string]bool{}
	for _, e := range entries {
		path := filepath.Join(s.Dir, e.Name())
		st, err := os.Stat(path) // segue symlink
		if err != nil || !st.Mode().IsRegular() {
			continue
		}
		id := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		switch {
		case st.Mode()&0o111 == 0 && !(runtime.GOOS == "windows" && strings.EqualFold(filepath.Ext(path), ".exe")):
			warnings = append(warnings, fmt.Sprintf("plugin %s ignorado: sem permissão de execução", e.Name()))
		case !idRe.MatchString(id):
			warnings = append(warnings, fmt.Sprintf("plugin %s ignorado: nome precisa casar %s", e.Name(), idRe))
		case seen[id]:
			warnings = append(warnings, fmt.Sprintf("plugin %s ignorado: id %q duplicado", e.Name(), id))
		default:
			seen[id] = true
			pls = append(pls, Plugin{ID: id, Path: path})
		}
	}
	return pls, warnings
}

// Env é o ambiente entregue a todo plugin: o do host mais os paths do app.
func (s *Service) Env() []string {
	return append(os.Environ(),
		"LAZYAGENTS_HOME="+s.paths.Home,
		"LAZYAGENTS_CONFIG_DIR="+s.paths.ConfigDir,
		"LAZYAGENTS_DATA_DIR="+s.paths.DataDir,
		"LAZYAGENTS_LIBRARY_DIR="+s.paths.LibraryDir(),
		fmt.Sprintf("LAZYAGENTS_PROTOCOL=%d", Protocol),
	)
}

// Run executa `<bin> args…` com stdio herdado (pass-through da CLI e doctor)
// e devolve o exit code. Falha ao iniciar = 1 com a causa em errw.
func (s *Service) Run(pl Plugin, args []string, in io.Reader, out, errw io.Writer) int {
	cmd := exec.Command(pl.Path, args...)
	cmd.Env = s.Env()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = in, out, errw
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode()
		}
		fmt.Fprintf(errw, "lazyagents: plugin %s: %v\n", pl.ID, err)
		return 1
	}
	return 0
}

// Close encerra todos os processos iniciados por Start.
func (s *Service) Close() {
	s.cancel()
	s.mu.Lock()
	procs := s.procs
	s.procs = nil
	s.mu.Unlock()
	for _, p := range procs {
		_ = p.Close()
	}
}

// Proc é um plugin em modo `serve`. Events entrega as mensagens do plugin
// após o manifesto e fecha quando o processo termina ou viola o protocolo;
// Err diz por quê.
type Proc struct {
	Plugin   Plugin
	Manifest Msg
	Events   <-chan Msg

	cmd    *exec.Cmd
	cancel context.CancelFunc
	stdin  io.WriteCloser
	out    chan []byte // fila do writer: Send nunca bloqueia o loop da TUI
	events chan Msg
	done   chan struct{} // fechado quando o processo foi colhido
	stderr *ring

	mu     sync.Mutex
	err    error
	closed bool
}

// Start executa `<bin> serve`, envia init e espera o manifesto. Qualquer falha
// encerra o processo e devolve erro; o chamador decide como exibir.
func (s *Service) Start(pl Plugin, init Msg) (*Proc, error) {
	ctx, cancel := context.WithCancel(s.ctx)
	cmd := exec.CommandContext(ctx, pl.Path, "serve")
	cmd.Env = s.Env()
	cmd.Cancel = func() error {
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
	cmd.WaitDelay = 2 * time.Second // depois disso: SIGKILL e pipes fechados
	p := &Proc{
		Plugin: pl, cmd: cmd, cancel: cancel,
		out: make(chan []byte, 256), events: make(chan Msg, 16),
		done: make(chan struct{}), stderr: &ring{max: 4096},
	}
	p.Events = p.events
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("plugin %s: %w", pl.ID, err)
	}
	p.stdin = stdin
	// io.Pipe em vez de StdoutPipe: o Wait drena a saída pelo copier do
	// exec e o WaitDelay fecha o descritor se um neto segurar o pipe.
	pr, pw := io.Pipe()
	cmd.Stdout, cmd.Stderr = pw, p.stderr
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("plugin %s: %w", pl.ID, err)
	}
	go p.write(ctx)
	go p.read(ctx, pr)
	go func() {
		werr := cmd.Wait()
		// erro antes de fechar o pipe: quem vê Events fechar já encontra Err.
		if werr != nil {
			p.setErr(fmt.Errorf("plugin encerrou: %w", werr))
		} else {
			p.setErr(errors.New("plugin encerrou"))
		}
		cancel()
		_ = pw.Close()
		close(p.done)
	}()

	// fail encerra o processo e devolve o erro com o stderr do plugin, que é
	// a única pista que o autor tem quando o handshake falha.
	fail := func(err error) (*Proc, error) {
		_ = p.Close()
		if tail := strings.TrimSpace(p.StderrTail()); tail != "" {
			return nil, fmt.Errorf("plugin %s: %w\n%s", pl.ID, err, tail)
		}
		return nil, fmt.Errorf("plugin %s: %w", pl.ID, err)
	}
	init.Type, init.Protocol, init.ID = "init", Protocol, pl.ID
	if err := p.Send(init); err != nil {
		return fail(err)
	}
	select {
	case m, ok := <-p.events:
		if !ok {
			return fail(p.Err())
		}
		if m.Type != "manifest" {
			return fail(fmt.Errorf("primeira mensagem precisa ser manifest, veio %q", m.Type))
		}
		p.Manifest = cleanManifest(pl.ID, m)
	case <-time.After(s.Handshake):
		return fail(fmt.Errorf("sem manifest em %s", s.Handshake))
	}
	s.mu.Lock()
	s.procs = append(s.procs, p)
	s.mu.Unlock()
	return p, nil
}

// cleanManifest aplica os limites do protocolo: título curto, nomes de comando
// válidos, textos de ajuda curtos.
func cleanManifest(id string, m Msg) Msg {
	m.Title = strings.TrimSpace(clip(m.Title, 41))
	if m.Title == "" || utf8.RuneCountInString(m.Title) > 40 {
		m.Title = id
	}
	cmds := m.Commands[:0]
	for _, c := range m.Commands {
		if nameRe.MatchString(c.Name) {
			c.Desc = clip(c.Desc, 60)
			cmds = append(cmds, c)
		}
	}
	m.Commands = cmds
	for i, g := range m.Help {
		m.Help[i].Title = clip(g.Title, 40)
		for j, kv := range g.Keys {
			m.Help[i].Keys[j] = [2]string{clip(kv[0], 20), clip(kv[1], 60)}
		}
	}
	return m
}

func clip(s string, n int) string {
	s = ansi.Strip(CleanView(s))
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// Send enfileira m para o stdin do plugin sem bloquear. Fila cheia (plugin
// parou de ler) ou processo fechado = erro e o plugin é encerrado.
func (p *Proc) Send(m Msg) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return errors.New("plugin fechado")
	}
	select {
	case p.out <- append(data, '\n'):
		return nil
	default:
		err := errors.New("plugin não lê stdin")
		p.setErr(err)
		_ = p.Close()
		return err
	}
}

// Close fecha stdin (EOF para o plugin), manda SIGTERM e espera o processo
// (SIGKILL após WaitDelay). Idempotente.
func (p *Proc) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		<-p.done
		return nil
	}
	p.closed = true
	p.mu.Unlock()
	_ = p.stdin.Close()
	p.cancel()
	<-p.done
	return nil
}

// Err é o motivo pelo qual Events fechou (nil enquanto o plugin vive).
func (p *Proc) Err() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

// StderrTail devolve os últimos 4 KiB do stderr do plugin (nunca herdado:
// sujaria a tela alternativa).
func (p *Proc) StderrTail() string { return p.stderr.String() }

func (p *Proc) setErr(err error) {
	p.mu.Lock()
	if p.err == nil {
		p.err = err
	}
	p.mu.Unlock()
}

func (p *Proc) write(ctx context.Context) {
	for {
		select {
		case data := <-p.out:
			if _, err := p.stdin.Write(data); err != nil {
				// EPIPE também acontece quando o plugin sai antes de ler init.
				// Deixe Wait registrar o exit status antes de encerrar o processo.
				if errors.Is(err, syscall.EPIPE) {
					select {
					case <-p.done:
						return
					case <-ctx.Done():
						return
					}
				}
				p.setErr(fmt.Errorf("escrevendo no plugin: %w", err))
				p.cancel()
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (p *Proc) read(ctx context.Context, pr *io.PipeReader) {
	defer close(p.events)
	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 0, 64<<10), MaxLine)
	for sc.Scan() {
		line := sc.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var m Msg
		if err := json.Unmarshal(line, &m); err != nil || m.Type == "" {
			p.setErr(fmt.Errorf("linha inválida do plugin: %.80s", line))
			_ = pr.CloseWithError(io.ErrClosedPipe)
			p.cancel()
			return
		}
		select {
		case p.events <- m:
		case <-ctx.Done():
			_ = pr.CloseWithError(io.ErrClosedPipe)
			return
		}
	}
	if err := sc.Err(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
		if errors.Is(err, bufio.ErrTooLong) {
			err = fmt.Errorf("linha do plugin maior que %d bytes", MaxLine)
		}
		p.setErr(err)
		_ = pr.CloseWithError(io.ErrClosedPipe)
		p.cancel()
	}
}

// ring guarda os últimos max bytes escritos.
type ring struct {
	mu  sync.Mutex
	max int
	buf []byte
}

func (r *ring) Write(b []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf = append(r.buf, b...)
	if len(r.buf) > r.max {
		r.buf = r.buf[len(r.buf)-r.max:]
	}
	return len(b), nil
}

func (r *ring) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return string(r.buf)
}
