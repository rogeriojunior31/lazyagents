package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

const geminiProviderStart = "\n# lazyagents provider start\n"
const geminiProviderEnd = "# lazyagents provider end\n"
const geminiAPIAuth = "gemini-api-key"

// Original credentials stay in the untouched dotenv prefix, never in ownership state.
type geminiProviderState struct {
	Path            string   `json:"path,omitempty"`
	Blocks          []string `json:"blocks,omitempty"`
	Auth            []string `json:"auth,omitempty"`
	PreviousAuth    *string  `json:"previousAuth,omitempty"`
	CreatedFile     bool     `json:"createdFile,omitempty"`
	CreatedSecurity bool     `json:"createdSecurity,omitempty"`
	CreatedAuth     bool     `json:"createdAuth,omitempty"`
	CreatedEnv      bool     `json:"createdEnv,omitempty"`
}

func (g *Gemini) ProviderFile() string    { return filepath.Join(g.configDir(), "settings.json") }
func (g *Gemini) providerEnvFile() string { return filepath.Join(g.configDir(), ".env") }

func geminiObject(data []byte) (*object, error) {
	if len(data) > 0 && !json.Valid(data) {
		return nil, fmt.Errorf("Gemini settings require strict JSON (values hidden)")
	}
	obj, err := decodeObject(data)
	if err != nil {
		return nil, fmt.Errorf("Gemini settings require a JSON object (values hidden)")
	}
	seen := map[string]bool{}
	for _, key := range obj.keys() {
		if seen[key] {
			return nil, fmt.Errorf("duplicate Gemini setting keys require native review (values hidden)")
		}
		seen[key] = true
	}
	return obj, nil
}
func geminiChild(parent *object, key string) (*object, bool, error) {
	raw, exists := parent.raw(key)
	if !exists {
		return &object{}, false, nil
	}
	obj, err := geminiObject(raw)
	return obj, true, err
}

var geminiEnvLine = regexp.MustCompile(`^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$`)

// The supported dotenv subset is deliberately single-line. Never source or expand it.
func geminiEnv(data []byte) (map[string]string, error) {
	values := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := geminiEnvLine.FindStringSubmatch(line)
		if parts == nil || strings.ContainsRune(line, '\x00') {
			return nil, fmt.Errorf("Gemini dotenv requires single-line assignments or comments (values hidden)")
		}
		v := strings.TrimSpace(parts[2])
		if len(v) > 0 && strings.ContainsRune("\"'`", rune(v[0])) {
			quote := v[0]
			end := strings.IndexByte(v[1:], quote)
			if end < 0 {
				return nil, fmt.Errorf("Gemini multiline or escaped dotenv values require native review (values hidden)")
			}
			end++
			rest := strings.TrimSpace(v[end+1:])
			if rest != "" && !strings.HasPrefix(rest, "#") || quote == '"' && strings.Contains(v[1:end], "\\") {
				return nil, fmt.Errorf("Gemini complex dotenv quoting requires native review (values hidden)")
			}
			v = v[1:end]
		} else if before, _, ok := strings.Cut(v, "#"); ok {
			v = strings.TrimSpace(before)
		}
		values[parts[1]] = v
	}
	return values, nil
}

func geminiBlock(data []byte) (prefix, block []byte, err error) {
	start := bytes.Index(data, []byte(geminiProviderStart))
	if start < 0 {
		if bytes.Contains(data, []byte("# lazyagents provider")) {
			return nil, nil, fmt.Errorf("Gemini provider markers were edited; review the dotenv file locally (values hidden)")
		}
		return data, nil, nil
	}
	tail := data[start:]
	if bytes.Count(data, []byte(geminiProviderStart)) != 1 || bytes.Count(data, []byte(geminiProviderEnd)) != 1 || !bytes.HasSuffix(tail, []byte(geminiProviderEnd)) {
		return nil, nil, fmt.Errorf("Gemini provider block must be unique and last; review the dotenv file locally (values hidden)")
	}
	return data[:start], tail, nil
}

