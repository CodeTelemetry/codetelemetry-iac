package provider

import (
	"context"
	"fmt"
	"math"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type enrichmentResource struct{ c *client }
type enrichmentModel struct {
	ID             types.String  `tfsdk:"id"`
	MintFloor      types.Float64 `tfsdk:"mint_floor"`
	MergeThreshold types.Float64 `tfsdk:"merge_threshold"`
	Cadence        types.String  `tfsdk:"consolidation_cadence"`
	BudgetPct      types.Float64 `tfsdk:"budget_pct"`
	Revision       types.Int64   `tfsdk:"config_revision"`
	Binding        types.String  `tfsdk:"workspace_binding"`
}
type enrichmentConfig struct {
	MintFloor      float64 `json:"mintFloor"`
	MergeThreshold float64 `json:"mergeThreshold"`
	Cadence        string  `json:"consolidationCadence"`
	BudgetPct      float64 `json:"budgetPct"`
	Revision       int64   `json:"configRev,omitempty"`
}

func newEnrichmentResource() resource.Resource { return &enrichmentResource{} }
func (*enrichmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_enrichment_config"
}
func (*enrichmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Workspace classifier/enrichment settings. Destroy restores service defaults; it does not delete classified telemetry. One resource per authenticated workspace.", Attributes: map[string]schema.Attribute{
		"id":                    schema.StringAttribute{Computed: true},
		"workspace_binding":     schema.StringAttribute{Computed: true, Description: "Fingerprint of service origin and stable workspace identity; prevents accidental credential or endpoint rebinding."},
		"mint_floor":            schema.Float64Attribute{Required: true, Description: "Label reuse confidence, between 0 and 1."},
		"merge_threshold":       schema.Float64Attribute{Required: true, Description: "Near-duplicate embedding similarity, between 0 and 1."},
		"consolidation_cadence": schema.StringAttribute{Required: true, Description: "nightly, weekly, or manual."},
		"budget_pct":            schema.Float64Attribute{Required: true, Description: "Enrichment budget as a fraction from 0 to 1 (0.25 means 25%)."},
		"config_revision":       schema.Int64Attribute{Computed: true, Description: "Server-owned cache invalidation revision."},
	}}
}
func (r *enrichmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider client", "Expected Code Telemetry API client.")
		return
	}
	r.c = c
}
func (*enrichmentResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data enrichmentModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	for key, value := range map[string]types.Float64{"mint_floor": data.MintFloor, "merge_threshold": data.MergeThreshold, "budget_pct": data.BudgetPct} {
		if !value.IsNull() && !value.IsUnknown() && (value.ValueFloat64() < 0 || value.ValueFloat64() > 1) {
			resp.Diagnostics.AddAttributeError(path.Root(key), "Invalid enrichment setting", "Value must be between 0 and 1.")
		}
	}
	if !data.Cadence.IsUnknown() && !data.Cadence.IsNull() && !validCadence(data.Cadence.ValueString()) {
		resp.Diagnostics.AddAttributeError(path.Root("consolidation_cadence"), "Invalid cadence", "Use nightly, weekly, or manual.")
	}
}
func validCadence(s string) bool { return s == "nightly" || s == "weekly" || s == "manual" }
func (r *enrichmentResource) transfer(ctx context.Context, method string, data *enrichmentModel) error {
	var body any
	if method == "PUT" {
		// Apply-time values may have been unknown during ValidateConfig. Reject
		// values the service would clamp rather than writing a surprising default.
		for _, value := range []types.Float64{data.MintFloor, data.MergeThreshold, data.BudgetPct} {
			if value.IsNull() || value.IsUnknown() || math.IsNaN(value.ValueFloat64()) || math.IsInf(value.ValueFloat64(), 0) || value.ValueFloat64() < 0 || value.ValueFloat64() > 1 {
				return fmt.Errorf("enrichment fractions must be known values between 0 and 1")
			}
		}
		if data.Cadence.IsNull() || data.Cadence.IsUnknown() || !validCadence(data.Cadence.ValueString()) {
			return fmt.Errorf("consolidation cadence must be nightly, weekly, or manual")
		}
		body = enrichmentConfig{MintFloor: data.MintFloor.ValueFloat64(), MergeThreshold: data.MergeThreshold.ValueFloat64(), Cadence: data.Cadence.ValueString(), BudgetPct: data.BudgetPct.ValueFloat64()}
	}
	// Pointers detect truncated successful responses instead of manufacturing zero settings.
	var remote struct {
		MintFloor      *float64 `json:"mintFloor"`
		MergeThreshold *float64 `json:"mergeThreshold"`
		Cadence        *string  `json:"consolidationCadence"`
		BudgetPct      *float64 `json:"budgetPct"`
		Revision       *int64   `json:"configRev"`
	}
	if err := r.c.request(ctx, method, "/api/enrichment-config", body, &remote); err != nil {
		return err
	}
	if remote.MintFloor == nil || remote.MergeThreshold == nil || remote.Cadence == nil || remote.BudgetPct == nil || remote.Revision == nil {
		return fmt.Errorf("incomplete enrichment response")
	}
	if *remote.MintFloor < 0 || *remote.MintFloor > 1 || *remote.MergeThreshold < 0 || *remote.MergeThreshold > 1 || *remote.BudgetPct < 0 || *remote.BudgetPct > 1 || !validCadence(*remote.Cadence) || *remote.Revision < 1 {
		return fmt.Errorf("invalid enrichment response")
	}
	*data = enrichmentModel{types.StringValue("workspace"), types.Float64Value(*remote.MintFloor), types.Float64Value(*remote.MergeThreshold), types.StringValue(*remote.Cadence), types.Float64Value(*remote.BudgetPct), types.Int64Value(*remote.Revision), types.StringValue(r.c.binding)}
	return nil
}
func (r *enrichmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data enrichmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.transfer(ctx, "PUT", &data); err != nil {
		resp.Diagnostics.AddError("Cannot save enrichment config", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *enrichmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data enrichmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.checkWorkspace(data.Binding); err != nil {
		resp.Diagnostics.AddError("Workspace mismatch", err.Error())
		return
	}
	if data.Binding.IsNull() && !data.MintFloor.IsNull() {
		resp.Diagnostics.AddWarning("Legacy workspace identity", "This state has no saved workspace fingerprint. Its first successful refresh binds the current endpoint and token identity. Verify the original workspace credentials before applying changes.")
	}
	if err := r.transfer(ctx, "GET", &data); err != nil {
		resp.Diagnostics.AddError("Cannot read enrichment config", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *enrichmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data, prior enrichmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.checkWorkspace(prior.Binding); err != nil {
		resp.Diagnostics.AddError("Workspace mismatch", err.Error())
		return
	}
	if err := r.transfer(ctx, "PUT", &data); err != nil {
		resp.Diagnostics.AddError("Cannot save enrichment config", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *enrichmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var prior enrichmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.checkWorkspace(prior.Binding); err != nil {
		resp.Diagnostics.AddError("Workspace mismatch", err.Error())
		return
	}
	data := enrichmentModel{MintFloor: types.Float64Value(0.6), MergeThreshold: types.Float64Value(0.86), Cadence: types.StringValue("nightly"), BudgetPct: types.Float64Value(0.25)}
	if err := r.transfer(ctx, "PUT", &data); err != nil {
		resp.Diagnostics.AddError("Cannot reset enrichment config", err.Error())
	}
}
func (*enrichmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID != "workspace" {
		resp.Diagnostics.AddError("Invalid import ID", "Use workspace; the token selects the authenticated workspace.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
