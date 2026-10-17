// Package hooks keeps a library of user hooks and installs them in the agents
// that support it (agent.HooksHost). As with skills, lazyagents manages only
// what is in the library: a hook is identified by (event, matcher, command), so
// no marker is needed in the CLI's file.
package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

// nameRe keeps hook names safe as file names.
const maxNameLen = 40

var nameRe = regexp.MustCompile(`^[\p{L}\p{N}_-]+$`)

// Hook is a library entry: the hook itself plus lazyagents fields (name,
// description, origin) that never reach the agent's file.
type Hook struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Source is where the hook was imported from ("user/repo · plugin"); empty = hand-made.
	Source string `json:"source,omitempty"`
	// Files is the folder with the scripts the commands use, copied on import.
	Files string `json:"files,omitempty"`
	// Hooks are the entry's commands. An imported plugin is one package (e.g.
	// security-guidance has 12 commands in 5 events) that toggles as a whole.
	Hooks []agent.Hook `json:"hooks"`
	// Off holds the indexes in Hooks the user turned off: the package stays whole
	// in the library, installing takes the rest.
	Off []int `json:"off,omitempty"`
}

// IsOff reports whether command i is turned off.
func (h Hook) IsOff(i int) bool {
	for _, o := range h.Off {
		if o == i {
			return true
		}
	}
	return false
}

// Active returns the entry's commands that are on.
func (h Hook) Active() []agent.Hook {
	var out []agent.Hook
	for i, c := range h.Hooks {
		if !h.IsOff(i) {
			out = append(out, c)
		}
	}
	return out
}

// Imported reports whether the hook came from another agent's plugin: those
// target the Claude Code protocol, and other CLIs may send a different stdin payload.
func (h Hook) Imported() bool { return h.Source != "" }

// Events returns the entry's distinct events in order of appearance.
func (h Hook) Events() []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range h.Hooks {
		if !seen[e.Event] {
			seen[e.Event] = true
			out = append(out, e.Event)
		}
	}
	return out
}

// Summary is the list text: the command for a single hook, counts for a package.
func (h Hook) Summary() string {
	if len(h.Hooks) == 1 {
		return displayCommand(h.Hooks[0].Command)
	}
	if len(h.Off) > 0 {
		return fmt.Sprintf("%d of %d commands in %d events", len(h.Active()), len(h.Hooks), len(h.Events()))
	}
	return fmt.Sprintf("%d commands in %d events", len(h.Hooks), len(h.Events()))
}

// Service is the hook library plus installation into agents.
type Service struct {
	adapters   []agent.Adapter
	dir        string
	backupsDir string
	home       string // only to shorten paths on screen
	// Detect returns the detected agents (the memoized feature.Deps.Agents);
	// nil falls back to direct detection.
	Detect func() []agent.Agent
}

func New(adapters []agent.Adapter, paths core.Paths) *Service {
	return &Service{adapters: adapters, dir: paths.HooksDir(), backupsDir: paths.BackupsDir(), home: paths.Home}
}

// Dir is the library folder.
func (s *Service) Dir() string { return s.dir }

// Library reads the library in alphabetical order. An invalid file becomes its
// own error instead of failing the listing.
func (s *Service) Library() ([]Hook, []string) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []string{fmt.Sprintf("reading the library: %v", err)}
	}
	var out []Hook
	var problems []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(s.dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", e.Name(), err))
			continue
		}
		var h Hook
		if err := json.Unmarshal(data, &h); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", e.Name(), err))
			continue
		}
		if h.Name == "" {
			h.Name = strings.TrimSuffix(e.Name(), ".json")
		}
		if h.Name+".json" != e.Name() || !nameRe.MatchString(h.Name) {
			problems = append(problems, fmt.Sprintf("%s: invalid name or different from the file name", e.Name()))
			continue
		}
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, problems
}

