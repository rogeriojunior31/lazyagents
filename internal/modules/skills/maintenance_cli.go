package skills

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"text/tabwriter"

	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

func maintenanceCommands(svc *Service) []cli.Command {
	return []cli.Command{
		{Name: "sources", Usage: "sources list|scan <name> [--json]", Summary: "list saved skill repositories or scan one on request", Help: `subcommands:
  list            list configured names, URLs, refs and subdirectories (default; offline)
  scan <name>     clone just this saved source and list repository-relative entries

options:
  --json          stable JSON; scans include source, revision, entries and notes

Scanning uses the saved branch/tag and subdirectory. It never installs,
activates or edits config. Duplicate destination names retain distinct paths.
Use the TUI source picker to install a saved source with exact-path provenance.
Interrupting a scan stops Git and its children and cleans staging.`,
			Run: func(c cli.Context, args []string) int { return cmdSources(c, args, svc) }},
		{Name: "updates", Usage: "updates check [directory...]|apply <directory...> [--review token] [--replace-local] [--json]", Summary: "review and apply Git skill updates with stale-review protection", Help: `subcommands:
  check [directory...]   review exact library folders; omitted selection checks all Git skills
  apply <directory...>   apply only these exact reviewed folders

options:
  --json                 check: review/items with stable status strings, origin, diff and hashes
                         apply: targets with id/outcome/detail/state and optional error
  --review token         apply: required identity from check for this exact selection
  --replace-local        apply: allow overwriting edits/missing baseline for exactly one folder

Selectors are canonical folder names from skills list, never display-name aliases.
Unknown/duplicate selections and extra arguments are refused before writes.
Check includes one token for the full scope and one per checked item, so a
script can select a single item from a full check. Apply stages and verifies
again; changed local files, origins or upstream commits require another check.
The same review/diff, backup, exact-path and replacement services serve the TUI.
Bulk updates skip local edits and missing baselines. --replace-local never
bypasses a stale review. Interruptions preserve completed items and their
backups; later items stay unchanged. A failure requires a new check.
Neither command changes activation links or install destination preferences.`,
			Run: func(c cli.Context, args []string) int { return cmdUpdates(c, args, svc) }},
		{Name: "profiles", Usage: "profiles list|show <name>|save <name>|preview <name>|apply <name>|remove <name> [--review token] [--json]", Summary: "save, inspect and apply skill activation profiles", Help: `subcommands:
  list             list profile names (default; offline)
  show <name>      inspect the saved canonical folder-to-agent specification
  save <name>      save/overwrite the current managed activation matrix
  preview <name>   compare the saved profile with the current matrix and destinations
  apply <name>     apply the reviewed profile to the full matrix
  remove <name>    remove just the saved profile; activation is unchanged

options:
  --json           stable JSON: names, profile specification, preview or operation result
  --review token   apply: required identity from preview; changed state is refused

Save uses the same managed-link snapshot as the TUI; local/foreign and shared
visibility that lazyagents cannot control are excluded. Apply has the same
full-inventory/agent scope as TUI profiles, including disabling managed skills
omitted from the saved profile. Preview lists canonical folders, display names,
agent additions/removals and the saved specification, including unavailable IDs.
Shared-reader constraints still apply; local/foreign copies are preserved.
Legacy profile arrays remain readable. Existing JSON metadata survives writes.
Removing a missing name is idempotent; neither save nor remove touches links.`,
			Run: func(c cli.Context, args []string) int { return cmdProfiles(c, args, svc) }},
	}
}

func maintenanceError(c cli.Context, err error) int {
	fmt.Fprintln(c.Err, "lazyagents:", reviewText(err.Error()))
	return 1
}
func maintenanceJSON(c cli.Context, value any) int {
	enc := json.NewEncoder(c.Out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return maintenanceError(c, err)
	}
	return 0
}
func maintenanceSub(args []string, defaultSub string) (string, []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return args[0], args[1:]
	}
	return defaultSub, args
}
func validReview(token string) bool {
	data, err := hex.DecodeString(token)
	return err == nil && len(data) == 32
}

