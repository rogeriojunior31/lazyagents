//go:build ignore

// record-fixtures records the transcript fixtures in
// internal/agent/testdata/fixtures: it runs each agent CLI installed here in a
// throwaway home against fakellm, a deterministic local model server, and keeps
// sanitized CLI output and configuration inputs. No account or paid inference;
// model endpoints are local, without an OS-level network sandbox.
//
//	go run scripts/record-fixtures.go                  # every installed CLI
//	go run scripts/record-fixtures.go -only codex,pi
//	go run scripts/record-fixtures.go -serve           # only the model server
//
// fakellm speaks OpenAI Chat Completions/Responses, Anthropic Messages and
// Gemini GenerateContent. The first turn asks for the CLI's shell tool, with
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
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
)

const (
	reply   = "The command printed fixture."
	command = "echo fixture"
)

const prompt = "run echo fixture please"

var openCodeProviderRoundtrip bool
var geminiProviderRoundtrip bool
var sessionLinks bool
var sessionLifecycle bool

// recorder runs one agent CLI in home against the model server at base.
type recorder struct {
	id, bin string
	// setup writes the CLI's config into home; run records the session.
	setup func(home, base string) error
	run   func(r *run) error
	// collect lists the files to keep, relative to home.
	collect []string // globs
	config  []string // recorder inputs accepted by the CLI, not CLI-generated defaults
	notes   string
	// workInHome puts the project inside home, for agents that keep sessions
	// in the project (Crush): the fixture home then holds them too.
	workInHome bool
}

type run struct {
	home, work, base string
	env              []string
}

func (r *run) cli(name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = 2 * time.Second
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
		id: "gemini-cli", bin: "gemini",
		setup: func(home, base string) error {
			return write(filepath.Join(home, ".gemini", "settings.json"), `{
 "security":{"auth":{"selectedType":"gemini-api-key"}},
 "general":{"enableAutoUpdate":false,"enableAutoUpdateNotification":false},
 "privacy":{"usageStatisticsEnabled":false},"telemetry":{"enabled":false}
}
`)
		},
		run: func(r *run) error {
			if geminiProviderRoundtrip {
				return recordGeminiProviders(r)
			}
			r.env = append(r.env, "GEMINI_API_KEY=fixture-key", "GOOGLE_GEMINI_BASE_URL="+r.base,
				"GEMINI_CLI_NO_RELAUNCH=1")
			// Native automation flag, scoped to our empty throwaway workspace;
			// never create or copy a user's persistent trust record.
			return r.cli("gemini", "-p", prompt, "--model", "gemini-2.5-flash", "--skip-trust", "--allowed-tools", "run_shell_command")
		},
		collect: []string{".gemini/tmp/*/chats/session-*", ".gemini/history/*/chats/session-*", "evidence/gemini-providers.json"},
		config:  []string{".gemini/settings.json", ".gemini/projects.json"},
		notes:   "gemini -p --skip-trust in an empty temporary workspace with a local GenerateContent endpoint and the shell tool; settings are recorder inputs, sessions are CLI output",
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
		config:  []string{".codex/config.toml"},
		notes:   "codex exec with a custom Responses provider",
	},
	{
		id: "opencode", bin: "opencode",
		setup: func(home, base string) error {
			if openCodeProviderRoundtrip {
				return write(filepath.Join(home, ".config", "opencode", "opencode.json"), `{"model":{"providerID":"fake","model":"fake-model"},"update":"disable","share":"disabled",
 "providers":{"fake":{"package":"@opencode/ai/providers/openai-compatible","name":"Foreign fixture","settings":{"baseURL":"`+base+`/v1","apiKey":"fixture-native-key"},"headers":{"X-Fixture":"preserved"},"models":{"fake-model":{"name":"Foreign model"}}}}}
`)
			}
			return write(filepath.Join(home, ".config", "opencode", "opencode.json"), `{"model":"fake/fake-model","autoupdate":false,"share":"disabled",
 "provider":{"fake":{"npm":"@ai-sdk/openai-compatible","name":"Fake","options":{"baseURL":"`+base+`/v1","apiKey":"x"},"models":{"fake-model":{"name":"Fake model"}}}}}
`)
		},
		run: func(r *run) error {
			r.env = append(r.env, "OPENCODE_DISABLE_MODELS_FETCH=1", "OPENCODE_DISABLE_AUTOUPDATE=1", "OPENCODE_DISABLE_LSP_DOWNLOAD=1")
			if sessionLinks {
				return r.cli("opencode", "run", "--standalone", "delegate fixture please")
			}
			if sessionLifecycle {
				return r.cli("opencode", "run", "--standalone", prompt)
			}
			if openCodeProviderRoundtrip {
				return recordOpenCodeProviders(r)
			}
			return r.cli("opencode", "run", prompt)
		},
		collect: []string{".local/share/opencode/opencode.db", "evidence/opencode-providers.json"},
		config:  []string{".config/opencode/opencode.json"},
		notes:   "opencode run with an OpenAI-compatible provider; the database is kept as an SQL dump",
	},
	{
		id: "crush", bin: "crush", workInHome: true,
		setup: func(home, base string) error {
			return write(filepath.Join(home, ".config", "crush", "crush.json"), `{"providers":{"fake":{"type":"openai-compat","base_url":"`+base+`/v1","api_key":"x",
 "models":[{"id":"fake-model","name":"Fake","context_window":100000,"default_max_tokens":1000,"cost_per_1m_in":1,"cost_per_1m_out":2}]}},
 "models":{"large":{"model":"fake-model","provider":"fake"},"small":{"model":"fake-model","provider":"fake"}},
 "permissions":{"allowed_tools":["bash","agent"]},"options":{"disable_metrics":true}}
`)
		},
		run: func(r *run) error {
			r.env = append(r.env, "CRUSH_DISABLE_METRICS=1", "CRUSH_DISABLE_PROVIDER_AUTO_UPDATE=1")
			if sessionLinks {
				return r.cli("crush", "run", "--quiet", "delegate fixture please")
			}
			if err := r.cli("crush", "run", "--quiet", prompt); err != nil {
				return err
			}
			return r.cli("crush", "run", "--quiet", "--continue", "and once more")
		},
		collect: []string{"work/proj/.crush/crush.db"},
		config:  []string{".config/crush/crush.json", ".local/share/crush/projects.json"},
		notes:   "crush run, then --continue for a second turn; the project sits in the home, the database is kept as an SQL dump",
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
		config:  []string{".pi/agent/models.json", ".pi/agent/settings.json"},
		notes:   "pi -p, named, then resumed with --session for a second turn",
	},
}