// Get finds a library hook by name.
func (s *Service) Get(name string) (Hook, error) {
	lib, _ := s.Library()
	for _, h := range lib {
		if h.Name == name {
			return h, nil
		}
	}
	return Hook{}, fmt.Errorf("hook %q is not in the library", name)
}

// Save creates or replaces a library hook.
func (s *Service) Save(h Hook) error {
	h.Name = strings.TrimSpace(h.Name)
	switch {
	case !nameRe.MatchString(h.Name):
		return fmt.Errorf("hook name: use only letters, digits, - and _")
	case len([]rune(h.Name)) > maxNameLen:
		return fmt.Errorf("hook name: at most %d characters", maxNameLen)
	case len(h.Hooks) == 0:
		return fmt.Errorf("hook %q needs at least one command", h.Name)
	}
	for _, e := range h.Hooks {
		switch {
		case strings.TrimSpace(e.Command) == "":
			return fmt.Errorf("hook %q needs a command", h.Name)
		case strings.TrimSpace(e.Event) == "":
			return fmt.Errorf("hook %q needs an event", h.Name)
		}
	}
	for _, i := range h.Off {
		if i < 0 || i >= len(h.Hooks) {
			return fmt.Errorf("hook %q turns off command %d, which does not exist", h.Name, i)
		}
	}
	h.Off = normOff(h.Off)
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return fmt.Errorf("writing hook %q: %w", h.Name, err)
	}
	return fsutil.WriteAtomic(filepath.Join(s.dir, h.Name+".json"), append(data, '\n'), 0o600)
}

// Delete leaves inline commands installed. Managed scripts require all agents
// to be readable and the entry uninstalled; shared scripts stay in the library.
func (s *Service) Delete(name string) error {
	h, err := s.Get(name)
	if err != nil {
		return err
	}
	shared := false
	var files string
	if h.Files != "" {
		root, err := filepath.Abs(s.dir)
		if err != nil {
			return err
		}
		files, err = filepath.Abs(h.Files)
		if err != nil {
			return err
		}
		if filepath.Dir(files) != root {
			return fmt.Errorf("scripts of %q are outside the hooks library: %s", name, h.Files)
		}

		fileInfo, err := os.Stat(files)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("checking scripts of %s: %w", name, err)
		}
		lib, problems := s.Library()
		if len(problems) > 0 {
			return fmt.Errorf("cannot verify script dependencies: %s", strings.Join(problems, "; "))
		}
		for _, other := range lib {
			if other.Name == name || other.Files == "" {
				continue
			}
			otherFiles, err := filepath.Abs(other.Files)
			if err != nil {
				return fmt.Errorf("checking scripts of %s: %w", other.Name, err)
			}
			if otherFiles == files {
				shared = true
				continue
			}
			otherInfo, err := os.Stat(otherFiles)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("checking scripts of %s: %w", other.Name, err)
			}
			if fileInfo != nil && otherInfo != nil && os.SameFile(fileInfo, otherInfo) {
				shared = true
			}
		}
		var blockers []error
		for _, ad := range s.adapters {
			host, ok := ad.(agent.HooksHost)
			if !ok {
				continue
			}
			installed, err := host.ReadHooks()
			if err != nil {
				blockers = append(blockers, fmt.Errorf("read hooks for %s before deleting scripts: %w", ad.ID(), err))
				continue
			}
			for _, one := range h.Hooks {
				if containsHook(installed, one) {
					blockers = append(blockers, fmt.Errorf("still installed in %s; uninstall first: lazyagents hooks disable %s --agent %s", ad.ID(), name, ad.ID()))
					break
				}
			}
		}
		if err := errors.Join(blockers...); err != nil {
			return fmt.Errorf("cannot delete hook %q: %w", name, err)
		}
	}
	if err := os.Remove(filepath.Join(s.dir, name+".json")); err != nil {
		return err
	}
	if h.Files == "" || shared {
		return nil
	}
	return os.RemoveAll(files)
}

