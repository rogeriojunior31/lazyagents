//go:build ignore

// record-fixtures records the transcript fixtures in
// internal/agent/testdata/fixtures: it runs each agent CLI installed here in a
// throwaway home against fakellm, a deterministic local model server, and keeps
// the files the CLI wrote, sanitized. No account, no network, no personal data.
//
//	go run scripts/record-fixtures.go                  # every installed CLI
//	go run scripts/record-fixtures.go -only codex,pi
//	go run scripts/record-fixtures.go -serve           # only the model server
//
// fakellm speaks OpenAI Chat Completions, OpenAI Responses and Anthropic
// Messages. The first turn asks for the shell tool the CLI offers, with
// `echo fixture`; once the tool ran it answers in text. Usage numbers are
// fixed, so a re-recording only changes what the CLI itself changed.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	reply   = "The command printed fixture."
	command = "echo fixture"
)

const prompt = "run echo fixture please"

// recorder runs one agent CLI in home against the model server at base.
type recorder struct {
	id, bin string
	// setup writes the CLI's config into home; run records the session.
	setup func(home, base string) error
	run   func(r *run) error
	// collect lists the files to keep, relative to home.
	collect []string // globs
	notes   string
}

type run struct {
	home, work, base string
	env              []string
}

func (r *run) cli(name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir, cmd.Env, cmd.Stdin = r.work, r.env, nil
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, out)
	}
	return nil
}

var recorders = []recorder{
	{
		id: "claude-code", bin: "claude",
		setup: func(home, base string) error { return nil },
		run: func(r *run) error {
			r.env = append(r.env, "ANTHROPIC_BASE_URL="+r.base, "ANTHROPIC_API_KEY=fake-key",
				"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1", "DISABLE_AUTOUPDATER=1")
			return r.cli("claude", "-p", prompt, "--model", "claude-sonnet-4-5", "--allowedTools", "Bash")
		},
		collect: []string{".claude/projects/*/*.jsonl"},
		notes:   "claude -p with the Bash tool; attachments reduced to their type",
	},
	{
		id: "codex", bin: "codex",
		setup: func(home, base string) error {
			return write(filepath.Join(home, ".codex", "config.toml"), `model = "fake-model"
model_provider = "fake"

[features]
plugins = false
remote_plugin = false

[model_providers.fake]
name = "fake"
base_url = "`+base+`/v1"
env_key = "FAKE_KEY"
wire_api = "responses"
`)
		},
		run: func(r *run) error {
			r.env = append(r.env, "CODEX_HOME="+filepath.Join(r.home, ".codex"), "FAKE_KEY=x")
			return r.cli("codex", "exec", "--skip-git-repo-check", "--dangerously-bypass-approvals-and-sandbox", prompt)
		},
		collect: []string{".codex/sessions/*/*/*/*.jsonl"},
		notes:   "codex exec with a custom Responses provider",
	},
	{
		id: "opencode", bin: "opencode",
		setup: func(home, base string) error {
			return write(filepath.Join(home, ".config", "opencode", "opencode.json"), `{"model":"fake/fake-model","autoupdate":false,"share":"disabled",
 "provider":{"fake":{"npm":"@ai-sdk/openai-compatible","name":"Fake","options":{"baseURL":"`+base+`/v1","apiKey":"x"},"models":{"fake-model":{"name":"Fake model"}}}}}
`)
		},
		run: func(r *run) error {
			r.env = append(r.env, "OPENCODE_DISABLE_MODELS_FETCH=1", "OPENCODE_DISABLE_AUTOUPDATE=1", "OPENCODE_DISABLE_LSP_DOWNLOAD=1")
			return r.cli("opencode", "run", prompt)
		},
		collect: []string{".local/share/opencode/opencode.db"},
		notes:   "opencode run with an OpenAI-compatible provider; the database is kept as an SQL dump",
	},
	{
		id: "pi", bin: "pi",
		setup: func(home, base string) error {
			return write(filepath.Join(home, ".pi", "agent", "models.json"), `{"providers":{"fake":{"baseUrl":"`+base+`/v1","api":"openai-completions","apiKey":"x",
 "models":[{"id":"fake-model","cost":{"input":1,"output":2,"cacheRead":0.1,"cacheWrite":0}}]}}}
`)
		},
		run: func(r *run) error {
			r.env = append(r.env, "PI_OFFLINE=1", "PI_SKIP_VERSION_CHECK=1")
			if err := r.cli("pi", "--provider", "fake", "--model", "fake-model", "-n", "Fixture session", "-p", prompt); err != nil {
				return err
			}
			files, _ := filepath.Glob(filepath.Join(r.home, ".pi/agent/sessions/*/*.jsonl"))
			if len(files) != 1 {
				return fmt.Errorf("pi wrote %d sessions, want 1", len(files))
			}
			return r.cli("pi", "--session", files[0], "-p", "and once more")
		},
		collect: []string{".pi/agent/sessions/*/*.jsonl"},
		notes:   "pi -p, named, then resumed with --session for a second turn",
	},
}

