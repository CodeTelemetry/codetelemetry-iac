package provider

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestResolvedAlarmInputsRejectBeforeHTTP(t *testing.T) {
	fixture, err := os.ReadFile("../../examples/rules.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{strings.Repeat(" ", 300001) + "[]", `[{}]`, strings.Replace(string(fixture), "Daily spend over $100", "\\ufeffDaily spend over $100", 1)} {
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("invalid resolved rules reached HTTP") }))
		c, _ := newClient(server.URL, "token")
		r := alarmRulesResource{c: c}
		data := alarmRulesModel{Rules: jsontypes.NewNormalizedValue(raw)}
		if err := r.save(context.Background(), &data, 0); err == nil {
			t.Error("accepted invalid resolved rules")
		}
		var schemaResp resource.SchemaResponse
		r.Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)
		plan := tfsdk.Plan{Schema: schemaResp.Schema}
		if d := plan.Set(context.Background(), &data); d.HasError() {
			t.Fatal(d)
		}
		resp := resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
		r.Create(context.Background(), resource.CreateRequest{Plan: plan}, &resp)
		if !resp.Diagnostics.HasError() {
			t.Error("create accepted invalid resolved rules")
		}
		server.Close()
	}
}

func TestResolvedEnrichmentInputsRejectBeforeHTTP(t *testing.T) {
	for _, mutate := range []func(*enrichmentModel){
		func(d *enrichmentModel) { d.BudgetPct = types.Float64Value(1.1) },
		func(d *enrichmentModel) { d.MintFloor = types.Float64Value(-.1) },
		func(d *enrichmentModel) { d.MergeThreshold = types.Float64Value(math.Inf(1)) },
		func(d *enrichmentModel) { d.BudgetPct = types.Float64Unknown() },
		func(d *enrichmentModel) { d.BudgetPct = types.Float64Null() },
		func(d *enrichmentModel) { d.Cadence = types.StringValue("hourly") },
	} {
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("invalid resolved enrichment reached HTTP") }))
		c, _ := newClient(server.URL, "token")
		r := enrichmentResource{c: c}
		data := enrichmentModel{MintFloor: types.Float64Value(.6), MergeThreshold: types.Float64Value(.86), BudgetPct: types.Float64Value(.25), Cadence: types.StringValue("nightly")}
		mutate(&data)
		if err := r.transfer(context.Background(), "PUT", &data); err == nil {
			t.Error("accepted invalid resolved enrichment")
		}
		server.Close()
	}
}