func geminiProfile(p ProviderProfile) error {
	if err := ValidateProviderProfile(p); err != nil {
		return err
	}
	if p.WireAPI != "" && p.WireAPI != "gemini" {
		return fmt.Errorf("Gemini supports only GenerateContent (wireApi gemini); Chat Completions, Responses and Vertex AI are unsupported")
	}
	u, _ := url.Parse(p.BaseURL)
	if u == nil || u.Hostname() == "" || u.RawQuery != "" {
		return fmt.Errorf("Gemini requires a base URL without query parameters")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && slices.Contains([]string{"localhost", "127.0.0.1", "::1"}, u.Hostname())) {
		return fmt.Errorf("Gemini requires HTTPS except for localhost endpoints")
	}
	if p.Model == "" || strings.ContainsAny(p.Model, "'\"\r\n\x00") {
		return fmt.Errorf("Gemini requires an explicit single-line model ID")
	}
	if p.EnvKey != "" && p.EnvKey != "GEMINI_API_KEY" {
		return fmt.Errorf("Gemini only supports the native GEMINI_API_KEY variable; dotenv does not expand named references")
	}
	if p.Token == "" && !p.HasToken && p.EnvKey == "" {
		return fmt.Errorf("Gemini requires a saved API key or envKey GEMINI_API_KEY; keyless connections and native login are not profile credentials")
	}
	if strings.ContainsAny(p.Token+p.BaseURL, "'\r\n\x00") {
		return fmt.Errorf("Gemini provider values must fit single-quoted single-line dotenv assignments (values hidden)")
	}
	return nil
}

type geminiProviderEdit struct {
	settings                             *settings
	env, afterEnv                        []byte
	envExists, removeEnv, removeSettings bool
	state, pending                       *geminiProviderState
}

func (g *Gemini) loadProviderState() (*geminiProviderState, error) {
	st := &geminiProviderState{}
	if g.ProviderState == "" {
		if g.providerMem != nil {
			data, _ := json.Marshal(g.providerMem)
			_ = json.Unmarshal(data, st)
		}
		return st, nil
	}
	data, err := os.ReadFile(g.ProviderState)
	if errors.Is(err, os.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading Gemini provider ownership: %w", err)
	}
	if json.Unmarshal(data, st) != nil {
		return nil, fmt.Errorf("invalid Gemini provider ownership (values hidden)")
	}
	return st, nil
}
func (g *Gemini) saveProviderState(st *geminiProviderState) error {
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	if g.ProviderState == "" {
		g.providerMem = &geminiProviderState{}
		return json.Unmarshal(data, g.providerMem)
	}
	return fsutil.WriteAtomic(g.ProviderState, append(data, '\n'), 0o600)
}

