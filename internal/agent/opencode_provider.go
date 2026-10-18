package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

const openCodeProviderID = "lazyagents"
const openCodeNoKey = "lazyagents-no-key"

// Only public model selection and ownership digests leave native configuration.
type openCodeProviderState struct {
	Path             string          `json:"path,omitempty"`
	Entries          []string        `json:"entries,omitempty"`
	Models           []string        `json:"models,omitempty"`
	PreviousModel    json.RawMessage `json:"previousModel,omitempty"`
	CreatedFile      bool            `json:"createdFile,omitempty"`
	CreatedProviders bool            `json:"createdProviders,omitempty"`
}

func (o *OpenCode) ProviderFile() string {
	path := filepath.Join(o.configDir(), "opencode.jsonc")
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return path
	}
	return filepath.Join(o.configDir(), "opencode.json")
}

func openCodeObject(raw []byte) (*object, error) {
	obj, err := decodeObject(raw)
	if err != nil {
		return nil, fmt.Errorf("expected an unambiguous JSON object (values hidden)")
	}
	seen := map[string]bool{}
	for _, key := range obj.keys() {
		if seen[key] {
			return nil, fmt.Errorf("duplicate JSON keys are not supported (values hidden)")
		}
		seen[key] = true
	}
	return obj, nil
}

func (o *OpenCode) providerSettings() (*settings, error) {
	path := o.ProviderFile()
	other := filepath.Join(o.configDir(), "opencode.json")
	if path != other {
		if _, err := os.Lstat(other); !os.IsNotExist(err) {
			return nil, fmt.Errorf("OpenCode provider control requires a single global config file; both opencode.json and opencode.jsonc exist or cannot be inspected")
		}
	}
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("OpenCode provider config must be a regular file: %s", path)
	}
	s, err := readSettings(path)
	if err != nil || !s.missing && !json.Valid(s.orig) {
		return nil, fmt.Errorf("OpenCode provider control requires a readable strict JSON object in %s; JSONC comments/trailing commas are left untouched (values hidden)", path)
	}
	obj, err := openCodeObject(s.orig)
	if err != nil {
		return nil, err
	}
	s.object = obj
	return s, nil
}

func openCodeProviders(s *settings, key string) (*object, bool, error) {
	raw, exists := s.raw(key)
	if !exists {
		return &object{}, false, nil
	}
	p, err := openCodeObject(raw)
	return p, true, err
}

// Short references and V2's explicit public selection are both native formats.
func openCodeModel(raw json.RawMessage) (provider, model string, err error) {
	if len(raw) == 0 {
		return "", "", nil
	}
	var short string
	if json.Unmarshal(raw, &short) == nil {
		provider, model, _ = strings.Cut(short, "/")
		if provider != "" && model != "" && !strings.Contains(short, "#") && !openCodeSubstitution(short) {
			return provider, model, nil
		}
	} else if obj, e := openCodeObject(raw); e == nil {
		var p, m string
		_, pe := obj.get("providerID", &p)
		_, me := obj.get("model", &m)
		if pe == nil && me == nil && p != "" && m != "" && !strings.ContainsAny(p, "/#") && !openCodeSubstitution(p+m) {
			// Unknown model-selection fields must not enter the ownership file.
			if len(obj.keys()) == 2 {
				return p, m, nil
			}
		}
	}
	return "", "", fmt.Errorf("OpenCode model selection is not a supported explicit provider/model reference (values hidden)")
}

func openCodeSubstitution(value string) bool {
	return strings.Contains(value, "{env:") || strings.Contains(value, "{file:")
}

type openCodeKey struct {
	has bool
	env string
}

func (k *openCodeKey) UnmarshalJSON(data []byte) error {
	var value string
	if json.Unmarshal(data, &value) != nil {
		return fmt.Errorf("OpenCode credential field is not a string (value hidden)")
	}
	*k = openCodeKey{}
	if value == "" || value == openCodeNoKey {
		return nil
	}
	if strings.HasPrefix(value, "{env:") && strings.HasSuffix(value, "}") {
		name := strings.TrimSuffix(strings.TrimPrefix(value, "{env:"), "}")
		if envNameRe.MatchString(name) {
			k.env = name
			return nil
		}
	}
	k.has = true // File references and literals stay inside the native agent.
	return nil
}

