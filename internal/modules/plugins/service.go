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

// Plugin is a binary found in <ConfigDir>/plugins; ID is the file name without
// extension and names the tab, the CLI subcommand and the config section.
type Plugin struct {
	ID   string
	Path string
}

// Service discovers and runs plugins, tracking live processes for Close on TUI exit.
type Service struct {
	Dir       string        // <ConfigDir>/plugins
	Handshake time.Duration // max wait for the manifest (3 s; tests shorten it)

	ctx    context.Context
	cancel context.CancelFunc
	paths  core.Paths
	mu     sync.Mutex
	procs  []*Proc
}

// New builds the service on the app paths.
func New(p core.Paths) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{Dir: p.PluginsDir(), Handshake: 3 * time.Second, paths: p, ctx: ctx, cancel: cancel}
}

// List returns valid plugins alphabetically and one warning per skipped entry
// (not executable, invalid name, duplicate id). A missing dir means none.
func (s *Service) List() (pls []Plugin, warnings []string) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []string{fmt.Sprintf("reading plugins: %v", err)}
	}
	seen := map[string]bool{}
	for _, e := range entries {
		path := filepath.Join(s.Dir, e.Name())
		st, err := os.Stat(path) // follows symlinks
		if err != nil || !st.Mode().IsRegular() {
			continue
		}
		id := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		switch {
		case st.Mode()&0o111 == 0 && !(runtime.GOOS == "windows" && strings.EqualFold(filepath.Ext(path), ".exe")):
			warnings = append(warnings, fmt.Sprintf("plugin %s ignored: not executable", e.Name()))
		case !idRe.MatchString(id):
			warnings = append(warnings, fmt.Sprintf("plugin %s ignored: name must match %s", e.Name(), idRe))
		case seen[id]:
			warnings = append(warnings, fmt.Sprintf("plugin %s ignored: duplicate id %q", e.Name(), id))
		default:
			seen[id] = true
			pls = append(pls, Plugin{ID: id, Path: path})
		}
	}
	return pls, warnings
}

// Env is the environment given to every plugin: the host's plus the app paths.
func (s *Service) Env() []string {
	return append(os.Environ(),
		"LAZYAGENTS_HOME="+s.paths.Home,
		"LAZYAGENTS_CONFIG_DIR="+s.paths.ConfigDir,
		"LAZYAGENTS_DATA_DIR="+s.paths.DataDir,
		"LAZYAGENTS_LIBRARY_DIR="+s.paths.LibraryDir(),
		fmt.Sprintf("LAZYAGENTS_PROTOCOL=%d", Protocol),
	)
}

// Run executes `<bin> args…` with inherited stdio (CLI pass-through and doctor)
// and returns the exit code; a start failure is 1 with the cause in errw.
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

// Close stops every process started by Start.
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

// Proc is a plugin in `serve` mode. Events delivers its messages after the
// manifest and closes when the process ends or breaks the protocol; Err says why.
type Proc struct {
	Plugin   Plugin
	Manifest Msg
	Events   <-chan Msg

	cmd    *exec.Cmd
	cancel context.CancelFunc
	stdin  io.WriteCloser
	out    chan []byte // writer queue: Send never blocks the TUI loop
	events chan Msg
	done   chan struct{} // closed once the process is reaped
	stderr *ring

	mu     sync.Mutex
	err    error
	closed bool
}

// Start runs `<bin> serve`, sends init and waits for the manifest. Any failure
// stops the process and returns an error; the caller decides how to show it.
func (s *Service) Start(pl Plugin, init Msg) (*Proc, error) {
	ctx, cancel := context.WithCancel(s.ctx)
	cmd := exec.CommandContext(ctx, pl.Path, "serve")
	cmd.Env = s.Env()
	ownGroup(cmd)                   // stopping the plugin stops what it started
	cmd.WaitDelay = 2 * time.Second // after this: SIGKILL and closed pipes
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
	// io.Pipe instead of StdoutPipe: Wait drains output through exec's copier and
	// WaitDelay closes the fd if a grandchild holds the pipe.
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
		reapGroup(cmd)
		// set the error before closing the pipe: whoever sees Events close finds Err.
		if werr != nil {
			p.setErr(fmt.Errorf("plugin exited: %w", werr))
		} else {
			p.setErr(errors.New("plugin exited"))
		}
		cancel()
		_ = pw.Close()
		close(p.done)
	}()

	// fail stops the process and returns the error with the plugin's stderr, the
	// author's only clue when the handshake fails.
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
			return fail(fmt.Errorf("first message must be manifest, got %q", m.Type))
		}
		p.Manifest = cleanManifest(pl.ID, m)
	case <-time.After(s.Handshake):
		return fail(fmt.Errorf("no manifest within %s", s.Handshake))
	}
	s.mu.Lock()
	s.procs = append(s.procs, p)
	s.mu.Unlock()
	return p, nil
}

// cleanManifest applies the protocol limits: short title, valid command names,
// short help texts.
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

// Send queues m for the plugin's stdin without blocking. A full queue (plugin
// stopped reading) or a closed process is an error and stops the plugin.
func (p *Proc) Send(m Msg) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return errors.New("plugin closed")
	}
	select {
	case p.out <- append(data, '\n'):
		return nil
	default:
		err := errors.New("plugin does not read stdin")
		p.setErr(err)
		_ = p.Close()
		return err
	}
}

// Close closes stdin (EOF to the plugin), sends SIGTERM and waits (SIGKILL after
// WaitDelay). Idempotent.
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

// Err is why Events closed (nil while the plugin lives).
func (p *Proc) Err() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

// StderrTail returns the last 4 KiB of the plugin's stderr (never inherited: it
// would dirty the alt screen).
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
				// EPIPE also happens when the plugin exits before reading init;
				// let Wait record the exit status before stopping the process.
				if errors.Is(err, syscall.EPIPE) {
					select {
					case <-p.done:
						return
					case <-ctx.Done():
						return
					}
				}
				p.setErr(fmt.Errorf("writing to plugin: %w", err))
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
			p.setErr(fmt.Errorf("invalid line from plugin: %.80s", line))
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
			err = fmt.Errorf("plugin line longer than %d bytes", MaxLine)
		}
		p.setErr(err)
		_ = pr.CloseWithError(io.ErrClosedPipe)
		p.cancel()
	}
}

// ring keeps the last max bytes written.
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