func (g *Gemini) prepareProvider(p *ProviderProfile) (*geminiProviderEdit, error) {
	if override := os.Getenv("GEMINI_CLI_HOME"); override != "" && !filepath.IsAbs(override) {
		return nil, fmt.Errorf("Gemini provider control requires an absolute GEMINI_CLI_HOME; relative or literal tilde paths need native review")
	}
	if p != nil {
		if err := geminiProfile(*p); err != nil {
			return nil, err
		}
	}
	for _, path := range ProviderFiles(g) {
		if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
			return nil, fmt.Errorf("Gemini provider destinations must be regular files: %s", path)
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("inspecting Gemini provider destination: %w", err)
		}
	}
	s, err := readSettings(g.ProviderFile())
	if err != nil {
		return nil, fmt.Errorf("Gemini settings cannot be read as JSON (values hidden)")
	}
	s.object, err = geminiObject(s.orig)
	if err != nil {
		return nil, err
	}
	security, hadSecurity, err := geminiChild(s.object, "security")
	if err != nil {
		return nil, err
	}
	auth, hadAuth, err := geminiChild(security, "auth")
	if err != nil {
		return nil, err
	}
	var selected, enforced string
	var external bool
	if _, err := auth.get("selectedType", &selected); err != nil {
		return nil, fmt.Errorf("Gemini selected auth type must be a string (value hidden)")
	}
	if _, err := auth.get("enforcedType", &enforced); err != nil {
		return nil, fmt.Errorf("Gemini enforced auth type must be a string (value hidden)")
	}
	if _, err := auth.get("useExternal", &external); err != nil {
		return nil, fmt.Errorf("Gemini external auth setting must be boolean (value hidden)")
	}
	if raw, exists := auth.raw("selectedType"); p != nil && exists && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, fmt.Errorf("Gemini null authentication selection requires native review (value hidden)")
	}
	if p != nil && selected != "" && !slices.Contains([]string{geminiAPIAuth, "oauth-personal", "vertex-ai", "cloud-shell", "compute-default-credentials", "gateway"}, selected) {
		return nil, fmt.Errorf("unknown Gemini authentication type requires native review (value hidden)")
	}
	if p != nil && (enforced != "" && enforced != geminiAPIAuth || external) {
		return nil, fmt.Errorf("Gemini enforced or external authentication requires native configuration; no files were changed")
	}
	env, err := os.ReadFile(g.providerEnvFile())
	exists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("reading Gemini dotenv: %w", err)
	}
	if _, err := geminiEnv(env); err != nil {
		return nil, err
	}
	if p != nil {
		values, _ := geminiEnv(env)
		for key, supported := range map[string]string{"GEMINI_API_KEY_AUTH_MECHANISM": "x-goog-api-key", "GEMINI_CLI_AUTH_OVERRIDE": geminiAPIAuth, "GOOGLE_GENAI_API_VERSION": "v1beta", "GOOGLE_GENAI_USE_VERTEXAI": "false", "GOOGLE_GENAI_USE_GCA": "false"} {
			value := values[key]
			if inherited, present := os.LookupEnv(key); present {
				value = inherited
			}
			if value != "" && value != supported {
				return nil, fmt.Errorf("Gemini provider control does not support the configured %s; review it locally (value hidden)", key)
			}
		}
	}
	prefix, block, err := geminiBlock(env)
	if err != nil {
		return nil, err
	}
	st, err := g.loadProviderState()
	if err != nil {
		return nil, err
	}
	if len(st.Blocks)+len(st.Auth) > 0 && st.Path != g.ProviderFile() {
		return nil, fmt.Errorf("Gemini provider ownership belongs to another home; clear it using the original GEMINI_CLI_HOME before changing homes")
	}
	if len(block) > 0 && !slices.Contains(st.Blocks, digest(string(block))) {
		return nil, fmt.Errorf("Gemini provider block is unowned or changed externally; review it locally before applying or clearing (values hidden)")
	}
	oldBlocks, oldAuth := slices.Clone(st.Blocks), slices.Clone(st.Auth)
	_, hadSelected := auth.raw("selectedType")
	ownedAuth := hadSelected && slices.Contains(st.Auth, digest(selected))
	after := slices.Clone(prefix)
	if p != nil {
		st.Path = g.ProviderFile()
		if !ownedAuth {
			st.PreviousAuth = nil
			if hadSelected {
				v := selected
				st.PreviousAuth = &v
			}
		}
		if len(st.Blocks) == 0 && len(st.Auth) == 0 {
			st.CreatedFile, st.CreatedSecurity, st.CreatedAuth, st.CreatedEnv = s.missing, !hadSecurity, !hadAuth, !exists
		}
		_ = auth.set("selectedType", geminiAPIAuth)
		block = []byte(geminiProviderStart + "GOOGLE_GEMINI_BASE_URL='" + p.BaseURL + "'\nGEMINI_MODEL='" + p.Model + "'\n")
		if p.Token != "" {
			block = append(block, []byte("GEMINI_API_KEY='"+p.Token+"'\n")...)
		}
		block = append(block, []byte(geminiProviderEnd)...)
		after = append(after, block...)
		st.Blocks, st.Auth = []string{digest(string(block))}, []string{digest(geminiAPIAuth)}
	} else {
		if ownedAuth {
			if st.PreviousAuth == nil {
				auth.delete("selectedType")
			} else {
				_ = auth.set("selectedType", *st.PreviousAuth)
			}
		}
		st.Blocks, st.Auth = nil, nil
	}
	if auth.empty() && st.CreatedAuth {
		security.delete("auth")
	} else if hadAuth || p != nil {
		_ = security.set("auth", auth)
	}
	if security.empty() && st.CreatedSecurity {
		s.delete("security")
	} else if hadSecurity || p != nil {
		_ = s.set("security", security)
	}
	pending := *st
	pending.Blocks = append(slices.Clone(st.Blocks), oldBlocks...)
	pending.Auth = append(slices.Clone(st.Auth), oldAuth...)
	e := &geminiProviderEdit{settings: s, env: env, afterEnv: after, envExists: exists, state: st, pending: &pending}
	e.removeEnv = p == nil && st.CreatedEnv && len(after) == 0
	e.removeSettings = p == nil && st.CreatedFile && !s.missing && s.empty()
	return e, nil
}