func main() {
	serve := flag.Bool("serve", false, "only run the model server")
	addr := flag.String("addr", "127.0.0.1:18765", "listen address with -serve")
	only := flag.String("only", "", "comma-separated agent ids to record (default: every installed CLI)")
	out := flag.String("out", "internal/agent/testdata/fixtures", "fixtures dir")
	claudeLimits := flag.Bool("claude-limits", false, "pin the shape of Claude Code's /api/oauth/usage response: one real call with your Claude Code login, values made synthetic")
	flag.BoolVar(&geminiProviderRoundtrip, "gemini-provider-roundtrip", false, "verify Gemini GenerateContent provider apply/clear with synthetic API keys against the fake model")
	flag.BoolVar(&openCodeProviderRoundtrip, "opencode-provider-roundtrip", false, "verify OpenCode provider apply/clear with token, environment reference and keyless credentials against the fake model")
	flag.StringVar(&codexLimitsFrom, "codex-limits-from", "", "Codex sessions dir (e.g. ~/.codex/sessions): pin the rate_limits shape of its newest rollout, with synthetic values")
	flag.BoolVar(&sessionLinks, "session-links", false, "record native OpenCode/Crush child sessions against the local fake model")
	flag.BoolVar(&sessionLifecycle, "session-lifecycle", false, "verify Gemini/OpenCode native resume and adapter backup/delete against the local fake model")
	flag.Parse()
	if sessionLifecycle && *only != "gemini-cli" && *only != "opencode" {
		log.Fatal("-session-lifecycle requires -only gemini-cli or opencode")
	}
	if (sessionLifecycle || sessionLinks) && (geminiProviderRoundtrip || openCodeProviderRoundtrip) {
		log.Fatal("record lifecycle and providers separately")
	}
	if sessionLinks && *only != "opencode" && *only != "crush" && *only != "opencode,crush" {
		log.Fatal("-session-links requires -only opencode, crush or opencode,crush")
	}
	if geminiProviderRoundtrip && *only != "gemini-cli" {
		log.Fatal("-gemini-provider-roundtrip requires -only gemini-cli")
	}
	if openCodeProviderRoundtrip && *only != "opencode" {
		log.Fatal("-opencode-provider-roundtrip requires -only opencode")
	}
	for _, id := range strings.Split(*only, ",") {
		if id != "" && !slices.ContainsFunc(recorders, func(r recorder) bool { return r.id == id }) {
			log.Fatalf("unknown recorder %q", id)
		}
	}
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
			if *only != "" {
				failed = true
			}
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
	if *claudeLimits {
		if dir, err := recordClaudeLimits(*out); err != nil {
			log.Printf("claude limits: %v", err)
			failed = true
		} else {
			log.Printf("claude limits: recorded %s", dir)
		}
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

// Keep the original session recording; this supplementary proof exercises native persisted settings.
func recordGeminiProviders(r *run) error {
	if os.Getenv("GEMINI_CLI_HOME") != "" {
		return fmt.Errorf("unset GEMINI_CLI_HOME before recording isolated Gemini provider evidence")
	}
	g := agent.NewGemini(r.home)
	g.ProviderState = filepath.Join(r.home, "evidence", "ownership.json")
	envPath := filepath.Join(r.home, ".gemini", ".env")
	beforeEnv := "# Foreign comment\nOTHER='preserve # value'\nGOOGLE_GEMINI_BASE_URL='" + r.base + "'\nGEMINI_MODEL=gemini-2.5-flash\nGEMINI_API_KEY=fixture-native-key\n"
	if err := write(envPath, beforeEnv); err != nil {
		return err
	}
	unusedHome := filepath.Join(r.home, "unused-home")
	if err := os.MkdirAll(unusedHome, 0o700); err != nil {
		return err
	}
	r.env = slices.DeleteFunc(r.env, func(v string) bool { return strings.HasPrefix(v, "HOME=") })
	r.env = append(r.env, "GEMINI_CLI_NO_RELAUNCH=1", "HOME="+unusedHome, "GEMINI_CLI_HOME="+r.home)
	native := func() error {
		return r.cli("gemini", "-p", prompt, "--skip-trust", "--allowed-tools", "run_shell_command")
	}
	if err := native(); err != nil {
		return err
	}
	before, err := os.ReadFile(g.ProviderFile())
	if err != nil {
		return err
	}
	version, err := versionIn("gemini", r)
	if err != nil {
		return err
	}
	type snapshot struct {
		Settings json.RawMessage
		Env      string
	}
	type probe struct {
		Credential                 string
		Applied, Cleared           snapshot
		AppliedModel, ClearedModel string
	}
	proof := struct {
		Version, OS, Arch, Recorded, How string
		Before                           snapshot
		Probes                           []probe
	}{Version: version, OS: runtime.GOOS, Arch: runtime.GOARCH, Recorded: time.Now().UTC().Format("2006-01-02"), How: "Adapter-generated settings and dotenv accepted by Gemini CLI with no model/endpoint/key flags; native GenerateContent inference before and after each apply/clear. The fake endpoint validates x-goog-api-key for the requested model. HOME and workspace are temporary; GEMINI_CLI_HOME differs from HOME and native sessions must appear under its .gemini directory. --skip-trust is native ephemeral automation consent, not a stored trust record. Keys are synthetic; native login is not read.", Before: snapshot{before, beforeEnv}}
	snapshotNow := func() (snapshot, error) {
		a, e := os.ReadFile(g.ProviderFile())
		if e != nil {
			return snapshot{}, e
		}
		b, e := os.ReadFile(envPath)
		return snapshot{a, string(b)}, e
	}
	for _, credential := range []string{"token", "environment"} {
		p := agent.ProviderProfile{Name: "fixture-gemini", BaseURL: r.base, Model: "gemini-2.5-pro", WireAPI: "gemini", Token: "fixture-managed-key"}
		if credential == "environment" {
			p.Token = ""
			p.EnvKey = "GEMINI_API_KEY"
			p.Model = "gemini-2.0-flash"
			r.env = append(r.env, "GEMINI_API_KEY=fixture-environment-key")
		}
		if _, err := g.ValidateProvider(p); err != nil {
			return err
		}
		if _, err := g.PreviewProvider(&p); err != nil {
			return err
		}
		if err := g.ApplyProvider(p, filepath.Join(r.home, "evidence", "backups")); err != nil {
			return err
		}
		applied, err := snapshotNow()
		if err != nil {
			return err
		}
		if err := native(); err != nil {
			return fmt.Errorf("%s apply: %w", credential, err)
		}
		appliedModel, err := recordedGeminiModel(r)
		if err != nil || appliedModel != p.Model {
			return fmt.Errorf("native Gemini applied model mismatch: %s (%v)", appliedModel, err)
		}
		if err := g.ClearProvider(filepath.Join(r.home, "evidence", "backups")); err != nil {
			return err
		}
		cleared, err := snapshotNow()
		if err != nil {
			return err
		}
		var a, b any
		_ = json.Unmarshal(before, &a)
		_ = json.Unmarshal(cleared.Settings, &b)
		aj, _ := json.Marshal(a)
		bj, _ := json.Marshal(b)
		if !bytes.Equal(aj, bj) || cleared.Env != beforeEnv {
			return fmt.Errorf("Gemini clear changed foreign configuration")
		}
		r.env = slices.DeleteFunc(r.env, func(v string) bool { return strings.HasPrefix(v, "GEMINI_API_KEY=") })
		if err := native(); err != nil {
			return fmt.Errorf("%s clear: %w", credential, err)
		}
		clearedModel, err := recordedGeminiModel(r)
		if err != nil || clearedModel != "gemini-2.5-flash" {
			return fmt.Errorf("native Gemini restored model mismatch: %s (%v)", clearedModel, err)
		}
		proof.Probes = append(proof.Probes, probe{credential, applied, cleared, appliedModel, clearedModel})
	}
	data, err := json.MarshalIndent(proof, "", "  ")
	if err != nil {
		return err
	}
	return write(filepath.Join(r.home, "evidence", "gemini-providers.json"), string(data)+"\n")
}

func recordedGeminiModel(r *run) (string, error) {
	files, err := filepath.Glob(filepath.Join(r.home, ".gemini", "tmp", "*", "chats", "session-*"))
	if err != nil {
		return "", err
	}
	var model, latest string
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		for _, line := range bytes.Split(data, []byte("\n")) {
			var msg struct {
				Type, Model, Timestamp string
				Content                json.RawMessage
			}
			if json.Unmarshal(line, &msg) != nil || msg.Type != "gemini" || msg.Timestamp < latest {
				continue
			}
			var text string
			if json.Unmarshal(msg.Content, &text) == nil && text == reply {
				model, latest = msg.Model, msg.Timestamp
			}
		}
	}
	if model == "" {
		return "", fmt.Errorf("native Gemini wrote no final assistant model")
	}
	return model, nil
}

// These snapshots are adapter-generated inputs accepted by the native CLI;
// successful local inference is required after every apply and clear.
func recordOpenCodeProviders(r *run) error {
	o := &agent.OpenCode{Home: r.home, ProviderState: filepath.Join(r.home, "evidence", "ownership.json")}
	if err := r.cli("opencode", "run", "--standalone", prompt); err != nil {
		return err
	}
	before, err := os.ReadFile(o.ProviderFile())
	if err != nil {
		return err
	}
	version, err := versionIn("opencode", r)
	if err != nil {
		return err
	}
	type probe struct {
		Credential                 string `json:"credential"`
		Applied, Cleared           json.RawMessage
		AppliedModel, ClearedModel string
	}
	proof := struct {
		Version, OS, Arch, Recorded, How string
		Before                           json.RawMessage
		Probes                           []probe
	}{Version: version, OS: runtime.GOOS, Arch: runtime.GOARCH, Recorded: time.Now().UTC().Format("2006-01-02"),
		How: "Adapter-generated config accepted by the real OpenCode CLI; opencode run --standalone succeeds against the local fake model before and after each apply/clear. The fake endpoint verifies synthetic Authorization headers. Credentials and costs are synthetic; native login is not read.", Before: before}
	r.env = append(r.env, "FIXTURE_OPEN_CODE_KEY=fixture-environment-key")
	for _, credential := range []string{"token", "environment", "keyless"} {
		p := agent.ProviderProfile{Name: "fixture-managed", BaseURL: r.base + "/v1", Model: "managed-fixture-model-" + credential, WireAPI: "chat"}
		if credential == "token" {
			p.Token = "fixture-managed-key"
		}
		if credential == "environment" {
			p.EnvKey = "FIXTURE_OPEN_CODE_KEY"
		}
		if _, err := o.ValidateProvider(p); err != nil {
			return err
		}
		if _, err := o.PreviewProvider(&p); err != nil {
			return err
		}
		if err := o.ApplyProvider(p, filepath.Join(r.home, "evidence", "backups")); err != nil {
			return err
		}
		applied, err := os.ReadFile(o.ProviderFile())
		if err != nil {
			return err
		}
		if err := r.cli("opencode", "run", "--standalone", prompt); err != nil {
			return fmt.Errorf("%s provider: %w", credential, err)
		}
		appliedModel, err := recordedOpenCodeModel(r)
		if err != nil || appliedModel != p.Model {
			return fmt.Errorf("%s native applied model mismatch: %s (%v)", credential, appliedModel, err)
		}
		if err := o.ClearProvider(filepath.Join(r.home, "evidence", "backups")); err != nil {
			return err
		}
		cleared, err := os.ReadFile(o.ProviderFile())
		if err != nil {
			return err
		}
		var a, b any
		_ = json.Unmarshal(before, &a)
		_ = json.Unmarshal(cleared, &b)
		aJSON, _ := json.Marshal(a)
		bJSON, _ := json.Marshal(b)
		if !bytes.Equal(aJSON, bJSON) {
			return fmt.Errorf("%s clear changed foreign configuration", credential)
		}
		if err := r.cli("opencode", "run", "--standalone", prompt); err != nil {
			return fmt.Errorf("%s restored provider: %w", credential, err)
		}
		clearedModel, err := recordedOpenCodeModel(r)
		if err != nil || clearedModel != "fake-model" {
			return fmt.Errorf("%s native restored model mismatch: %s (%v)", credential, clearedModel, err)
		}
		proof.Probes = append(proof.Probes, probe{credential, applied, cleared, appliedModel, clearedModel})
	}
	data, err := json.MarshalIndent(proof, "", "  ")
	if err != nil {
		return err
	}
	return write(filepath.Join(r.home, "evidence", "opencode-providers.json"), string(data)+"\n")
}

func recordedOpenCodeModel(r *run) (string, error) {
	raw, err := exec.Command("sqlite3", "-json", "-readonly", filepath.Join(r.home, ".local", "share", "opencode", "opencode.db"),
		`SELECT json_extract(data,'$.model.id') AS model FROM session_message
		WHERE type='assistant' AND session_id=(SELECT id FROM session_v2 ORDER BY time_created DESC LIMIT 1)
		ORDER BY seq DESC LIMIT 1`).Output()
	if err != nil {
		return "", err
	}
	var rows []struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return "", err
	}
	if len(rows) != 1 {
		return "", fmt.Errorf("native CLI recorded no assistant model")
	}
	return rows[0].Model, nil
}