// Status is the state of hooks in one agent.
type Status struct {
	AgentID   string   `json:"agent"`
	AgentName string   `json:"name"`
	Short     string   `json:"-"` // agent column letter in the TUI
	File      string   `json:"file"`
	Installed bool     `json:"installed"`
	Events    []string `json:"events"`
	// Enabled are library entries with ALL supported commands installed.
	Enabled []string `json:"enabled,omitempty"`
	// Partial are half-installed entries (a package whose enable failed midway,
	// or a command removed by hand).
	Partial []string `json:"partial,omitempty"`
	// Foreign counts hooks in the agent that did not come from the library;
	// lazyagents never touches them.
	Foreign int    `json:"foreign"`
	Note    string `json:"note,omitempty"`
	Err     string `json:"error,omitempty"`
}

// Status returns one Status per hook-capable agent, in registry order.
func (s *Service) Status() []Status {
	lib, _ := s.Library()
	agents := s.detectAll()
	var out []Status
	for _, ad := range s.adapters {
		host, ok := ad.(agent.HooksHost)
		if !ok {
			continue
		}
		a := findAgent(agents, ad.ID())
		st := Status{
			AgentID: ad.ID(), AgentName: a.Name, Short: a.Short, File: host.HooksFile(),
			Installed: a.Installed, Events: host.HookEvents(), Note: host.HooksNote(),
		}
		installed, err := host.ReadHooks()
		if err != nil {
			st.Err = err.Error()
			out = append(out, st)
			continue
		}
		for _, ins := range installed {
			if !claimedBy(lib, ins) {
				st.Foreign++ // the user's own hook: never touched
			}
		}
		for _, entry := range lib {
			want := supportedHooks(host, entry)
			if len(want) == 0 {
				continue // the agent fires none of the entry's events
			}
			have := 0
			for _, w := range want {
				if containsHook(installed, w) {
					have++
				}
			}
			switch {
			case have == len(want):
				st.Enabled = append(st.Enabled, entry.Name)
			case have > 0:
				st.Partial = append(st.Partial, entry.Name)
			}
		}
		out = append(out, st)
	}
	return out
}

// claimedBy reports whether an installed hook belongs to a library entry.
func claimedBy(lib []Hook, h agent.Hook) bool {
	for _, entry := range lib {
		if containsHook(entry.Hooks, h) {
			return true
		}
	}
	return false
}

func containsHook(list []agent.Hook, h agent.Hook) bool {
	for _, item := range list {
		if item.Same(h) {
			return true
		}
	}
	return false
}

// normOff sorts and dedupes the off indexes (nil when empty, so the JSON field disappears).
func normOff(off []int) []int {
	sort.Ints(off)
	var out []int
	for i, o := range off {
		if i == 0 || o != off[i-1] {
			out = append(out, o)
		}
	}
	return out
}

// supportedHooks filters the entry's active commands the agent can run. Imported
// packages often have events only one CLI fires; installing what fits beats
// rejecting the whole package.
func supportedHooks(host agent.HooksHost, entry Hook) []agent.Hook {
	var out []agent.Hook
	for _, h := range entry.Active() {
		if supportsEvent(host, h.Event) {
			out = append(out, h)
		}
	}
	return out
}

// Enable installs the entry's active commands whose events the agent fires;
// empty agentID means every installed agent that supports hooks. Commands turned
// off are removed.
func (s *Service) Enable(name, agentID string) error {
	return s.EnableResult(name, core.TargetIDs(agentID)).AsError()
}

func (s *Service) EnableResult(name string, ids []string) core.OperationResult {
	h, err := s.Get(name)
	if err != nil {
		return core.OperationResult{Err: err}
	}
	if len(h.Active()) == 0 {
		return core.OperationResult{Err: fmt.Errorf("every command of %q is turned off", h.Name)}
	}
	return s.each(ids, h, true, func(host agent.HooksHost) error {
		want := supportedHooks(host, h)
		if len(want) == 0 {
			return fmt.Errorf("does not fire any event of %q (%s)", h.Name, strings.Join(h.Events(), ", "))
		}
		for _, one := range want {
			if err := host.AddHook(one, s.backupsDir); err != nil {
				return err
			}
		}
		return s.removeOff(host, h)
	})
}

