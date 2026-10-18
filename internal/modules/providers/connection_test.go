package providers

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rogeriojunior31/lazyagents/internal/agent"
	"github.com/rogeriojunior31/lazyagents/internal/cli"
	"github.com/rogeriojunior31/lazyagents/internal/core"
)

func TestConnectionProbeAndCatalogEvidence(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.ContentLength != 0 {
			t.Error("check sent a request body")
		}
		switch r.Method {
		case http.MethodHead:
			if r.URL.Path != "/tenant/v1/" || r.Header.Get("Authorization") != "" {
				t.Error("probe changed the base path or sent credentials")
			}
			w.WriteHeader(http.StatusUnauthorized)
		case http.MethodGet:
			if r.URL.Path != "/tenant/v1/models" || r.Header.Get("Authorization") != "Bearer fixture-saved-secret" {
				t.Error("catalog lost path or credential precedence")
			}
			w.Write([]byte(`{"data":[{"id":"z-model"},{"id":"a-model"},{"id":"z-model"}]}`))
		default:
			t.Error("unexpected request method")
		}
	}))
	defer server.Close()
	paths := core.PathsIn(t.TempDir())
	svc := New(nil, paths)
	p := agent.ProviderProfile{Name: "network", BaseURL: server.URL + "/tenant/v1/", Model: "a-model", Token: "fixture-saved-secret", EnvKey: "NETWORK_TEST_KEY", WireAPI: "responses"}
	t.Setenv(p.EnvKey, "different-env-secret")
	if err := svc.Save(p); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(svc.Path())
	probe := svc.CheckSaved(context.Background(), p.Name, false)
	if !probe.Reachable || probe.HTTPStatus != 401 || probe.Credential != "not_sent" || probe.Catalog || probe.ModelStatus != "not_checked" || probe.Error != "" {
		t.Fatalf("probe overstated evidence: %+v", probe)
	}
	models := svc.CheckSaved(context.Background(), p.Name, true)
	if !models.Reachable || !models.Catalog || models.Credential != "sent" || models.ModelStatus != "listed" || !reflect.DeepEqual(models.Models, []string{"a-model", "z-model"}) {
		t.Fatalf("catalog evidence: %+v", models)
	}
	after, _ := os.ReadFile(svc.Path())
	if !bytes.Equal(before, after) || requests.Load() != 2 {
		t.Fatal("checks mutated library or made extra requests")
	}
	if _, err := os.Stat(paths.BackupsDir()); !os.IsNotExist(err) {
		t.Fatal("checks wrote backups")
	}
	var out, errs bytes.Buffer
	if code := providerConnection("models", []string{p.Name, "--json"}, cli.Context{Out: &out, Err: &errs}, svc); code != 0 || !json.Valid(out.Bytes()) {
		t.Fatal("catalog CLI failed")
	}
	if strings.Contains(out.String()+errs.String(), p.Token) || strings.Contains(out.String(), "different-env-secret") {
		t.Fatal("credential exposed")
	}
}

func TestCatalogPathsAndEnvironmentReferences(t *testing.T) {
	pathSeen := make(chan string, 8)
	authSeen := make(chan bool, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathSeen <- r.URL.EscapedPath()
		authSeen <- r.Header.Get("Authorization") == "Bearer fixture-env-secret"
		w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()
	t.Setenv("NETWORK_TEST_KEY", "fixture-env-secret")
	for _, prefix := range []string{"/v1", "/gateway/v1/", "/custom/", "/tenant%2Fscope/v1"} {
		r := checkConnection(context.Background(), agent.ProviderProfile{BaseURL: server.URL + prefix, EnvKey: "NETWORK_TEST_KEY", Model: "missing"}, true)
		if !r.Catalog || r.ModelStatus != "not_listed" || <-pathSeen != strings.TrimRight(prefix, "/")+"/models" || !<-authSeen {
			t.Fatalf("catalog path/ref failed for %s: %+v", prefix, r)
		}
	}
	r := checkConnection(context.Background(), agent.ProviderProfile{BaseURL: server.URL, WireAPI: "chat"}, true)
	if !r.Catalog || r.Credential != "not_sent" || r.ModelStatus != "not_selected" || <-pathSeen != "/models" || <-authSeen {
		t.Fatal("keyless catalog failed")
	}
	t.Setenv("NETWORK_TEST_KEY", "")
	for _, p := range []agent.ProviderProfile{
		{BaseURL: server.URL, EnvKey: "NETWORK_TEST_KEY"},
		{BaseURL: server.URL, EnvKey: "INVALID KEY"},
		{BaseURL: server.URL, Token: "bad\nfixture-secret"},
		{BaseURL: server.URL + "?key=fixture-secret"},
		{BaseURL: server.URL + "?"},
		{BaseURL: "http://user:fixture-secret@example.test"},
		{BaseURL: server.URL + "/v1/models"},
		{BaseURL: server.URL, WireAPI: "anthropic"},
		{BaseURL: server.URL, WireAPI: "gemini", Token: "fixture-secret"},
	} {
		r := checkConnection(context.Background(), p, true)
		if r.Error == "" || r.Reachable || strings.Contains(r.Error, "fixture-secret") {
			t.Fatal("invalid catalog inputs were requested or exposed")
		}
	}
	select {
	case <-pathSeen:
		t.Fatal("invalid inputs made a network request")
	default:
	}
}

func TestCatalogFailuresAndHostileResponsesStayRedacted(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		body       string
		credential string
	}{
		{"unauthorized", 401, `fixture-secret`, "rejected"},
		{"forbidden", 403, `fixture-secret`, "forbidden"},
		{"unsupported", 404, `fixture-secret`, "sent"},
		{"method", 405, `fixture-secret`, "sent"},
		{"server", 500, `fixture-secret`, "sent"},
		{"html", 200, `<html>fixture-secret</html>`, "sent"},
		{"null", 200, `{"data":null}`, "sent"},
		{"missing-id", 200, `{"data":[{}]}`, "sent"},
		{"long-id", 200, `{"data":[{"id":"` + strings.Repeat("x", 257) + `"}]}`, "sent"},
		{"id-limit", 200, `{"data":[` + strings.TrimSuffix(strings.Repeat(`{"id":"m"},`, catalogModelLimit+1), ",") + `]}`, "sent"},
		{"missing", 200, `{}`, "sent"},
		{"partial", 200, `{"data":[],"has_more":true}`, "sent"},
		{"echo", 200, `{"data":[{"id":"echo-fixture-secret"}]}`, "sent"},
		{"controls", 200, `{"data":[{"id":"bad\u001b[2J"}]}`, "sent"},
		{"oversized", 200, strings.Repeat("x", catalogBodyLimit+1), "sent"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tt.status); w.Write([]byte(tt.body)) }))
			defer server.Close()
			result := checkConnection(context.Background(), agent.ProviderProfile{BaseURL: server.URL, Token: "fixture-secret"}, true)
			if !result.Reachable || result.Catalog || len(result.Models) != 0 || result.Error == "" || result.Credential != tt.credential || result.ModelStatus != "not_checked" {
				t.Fatalf("false success: %+v", result)
			}
			encoded, _ := json.Marshal(result)
			if bytes.Contains(encoded, []byte("fixture-secret")) || bytes.Contains(encoded, []byte("<html>")) {
				t.Fatal("server content leaked into result")
			}
		})
	}
}