func (g *Gemini) ValidateProvider(p ProviderProfile) (ProviderValidation, error) {
	v := providerValidation(ProviderFiles(g), "Gemini uses GenerateContent only (wireApi gemini), with a saved API key or the fixed GEMINI_API_KEY environment variable. Login credentials are never read or removed.")
	v.Notes = append(v.Notes, "The managed dotenv block overrides earlier file assignments and is removed on clear. Shell environment wins over dotenv; a nearer project/parent .env replaces the home dotenv file. Trusted workspaces are required to load .gemini/.env; review trust in Gemini itself.", "Project/system settings, CLI flags, keychain credentials and running processes are not evaluated. Start a new trusted Gemini session; this is a global configuration snapshot.")
	_, err := g.prepareProvider(&p)
	return v, err
}

func (g *Gemini) PreviewProvider(p *ProviderProfile) (ProviderPreview, error) {
	e, err := g.prepareProvider(p)
	if err != nil {
		return ProviderPreview{}, err
	}
	sf, after, err := jsonProviderPreview(e.settings, e.removeSettings)
	if err != nil {
		return ProviderPreview{}, err
	}
	ef, err := previewProviderFile(g.providerEnvFile(), e.env, e.afterEnv, e.envExists, e.removeEnv, geminiEnvChanges)
	state, _ := json.Marshal(e.pending)
	return ProviderPreview{Managed: p == nil && (len(e.pending.Blocks) > 0 || len(e.pending.Auth) > 0), Files: []ProviderFilePreview{sf, ef}, Revision: providerRevision(p, e.settings.orig, after, e.env, e.afterEnv, state, []byte(fmt.Sprint(e.envExists, e.settings.missing)))}, err
}

func geminiEnvChanges(before, after []byte) ([]string, error) {
	a, err := geminiEnv(before)
	if err != nil {
		return nil, err
	}
	b, err := geminiEnv(after)
	if err != nil {
		return nil, err
	}
	return hiddenProviderChanges(a, b), nil
}

func (g *Gemini) ApplyProvider(p ProviderProfile, backupsDir string) error {
	if p.Token == "" && p.EnvKey == "" {
		return fmt.Errorf("Gemini requires the saved API key value or envKey GEMINI_API_KEY before apply")
	}
	return g.writeProvider(&p, backupsDir)
}
func (g *Gemini) ClearProvider(backupsDir string) error { return g.writeProvider(nil, backupsDir) }

// Tests simulate a native settings write after the dotenv save.
var geminiBeforeSettingsSave = func() {}

// Back up both files first; partial writes retain ownership for retry/clear.
func (g *Gemini) writeProvider(p *ProviderProfile, backupsDir string) error {
	e, err := g.prepareProvider(p)
	if err != nil {
		return err
	}
	if p == nil && len(e.pending.Blocks) == 0 && len(e.pending.Auth) == 0 {
		return nil
	}
	if err := g.saveProviderState(e.pending); err != nil {
		return err
	}
	operation := "provider.apply"
	if p == nil {
		operation = "provider.clear"
	}
	origin := fsutil.BackupOrigin{Agent: g.ID(), Operation: operation}
	if backupsDir != "" {
		for _, path := range ProviderFiles(g) {
			backup, err := fsutil.Backup(path, backupsDir, origin)
			if err != nil {
				return err
			}
			if backup != "" {
				if err := os.Chmod(backup, 0o600); err != nil {
					return fmt.Errorf("securing Gemini config backup: %w", err)
				}
			}
			_ = fsutil.RotateBackups(backupsDir, filepath.Base(path)+".", settingsBackups)
		}
	}
	if !bytes.Equal(e.env, e.afterEnv) || p != nil {
		if e.removeEnv {
			err = fsutil.RemoveIfUnchanged(g.providerEnvFile(), e.env)
		} else {
			err = fsutil.WriteAtomicIfUnchanged(g.providerEnvFile(), e.env, e.envExists, e.afterEnv, 0o600)
		}
		if err != nil {
			return fmt.Errorf("writing Gemini dotenv: %w", err)
		}
	}
	geminiBeforeSettingsSave()
	if e.removeSettings {
		err = e.settings.remove("")
	} else if after, _ := e.settings.indented(); (!e.settings.missing || !e.settings.empty()) && (p != nil || !bytes.Equal(e.settings.orig, after)) {
		err = e.settings.save("")
	}
	if err != nil {
		return fmt.Errorf("Gemini dotenv was updated but settings were not; review Recovery or retry: %w", err)
	}
	if p == nil {
		e.state = &geminiProviderState{}
	}
	return g.saveProviderState(e.state)
}