func main() {
	serve := flag.Bool("serve", false, "only run the model server")
	addr := flag.String("addr", "127.0.0.1:18765", "listen address with -serve")
	only := flag.String("only", "", "comma-separated agent ids to record (default: every installed CLI)")
	out := flag.String("out", "internal/agent/testdata/fixtures", "fixtures dir")
	flag.StringVar(&codexLimitsFrom, "codex-limits-from", "", "Codex sessions dir (e.g. ~/.codex/sessions): pin the rate_limits shape of its newest rollout, with synthetic values")
	flag.Parse()
	http.HandleFunc("/", handle)
	if *serve {
		log.Printf("fakellm on http://%s", *addr)
		log.Fatal(http.ListenAndServe(*addr, nil))
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	go http.Serve(ln, nil)
	base := "http://" + ln.Addr().String()
	failed := false
	for _, rec := range recorders {
		if *only != "" && !slices.Contains(strings.Split(*only, ","), rec.id) {
			continue
		}
		if _, err := exec.LookPath(rec.bin); err != nil {
			log.Printf("%s: %s not installed, skipped", rec.id, rec.bin)
			continue
		}
		dir, err := record(rec, base, *out)
		if err != nil {
			log.Printf("%s: %v", rec.id, err)
			failed = true
			continue
		}
		log.Printf("%s: recorded %s", rec.id, dir)
	}
	if codexLimitsFrom != "" {
		dir, err := recordCodexLimits(*out)
		if err != nil {
			log.Printf("codex limits: %v", err)
			failed = true
		} else {
			log.Printf("codex limits: recorded %s", dir)
		}
	}
	if failed {
		os.Exit(1)
	}
}

func writeOrigin(dest, agent, version, how, by string) error {
	origin, _ := json.MarshalIndent(map[string]string{
		"agent": agent, "version": version, "recorded": time.Now().UTC().Format("2006-01-02"), "how": how, "by": by,
	}, "", "  ")
	return write(filepath.Join(dest, "origin.json"), string(origin)+"\n")
}

func record(rec recorder, base, out string) (string, error) {
	version, err := cliVersion(rec.bin)
	if err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp("", "lazyagents-fixture-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	home, work := filepath.Join(tmp, "home"), filepath.Join(tmp, "work", "proj")
	if err := os.MkdirAll(work, 0o755); err != nil {
		return "", err
	}
	defer exec.Command("pkill", "-f", tmp).Run() // daemons some CLIs leave behind
	if err := rec.setup(home, base); err != nil {
		return "", err
	}
	r := &run{home: home, work: work, base: base, env: []string{
		"HOME=" + home, "PATH=" + os.Getenv("PATH"), "TERM=dumb",
		"XDG_CONFIG_HOME=" + filepath.Join(home, ".config"), "XDG_DATA_HOME=" + filepath.Join(home, ".local", "share"),
		"XDG_CACHE_HOME=" + filepath.Join(home, ".cache"), "XDG_STATE_HOME=" + filepath.Join(home, ".local", "state"),
	}}
	if err := rec.run(r); err != nil {
		return "", err
	}
	var files []string
	for _, g := range rec.collect {
		m, _ := filepath.Glob(filepath.Join(home, g))
		files = append(files, m...)
	}
	if len(files) == 0 {
		return "", errors.New("the CLI wrote no session file")
	}
	dest := filepath.Join(out, rec.id, version)
	if err := os.RemoveAll(dest); err != nil {
		return "", err
	}
	san := newSanitizer(home, work)
	for _, f := range files {
		rel, _ := filepath.Rel(home, f)
		var data []byte
		if strings.HasSuffix(f, ".db") {
			rel = strings.TrimSuffix(rel, ".db") + ".sql"
			if data, err = exec.Command("sqlite3", f, ".dump").Output(); err != nil {
				return "", fmt.Errorf("dumping %s: %w", f, err)
			}
		} else if data, err = os.ReadFile(f); err != nil {
			return "", err
		}
		clean, err := san.clean(rec.id, data, strings.HasSuffix(f, ".jsonl"))
		if err != nil {
			return "", fmt.Errorf("%s: %w", rel, err)
		}
		if err := write(filepath.Join(dest, "home", san.path(rel)), string(clean)); err != nil {
			return "", err
		}
	}
	return dest, writeOrigin(dest, rec.id, version, rec.notes, "scripts/record-fixtures.go against its local fake model")
}

var codexLimitsFrom string

// recordCodexLimits pins the rate_limits format. The fake model cannot
// produce limits, so the shape comes from the newest real rollout that has
// them; every value that could describe the account (usage, reset times,
// credits, plan) is replaced and nothing else of that rollout is kept. When a
// session of the same Codex version was recorded, the line joins it;
// otherwise it gets its own fixture with a minimal session_meta.
func recordCodexLimits(out string) (string, error) {
	var newest, version string
	var newestMod time.Time
	var limits map[string]any
	_ = filepath.WalkDir(codexLimitsFrom, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".jsonl") {
			return nil
		}
		info, err := d.Info()
		if err != nil || !info.ModTime().After(newestMod) {
			return nil
		}
		if l := lastLimits(p); l != nil {
			newest, newestMod, limits, version = p, info.ModTime(), l, codexVersionOf(p)
		}
		return nil
	})
	if newest == "" || version == "" {
		return "", fmt.Errorf("no rollout with rate limits in %s", codexLimitsFrom)
	}
	synthetic(limits, "")
	line, _ := json.Marshal(map[string]any{"timestamp": "2026-01-01T00:00:00.000Z", "type": "event_msg",
		"payload": map[string]any{"type": "token_count", "info": nil, "rate_limits": limits}})
	dest := filepath.Join(out, "codex", version)
	rollouts, _ := filepath.Glob(filepath.Join(dest, "home/.codex/sessions/*/*/*/*.jsonl"))
	switch len(rollouts) {
	case 0:
		meta, _ := json.Marshal(map[string]any{"timestamp": "2026-01-01T00:00:00.000Z", "type": "session_meta",
			"payload": map[string]any{"id": "00000000-0000-4000-8000-000000000001", "timestamp": "2026-01-01T00:00:00.000Z",
				"cwd": "/work/proj", "originator": "codex_exec", "cli_version": version}})
		path := filepath.Join(dest, "home/.codex/sessions/2026/01/01/rollout-2026-01-01T00-00-00-00000000-0000-4000-8000-000000000001.jsonl")
		if err := write(path, string(meta)+"\n"+string(line)+"\n"); err != nil {
			return "", err
		}
		return dest, writeOrigin(dest, "codex", version,
			"rate_limits only: the token_count line has the shape of a real rollout of this version, with synthetic values; the session_meta is made up",
			"scripts/record-fixtures.go -codex-limits-from")
	case 1:
		f, err := os.OpenFile(rollouts[0], os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			return "", err
		}
		defer f.Close()
		_, err = f.Write(append(line, '\n'))
		return dest, err
	}
	return "", fmt.Errorf("%s has %d rollouts, want 1", dest, len(rollouts))
}

