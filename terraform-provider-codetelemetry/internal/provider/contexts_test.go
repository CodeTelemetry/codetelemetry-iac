package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
)

func TestExplicitOrganizationNeverUsesPersonalRoutes(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Query().Get("team") != "tm_synthetic" {
			t.Error("missing organization fence")
		}
		fmt.Fprint(w, `{}`)
	}))
	defer server.Close()
	c, err := configuredClient(context.Background(), server.URL, testIDToken("owner", "one"), "tm_synthetic", "")
	if err != nil {
		t.Fatal(err)
	}
	personal, _ := workspaceBinding(server.URL, c.token)
	if c.binding == personal {
		t.Fatal("organization reused personal state binding")
	}
	if err := c.request(context.Background(), "GET", "/api/alert-rules", nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.request(context.Background(), "GET", "/api/enrichment-config", nil, nil); err == nil {
		t.Fatal("unsupported org resource fell back to personal")
	}
	if requests != 1 {
		t.Fatal("unsupported resource reached HTTP")
	}
}

func TestSavedContextConfiguresOrganizationGraderWithoutJWT(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODETELEMETRY_CONTEXT_HOME", home)
	team := "tm_synthetic"
	scope := contextScope{Tenant: "owner", Team: &team, Billing: "org-billing"}
	key := "cot_synthetic_context_0000000000000000"
	var mu sync.Mutex
	checks := []map[string]json.RawMessage{}
	revision := 1
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer "+key {
			t.Error("context credential missing")
		}
		if r.URL.Path == "/api/connectivity/check" {
			var body struct {
				Expected contextScope `json:"expected"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			if body.Expected.Billing != scope.Billing || body.Expected.Team == nil || *body.Expected.Team != team {
				t.Error("scope preflight missing")
			}
			w.WriteHeader(402)
			json.NewEncoder(w).Encode(map[string]any{"state": "budget_exhausted", "scope": scope})
			return
		}
		var bound contextScope
		if json.Unmarshal([]byte(r.Header.Get("X-CodeTelemetry-Workspace")), &bound) != nil || bound.Team == nil || *bound.Team != team {
			t.Error("request scope not fenced")
		}
		if r.URL.Path == "/api/agent/grader" {
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "config": map[string]any{"revision": fmt.Sprintf("%064x", revision), "draft": map[string]any{"checks": checks, "requestedFeatures": "Keep focus"}, "totalChecks": len(checks)}})
			return
		}
		if r.URL.Path == "/api/agent/grader/configure" {
			var body struct {
				Upserts  []map[string]json.RawMessage `json:"upsertChecks"`
				Features string                       `json:"features"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			if body.Features != "Keep focus" {
				t.Error("lost existing requested features")
			}
			checks = body.Upserts
			revision++
			writes++
			fmt.Fprint(w, `{"ok":true,"config":{}}`)
			return
		}
		t.Errorf("unexpected route %s", r.URL.Path)
		w.WriteHeader(404)
	}))
	defer server.Close()
	saved := localContext{Version: 1, Name: "synthetic", Endpoint: server.URL, Scope: scope, Key: key}
	raw, _ := json.Marshal(saved)
	file := filepath.Join(home, "synthetic.json")
	if err := os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	c, err := configuredClient(context.Background(), "", "", "tm_synthetic", "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	r := &graderConfigResource{c: c}
	data := graderConfigModel{Checks: jsontypes.NewNormalizedValue(graderJSON(graderExample(t)))}
	if err := r.save(context.Background(), &data, nil); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	if writes != 1 || len(checks) == 0 {
		t.Fatal("context did not configure organization Grader")
	}
	mu.Unlock()
	if _, err := configuredClient(context.Background(), "", "", "tm_other", "synthetic"); err == nil {
		t.Fatal("accepted organization mismatch")
	}
	if _, err := configuredClient(context.Background(), "", "other-credential", "", "synthetic"); err == nil {
		t.Fatal("accepted credential conflict")
	}
	os.Chmod(file, 0644)
	if _, err := configuredClient(context.Background(), "", "", "", "synthetic"); err == nil {
		t.Fatal("accepted public credentials file")
	}
	os.Chmod(file, 0600)
	if err := c.request(context.Background(), "GET", "/api/alert-rules", nil, nil); err == nil || !strings.Contains(err.Error(), "management token") {
		t.Fatal("ingest context escaped Grader capability")
	}
	if os.Getenv("TF_ACC") == "1" {
		t.Setenv("CODETELEMETRY_TOKEN", "")
		t.Setenv("CODETELEMETRY_ENDPOINT", "")
		config := func(weight int) string {
			selected := graderExample(t)
			selected[0]["weight"] = json.RawMessage(fmt.Sprint(weight))
			return fmt.Sprintf(`provider "codetelemetry" {workspace="synthetic"}
resource "codetelemetry_grader_config" "main" {checks_json=%q}
`, graderJSON(selected))
		}
		tfresource.Test(t, tfresource.TestCase{ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){"codetelemetry": providerserver.NewProtocol6WithError(New("test")())}, Steps: []tfresource.TestStep{
			{Config: config(1), Check: tfresource.TestCheckResourceAttrSet("codetelemetry_grader_config.main", "workspace_binding")},
			{Config: config(1), PlanOnly: true},
			{Config: config(2)},
		}})
		mu.Lock()
		defer mu.Unlock()
		if len(checks) != 0 {
			t.Fatal("context Grader destroy did not remove managed checks")
		}
	}

}
