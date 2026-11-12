package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccWorkspaceLifecycle(t *testing.T) {
	fixture, err := os.ReadFile("../../examples/rules.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRules(string(fixture)); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	rules := json.RawMessage("[]")
	revision := int64(0)
	config := enrichmentConfig{0.6, 0.86, "nightly", 0.25, 1}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer "+testIDToken("workspace-a", "one") {
			w.WriteHeader(401)
			return
		}
		switch r.URL.Path {
		case "/api/alert-rules":
			if r.Method == "PUT" {
				var body struct {
					Rules    json.RawMessage `json:"rules"`
					Revision int64           `json:"rev"`
				}
				if json.NewDecoder(r.Body).Decode(&body) != nil {
					w.WriteHeader(400)
					return
				}
				if body.Revision != revision {
					w.WriteHeader(409)
					return
				}
				rules = body.Rules
				revision++
			}
			json.NewEncoder(w).Encode(rulesResponse{rules, &revision})
		case "/api/enrichment-config":
			if r.Method == "PUT" {
				next := config
				if json.NewDecoder(r.Body).Decode(&next) != nil {
					w.WriteHeader(400)
					return
				}
				next.Revision = config.Revision + 1
				config = next
			}
			json.NewEncoder(w).Encode(config)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("CODETELEMETRY_ENDPOINT", server.URL)
	t.Setenv("CODETELEMETRY_TOKEN", testIDToken("workspace-a", "one"))
	terraformConfig := func(budget float64, threshold int) string {
		var configured []map[string]any
		if err := json.Unmarshal(fixture, &configured); err != nil {
			t.Fatal(err)
		}
		configured[0]["tql"].(map[string]any)["threshold"] = threshold
		configuredJSON, err := json.Marshal(configured)
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf(`
provider "codetelemetry" {}
resource "codetelemetry_alarm_rules" "main" { rules_json = %q }
resource "codetelemetry_enrichment_config" "main" {
 mint_floor = 0.6
 merge_threshold = 0.86
 consolidation_cadence = "nightly"
 budget_pct = %g
}
`, string(configuredJSON), budget)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){"codetelemetry": providerserver.NewProtocol6WithError(New("test")())},
		Steps: []resource.TestStep{
			{Config: terraformConfig(0.25, 100), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("codetelemetry_alarm_rules.main", "id", "workspace"), resource.TestCheckResourceAttr("codetelemetry_alarm_rules.main", "revision", "1"), resource.TestCheckResourceAttr("codetelemetry_enrichment_config.main", "budget_pct", "0.25"))},
			{ResourceName: "codetelemetry_alarm_rules.main", ImportState: true, ImportStateId: "workspace", ImportStateVerify: true, ImportStateVerifyIgnore: []string{"rules_json"}, ImportStateCheck: func(states []*terraform.InstanceState) error {
				var want, got any
				if len(states) != 1 {
					return fmt.Errorf("expected one imported resource")
				}
				if err := json.Unmarshal(fixture, &want); err != nil {
					return err
				}
				if err := json.Unmarshal([]byte(states[0].Attributes["rules_json"]), &got); err != nil {
					return err
				}
				if !reflect.DeepEqual(want, got) {
					return fmt.Errorf("imported rules differ")
				}
				return nil
			}},
			{ResourceName: "codetelemetry_enrichment_config.main", ImportState: true, ImportStateId: "workspace", ImportStateVerify: true},
			{Config: terraformConfig(0, 200), Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("codetelemetry_enrichment_config.main", "budget_pct", "0"),
				resource.TestCheckResourceAttr("codetelemetry_alarm_rules.main", "revision", "2"),
				func(_ *terraform.State) error {
					mu.Lock()
					defer mu.Unlock()
					var saved []map[string]any
					if err := json.Unmarshal(rules, &saved); err != nil {
						return err
					}
					if saved[0]["tql"].(map[string]any)["threshold"] != float64(200) || config.BudgetPct != 0 {
						return fmt.Errorf("updates did not reach API: %s %+v", rules, config)
					}
					return nil
				},
			)},
			{Config: terraformConfig(0, 200), PlanOnly: true},
			{Config: terraformConfig(0.4, 200), Check: resource.TestCheckResourceAttr("codetelemetry_enrichment_config.main", "budget_pct", "0.4")},
			{Config: terraformConfig(0.4, 200), PlanOnly: true, ExpectNonEmptyPlan: true, PreConfig: func() {
				mu.Lock()
				defer mu.Unlock()
				config.BudgetPct = 0.9
				rules = json.RawMessage("[]")
				revision++
			}},
			{Config: terraformConfig(0.4, 200), Check: resource.TestCheckResourceAttr("codetelemetry_alarm_rules.main", "revision", "4")},
		},
	})
	mu.Lock()
	defer mu.Unlock()
	if string(rules) != "[]" || config.MintFloor != 0.6 || config.MergeThreshold != 0.86 || config.Cadence != "nightly" || config.BudgetPct != 0.25 {
		t.Fatalf("destroy did not clear/reset settings: %s %+v", rules, config)
	}
}