// lastLimits is the last rate_limits with a window in a rollout.
func lastLimits(path string) map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var limits map[string]any
	for line := range bytes.Lines(data) {
		var e struct {
			Payload struct {
				Type       string         `json:"type"`
				RateLimits map[string]any `json:"rate_limits"`
			} `json:"payload"`
		}
		if json.Unmarshal(line, &e) == nil && e.Payload.Type == "token_count" &&
			(e.Payload.RateLimits["primary"] != nil || e.Payload.RateLimits["secondary"] != nil) {
			limits = e.Payload.RateLimits
		}
	}
	return limits
}

func codexVersionOf(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	var meta struct {
		Payload struct {
			CLIVersion string `json:"cli_version"`
		} `json:"payload"`
	}
	_ = json.NewDecoder(f).Decode(&meta)
	return meta.Payload.CLIVersion
}

// synthetic keeps keys, types and window sizes and replaces the rest.
func synthetic(m map[string]any, parent string) {
	for k, v := range m {
		switch x := v.(type) {
		case map[string]any:
			synthetic(x, k)
		case float64:
			switch k {
			case "window_minutes":
			case "used_percent":
				m[k] = map[string]float64{"primary": 42.5, "secondary": 17}[parent]
			case "resets_at":
				m[k] = map[string]float64{"primary": 1767243600, "secondary": 1767772800}[parent]
			default:
				m[k] = 0
			}
		case string:
			switch k {
			case "limit_id":
			case "plan_type":
				m[k] = "plus"
			default:
				m[k] = "0"
			}
		}
	}
}

