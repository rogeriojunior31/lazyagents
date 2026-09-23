package app

import (
	"fmt"
	"slices"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/tui"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
)

// maxSplash é o teto de splashSeconds: a tela inicial é boas-vindas, não espera.
const maxSplash = 10 * time.Second

// layoutConfig é a seção `tui:` do config.yaml: o layout da TUI é do app, não
// de um módulo, e por isso não é chave de core.Config.
type layoutConfig struct {
	Splash        *bool    `yaml:"splash"`        // false pula a tela inicial
	SplashSeconds *float64 `yaml:"splashSeconds"` // 0 também pula; teto maxSplash
	StartTab      string   `yaml:"startTab"`      // id da aba em que o app abre
	Tabs          []string `yaml:"tabs"`          // ordem; as não listadas vêm depois
	Hidden        []string `yaml:"hidden"`        // fora da barra, mas vivas
}

// layout é a seção tui: resolvida contra as abas que existem.
type layout struct {
	order    []string // visíveis, na ordem final
	hidden   []string // ocultas criadas (segundo plano), na ordem padrão
	start    string
	noSplash bool
	splash   time.Duration // 0 = padrão da TUI
	notices  []string
}

// resolve aplica cfg às abas ids (na ordem padrão). skipped são ocultas que a
// feature nem criou (plugins): existem, mas não entram em ids. Nada aqui é
// erro: valor inválido vira aviso e cai no padrão.
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
			warn("hidden: aba desconhecida %q", id)
		case hidden[id]:
			warn("hidden: %q repetida", id)
		default:
			hidden[id] = true
		}
	}

	var order []string
	for _, id := range cfg.Tabs {
		switch {
		case !exists(id):
			warn("tabs: aba desconhecida %q", id)
		case slices.Contains(order, id):
			warn("tabs: %q repetida", id)
		case slices.Contains(ids, id):
			order = append(order, id)
		}
	}
	for _, id := range ids { // as não listadas seguem na ordem padrão
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
		warn("hidden: todas as abas ocultas; mostrando todas")
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
		warn("startTab: %q está oculta; abrindo em %q", id, l.start)
	default:
		warn("startTab: aba desconhecida %q; abrindo em %q", id, l.start)
	}

	l.noSplash = cfg.Splash != nil && !*cfg.Splash
	if s := cfg.SplashSeconds; s != nil {
		d := time.Duration(*s * float64(time.Second))
		switch {
		case *s < 0:
			warn("splashSeconds: %v é negativo; usando o padrão", *s)
		case d > maxSplash:
			warn("splashSeconds: %v passa do teto; usando %v", *s, maxSplash.Seconds())
			l.splash = maxSplash
		case d == 0:
			l.noSplash = true
		default:
			l.splash = d
		}
	}
	return l
}

// Layout instancia as abas e aplica a seção tui: do config.yaml: ordem, abas
// ocultas (em segundo plano), aba inicial e splash. Os avisos vão para
// Deps.Notice. Instancia de novo a cada chamada: chamar uma vez por TUI.
func (a *App) Layout() ([]module.Module, tui.Options) {
	var cfg layoutConfig
	if err := a.Deps.Config.Section("tui", &cfg); err != nil {
		a.Deps.Notice(err.Error() + "; usando o layout padrão")
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