func TestCatalogNeverFollowsRedirects(t *testing.T) {
	var sinkCalls atomic.Int32
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { sinkCalls.Add(1) }))
	defer sink.Close()
	var redirectCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectCalls.Add(1)
		target := sink.URL + "/steal?key=fixture-secret"
		if r.URL.Path == "/same/models" {
			target = "/elsewhere?key=fixture-secret"
		}
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	for _, suffix := range []string{"/same", "/cross"} {
		r := checkConnection(context.Background(), agent.ProviderProfile{BaseURL: server.URL + suffix, Token: "fixture-secret"}, true)
		if !r.Reachable || r.HTTPStatus != 307 || r.Catalog || !strings.Contains(r.Error, "redirect") || strings.Contains(r.Error, "fixture-secret") {
			t.Fatal("redirect accepted or exposed")
		}
	}
	if sinkCalls.Load() != 0 || redirectCalls.Load() != 2 {
		t.Fatal("credentials/request forwarded through redirect")
	}
}

func TestConnectionCancellationAndTimeout(t *testing.T) {
	for _, bodyStall := range []bool{false, true} {
		t.Run(map[bool]string{false: "headers", true: "body"}[bodyStall], func(t *testing.T) {
			started := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if bodyStall {
					w.Write([]byte(`{"data":[`))
					w.(http.Flusher).Flush()
				}
				close(started)
				<-r.Context().Done()
			}))
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan ConnectionResult, 1)
			go func() {
				done <- checkConnection(ctx, agent.ProviderProfile{BaseURL: server.URL, Token: "fixture-secret"}, true)
			}()
			<-started
			cancel()
			select {
			case r := <-done:
				if !strings.Contains(r.Error, "cancelled") || r.Catalog || len(r.Models) > 0 {
					t.Fatalf("cancel not honored: %+v", r)
				}
			case <-time.After(time.Second):
				t.Fatal("cancellation did not stop the request")
			}
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	r := checkConnection(ctx, agent.ProviderProfile{BaseURL: server.URL}, false)
	if !strings.Contains(r.Error, "timed out") {
		t.Fatalf("deadline not honored: %+v", r)
	}
}

func TestConnectionBoundsAndTLSVerification(t *testing.T) {
	for _, compressed := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if compressed {
				w.Header().Set("Content-Encoding", "gzip")
				gz := gzip.NewWriter(w)
				gz.Write([]byte(strings.Repeat("x", catalogBodyLimit+1)))
				gz.Close()
			} else {
				w.Header().Set("X-Large", strings.Repeat("x", 128<<10))
				w.Write([]byte(`{"data":[]}`))
			}
		}))
		r := checkConnection(context.Background(), agent.ProviderProfile{BaseURL: server.URL}, true)
		server.Close()
		if r.Error == "" || r.Catalog || compressed && !strings.Contains(r.Error, "1 MiB") {
			t.Fatalf("response bounds not enforced: %+v", r)
		}
	}
	var calls atomic.Int32
	tlsServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	tlsServer.Config.ErrorLog = log.New(io.Discard, "", 0)
	tlsServer.StartTLS()
	defer tlsServer.Close()
	r := checkConnection(context.Background(), agent.ProviderProfile{BaseURL: tlsServer.URL + "/fixture-secret", Token: "fixture-secret"}, true)
	encoded, _ := json.Marshal(r)
	if r.Error == "" || r.Reachable || calls.Load() != 0 || bytes.Contains(encoded, []byte("fixture-secret")) {
		t.Fatal("untrusted TLS certificate accepted or URL/credential exposed")
	}
}
