// Command generate copies the SP Night palette and semantic roles, verbatim,
// into the bundled themes.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/rogeriojunior31/lazyagents/internal/fsutil"
)

var flavors = []string{"noite", "garoa", "jaragua"}

// groups are the role groups copied from roles.json, in the schema order.
var groups = []string{"ui", "syntax", "diagnostic", "git", "ansi"}

const header = "# Gerado a partir da paleta e dos papéis do SP Night; não edite.\n" +
	"# https://github.com/sp-night/sp-night — MIT (ver ../LICENSE-SP-Night)\n"

func main() {
	source := flag.String("source", "", "SP-Night/sp-night directory")
	out := flag.String("out", "internal/tui/theme/themes", "output directory")
	check := flag.Bool("check", false, "verify generated files without writing")
	flag.Parse()
	if err := generate(*source, *out, *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// JSON is YAML, so both files are read as yaml.Node: key order survives.
func read(source, name string) (*yaml.Node, error) {
	b, err := os.ReadFile(filepath.Join(source, "palette", name))
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return doc.Content[0], nil
}

func get(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func scalar(v string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Value: v} }

// clean copies a flat string map, dropping annotation keys ($comment, $_).
func clean(m *yaml.Node) *yaml.Node {
	out := &yaml.Node{Kind: yaml.MappingNode}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if strings.HasPrefix(m.Content[i].Value, "$") {
			continue
		}
		v := scalar(m.Content[i+1].Value)
		if strings.HasPrefix(v.Value, "#") {
			v.Style = yaml.DoubleQuotedStyle
		}
		out.Content = append(out.Content, scalar(m.Content[i].Value), v)
	}
	return out
}

func generate(source, out string, check bool) error {
	palette, err := read(source, "sp_night.json")
	if err != nil {
		return err
	}
	roles, err := read(source, "roles.json")
	if err != nil {
		return err
	}
	all := get(palette, "flavors")
	for _, id := range flavors {
		flavor := get(all, id)
		if flavor == nil {
			return fmt.Errorf("missing SP Night flavor %s", id)
		}
		doc := &yaml.Node{Kind: yaml.MappingNode}
		add := func(k string, v *yaml.Node) { doc.Content = append(doc.Content, scalar(k), v) }
		add("id", scalar(id))
		for _, k := range []string{"label", "description", "appearance"} {
			if v := get(flavor, k); v != nil {
				add(k, scalar(v.Value))
			}
		}
		add("palette", clean(get(flavor, "colors")))
		for _, g := range groups {
			n := get(roles, g)
			if n == nil {
				return fmt.Errorf("missing SP Night role group %s", g)
			}
			add(g, clean(n))
		}
		var buf bytes.Buffer
		buf.WriteString(header)
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(doc); err != nil {
			return err
		}
		if err := enc.Close(); err != nil {
			return err
		}
		path := filepath.Join(out, id+".yaml")
		if check {
			old, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if !bytes.Equal(old, buf.Bytes()) {
				return fmt.Errorf("generated theme differs: %s", path)
			}
		} else if err := fsutil.WriteAtomic(path, buf.Bytes(), 0o644); err != nil {
			return err
		}
	}
	return nil
}