func (g *Gemini) ReadProvider() (ProviderProfile, bool, error) {
	e, err := g.prepareProvider(nil)
	if err != nil {
		return ProviderProfile{}, false, err
	}
	var selected string
	// Read the original selection, since preparation computes a potential clear.
	original, _ := geminiObject(e.settings.orig)
	sec, _, _ := geminiChild(original, "security")
	auth, _, _ := geminiChild(sec, "auth")
	_, _ = auth.get("selectedType", &selected)
	if selected != geminiAPIAuth {
		return ProviderProfile{}, false, nil
	}
	values, _ := geminiEnv(e.env)
	credentialValues := values
	if _, block, err := geminiBlock(e.env); err == nil && len(block) > 0 {
		credentialValues, _ = geminiEnv(block)
	}
	p := ProviderProfile{BaseURL: values["GOOGLE_GEMINI_BASE_URL"], Model: values["GEMINI_MODEL"], WireAPI: "gemini", HasToken: credentialValues["GEMINI_API_KEY"] != ""}
	if !p.HasToken {
		p.EnvKey = "GEMINI_API_KEY"
	}
	if p.BaseURL == "" {
		return ProviderProfile{}, false, nil
	}
	if u, parseErr := url.Parse(p.BaseURL); ValidateProviderProfile(p) != nil || parseErr != nil || u.RawQuery != "" {
		return ProviderProfile{}, false, fmt.Errorf("Gemini endpoint cannot be represented safely (value hidden)")
	}
	return p, true, nil
}

func (g *Gemini) ProviderFields(p ProviderProfile) map[string]string {
	fields := providerFields(p)
	fields["protocol"] = "gemini"
	if p.Token == "" && !p.HasToken {
		fields["credential"] = "env $GEMINI_API_KEY"
	}
	return fields
}

func (g *Gemini) ProviderEnvironment() ([]ProviderDiagnostic, error) {
	var out []ProviderDiagnostic
	for _, key := range []string{"GOOGLE_GEMINI_BASE_URL", "GEMINI_MODEL", "GEMINI_API_KEY", "GEMINI_CLI_AUTH_OVERRIDE", "GEMINI_API_KEY_AUTH_MECHANISM", "GOOGLE_GENAI_USE_VERTEXAI", "GOOGLE_GENAI_USE_GCA", "GOOGLE_GENAI_API_VERSION"} {
		if _, present := os.LookupEnv(key); !present {
			continue
		}
		out = append(out, ProviderDiagnostic{Code: "environment_config_override", Variable: key, Source: "current process environment", OtherSource: g.providerEnvFile(), Explanation: "Gemini preserves inherited variables instead of replacing them from dotenv; this can override the saved connection, model or authentication.", NextStep: "Review the named variable in the environment used to start Gemini. Start a new session; lazyagents never unsets variables or edits shell profiles."})
	}
	p, active, err := g.ReadProvider()
	if err != nil {
		return nil, err
	}
	if active && !p.HasToken {
		out = append(out, missingProviderEnvironment("GEMINI_API_KEY", g.providerEnvFile(), "Native keychain credentials are not inspected.")...)
	}
	return out, nil
}
func (g *Gemini) RecoveryFiles() []RecoveryFile {
	return []RecoveryFile{{g.ProviderFile(), recoveryJSONChanges}, {g.providerEnvFile(), geminiEnvChanges}}
}