func (o *OpenCode) ReadProvider() (ProviderProfile, bool, error) {
	s, err := o.providerSettings()
	if err != nil {
		return ProviderProfile{}, false, err
	}
	rawModel, _ := s.raw("model")
	id, model, err := openCodeModel(rawModel)
	if err != nil || id == "" {
		return ProviderProfile{}, false, err
	}
	legacy, _, err := openCodeProviders(s, "provider")
	if err != nil {
		return ProviderProfile{}, false, err
	}
	native, _, err := openCodeProviders(s, "providers")
	if err != nil {
		return ProviderProfile{}, false, err
	}
	raw, found := legacy.raw(id)
	nativeRaw, nativeFound := native.raw(id)
	if found && nativeFound {
		return ProviderProfile{}, false, fmt.Errorf("selected OpenCode provider occurs in both provider and providers; review the merged configuration locally (values hidden)")
	}
	if !found && !nativeFound {
		return ProviderProfile{}, false, nil
	} // Native/default providers stay native.
	if nativeFound {
		raw = nativeRaw
	}
	config, err := openCodeObject(raw)
	if err != nil {
		return ProviderProfile{}, false, err
	}
	packageField, optionsField := "npm", "options"
	if nativeFound {
		packageField, optionsField = "package", "settings"
	}
	var pack, base string
	var key openCodeKey
	// Native JSON keys are case-sensitive, unlike struct-field matching in encoding/json.
	if _, err := config.get(packageField, &pack); err != nil {
		return ProviderProfile{}, false, fmt.Errorf("OpenCode provider package must be a string (value hidden)")
	}
	if pack != "@ai-sdk/openai-compatible" && pack != "@opencode/ai/providers/openai-compatible" {
		return ProviderProfile{Name: id, Model: model, WireAPI: "native"}, true, nil
	}
	optionsRaw, _ := config.raw(optionsField)
	options, err := openCodeObject(optionsRaw)
	if err != nil {
		return ProviderProfile{}, false, err
	}
	if _, err := options.get("baseURL", &base); err != nil {
		return ProviderProfile{}, false, fmt.Errorf("OpenCode endpoint must be a string (value hidden)")
	}
	if _, err := options.get("apiKey", &key); err != nil {
		return ProviderProfile{}, false, fmt.Errorf("OpenCode credential must be a string (value hidden)")
	}
	var models map[string]json.RawMessage
	if _, err := config.get("models", &models); err != nil {
		return ProviderProfile{}, false, fmt.Errorf("OpenCode models must be an object (values hidden)")
	}
	if rawModel, ok := models[model]; ok {
		selected, err := openCodeObject(rawModel)
		if err != nil {
			return ProviderProfile{}, false, err
		}
		for _, field := range []string{"npm", "api", "package", "options", "settings"} {
			if _, override := selected.raw(field); override {
				return ProviderProfile{Name: id, Model: model, WireAPI: "native"}, true, nil
			}
		}
	}
	if openCodeSubstitution(base) {
		return ProviderProfile{}, false, fmt.Errorf("OpenCode endpoint substitutions are not resolved by lazyagents; review the native config locally (value hidden)")
	}
	if err := ValidateProviderProfile(ProviderProfile{BaseURL: base}); err != nil {
		return ProviderProfile{}, false, fmt.Errorf("OpenCode endpoint is not a supported public URL (value hidden)")
	}
	return ProviderProfile{Name: id, BaseURL: base, Model: model, WireAPI: "chat", HasToken: key.has, EnvKey: key.env}, true, nil
}