// removeOff removes the entry's turned-off commands from the agent.
func (s *Service) removeOff(host agent.HooksHost, h Hook) error {
	installed, err := host.ReadHooks()
	if err != nil {
		return err
	}
	for i, one := range h.Hooks {
		if h.IsOff(i) && containsHook(installed, one) {
			if err := host.RemoveHook(one, s.backupsDir); err != nil {
				return err
			}
		}
	}
	return nil
}

// SetCommand turns command i on or off. Agents that already have the entry
// (fully or partly) change right away; elsewhere only the library changes.
func (s *Service) SetCommand(name string, i int, on bool) error {
	return s.SetCommandResult(name, i, on, nil).AsError()
}

// SetCommandResult keeps failed agent IDs usable after the library flag was
// saved: retry must reconcile those files even when the flag already matches.
func (s *Service) SetCommandResult(name string, i int, on bool, ids []string) core.OperationResult {
	if ids != nil && len(ids) == 0 {
		return core.OperationResult{}
	}
	h, err := s.Get(name)
	if err != nil {
		return core.OperationResult{Err: err}
	}
	if i < 0 || i >= len(h.Hooks) {
		return core.OperationResult{Err: fmt.Errorf("hook %q has no command %d", name, i)}
	}
	if on != !h.IsOff(i) {
		h.Off = slices.DeleteFunc(h.Off, func(o int) bool { return o == i })
		if !on {
			h.Off = append(h.Off, i)
		}
		if err := s.Save(h); err != nil {
			return core.OperationResult{Err: err}
		}
	} else if ids == nil {
		return core.OperationResult{}
	}
	result := core.OperationResult{}
	selected := []string{}
	for _, ad := range s.adapters {
		if ids != nil && !slices.Contains(ids, ad.ID()) {
			continue
		}
		t := core.TargetResult{ID: ad.ID(), Outcome: core.Skipped}
		host, ok := ad.(agent.HooksHost)
		if !ok {
			t.Detail = "does not support hooks"
		} else {
			installed, err := host.ReadHooks()
			switch {
			case err != nil:
				t.Outcome, t.Detail = core.Failed, err.Error()
				t.State = "unreadable"
			case on && !supportsEvent(host, h.Hooks[i].Event):
				t.Detail = "does not fire this command's event"
			case ids != nil || slices.ContainsFunc(h.Hooks, func(one agent.Hook) bool { return containsHook(installed, one) }):
				selected = append(selected, ad.ID())
				continue
			default:
				t.Detail = "hook entry not installed; library setting saved"
			}
		}
		result.Targets = append(result.Targets, t)
	}
	for _, id := range ids {
		if !slices.ContainsFunc(s.adapters, func(ad agent.Adapter) bool { return ad.ID() == id }) {
			selected = append(selected, id)
		}
	}
	next := s.each(selected, h, false, func(host agent.HooksHost) error {
		if on {
			return host.AddHook(h.Hooks[i], s.backupsDir)
		}
		return host.RemoveHook(h.Hooks[i], s.backupsDir)
	})
	result.MergeRetry(next)
	return result
}

// Disable uninstalls every command of the entry.
func (s *Service) Disable(name, agentID string) error {
	return s.DisableResult(name, core.TargetIDs(agentID)).AsError()
}

func (s *Service) DisableResult(name string, ids []string) core.OperationResult {
	h, err := s.Get(name)
	if err != nil {
		return core.OperationResult{Err: err}
	}
	return s.each(ids, h, false, func(host agent.HooksHost) error {
		for _, one := range h.Hooks {
			if err := host.RemoveHook(one, s.backupsDir); err != nil {
				return err
			}
		}
		return nil
	})
}

