package skills

import (
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func localRepositoryURL(dir string) string {
	p := filepath.ToSlash(dir)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}

func TestRepositoryDescriptorsStaySeparateFromInstalledOrigins(t *testing.T) {
	p := testPaths(t)
	svc := New(p)
	if err := os.MkdirAll(p.ConfigDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.ConfigPath(), []byte("# keep\nskills:\n  installTargets: [codex]\n  future: keep\nforeign:\n  value: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := repository{Name: "ECC", URL: "example/repository"}
	if err := svc.saveRepository(r, nil, false); err != nil {
		t.Fatal(err)
	}
	items, err := svc.repositories()
	if err != nil || len(items) != 1 || items[0].URL != "https://github.com/example/repository" {
		t.Fatalf("descriptors: %v %v", items, err)
	}
	r = items[0]
	if err := svc.saveRepository(repository{Name: "ecc", URL: r.URL}, nil, false); err == nil {
		t.Fatal("duplicate source name accepted")
	}
	writeSkill(t, p.LibraryDir(), "installed", validMD("installed", "kept"))
	origin := Origin{Type: "git", Source: r.URL, Ref: "release", Sub: "skills/installed", ExactPath: true}
	if err := writeOrigin(filepath.Join(p.LibraryDir(), "installed"), origin); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(p.LibraryDir(), "installed", originFile))
	edited := r
	edited.URL = "https://example.invalid/another-repository.git"
	edited.Ref = "next"
	if err := svc.saveRepository(edited, &r, false); err != nil {
		t.Fatal(err)
	}
	if err := svc.saveRepository(r, &r, true); err == nil {
		t.Fatal("stale descriptor review removed an edited source")
	}
	if err := svc.saveRepository(edited, &edited, true); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(p.LibraryDir(), "installed", originFile))
	if string(before) != string(after) {
		t.Fatal("source edit/removal redirected installed provenance")
	}
	if scanOne(t, svc, nil, "installed").Description != "kept" {
		t.Fatal("removing descriptor removed installed files")
	}
	data, _ := os.ReadFile(p.ConfigPath())
	for _, want := range []string{"# keep", "installTargets: [codex]", "future: keep", "value: true", "sources: []"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("config lost %q: %s", want, data)
		}
	}
	// Listing descriptors and constructing the tab must not invoke Git.
	if err := svc.saveRepository(edited, nil, false); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "")
	if listed, err := svc.repositories(); err != nil || len(listed) != 1 || listed[0].Ref != "next" {
		t.Fatal(err)
	}
	_ = newTab(svc)
}

func TestRepositoryBranchSubfolderAndExactUpdateIdentity(t *testing.T) {
	repo := t.TempDir()
	git := makeGitRepo(t, repo)
	git("checkout", "-b", "stable")
	for _, name := range []string{"tagged", "moving"} {
		writeSkill(t, filepath.Join(repo, "catalog"), name, validMD(name, "v1"))
	}
	writeSkill(t, repo, "outside", validMD("outside", "outside scope"))
	git("add", ".")
	git("commit", "-m", "v1")
	git("tag", "v1")
	p := testPaths(t)
	svc := New(p)
	r := repository{Name: "Pinned", URL: localRepositoryURL(repo), Ref: "v1", Subdir: "catalog"}
	found, origin, cleanup, err := svc.scanRepository(r)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(cleanup)
	if len(found) != 2 || origin.Ref != "v1" || len(origin.Revision) < 40 || !origin.ExactPath {
		t.Fatalf("scan: %+v %+v", found, origin)
	}
	selected := slices.DeleteFunc(slices.Clone(found), func(f Found) bool { return f.Name != "tagged" })
	if _, err := svc.Install(selected, origin); err != nil {
		t.Fatal(err)
	}
	r.Ref = "stable"
	found, origin, otherCleanup, err := svc.scanRepository(r)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(otherCleanup)
	selected = slices.DeleteFunc(slices.Clone(found), func(f Found) bool { return f.Name != "moving" })
	if _, err := svc.Install(selected, origin); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, filepath.Join(repo, "catalog"), "tagged", validMD("tagged", "v2"))
	writeSkill(t, filepath.Join(repo, "catalog"), "moving", validMD("moving", "v2"))
	git("add", ".")
	git("commit", "-m", "v2")
	skills, _ := svc.Scan(nil)
	checks, err := svc.CheckUpdates(skills)
	if err != nil || len(checks) != 2 {
		t.Fatalf("checks %v %v", checks, err)
	}
	for _, c := range checks {
		want := UpdateStatusAvailable
		if c.Skill.Dir == "tagged" {
			want = UpdateStatusUpToDate
		}
		if c.Status != want {
			t.Fatalf("ref grouped incorrectly: %+v", c)
		}
	}
	moving := scanOne(t, svc, nil, "moving")
	oldRevision := moving.Origin.Revision
	if err := svc.Update(moving); err != nil {
		t.Fatal(err)
	}
	moving = scanOne(t, svc, nil, "moving")
	if moving.Description != "v2" || moving.Origin.Revision == oldRevision || moving.Origin.Sub != "catalog/moving" {
		t.Fatalf("update: %+v", moving)
	}
	if err := svc.Update(scanOne(t, svc, nil, "tagged")); err != nil {
		t.Fatal(err)
	}
	if scanOne(t, svc, nil, "tagged").Description != "v1" {
		t.Fatal("tag update followed the default branch")
	}
	// A same-name skill at a new path is never an implicit replacement.
	if err := os.RemoveAll(filepath.Join(repo, "catalog", "moving")); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, repo, "elsewhere", validMD("moving", "wrong replacement"))
	git("add", ".")
	git("commit", "-m", "move source path")
	if err := svc.Update(moving); err == nil || !strings.Contains(err.Error(), "refusing another source path") {
		t.Fatalf("missing path fallback: %v", err)
	}
	if scanOne(t, svc, nil, "moving").Description != "v2" {
		t.Fatal("missing path update changed local files")
	}
	// Old origins without the new fields still decode and use legacy discovery.
	if err := writeOrigin(filepath.Join(p.LibraryDir(), "tagged"), Origin{Type: "git", Source: r.URL, Sub: "catalog/tagged"}); err != nil {
		t.Fatal(err)
	}
	if scanOne(t, svc, nil, "tagged").Origin.ExactPath {
		t.Fatal("legacy origins changed policy")
	}
}

