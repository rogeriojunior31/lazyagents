package app

import (
	"fmt"
	"slices"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/tui"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
)

// maxSplash caps splashSeconds: the splash is a greeting, not a wait.
const maxSplash = 10 * time.Second

// layoutConfig is the `tui:` section of config.yaml. The layout belongs to
// the app, not a module, so it is not a core.Config key.
type layoutConfig struct {
	Splash        *bool    `yaml:"splash"`        // false skips the splash
	SplashSeconds *float64 `yaml:"splashSeconds"` // 0 also skips; capped at maxSplash
	StartTab      string   `yaml:"startTab"`      // tab the app opens on
	Tabs          []string `yaml:"tabs"`          // order; unlisted tabs follow
	Hidden        []string `yaml:"hidden"`        // off the bar but alive
}

// layout is the tui: section resolved against the existing tabs.
type layout struct {
	order    []string // visible, in final order
	hidden   []string // hidden but created (background), default order
	start    string
	noSplash bool
	splash   time.Duration // 0 = TUI default
	notices  []string
}

// resolve applies cfg to tabs ids (default order). skipped are hidden tabs a
// feature never created (plugins): they exist but are not in ids. Nothing is
// an error here: an invalid value becomes a notice and falls back to the default.
func resolve(ids []string, skipped map[string]bool, cfg layoutConfig) layout {
	var l layout
	warn := func(format string, args ...any) {
		l.notices = append(l.notices, "config tui: "+fmt.Sprintf(format, args...))
	}
	exists := func(id string) bool { return slices.Contains(ids, id) || skipped[id] }

	hidden := map[string]bool{}
	for _, id := range cfg.Hidden {
		switch {
		case !exists(id):
			warn("hidden: unknown tab %q", id)
		case hidden[id]:
			warn("hidden: %q repeated", id)
		default:
			hidden[id] = true
		}
	}

	var order []string
	for _, id := range cfg.Tabs {
		switch {
		case !exists(id):
			warn("tabs: unknown tab %q", id)
		case slices.Contains(order, id):
			warn("tabs: %q repeated", id)
		case slices.Contains(ids, id):
			order = append(order, id)
		}
	}
	for _, id := range ids { // unlisted tabs keep the default order
		if !slices.Contains(order, id) {
			order = append(order, id)
		}
	}
	for _, id := range order {
		if hidden[id] {
			l.hidden = append(l.hidden, id)
		} else {
			l.order = append(l.order, id)
		}
	}
	if len(l.order) == 0 && len(ids) > 0 {
		warn("hidden: every tab is hidden; showing all")
		l.order, l.hidden = order, nil
	}

	if len(l.order) > 0 {
		l.start = l.order[0]
	}
	switch id := cfg.StartTab; {
	case id == "":
	case slices.Contains(l.order, id):
		l.start = id
	case exists(id):
		warn("startTab: %q is hidden; opening %q", id, l.start)
	default:
		warn("startTab: unknown tab %q; opening %q", id, l.start)
	}

	l.noSplash = cfg.Splash != nil && !*cfg.Splash
	if s := cfg.SplashSeconds; s != nil {
		d := time.Duration(*s * float64(time.Second))
		switch {
		case *s < 0:
			warn("splashSeconds: %v is negative; using the default", *s)
		case d > maxSplash:
			warn("splashSeconds: %v is above the limit; using %v", *s, maxSplash.Seconds())
			l.splash = maxSplash
		case d == 0:
			l.noSplash = true
		default:
			l.splash = d
		}
	}
	return l
}

// Layout builds the tabs and applies config.yaml tui: (order, hidden tabs,
// start tab, splash); notices go to Deps.Notice. It builds new tabs on every
// call, so call it once per TUI.
func (a *App) Layout() ([]module.Module, tui.Options) {
	var cfg layoutConfig
	if err := a.Deps.Config.Section("tui", &cfg); err != nil {
		a.Deps.Notice(err.Error() + "; using the default layout")
		cfg = layoutConfig{}
	}
	a.Deps.HideTabs(cfg.Hidden...)
	all := a.instantiate()
	byID := make(map[string]module.Module, len(all))
	ids := make([]string, 0, len(all))
	for _, m := range all {
		byID[m.ID()] = m
		ids = append(ids, m.ID())
	}
	l := resolve(ids, a.Deps.SkippedTabs(), cfg)
	a.Deps.Notice(l.notices...)

	opts := tui.Options{NoSplash: l.noSplash, SplashDelay: l.splash}
	mods := make([]module.Module, 0, len(l.order))
	for i, id := range l.order {
		mods = append(mods, byID[id])
		if id == l.start {
			opts.Start = i
		}
	}
	for _, id := range l.hidden {
		opts.Background = append(opts.Background, byID[id])
	}
	return mods, opts
}