func (o *OpenCode) ProviderEnvironment() ([]ProviderDiagnostic, error) {
	p, active, err := o.ReadProvider()
	if err != nil {
		return nil, err
	}
	var out []ProviderDiagnostic
	if active {
		out = missingProviderEnvironment(p.EnvKey, o.ProviderFile()+" (selected provider apiKey)", "OpenCode resolves the reference in its own process; native login is not evaluated here.")
	}
	for _, name := range []string{"OPENCODE_CONFIG", "OPENCODE_CONFIG_CONTENT", "OPENCODE_CONFIG_DIR"} {
		if os.Getenv(name) != "" {
			out = append(out, ProviderDiagnostic{Code: "additional_config_source", Variable: name, Source: "current process environment", OtherSource: o.ProviderFile(),
				Explanation: "OpenCode has an additional configuration source which may override the global provider snapshot. Its contents are not inspected.",
				NextStep:    "Review that source and project/managed settings locally, then start a new OpenCode session. Applying or clearing here changes only the global file."})
		}
	}
	return out, nil
}

func openCodeProviderFields(p ProviderProfile) error {
	if err := ValidateProviderProfile(p); err != nil {
		return err
	}
	if p.BaseURL == "" || strings.TrimSpace(p.Model) == "" {
		return fmt.Errorf("OpenCode requires an endpoint and explicit model ID")
	}
	if p.WireAPI != "" && p.WireAPI != "chat" {
		return fmt.Errorf("OpenCode provider control currently supports wireApi chat only")
	}
	for _, value := range []string{p.Name, p.BaseURL, p.Model, p.Token} {
		if openCodeSubstitution(value) || strings.ContainsAny(value, "\r\n\x00") {
			return fmt.Errorf("OpenCode profile values cannot contain substitutions, newlines or NUL (values hidden)")
		}
	}
	if strings.Contains(p.Model, "#") {
		return fmt.Errorf("OpenCode requires a model ID without a variant suffix")
	}
	return nil
}

func openCodeConfig(p ProviderProfile) any {
	key := openCodeNoKey
	if p.Token != "" {
		key = p.Token
	} else if p.EnvKey != "" {
		key = "{env:" + p.EnvKey + "}"
	}
	return struct {
		NPM     string            `json:"npm"`
		Name    string            `json:"name"`
		Options map[string]string `json:"options"`
		Models  map[string]any    `json:"models"`
	}{"@ai-sdk/openai-compatible", "lazyagents: " + p.Name, map[string]string{"baseURL": p.BaseURL, "apiKey": key}, map[string]any{p.Model: map[string]string{"name": p.Model}}}
}

func openCodeHash(raw []byte) string {
	var compact bytes.Buffer
	_ = json.Compact(&compact, raw)
	return digest(compact.String())
}

// Explicit reapply may replace edited public fields in our own simple entry.
// Advanced additions are foreign content, so refuse instead of deleting them.
func openCodeManagedShape(raw []byte) bool {
	entry, err := openCodeObject(raw)
	if err != nil || len(entry.keys()) != 4 {
		return false
	}
	for _, key := range []string{"npm", "name"} {
		var value string
		if ok, err := entry.get(key, &value); !ok || err != nil {
			return false
		}
	}
	optionsRaw, _ := entry.raw("options")
	options, err := openCodeObject(optionsRaw)
	if err != nil || len(options.keys()) != 2 {
		return false
	}
	for _, key := range []string{"baseURL", "apiKey"} {
		var v string
		if ok, err := options.get(key, &v); !ok || err != nil {
			return false
		}
	}
	modelsRaw, _ := entry.raw("models")
	models, err := openCodeObject(modelsRaw)
	if err != nil || len(models.keys()) != 1 {
		return false
	}
	modelRaw, _ := models.raw(models.keys()[0])
	model, err := openCodeObject(modelRaw)
	if err != nil || len(model.keys()) != 1 {
		return false
	}
	var name string
	ok, err := model.get("name", &name)
	return ok && err == nil
}

