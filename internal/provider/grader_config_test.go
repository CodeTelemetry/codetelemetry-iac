package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

type graderFixture struct {
	mu       sync.Mutex
	checks   []map[string]json.RawMessage
	revision int
	writes   int
	conflict bool
}

func (f *graderFixture) handler(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if r.Header.Get("Authorization") != "Bearer "+testIDToken("workspace-a", "grader") {
		w.WriteHeader(401)
		return
	}
	if r.URL.Path == graderAPI && r.Method == "GET" {
		json.NewEncoder(w).Encode(map[string]any{"draft": map[string]any{"checks": f.checks, "requestedFeatures": "Keep focus"}, "revision": fmt.Sprintf("%064x", f.revision)})
		return
	}
	if r.URL.Path != graderAPI+"/draft" || r.Method != "PUT" {
		w.WriteHeader(404)
		return
	}
	var body struct {
		Checks   []map[string]json.RawMessage `json:"checks"`
		Revision string                       `json:"expectedRevision"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		w.WriteHeader(400)
		return
	}
	if body.Revision != fmt.Sprintf("%064x", f.revision) || f.conflict {
		w.WriteHeader(409)
		return
	}
	f.checks = body.Checks
	f.revision++
	f.writes++
	json.NewEncoder(w).Encode(map[string]any{"draft": map[string]any{"checks": f.checks, "requestedFeatures": "Keep focus"}})
}
func graderExample(t *testing.T) []map[string]json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile("../../examples/grader-checks.json")
	if err != nil {
		t.Fatal(err)
	}
	checks, err := parseGraderChecks(string(raw), true)
	if err != nil {
		t.Fatal(err)
	}
	return checks
}
func graderJSON(checks []map[string]json.RawMessage) string {
	raw, _ := json.Marshal(checks)
	return string(raw)
}
func TestAccGraderLifecycle(t *testing.T) {
	unrelated := graderExample(t)[0]
	unrelated["id"] = json.RawMessage(`"unrelated"`)
	fixture := graderFixture{checks: []map[string]json.RawMessage{unrelated}, revision: 1}
	server := httptest.NewServer(http.HandlerFunc(fixture.handler))
	defer server.Close()
	t.Setenv("CODETELEMETRY_ENDPOINT", server.URL)
	t.Setenv("CODETELEMETRY_TOKEN", testIDToken("workspace-a", "grader"))
	config := func(weight int) string {
		checks := graderExample(t)
		checks[0]["weight"] = json.RawMessage(fmt.Sprint(weight))
		return fmt.Sprintf(`provider "codetelemetry" {}
resource "codetelemetry_grader_config" "main" {checks_json=%q}
`, graderJSON(checks))
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){"codetelemetry": providerserver.NewProtocol6WithError(New("test")())},
		Steps: []resource.TestStep{
			{Config: config(1), Check: resource.TestCheckResourceAttr("codetelemetry_grader_config.main", "id", "workspace")},
			{Config: config(1), PlanOnly: true},
			{Config: config(2), Check: resource.TestCheckResourceAttr("codetelemetry_grader_config.main", "revision", fmt.Sprintf("%064x", 3))},
			{Config: config(2), PlanOnly: true, ExpectNonEmptyPlan: true, PreConfig: func() {
				fixture.mu.Lock()
				defer fixture.mu.Unlock()
				fixture.checks[1]["weight"] = json.RawMessage("9")
				fixture.revision++
			}},
			{Config: config(2)},
			{Config: config(3), ExpectError: regexp.MustCompile("HTTP 409"), PreConfig: func() { fixture.mu.Lock(); defer fixture.mu.Unlock(); fixture.conflict = true }},
			{Config: config(3), PreConfig: func() { fixture.mu.Lock(); defer fixture.mu.Unlock(); fixture.conflict = false }},
		},
	})
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if len(fixture.checks) != 1 || graderCheckID(fixture.checks[0]) != "unrelated" {
		t.Fatalf("destroy changed unrelated checks: %s", graderJSON(fixture.checks))
	}
}
func TestAccGraderImport(t *testing.T) {
	checks := graderExample(t)
	fixture := graderFixture{checks: checks, revision: 1}
	server := httptest.NewServer(http.HandlerFunc(fixture.handler))
	defer server.Close()
	t.Setenv("CODETELEMETRY_ENDPOINT", server.URL)
	t.Setenv("CODETELEMETRY_TOKEN", testIDToken("workspace-a", "grader"))
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){"codetelemetry": providerserver.NewProtocol6WithError(New("test")())},
		Steps: []resource.TestStep{
			{Config: fmt.Sprintf(`provider "codetelemetry" {}
resource "codetelemetry_grader_config" "main" {checks_json=%q}`, graderJSON(checks)), Check: func(_ *terraform.State) error {
				fixture.mu.Lock()
				defer fixture.mu.Unlock()
				if fixture.writes != 0 {
					return fmt.Errorf("identical adoption wrote config")
				}
				return nil
			}},
			{ResourceName: "codetelemetry_grader_config.main", ImportState: true, ImportStateId: "workspace", ImportStateVerify: true},
		},
	})
}
func TestGraderGuards(t *testing.T) {
	checks := graderExample(t)
	fixture := graderFixture{checks: checks, revision: 5}
	server := httptest.NewServer(http.HandlerFunc(fixture.handler))
	defer server.Close()
	c, _ := newClient(server.URL, testIDToken("workspace-a", "grader"))
	c.binding = "bound"
	r := graderConfigResource{c: c}
	data := graderConfigModel{Checks: jsontypes.NewNormalizedValue(graderJSON(checks))}
	prior := graderConfigModel{Checks: data.Checks, Revision: types.StringValue(fmt.Sprintf("%064x", 4)), Binding: types.StringValue("bound")}
	if err := r.save(context.Background(), &data, &prior); err == nil {
		t.Fatal("accepted stale revision")
	}
	prior.Binding = types.StringValue("other")
	if err := r.save(context.Background(), &data, &prior); err == nil {
		t.Fatal("accepted changed workspace")
	}
	changed := graderExample(t)
	changed[0]["weight"] = json.RawMessage("2")
	data.Checks = jsontypes.NewNormalizedValue(graderJSON(changed))
	if err := r.save(context.Background(), &data, nil); err == nil {
		t.Fatal("silently replaced unowned existing check")
	}
	for _, raw := range []string{"null", "[null]", "[{}]", `[{"id":"x"}]`, graderJSON(append(checks, checks[0]))} {
		if _, err := parseGraderChecks(raw, true); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.writes != 0 {
		t.Fatal("guards performed a write")
	}
}

func TestAccGraderTerraformArtifact(t *testing.T) {
	root := os.Getenv("CODETELEMETRY_MCP_ROOT")
	if root == "" {
		t.Skip("optional cross-repository artifact check: set CODETELEMETRY_MCP_ROOT to a built public MCP checkout")
	}
	directory := t.TempDir()
	script := `
import fs from 'node:fs';
import path from 'node:path';
import {pathToFileURL} from 'node:url';
const {callTool}=await import(pathToFileURL(path.join(process.env.CODETELEMETRY_MCP_ROOT,'dist/src/tools.js')));
const {OnboardingSession}=await import(pathToFileURL(path.join(process.env.CODETELEMETRY_MCP_ROOT,'dist/src/session.js')));
const checks=JSON.parse(fs.readFileSync(process.env.GRADER_FIXTURE,'utf8'));
checks[0].description='Literal ${file("/etc/passwd")} and %{if true}';
fs.writeFileSync(path.join(process.env.GRADER_DIRECTORY,'main.tf'),'provider "codetelemetry" {}');
const result=await callTool('write_grader_config_artifact',{repositoryPath:process.env.GRADER_DIRECTORY,features:'Test ${literal}',checks},{session:new OnboardingSession()});
if(!result.ok || result.artifact.format!=='terraform')throw new Error(JSON.stringify(result));
`
	fixturePath, err := filepath.Abs("../../examples/grader-checks.json")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", "--input-type=module", "-e", script)
	cmd.Env = append(os.Environ(), "GRADER_FIXTURE="+fixturePath, "GRADER_DIRECTORY="+directory)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("artifact tool: %v: %s", err, output)
	}
	fixture := graderFixture{checks: []map[string]json.RawMessage{}, revision: 1}
	server := httptest.NewServer(http.HandlerFunc(fixture.handler))
	defer server.Close()
	t.Setenv("CODETELEMETRY_ENDPOINT", server.URL)
	t.Setenv("CODETELEMETRY_TOKEN", testIDToken("workspace-a", "grader"))
	resource.Test(t, resource.TestCase{
		Steps: []resource.TestStep{{ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){"codetelemetry": providerserver.NewProtocol6WithError(New("test")())}, ConfigDirectory: config.StaticDirectory(directory), Check: func(_ *terraform.State) error {
			fixture.mu.Lock()
			defer fixture.mu.Unlock()
			var description string
			_ = json.Unmarshal(fixture.checks[0]["description"], &description)
			if description != `Literal ${file("/etc/passwd")} and %{if true}` {
				return fmt.Errorf("template text did not remain literal")
			}
			return nil
		}}},
	})
}

func TestGraderPreservesServiceSupportedCheckShapes(t *testing.T) {
	excluded := graderExample(t)[0]
	excluded["matcherRule"] = json.RawMessage(`{"exclude":{"field":"executable","operator":"equals","value":"echo"}}`)
	linked := graderExample(t)[0]
	linked["id"] = json.RawMessage(`"linked-fixture"`)
	linked["satisfactionMode"] = json.RawMessage(`"composite"`)
	linked["linkage"] = json.RawMessage(`"signadot"`)
	linked["components"] = json.RawMessage(`[
 {"name":"Sandbox","requireSuccess":true,"matcherRule":{"include":{"field":"executable","operator":"equals","value":"signadot"}}},
 {"name":"Request","requireSuccess":true,"matcherRule":{"include":{"field":"executable","operator":"equals","value":"curl"}}},
 {"name":"Tests","requireSuccess":true,"matcherRule":{"include":{"field":"executable","operator":"equals","value":"npm"}}}
 ]`)
	for _, check := range []map[string]json.RawMessage{excluded, linked} {
		parsed, err := parseGraderChecks(graderJSON([]map[string]json.RawMessage{check}), true)
		if err != nil {
			t.Fatal(err)
		}
		if len(parsed) != 1 || !sameGraderCheck(parsed[0], check) {
			t.Fatal("changed a service-supported check")
		}
	}
}