func TestRepositoryDuplicateNamesAndScanFailures(t *testing.T) {
	repo := t.TempDir()
	git := makeGitRepo(t, repo)
	writeSkill(t, filepath.Join(repo, "catalog"), "first", validMD("same", "first path"))
	writeSkill(t, filepath.Join(repo, "catalog"), "second", validMD("same", "second path"))
	writeSkill(t, filepath.Join(repo, "catalog"), "unique", validMD("unique", "unique"))
	git("add", ".")
	git("commit", "-m", "duplicates")
	svc := New(testPaths(t))
	r := repository{Name: "Duplicate fixture", URL: localRepositoryURL(repo), Subdir: "catalog"}
	found, origin, cleanup, err := svc.scanRepository(r)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(cleanup)
	if len(found) != 3 || !found[0].Collision || !found[1].Collision || found[0].Rel == found[1].Rel {
		t.Fatalf("same names collapsed: %+v", found)
	}
	if _, err := svc.Install(found[:2], origin); err == nil {
		t.Fatal("duplicate destination installed an arbitrary winner")
	}
	if _, err := os.Stat(filepath.Join(svc.paths.LibraryDir(), "same")); !os.IsNotExist(err) {
		t.Fatal("collision wrote a library folder")
	}
	out := svc.installWithTargets(found[:2], origin, nil, nil)
	if len(out.result.FailedIDs()) != 2 {
		t.Fatalf("per-entry install bypassed collision guard: %+v", out.result)
	}
	if _, err := os.Stat(filepath.Join(svc.paths.LibraryDir(), "same")); !os.IsNotExist(err) {
		t.Fatal("install/activate flow chose an arbitrary winner")
	}
	if _, err := svc.Install(found[1:2], origin); err != nil {
		t.Fatal(err)
	}
	if scanOne(t, svc, nil, "same").Description != "second path" {
		t.Fatal("explicit source path was not installed")
	}
	for _, ref := range []string{"missing-branch", "--upload-pack=evil", "bad..ref"} {
		bad := r
		bad.Ref = ref
		_, _, tmp, err := svc.scanRepository(bad)
		if err == nil || tmp != "" {
			t.Fatalf("bad ref retained staging: %q %q %v", ref, tmp, err)
		}
	}
	bad := r
	bad.Subdir = "missing"
	if _, _, tmp, err := svc.scanRepository(bad); err == nil || tmp != "" {
		t.Fatal("missing subfolder retained staging")
	}
	for _, sub := range []string{"../escape", "/absolute", `..\escape`, "C:/escape"} {
		bad := r
		bad.Subdir = sub
		if _, err := normalizeRepository(bad); err == nil {
			t.Fatalf("accepted subfolder %q", sub)
		}
	}
	for _, address := range []string{"ext::sh -c evil", "--upload-pack=evil", "https://user:password@example.test/repo.git"} {
		bad := r
		bad.URL = address
		if _, err := normalizeRepository(bad); err == nil {
			t.Fatalf("accepted unsafe URL %q", address)
		}
	}
}

func TestRepositoryScopedMarketplaceAndSymlinkEscape(t *testing.T) {
	repo := t.TempDir()
	git := makeGitRepo(t, repo)
	marketplaceRepo(t, repo)
	git("add", ".")
	git("commit", "-m", "marketplace")
	svc := New(testPaths(t))
	r := repository{Name: "Marketplace", URL: localRepositoryURL(repo)}
	found, origin, cleanup, err := svc.scanRepository(r)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(cleanup)
	if len(found) != 3 || len(origin.Notes) == 0 {
		t.Fatalf("marketplace not reused: %+v %+v", found, origin)
	}
	external := t.TempDir()
	writeSkill(t, external, "secret", validMD("secret", "outside"))
	mustSymlink(t, external, filepath.Join(repo, "escape"))
	git("add", ".")
	git("commit", "-m", "external symlink")
	r.Subdir = "escape"
	if _, _, tmp, err := svc.scanRepository(r); err == nil || tmp != "" {
		t.Fatalf("followed external source folder: %s %v", tmp, err)
	}
}