func (o *OpenCode) loadOpenCodeProviderState() (openCodeProviderState, error) {
	if o.ProviderState == "" {
		if o.providerMem != nil {
			return *o.providerMem, nil
		}
		return openCodeProviderState{}, nil
	}
	var st openCodeProviderState
	data, err := os.ReadFile(o.ProviderState)
	if errors.Is(err, os.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return st, fmt.Errorf("reading OpenCode provider ownership: %w", err)
	}
	if json.Unmarshal(data, &st) != nil {
		return st, fmt.Errorf("OpenCode provider ownership is invalid (values hidden)")
	}
	if _, _, err := openCodeModel(st.PreviousModel); err != nil {
		return st, err
	}
	return st, nil
}

func (o *OpenCode) saveOpenCodeProviderState(st openCodeProviderState) error {
	if o.ProviderState == "" {
		o.providerMem = &st
		return nil
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteAtomic(o.ProviderState, append(data, '\n'), 0o600)
}

func (o *OpenCode) prepareOpenCodeProvider(p *ProviderProfile) (*settings, openCodeProviderState, bool, error) {
	st, err := o.loadOpenCodeProviderState()
	if err != nil {
		return nil, st, false, err
	}
	s, err := o.providerSettings()
	if err != nil {
		return nil, st, false, err
	}
	if st.Path != "" && st.Path != s.path {
		return nil, st, false, fmt.Errorf("OpenCode global config selection changed; review the previous managed file before applying or clearing")
	}
	providers, had, err := openCodeProviders(s, "provider")
	if err != nil {
		return nil, st, false, err
	}
	native, _, err := openCodeProviders(s, "providers")
	if err != nil {
		return nil, st, false, err
	}
	if _, conflict := native.raw(openCodeProviderID); conflict {
		return nil, st, false, fmt.Errorf("OpenCode already has a native providers.lazyagents entry; it is not managed by lazyagents")
	}
	entry, exists := providers.raw(openCodeProviderID)
	if exists && !slices.Contains(st.Entries, openCodeHash(entry)) && !(p != nil && st.Path != "" && openCodeManagedShape(entry)) {
		return nil, st, false, fmt.Errorf("OpenCode provider.lazyagents is foreign or was changed outside lazyagents; it was kept")
	}
	model, _ := s.raw("model")
	if p == nil {
		if st.Path == "" {
			return s, st, false, nil
		}
		if slices.Contains(st.Models, openCodeHash(model)) {
			var prev any
			if len(st.PreviousModel) > 0 {
				prev = json.RawMessage(st.PreviousModel)
			}
			_ = s.set("model", prev)
		} else if id, _, _ := openCodeModel(model); id == openCodeProviderID {
			return nil, st, false, fmt.Errorf("OpenCode model was changed to another managed-provider reference; review it locally before clearing")
		}
		providers.delete(openCodeProviderID)
		if !providers.empty() || !st.CreatedProviders {
			_ = s.set("provider", providers)
		} else {
			_ = s.set("provider", nil)
		}
		return s, st, st.CreatedFile && s.object.empty(), nil
	}
	if err := openCodeProviderFields(*p); err != nil {
		return nil, st, false, err
	}
	for _, key := range []string{"enabled_providers", "disabled_providers"} {
		var ids []string
		if present, err := s.get(key, &ids); err != nil {
			return nil, st, false, fmt.Errorf("OpenCode provider filters have unsupported values (hidden)")
		} else if present {
			if key == "enabled_providers" && !slices.Contains(ids, openCodeProviderID) || key == "disabled_providers" && slices.Contains(ids, openCodeProviderID) {
				return nil, st, false, fmt.Errorf("OpenCode provider filters exclude the managed provider; review them locally before applying")
			}
		}
	}
	id, _, err := openCodeModel(model)
	if err != nil {
		return nil, st, false, err
	}
	if id == openCodeProviderID && !slices.Contains(st.Models, openCodeHash(model)) {
		return nil, st, false, fmt.Errorf("OpenCode model references an unverified managed selection; choose a native provider locally before applying")
	}
	if st.Path == "" {
		st.Path, st.PreviousModel, st.CreatedFile, st.CreatedProviders = s.path, slices.Clone(model), s.missing, !had
	} else if !slices.Contains(st.Models, openCodeHash(model)) {
		st.PreviousModel = slices.Clone(model)
	}
	_ = providers.set(openCodeProviderID, openCodeConfig(*p))
	_ = s.set("provider", providers)
	_ = s.set("model", openCodeProviderID+"/"+p.Model)
	return s, st, false, nil
}

func (o *OpenCode) ValidateProvider(p ProviderProfile) (ProviderValidation, error) {
	v := providerValidation(ProviderFiles(o), "OpenCode uses Chat Completions. A saved token takes precedence over envKey; otherwise the native {env:NAME} reference is written. Keyless endpoints receive a placeholder credential.")
	v.Notes = append(v.Notes, "Only one strict-JSON global config is supported; comments, trailing commas and multiple global files are refused. Project, custom/inline, managed settings and session selection may override it; native login is never read or changed.", "OpenCode 2 may keep a background server with cached settings. Use opencode reload or restart its service before testing a new session; lazyagents never controls that service.")
	_, _, _, err := o.prepareOpenCodeProvider(&p)
	return v, err
}

func (o *OpenCode) ApplyProvider(p ProviderProfile, backupsDir string) error {
	return o.writeOpenCodeProvider(&p, backupsDir)
}
func (o *OpenCode) ClearProvider(backupsDir string) error {
	return o.writeOpenCodeProvider(nil, backupsDir)
}

var beforeOpenCodeProviderWrite = func() {}

func (o *OpenCode) writeOpenCodeProvider(p *ProviderProfile, backupsDir string) error {
	s, st, remove, err := o.prepareOpenCodeProvider(p)
	if err != nil {
		return err
	}
	if p == nil && st.Path == "" {
		return nil
	}
	pending := st
	if p != nil {
		providers, _, _ := openCodeProviders(s, "provider")
		entry, _ := providers.raw(openCodeProviderID)
		model, _ := s.raw("model")
		pending.Entries = append(slices.Clone(st.Entries), openCodeHash(entry))
		pending.Models = append(slices.Clone(st.Models), openCodeHash(model))
	}
	// Retain both old/new ownership if the conditional native write fails.
	if err := o.saveOpenCodeProviderState(pending); err != nil {
		return err
	}
	if p != nil && p.Token != "" {
		s.perm &= 0o600
	}
	beforeOpenCodeProviderWrite()
	op := "provider.apply"
	if p == nil {
		op = "provider.clear"
	}
	origin := fsutil.BackupOrigin{Agent: o.ID(), Operation: op}
	if remove {
		err = s.remove(backupsDir, origin)
	} else {
		err = s.save(backupsDir, origin)
	}
	if err != nil {
		return err
	}
	if p == nil {
		return o.saveOpenCodeProviderState(openCodeProviderState{})
	}
	pending.Entries = pending.Entries[len(pending.Entries)-1:]
	pending.Models = pending.Models[len(pending.Models)-1:]
	return o.saveOpenCodeProviderState(pending)
}

func (o *OpenCode) PreviewProvider(p *ProviderProfile) (ProviderPreview, error) {
	s, st, remove, err := o.prepareOpenCodeProvider(p)
	if err != nil {
		return ProviderPreview{}, err
	}
	file, after, err := jsonProviderPreview(s, remove)
	if p == nil && st.Path == "" {
		file.Action, file.Changes = "unchanged", nil
	}
	state, _ := json.Marshal(st)
	return ProviderPreview{Managed: st.Path != "", Files: []ProviderFilePreview{file}, Revision: providerRevision(p, s.orig, after, state, []byte(s.path), []byte(fmt.Sprint(s.missing)))}, err
}

func (o *OpenCode) RecoveryFiles() []RecoveryFile {
	return []RecoveryFile{{filepath.Join(o.configDir(), "opencode.json"), recoveryJSONChanges}, {filepath.Join(o.configDir(), "opencode.jsonc"), recoveryJSONChanges}}
}

func (o *OpenCode) ProviderFields(p ProviderProfile) map[string]string {
	fields := providerFields(p)
	if p.WireAPI == "" {
		fields["protocol"] = "chat"
	}
	return fields
}
