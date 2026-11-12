package provider

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestWorkspaceBinding(t *testing.T) {
	a, err := workspaceBinding("https://api.example.com", testIDToken("a", "original"))
	if err != nil {
		t.Fatal(err)
	}
	renewed, err := workspaceBinding("https://api.example.com", testIDToken("a", "renewed"))
	if err != nil {
		t.Fatal(err)
	}
	other, _ := workspaceBinding("https://api.example.com", testIDToken("b", "original"))
	endpoint, _ := workspaceBinding("https://other.example.com", testIDToken("a", "original"))
	if a != renewed || a == other || a == endpoint || len(a) != 64 {
		t.Fatal("workspace binding failed scope separation or token rotation")
	}
	for _, token := range []string{"token", "a.b.c", strings.Repeat("a", 16385)} {
		if _, err := workspaceBinding("https://api.example.com", token); err == nil {
			t.Fatal("accepted invalid token")
		}
	}
}

// A refresh is itself dangerous when it rebinds a singleton's state to another
// workspace. Check every lifecycle path and prove rejection before any HTTP.
func TestWorkspaceChangeRejectedBeforeHTTP(t *testing.T) {
	ctx := context.Background()
	for _, operation := range []string{"read", "update", "delete"} {
		t.Run("alarms/"+operation, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("workspace mismatch reached HTTP") }))
			defer server.Close()
			c, _ := newClient(server.URL, testIDToken("b", "two"))
			c.binding, _ = workspaceBinding(c.endpoint, c.token)
			old, _ := workspaceBinding(c.endpoint, testIDToken("a", "one"))
			r := &alarmRulesResource{c: c}
			var schemaResp resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
			prior := alarmRulesModel{ID: types.StringValue("workspace"), Rules: jsontypes.NewNormalizedValue("[]"), Revision: types.Int64Value(1), Binding: types.StringValue(old)}
			state := tfsdk.State{Schema: schemaResp.Schema}
			if d := state.Set(ctx, &prior); d.HasError() {
				t.Fatal(d)
			}
			plan := tfsdk.Plan{Schema: schemaResp.Schema}
			if d := plan.Set(ctx, &prior); d.HasError() {
				t.Fatal(d)
			}
			switch operation {
			case "read":
				resp := resource.ReadResponse{State: state}
				r.Read(ctx, resource.ReadRequest{State: state}, &resp)
				if !resp.Diagnostics.HasError() {
					t.Fatal("accepted changed workspace")
				}
			case "update":
				resp := resource.UpdateResponse{State: state}
				r.Update(ctx, resource.UpdateRequest{State: state, Plan: plan}, &resp)
				if !resp.Diagnostics.HasError() {
					t.Fatal("accepted changed workspace")
				}
			case "delete":
				resp := resource.DeleteResponse{State: state}
				r.Delete(ctx, resource.DeleteRequest{State: state}, &resp)
				if !resp.Diagnostics.HasError() {
					t.Fatal("accepted changed workspace")
				}
			}
		})
		t.Run("enrichment/"+operation, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("workspace mismatch reached HTTP") }))
			defer server.Close()
			c, _ := newClient(server.URL, testIDToken("b", "two"))
			c.binding, _ = workspaceBinding(c.endpoint, c.token)
			old, _ := workspaceBinding(c.endpoint, testIDToken("a", "one"))
			r := &enrichmentResource{c: c}
			var schemaResp resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
			prior := enrichmentModel{ID: types.StringValue("workspace"), MintFloor: types.Float64Value(.6), MergeThreshold: types.Float64Value(.86), Cadence: types.StringValue("nightly"), BudgetPct: types.Float64Value(.25), Revision: types.Int64Value(1), Binding: types.StringValue(old)}
			state := tfsdk.State{Schema: schemaResp.Schema}
			if d := state.Set(ctx, &prior); d.HasError() {
				t.Fatal(d)
			}
			plan := tfsdk.Plan{Schema: schemaResp.Schema}
			if d := plan.Set(ctx, &prior); d.HasError() {
				t.Fatal(d)
			}
			switch operation {
			case "read":
				resp := resource.ReadResponse{State: state}
				r.Read(ctx, resource.ReadRequest{State: state}, &resp)
				if !resp.Diagnostics.HasError() {
					t.Fatal("accepted changed workspace")
				}
			case "update":
				resp := resource.UpdateResponse{State: state}
				r.Update(ctx, resource.UpdateRequest{State: state, Plan: plan}, &resp)
				if !resp.Diagnostics.HasError() {
					t.Fatal("accepted changed workspace")
				}
			case "delete":
				resp := resource.DeleteResponse{State: state}
				r.Delete(ctx, resource.DeleteRequest{State: state}, &resp)
				if !resp.Diagnostics.HasError() {
					t.Fatal("accepted changed workspace")
				}
			}
		})
	}
}

