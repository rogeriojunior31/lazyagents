package docs

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/app"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/tui"
	"github.com/rogeriojunior31/lazyagents/internal/tui/module"
	"github.com/rogeriojunior31/lazyagents/internal/tui/theme"
)

var update = flag.Bool("update", false, "rewrite docs/reference from the code")

const generated = "<!-- Generated from the code by `go test ./docs -update`. Do not edit by hand. -->\n\n"

// loadApp boots lazyagents in an empty home with no agent binaries in PATH,
// so the output never depends on the machine running the test.
func loadApp(t *testing.T) (*app.App, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("PATH", "")
	t.Setenv("PI_CODING_AGENT_DIR", "")
	// Hermes and Pi only announce their skills dir once their config dir exists.
	for _, dir := range []string{".hermes", ".pi/agent"} {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	a, err := app.LoadWith(core.PathsIn(home), "docs")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	return a, home
}

// The reference pages are the code rendered as Markdown; a stale page fails
// here, so a new command, key, agent or theme cannot ship undocumented.
func TestReferenceIsCurrent(t *testing.T) {
	a, home := loadApp(t)
	pages := map[string]string{
		"cli.md":    cliReference(a),
		"keys.md":   keysReference(a),
		"agents.md": agentsReference(a.Deps.Adapters, home),
		"themes.md": themesReference(),
	}
	for name, want := range pages {
		path := filepath.Join("reference", name)
		if *update {
			if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Errorf("docs/%s is out of date: run `go test ./docs -update` and commit the result", path)
		}
	}
}

// Every command explains itself in `lazyagents help <command>`; that text is
// also the CLI reference.
func TestCommandsHaveHelp(t *testing.T) {
	a, _ := loadApp(t)
	for _, c := range a.Commands() {
		if c.Summary == "" || c.Help == "" {
			t.Errorf("command %q needs Summary and Help (they become docs/reference/cli.md)", c.Name)
		}
	}
}

// Each module has a guide linked from the docs index.
func TestEveryModuleHasGuide(t *testing.T) {
	a, _ := loadApp(t)
	index, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range a.FeatureNames() {
		guide := "guide/" + name + ".md"
		if _, err := os.Stat(guide); err != nil {
			t.Errorf("module %q has no guide: write docs/%s", name, guide)
		}
		if !strings.Contains(string(index), "("+guide+")") {
			t.Errorf("docs/README.md does not link %s", guide)
		}
	}
}

// Relative links and #anchors in every Markdown file of the repository point
// to something that exists, so renames and removed headings are caught.
func TestLinksResolve(t *testing.T) {
	root := ".."
	anchors := map[string]map[string]bool{}
	headingsOf := func(path string) map[string]bool {
		if h, ok := anchors[path]; ok {
			return h
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		anchors[path] = slugs(string(b))
		return anchors[path]
	}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != root && (strings.HasPrefix(d.Name(), ".") && d.Name() != ".github" || d.Name() == "dist") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, target := range links(string(b)) {
			if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			file, anchor, _ := strings.Cut(target, "#")
			dest := path
			if file != "" {
				dest = filepath.Join(filepath.Dir(path), file)
				if _, err := os.Stat(dest); err != nil {
					t.Errorf("%s: broken link %q", path, target)
					continue
				}
			}
			if anchor != "" && strings.HasSuffix(dest, ".md") && !headingsOf(dest)[anchor] {
				t.Errorf("%s: link %q has no matching heading", path, target)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func cliReference(a *app.App) string {
	var b strings.Builder
	b.WriteString(generated)
	b.WriteString("# CLI reference\n\n")
	b.WriteString("Every command also prints its own page with `lazyagents help <command>`. Without arguments, `lazyagents` opens the TUI. ")
	b.WriteString("Each external plugin adds one more command named after its id, described by `lazyagents help` once the plugin is installed ([plugins guide](../guide/plugins.md)).\n\n")
	b.WriteString("Commands exit with `0` on success and non-zero on failure.\n\n")
	b.WriteString("| Command | What it does |\n|---|---|\n")
	cmds := a.Commands()
	for _, c := range cmds {
		fmt.Fprintf(&b, "| [`%s`](#%s) | %s |\n", c.Name, slug(c.Name), cell(c.Summary))
	}
	for _, c := range cmds {
		fmt.Fprintf(&b, "\n## %s\n\n```text\nlazyagents %s\n```\n\n%s\n", c.Name, c.Usage, sentence(c.Summary))
		if c.Help != "" {
			fmt.Fprintf(&b, "\n```text\n%s\n```\n", strings.TrimRight(c.Help, "\n"))
		}
	}
	return b.String()
}

func keysReference(a *app.App) string {
	var b strings.Builder
	b.WriteString(generated)
	b.WriteString("# Key reference\n\n")
	b.WriteString("The same lists show inside the TUI: `?` opens the help of the current tab. `:` opens the command palette, which runs any entry below by typing part of its name.\n\n")
	b.WriteString("While a text field, filter or confirmation is open it owns the keyboard: `esc` closes it, `enter` confirms.\n")
	helpGroup(&b, "##", tui.Navigation)
	mods := a.Modules()
	for _, m := range mods {
		fmt.Fprintf(&b, "\n## %s tab\n", m.Title())
		for _, g := range m.Help() {
			helpGroup(&b, "###", g)
		}
	}
	b.WriteString("\n## Command palette\n\nType `:` and part of a name. Tab entries run inside their tab.\n\n| Entry | What it does |\n|---|---|\n")
	for _, c := range tui.PaletteCommands(mods) {
		fmt.Fprintf(&b, "| `%s` | %s |\n", c.Name, cell(c.Desc))
	}
	return b.String()
}

func helpGroup(b *strings.Builder, level string, g module.HelpGroup) {
	fmt.Fprintf(b, "\n%s %s\n\n| Key | Action |\n|---|---|\n", level, g.Title)
	for _, kv := range g.Keys {
		fmt.Fprintf(b, "| `%s` | %s |\n", kv[0], cell(kv[1]))
	}
}

func agentsReference(adapters []agent.Adapter, home string) string {
	tilde := func(p string) string { return "`" + filepath.ToSlash(core.Tilde(p, home)) + "`" }
	yes := func(ok bool) string {
		if ok {
			return "yes"
		}
		return "—"
	}
	var b strings.Builder
	b.WriteString(generated)
	b.WriteString("# Agent support\n\n")
	b.WriteString("What lazyagents can do with each agent, read from the adapters in `internal/agent`. Paths are the defaults on Linux and macOS. ")
	b.WriteString("How sessions are read per agent is in the [sessions guide](../guide/sessions.md#where-sessions-come-from).\n\n")
	b.WriteString("## Skills\n\nA directory marked (shared) is read by several agents: lazyagents links a skill there only while it is enabled for every installed agent that reads it, and otherwise in each agent's own directory.\n\n| Agent | id | Enables skills in | Also reads |\n|---|---|---|---|\n")
	var hosts []agent.HooksHost
	var hostNames []string
	for _, ad := range adapters {
		a := ad.Detect()
		managed, reads := "— (not manageable locally)", "—"
		if a.ManagedDir != "" {
			managed = tilde(a.ManagedDir)
		}
		var extra []string
		for _, d := range a.ReadDirs {
			switch d {
			case a.ManagedDir:
			case a.SharedDir:
				extra = append(extra, tilde(d)+" (shared)")
			default:
				extra = append(extra, tilde(d))
			}
		}
		if len(extra) > 0 {
			reads = strings.Join(extra, ", ")
		}
		fmt.Fprintf(&b, "| %s | `%s` | %s | %s |\n", a.Name, a.ID, managed, reads)
		if h, ok := ad.(agent.HooksHost); ok {
			hosts, hostNames = append(hosts, h), append(hostNames, a.Name)
		}
	}
	b.WriteString("\n## Capabilities\n\n| Agent | Providers (writes) | Hooks (writes) | Subscription limits | Usage history | Live-session badge |\n|---|---|---|---|---|---|\n")
	for _, ad := range adapters {
		name := ad.Detect().Name
		prov, hooks := "—", "—"
		if p, ok := ad.(agent.ProviderHost); ok {
			prov = tilde(p.ProviderFile())
		}
		if h, ok := ad.(agent.HooksHost); ok {
			hooks = tilde(h.HooksFile())
		}
		_, limits := ad.(agent.RateLimitReader)
		_, events := ad.(agent.UsageEventReader)
		_, live := ad.(agent.LiveChecker)
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n", name, prov, hooks, yes(limits), yes(events), yes(live))
	}
	b.WriteString("\n## Hook events\n\nA hook only installs in agents that fire its event; the TUI marks the others with `–`.\n\n")
	b.WriteString("| Event | " + strings.Join(hostNames, " | ") + " |\n|---|" + strings.Repeat("---|", len(hosts)) + "\n")
	var events []string
	for _, h := range hosts {
		for _, e := range h.HookEvents() {
			if !slices.Contains(events, e) {
				events = append(events, e)
			}
		}
	}
	for _, e := range events {
		row := "| `" + e + "` |"
		for _, h := range hosts {
			row += " " + yes(slices.Contains(h.HookEvents(), e)) + " |"
		}
		b.WriteString(row + "\n")
	}
	return b.String()
}

func themesReference() string {
	var b strings.Builder
	b.WriteString(generated)
	b.WriteString("# Built-in themes\n\nSet one with `theme: <id>` in `config.yaml`. Writing your own is covered in the [themes guide](../themes.md).\n\n")
	b.WriteString("| id | Name | Appearance | Description |\n|---|---|---|---|\n")
	for _, p := range theme.Options() {
		if !p.User {
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", p.ID, p.Label, p.Appearance, cell(p.Description))
		}
	}
	return b.String()
}

// cell escapes text for a Markdown table cell.
func cell(s string) string { return strings.ReplaceAll(s, "|", `\|`) }

// sentence capitalizes s and ends it with a period.
func sentence(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return strings.TrimSuffix(string(r), ".") + "."
}

var (
	fence    = regexp.MustCompile("(?ms)^```.*?^```")
	code     = regexp.MustCompile("`[^`\n]*`")
	mdLink   = regexp.MustCompile(`\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	htmlLink = regexp.MustCompile(`(?:src|href)="([^"]+)"`)
	heading  = regexp.MustCompile(`(?m)^#{1,6}\s+(.+?)\s*#*\s*$`)
)

// links returns the link targets of a Markdown text, ignoring code.
func links(md string) []string {
	md = code.ReplaceAllString(fence.ReplaceAllString(md, ""), "")
	var out []string
	for _, re := range []*regexp.Regexp{mdLink, htmlLink} {
		for _, m := range re.FindAllStringSubmatch(md, -1) {
			out = append(out, m[1])
		}
	}
	return out
}

// slugs returns the anchors GitHub generates for the headings of md.
func slugs(md string) map[string]bool {
	out := map[string]bool{}
	seen := map[string]int{}
	for _, m := range heading.FindAllStringSubmatch(fence.ReplaceAllString(md, ""), -1) {
		s := slug(m[1])
		if n := seen[s]; n > 0 {
			out[fmt.Sprintf("%s-%d", s, n)] = true
		} else {
			out[s] = true
		}
		seen[s]++
	}
	return out
}

// slug mimics GitHub: lowercase, drop punctuation, spaces become hyphens.
func slug(text string) string {
	text = mdLink.ReplaceAllString(text, "]")
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	return b.String()
}
