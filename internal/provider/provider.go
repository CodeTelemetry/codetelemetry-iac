package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type telemetryProvider struct{ version string }
type providerModel struct {
	Endpoint     types.String `tfsdk:"endpoint"`
	Token        types.String `tfsdk:"token"`
	Organization types.String `tfsdk:"organization_id"`
	Workspace    types.String `tfsdk:"workspace"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &telemetryProvider{version} }
}
func (p *telemetryProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "codetelemetry"
	resp.Version = p.version
}
func (*telemetryProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Manage Code Telemetry workspace configuration through its authenticated API.", Attributes: map[string]schema.Attribute{
		"endpoint":        schema.StringAttribute{Optional: true, Description: "Service origin. Falls back to CODETELEMETRY_ENDPOINT. HTTPS required except on loopback."},
		"organization_id": schema.StringAttribute{Optional: true, Description: "Explicit organization ID, or CODETELEMETRY_ORGANIZATION_ID. Never falls back to personal scope."},
		"workspace":       schema.StringAttribute{Optional: true, Description: "Named Connect/MCP workspace context, or CODETELEMETRY_WORKSPACE. Repository bindings are discovered automatically. Credentials remain outside Terraform state."},
		"token":           schema.StringAttribute{Optional: true, Sensitive: true, Description: "Cognito ID token, or CODETELEMETRY_TOKEN. Ingest keys are not accepted."},
	}}
}
func (*telemetryProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.Endpoint.IsUnknown() || data.Token.IsUnknown() || data.Organization.IsUnknown() || data.Workspace.IsUnknown() {
		resp.Diagnostics.AddError("Unknown provider configuration", "Endpoint and token must be known before configuring the provider.")
		return
	}
	endpoint, token := data.Endpoint.ValueString(), data.Token.ValueString()
	if data.Endpoint.IsNull() {
		endpoint = os.Getenv("CODETELEMETRY_ENDPOINT")
	}
	if data.Token.IsNull() {
		token = os.Getenv("CODETELEMETRY_TOKEN")
	}
	organization := data.Organization.ValueString()
	if data.Organization.IsNull() {
		organization = os.Getenv("CODETELEMETRY_ORGANIZATION_ID")
	}
	workspace := data.Workspace.ValueString()
	if data.Workspace.IsNull() {
		workspace = os.Getenv("CODETELEMETRY_WORKSPACE")
	}
	c, err := configuredClient(ctx, endpoint, token, organization, workspace)
	if err != nil {
		resp.Diagnostics.AddError("Invalid workspace configuration", err.Error())
		return
	}
	resp.ResourceData = c
}
func (*telemetryProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{newAlarmRulesResource, newEnrichmentResource, newGraderConfigResource}
}
func (*telemetryProvider) DataSources(context.Context) []func() datasource.DataSource { return nil }