func TestLegacyWorkspaceBindingBackfill(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/alert-rules" {
			w.Write([]byte(`{"rules":[],"rev":1}`))
			return
		}
		w.Write([]byte(`{"mintFloor":0.6,"mergeThreshold":0.86,"consolidationCadence":"nightly","budgetPct":0.25,"configRev":1}`))
	}))
	defer server.Close()
	c, _ := newClient(server.URL, testIDToken("a", "one"))
	c.binding, _ = workspaceBinding(c.endpoint, c.token)
	alarms := &alarmRulesResource{c: c}
	var aSchema resource.SchemaResponse
	alarms.Schema(ctx, resource.SchemaRequest{}, &aSchema)
	aState := tfsdk.State{Schema: aSchema.Schema}
	aPrior := alarmRulesModel{ID: types.StringValue("workspace"), Rules: jsontypes.NewNormalizedValue("[]"), Revision: types.Int64Value(1)}
	if d := aState.Set(ctx, &aPrior); d.HasError() {
		t.Fatal(d)
	}
	aResp := resource.ReadResponse{State: aState}
	alarms.Read(ctx, resource.ReadRequest{State: aState}, &aResp)
	if aResp.Diagnostics.HasError() {
		t.Fatal(aResp.Diagnostics)
	}
	var aAfter alarmRulesModel
	if d := aResp.State.Get(ctx, &aAfter); d.HasError() {
		t.Fatal(d)
	}
	if aAfter.Binding.ValueString() != c.binding {
		t.Fatal("legacy alarm state was not bound")
	}
	enrichment := &enrichmentResource{c: c}
	var eSchema resource.SchemaResponse
	enrichment.Schema(ctx, resource.SchemaRequest{}, &eSchema)
	eState := tfsdk.State{Schema: eSchema.Schema}
	ePrior := enrichmentModel{ID: types.StringValue("workspace")}
	if d := eState.Set(ctx, &ePrior); d.HasError() {
		t.Fatal(d)
	}
	eResp := resource.ReadResponse{State: eState}
	enrichment.Read(ctx, resource.ReadRequest{State: eState}, &eResp)
	if eResp.Diagnostics.HasError() {
		t.Fatal(eResp.Diagnostics)
	}
	var eAfter enrichmentModel
	if d := eResp.State.Get(ctx, &eAfter); d.HasError() {
		t.Fatal(d)
	}
	if eAfter.Binding.ValueString() != c.binding {
		t.Fatal("legacy enrichment state was not bound")
	}
}

// Exercise the actual protocol upgrade path, not only an already-shaped model:
// old JSON state has no workspace_binding attribute at all.
func TestLegacyJSONStateUpgrade(t *testing.T) {
	ctx := context.Background()
	server := providerserver.NewProtocol6(New("test")())()
	for _, tc := range []struct {
		name, raw string
		model     resource.Resource
	}{
		{"codetelemetry_alarm_rules", `{"id":"workspace","rules_json":"[]","revision":1}`, &alarmRulesResource{}},
		{"codetelemetry_enrichment_config", `{"id":"workspace","mint_floor":0.6,"merge_threshold":0.86,"consolidation_cadence":"nightly","budget_pct":0.25,"config_revision":1}`, &enrichmentResource{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := server.UpgradeResourceState(ctx, &tfprotov6.UpgradeResourceStateRequest{TypeName: tc.name, Version: 0, RawState: &tfprotov6.RawState{JSON: []byte(tc.raw)}})
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range result.Diagnostics {
				if d.Severity == tfprotov6.DiagnosticSeverityError {
					t.Fatal(d.Summary, d.Detail)
				}
			}
			if result.UpgradedState == nil {
				t.Fatal("missing upgraded state")
			}
			var schemaResp resource.SchemaResponse
			tc.model.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
			value, err := result.UpgradedState.Unmarshal(schemaResp.Schema.Type().TerraformType(ctx))
			if err != nil {
				t.Fatal(err)
			}
			var attributes map[string]tftypes.Value
			if err := value.As(&attributes); err != nil {
				t.Fatal(err)
			}
			binding, ok := attributes["workspace_binding"]
			if !ok || !binding.IsNull() {
				t.Fatal("legacy state did not acquire a null binding for first refresh")
			}
		})
	}
}

func TestWorkspaceClaimNamesAreExact(t *testing.T) {
	expected, err := workspaceBinding("https://api.example.com", testIDToken("workspace-b", "one"))
	if err != nil {
		t.Fatal(err)
	}
	raw := `{"iss":"https://cognito-idp.example.test/pool","sub":"workspace-b","SUB":"workspace-a","token_use":"id"}`
	token := "header." + base64.RawURLEncoding.EncodeToString([]byte(raw)) + ".signature"
	actual, err := workspaceBinding("https://api.example.com", token)
	if err != nil {
		t.Fatal(err)
	}
	if actual != expected {
		t.Fatal("uppercase alias shadowed the service's lowercase subject")
	}
	raw = `{"iss":"https://cognito-idp.example.test/pool","SUB":"workspace-a","TOKEN_USE":"id"}`
	token = "header." + base64.RawURLEncoding.EncodeToString([]byte(raw)) + ".signature"
	if _, err := workspaceBinding("https://api.example.com", token); err == nil {
		t.Fatal("accepted noncanonical claim aliases")
	}
}