func writeOrigin(dest, agent, version, how, by string) error {
	origin, _ := json.MarshalIndent(map[string]string{
		"agent": agent, "version": version, "recorded": time.Now().UTC().Format("2006-01-02"), "how": how, "by": by,
		"os": runtime.GOOS, "arch": runtime.GOARCH,
	}, "", "  ")
	return write(filepath.Join(dest, "origin.json"), string(origin)+"\n")
}

func record(rec recorder, base, out string) (string, error) {
	tmp, err := os.MkdirTemp("", "lazyagents-fixture-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	home, work := filepath.Join(tmp, "home"), filepath.Join(tmp, "work", "proj")
	if rec.workInHome {
		work = filepath.Join(home, "work", "proj")
	}
	runDir := filepath.Join(tmp, "run") // XDG_RUNTIME_DIR: sockets (Crush's server) stay in the throwaway dir
	if err := os.MkdirAll(work, 0o755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		return "", err
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return "", err
	}
	defer exec.Command("pkill", "-f", tmp).Run() // daemons some CLIs leave behind
	if err := rec.setup(home, base); err != nil {
		return "", err
	}
	r := &run{home: home, work: work, base: base, env: []string{
		"HOME=" + home, "PATH=" + os.Getenv("PATH"), "TERM=dumb", "XDG_RUNTIME_DIR=" + runDir,
		"XDG_CONFIG_HOME=" + filepath.Join(home, ".config"), "XDG_DATA_HOME=" + filepath.Join(home, ".local", "share"),
		"XDG_CACHE_HOME=" + filepath.Join(home, ".cache"), "XDG_STATE_HOME=" + filepath.Join(home, ".local", "state"),
		"TMPDIR=" + tmp, "TMP=" + tmp, "TEMP=" + tmp,
	}}
	version, err := versionIn(rec.bin, r)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(out, rec.id, version)
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		return "", fmt.Errorf("fixture destination already exists or cannot be inspected: %s (record to a new -out directory)", dest)
	}
	if err := rec.run(r); err != nil {
		return "", err
	}
	var lifecycle *sessionProof
	if sessionLifecycle {
		lifecycle, err = recordSessionResume(rec.id, r)
		if err != nil {
			return "", err
		}
	}
	var files []string
	for _, g := range rec.collect {
		m, _ := filepath.Glob(filepath.Join(home, g))
		files = append(files, m...)
	}
	if len(files) == 0 {
		return "", errors.New("the CLI wrote no session file")
	}
	for _, g := range rec.config {
		m, _ := filepath.Glob(filepath.Join(home, g))
		files = append(files, m...)
	}
	// Publish only a completely sanitized recording; older evidence is never removed.
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(filepath.Dir(dest), ".recording-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
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
		data = bytes.ReplaceAll(data, []byte(base), []byte("http://127.0.0.1:18765"))
		clean, err := san.clean(rec.id, data, strings.HasSuffix(f, ".jsonl"))
		if err != nil {
			return "", fmt.Errorf("%s: %w", rel, err)
		}
		if err := write(filepath.Join(stage, "home", san.path(rel)), string(clean)); err != nil {
			return "", err
		}
	}
	if sessionLinks {
		rec.notes = "Native child delegation (the child runs a shell tool only when its native tool set permits it); read-only SQL dump retains native parent, message and call IDs. Temporary HOME and local OpenAI-compatible model only; configuration is recorder input."
	}
	if lifecycle != nil {
		if err := recordSessionDelete(rec.id, r, lifecycle); err != nil {
			return "", err
		}
		raw, _ := json.MarshalIndent(lifecycle, "", "  ")
		clean, err := san.clean(rec.id, raw, false)
		if err != nil {
			return "", err
		}
		if err := write(filepath.Join(stage, "lifecycle.json"), string(clean)+"\n"); err != nil {
			return "", err
		}
		rec.notes += "; native resume added a user turn to the same ID before this snapshot; adapter backup/delete then ran in the isolated home, see lifecycle.json"
	}
	if err := writeOrigin(stage, rec.id, version, rec.notes, "scripts/record-fixtures.go against its local fake model; collected configuration files are recorder inputs unless noted"); err != nil {
		return "", err
	}
	return dest, os.Rename(stage, dest)
}