var versionRe = regexp.MustCompile(`\d+\.\d+\.\d+`)

func cliVersion(bin string) (string, error) {
	out, err := exec.Command(bin, "--version").Output()
	if v := versionRe.FindString(string(out)); err == nil && v != "" {
		return v, nil
	}
	return "", fmt.Errorf("%s --version: no version in %q (%v)", bin, out, err)
}

// sanitizer turns a recording into a fixture: throwaway paths become
// /work/proj and /home/user (also in the dir-name encodings agents use), long
// strings (system prompts, tool listings) are cut, and anything naming this
// machine fails the recording instead of reaching the repo.
type sanitizer struct {
	repl      *strings.Replacer
	forbidden []string
}

func newSanitizer(home, work string) *sanitizer {
	const fakeWork, fakeHome = "/work/proj", "/home/user"
	pairs := []string{work, fakeWork, home, fakeHome}
	for _, enc := range []func(string) string{claudeDir, piDir} {
		pairs = append(pairs, enc(work), enc(fakeWork))
	}
	tmp := filepath.Dir(filepath.Dir(work)) // TMPDIR of the recording
	pairs = append(pairs, tmp, "/tmp/rec")
	s := &sanitizer{repl: strings.NewReplacer(pairs...)}
	if u, err := user.Current(); err == nil {
		s.forbidden = append(s.forbidden, u.Username)
	}
	if h, err := os.Hostname(); err == nil {
		s.forbidden = append(s.forbidden, h)
	}
	if out, err := exec.Command("uname", "-r").Output(); err == nil {
		s.forbidden = append(s.forbidden, strings.TrimSpace(string(out)))
	}
	return s
}

// claudeDir and piDir are how Claude Code and pi name a project's folder.
func claudeDir(p string) string { return regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAllString(p, "-") }
func piDir(p string) string {
	return "--" + strings.NewReplacer("/", "-", "\\", "-", ":", "-").Replace(strings.TrimPrefix(p, "/")) + "--"
}

func (s *sanitizer) path(rel string) string { return s.repl.Replace(rel) }

func (s *sanitizer) clean(agent string, data []byte, jsonl bool) ([]byte, error) {
	if jsonl {
		var out bytes.Buffer
		for line := range bytes.Lines(data) {
			var v any
			if json.Unmarshal(line, &v) != nil {
				continue
			}
			v = trim(agent, v)
			raw, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			out.Write(raw)
			out.WriteByte('\n')
		}
		data = out.Bytes()
	}
	data = []byte(s.repl.Replace(string(data)))
	for _, f := range s.forbidden {
		if f != "" && bytes.Contains(data, []byte(f)) {
			return nil, fmt.Errorf("still names this machine (%q): extend the sanitizer", f)
		}
	}
	return data, nil
}

// trim cuts long strings, keeping their start, and, for Claude Code, reduces attachments (system
// prompt snapshots, environment, skill listings) to their type.
func trim(agent string, v any) any {
	switch x := v.(type) {
	case map[string]any:
		if agent == "claude-code" && x["type"] == "attachment" {
			if a, ok := x["attachment"].(map[string]any); ok {
				x["attachment"] = map[string]any{"type": a["type"]}
			}
		}
		for k, e := range x {
			x[k] = trim(agent, e)
		}
	case []any:
		for i, e := range x {
			x[i] = trim(agent, e)
		}
	case string:
		// keep the start: agents skip harness text by its first characters ("<env…")
		if r := []rune(x); len(r) > 200 {
			return string(r[:40]) + fmt.Sprintf("… (trimmed: %d bytes)", len(x))
		}
	}
	return v
}

func write(path, data string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(data), 0o644)
}

func handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var req map[string]any
	_ = json.Unmarshal(body, &req)
	path := r.URL.Path
	log.Printf("%s %s", r.Method, path)
	switch {
	case strings.HasSuffix(path, "/models"):
		writeJSON(w, map[string]any{"object": "list", "data": []any{map[string]any{"id": "fake-model", "object": "model"}}})
	case strings.HasSuffix(path, "/count_tokens"):
		writeJSON(w, map[string]any{"input_tokens": 100})
	case strings.HasSuffix(path, "/chat/completions"):
		chat(w, req)
	case strings.HasSuffix(path, "/responses"):
		responses(w, req)
	case strings.HasSuffix(path, "/messages"):
		messages(w, req)
	default:
		http.NotFound(w, r)
	}
}

// shellTool picks the tool a CLI offers for shell commands and builds its
// arguments from the tool's own schema.
func shellTool(name string, schema any) (map[string]any, bool) {
	n := strings.ToLower(name)
	if !strings.Contains(n, "bash") && !strings.Contains(n, "shell") && !strings.Contains(n, "exec") {
		return nil, false
	}
	props, _ := dig(schema, "properties").(map[string]any)
	args := map[string]any{}
	switch {
	case props["cmd"] != nil:
		args["cmd"] = command
	case dig(props, "command", "type") == "array":
		args["command"] = []string{"bash", "-lc", command}
	default:
		args["command"] = command
	}
	if props["description"] != nil {
		args["description"] = "Print fixture"
	}
	return args, true
}

func dig(v any, keys ...string) any {
	for _, k := range keys {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[k]
	}
	return v
}

func list(v any) []any { l, _ := v.([]any); return l }

