package providers

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

func TestGeminiAdoptionAndReviewedMultiFileWrites(t *testing.T) {
	paths := core.PathsIn(t.TempDir())
	adapters := agent.AllWithIndex(paths.Home, filepath.Join(paths.DataDir, "index.json"))
	host := agent.ByID(adapters, "gemini-cli").(agent.ProviderHost)
	files := agent.ProviderFiles(host)
	config := `{"security":{"auth":{"selectedType":"gemini-api-key"}},"foreign":true}`
	env := "OTHER=keep\nGOOGLE_GEMINI_BASE_URL=https://external.test\nGEMINI_MODEL=native-model\nGEMINI_API_KEY=native-private-key\n"
	writeProviderFixture(t, files[0], config)
	writeProviderFixture(t, files[1], env)
	svc := New([]agent.Adapter{agent.ByID(adapters, "gemini-cli")}, paths)
	svc.Detect = func() []agent.Agent { return []agent.Agent{{ID: "gemini-cli", Name: "Gemini CLI", Installed: true}} }
	adoption, err := svc.AdoptDraft("gemini-cli")
	if err != nil || !adoption.NeedsCredential || adoption.Profile.Token != "" {
		t.Fatalf("adoption: %+v %v", adoption, err)
	}
	encoded, _ := json.Marshal(adoption)
	if bytes.Contains(encoded, []byte("native-private-key")) {
		t.Fatal("adopt leaked token")
	}
	p := adoption.Profile
	if err := svc.SaveAdoption(adoption, p); err == nil {
		t.Fatal("saved without explicit credential")
	}
	p.Token = "reentered-private-key"
	if err := svc.SaveAdoption(adoption, p); err != nil {
		t.Fatal(err)
	}
	plan := svc.PrepareSaved(p.Name, []string{"gemini-cli"})
	if !plan.valid() || len(plan.Targets[0].Preview.Files) != 2 {
		t.Fatalf("plan: %+v", plan)
	}
	// Either native file invalidates the review before writing the other one.
	writeProviderFixture(t, files[1], env+"NEW=keep\n")
	if r := svc.ApplyReviewed(p.Name, []string{"gemini-cli"}, plan); r.AsError() == nil {
		t.Fatal("accepted stale dotenv")
	}
	got, _ := os.ReadFile(files[0])
	if string(got) != config {
		t.Fatal("stale plan wrote settings")
	}
	plan = svc.PrepareSaved(p.Name, []string{"gemini-cli"})
	if r := svc.ApplyReviewed(p.Name, []string{"gemini-cli"}, plan); r.AsError() != nil {
		t.Fatal(r.AsError())
	}
	clear := svc.PrepareClear([]string{"gemini-cli"})
	if !clear.valid() {
		t.Fatalf("clear: %+v", clear)
	}
	if r := svc.ClearReviewed([]string{"gemini-cli"}, clear); r.AsError() != nil {
		t.Fatal(r.AsError())
	}
	got, _ = os.ReadFile(files[1])
	if string(got) != env+"NEW=keep\n" {
		t.Fatal("clear lost foreign dotenv")
	}
}
