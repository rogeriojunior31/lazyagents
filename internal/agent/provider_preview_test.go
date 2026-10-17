package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestProviderPreviewMatchesNativeChangesWithoutWriting(t *testing.T) {
	for _, id := range []string{"claude-code", "codex", "pi", "crush"} {
		t.Run(id, func(t *testing.T) {
			home := t.TempDir()
			ad := ByID(All(home), id)
			host := ad.(ProviderHost)
			previewer := ad.(ProviderPreviewer)
			p := ProviderProfile{Name: "preview", BaseURL: "https://example.test/v1", Model: "first", Token: "hidden-test-token", EnvKey: "TEST_KEY"}
			if id == "codex" {
				p.WireAPI = "responses"
			}
			before, err := previewer.PreviewProvider(&p)
			if err != nil {
				t.Fatal(err)
			}
			again, err := previewer.PreviewProvider(&p)
			if err != nil || !reflect.DeepEqual(before, again) {
				t.Fatal("preview changed ownership or file state")
			}
			for _, f := range before.Files {
				if f.Action != "create" || len(f.Changes) == 0 {
					t.Fatalf("missing creation preview: %+v", f)
				}
				if _, err := os.Stat(f.Path); !os.IsNotExist(err) {
					t.Fatal("preview created a file")
				}
			}
			if err := host.ApplyProvider(p, filepath.Join(home, "backups")); err != nil {
				t.Fatal(err)
			}
			stable, err := previewer.PreviewProvider(&p)
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range stable.Files {
				if f.Action != "unchanged" || len(f.Changes) != 0 {
					t.Fatalf("preview and actual apply differ: %+v", f)
				}
			}
			p.Model = "second"
			p.Token = "changed-hidden-token"
			changed, err := previewer.PreviewProvider(&p)
			if err != nil || changed.Revision == stable.Revision {
				t.Fatal("changed operation was not bound to review")
			}
			data, _ := json.Marshal(changed)
			if strings.Contains(string(data), p.Token) || strings.Contains(string(data), changed.Revision) || !strings.Contains(string(data), "hidden") {
				t.Fatal("unsafe or missing changed fields")
			}
			// Clearing an owned file must accurately distinguish deletion from updates.
			clear, err := previewer.PreviewProvider(nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := host.ClearProvider(filepath.Join(home, "backups")); err != nil {
				t.Fatal(err)
			}
			for _, f := range clear.Files {
				info, err := os.Stat(f.Path)
				switch f.Action {
				case "remove":
					if !os.IsNotExist(err) {
						t.Fatalf("previewed deletion did not happen: %+v", f)
					}
				case "update":
					if err != nil || info.IsDir() {
						t.Fatalf("previewed update did not happen: %+v", f)
					}
				default:
					t.Fatalf("missing clear edit: %+v", f)
				}
			}
		})
	}
}