// Source results never expose temporary absolute paths that die with the scan.
type jsonSourceEntry struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Kind        string `json:"kind"`
	Valid       bool   `json:"valid"`
	Collision   bool   `json:"collision"`
}
type jsonSourceScan struct {
	Source   repository        `json:"source"`
	Revision string            `json:"revision"`
	Entries  []jsonSourceEntry `json:"entries"`
	Notes    []string          `json:"notes"`
}

func cmdSources(c cli.Context, args []string, svc *Service) int {
	sub, args := maintenanceSub(args, "list")
	if sub != "list" && sub != "scan" {
		return maintenanceError(c, fmt.Errorf("unknown sources subcommand %q (list, scan)", sub))
	}
	fs := cli.Flags("skills sources "+sub, c.Err)
	jsonOut := fs.Bool("json", false, "JSON output")
	pos, ok := positionals(fs, args)
	if !ok || (sub == "list" && len(pos) != 0) || (sub == "scan" && len(pos) != 1) {
		return maintenanceError(c, fmt.Errorf("usage: lazyagents skills sources list|scan <name> [--json]"))
	}
	sources, err := svc.repositories()
	if err != nil {
		return maintenanceError(c, err)
	}
	if sub == "list" {
		if sources == nil {
			sources = []repository{}
		}
		if *jsonOut {
			return maintenanceJSON(c, sources)
		}
		tw := tabwriter.NewWriter(c.Out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "SOURCE\tURL\tREF\tSUBDIRECTORY")
		for _, r := range sources {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", reviewText(r.Name), reviewText(r.URL), reviewText(r.Ref), reviewText(r.Subdir))
		}
		if err := tw.Flush(); err != nil {
			return maintenanceError(c, err)
		}
		return 0
	}
	index := slices.IndexFunc(sources, func(r repository) bool { return strings.EqualFold(r.Name, pos[0]) })
	if index < 0 {
		return maintenanceError(c, fmt.Errorf("source %q not found; list saved sources first", pos[0]))
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	found, origin, cleanup, err := svc.scanRepositoryContext(ctx, sources[index])
	if cleanup != "" {
		defer os.RemoveAll(cleanup)
	}
	if err != nil {
		return maintenanceError(c, err)
	}
	out := jsonSourceScan{Source: sources[index], Revision: origin.Revision, Entries: []jsonSourceEntry{}, Notes: append([]string{}, origin.Notes...)}
	for _, f := range found {
		kind := "skill"
		if f.Hook != nil {
			kind = "hook"
		}
		out.Entries = append(out.Entries, jsonSourceEntry{f.Rel, f.Name, f.Description, kind, f.Valid, f.Collision})
	}
	if *jsonOut {
		return maintenanceJSON(c, out)
	}
	fmt.Fprintf(c.Out, "Source: %s\nReviewed commit: %s\n", reviewText(out.Source.Name), out.Revision)
	for _, e := range out.Entries {
		fmt.Fprintf(c.Out, "%s: %s (%s; valid=%t, collision=%t)\n", e.Kind, reviewText(e.Path), reviewText(e.Name), e.Valid, e.Collision)
	}
	for _, n := range out.Notes {
		fmt.Fprintln(c.Out, reviewText(n))
	}
	return 0
}

type jsonUpdateItem struct {
	ExactPath       bool        `json:"exact_path"`
	Dir             string      `json:"dir"`
	Status          string      `json:"status"`
	Origin          *jsonOrigin `json:"origin"`
	Revision        string      `json:"revision"`
	LocalHash       string      `json:"local_hash"`
	RemoteHash      string      `json:"remote_hash"`
	BaselineKnown   bool        `json:"baseline_known"`
	LocalChanged    *bool       `json:"local_changed"`
	UpstreamChanged *bool       `json:"upstream_changed"`
	Files           []string    `json:"files"`
	Diff            string      `json:"diff"`
	Review          string      `json:"review"`
	Error           string      `json:"error,omitempty"`
}
type jsonUpdateReview struct {
	Review string           `json:"review"`
	Items  []jsonUpdateItem `json:"items"`
}

func jsonUpdate(svc *Service, c UpdateCheck) jsonUpdateItem {
	out := jsonUpdateItem{Dir: c.Skill.Dir, Status: "unknown", Origin: toJSONSkill(c.Skill).Origin, Revision: c.Revision, LocalHash: c.LocalHash, RemoteHash: c.RemoteHash, Files: append([]string{}, c.Files...), Diff: c.Diff, Review: svc.updateReviewToken([]UpdateCheck{c})}
	if c.Skill.Origin != nil {
		out.ExactPath = c.Skill.Origin.ExactPath
	}
	if c.Skill.Origin != nil && c.Skill.Origin.Hash != "" {
		out.BaselineKnown = true
		out.LocalChanged = c.localChanged
		out.UpstreamChanged = c.upstreamChanged
	}
	switch c.Status {
	case UpdateStatusUpToDate:
		out.Status = "up_to_date"
	case UpdateStatusAvailable:
		out.Status = "available"
	case UpdateStatusLocallyEdited:
		out.Status = "locally_edited"
	}
	if c.Err != nil {
		out.Error = c.Err.Error()
		if updateIsSkipped(c.Err) {
			out.Status = "skipped"
		}
	}
	return out
}

func selectUpdateSkills(skills []Skill, names []string) ([]Skill, error) {
	byDir := map[string]Skill{}
	for _, sk := range skills {
		byDir[sk.Dir] = sk
	}
	chosen := []Skill{}
	if len(names) == 0 {
		for _, sk := range skills {
			if sk.InLibrary && sk.Origin != nil && sk.Origin.Type == "git" {
				chosen = append(chosen, sk)
			}
		}
	} else {
		seen := map[string]bool{}
		for _, name := range names {
			sk, found := byDir[name]
			if !found || seen[name] {
				return nil, fmt.Errorf("unknown or duplicate library folder %q; use exact directories from skills list", name)
			}
			seen[name] = true
			if !sk.InLibrary || sk.Origin == nil || sk.Origin.Type != "git" {
				return nil, fmt.Errorf("skill %q has no library Git source", name)
			}
			chosen = append(chosen, sk)
		}
	}
	slices.SortFunc(chosen, func(a, b Skill) int { return strings.Compare(a.Dir, b.Dir) })
	return chosen, nil
}

func cmdUpdates(c cli.Context, args []string, svc *Service) int {
	sub, args := maintenanceSub(args, "check")
	if sub != "check" && sub != "apply" {
		return maintenanceError(c, fmt.Errorf("unknown updates subcommand %q (check, apply)", sub))
	}
	fs := cli.Flags("skills updates "+sub, c.Err)
	jsonOut := fs.Bool("json", false, "JSON output")
	var token string
	var replace bool
	if sub == "apply" {
		fs.StringVar(&token, "review", "", "identity from check")
		fs.BoolVar(&replace, "replace-local", false, "explicit single-skill overwrite")
	}
	names, ok := positionals(fs, args)
	if !ok {
		return 1
	}
	if sub == "apply" && (len(names) == 0 || !validReview(token) || (replace && len(names) != 1)) {
		return maintenanceError(c, fmt.Errorf("usage: lazyagents skills updates apply <directory...> --review token [--replace-local] [--json]"))
	}
	skills, err := svc.Scan(nil)
	if err != nil {
		return maintenanceError(c, err)
	}
	chosen, err := selectUpdateSkills(skills, names)
	if err != nil {
		return maintenanceError(c, err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	r, err := svc.ReviewUpdates(ctx, chosen, nil)
	if err != nil {
		return maintenanceError(c, err)
	}
	defer r.Close()
	if sub == "apply" {
		if expected := svc.updateReviewToken(r.Checks); expected == "" || expected != token {
			return maintenanceError(c, fmt.Errorf("selection, local content, origin or upstream changed since check; check again"))
		}
		return writeMaintenanceResult(c, svc.ApplyUpdates(ctx, r, replace, nil), *jsonOut, "Run skills updates check again before another attempt; skills backups lists archives.")
	}
	out := jsonUpdateReview{Review: svc.updateReviewToken(r.Checks), Items: []jsonUpdateItem{}}
	failed := false
	for _, item := range r.Checks {
		out.Items = append(out.Items, jsonUpdate(svc, item))
		if item.Err != nil && !updateIsSkipped(item.Err) {
			failed = true
		}
	}
	if *jsonOut {
		if code := maintenanceJSON(c, out); code != 0 {
			return code
		}
	} else {
		for _, item := range out.Items {
			fmt.Fprintf(c.Out, "%s: %s\n", reviewText(item.Dir), item.Status)
			if item.Error != "" {
				fmt.Fprintln(c.Out, reviewText(item.Error))
			}
			fmt.Fprintf(c.Out, "Installed commit: %s\nReviewed commit: %s\nBaseline known: %t\n", reviewText(item.Origin.Revision), reviewText(item.Revision), item.BaselineKnown)
			for _, f := range item.Files {
				fmt.Fprintln(c.Out, f)
			}
			fmt.Fprintln(c.Out, item.Diff)
			fmt.Fprintf(c.Out, "Item review: %s\n", item.Review)
		}
		fmt.Fprintf(c.Out, "Selection review: %s\nUse skills updates apply <directory...> --review <token> for that exact scope.\n", out.Review)
	}
	if failed {
		return maintenanceError(c, fmt.Errorf("some update checks failed; inspect item errors"))
	}
	return 0
}

type jsonMaintenanceTarget struct {
	ID      string             `json:"id"`
	Outcome core.TargetOutcome `json:"outcome"`
	Detail  string             `json:"detail"`
	State   string             `json:"state"`
}
type jsonMaintenanceResult struct {
	Targets []jsonMaintenanceTarget `json:"targets"`
	Error   string                  `json:"error,omitempty"`
}

func writeMaintenanceResult(c cli.Context, result core.OperationResult, jsonOut bool, recovery string) int {
	if result.HistoryWarning != "" {
		fmt.Fprintln(c.Err, result.HistoryWarning)
	}
	out := jsonMaintenanceResult{Targets: []jsonMaintenanceTarget{}}
	for _, t := range result.Targets {
		out.Targets = append(out.Targets, jsonMaintenanceTarget{t.ID, t.Outcome, t.Detail, t.State})
	}
	if result.Err != nil {
		out.Error = result.Err.Error()
	}
	if jsonOut {
		if code := maintenanceJSON(c, out); code != 0 {
			return code
		}
	} else {
		for _, t := range out.Targets {
			fmt.Fprintf(c.Out, "%s: %s — %s (%s)\n", reviewText(t.ID), t.Outcome, reviewText(t.Detail), reviewText(t.State))
		}
	}
	if err := result.AsError(); err != nil {
		fmt.Fprintln(c.Err, recovery)
		return maintenanceError(c, err)
	}
	return 0
}

type jsonProfileChange struct {
	Dir    string   `json:"dir"`
	Name   string   `json:"name"`
	Add    []string `json:"add"`
	Remove []string `json:"remove"`
}
type jsonProfilePreview struct {
	Name    string              `json:"name"`
	Spec    ProfileSpec         `json:"spec"`
	Targets []string            `json:"targets"`
	Changes []jsonProfileChange `json:"changes"`
	Review  string              `json:"review"`
}

func cmdProfiles(c cli.Context, args []string, svc *Service) int {
	sub, args := maintenanceSub(args, "list")
	if !slices.Contains([]string{"list", "show", "save", "preview", "apply", "remove"}, sub) {
		return maintenanceError(c, fmt.Errorf("unknown profiles subcommand %q (list, show, save, preview, apply, remove)", sub))
	}
	fs := cli.Flags("skills profiles "+sub, c.Err)
	jsonOut := fs.Bool("json", false, "JSON output")
	var token string
	if sub == "apply" {
		fs.StringVar(&token, "review", "", "identity from preview")
	}
	pos, ok := positionals(fs, args)
	if !ok || (sub == "list" && len(pos) != 0) || (sub != "list" && (len(pos) != 1 || strings.TrimSpace(pos[0]) == "")) || (sub == "apply" && !validReview(token)) {
		return maintenanceError(c, fmt.Errorf("usage: lazyagents skills profiles list|show <name>|save <name>|preview <name>|apply <name> --review token|remove <name> [--json]"))
	}
	if sub == "list" {
		names, err := svc.ListProfiles()
		if err != nil {
			return maintenanceError(c, err)
		}
		if *jsonOut {
			return maintenanceJSON(c, names)
		}
		for _, n := range names {
			fmt.Fprintln(c.Out, reviewText(n))
		}
		return 0
	}
	name := pos[0]
	switch sub {
	case "show":
		spec, err := svc.GetProfile(name)
		if err != nil {
			return maintenanceError(c, err)
		}
		// Specifications are mappings; JSON is also the readable text representation.
		return maintenanceJSON(c, struct {
			Name string      `json:"name"`
			Spec ProfileSpec `json:"spec"`
		}{name, spec})
	case "save", "remove":
		var err error
		if sub == "save" {
			agents := c.Agents()
			skills, scanErr := svc.Scan(agents)
			if scanErr != nil {
				return maintenanceError(c, scanErr)
			}
			err = svc.SaveProfile(name, BuildProfileSpec(skills, agents))
		} else {
			err = svc.DeleteProfile(name)
		}
		if err != nil {
			return maintenanceError(c, err)
		}
		if *jsonOut {
			return maintenanceJSON(c, struct {
				Name   string `json:"name"`
				Action string `json:"action"`
			}{name, sub})
		}
		fmt.Fprintf(c.Out, "profile %q: %s\n", name, sub)
		return 0
	case "apply":
		return writeMaintenanceResult(c, svc.applyReviewedProfile(name, token, c.Agents()), *jsonOut, "Preview the skill profile again before another attempt; activation can be adjusted with enable/disable.")
	case "preview":
		agents := c.Agents()
		r, err := svc.previewProfile(name, agents)
		if err != nil {
			return maintenanceError(c, err)
		}
		out := jsonProfilePreview{Name: name, Spec: r.spec, Targets: []string{}, Changes: []jsonProfileChange{}, Review: r.token}
		for _, a := range skillCapableAgents(agents) {
			out.Targets = append(out.Targets, a.ID)
		}
		slices.Sort(out.Targets)
		for _, change := range r.changes {
			out.Changes = append(out.Changes, jsonProfileChange{change.Dir, change.Skill, append([]string{}, change.Add...), append([]string{}, change.Remove...)})
		}
		if *jsonOut {
			return maintenanceJSON(c, out)
		}
		fmt.Fprintf(c.Out, "Profile: %s\nFull matrix destinations: %s\n", reviewText(name), reviewText(strings.Join(out.Targets, ", ")))
		fmt.Fprintln(c.Out, "Saved folder-to-agent specification:")
		if code := maintenanceJSON(c, out.Spec); code != 0 {
			return code
		}
		for _, change := range out.Changes {
			fmt.Fprintf(c.Out, "%s (%s): enable [%s], disable [%s]\n", reviewText(change.Dir), reviewText(change.Name), reviewText(strings.Join(change.Add, ", ")), reviewText(strings.Join(change.Remove, ", ")))
		}
		fmt.Fprintf(c.Out, "Review: %s\nUse skills profiles apply <name> --review <token> for this full matrix.\n", out.Review)
		return 0
	}
	return 1
}