// OpenAI Chat Completions (pi, OpenCode).
func chat(w http.ResponseWriter, req map[string]any) {
	msgs := list(req["messages"])
	toolTurn := len(msgs) > 0 && dig(msgs[len(msgs)-1], "role") == "user"
	var name string
	var args map[string]any
	if toolTurn {
		for _, t := range list(req["tools"]) {
			n, _ := dig(t, "function", "name").(string)
			if a, ok := shellTool(n, dig(t, "function", "parameters")); ok {
				name, args = n, a
				break
			}
		}
	}
	chunk := func(delta map[string]any, finish any) map[string]any {
		return map[string]any{"id": "chatcmpl-1", "object": "chat.completion.chunk", "created": 0, "model": req["model"],
			"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}}
	}
	var out []any
	out = append(out, chunk(map[string]any{"role": "assistant"}, nil))
	usage := map[string]any{"prompt_tokens": 150, "completion_tokens": 6, "total_tokens": 156,
		"prompt_tokens_details": map[string]any{"cached_tokens": 100}}
	if name != "" {
		raw, _ := json.Marshal(args)
		out = append(out, chunk(map[string]any{"tool_calls": []any{map[string]any{"index": 0, "id": "call_1", "type": "function",
			"function": map[string]any{"name": name, "arguments": string(raw)}}}}, nil), chunk(map[string]any{}, "tool_calls"))
		usage = map[string]any{"prompt_tokens": 120, "completion_tokens": 8, "total_tokens": 128}
	} else {
		out = append(out, chunk(map[string]any{"content": reply}, nil), chunk(map[string]any{}, "stop"))
	}
	out = append(out, map[string]any{"id": "chatcmpl-1", "object": "chat.completion.chunk", "created": 0, "model": req["model"],
		"choices": []any{}, "usage": usage})
	sse(w, "", out)
}

// OpenAI Responses (Codex).
func responses(w http.ResponseWriter, req map[string]any) {
	input := list(req["input"])
	last := map[string]any{}
	if len(input) > 0 {
		last, _ = input[len(input)-1].(map[string]any)
	}
	toolTurn := last["role"] == "user"
	var item map[string]any
	if toolTurn {
		for _, t := range list(req["tools"]) {
			n, _ := dig(t, "name").(string)
			if a, ok := shellTool(n, dig(t, "parameters")); ok {
				raw, _ := json.Marshal(a)
				item = map[string]any{"type": "function_call", "id": "fc_1", "call_id": "call_1", "name": n,
					"arguments": string(raw), "status": "completed"}
				break
			}
		}
	}
	usage := map[string]any{"input_tokens": 150, "input_tokens_details": map[string]any{"cached_tokens": 100},
		"output_tokens": 6, "output_tokens_details": map[string]any{"reasoning_tokens": 0}, "total_tokens": 156}
	if item == nil {
		item = map[string]any{"type": "message", "id": "msg_1", "role": "assistant", "status": "completed",
			"content": []any{map[string]any{"type": "output_text", "text": reply, "annotations": []any{}}}}
	} else {
		usage = map[string]any{"input_tokens": 120, "input_tokens_details": map[string]any{"cached_tokens": 0},
			"output_tokens": 8, "output_tokens_details": map[string]any{"reasoning_tokens": 0}, "total_tokens": 128}
	}
	resp := map[string]any{"id": "resp_1", "object": "response", "model": req["model"], "status": "in_progress", "output": []any{}}
	done := map[string]any{"id": "resp_1", "object": "response", "model": req["model"], "status": "completed",
		"output": []any{item}, "usage": usage}
	sse(w, "type", []any{
		map[string]any{"type": "response.created", "response": resp},
		map[string]any{"type": "response.output_item.added", "output_index": 0, "item": item},
		map[string]any{"type": "response.output_item.done", "output_index": 0, "item": item},
		map[string]any{"type": "response.completed", "response": done},
	})
}

// Anthropic Messages (Claude Code).
func messages(w http.ResponseWriter, req map[string]any) {
	msgs := list(req["messages"])
	toolTurn := false
	if len(msgs) > 0 && dig(msgs[len(msgs)-1], "role") == "user" {
		toolTurn = true
		for _, b := range list(dig(msgs[len(msgs)-1], "content")) {
			if dig(b, "type") == "tool_result" {
				toolTurn = false
			}
		}
	}
	var block map[string]any
	var args map[string]any
	if toolTurn {
		for _, t := range list(req["tools"]) {
			n, _ := dig(t, "name").(string)
			if a, ok := shellTool(n, dig(t, "input_schema")); ok {
				block, args = map[string]any{"type": "tool_use", "id": "toolu_1", "name": n, "input": map[string]any{}}, a
				break
			}
		}
	}
	stop := "end_turn"
	usage := map[string]any{"input_tokens": 50, "output_tokens": 6, "cache_read_input_tokens": 100, "cache_creation_input_tokens": 0}
	if block != nil {
		stop = "tool_use"
		usage = map[string]any{"input_tokens": 120, "output_tokens": 8, "cache_read_input_tokens": 0, "cache_creation_input_tokens": 0}
	}
	if stream, _ := req["stream"].(bool); !stream {
		content := []any{map[string]any{"type": "text", "text": reply}}
		if block != nil {
			block["input"] = args
			content = []any{block}
		}
		writeJSON(w, map[string]any{"id": "msg_1", "type": "message", "role": "assistant", "model": req["model"],
			"content": content, "stop_reason": stop, "stop_sequence": nil, "usage": usage})
		return
	}
	start := map[string]any{"type": "message_start", "message": map[string]any{"id": "msg_1", "type": "message", "role": "assistant",
		"model": req["model"], "content": []any{}, "stop_reason": nil, "stop_sequence": nil,
		"usage": map[string]any{"input_tokens": usage["input_tokens"], "output_tokens": 1,
			"cache_read_input_tokens": usage["cache_read_input_tokens"], "cache_creation_input_tokens": 0}}}
	events := []any{start}
	if block != nil {
		raw, _ := json.Marshal(args)
		events = append(events,
			map[string]any{"type": "content_block_start", "index": 0, "content_block": block},
			map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "input_json_delta", "partial_json": string(raw)}})
	} else {
		events = append(events,
			map[string]any{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "text", "text": ""}},
			map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": reply}})
	}
	events = append(events,
		map[string]any{"type": "content_block_stop", "index": 0},
		map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": stop, "stop_sequence": nil},
			"usage": map[string]any{"output_tokens": usage["output_tokens"]}},
		map[string]any{"type": "message_stop"})
	sse(w, "type", events)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// sse streams events; eventKey names the SSE "event:" field when the API uses
// one (Responses and Anthropic), and Chat Completions ends with [DONE].
func sse(w http.ResponseWriter, eventKey string, events []any) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, e := range events {
		raw, _ := json.Marshal(e)
		if eventKey != "" {
			fmt.Fprintf(w, "event: %s\n", dig(e, eventKey))
		}
		fmt.Fprintf(w, "data: %s\n\n", raw)
	}
	if eventKey == "" {
		fmt.Fprint(w, "data: [DONE]\n\n")
	}
}
