package agent

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestTOMLValueLines(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want []bool
	}{
		{"plain", "a = 1\n[t]\nb = 2", []bool{false, false, false}},
		{"multiline basic", "a = \"\"\"\n[x]\n\"\"\"\nb = 1", []bool{false, true, true, false}},
		{"escaped quotes", "a = \"\"\"\\\"\"\"\n[x]\n\"\"\"", []bool{false, true, true}},
		{"literal", "a = '''\n[x] \\'''\nb = 1", []bool{false, true, false}},
		{"one-line strings", "a = \"\"\"x\"\"\"\nb = '''y'''\nc = \"[\"", []bool{false, false, false}},
		{"array", "a = [\n  [1, 2],\n  \"]\",\n]\n[t]", []bool{false, true, true, true, false}},
		{"comment", "a = 1 # \"\"\" [\n[t]", []bool{false, false}},
		{"inline table", "a = { b = [\n1 ] }\nc = 1", []bool{false, true, false}},
	} {
		got, err := tomlValueLines(strings.Split(tc.in, "\n"))
		if err != nil || !slices.Equal(got, tc.want) {
			t.Errorf("%s: %v %v, want %v", tc.name, got, err, tc.want)
		}
	}
	for _, bad := range []string{"a = \"\"\"\nnever closed", "a = [\n1,", "a = '''\nx"} {
		if _, err := tomlValueLines(strings.Split(bad, "\n")); err == nil {
			t.Errorf("%q: want an unterminated error", bad)
		}
	}
}

// Multiline strings and arrays are content: a header, key or marker inside
// them is never touched, the block goes before the real first table, and
// apply then clear gives the file back byte for byte.
func TestCodexProviderMultilineTOML(t *testing.T) {
	c := NewCodex(t.TempDir())
	original := `model = "gpt-5"
developer_instructions = """
[not.a.table]
model = "inside a string"
# lazyagents — managed block start (do not edit by hand)
"""
notify = [
  "bash",
  ["nested", "array"],
]
literal = '''
[also.not.a.table]
'''

[projects."/home/me/app"]
trust_level = "trusted"
`
	if err := os.MkdirAll(filepath.Dir(c.ProviderFile()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.ProviderFile(), []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := c.ApplyProvider(ProviderProfile{Name: "local", BaseURL: "http://localhost:11434/v1", Model: "qwen3", EnvKey: "K"}, ""); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(c.ProviderFile())
	applied := string(data)
	for _, keep := range []string{
		"developer_instructions = \"\"\"\n[not.a.table]\nmodel = \"inside a string\"\n# lazyagents — managed block start (do not edit by hand)\n\"\"\"\n",
		"notify = [\n  \"bash\",\n  [\"nested\", \"array\"],\n]\n",
		"literal = '''\n[also.not.a.table]\n'''\n",
	} {
		if !strings.Contains(applied, keep) {
			t.Errorf("multiline value changed:\n%s", applied)
		}
	}
	if block, projects := strings.Index(applied, "model_provider = \"lazyagents\""), strings.Index(applied, "[projects."); block < strings.Index(applied, "'''\n\n") || block > projects {
		t.Errorf("top-level block not between the last value and the first table:\n%s", applied)
	}
	if got, ok, _ := c.ReadProvider(); !ok || got.Model != "qwen3" || got.BaseURL != "http://localhost:11434/v1" {
		t.Errorf("ReadProvider = %+v %v", got, ok)
	}
	if err := c.ClearProvider(""); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(c.ProviderFile()); string(data) != original {
		t.Errorf("clear did not give the file back:\n%s", data)
	}
}

func TestCodexRefusesUnterminatedTOML(t *testing.T) {
	c := NewCodex(t.TempDir())
	original := "instructions = \"\"\"\nnever closed\n"
	if err := os.MkdirAll(filepath.Dir(c.ProviderFile()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.ProviderFile(), []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := c.ApplyProvider(ProviderProfile{BaseURL: "https://example.com"}, ""); err == nil {
		t.Fatal("accepted a config whose string never closes")
	}
	if data, _ := os.ReadFile(c.ProviderFile()); string(data) != original {
		t.Fatal("modified config")
	}
}