// recordClaudeLimits pins the subscription limits response. It makes the one
// call lazyagents' Usage tab makes, through agent.Claude.RateLimits itself,
// pointed at an in-process proxy that forwards to the real endpoint and keeps
// the response body: the login is used for that call only and never stored.
// Only the fields the adapter reads are kept, and every value in them is
// replaced; keys, types and enums remain.
func recordClaudeLimits(out string) (string, error) {
	version, err := cliVersion("claude")
	if err != nil {
		return "", err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	var body []byte
	noRedirect := &http.Client{Timeout: 15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	proxy := http.NewServeMux()
	proxy.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		req, err := http.NewRequestWithContext(r.Context(), r.Method, agent.ClaudeUsageURL, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		for _, h := range []string{"Authorization", "Anthropic-Beta", "Accept"} {
			req.Header.Set(h, r.Header.Get(h))
		}
		resp, err := noRedirect.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		body, _ = io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(body)
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer ln.Close()
	go http.Serve(ln, proxy)
	c := agent.NewClaude(home)
	c.UsageURL = "http://" + ln.Addr().String() + "/"
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := c.RateLimits(ctx); err != nil {
		return "", fmt.Errorf("the real call failed, nothing recorded: %w", err)
	}
	var full map[string]any
	if err := json.Unmarshal(body, &full); err != nil {
		return "", fmt.Errorf("response is not a JSON object: %w", err)
	}
	// Only the fields the adapter reads: the rest of this private endpoint is
	// internal (codenamed feature flags) and has no place in a public repo.
	shape := map[string]any{}
	for _, k := range []string{"limits", "five_hour", "seven_day"} {
		if v, ok := full[k]; ok {
			shape[k] = syntheticClaude("", k, v)
		}
	}
	data, _ := json.MarshalIndent(shape, "", "  ")
	san := newSanitizer("/nonexistent-home", "/nonexistent-work")
	clean, err := san.clean("claude-code", append(data, '\n'), false)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(out, "claude-code", version)
	if err := write(filepath.Join(dest, "limits.json"), string(clean)); err != nil {
		return "", err
	}
	const note = "; limits.json has the shape of a real /api/oauth/usage response, with synthetic values"
	originPath := filepath.Join(dest, "origin.json")
	var origin map[string]string
	if raw, err := os.ReadFile(originPath); err == nil && json.Unmarshal(raw, &origin) == nil {
		if !strings.Contains(origin["how"], note) {
			origin["how"] += note
		}
		raw, _ = json.MarshalIndent(origin, "", "  ")
		return dest, write(originPath, string(raw)+"\n")
	}
	return dest, writeOrigin(dest, "claude-code", version, "limits only"+note, "scripts/record-fixtures.go -claude-limits")
}

// syntheticClaude keeps keys, types and the enum-like strings the adapter
// matches on, and replaces every other value: nothing about the account's
// usage, plan or identity survives.
func syntheticClaude(parent, key string, v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			x[k] = syntheticClaude(key, k, e)
		}
		return x
	case []any:
		for i, e := range x {
			x[i] = syntheticClaude(parent, key, e)
		}
		return x
	case float64:
		if key == "percent" || key == "utilization" {
			return 42.0
		}
		return 0.0
	case string:
		switch {
		case key == "kind", key == "severity", key == "type", key == "status":
			return x
		case key == "display_name" && parent == "model": // a model name, not the account's
			return x
		case key == "resets_at":
			return "2100-01-01T05:00:00+00:00" // never already reset
		}
		return "x"
	}
	return v // bool, null
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
				m[k] = map[string]float64{"primary": 4102462800, "secondary": 4102992000}[parent] // 2100: never already reset
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
	return versionIn(bin, nil)
}

func versionIn(bin string, r *run) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "--version")
	cmd.WaitDelay = 2 * time.Second
	if r != nil {
		cmd.Dir, cmd.Env = r.work, r.env
	}
	out, err := cmd.Output()
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
	const fakeHome = "/home/user"
	fakeWork := "/work/proj"
	if rel, err := filepath.Rel(home, work); err == nil && !strings.HasPrefix(rel, "..") {
		fakeWork = fakeHome + "/" + filepath.ToSlash(rel) // a project kept inside home
	}
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
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}
			var v any
			if err := json.Unmarshal(line, &v); err != nil {
				return nil, fmt.Errorf("invalid JSONL recording: %w", err)
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
	if openCodeProviderRoundtrip && strings.HasSuffix(path, "/chat/completions") {
		keys := map[string]string{"fake-model": "fixture-native-key", "managed-fixture-model-token": "fixture-managed-key", "managed-fixture-model-environment": "fixture-environment-key", "managed-fixture-model-keyless": "lazyagents-no-key"}
		model, _ := req["model"].(string)
		key, ok := keys[model]
		if !ok || r.Header.Get("Authorization") != "Bearer "+key {
			http.Error(w, "fixture authentication mismatch", http.StatusUnauthorized)
			return
		}
	}
	if geminiProviderRoundtrip && (strings.HasSuffix(path, ":generateContent") || strings.HasSuffix(path, ":streamGenerateContent")) {
		keys := map[string]string{"gemini-2.5-flash": "fixture-native-key", "gemini-2.5-pro": "fixture-managed-key", "gemini-2.0-flash": "fixture-environment-key"}
		model := strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(path, "/v1beta/models/"), ":streamGenerateContent"), ":generateContent")
		key, ok := keys[model]
		if !ok || r.Header.Get("x-goog-api-key") != key {
			http.Error(w, "fixture authentication mismatch", http.StatusUnauthorized)
			return
		}
	}
	switch {
	case strings.HasSuffix(path, "/models"):
		writeJSON(w, map[string]any{"object": "list", "data": []any{map[string]any{"id": "fake-model", "object": "model"}}})
	case strings.HasSuffix(path, "/count_tokens"):
		writeJSON(w, map[string]any{"input_tokens": 100})
	case strings.HasSuffix(path, ":countTokens"):
		writeJSON(w, map[string]any{"totalTokens": 100})
	case strings.HasSuffix(path, ":streamGenerateContent"), strings.HasSuffix(path, ":generateContent"):
		generateContent(w, r, req)
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

// Delegation is requested only by the parent prompt; the child receives the shell prompt.
func fixtureTool(name string, schema any, msgs []any) (map[string]any, bool) {
	if sessionLinks && len(msgs) > 0 {
		last, _ := json.Marshal(msgs[len(msgs)-1])
		if bytes.Contains(last, []byte("delegate fixture please")) {
			switch name {
			case "agent":
				return map[string]any{"prompt": prompt}, true
			case "subagent":
				return map[string]any{"agent": "explore", "description": "Print fixture", "prompt": prompt}, true
			default:
				return nil, false
			}
		}
	}
	return shellTool(name, schema)
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

// Gemini GenerateContent uses SSE without the OpenAI [DONE] sentinel.
func generateContent(w http.ResponseWriter, r *http.Request, req map[string]any) {
	parts := []any{map[string]any{"text": reply}}
	contents := list(req["contents"])
	if len(contents) > 0 && dig(contents[len(contents)-1], "role") == "user" {
		toolResult := false
		for _, p := range list(dig(contents[len(contents)-1], "parts")) {
			if dig(p, "functionResponse") != nil {
				toolResult = true
			}
		}
		if !toolResult {
			for _, tool := range list(req["tools"]) {
				for _, f := range list(dig(tool, "functionDeclarations")) {
					name, _ := dig(f, "name").(string)
					if args, ok := shellTool(name, dig(f, "parameters")); ok {
						parts = []any{map[string]any{"functionCall": map[string]any{"name": name, "args": args}}}
						break
					}
				}
			}
		}
	}
	v := map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"role": "model", "parts": parts}, "finishReason": "STOP", "index": 0}},
		"usageMetadata": map[string]any{"promptTokenCount": 120, "candidatesTokenCount": 8, "totalTokenCount": 128}, "modelVersion": "gemini-2.5-flash"}
	if strings.HasSuffix(r.URL.Path, ":generateContent") {
		writeJSON(w, v)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	raw, _ := json.Marshal(v)
	fmt.Fprintf(w, "data: %s\n\n", raw)
}