func TestAlarmRevisionConflict(t *testing.T) {
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writes++; w.WriteHeader(409) }))
	defer server.Close()
	c, _ := newClient(server.URL, "token")
	r := alarmRulesResource{c: c}
	data := alarmRulesModel{}
	data.Rules = jsontypes.NewNormalizedValue("[]")
	if err := r.save(context.Background(), &data, 7); err == nil {
		t.Fatal("accepted stale revision")
	}
	if writes != 1 {
		t.Fatal("retried a conflicting write")
	}
}

func TestRulesResponseRequiresRevision(t *testing.T) {
	if err := validRulesResponse(rulesResponse{Rules: json.RawMessage("[]")}); err == nil {
		t.Fatal("accepted missing revision")
	}
	for _, raw := range []string{"null", "{}", "[null]", "[{}]"} {
		if err := validateRules(raw); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}

func TestRulesRejectLossyInputs(t *testing.T) {
	fixture, err := os.ReadFile("../../examples/rules.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func([]map[string]any){
		func(r []map[string]any) { r[0]["name"] = " padded " },
		func(r []map[string]any) { r[0]["name"] = "\ufeffpadded" },
		func(r []map[string]any) { r[0]["unexpected"] = true },
		func(r []map[string]any) { r[0]["id name"] = true },
		func(r []map[string]any) { r[0]["tql"].(map[string]any)["query"] = " invocations | stats count() as n" },
		func(r []map[string]any) {
			r[0]["channels"] = []any{map[string]any{"type": "in_app", "url": "https://example.com"}}
		},
		func(r []map[string]any) { r[0]["origin"] = map[string]any{"kind": "custom", "version": 1} },
	} {
		var rules []map[string]any
		if err := json.Unmarshal(fixture, &rules); err != nil {
			t.Fatal(err)
		}
		mutate(rules)
		raw, _ := json.Marshal(rules)
		if err := validateRules(string(raw)); err == nil {
			t.Fatal("accepted lossy input")
		}
	}
}

func TestAccAlarmRulesRefuseExisting(t *testing.T) {
	fixture, err := os.ReadFile("../../examples/rules.json")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	writes := 0
	revision := int64(1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method != "GET" {
			writes++
			w.WriteHeader(500)
			return
		}
		json.NewEncoder(w).Encode(rulesResponse{fixture, &revision})
	}))
	defer server.Close()
	t.Setenv("CODETELEMETRY_ENDPOINT", server.URL)
	t.Setenv("CODETELEMETRY_TOKEN", testIDToken("workspace-a", "one"))
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){"codetelemetry": providerserver.NewProtocol6WithError(New("test")())},
		Steps: []resource.TestStep{{Config: fmt.Sprintf(`provider "codetelemetry" {}
resource "codetelemetry_alarm_rules" "main" {rules_json=%q}
`, string(fixture)), ExpectError: regexp.MustCompile("Alarm rules already exist")}},
	})
	mu.Lock()
	defer mu.Unlock()
	if writes != 0 {
		t.Fatalf("attempted %d writes to unadopted workspace", writes)
	}
}

func testIDToken(subject, nonce string) string {
	raw, _ := json.Marshal(map[string]string{"iss": "https://cognito-idp.example.test/pool", "sub": subject, "token_use": "id", "nonce": nonce})
	return "eyJhbGciOiJub25lIn0." + base64.RawURLEncoding.EncodeToString(raw) + ".local-test-signature"
}

// Persist imported state before planning: isolated import verification misses
// JSON ordering/escaping drift against the user's configuration.
func TestAccAlarmImportHasNoDrift(t *testing.T) {
	fixture, err := os.ReadFile("../../examples/rules.json")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method != "GET" {
			writes++
			t.Error("import and plan must not write")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"rules":%s,"rev":7}`, fixture)
	}))
	defer server.Close()
	t.Setenv("CODETELEMETRY_ENDPOINT", server.URL)
	t.Setenv("CODETELEMETRY_TOKEN", testIDToken("workspace-a", "one"))
	config := fmt.Sprintf(`provider "codetelemetry" {}
 resource "codetelemetry_alarm_rules" "main" { rules_json = %q }
 `, string(fixture))
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){"codetelemetry": providerserver.NewProtocol6WithError(New("test")())},
		Steps: []resource.TestStep{
			{Config: config, ResourceName: "codetelemetry_alarm_rules.main", ImportState: true, ImportStateId: "workspace", ImportStatePersist: true},
			{Config: config, PlanOnly: true},
			{Config: `provider "codetelemetry" {}
removed {
from = codetelemetry_alarm_rules.main
lifecycle { destroy = false }
}`},
		},
	})
	mu.Lock()
	defer mu.Unlock()
	if writes != 0 {
		t.Fatalf("unexpected writes: %d", writes)
	}
}
