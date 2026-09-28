package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

func ptr[T any](v T) *T { return &v }

func TestResolveLayout(t *testing.T) {
	ids := []string{"skills", "sessions", "agents", "usage"}
	cases := []struct {
		name     string
		cfg      layoutConfig
		order    []string
		hidden   []string
		start    string
		noSplash bool
		splash   time.Duration
		notices  int
	}{
		{name: "default", order: ids, start: "skills"},
		{name: "partial order moves to the front",
			cfg:   layoutConfig{Tabs: []string{"usage", "sessions"}},
			order: []string{"usage", "sessions", "skills", "agents"}, start: "usage"},
		{name: "unknown and repeated become notices",
			cfg:   layoutConfig{Tabs: []string{"xyz", "agents", "agents"}, Hidden: []string{"nope"}},
			order: []string{"agents", "skills", "sessions", "usage"}, start: "agents", notices: 3},
		{name: "hidden goes to the background",
			cfg:   layoutConfig{Hidden: []string{"sessions"}, StartTab: "usage"},
			order: []string{"skills", "agents", "usage"}, hidden: []string{"sessions"}, start: "usage"},
		{name: "hidden startTab falls back to the first",
			cfg:   layoutConfig{Hidden: []string{"skills"}, StartTab: "skills"},
			order: []string{"sessions", "agents", "usage"}, hidden: []string{"skills"}, start: "sessions", notices: 1},
		{name: "unknown startTab",
			cfg:   layoutConfig{StartTab: "xyz"},
			order: ids, start: "skills", notices: 1},
		{name: "all hidden shows all",
			cfg:   layoutConfig{Hidden: ids},
			order: ids, start: "skills", notices: 1},
		{name: "splash off",
			cfg:   layoutConfig{Splash: ptr(false)},
			order: ids, start: "skills", noSplash: true},
		{name: "splashSeconds 0 skips",
			cfg:   layoutConfig{SplashSeconds: ptr(0.0)},
			order: ids, start: "skills", noSplash: true},
		{name: "fractional splashSeconds",
			cfg:   layoutConfig{SplashSeconds: ptr(0.5)},
			order: ids, start: "skills", splash: 500 * time.Millisecond},
		{name: "negative splashSeconds",
			cfg:   layoutConfig{SplashSeconds: ptr(-1.0)},
			order: ids, start: "skills", notices: 1},
		{name: "splashSeconds above the cap",
			cfg:   layoutConfig{SplashSeconds: ptr(60.0)},
			order: ids, start: "skills", splash: maxSplash, notices: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := resolve(ids, nil, tc.cfg)
			if !slices.Equal(l.order, tc.order) || !slices.Equal(l.hidden, tc.hidden) || l.start != tc.start {
				t.Errorf("order %v hidden %v start %q; want %v %v %q", l.order, l.hidden, l.start, tc.order, tc.hidden, tc.start)
			}
			if l.noSplash != tc.noSplash || l.splash != tc.splash {
				t.Errorf("splash = %v %v; want %v %v", l.noSplash, l.splash, tc.noSplash, tc.splash)
			}
			if len(l.notices) != tc.notices {
				t.Errorf("notices = %q; want %d", l.notices, tc.notices)
			}
		})
	}
	// a hidden tab the feature never created (plugin) exists: no unknown-tab notice
	if l := resolve(ids, map[string]bool{"zz": true}, layoutConfig{Hidden: []string{"zz"}}); len(l.notices) != 0 {
		t.Errorf("hidden plugin became a notice: %q", l.notices)
	}
}

// Real config.yaml: order, hidden tab in the background, start tab, and a
// hidden plugin is not a tab (no process) but keeps its command.
func TestLayoutFromConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake plugin is a POSIX shell script")
	}
	p := core.PathsIn(t.TempDir())
	hasSh := false
	if _, err := exec.LookPath("sh"); err == nil {
		hasSh = true
		if err := os.MkdirAll(p.PluginsDir(), 0o755); err != nil {
			t.Fatal(err)
		}
		script := "#!/bin/sh\ncase \"$1\" in serve) read i; echo '{\"type\":\"manifest\",\"title\":\"Hi\"}'; cat >/dev/null;; *) exit 0;; esac\n"
		if err := os.WriteFile(filepath.Join(p.PluginsDir(), "zz"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cfg := "tui:\n  splash: false\n  startTab: usage\n  tabs: [usage, sessions]\n  hidden: [hooks, zz, xyz]\n"
	if err := os.MkdirAll(filepath.Dir(p.ConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.ConfigPath(), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := LoadWith(p, "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	mods, opts := d.Layout()
	var ids []string
	for _, m := range mods {
		ids = append(ids, m.ID())
	}
	if len(ids) < 2 || ids[0] != "usage" || ids[1] != "sessions" || slices.Contains(ids, "hooks") || slices.Contains(ids, "zz") {
		t.Fatalf("tabs = %v", ids)
	}
	if !opts.NoSplash || opts.Start != 0 || len(opts.Background) != 1 || opts.Background[0].ID() != "hooks" {
		t.Errorf("options = %+v", opts)
	}
	var unknown []string
	for _, n := range d.Deps.Notices() {
		if strings.Contains(n, "unknown tab") {
			unknown = append(unknown, n)
		}
	}
	if len(unknown) != 1 || !strings.Contains(unknown[0], `"xyz"`) {
		t.Errorf("unknown-tab notices = %q", unknown)
	}
	if hasSh {
		if code := cli.Run([]string{"zz"}, cli.Context{Out: os.Stderr, Err: os.Stderr}, d.Commands()); code != 0 {
			t.Errorf("hidden plugin command = %d", code)
		}
	}
}