// each records every adapter, including targets skipped by a bulk selection.
func (s *Service) each(ids []string, entry Hook, checkEvents bool, fn func(agent.HooksHost) error) core.OperationResult {
	var result core.OperationResult
	var agents []agent.Agent
	if ids == nil {
		agents = s.detectAll()
	}
	attempted := false
	seen := map[string]bool{}
	for _, ad := range s.adapters {
		id := ad.ID()
		if ids != nil && !slices.Contains(ids, id) {
			continue
		}
		seen[id] = true
		t := core.TargetResult{ID: id, Outcome: core.Skipped}
		host, ok := ad.(agent.HooksHost)
		switch {
		case !ok:
			t.Detail = "does not support hooks"
		case ids == nil && !findAgent(agents, id).Installed:
			t.Detail = "not installed"
		case checkEvents && len(entry.Hooks) > 0 && len(supportedHooks(host, entry)) == 0:
			t.Detail = fmt.Sprintf("does not fire any event of %q (%s)", entry.Name, strings.Join(entry.Events(), ", "))
		default:
			attempted = true
			t.Outcome = core.Succeeded
			if err := fn(host); err != nil {
				t.Outcome, t.Detail = core.Failed, err.Error()
			}
			hooks, err := host.ReadHooks()
			if err != nil {
				t.Outcome = core.Failed
				t.Detail = strings.TrimSpace(t.Detail + " verification: " + err.Error())
				t.State = "unreadable"
			} else {
				count := 0
				for _, one := range entry.Hooks {
					if containsHook(hooks, one) {
						count++
					}
				}
				t.State = fmt.Sprintf("%d/%d command(s) of %q installed", count, len(entry.Hooks), entry.Name)
			}
			if note := host.HooksNote(); note != "" {
				t.State += "; " + note
			}
		}
		if ids != nil && t.Outcome == core.Skipped {
			t.Outcome = core.Failed
		}
		result.Targets = append(result.Targets, t)
	}
	for _, id := range ids {
		if !seen[id] {
			result.Targets = append(result.Targets, core.TargetResult{ID: id, Outcome: core.Failed, Detail: fmt.Sprintf("agent %q does not exist", id)})
		}
	}
	if !attempted && ids == nil {
		result.Err = fmt.Errorf("no installed agent supports this hook")
	}
	return result
}

func supportsEvent(host agent.HooksHost, event string) bool {
	for _, e := range host.HookEvents() {
		if strings.EqualFold(e, event) {
			return true
		}
	}
	return false
}

// CommandProblem warns when a command's executable is not in PATH (the hook
// would fail silently when the event fires).
func CommandProblem(h Hook) string {
	for _, one := range h.Hooks {
		if p := commandProblem(one.Command); p != "" {
			return p
		}
	}
	return ""
}

func commandProblem(command string) string {
	fields := strings.Fields(stripRootExport(command)) // the executable comes after the export
	if len(fields) == 0 {
		return "empty command"
	}
	bin := fields[0]
	if strings.ContainsAny(bin, "/\\") {
		if info, err := os.Stat(bin); err != nil {
			return bin + ": not found"
		} else if info.Mode()&0o111 == 0 && runtime.GOOS != "windows" { // Windows has no exec bit
			return bin + ": not executable"
		}
		return ""
	}
	if _, err := exec.LookPath(bin); err != nil {
		return bin + ": not in PATH"
	}
	return ""
}

func (s *Service) detectAll() []agent.Agent {
	if s.Detect != nil {
		return s.Detect()
	}
	return agent.DetectAll(s.adapters)
}

func findAgent(agents []agent.Agent, id string) agent.Agent {
	for _, a := range agents {
		if a.ID == id {
			return a
		}
	}
	return agent.Agent{ID: id}
}