// OpenAI Chat Completions (pi, OpenCode).
func chat(w http.ResponseWriter, req map[string]any) {
	msgs := list(req["messages"])
	toolTurn := len(msgs) > 0 && dig(msgs[len(msgs)-1], "role") == "user"
	var name string
	var args map[string]any
	if toolTurn {
		for _, t := range list(req["tools"]) {
			n, _ := dig(t, "function", "name").(string)
			if a, ok := fixtureTool(n, dig(t, "function", "parameters"), msgs); ok {
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
	// the fields the real API reports: cache writes split by TTL, tier, speed, region
	usage := map[string]any{"input_tokens": 50, "output_tokens": 6, "cache_read_input_tokens": 100,
		"cache_creation_input_tokens": 40, "cache_creation": map[string]any{"ephemeral_5m_input_tokens": 10, "ephemeral_1h_input_tokens": 30},
		"service_tier": "standard", "speed": "standard", "inference_geo": "not_available"}
	if block != nil {
		stop = "tool_use"
		usage = map[string]any{"input_tokens": 120, "output_tokens": 8, "cache_read_input_tokens": 0,
			"cache_creation_input_tokens": 0, "cache_creation": map[string]any{"ephemeral_5m_input_tokens": 0, "ephemeral_1h_input_tokens": 0},
			"service_tier": "standard", "speed": "standard", "inference_geo": "not_available"}
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
			"cache_read_input_tokens": usage["cache_read_input_tokens"], "cache_creation_input_tokens": usage["cache_creation_input_tokens"],
			"cache_creation": usage["cache_creation"], "service_tier": usage["service_tier"], "inference_geo": usage["inference_geo"]}}}
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

// The recording is captured before deletion; this proof describes the later operation.
type sessionProof struct {
	Agent, Version, OS, Arch, Recorded, How string
	ID                                      string
	Resume                                  []string
	BeforeUsers, AfterUsers                 int
	BackupCount                             int
	BackupIDs                               []string
	BackupPrivate, Deleted                  bool
	session                                 agent.Session
	adapter                                 agent.Adapter
}

func recordSessionResume(id string, r *run) (*sessionProof, error) {
	if os.Getenv("GEMINI_CLI_HOME") != "" {
		return nil, errors.New("unset GEMINI_CLI_HOME before recording lifecycle evidence")
	}
	var ad agent.Adapter
	switch id {
	case "gemini-cli":
		ad = agent.NewGemini(r.home)
	case "opencode":
		ad = &agent.OpenCode{Home: r.home, Look: exec.LookPath}
	default:
		return nil, errors.New("unsupported lifecycle recorder")
	}
	sessions, err := ad.ListSessions()
	if err != nil || len(sessions) != 1 {
		return nil, fmt.Errorf("expected one native parent session: %v", err)
	}
	s := sessions[0]
	count := func() (int, error) {
		entries, err := ad.Transcript(s)
		n := 0
		for _, e := range entries {
			if e.Role == agent.RoleUser {
				n++
			}
		}
		return n, err
	}
	before, err := count()
	if err != nil {
		return nil, err
	}
	argv, _, ok := ad.ResumeCmd(s)
	if !ok {
		return nil, errors.New("adapter cannot resume")
	}
	var args []string
	if id == "gemini-cli" {
		args = append(slices.Clone(argv[1:]), "-p", "resume fixture please", "--model", "gemini-2.5-flash", "--skip-trust", "--allowed-tools", "run_shell_command")
	} else {
		// The headless run command accepts the same native --session ID as the TUI.
		args = append([]string{"run"}, argv[1:]...)
		args = append(args, "--standalone", "resume fixture please")
	}
	if err := r.cli(argv[0], args...); err != nil {
		return nil, err
	}
	after, err := count()
	if err != nil || after != before+1 {
		return nil, fmt.Errorf("resume did not add exactly one native user turn: %d -> %d: %v", before, after, err)
	}
	sessions, err = ad.ListSessions()
	if err != nil || len(sessions) != 1 || sessions[0].ID != s.ID {
		return nil, errors.New("resume created or replaced the parent session")
	}
	version, err := versionIn(argv[0], r)
	if err != nil {
		return nil, err
	}
	return &sessionProof{Agent: id, Version: version, OS: runtime.GOOS, Arch: runtime.GOARCH, Recorded: time.Now().UTC().Format("2006-01-02"),
		How: "Native headless resume using the adapter's session ID/flag; same ID and one added user turn verified. Snapshot captured after resume, before adapter deletion. Temporary HOME, allowlisted environment and local fake model only. OpenCode uses headless run instead of its interactive TUI. When recording Gemini, --skip-trust is ephemeral native consent, never a stored trust record.",
		ID:  s.ID, Resume: append([]string{argv[0]}, args...), BeforeUsers: before, AfterUsers: after, session: s, adapter: ad}, nil
}

func recordSessionDelete(id string, r *run, proof *sessionProof) error {
	if o, ok := proof.adapter.(*agent.OpenCode); ok {
		if runtime.GOOS == "windows" {
			return errors.New("isolated native deletion recorder currently requires POSIX env")
		}
		bin, err := exec.LookPath("opencode")
		if err != nil {
			return err
		}
		quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
		script := "#!/bin/sh\nexec env -i"
		for _, v := range r.env {
			script += " " + quote(v)
		}
		script += " " + quote(bin) + " \"$@\"\n"
		wrapper := filepath.Join(r.home, "native-opencode")
		if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
			return err
		}
		o.Look = func(name string) (string, error) {
			if name == "opencode" {
				return wrapper, nil
			}
			return exec.LookPath(name)
		}
	}
	var nativeIDs []string
	if id == "opencode" {
		sqlite, err := exec.LookPath("sqlite3")
		if err != nil {
			return err
		}
		raw, err := exec.Command(sqlite, "-readonly", proof.session.Path, "SELECT id FROM session_v2 ORDER BY id").Output()
		if err != nil {
			return err
		}
		nativeIDs = strings.Fields(string(raw))
	}
	backups := filepath.Join(r.home, "delete-backups")
	var before []byte
	if id == "gemini-cli" {
		before, _ = os.ReadFile(proof.session.Path)
	}
	if err := proof.adapter.DeleteSession(proof.session, backups); err != nil {
		return err
	}
	files, err := os.ReadDir(backups)
	if err != nil || len(files) == 0 {
		return errors.New("native session deletion produced no backup")
	}
	proof.BackupPrivate = true
	for _, f := range files {
		path := filepath.Join(backups, f.Name())
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		proof.BackupPrivate = proof.BackupPrivate && info.Mode().Perm() == 0o600
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if id == "gemini-cli" {
			if !bytes.Equal(raw, before) {
				return errors.New("Gemini backup differs from original transcript")
			}
			proof.BackupIDs = append(proof.BackupIDs, proof.ID)
		} else {
			var export struct {
				Info struct {
					ID string `json:"id"`
				} `json:"info"`
				Messages []json.RawMessage `json:"messages"`
			}
			if json.Unmarshal(raw, &export) != nil || export.Info.ID == "" || len(export.Messages) == 0 {
				return errors.New("invalid native OpenCode session backup")
			}
			proof.BackupIDs = append(proof.BackupIDs, export.Info.ID)
		}
	}
	sessions, err := proof.adapter.ListSessions()
	if err != nil || len(sessions) != 0 {
		return fmt.Errorf("deleted session remains listed: %v", err)
	}
	if id == "opencode" {
		sqlite, err := exec.LookPath("sqlite3")
		if err != nil {
			return err
		}
		raw, err := exec.Command(sqlite, "-readonly", proof.session.Path, "SELECT count(*) FROM session_v2").Output()
		if err != nil || strings.TrimSpace(string(raw)) != "0" {
			return errors.New("native OpenCode deletion left child sessions")
		}
	}
	if !proof.BackupPrivate {
		return errors.New("session backup is not private")
	}
	slices.Sort(proof.BackupIDs)
	if id == "opencode" && !slices.Equal(nativeIDs, proof.BackupIDs) {
		return errors.New("not every native parent/child session has a backup")
	}
	proof.BackupCount, proof.Deleted = len(files), true
	return nil
}
