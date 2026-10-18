//go:build ignore

package main

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func releaseSums(version string) string {
	var b strings.Builder
	for i, target := range []string{"darwin_amd64.tar.gz", "darwin_arm64.tar.gz", "linux_amd64.tar.gz", "linux_arm64.tar.gz", "windows_amd64.zip"} {
		b.WriteString(strings.Repeat(string(rune('a'+i)), 64) + "  lazyagents_" + version + "_" + target + "\n")
	}
	return b.String()
}

func TestPackageChecksumsRejectInvalidOrMixedRelease(t *testing.T) {
	valid := releaseSums("0.2.3")
	for _, tc := range []struct{ name, version, manifest string }{
		{"missing target", "0.2.3", strings.SplitN(valid, "\n", 2)[1]},
		{"different release", "0.2.4", valid},
		{"duplicate", "0.2.3", valid + strings.Split(valid, "\n")[0] + "\n"},
		{"skip hash", "0.2.3", strings.Replace(valid, strings.Repeat("a", 64), "SKIP", 1)},
		{"invalid hash", "0.2.3", strings.Replace(valid, strings.Repeat("a", 64), strings.Repeat("g", 64), 1)},
		{"extra fields", "0.2.3", valid + "unexpected row contents\n"},
		{"unsafe version", "../0.2.3", valid},
		{"leading zero", "0.02.3", valid},
		{"tag prefix", "v0.2.3", valid},
		{"prerelease", "0.2.3-rc.1", valid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := packageSums(tc.version, strings.NewReader(tc.manifest)); err == nil {
				t.Fatal("accepted invalid release manifest")
			}
		})
	}
	sums, err := packageSums("0.2.3", strings.NewReader(valid))
	if err != nil || len(sums) != 5 {
		t.Fatalf("valid manifest: %v", err)
	}
	// A malformed update cannot replace reviewed recipes.
	root := t.TempDir()
	target := filepath.Join(root, "homebrew", "Formula", "lazyagents.rb")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("reviewed recipe"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(t.TempDir(), "SHA256SUMS")
	if err := os.WriteFile(manifest, []byte("invalid\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := updatePackages("0.2.3", manifest, root); err == nil {
		t.Fatal("invalid manifest replaced recipes")
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "reviewed recipe" {
		t.Fatal("existing recipe changed on invalid input")
	}
}

func TestHomebrewSourcesKeepTheirArchitectureChecksums(t *testing.T) {
	manifest := filepath.Join(t.TempDir(), "SHA256SUMS")
	if err := os.WriteFile(manifest, []byte(releaseSums("0.2.3")), 0o644); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := updatePackages("0.2.3", manifest, root); err != nil {
		t.Fatal(err)
	}
	formula, err := os.ReadFile(filepath.Join(root, "homebrew", "Formula", "lazyagents.rb"))
	if err != nil {
		t.Fatal(err)
	}
	pairs := regexp.MustCompile(`url "([^"]+)"\s+sha256 "([^"]+)"`).FindAllStringSubmatch(string(formula), -1)
	want := map[string]string{}
	for i, target := range []string{"darwin_amd64", "darwin_arm64", "linux_amd64", "linux_arm64"} {
		want["lazyagents_0.2.3_"+target+".tar.gz"] = strings.Repeat(string(rune('a'+i)), 64)
	}
	if len(pairs) != len(want) {
		t.Fatal("missing Homebrew platform source")
	}
	for _, pair := range pairs {
		name := path.Base(pair[1])
		if want[name] == "" || want[name] != pair[2] {
			t.Fatalf("wrong checksum for %s", name)
		}
		delete(want, name)
	}
}
